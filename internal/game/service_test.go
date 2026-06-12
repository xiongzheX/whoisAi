package game

import (
	"errors"
	"testing"
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
	for _, event := range events {
		if event.Name != "missionPuzzle" {
			continue
		}
		puzzlePayload = event.Payload.(map[string]any)["puzzle"].(map[string]any)
		break
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
	if _, _, err := service.StartGame("room1", "p1"); err == nil || err.Error() != "至少需要 5 名玩家才能开始" {
		t.Fatalf("StartGame with four players returned %v, want minimum player error", err)
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
	for _, id := range players {
		if _, _, err := service.TeamVote("room1", id, true); err != nil {
			t.Fatalf("TeamVote returned error: %v", err)
		}
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
	for _, id := range players {
		if _, _, err := service.TeamVote("room1", id, true); err != nil {
			t.Fatalf("TeamVote returned error: %v", err)
		}
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
}

func hasEvent(events []Event, name string) bool {
	for _, event := range events {
		if event.Name == name {
			return true
		}
	}
	return false
}
