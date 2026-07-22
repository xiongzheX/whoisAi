package game

import (
	"errors"
	"math/rand"
	"strconv"
	"strings"
	"time"
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
	connectedPlayers := room.Players[:0]
	for _, player := range room.Players {
		if player.Disconnected {
			continue
		}
		player.Position = len(connectedPlayers)
		connectedPlayers = append(connectedPlayers, player)
	}
	room.Players = connectedPlayers

	humans := humanPlayers(room)
	if room.Mode != ModeTest && room.Mode != ModeSolo && len(humans) < 3 {
		return nil, nil, errors.New("至少需要 3 名真人玩家")
	}
	if room.Mode != ModeTest && room.Mode != ModeSolo && len(room.Players) < MinPlayers {
		return nil, nil, errors.New("人数不足 5 人时请开启 AI 补位")
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
	room.AutoTeamVotes = map[string]bool{}
	room.AutoMissionVotes = map[string]bool{}
	room.MessageCount = map[string]int{}
	room.ChatMessages = []ChatMessage{}
	room.SignalHistory = []SignalRecord{}
	room.SuspicionEvents = []string{}
	room.DiscussionFocus = []string{}
	room.AutoManagedActions = map[string][]AutoManagedAction{}
	room.StartedAtMillis = time.Now().UnixMilli()
	room.EndedAtMillis = 0

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
	proposeTimeLimit := phaseTimeLimit(room, PhasePropose)
	// Reserve enough time for the identity card to be read before the first
	// nomination countdown becomes urgent.
	room.PhaseDeadline = phaseDeadline(proposeTimeLimit, 8)
	events = append(events, Event{
		Name:   "phaseChange",
		RoomID: roomID,
		Payload: map[string]any{
			"phase":          PhasePropose,
			"roundNumber":    room.CurrentRound,
			"totalRounds":    MaxRounds,
			"leader":         leaderPayload(room),
			"teamSize":       MissionTeamSize(len(roleIDs), room.CurrentRound),
			"missionResults": room.MissionResults,
			"timeLimit":      proposeTimeLimit,
			"deadlineAt":     room.PhaseDeadline,
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
		PlayerID:        playerID,
		PlayerName:      player.Name,
		Original:        message,
		Displayed:       displayed,
		Possessed:       possessed,
		Round:           room.CurrentRound,
		Channel:         "discussion",
		CreatedAtMillis: time.Now().UnixMilli(),
	}
	room.ChatMessages = append(room.ChatMessages, chatMessage)
	return chatMessage, MaxMessagesPerRound - room.MessageCount[playerID], nil
}

func (s *Service) MissionChat(roomID, playerID, input string) (ChatMessage, int, error) {
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
	if room.CurrentPhase != PhaseMission || room.MissionSubPhase != "discuss" {
		return ChatMessage{}, 0, errors.New("当前不是任务讨论阶段")
	}
	if !contains(room.ProposedTeam, playerID) {
		return ChatMessage{}, 0, errors.New("只有行动小队成员可以发言")
	}
	player, ok := findPlayer(room, playerID)
	if !ok {
		return ChatMessage{}, 0, errors.New("玩家不在房间中")
	}
	key := "mission:" + playerID
	if room.MessageCount[key] >= MaxMissionMessages {
		return ChatMessage{}, 0, errors.New("本轮任务发言次数已用完")
	}
	room.MessageCount[key]++
	chatMessage := ChatMessage{
		PlayerID:        playerID,
		PlayerName:      player.Name,
		Original:        message,
		Displayed:       message,
		Round:           room.CurrentRound,
		Channel:         "mission",
		CreatedAtMillis: time.Now().UnixMilli(),
	}
	room.ChatMessages = append(room.ChatMessages, chatMessage)
	return chatMessage, MaxMissionMessages - room.MessageCount[key], nil
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
	return s.ProposeMissionWithReason(roomID, proposerID, memberIDs, "队长未填写理由")
}

func (s *Service) ProposeMissionWithReason(roomID, proposerID string, memberIDs []string, inputReason string) (*Room, []Event, error) {
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
	reason, err := ValidateSocialReason(inputReason)
	if err != nil {
		return nil, nil, err
	}

	teamSize := MissionTeamSize(len(alivePlayers(room)), room.CurrentRound)
	if len(memberIDs) != teamSize {
		return nil, nil, errors.New("提名人数不正确")
	}
	valid := map[string]bool{}
	for _, player := range room.Players {
		if !player.Eliminated {
			valid[player.ID] = true
		}
	}
	selected := make(map[string]bool, len(memberIDs))
	for _, id := range memberIDs {
		if !valid[id] || selected[id] {
			return nil, nil, errors.New("提名了无效的玩家")
		}
		selected[id] = true
	}

	room.ProposedTeam = append([]string(nil), memberIDs...)
	room.NominationReason = reason
	room.NominationHistory = append(room.NominationHistory, NominationRecord{
		Round:      room.CurrentRound,
		LeaderID:   proposerID,
		LeaderName: playerName(room, proposerID),
		Team:       append([]string(nil), memberIDs...),
		TeamNames:  playerNames(room, memberIDs),
		Reason:     reason,
	})
	room.CurrentPhase = PhaseDiscuss
	room.MessageCount = map[string]int{}
	discussTimeLimit := phaseTimeLimit(room, PhaseDiscuss)
	room.PhaseDeadline = phaseDeadline(discussTimeLimit, 0)

	events := []Event{
		{
			Name:   "missionProposed",
			RoomID: roomID,
			Payload: map[string]any{
				"leaderId":    proposerID,
				"leaderName":  playerName(room, proposerID),
				"memberIds":   memberIDs,
				"memberNames": playerNames(room, memberIDs),
				"reason":      reason,
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
				"timeLimit":         discussTimeLimit,
				"deadlineAt":        room.PhaseDeadline,
				"nominationReason":  reason,
			},
		},
	}
	return cloneRoom(room), events, nil
}

func (s *Service) SubmitStance(roomID, playerID, trustID, suspectID, inputReason string) (*Room, []Event, error) {
	s.store.mu.Lock()
	defer s.store.mu.Unlock()

	room, ok := s.store.rooms[roomID]
	if !ok {
		return nil, nil, errors.New("房间不存在")
	}
	if room.CurrentPhase != PhaseDiscuss && room.CurrentPhase != PhaseTeamVote {
		return nil, nil, errors.New("当前不是立场表态阶段")
	}
	if !playerCanVote(room, playerID) || trustID == suspectID || trustID == playerID || suspectID == playerID {
		return nil, nil, errors.New("请选择不同的可信与怀疑对象")
	}
	if !playerCanVote(room, trustID) || !playerCanVote(room, suspectID) {
		return nil, nil, errors.New("表态对象无效")
	}
	reason, err := ValidateSocialReason(inputReason)
	if err != nil {
		return nil, nil, err
	}
	if room.Stances == nil {
		room.Stances = map[int]map[string]StanceRecord{}
	}
	if room.Stances[room.CurrentRound] == nil {
		room.Stances[room.CurrentRound] = map[string]StanceRecord{}
	}
	record := StanceRecord{
		Round:       room.CurrentRound,
		PlayerID:    playerID,
		PlayerName:  playerName(room, playerID),
		TrustID:     trustID,
		TrustName:   playerName(room, trustID),
		SuspectID:   suspectID,
		SuspectName: playerName(room, suspectID),
		Reason:      reason,
	}
	room.Stances[room.CurrentRound][playerID] = record
	event := Event{
		Name:   "stanceUpdated",
		RoomID: roomID,
		Payload: map[string]any{
			"roundNumber": room.CurrentRound,
			"stances":     room.Stances[room.CurrentRound],
		},
	}
	return cloneRoom(room), []Event{event}, nil
}

func (s *Service) TeamVote(roomID, voterID string, approve bool) (*Room, []Event, error) {
	return s.teamVote(roomID, voterID, approve, false)
}

// AutoTeamVote records a server-managed vote so clients can distinguish it
// from an intentional player choice.
func (s *Service) AutoTeamVote(roomID, voterID string, approve bool) (*Room, []Event, error) {
	return s.teamVote(roomID, voterID, approve, true)
}

func (s *Service) teamVote(roomID, voterID string, approve, autoManaged bool) (*Room, []Event, error) {
	s.store.mu.Lock()
	defer s.store.mu.Unlock()

	room, ok := s.store.rooms[roomID]
	if !ok {
		return nil, nil, errors.New("房间不存在")
	}
	if room.CurrentPhase != PhaseTeamVote {
		return nil, nil, errors.New("当前不是投票阶段")
	}
	if !playerCanVote(room, voterID) {
		return nil, nil, errors.New("无效投票玩家")
	}
	room.TeamVotes[voterID] = approve
	if room.AutoTeamVotes == nil {
		room.AutoTeamVotes = map[string]bool{}
	}
	room.AutoTeamVotes[voterID] = autoManaged
	if hasAIPlayers(room) {
		for _, player := range room.Players {
			if player.IsAI && !player.Eliminated {
				room.TeamVotes[player.ID] = aiTeamVote(room, player.ID)
				room.AutoTeamVotes[player.ID] = true
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
		voteDisplay[player.ID] = PlayerVote{
			VoterName:   player.Name,
			Approved:    vote,
			AutoManaged: room.AutoTeamVotes[player.ID],
		}
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
			"voteHistory":  publicVoteHistory(room.VoteHistory),
		},
	}}
	if !approved {
		room.RejectStreak++
		room.TeamVotes = map[string]bool{}
		room.AutoTeamVotes = map[string]bool{}
		room.ProposedTeam = []string{}
		if room.RejectStreak >= 5 {
			room.Status = StatusFinished
			room.EndedAtMillis = time.Now().UnixMilli()
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
	room.MissionSubPhase = "discuss"
	room.MissionVotes = map[string]string{}
	room.AutoMissionVotes = map[string]bool{}
	events = append(events, missionStartEvents(roomID, room)...)
	return cloneRoom(room), events, nil
}

func (s *Service) StartMissionVote(roomID string) (*Room, []Event, error) {
	s.store.mu.Lock()
	defer s.store.mu.Unlock()

	room, ok := s.store.rooms[roomID]
	if !ok {
		return nil, nil, errors.New("房间不存在")
	}
	if room.CurrentPhase != PhaseMission || room.MissionSubPhase != "discuss" {
		return nil, nil, errors.New("当前不是任务讨论阶段")
	}
	room.MissionSubPhase = "vote"
	voteTimeLimit := missionVoteTimeLimit(room)
	room.PhaseDeadline = phaseDeadline(voteTimeLimit, 0)
	event := Event{
		Name:   "missionSubPhase",
		RoomID: roomID,
		Payload: map[string]any{
			"subPhase":   "vote",
			"timeLimit":  voteTimeLimit,
			"deadlineAt": room.PhaseDeadline,
		},
	}
	return cloneRoom(room), []Event{event}, nil
}

// SetDebugMissionResult fixes the outcome of the next resolved mission in a
// solo room. It is intentionally unavailable in player-facing modes.
func (s *Service) SetDebugMissionResult(roomID, playerID string, success bool) (*Room, error) {
	s.store.mu.Lock()
	defer s.store.mu.Unlock()

	room, ok := s.store.rooms[roomID]
	if !ok {
		return nil, errors.New("房间不存在")
	}
	if room.Mode != ModeSolo || !playerInRoom(room, playerID) || isAIPlayer(room, playerID) {
		return nil, errors.New("只有单人调试房间的真人玩家可以设置任务结果")
	}
	room.DebugMissionResult = &success
	return cloneRoom(room), nil
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
	room.AutoTeamVotes = map[string]bool{}
	voteTimeLimit := phaseTimeLimit(room, PhaseTeamVote)
	room.PhaseDeadline = phaseDeadline(voteTimeLimit, 0)
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
			"timeLimit":         voteTimeLimit,
			"deadlineAt":        room.PhaseDeadline,
		},
	}
	return cloneRoom(room), []Event{event}, nil
}

const (
	MissionActionSupport  = "support"
	MissionActionSabotage = "sabotage"
)

func (s *Service) MissionVote(roomID, voterID, action string) (*Room, []Event, error) {
	s.store.mu.Lock()
	defer s.store.mu.Unlock()

	room, ok := s.store.rooms[roomID]
	if !ok {
		return nil, nil, errors.New("房间不存在")
	}
	if room.CurrentPhase != PhaseMission || room.MissionSubPhase != "vote" {
		return nil, nil, errors.New("当前不是任务阶段")
	}
	if !contains(room.ProposedTeam, voterID) {
		return nil, nil, errors.New("只有小队成员可以执行任务")
	}
	room.MissionVotes[voterID] = normalizeMissionAction(room, voterID, action)
	if hasAIPlayers(room) {
		for _, playerID := range room.ProposedTeam {
			if isAIPlayer(room, playerID) {
				room.MissionVotes[playerID] = aiMissionAction(room, playerID)
			}
		}
	}
	if len(room.MissionVotes) < len(room.ProposedTeam) {
		return cloneRoom(room), nil, nil
	}
	if room.DebugMissionResult != nil && room.Mode == ModeSolo {
		for _, playerID := range room.ProposedTeam {
			room.MissionVotes[playerID] = MissionActionSupport
		}
		if !*room.DebugMissionResult && len(room.ProposedTeam) > 0 {
			room.MissionVotes[room.ProposedTeam[0]] = MissionActionSabotage
		}
		room.DebugMissionResult = nil
	}

	success := true
	for _, vote := range room.MissionVotes {
		if vote == MissionActionSabotage {
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

	scenario := MissionScenarioForRound(room.CurrentRound)
	sabotageCount := countSabotage(room.MissionVotes)
	teamNames := playerNames(room, room.ProposedTeam)
	focusPrompts := missionFocusPrompts(room, success, sabotageCount)
	suspicionEvents := missionSuspicionEvents(room, success, sabotageCount)
	room.DiscussionFocus = append([]string(nil), focusPrompts...)
	room.SuspicionEvents = append(room.SuspicionEvents, suspicionEvents...)

	events := []Event{{
		Name:   "missionReveal",
		RoomID: roomID,
		Payload: map[string]any{
			"roundNumber":       room.CurrentRound,
			"explanation":       scenario.Explanation,
			"team":              room.ProposedTeam,
			"teamNames":         teamNames,
			"success":           success,
			"riskActionCount":   sabotageCount,
			"sabotageCount":     sabotageCount,
			"focusPrompts":      focusPrompts,
			"suspicionEvents":   suspicionEvents,
			"missionResults":    room.MissionResults,
			"missionSuccesses":  room.MissionSuccesses,
			"missionFailures":   room.MissionFailures,
			"voteHistory":       publicVoteHistory(room.VoteHistory),
			"nominationHistory": room.NominationHistory,
			"stances":           room.Stances,
			"hadPossession":     room.PossessedPlayer != "",
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
			"explanation":      scenario.Explanation,
			"team":             room.ProposedTeam,
			"teamNames":        teamNames,
			"riskActionCount":  sabotageCount,
			"sabotageCount":    sabotageCount,
			"focusPrompts":     focusPrompts,
			"suspicionEvents":  suspicionEvents,
			"hadPossession":    room.PossessedPlayer != "",
		},
	}}

	if room.MissionSuccesses >= MissionsToWin || room.MissionFailures >= MissionsToWin || room.CurrentRound >= MaxRounds {
		room.Status = StatusFinished
		room.EndedAtMillis = time.Now().UnixMilli()
		winner := "engineer"
		if room.MissionFailures >= MissionsToWin || room.MissionSuccesses < MissionsToWin {
			winner = "infiltrator"
		}
		events = append(events, gameFinishedEvent(roomID, room, winner))
		return cloneRoom(room), events, nil
	}

	room.CurrentRound++
	room.CurrentPhase = PhasePropose
	room.MissionSubPhase = ""
	room.TeamVotes = map[string]bool{}
	room.MissionVotes = map[string]string{}
	room.AutoTeamVotes = map[string]bool{}
	room.AutoMissionVotes = map[string]bool{}
	room.ProposedTeam = []string{}
	room.MessageCount = map[string]int{}
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
		ids = append(ids, player.ID)
	}
	return ids
}

func hasAIPlayers(room *Room) bool {
	for _, player := range room.Players {
		if player.IsAI && !player.Eliminated {
			return true
		}
	}
	return false
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
		if player.Eliminated || player.IsAI || player.Disconnected {
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

func aiTeamVote(room *Room, playerID string) bool {
	if room.Mode == ModeTest || room.Mode == ModeSolo {
		return true
	}
	role := room.Roles[playerID]
	if roleFaction(role) == "evil" {
		return proposedTeamHasFaction(room, "evil") || room.CurrentRound == 1
	}
	if len(room.MissionResults) == 0 || room.MissionResults[len(room.MissionResults)-1] {
		return true
	}
	if len(room.VoteHistory) == 0 {
		return true
	}
	lastTeam := room.VoteHistory[len(room.VoteHistory)-1].Team
	return !teamsOverlap(room.ProposedTeam, lastTeam)
}

func normalizeMissionAction(room *Room, playerID, action string) string {
	if action != MissionActionSabotage {
		return MissionActionSupport
	}
	if roleFaction(room.Roles[playerID]) != "evil" {
		return MissionActionSupport
	}
	return MissionActionSabotage
}

func aiMissionAction(room *Room, playerID string) string {
	if roleFaction(room.Roles[playerID]) == "evil" {
		return MissionActionSabotage
	}
	return MissionActionSupport
}

func gameFinishedEvent(roomID string, room *Room, winner string) Event {
	return Event{
		Name:   "gameFinished",
		RoomID: roomID,
		Payload: map[string]any{
			"winner":            winner,
			"winnerLabel":       winnerLabel(winner),
			"roles":             buildRoleReveal(room, winner),
			"missionResults":    room.MissionResults,
			"missionSuccesses":  room.MissionSuccesses,
			"missionFailures":   room.MissionFailures,
			"voteHistory":       room.VoteHistory,
			"signalHistory":     room.SignalHistory,
			"nominationHistory": room.NominationHistory,
			"stances":           room.Stances,
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

func countSabotage(votes map[string]string) int {
	count := 0
	for _, action := range votes {
		if action == MissionActionSabotage {
			count++
		}
	}
	return count
}

func proposedTeamHasFaction(room *Room, faction string) bool {
	for _, playerID := range room.ProposedTeam {
		if roleFaction(room.Roles[playerID]) == faction {
			return true
		}
	}
	return false
}

func teamsOverlap(a, b []string) bool {
	seen := map[string]bool{}
	for _, item := range a {
		seen[item] = true
	}
	for _, item := range b {
		if seen[item] {
			return true
		}
	}
	return false
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
		prompts = append(prompts, "本轮出现多次破坏，队内可能不止一个渗透者或伪装者在行动。")
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
		"本轮出现 " + itoa(riskActionCount) + " 次破坏，但具体归属保持隐藏。",
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
			"id":           player.ID,
			"name":         player.Name,
			"position":     player.Position,
			"isAI":         player.IsAI,
			"disconnected": player.Disconnected,
		})
	}
	return output
}

func (s *Service) ReconnectEvents(roomID, playerID string) ([]Event, error) {
	s.store.mu.Lock()
	defer s.store.mu.Unlock()

	room, ok := s.store.rooms[roomID]
	if !ok {
		return nil, errors.New("房间不存在")
	}
	role, ok := room.Roles[playerID]
	if !ok {
		return nil, errors.New("玩家身份不存在")
	}
	events := []Event{{
		Name:     "rolesRevealed",
		RoomID:   roomID,
		SocketID: playerID,
		Payload: map[string]any{
			"role":            role,
			"roleLabel":       RoleLabels[role],
			"roleDescription": RoleDescription(role),
			"players":         publicPlayers(room.Players),
			"reconnected":     true,
		},
	}}

	switch room.Status {
	case StatusFinished:
		event := gameFinishedEvent(roomID, room, winningFaction(room))
		event.SocketID = playerID
		return append(events, event), nil
	case StatusPlaying:
		phaseEvents := reconnectPhaseEvents(roomID, playerID, room)
		for i := range phaseEvents {
			phaseEvents[i].SocketID = playerID
		}
		events = append(events, phaseEvents...)
		if stances := room.Stances[room.CurrentRound]; len(stances) > 0 {
			events = append(events, Event{
				Name:     "stanceUpdated",
				RoomID:   roomID,
				SocketID: playerID,
				Payload: map[string]any{
					"roundNumber": room.CurrentRound,
					"stances":     stances,
				},
			})
		}
		if len(room.ChatMessages) > 0 {
			events = append(events, Event{
				Name:     "chatHistory",
				RoomID:   roomID,
				SocketID: playerID,
				Payload: map[string]any{
					"messages": publicChatMessages(room.ChatMessages),
				},
			})
		}
	}
	return events, nil
}

func reconnectPhaseEvents(roomID, playerID string, room *Room) []Event {
	switch room.CurrentPhase {
	case PhasePropose:
		return []Event{reconnectProposeEvent(roomID, room)}
	case PhaseDiscuss:
		return []Event{{
			Name:   "phaseChange",
			RoomID: roomID,
			Payload: map[string]any{
				"phase":             PhaseDiscuss,
				"roundNumber":       room.CurrentRound,
				"totalRounds":       MaxRounds,
				"proposedTeam":      room.ProposedTeam,
				"proposedTeamNames": playerNames(room, room.ProposedTeam),
				"missionResults":    room.MissionResults,
				"maxMessages":       MaxMessagesPerRound,
				"maxChars":          MaxCharsPerMessage,
				"leader":            leaderPayload(room),
				"timeLimit":         phaseTimeLimit(room, PhaseDiscuss),
				"deadlineAt":        room.PhaseDeadline,
				"nominationReason":  room.NominationReason,
			},
		}}
	case PhaseTeamVote:
		return []Event{{
			Name:   "phaseChange",
			RoomID: roomID,
			Payload: map[string]any{
				"phase":             PhaseTeamVote,
				"roundNumber":       room.CurrentRound,
				"totalRounds":       MaxRounds,
				"proposedTeam":      room.ProposedTeam,
				"proposedTeamNames": playerNames(room, room.ProposedTeam),
				"missionResults":    room.MissionResults,
				"leader":            leaderPayload(room),
				"nominationReason":  room.NominationReason,
				"timeLimit":         phaseTimeLimit(room, PhaseTeamVote),
				"deadlineAt":        room.PhaseDeadline,
			},
		}}
	case PhaseMission:
		return reconnectMissionEvents(roomID, playerID, room)
	default:
		return nil
	}
}

func reconnectMissionEvents(roomID, playerID string, room *Room) []Event {
	teamNames := playerNames(room, room.ProposedTeam)
	scenario := MissionScenarioForRound(room.CurrentRound)
	events := []Event{{
		Name:   "phaseChange",
		RoomID: roomID,
		Payload: map[string]any{
			"phase":             PhaseMission,
			"roundNumber":       room.CurrentRound,
			"totalRounds":       MaxRounds,
			"proposedTeam":      room.ProposedTeam,
			"proposedTeamNames": teamNames,
			"missionResults":    room.MissionResults,
			"timeLimit":         missionVoteTimeLimit(room),
			"deadlineAt":        room.PhaseDeadline,
		},
	}}
	if contains(room.ProposedTeam, playerID) {
		events = append(events, Event{
			Name:     "missionPuzzle",
			RoomID:   roomID,
			SocketID: playerID,
			Payload: map[string]any{
				"puzzle": map[string]any{
					"id":       scenario.ID,
					"title":    scenario.Title,
					"scenario": scenario.Scenario,
				},
				"isPossessed": playerID == room.PossessedPlayer,
				"canSabotage": roleFaction(room.Roles[playerID]) == "evil",
				"teamNames":   teamNames,
			},
		})
	} else {
		events = append(events, Event{
			Name:     "missionSpectate",
			RoomID:   roomID,
			SocketID: playerID,
			Payload: map[string]any{
				"teamNames":   teamNames,
				"puzzleTitle": scenario.Title,
			},
		})
	}
	events = append(events, Event{
		Name:     "missionSubPhase",
		RoomID:   roomID,
		SocketID: playerID,
		Payload: map[string]any{
			"subPhase":    room.MissionSubPhase,
			"timeLimit":   missionVoteTimeLimit(room),
			"deadlineAt":  room.PhaseDeadline,
			"maxMessages": MaxMissionMessages,
			"maxChars":    40,
		},
	})
	return events
}

func winningFaction(room *Room) string {
	if room.MissionFailures >= MissionsToWin || room.MissionSuccesses < MissionsToWin {
		return "infiltrator"
	}
	return "engineer"
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
	scenario := MissionScenarioForRound(room.CurrentRound)
	discussTimeLimit := missionDiscussTimeLimit(room)
	room.PhaseDeadline = phaseDeadline(discussTimeLimit, 0)
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
			"timeLimit":         discussTimeLimit,
			"deadlineAt":        room.PhaseDeadline,
		},
	}}
	for _, memberID := range room.ProposedTeam {
		events = append(events, Event{
			Name:     "missionPuzzle",
			RoomID:   roomID,
			SocketID: memberID,
			Payload: map[string]any{
				"puzzle": map[string]any{
					"id":       scenario.ID,
					"title":    scenario.Title,
					"scenario": scenario.Scenario,
				},
				"isPossessed":   memberID == room.PossessedPlayer,
				"possessedHint": possessionHint(memberID == room.PossessedPlayer),
				"canSabotage":   room.Roles[memberID] == RoleInfiltrator,
				"teamNames":     teamNames,
			},
		})
	}
	for _, player := range room.Players {
		if player.IsAI || player.Eliminated || player.Disconnected || contains(room.ProposedTeam, player.ID) {
			continue
		}
		events = append(events, Event{
			Name:     "missionSpectate",
			RoomID:   roomID,
			SocketID: player.ID,
			Payload: map[string]any{
				"teamNames":   teamNames,
				"puzzleTitle": scenario.Title,
			},
		})
	}
	events = append(events, Event{
		Name:   "missionSubPhase",
		RoomID: roomID,
		Payload: map[string]any{
			"subPhase":    "discuss",
			"timeLimit":   discussTimeLimit,
			"deadlineAt":  room.PhaseDeadline,
			"maxMessages": 3,
			"maxChars":    40,
		},
	})
	return events
}

func phaseProposeEvent(roomID string, room *Room) Event {
	timeLimit := phaseTimeLimit(room, PhasePropose)
	room.PhaseDeadline = phaseDeadline(timeLimit, 0)
	return proposePhaseEvent(roomID, room, timeLimit)
}

func reconnectProposeEvent(roomID string, room *Room) Event {
	return proposePhaseEvent(roomID, room, phaseTimeLimit(room, PhasePropose))
}

func proposePhaseEvent(roomID string, room *Room, timeLimit int) Event {
	return Event{
		Name:   "phaseChange",
		RoomID: roomID,
		Payload: map[string]any{
			"phase":          PhasePropose,
			"roundNumber":    room.CurrentRound,
			"totalRounds":    MaxRounds,
			"leader":         leaderPayload(room),
			"teamSize":       MissionTeamSize(len(alivePlayers(room)), room.CurrentRound),
			"missionResults": room.MissionResults,
			"timeLimit":      timeLimit,
			"deadlineAt":     room.PhaseDeadline,
		},
	}
}

func phaseTimeLimit(room *Room, phase Phase) int {
	if room.Mode == ModeTest || room.Mode == ModeSolo {
		switch phase {
		case PhasePropose:
			return 12
		case PhaseDiscuss:
			return 25
		case PhaseTeamVote:
			return 15
		}
	}
	switch phase {
	case PhaseDiscuss:
		return 45
	case PhaseTeamVote:
		return 20
	default:
		return 15
	}
}

func missionDiscussTimeLimit(room *Room) int {
	if room.Mode == ModeTest || room.Mode == ModeSolo {
		return 12
	}
	return 12
}

func missionVoteTimeLimit(room *Room) int {
	if room.Mode == ModeTest || room.Mode == ModeSolo {
		return 12
	}
	return 15
}

func phaseDeadline(timeLimit, graceSeconds int) int64 {
	return time.Now().Add(time.Duration(timeLimit+graceSeconds) * time.Second).UnixMilli()
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
