package game

import (
	"errors"
	"testing"
	"time"
)

var errTestRewrite = errors.New("rewrite failed")

type stubMessageRewriter struct {
	rewritten string
	err       error
	calls     int
	message   string
	style     Style
}

func (s *stubMessageRewriter) Rewrite(message string, style Style) (string, error) {
	s.calls++
	s.message = message
	s.style = style
	return s.rewritten, s.err
}

func TestServiceStartGameAssignsRolesAndBeginsProposePhase(t *testing.T) {
	t.Parallel()

	store := NewStore()
	service := NewService(store)
	store.CreateRoom("room1")
	for i := 0; i < 5; i++ {
		_, err := store.AddPlayer("room1", string(rune('a'+i)), "玩家", ModeNormal)
		if err != nil {
			t.Fatalf("AddPlayer %d returned error: %v", i, err)
		}
	}

	state, events, err := service.StartGame("room1", "a")
	if err != nil {
		t.Fatalf("StartGame returned error: %v", err)
	}
	if state.Status != StatusPlaying {
		t.Fatalf("status = %q, want %q", state.Status, StatusPlaying)
	}
	if state.CurrentPhase != PhasePropose {
		t.Fatalf("phase = %q, want %q", state.CurrentPhase, PhasePropose)
	}
	if len(state.Roles) != 5 {
		t.Fatalf("roles = %d, want 5", len(state.Roles))
	}
	if len(events) < 7 {
		t.Fatalf("events = %d, want role, phase, and signal events", len(events))
	}
	if !hasEvent(events, "phaseChange") || !hasEvent(events, "signalCheck") {
		t.Fatalf("events = %+v, want phaseChange and signalCheck", events)
	}
}

func TestServiceChatRewritesPossessedPlayerAndUsesPlayerName(t *testing.T) {
	t.Parallel()

	store := NewStore()
	service := NewService(store)
	store.CreateRoom("room1")
	for _, player := range []struct {
		id   string
		name string
	}{
		{id: "p1", name: "玩家一"},
		{id: "p2", name: "玩家二"},
		{id: "p3", name: "玩家三"},
		{id: "p4", name: "玩家四"},
		{id: "p5", name: "玩家五"},
	} {
		if _, err := store.AddPlayer("room1", player.id, player.name, ModeNormal); err != nil {
			t.Fatalf("AddPlayer returned error: %v", err)
		}
	}
	if _, _, err := service.StartGame("room1", "p1"); err != nil {
		t.Fatalf("StartGame returned error: %v", err)
	}

	store.mu.Lock()
	store.rooms["room1"].PossessedPlayer = "p1"
	store.rooms["room1"].PossessionStyle = StylePolite
	store.mu.Unlock()

	message, left, err := service.Chat("room1", "p1", "我觉得不行")
	if err != nil {
		t.Fatalf("Chat returned error: %v", err)
	}
	if !message.Possessed {
		t.Fatal("Chat possessed = false, want true")
	}
	if message.PlayerName != "玩家一" {
		t.Fatalf("PlayerName = %q, want 玩家一", message.PlayerName)
	}
	if message.Displayed != "可能我觉得不行" {
		t.Fatalf("Displayed = %q, want rewritten message", message.Displayed)
	}
	if message.Original != "我觉得不行" {
		t.Fatalf("Original = %q, want sanitized original", message.Original)
	}
	if left != MaxMessagesPerRound-1 {
		t.Fatalf("messages left = %d, want %d", left, MaxMessagesPerRound-1)
	}
}

func TestServiceChatUsesExternalRewriterWhenAvailable(t *testing.T) {
	t.Parallel()

	store := NewStore()
	rewriter := &stubMessageRewriter{rewritten: "外部改写结果"}
	service := NewService(store, WithMessageRewriter(rewriter))
	store.CreateRoom("room1")
	for _, player := range []struct {
		id   string
		name string
	}{
		{id: "p1", name: "玩家一"},
		{id: "p2", name: "玩家二"},
		{id: "p3", name: "玩家三"},
		{id: "p4", name: "玩家四"},
		{id: "p5", name: "玩家五"},
	} {
		if _, err := store.AddPlayer("room1", player.id, player.name, ModeNormal); err != nil {
			t.Fatalf("AddPlayer returned error: %v", err)
		}
	}
	if _, _, err := service.StartGame("room1", "p1"); err != nil {
		t.Fatalf("StartGame returned error: %v", err)
	}

	store.mu.Lock()
	store.rooms["room1"].PossessedPlayer = "p1"
	store.rooms["room1"].PossessionStyle = StyleAwkward
	store.mu.Unlock()

	message, _, err := service.Chat("room1", "p1", "我觉得不行")
	if err != nil {
		t.Fatalf("Chat returned error: %v", err)
	}
	if got := message.Displayed; got != "外部改写结果" {
		t.Fatalf("Displayed = %q, want external rewritten message", got)
	}
	if rewriter.calls != 1 {
		t.Fatalf("Rewrite calls = %d, want 1", rewriter.calls)
	}
	if rewriter.message != "我觉得不行" || rewriter.style != StyleAwkward {
		t.Fatalf("rewriter received (%q, %q), want original message and style", rewriter.message, rewriter.style)
	}
}

func TestServiceChatFallsBackWhenExternalRewriterFails(t *testing.T) {
	t.Parallel()

	store := NewStore()
	rewriter := &stubMessageRewriter{err: errTestRewrite}
	service := NewService(store, WithMessageRewriter(rewriter))
	store.CreateRoom("room1")
	for _, player := range []struct {
		id   string
		name string
	}{
		{id: "p1", name: "玩家一"},
		{id: "p2", name: "玩家二"},
		{id: "p3", name: "玩家三"},
		{id: "p4", name: "玩家四"},
		{id: "p5", name: "玩家五"},
	} {
		if _, err := store.AddPlayer("room1", player.id, player.name, ModeNormal); err != nil {
			t.Fatalf("AddPlayer returned error: %v", err)
		}
	}
	if _, _, err := service.StartGame("room1", "p1"); err != nil {
		t.Fatalf("StartGame returned error: %v", err)
	}

	store.mu.Lock()
	store.rooms["room1"].PossessedPlayer = "p1"
	store.rooms["room1"].PossessionStyle = StylePolite
	store.mu.Unlock()

	message, _, err := service.Chat("room1", "p1", "我觉得不行")
	if err != nil {
		t.Fatalf("Chat returned error: %v", err)
	}
	if got := message.Displayed; got != "可能我觉得不行" {
		t.Fatalf("Displayed = %q, want local fallback rewrite", got)
	}
	if rewriter.calls != 1 {
		t.Fatalf("Rewrite calls = %d, want 1", rewriter.calls)
	}
}

func TestServiceProposeMissionMovesToDiscuss(t *testing.T) {
	t.Parallel()

	store := NewStore()
	service := NewService(store)
	store.CreateRoom("room1")
	players := []string{"p1", "p2", "p3", "p4", "p5"}
	for _, id := range players {
		if _, err := store.AddPlayer("room1", id, id, ModeNormal); err != nil {
			t.Fatalf("AddPlayer returned error: %v", err)
		}
	}
	state, _, err := service.StartGame("room1", "p1")
	if err != nil {
		t.Fatalf("StartGame returned error: %v", err)
	}

	teamSize := MissionTeamSize(5, state.CurrentRound)
	next, events, err := service.ProposeMission("room1", state.CurrentLeader, players[:teamSize])
	if err != nil {
		t.Fatalf("ProposeMission returned error: %v", err)
	}
	if next.CurrentPhase != PhaseDiscuss {
		t.Fatalf("phase = %q, want %q", next.CurrentPhase, PhaseDiscuss)
	}
	if len(events) != 2 {
		t.Fatalf("events = %d, want missionProposed and phaseChange", len(events))
	}
}

func TestServiceTeamVoteApprovalStartsMission(t *testing.T) {
	t.Parallel()

	store := NewStore()
	service := NewService(store)
	store.CreateRoom("room1")
	players := []string{"p1", "p2", "p3", "p4", "p5"}
	for _, id := range players {
		if _, err := store.AddPlayer("room1", id, id, ModeNormal); err != nil {
			t.Fatalf("AddPlayer returned error: %v", err)
		}
	}
	state, _, err := service.StartGame("room1", "p1")
	if err != nil {
		t.Fatalf("StartGame returned error: %v", err)
	}
	teamSize := MissionTeamSize(5, state.CurrentRound)
	if _, _, err := service.ProposeMission("room1", state.CurrentLeader, players[:teamSize]); err != nil {
		t.Fatalf("ProposeMission returned error: %v", err)
	}
	if _, _, err := service.TeamVote("room1", players[0], true); err == nil {
		t.Fatal("TeamVote succeeded before the discussion timer advanced the phase")
	}
	if _, _, err := service.StartTeamVote("room1"); err != nil {
		t.Fatalf("StartTeamVote returned error: %v", err)
	}

	var events []Event
	for _, id := range players {
		state, events, err = service.TeamVote("room1", id, true)
		if err != nil {
			t.Fatalf("TeamVote returned error: %v", err)
		}
	}
	if state.CurrentPhase != PhaseMission {
		t.Fatalf("phase = %q, want %q", state.CurrentPhase, PhaseMission)
	}
	if len(events) == 0 || events[0].Name != "teamVoteResult" {
		t.Fatalf("events = %+v, want teamVoteResult first", events)
	}
	var puzzlePayload map[string]any
	spectatorCount := 0
	for _, event := range events {
		if event.Name == "missionSpectate" {
			spectatorCount++
		}
		if event.Name != "missionPuzzle" || puzzlePayload != nil {
			continue
		}
		puzzlePayload = event.Payload.(map[string]any)["puzzle"].(map[string]any)
	}
	if puzzlePayload == nil {
		t.Fatalf("events = %+v, want missionPuzzle payload", events)
	}
	if puzzlePayload["title"] == "" {
		t.Fatalf("puzzle title = %v, want task title", puzzlePayload["title"])
	}
	if containsAny(puzzlePayload["scenario"].(string), "服务器", "日志", "修复") {
		t.Fatalf("puzzle scenario = %q, should avoid engineering knowledge", puzzlePayload["scenario"])
	}
	if _, ok := puzzlePayload["options"]; ok {
		t.Fatalf("puzzle payload exposed options: %+v", puzzlePayload)
	}
	if got, want := spectatorCount, len(players)-teamSize; got != want {
		t.Errorf("missionSpectate event count = %d, want %d", got, want)
	}
}

func TestServiceStartGameRequiresRoomPlayerAndMinimumPlayers(t *testing.T) {
	t.Parallel()

	store := NewStore()
	service := NewService(store)
	store.CreateRoom("room1")
	for _, id := range []string{"p1", "p2", "p3", "p4"} {
		if _, err := store.AddPlayer("room1", id, id, ModeNormal); err != nil {
			t.Fatalf("AddPlayer returned error: %v", err)
		}
	}

	if _, _, err := service.StartGame("room1", "outsider"); err == nil {
		t.Fatalf("StartGame by outsider returned nil error, want rejection")
	}
	if _, _, err := service.StartGame("room1", "p1"); err == nil || err.Error() != "人数不足 5 人时请开启 AI 补位" {
		t.Fatalf("StartGame with four players returned %v, want AI fill error", err)
	}
}

func TestServiceStartGameAllowsThreeHumansWithAIFill(t *testing.T) {
	t.Parallel()

	store := NewStore()
	service := NewService(store)
	store.CreateRoom("room1")
	for _, id := range []string{"host", "p2", "p3"} {
		if _, err := store.AddPlayer("room1", id, id, ModeNormal); err != nil {
			t.Fatalf("AddPlayer returned error: %v", err)
		}
	}
	room, err := store.FillWithAIByHost("room1", "host", MinPlayers)
	if err != nil {
		t.Fatalf("FillWithAIByHost returned error: %v", err)
	}
	if len(room.Players) != MinPlayers {
		t.Fatalf("players = %d, want %d", len(room.Players), MinPlayers)
	}
	state, _, err := service.StartGame("room1", "host")
	if err != nil {
		t.Fatalf("StartGame returned error: %v", err)
	}
	if len(state.Roles) != MinPlayers {
		t.Fatalf("roles = %d, want %d including AI", len(state.Roles), MinPlayers)
	}
}

func TestServiceStartGameRequiresHostInNormalMode(t *testing.T) {
	t.Parallel()

	store := NewStore()
	service := NewService(store)
	store.CreateRoom("room1")
	for _, id := range []string{"host", "p2", "p3", "p4", "p5"} {
		if _, err := store.AddPlayer("room1", id, id, ModeNormal); err != nil {
			t.Fatalf("AddPlayer returned error: %v", err)
		}
	}

	if _, _, err := service.StartGame("room1", "p2"); err == nil || err.Error() != "只有房主可以开始游戏" {
		t.Fatalf("StartGame by non-host returned %v, want host rejection", err)
	}
	if _, _, err := service.StartGame("room1", "host"); err != nil {
		t.Fatalf("StartGame by host returned error: %v", err)
	}
}

func TestServiceTeamVoteTieRejectsTeam(t *testing.T) {
	t.Parallel()

	store := NewStore()
	service := NewService(store)
	store.CreateRoom("room1")
	players := []string{"p1", "p2", "p3", "p4", "p5", "p6"}
	for _, id := range players {
		if _, err := store.AddPlayer("room1", id, id, ModeNormal); err != nil {
			t.Fatalf("AddPlayer returned error: %v", err)
		}
	}
	state, _, err := service.StartGame("room1", "p1")
	if err != nil {
		t.Fatalf("StartGame returned error: %v", err)
	}
	teamSize := MissionTeamSize(len(players), state.CurrentRound)
	if _, _, err := service.ProposeMission("room1", state.CurrentLeader, players[:teamSize]); err != nil {
		t.Fatalf("ProposeMission returned error: %v", err)
	}
	if _, _, err := service.StartTeamVote("room1"); err != nil {
		t.Fatalf("StartTeamVote returned error: %v", err)
	}

	var events []Event
	for i, id := range players {
		state, events, err = service.TeamVote("room1", id, i < len(players)/2)
		if err != nil {
			t.Fatalf("TeamVote returned error: %v", err)
		}
	}

	if state.CurrentPhase != PhasePropose {
		t.Fatalf("phase = %q, want %q after tied vote", state.CurrentPhase, PhasePropose)
	}
	if len(events) == 0 || events[0].Name != "teamVoteResult" {
		t.Fatalf("events = %+v, want teamVoteResult first", events)
	}
	payload := events[0].Payload.(map[string]any)
	if payload["approved"] != false {
		t.Fatalf("approved = %v, want false for tied vote", payload["approved"])
	}
}

func TestServiceFiveRejectedTeamsFinishesForInfiltrators(t *testing.T) {
	t.Parallel()

	store := NewStore()
	service := NewService(store)
	store.CreateRoom("room1")
	players := []string{"p1", "p2", "p3", "p4", "p5"}
	for _, id := range players {
		if _, err := store.AddPlayer("room1", id, id, ModeNormal); err != nil {
			t.Fatalf("AddPlayer returned error: %v", err)
		}
	}
	state, _, err := service.StartGame("room1", "p1")
	if err != nil {
		t.Fatalf("StartGame returned error: %v", err)
	}

	var events []Event
	for rejection := 1; rejection <= 5; rejection++ {
		teamSize := MissionTeamSize(len(players), state.CurrentRound)
		if _, _, err := service.ProposeMission("room1", state.CurrentLeader, players[:teamSize]); err != nil {
			t.Fatalf("ProposeMission rejection %d returned error: %v", rejection, err)
		}
		if _, _, err := service.StartTeamVote("room1"); err != nil {
			t.Fatalf("StartTeamVote rejection %d returned error: %v", rejection, err)
		}
		for _, id := range players {
			state, events, err = service.TeamVote("room1", id, false)
			if err != nil {
				t.Fatalf("TeamVote rejection %d returned error: %v", rejection, err)
			}
		}
	}

	if state.Status != StatusFinished {
		t.Fatalf("status = %q, want %q after five rejections", state.Status, StatusFinished)
	}
	var finish Event
	for _, event := range events {
		if event.Name == "gameFinished" {
			finish = event
			break
		}
	}
	if finish.Name == "" {
		t.Fatalf("events = %+v, want gameFinished after five rejections", events)
	}
	payload := finish.Payload.(map[string]any)
	if payload["winner"] != "infiltrator" {
		t.Fatalf("winner = %v, want infiltrator", payload["winner"])
	}
}

func TestServiceTestModeAssignsAIRolesAndAISabotagesWhenInfiltrator(t *testing.T) {
	t.Parallel()

	store := NewStore()
	service := NewService(store)
	store.CreateRoom("room1")
	if _, err := store.AddPlayer("room1", "human", "真人", ModeTest); err != nil {
		t.Fatalf("AddPlayer returned error: %v", err)
	}
	for range 5 {
		if _, err := store.AddAIPlayer("room1"); err != nil {
			t.Fatalf("AddAIPlayer returned error: %v", err)
		}
	}
	state, _, err := service.StartGame("room1", "human")
	if err != nil {
		t.Fatalf("StartGame returned error: %v", err)
	}
	if len(state.Roles) != len(state.Players) {
		t.Fatalf("roles = %d, want %d including AI players", len(state.Roles), len(state.Players))
	}
	aiInfiltrator := ""
	for _, player := range state.Players {
		if player.IsAI && state.Roles[player.ID] == RoleInfiltrator {
			aiInfiltrator = player.ID
			break
		}
	}
	if aiInfiltrator == "" {
		t.Fatalf("roles = %+v, want at least one infiltrator AI in test mode", state.Roles)
	}

	state.CurrentPhase = PhaseMission
	state.ProposedTeam = []string{"human", aiInfiltrator}
	store.mu.Lock()
	store.rooms["room1"].CurrentPhase = state.CurrentPhase
	store.rooms["room1"].MissionSubPhase = "vote"
	store.rooms["room1"].ProposedTeam = append([]string(nil), state.ProposedTeam...)
	store.mu.Unlock()

	state, events, err := service.MissionVote("room1", "human", MissionActionSupport)
	if err != nil {
		t.Fatalf("MissionVote returned error: %v", err)
	}
	if state.Status != StatusPlaying {
		t.Fatalf("status = %q, want game still playing after one mission", state.Status)
	}
	resultPayload := events[1].Payload.(map[string]any)
	if _, ok := resultPayload["votes"]; ok {
		t.Fatalf("mission result exposed individual votes: %+v", resultPayload["votes"])
	}
	if got := resultPayload["riskActionCount"]; got != 1 {
		t.Fatalf("riskActionCount = %v, want 1", got)
	}
	if len(state.MissionResults) != 1 || state.MissionResults[0] {
		t.Fatalf("mission results = %+v, want failure caused by infiltrator AI", state.MissionResults)
	}
}

func TestServiceStartTeamVoteEmitsPhaseChange(t *testing.T) {
	t.Parallel()

	store := NewStore()
	service := NewService(store)
	store.CreateRoom("room1")
	players := []string{"p1", "p2", "p3", "p4", "p5"}
	for _, id := range players {
		if _, err := store.AddPlayer("room1", id, id, ModeNormal); err != nil {
			t.Fatalf("AddPlayer returned error: %v", err)
		}
	}
	state, _, err := service.StartGame("room1", "p1")
	if err != nil {
		t.Fatalf("StartGame returned error: %v", err)
	}
	teamSize := MissionTeamSize(5, state.CurrentRound)
	if _, _, err := service.ProposeMission("room1", state.CurrentLeader, players[:teamSize]); err != nil {
		t.Fatalf("ProposeMission returned error: %v", err)
	}

	state, events, err := service.StartTeamVote("room1")
	if err != nil {
		t.Fatalf("StartTeamVote returned error: %v", err)
	}
	if state.CurrentPhase != PhaseTeamVote {
		t.Fatalf("phase = %q, want %q", state.CurrentPhase, PhaseTeamVote)
	}
	if len(events) != 1 || events[0].Name != "phaseChange" {
		t.Fatalf("events = %+v, want phaseChange", events)
	}
}

func TestServiceMissionChatAndReconnectEventsStayPrivate(t *testing.T) {
	t.Parallel()

	store := NewStore()
	service := NewService(store)
	store.CreateRoom("room1")
	players := []string{"p1", "p2", "p3", "p4", "p5"}
	for _, id := range players {
		if _, err := store.AddPlayer("room1", id, id, ModeNormal); err != nil {
			t.Fatalf("AddPlayer returned error: %v", err)
		}
	}
	state, _, err := service.StartGame("room1", "p1")
	if err != nil {
		t.Fatalf("StartGame returned error: %v", err)
	}
	team := players[:MissionTeamSize(len(players), state.CurrentRound)]
	if _, _, err = service.ProposeMission("room1", state.CurrentLeader, team); err != nil {
		t.Fatalf("ProposeMission returned error: %v", err)
	}
	if _, _, err = service.StartTeamVote("room1"); err != nil {
		t.Fatalf("StartTeamVote returned error: %v", err)
	}
	for _, id := range players {
		if _, _, err = service.TeamVote("room1", id, true); err != nil {
			t.Fatalf("TeamVote returned error: %v", err)
		}
	}
	message, left, err := service.MissionChat("room1", team[0], "我倾向支持")
	if err != nil || message.Displayed == "" || left != MaxMissionMessages-1 {
		t.Fatalf("MissionChat = %+v, %d, %v", message, left, err)
	}
	if _, _, err = service.MissionChat("room1", players[len(players)-1], "旁观发言"); err == nil {
		t.Fatal("MissionChat allowed a spectator to speak")
	}

	events, err := service.ReconnectEvents("room1", team[0])
	if err != nil {
		t.Fatalf("ReconnectEvents returned error: %v", err)
	}
	if len(events) < 3 {
		t.Fatalf("ReconnectEvents = %+v, want role, phase, and mission state", events)
	}
	for _, event := range events {
		if event.SocketID != team[0] {
			t.Fatalf("event %q targets %q, want %q", event.Name, event.SocketID, team[0])
		}
	}
}

func TestServiceRecordsNominationReasonAndPublicStance(t *testing.T) {
	t.Parallel()

	store := NewStore()
	service := NewService(store)
	store.CreateRoom("room1")
	players := []string{"p1", "p2", "p3", "p4", "p5"}
	for _, id := range players {
		if _, err := store.AddPlayer("room1", id, id, ModeNormal); err != nil {
			t.Fatalf("AddPlayer returned error: %v", err)
		}
	}
	state, _, err := service.StartGame("room1", "p1")
	if err != nil {
		t.Fatalf("StartGame returned error: %v", err)
	}
	team := players[:MissionTeamSize(len(players), state.CurrentRound)]
	state, events, err := service.ProposeMissionWithReason("room1", state.CurrentLeader, team, "他们上轮立场比较一致")
	if err != nil {
		t.Fatalf("ProposeMissionWithReason returned error: %v", err)
	}
	if state.NominationReason == "" || len(state.NominationHistory) != 1 {
		t.Fatalf("nomination state = %+v", state.NominationHistory)
	}
	payload := events[0].Payload.(map[string]any)
	if payload["reason"] != "他们上轮立场比较一致" {
		t.Fatalf("missionProposed reason = %v", payload["reason"])
	}

	state, events, err = service.SubmitStance("room1", "p1", "p2", "p3", "p2解释更连贯，p3跟票太快")
	if err != nil {
		t.Fatalf("SubmitStance returned error: %v", err)
	}
	if len(events) != 1 || events[0].Name != "stanceUpdated" || len(state.Stances[1]) != 1 {
		t.Fatalf("stance state/events = %+v / %+v", state.Stances, events)
	}
	if _, _, err = service.SubmitStance("room1", "p1", "p2", "p2", "相同对象"); err == nil {
		t.Fatal("SubmitStance accepted the same trust and suspect target")
	}
}

func TestServiceReconnectRestoresDiscussionContext(t *testing.T) {
	t.Parallel()

	store := NewStore()
	service := NewService(store)
	store.CreateRoom("reconnect-context")
	players := []string{"p1", "p2", "p3", "p4", "p5"}
	for _, id := range players {
		if _, err := store.AddPlayer("reconnect-context", id, id, ModeNormal); err != nil {
			t.Fatalf("AddPlayer(%q) returned error: %v", id, err)
		}
	}
	state, _, err := service.StartGame("reconnect-context", "p1")
	if err != nil {
		t.Fatalf("StartGame returned error: %v", err)
	}
	team := players[:MissionTeamSize(len(players), state.CurrentRound)]
	if _, _, err := service.ProposeMissionWithReason(
		"reconnect-context",
		state.CurrentLeader,
		team,
		"刷新后仍应看到这个理由",
	); err != nil {
		t.Fatalf("ProposeMissionWithReason returned error: %v", err)
	}
	if _, _, err := service.Chat("reconnect-context", "p1", "刷新前的讨论消息"); err != nil {
		t.Fatalf("Chat returned error: %v", err)
	}

	events, err := service.ReconnectEvents("reconnect-context", "p2")
	if err != nil {
		t.Fatalf("ReconnectEvents returned error: %v", err)
	}
	phaseEvent := findEvent(events, "phaseChange")
	if phaseEvent == nil {
		t.Fatal("ReconnectEvents missing phaseChange")
	}
	payload := phaseEvent.Payload.(map[string]any)
	if payload["leader"] == nil || payload["nominationReason"] != "刷新后仍应看到这个理由" {
		t.Errorf("reconnect phase payload = %+v, want leader and nomination reason", payload)
	}
	if payload["deadlineAt"] == nil {
		t.Errorf("reconnect phase payload = %+v, want server deadline", payload)
	}
	chatEvent := findEvent(events, "chatHistory")
	if chatEvent == nil {
		t.Fatal("ReconnectEvents missing chatHistory")
	}
	chatPayload := chatEvent.Payload.(map[string]any)
	messages := chatPayload["messages"].([]PublicChatMessage)
	if len(messages) != 1 || messages[0].PlayerID != "p1" || messages[0].Displayed == "" {
		t.Errorf("reconnect chat history = %+v, want saved discussion message", messages)
	}
}

func TestServiceMissionVoteSuccessAdvancesRound(t *testing.T) {
	t.Parallel()

	store := NewStore()
	service := NewService(store)
	store.CreateRoom("room1")
	players := []string{"p1", "p2", "p3", "p4", "p5"}
	for _, id := range players {
		if _, err := store.AddPlayer("room1", id, id, ModeNormal); err != nil {
			t.Fatalf("AddPlayer returned error: %v", err)
		}
	}
	state, _, err := service.StartGame("room1", "p1")
	if err != nil {
		t.Fatalf("StartGame returned error: %v", err)
	}
	teamSize := MissionTeamSize(5, state.CurrentRound)
	team := players[:teamSize]
	if _, _, err := service.ProposeMission("room1", state.CurrentLeader, team); err != nil {
		t.Fatalf("ProposeMission returned error: %v", err)
	}
	if _, _, err := service.StartTeamVote("room1"); err != nil {
		t.Fatalf("StartTeamVote returned error: %v", err)
	}
	for _, id := range players {
		if _, _, err := service.TeamVote("room1", id, true); err != nil {
			t.Fatalf("TeamVote returned error: %v", err)
		}
	}
	if _, _, err = service.MissionVote("room1", team[0], MissionActionSupport); err == nil {
		t.Fatal("MissionVote succeeded during the mission discussion sub-phase")
	}
	if _, _, err = service.StartMissionVote("room1"); err != nil {
		t.Fatalf("StartMissionVote returned error: %v", err)
	}

	var events []Event
	for _, id := range team {
		state, events, err = service.MissionVote("room1", id, MissionActionSupport)
		if err != nil {
			t.Fatalf("MissionVote returned error: %v", err)
		}
	}
	if len(state.MissionResults) != 1 || !state.MissionResults[0] {
		t.Fatalf("mission results = %+v, want first success", state.MissionResults)
	}
	if state.CurrentRound != 2 {
		t.Fatalf("round = %d, want 2", state.CurrentRound)
	}
	if len(events) < 2 || events[0].Name != "missionReveal" || events[1].Name != "missionResult" {
		t.Fatalf("events = %+v, want missionReveal then missionResult", events)
	}
	resultPayload := events[1].Payload.(map[string]any)
	if resultPayload["roundNumber"] != 1 {
		t.Fatalf("roundNumber = %v, want 1", resultPayload["roundNumber"])
	}
}

func TestServiceMissionVoteIgnoresSabotageFromGoodPlayer(t *testing.T) {
	t.Parallel()

	store := NewStore()
	service := NewService(store)
	store.CreateRoom("room1")
	players := []string{"p1", "p2", "p3", "p4", "p5"}
	for _, id := range players {
		if _, err := store.AddPlayer("room1", id, id, ModeNormal); err != nil {
			t.Fatalf("AddPlayer returned error: %v", err)
		}
	}
	state, _, err := service.StartGame("room1", "p1")
	if err != nil {
		t.Fatalf("StartGame returned error: %v", err)
	}
	team := players[:2]
	store.mu.Lock()
	room := store.rooms["room1"]
	room.CurrentPhase = PhaseMission
	room.MissionSubPhase = "vote"
	room.ProposedTeam = append([]string(nil), team...)
	room.MissionVotes = map[string]string{}
	room.CurrentLeader = state.CurrentLeader
	for _, id := range team {
		room.Roles[id] = RoleEngineer
	}
	store.mu.Unlock()

	var events []Event
	for _, id := range team {
		state, events, err = service.MissionVote("room1", id, MissionActionSabotage)
		if err != nil {
			t.Fatalf("MissionVote(%q, sabotage) returned error: %v", id, err)
		}
	}
	if len(state.MissionResults) != 1 || !state.MissionResults[0] {
		t.Fatalf("mission results = %+v, want success because good players cannot sabotage", state.MissionResults)
	}
	resultPayload := events[1].Payload.(map[string]any)
	if got := resultPayload["riskActionCount"]; got != 0 {
		t.Fatalf("riskActionCount = %v, want 0", got)
	}
}

func TestServiceMissionVoteFailsWhenInfiltratorSabotages(t *testing.T) {
	t.Parallel()

	store := NewStore()
	service := NewService(store)
	store.CreateRoom("room1")
	players := []string{"p1", "p2", "p3", "p4", "p5"}
	for _, id := range players {
		if _, err := store.AddPlayer("room1", id, id, ModeNormal); err != nil {
			t.Fatalf("AddPlayer returned error: %v", err)
		}
	}
	state, _, err := service.StartGame("room1", "p1")
	if err != nil {
		t.Fatalf("StartGame returned error: %v", err)
	}
	team := players[:2]
	store.mu.Lock()
	room := store.rooms["room1"]
	room.CurrentPhase = PhaseMission
	room.MissionSubPhase = "vote"
	room.ProposedTeam = append([]string(nil), team...)
	room.MissionVotes = map[string]string{}
	room.CurrentLeader = state.CurrentLeader
	room.Roles[team[0]] = RoleEngineer
	room.Roles[team[1]] = RoleInfiltrator
	store.mu.Unlock()

	if _, _, err = service.MissionVote("room1", team[0], MissionActionSupport); err != nil {
		t.Fatalf("MissionVote(%q, support) returned error: %v", team[0], err)
	}
	state, events, err := service.MissionVote("room1", team[1], MissionActionSabotage)
	if err != nil {
		t.Fatalf("MissionVote(%q, sabotage) returned error: %v", team[1], err)
	}
	if len(state.MissionResults) != 1 || state.MissionResults[0] {
		t.Fatalf("mission results = %+v, want failure when infiltrator sabotages", state.MissionResults)
	}
	resultPayload := events[1].Payload.(map[string]any)
	if got := resultPayload["sabotageCount"]; got != 1 {
		t.Fatalf("sabotageCount = %v, want 1", got)
	}
}

func TestServiceSoloModeUsesServerDeadlines(t *testing.T) {
	t.Parallel()

	store := NewStore()
	service := NewService(store)
	store.CreateRoom("solo-deadline")
	if _, err := store.AddPlayer("solo-deadline", "host", "房主", ModeSolo); err != nil {
		t.Fatalf("AddPlayer returned error: %v", err)
	}
	for range 5 {
		if _, err := store.AddAIPlayer("solo-deadline"); err != nil {
			t.Fatalf("AddAIPlayer returned error: %v", err)
		}
	}

	state, events, err := service.StartGame("solo-deadline", "host")
	if err != nil {
		t.Fatalf("StartGame returned error: %v", err)
	}
	if state.PhaseDeadline <= time.Now().UnixMilli() {
		t.Fatalf("StartGame phase deadline = %d, want a future deadline", state.PhaseDeadline)
	}
	phaseEvent := findEvent(events, "phaseChange")
	if phaseEvent == nil {
		t.Fatal("StartGame events missing phaseChange")
	}
	payload := phaseEvent.Payload.(map[string]any)
	if got, want := payload["timeLimit"], 12; got != want {
		t.Errorf("solo propose timeLimit = %v, want %v", got, want)
	}
	if got := payload["deadlineAt"]; got != state.PhaseDeadline {
		t.Errorf("phaseChange deadlineAt = %v, want %d", got, state.PhaseDeadline)
	}
}

func TestAITeamVoteAlwaysAdvancesScriptedModes(t *testing.T) {
	t.Parallel()

	for _, mode := range []Mode{ModeTest, ModeSolo} {
		t.Run(string(mode), func(t *testing.T) {
			room := &Room{
				Mode:           mode,
				Roles:          map[string]Role{"ai": RoleInfiltrator},
				MissionResults: []bool{false},
				VoteHistory:    []VoteRecord{{Team: []string{"other"}}},
				ProposedTeam:   []string{"human"},
			}
			if !aiTeamVote(room, "ai") {
				t.Errorf("aiTeamVote(mode=%q) = false, want true for deterministic scripted flow", mode)
			}
		})
	}
}

func TestServiceAutoTeamVoteIsMarkedInHistory(t *testing.T) {
	t.Parallel()

	store := NewStore()
	service := NewService(store)
	store.CreateRoom("auto-vote")
	players := []string{"p1", "p2", "p3", "p4", "p5"}
	for _, id := range players {
		if _, err := store.AddPlayer("auto-vote", id, id, ModeNormal); err != nil {
			t.Fatalf("AddPlayer(%q) returned error: %v", id, err)
		}
	}
	state, _, err := service.StartGame("auto-vote", "p1")
	if err != nil {
		t.Fatalf("StartGame returned error: %v", err)
	}
	if _, _, err := service.ProposeMission("auto-vote", state.CurrentLeader, players[:2]); err != nil {
		t.Fatalf("ProposeMission returned error: %v", err)
	}
	if _, _, err := service.StartTeamVote("auto-vote"); err != nil {
		t.Fatalf("StartTeamVote returned error: %v", err)
	}
	if _, _, err := service.AutoTeamVote("auto-vote", "p1", false); err != nil {
		t.Fatalf("AutoTeamVote returned error: %v", err)
	}
	for _, id := range players[1:] {
		state, _, err = service.TeamVote("auto-vote", id, true)
		if err != nil {
			t.Fatalf("TeamVote(%q) returned error: %v", id, err)
		}
	}
	if len(state.VoteHistory) != 1 {
		t.Fatalf("vote history length = %d, want 1", len(state.VoteHistory))
	}
	if !state.VoteHistory[0].Votes["p1"].AutoManaged {
		t.Errorf("auto vote = %+v, want autoManaged=true", state.VoteHistory[0].Votes["p1"])
	}
	if state.VoteHistory[0].Votes["p2"].AutoManaged {
		t.Errorf("manual vote = %+v, want autoManaged=false", state.VoteHistory[0].Votes["p2"])
	}
}

func TestServiceSoloDebugCanForceMissionFailure(t *testing.T) {
	t.Parallel()

	store := NewStore()
	service := NewService(store)
	store.CreateRoom("solo-debug")
	if _, err := store.AddPlayer("solo-debug", "host", "房主", ModeSolo); err != nil {
		t.Fatalf("AddPlayer returned error: %v", err)
	}
	for range 4 {
		if _, err := store.AddAIPlayer("solo-debug"); err != nil {
			t.Fatalf("AddAIPlayer returned error: %v", err)
		}
	}
	if _, _, err := service.StartGame("solo-debug", "host"); err != nil {
		t.Fatalf("StartGame returned error: %v", err)
	}

	store.mu.Lock()
	room := store.rooms["solo-debug"]
	room.CurrentPhase = PhaseMission
	room.MissionSubPhase = "vote"
	room.ProposedTeam = []string{room.Players[0].ID, room.Players[1].ID}
	room.MissionVotes = map[string]string{}
	store.mu.Unlock()

	if _, err := service.SetDebugMissionResult("solo-debug", "host", false); err != nil {
		t.Fatalf("SetDebugMissionResult returned error: %v", err)
	}
	state, _, err := service.MissionVote("solo-debug", "host", MissionActionSupport)
	if err != nil {
		t.Fatalf("MissionVote returned error: %v", err)
	}
	if len(state.MissionResults) != 1 || state.MissionResults[0] {
		t.Fatalf("mission results = %+v, want forced failure", state.MissionResults)
	}
	if state.DebugMissionResult != nil {
		t.Errorf("debug mission result = %v, want cleared after resolution", *state.DebugMissionResult)
	}
}

func TestServiceGameFinishedIncludesRoleRevealPayload(t *testing.T) {
	t.Parallel()

	store := NewStore()
	service := NewService(store)
	store.CreateRoom("room1")
	players := []string{"p1", "p2", "p3", "p4", "p5"}
	for _, id := range players {
		if _, err := store.AddPlayer("room1", id, id, ModeNormal); err != nil {
			t.Fatalf("AddPlayer returned error: %v", err)
		}
	}
	state, _, err := service.StartGame("room1", "p1")
	if err != nil {
		t.Fatalf("StartGame returned error: %v", err)
	}
	state.MissionSuccesses = MissionsToWin - 1
	store.mu.Lock()
	store.rooms["room1"].MissionSuccesses = state.MissionSuccesses
	store.mu.Unlock()

	teamSize := MissionTeamSize(5, state.CurrentRound)
	team := players[:teamSize]
	if _, _, err := service.ProposeMission("room1", state.CurrentLeader, team); err != nil {
		t.Fatalf("ProposeMission returned error: %v", err)
	}
	if _, _, err := service.StartTeamVote("room1"); err != nil {
		t.Fatalf("StartTeamVote returned error: %v", err)
	}
	for _, id := range players {
		if _, _, err := service.TeamVote("room1", id, true); err != nil {
			t.Fatalf("TeamVote returned error: %v", err)
		}
	}
	if _, _, err = service.StartMissionVote("room1"); err != nil {
		t.Fatalf("StartMissionVote returned error: %v", err)
	}

	var events []Event
	for _, id := range team {
		_, events, err = service.MissionVote("room1", id, MissionActionSupport)
		if err != nil {
			t.Fatalf("MissionVote returned error: %v", err)
		}
	}

	var finish Event
	for _, event := range events {
		if event.Name == "gameFinished" {
			finish = event
			break
		}
	}
	if finish.Name == "" {
		t.Fatalf("events = %+v, want gameFinished", events)
	}
	payload := finish.Payload.(map[string]any)
	roles := payload["roles"].(map[string]RoleReveal)
	if len(roles) != len(players) {
		t.Fatalf("role reveal count = %d, want %d", len(roles), len(players))
	}
	for id, reveal := range roles {
		if reveal.Name == "" || reveal.RoleLabel == "" || reveal.Faction == "" {
			t.Fatalf("roles[%s] = %+v, want complete reveal payload", id, reveal)
		}
	}
	if nominations := payload["nominationHistory"].([]NominationRecord); len(nominations) != 1 {
		t.Errorf("gameFinished nomination history length = %d, want 1", len(nominations))
	}
	if votes := payload["voteHistory"].([]VoteRecord); len(votes) != 1 {
		t.Errorf("gameFinished vote history length = %d, want 1", len(votes))
	}
}

func hasEvent(events []Event, name string) bool {
	for _, event := range events {
		if event.Name == name {
			return true
		}
	}
	return false
}

func findEvent(events []Event, name string) *Event {
	for i := range events {
		if events[i].Name == name {
			return &events[i]
		}
	}
	return nil
}
