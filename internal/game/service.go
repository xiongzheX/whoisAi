package game

import (
	"errors"
	"math/rand"
	"strconv"
	"strings"
)

type Event struct {
	Name     string
	RoomID   string
	SocketID string
	Payload  any
}

type Service struct {
	store           *Store
	messageRewriter MessageRewriter
}

func NewService(store *Store, options ...ServiceOption) *Service {
	service := &Service{
		store:           store,
		messageRewriter: localMessageRewriter{},
	}
	for _, option := range options {
		if option != nil {
			option(service)
		}
	}
	return service
}

func MissionTeamSize(playerCount, roundNumber int) int {
	configs := map[int][]int{
		5: {2, 3, 2, 3},
		6: {2, 3, 4, 3},
		7: {2, 3, 3, 4},
		8: {3, 4, 4, 4},
	}
	config, ok := configs[playerCount]
	if !ok {
		config = configs[6]
	}
	index := roundNumber - 1
	if index < 0 {
		index = 0
	}
	if index >= len(config) {
		index = len(config) - 1
	}
	return config[index]
}

func (s *Service) StartGame(roomID, starterID string) (*Room, []Event, error) {
	s.store.mu.Lock()
	defer s.store.mu.Unlock()

	room, ok := s.store.rooms[roomID]
	if !ok {
		return nil, nil, errors.New("房间不存在")
	}
	if !playerInRoom(room, starterID) {
		return nil, nil, errors.New("只有房间内玩家可以开始游戏")
	}
	if !roomHostCanStart(room, starterID) {
		return nil, nil, errors.New("只有房主可以开始游戏")
	}
	if room.Status == StatusPlaying {
		return cloneRoom(room), nil, nil
	}

	humans := humanPlayers(room)
	if room.Mode != ModeTest && room.Mode != ModeSolo && len(humans) < MinPlayers {
		return nil, nil, errors.New("至少需要 5 名玩家才能开始")
	}
	if (room.Mode == ModeTest || room.Mode == ModeSolo) && len(humans) < 1 {
		return nil, nil, errors.New("至少需要 1 名真人玩家")
	}

	room.Status = StatusPlaying
	room.CurrentRound = 1
	room.CurrentPhase = PhasePropose
	room.MissionResults = []bool{}
	room.MissionSuccesses = 0
	room.MissionFailures = 0
	room.ProposedTeam = []string{}
	room.TeamVotes = map[string]bool{}
	room.MessageCount = map[string]int{}
	room.ChatMessages = []ChatMessage{}
	room.SignalHistory = []SignalRecord{}

	humanIDs := make([]string, 0, len(humans))
	for _, player := range humans {
		humanIDs = append(humanIDs, player.ID)
	}
	roleIDs := roleEligiblePlayerIDs(room)
	room.Roles = assignRoomRoles(room, roleIDs)
	room.CurrentLeader = humanIDs[rand.Intn(len(humanIDs))]
	room.PossessedPlayer, room.PossessionStyle = choosePossession(humanIDs)
	room.SignalHistory = append(room.SignalHistory, SignalRecord{
		Round:         room.CurrentRound,
		HasPossession: room.PossessedPlayer != "",
	})

	events := make([]Event, 0, len(humans)+1)
	for _, player := range humans {
		role := room.Roles[player.ID]
		events = append(events, Event{
			Name:     "rolesRevealed",
			RoomID:   roomID,
			SocketID: player.ID,
			Payload: map[string]any{
				"role":            role,
				"roleLabel":       RoleLabels[role],
				"roleDescription": RoleDescription(role),
				"players":         publicPlayers(room.Players),
			},
		})
	}
	events = append(events, Event{
		Name:   "phaseChange",
		RoomID: roomID,
		Payload: map[string]any{
			"phase":          PhasePropose,
			"roundNumber":    room.CurrentRound,
			"totalRounds":    MaxRounds,
			"leader":         leaderPayload(room),
			"teamSize":       MissionTeamSize(len(humans), room.CurrentRound),
			"missionResults": room.MissionResults,
			"timeLimit":      15,
		},
	})
	events = append(events, possessionEvents(roomID, room)...)
	return cloneRoom(room), events, nil
}

func (s *Service) Chat(roomID, playerID, input string) (ChatMessage, int, error) {
	message, err := ValidateChatMessage(input)
	if err != nil {
		return ChatMessage{}, 0, err
	}

	s.store.mu.Lock()
	defer s.store.mu.Unlock()

	room, ok := s.store.rooms[roomID]
	if !ok {
		return ChatMessage{}, 0, errors.New("房间不存在")
	}
	player, ok := findPlayer(room, playerID)
	if !ok {
		return ChatMessage{}, 0, errors.New("玩家不在房间中")
	}
	if room.MessageCount == nil {
		room.MessageCount = map[string]int{}
	}
	if room.MessageCount[playerID] >= MaxMessagesPerRound {
		return ChatMessage{}, 0, errors.New("本轮发言次数已用完")
	}

	displayed := message
	possessed := playerID == room.PossessedPlayer
	if possessed {
		displayed = s.rewritePossessedMessage(message, room.PossessionStyle)
	}
	room.MessageCount[playerID]++
	chatMessage := ChatMessage{
		PlayerID:   playerID,
		PlayerName: player.Name,
		Original:   message,
		Displayed:  displayed,
		Possessed:  possessed,
	}
	room.ChatMessages = append(room.ChatMessages, chatMessage)
	return chatMessage, MaxMessagesPerRound - room.MessageCount[playerID], nil
}

func (s *Service) rewritePossessedMessage(message string, style Style) string {
	if s.messageRewriter == nil {
		return RewriteMessage(message, style)
	}
	rewritten, err := s.messageRewriter.Rewrite(message, style)
	if err != nil || strings.TrimSpace(rewritten) == "" {
		return RewriteMessage(message, style)
	}
	return rewritten
}

func (s *Service) ProposeMission(roomID, proposerID string, memberIDs []string) (*Room, []Event, error) {
	s.store.mu.Lock()
	defer s.store.mu.Unlock()

	room, ok := s.store.rooms[roomID]
	if !ok {
		return nil, nil, errors.New("房间不存在")
	}
	if room.CurrentPhase != PhasePropose {
		return nil, nil, errors.New("当前不是提名阶段")
	}
	if room.CurrentLeader != proposerID {
		return nil, nil, errors.New("只有队长可以提名")
	}

	humans := humanPlayers(room)
	teamSize := MissionTeamSize(len(humans), room.CurrentRound)
	if len(memberIDs) != teamSize {
		return nil, nil, errors.New("提名人数不正确")
	}
	valid := map[string]bool{}
	for _, player := range room.Players {
		if !player.Eliminated && (room.Mode == ModeTest || room.Mode == ModeSolo || !player.IsAI) {
			valid[player.ID] = true
		}
	}
	for _, id := range memberIDs {
		if !valid[id] {
			return nil, nil, errors.New("提名了无效的玩家")
		}
	}

	room.ProposedTeam = append([]string(nil), memberIDs...)
	room.CurrentPhase = PhaseDiscuss
	room.MessageCount = map[string]int{}
	room.ChatMessages = []ChatMessage{}

	events := []Event{
		{
			Name:   "missionProposed",
			RoomID: roomID,
			Payload: map[string]any{
				"leaderId":    proposerID,
				"leaderName":  playerName(room, proposerID),
				"memberIds":   memberIDs,
				"memberNames": playerNames(room, memberIDs),
			},
		},
		{
			Name:   "phaseChange",
			RoomID: roomID,
			Payload: map[string]any{
				"phase":             PhaseDiscuss,
				"roundNumber":       room.CurrentRound,
				"totalRounds":       MaxRounds,
				"proposedTeam":      room.ProposedTeam,
				"proposedTeamNames": playerNames(room, memberIDs),
				"missionResults":    room.MissionResults,
				"maxMessages":       MaxMessagesPerRound,
				"maxChars":          MaxCharsPerMessage,
				"timeLimit":         45,
			},
		},
	}
	return cloneRoom(room), events, nil
}

func (s *Service) TeamVote(roomID, voterID string, approve bool) (*Room, []Event, error) {
	s.store.mu.Lock()
	defer s.store.mu.Unlock()

	room, ok := s.store.rooms[roomID]
	if !ok {
		return nil, nil, errors.New("房间不存在")
	}
	if room.CurrentPhase != PhaseTeamVote && room.CurrentPhase != PhaseDiscuss {
		return nil, nil, errors.New("当前不是投票阶段")
	}
	if !playerCanVote(room, voterID) {
		return nil, nil, errors.New("无效投票玩家")
	}
	room.CurrentPhase = PhaseTeamVote
	room.TeamVotes[voterID] = approve
	if room.Mode == ModeTest || room.Mode == ModeSolo {
		for _, player := range room.Players {
			if player.IsAI && !player.Eliminated {
				room.TeamVotes[player.ID] = true
			}
		}
	}

	voters := alivePlayers(room)
	if len(room.TeamVotes) < len(voters) {
		return cloneRoom(room), nil, nil
	}

	approveCount := 0
	voteDisplay := map[string]PlayerVote{}
	for _, player := range voters {
		vote := room.TeamVotes[player.ID]
		if vote {
			approveCount++
		}
		voteDisplay[player.ID] = PlayerVote{VoterName: player.Name, Approved: vote}
	}
	approved := approveCount*2 > len(voters)
	room.VoteHistory = append(room.VoteHistory, VoteRecord{
		Round:        room.CurrentRound,
		Votes:        voteDisplay,
		Approved:     approved,
		Team:         append([]string(nil), room.ProposedTeam...),
		ApproveCount: approveCount,
		RejectCount:  len(voters) - approveCount,
		HasPossess:   room.PossessedPlayer != "",
	})

	events := []Event{{
		Name:   "teamVoteResult",
		RoomID: roomID,
		Payload: map[string]any{
			"approved":     approved,
			"approveCount": approveCount,
			"rejectCount":  len(voters) - approveCount,
			"votes":        voteDisplay,
			"voteHistory":  room.VoteHistory,
		},
	}}

	if !approved {
		room.RejectStreak++
		room.TeamVotes = map[string]bool{}
		room.ProposedTeam = []string{}
		if room.RejectStreak >= 5 {
			room.Status = StatusFinished
			events = append(events, gameFinishedEvent(roomID, room, "infiltrator"))
			return cloneRoom(room), events, nil
		}
		room.CurrentPhase = PhasePropose
		rotateLeader(room)
		events = append(events, phaseProposeEvent(roomID, room))
		return cloneRoom(room), events, nil
	}

	room.RejectStreak = 0
	room.CurrentPhase = PhaseMission
	room.MissionVotes = map[string]string{}
	events = append(events, missionStartEvents(roomID, room)...)
	return cloneRoom(room), events, nil
}

func (s *Service) StartTeamVote(roomID string) (*Room, []Event, error) {
	s.store.mu.Lock()
	defer s.store.mu.Unlock()

	room, ok := s.store.rooms[roomID]
	if !ok {
		return nil, nil, errors.New("房间不存在")
	}
	if room.CurrentPhase != PhaseDiscuss {
		return nil, nil, errors.New("当前不是讨论阶段")
	}
	room.CurrentPhase = PhaseTeamVote
	room.TeamVotes = map[string]bool{}
	event := Event{
		Name:   "phaseChange",
		RoomID: roomID,
		Payload: map[string]any{
			"phase":             PhaseTeamVote,
			"roundNumber":       room.CurrentRound,
			"totalRounds":       MaxRounds,
			"proposedTeam":      room.ProposedTeam,
			"proposedTeamNames": playerNames(room, room.ProposedTeam),
			"missionResults":    room.MissionResults,
			"timeLimit":         20,
		},
	}
	return cloneRoom(room), []Event{event}, nil
}

func (s *Service) MissionVote(roomID, voterID, answer string) (*Room, []Event, error) {
	s.store.mu.Lock()
	defer s.store.mu.Unlock()

	room, ok := s.store.rooms[roomID]
	if !ok {
		return nil, nil, errors.New("房间不存在")
	}
	if room.CurrentPhase != PhaseMission {
		return nil, nil, errors.New("当前不是任务阶段")
	}
	if !contains(room.ProposedTeam, voterID) {
		return nil, nil, errors.New("只有小队成员可以执行任务")
	}
	if answer == "" {
		answer = "A"
	}
	room.MissionVotes[voterID] = answer
	if room.Mode == ModeTest || room.Mode == ModeSolo {
		for _, playerID := range room.ProposedTeam {
			if isAIPlayer(room, playerID) {
				room.MissionVotes[playerID] = aiMissionAnswer(room, playerID)
			}
		}
	}
	if len(room.MissionVotes) < len(room.ProposedTeam) {
		return cloneRoom(room), nil, nil
	}

	success := true
	for _, vote := range room.MissionVotes {
		if vote != "A" {
			success = false
			break
		}
	}
	room.MissionResults = append(room.MissionResults, success)
	if success {
		room.MissionSuccesses++
	} else {
		room.MissionFailures++
	}

	riskActionCount := wrongCount(room.MissionVotes)
	teamNames := playerNames(room, room.ProposedTeam)
	focusPrompts := missionFocusPrompts(room, success, riskActionCount)
	suspicionEvents := missionSuspicionEvents(room, success, riskActionCount)

	events := []Event{{
		Name:   "missionReveal",
		RoomID: roomID,
		Payload: map[string]any{
			"roundNumber":      room.CurrentRound,
			"explanation":      "先控制异常区域并确认风险来源，是当前最稳妥的行动方案。",
			"team":             room.ProposedTeam,
			"teamNames":        teamNames,
			"success":          success,
			"riskActionCount":  riskActionCount,
			"sabotageCount":    riskActionCount,
			"focusPrompts":     focusPrompts,
			"suspicionEvents":  suspicionEvents,
			"missionResults":   room.MissionResults,
			"missionSuccesses": room.MissionSuccesses,
			"missionFailures":  room.MissionFailures,
			"hadPossession":    room.PossessedPlayer != "",
		},
	}, {
		Name:   "missionResult",
		RoomID: roomID,
		Payload: map[string]any{
			"roundNumber":      room.CurrentRound,
			"success":          success,
			"missionResults":   room.MissionResults,
			"missionSuccesses": room.MissionSuccesses,
			"missionFailures":  room.MissionFailures,
			"explanation":      "先控制异常区域并确认风险来源，是当前最稳妥的行动方案。",
			"team":             room.ProposedTeam,
			"teamNames":        teamNames,
			"riskActionCount":  riskActionCount,
			"focusPrompts":     focusPrompts,
			"suspicionEvents":  suspicionEvents,
			"hadPossession":    room.PossessedPlayer != "",
		},
	}}

	if room.MissionSuccesses >= MissionsToWin || room.MissionFailures >= MissionsToWin || room.CurrentRound >= MaxRounds {
		room.Status = StatusFinished
		winner := "engineer"
		if room.MissionFailures >= MissionsToWin || room.MissionSuccesses < MissionsToWin {
			winner = "infiltrator"
		}
		events = append(events, gameFinishedEvent(roomID, room, winner))
		return cloneRoom(room), events, nil
	}

	room.CurrentRound++
	room.CurrentPhase = PhasePropose
	room.TeamVotes = map[string]bool{}
	room.MissionVotes = map[string]string{}
	room.ProposedTeam = []string{}
	room.MessageCount = map[string]int{}
	room.ChatMessages = []ChatMessage{}
	humanIDs := humanIDs(room)
	room.PossessedPlayer, room.PossessionStyle = choosePossession(humanIDs)
	room.SignalHistory = append(room.SignalHistory, SignalRecord{
		Round:         room.CurrentRound,
		HasPossession: room.PossessedPlayer != "",
	})
	rotateLeader(room)
	events = append(events, phaseProposeEvent(roomID, room))
	events = append(events, possessionEvents(roomID, room)...)
	return cloneRoom(room), events, nil
}

func humanPlayers(room *Room) []Player {
	players := make([]Player, 0, len(room.Players))
	for _, player := range room.Players {
		if !player.IsAI && !player.Eliminated {
			players = append(players, player)
		}
	}
	return players
}

func alivePlayers(room *Room) []Player {
	players := make([]Player, 0, len(room.Players))
	for _, player := range room.Players {
		if !player.Eliminated {
			players = append(players, player)
		}
	}
	return players
}

func humanIDs(room *Room) []string {
	humans := humanPlayers(room)
	ids := make([]string, 0, len(humans))
	for _, player := range humans {
		ids = append(ids, player.ID)
	}
	return ids
}

func roleEligiblePlayerIDs(room *Room) []string {
	ids := make([]string, 0, len(room.Players))
	for _, player := range room.Players {
		if player.Eliminated {
			continue
		}
		if room.Mode == ModeTest || room.Mode == ModeSolo || !player.IsAI {
			ids = append(ids, player.ID)
		}
	}
	return ids
}

func assignRoomRoles(room *Room, playerIDs []string) map[string]Role {
	roles := AssignRoles(playerIDs)
	if room.Mode != ModeTest && room.Mode != ModeSolo {
		return roles
	}

	aiID := ""
	for _, player := range room.Players {
		if player.IsAI && !player.Eliminated {
			aiID = player.ID
			if roles[player.ID] == RoleInfiltrator {
				return roles
			}
		}
	}
	if aiID == "" {
		return roles
	}

	for playerID, role := range roles {
		if role == RoleInfiltrator {
			roles[playerID] = roles[aiID]
			roles[aiID] = RoleInfiltrator
			return roles
		}
	}
	roles[aiID] = RoleInfiltrator
	return roles
}

func playerInRoom(room *Room, playerID string) bool {
	if playerID == "" {
		return false
	}
	for _, player := range room.Players {
		if player.ID == playerID && !player.Eliminated {
			return true
		}
	}
	return false
}

func roomHostCanStart(room *Room, playerID string) bool {
	if room.Mode == ModeTest || room.Mode == ModeSolo {
		return true
	}
	for _, player := range room.Players {
		if player.Eliminated || player.IsAI {
			continue
		}
		return player.ID == playerID
	}
	return false
}

func playerCanVote(room *Room, playerID string) bool {
	for _, player := range room.Players {
		if player.ID == playerID && !player.Eliminated {
			return true
		}
	}
	return false
}

func isAIPlayer(room *Room, playerID string) bool {
	for _, player := range room.Players {
		if player.ID == playerID {
			return player.IsAI
		}
	}
	return false
}

func aiMissionAnswer(room *Room, playerID string) string {
	if roleFaction(room.Roles[playerID]) == "evil" {
		return "B"
	}
	return "A"
}

func gameFinishedEvent(roomID string, room *Room, winner string) Event {
	return Event{
		Name:   "gameFinished",
		RoomID: roomID,
		Payload: map[string]any{
			"winner":           winner,
			"winnerLabel":      winnerLabel(winner),
			"roles":            buildRoleReveal(room, winner),
			"missionResults":   room.MissionResults,
			"missionSuccesses": room.MissionSuccesses,
			"missionFailures":  room.MissionFailures,
		},
	}
}

func buildRoleReveal(room *Room, winner string) map[string]RoleReveal {
	reveal := make(map[string]RoleReveal, len(room.Roles))
	for playerID, role := range room.Roles {
		faction := roleFaction(role)
		reveal[playerID] = RoleReveal{
			Name:      playerName(room, playerID),
			Role:      role,
			RoleLabel: RoleLabels[role],
			Faction:   faction,
			IsWinner:  (winner == "engineer" && faction == "good") || (winner == "infiltrator" && faction == "evil"),
		}
	}
	return reveal
}

func wrongCount(votes map[string]string) int {
	count := 0
	for _, answer := range votes {
		if answer != "A" {
			count++
		}
	}
	return count
}

func missionFocusPrompts(room *Room, success bool, riskActionCount int) []string {
	team := playerNames(room, room.ProposedTeam)
	if success {
		return []string{
			"这支小队完成了任务，下一轮可以观察谁试图无理由拆散这组关系。",
			"不要只因为成功就完全放下戒心，渗透者也可能选择暂时隐藏。",
		}
	}

	prompts := []string{
		"任务失败，嫌疑范围先锁定在行动小队：" + joinNames(team) + "。",
		"下一轮优先追问队长为什么选择这支小队，以及队内成员如何解释失败。",
	}
	supporters := approvedVoterNames(room)
	if len(supporters) > 0 {
		prompts = append(prompts, "也要回看支持这支失败小队的玩家："+joinNames(supporters)+"。")
	}
	if room.PossessedPlayer != "" {
		prompts = append(prompts, "本轮存在 AI 信号干扰，发言异常要结合投票和上队记录判断。")
	}
	if riskActionCount > 1 {
		prompts = append(prompts, "本轮出现多次高风险行动，队内可能不止一个人在推动失败。")
	}
	return prompts
}

func missionSuspicionEvents(room *Room, success bool, riskActionCount int) []string {
	team := playerNames(room, room.ProposedTeam)
	round := room.CurrentRound
	if success {
		return []string{
			"第" + itoa(round) + "轮任务成功：行动小队 " + joinNames(team) + " 暂时获得信任。",
		}
	}

	events := []string{
		"第" + itoa(round) + "轮任务失败：嫌疑范围在行动小队 " + joinNames(team) + " 内。",
		"本轮出现 " + itoa(riskActionCount) + " 次高风险行动，但具体归属保持隐藏。",
	}
	supporters := approvedVoterNames(room)
	if len(supporters) > 0 {
		events = append(events, "支持这支失败小队的玩家："+joinNames(supporters)+"。")
	}
	return events
}

func approvedVoterNames(room *Room) []string {
	if len(room.VoteHistory) == 0 {
		return nil
	}
	record := room.VoteHistory[len(room.VoteHistory)-1]
	names := make([]string, 0, len(record.Votes))
	for _, vote := range record.Votes {
		if vote.Approved {
			names = append(names, vote.VoterName)
		}
	}
	return names
}

func joinNames(names []string) string {
	if len(names) == 0 {
		return "暂无"
	}
	return strings.Join(names, "、")
}

func itoa(value int) string {
	return strconv.Itoa(value)
}

func roleFaction(role Role) string {
	switch role {
	case RoleInfiltrator, RoleDisruptor:
		return "evil"
	default:
		return "good"
	}
}

func publicPlayers(players []Player) []map[string]any {
	output := make([]map[string]any, 0, len(players))
	for _, player := range players {
		output = append(output, map[string]any{
			"id":       player.ID,
			"name":     player.Name,
			"position": player.Position,
			"isAI":     player.IsAI,
		})
	}
	return output
}

func choosePossession(playerIDs []string) (string, Style) {
	if len(playerIDs) == 0 || rand.Float64() >= 0.5 {
		return "", ""
	}
	styles := []Style{StylePolite, StyleVerbose, StyleNeutral, StyleAwkward}
	return playerIDs[rand.Intn(len(playerIDs))], styles[rand.Intn(len(styles))]
}

func leaderPayload(room *Room) map[string]any {
	if room.CurrentLeader == "" {
		return nil
	}
	return map[string]any{
		"id":   room.CurrentLeader,
		"name": playerName(room, room.CurrentLeader),
	}
}

func playerName(room *Room, playerID string) string {
	for _, player := range room.Players {
		if player.ID == playerID {
			return player.Name
		}
	}
	return "???"
}

func findPlayer(room *Room, playerID string) (Player, bool) {
	for _, player := range room.Players {
		if player.ID == playerID {
			return player, true
		}
	}
	return Player{}, false
}

func playerNames(room *Room, playerIDs []string) []string {
	names := make([]string, 0, len(playerIDs))
	for _, id := range playerIDs {
		names = append(names, playerName(room, id))
	}
	return names
}

func missionStartEvents(roomID string, room *Room) []Event {
	teamNames := playerNames(room, room.ProposedTeam)
	events := []Event{{
		Name:   "phaseChange",
		RoomID: roomID,
		Payload: map[string]any{
			"phase":             PhaseMission,
			"missionSubPhase":   "puzzle",
			"roundNumber":       room.CurrentRound,
			"totalRounds":       MaxRounds,
			"proposedTeam":      room.ProposedTeam,
			"proposedTeamNames": teamNames,
			"missionResults":    room.MissionResults,
			"timeLimit":         35,
		},
	}}
	for _, memberID := range room.ProposedTeam {
		events = append(events, Event{
			Name:     "missionPuzzle",
			RoomID:   roomID,
			SocketID: memberID,
			Payload: map[string]any{
				"puzzle": map[string]any{
					"id":       "go-minimal-1",
					"title":    "危机行动选择",
					"scenario": "任务现场出现异常信号，小队需要选择最稳妥的行动方案。",
					"options": map[string]string{
						"A": "先控制异常区域，确认风险来源",
						"B": "立刻开放全部通道，加快行动",
						"C": "忽略警报，继续原计划",
					},
				},
				"isPossessed":   memberID == room.PossessedPlayer,
				"possessedHint": possessionHint(memberID == room.PossessedPlayer),
				"canSabotage":   room.Roles[memberID] == RoleInfiltrator,
				"teamNames":     teamNames,
			},
		})
	}
	events = append(events, Event{
		Name:   "missionSubPhase",
		RoomID: roomID,
		Payload: map[string]any{
			"subPhase":    "discuss",
			"timeLimit":   5,
			"maxMessages": 2,
			"maxChars":    30,
		},
	})
	return events
}

func phaseProposeEvent(roomID string, room *Room) Event {
	humans := humanPlayers(room)
	return Event{
		Name:   "phaseChange",
		RoomID: roomID,
		Payload: map[string]any{
			"phase":          PhasePropose,
			"roundNumber":    room.CurrentRound,
			"totalRounds":    MaxRounds,
			"leader":         leaderPayload(room),
			"teamSize":       MissionTeamSize(len(humans), room.CurrentRound),
			"missionResults": room.MissionResults,
			"timeLimit":      15,
		},
	}
}

func rotateLeader(room *Room) {
	humans := humanPlayers(room)
	if len(humans) == 0 {
		return
	}
	for i, player := range humans {
		if player.ID == room.CurrentLeader {
			room.CurrentLeader = humans[(i+1)%len(humans)].ID
			return
		}
	}
	room.CurrentLeader = humans[0].ID
}

func contains(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func possessionHint(possessed bool) string {
	if possessed {
		return "你感觉题目信息有些不稳定。"
	}
	return ""
}

func possessionEvents(roomID string, room *Room) []Event {
	events := []Event{}
	if room.PossessedPlayer != "" {
		events = append(events, Event{
			Name:     "possessionAlert",
			RoomID:   roomID,
			SocketID: room.PossessedPlayer,
			Payload: map[string]any{
				"roundNumber": room.CurrentRound,
			},
		})
	}
	for playerID, role := range room.Roles {
		if role != RoleSignalKeeper {
			continue
		}
		events = append(events, Event{
			Name:     "signalCheck",
			RoomID:   roomID,
			SocketID: playerID,
			Payload: map[string]any{
				"hasPossession": room.PossessedPlayer != "",
				"roundNumber":   room.CurrentRound,
				"signalHistory": room.SignalHistory,
			},
		})
	}
	return events
}

func winnerLabel(winner string) string {
	if winner == "engineer" {
		return "守护者阵营"
	}
	return "渗透者阵营"
}
