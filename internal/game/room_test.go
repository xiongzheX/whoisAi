package game

import (
	"strings"
	"testing"
	"time"
)

func TestStoreAddPlayerAndAI(t *testing.T) {
	t.Parallel()

	store := NewStore()
	room := store.CreateRoom("room1")
	if room.Status != StatusWaiting {
		t.Fatalf("new room status = %q, want %q", room.Status, StatusWaiting)
	}

	player, err := store.AddPlayer("room1", "socket-1", "玩家1", ModeSolo)
	if err != nil {
		t.Fatalf("AddPlayer returned error: %v", err)
	}
	if player.Position != 0 || player.IsAI {
		t.Fatalf("player = %+v, want human at position 0", player)
	}

	ai, err := store.AddAIPlayer("room1")
	if err != nil {
		t.Fatalf("AddAIPlayer returned error: %v", err)
	}
	if !ai.IsAI || ai.Position != 1 {
		t.Fatalf("ai = %+v, want AI at position 1", ai)
	}
}

func TestStoreSetDebugPausedByHost(t *testing.T) {
	t.Parallel()

	store := NewStore()
	store.CreateRoom("pause-room")
	if _, err := store.AddPlayer("pause-room", "host", "房主", ModeSolo); err != nil {
		t.Fatalf("AddPlayer() error: %v", err)
	}
	store.mu.Lock()
	store.rooms["pause-room"].Status = StatusPlaying
	store.rooms["pause-room"].PhaseDeadline = time.Now().Add(5 * time.Second).UnixMilli()
	store.mu.Unlock()

	pausedRoom, remainingSeconds, err := store.SetDebugPausedByHost("pause-room", "host", true)
	if err != nil {
		t.Fatalf("SetDebugPausedByHost(paused=true) error: %v", err)
	}
	if !pausedRoom.DebugPaused || pausedRoom.PhaseDeadline != 0 {
		t.Errorf("paused room = %+v, want paused with cleared deadline", pausedRoom)
	}
	if remainingSeconds < 4 || remainingSeconds > 5 {
		t.Errorf("remainingSeconds = %d, want 4..5", remainingSeconds)
	}
	store.mu.Lock()
	store.rooms["pause-room"].PhaseDeadline = time.Now().Add(12 * time.Second).UnixMilli()
	store.mu.Unlock()
	refreshedRoom, err := store.RefreshDebugPauseByHost("pause-room", "host")
	if err != nil {
		t.Fatalf("RefreshDebugPauseByHost() error: %v", err)
	}
	if refreshedRoom.PhaseDeadline != 0 || refreshedRoom.DebugRemainingMS < 11000 {
		t.Errorf("refreshed room = %+v, want paused with about 12 seconds remaining", refreshedRoom)
	}

	resumedRoom, _, err := store.SetDebugPausedByHost("pause-room", "host", false)
	if err != nil {
		t.Fatalf("SetDebugPausedByHost(paused=false) error: %v", err)
	}
	if resumedRoom.DebugPaused || resumedRoom.PhaseDeadline <= time.Now().UnixMilli() {
		t.Errorf("resumed room = %+v, want running with a future deadline", resumedRoom)
	}
}

func TestStoreSetDebugPausedRejectsNonHost(t *testing.T) {
	t.Parallel()

	store := NewStore()
	store.CreateRoom("pause-auth")
	if _, err := store.AddPlayer("pause-auth", "host", "房主", ModeSolo); err != nil {
		t.Fatalf("AddPlayer(host) error: %v", err)
	}
	if _, err := store.AddPlayer("pause-auth", "guest", "访客", ModeSolo); err != nil {
		t.Fatalf("AddPlayer(guest) error: %v", err)
	}
	store.mu.Lock()
	store.rooms["pause-auth"].Status = StatusPlaying
	store.mu.Unlock()

	if _, _, err := store.SetDebugPausedByHost("pause-auth", "guest", true); err == nil {
		t.Fatal("SetDebugPausedByHost() succeeded for a non-host player")
	}
}

func TestStoreAddPlayerKeepsRoomModeLocked(t *testing.T) {
	t.Parallel()

	store := NewStore()
	store.CreateRoom("party")
	if _, err := store.AddPlayer("party", "socket-1", "房主", ModeNormal); err != nil {
		t.Fatalf("AddPlayer host returned error: %v", err)
	}
	if _, err := store.AddPlayer("party", "socket-2", "测试员", ModeTest); err == nil {
		t.Fatal("AddPlayer allowed a later player to change room mode")
	}

	room, ok := store.Room("party")
	if !ok {
		t.Fatal("room not found")
	}
	if room.Mode != ModeNormal {
		t.Fatalf("room mode = %q, want %q", room.Mode, ModeNormal)
	}
}

func TestStoreLeavePlayerAllowsFinishedRoom(t *testing.T) {
	t.Parallel()

	store := NewStore()
	store.CreateRoom("finished-room")
	if _, err := store.AddPlayer("finished-room", "host", "房主", ModeSolo); err != nil {
		t.Fatalf("AddPlayer() error: %v", err)
	}
	store.mu.Lock()
	store.rooms["finished-room"].Status = StatusFinished
	store.mu.Unlock()

	room, err := store.LeavePlayer("finished-room", "host")
	if err != nil {
		t.Fatalf("LeavePlayer() error: %v", err)
	}
	if room != nil {
		t.Fatalf("LeavePlayer() room = %+v, want nil after final player leaves", room)
	}
	if _, exists := store.Room("finished-room"); exists {
		t.Fatal("finished room still exists after final player leaves")
	}
}

func TestStoreLeavePlayerRejectsPlayingRoom(t *testing.T) {
	t.Parallel()

	store := NewStore()
	store.CreateRoom("playing-room")
	if _, err := store.AddPlayer("playing-room", "host", "房主", ModeSolo); err != nil {
		t.Fatalf("AddPlayer() error: %v", err)
	}
	store.mu.Lock()
	store.rooms["playing-room"].Status = StatusPlaying
	store.mu.Unlock()

	if _, err := store.LeavePlayer("playing-room", "host"); err == nil {
		t.Fatal("LeavePlayer() succeeded during an active game")
	}
}

func TestStoreJoinPlayerRestoresSeatAndTransfersHost(t *testing.T) {
	t.Parallel()

	store := NewStore()
	store.CreateRoom("party")
	if _, _, err := store.JoinPlayer("party", "player_000000000001", "房主", ModeNormal); err != nil {
		t.Fatalf("JoinPlayer host returned error: %v", err)
	}
	if _, _, err := store.JoinPlayer("party", "player_000000000002", "队友", ModeNormal); err != nil {
		t.Fatalf("JoinPlayer teammate returned error: %v", err)
	}
	if _, err := store.SetPlayerDisconnected("party", "player_000000000001", true); err != nil {
		t.Fatalf("SetPlayerDisconnected returned error: %v", err)
	}
	if _, err := store.ResetRoomByHost("party", "player_000000000001"); err == nil {
		t.Fatal("offline original host could still reset room")
	}
	if _, err := store.ResetRoomByHost("party", "player_000000000002"); err != nil {
		t.Fatalf("connected successor could not reset room: %v", err)
	}

	store.CreateRoom("restore")
	const playerID = "player_000000000003"
	if _, _, err := store.JoinPlayer("restore", playerID, "玩家", ModeNormal); err != nil {
		t.Fatalf("JoinPlayer returned error: %v", err)
	}
	if _, err := store.SetPlayerDisconnected("restore", playerID, true); err != nil {
		t.Fatalf("SetPlayerDisconnected returned error: %v", err)
	}
	player, restored, err := store.JoinPlayer("restore", playerID, "新名字", ModeNormal)
	if err != nil || !restored {
		t.Fatalf("JoinPlayer restore = %+v, %v, %v", player, restored, err)
	}
	if player.Disconnected || player.Position != 0 {
		t.Fatalf("restored player = %+v, want original online seat", player)
	}
}

func TestStorePrepareRematchKeepsConnectedHumans(t *testing.T) {
	t.Parallel()

	store := NewStore()
	store.CreateRoom("party")
	for _, id := range []string{"host", "p2", "p3"} {
		if _, err := store.AddPlayer("party", id, id, ModeNormal); err != nil {
			t.Fatalf("AddPlayer returned error: %v", err)
		}
	}
	if _, err := store.AddAIPlayer("party"); err != nil {
		t.Fatalf("AddAIPlayer returned error: %v", err)
	}
	if _, err := store.SetPlayerDisconnected("party", "p3", true); err != nil {
		t.Fatalf("SetPlayerDisconnected returned error: %v", err)
	}
	room, err := store.PrepareRematchByHost("party", "host")
	if err != nil {
		t.Fatalf("PrepareRematchByHost returned error: %v", err)
	}
	if room.Status != StatusWaiting || len(room.Players) != 2 {
		t.Fatalf("rematch room = %+v, want two connected humans waiting", room)
	}
	for _, player := range room.Players {
		if player.IsAI || player.Disconnected {
			t.Fatalf("rematch kept unavailable player: %+v", player)
		}
	}
}

func TestStoreRoomReturnsDeepCopyOfEvidence(t *testing.T) {
	t.Parallel()

	store := NewStore()
	store.CreateRoom("evidence-copy")
	store.mu.Lock()
	store.rooms["evidence-copy"].VoteHistory = []VoteRecord{{
		Round: 1,
		Votes: map[string]PlayerVote{"p1": {VoterName: "玩家1", Approved: true}},
		Team:  []string{"p1", "p2"},
	}}
	store.rooms["evidence-copy"].NominationHistory = []NominationRecord{{
		Round: 1, Team: []string{"p1", "p2"}, TeamNames: []string{"玩家1", "玩家2"},
	}}
	store.mu.Unlock()

	first, ok := store.Room("evidence-copy")
	if !ok {
		t.Fatal("Room(evidence-copy) missing")
	}
	first.VoteHistory[0].Votes["p1"] = PlayerVote{VoterName: "被修改", Approved: false}
	first.VoteHistory[0].Team[0] = "changed"
	first.NominationHistory[0].TeamNames[0] = "被修改"

	second, _ := store.Room("evidence-copy")
	if got, want := second.VoteHistory[0].Votes["p1"].VoterName, "玩家1"; got != want {
		t.Errorf("Room() vote map leaked mutation: got %q, want %q", got, want)
	}
	if got, want := second.VoteHistory[0].Team[0], "p1"; got != want {
		t.Errorf("Room() vote team leaked mutation: got %q, want %q", got, want)
	}
	if got, want := second.NominationHistory[0].TeamNames[0], "玩家1"; got != want {
		t.Errorf("Room() nomination names leaked mutation: got %q, want %q", got, want)
	}
}

func TestStoreLeavePlayerReleasesWaitingSeat(t *testing.T) {
	t.Parallel()

	store := NewStore()
	store.CreateRoom("leave-room")
	if _, _, err := store.JoinPlayer("leave-room", "host", "房主", ModeNormal); err != nil {
		t.Fatalf("JoinPlayer(host) error = %v", err)
	}
	if _, _, err := store.JoinPlayer("leave-room", "guest", "队友", ModeNormal); err != nil {
		t.Fatalf("JoinPlayer(guest) error = %v", err)
	}
	room, err := store.LeavePlayer("leave-room", "host")
	if err != nil {
		t.Fatalf("LeavePlayer(host) error = %v", err)
	}
	if got, want := len(room.Players), 1; got != want {
		t.Fatalf("LeavePlayer(host) player count = %d, want %d", got, want)
	}
	if got, want := room.Players[0].Position, 0; got != want {
		t.Errorf("remaining player position = %d, want %d", got, want)
	}
	if _, err := store.LeavePlayer("leave-room", "guest"); err != nil {
		t.Fatalf("LeavePlayer(last player) error = %v", err)
	}
	if _, exists := store.Room("leave-room"); exists {
		t.Error("empty room still exists after last player left")
	}
}

func TestAssignRolesForSixHumans(t *testing.T) {
	t.Parallel()

	roles := AssignRoles([]string{"p1", "p2", "p3", "p4", "p5", "p6"})
	if len(roles) != 6 {
		t.Fatalf("AssignRoles returned %d roles, want 6", len(roles))
	}

	counts := map[Role]int{}
	for _, role := range roles {
		counts[role]++
	}

	if counts[RoleEngineer] != 3 {
		t.Fatalf("engineers = %d, want 3", counts[RoleEngineer])
	}
	if counts[RoleInfiltrator] != 1 {
		t.Fatalf("infiltrators = %d, want 1", counts[RoleInfiltrator])
	}
	if counts[RoleSignalKeeper] != 1 {
		t.Fatalf("signal keepers = %d, want 1", counts[RoleSignalKeeper])
	}
	if counts[RoleObserver] != 1 {
		t.Fatalf("observers = %d, want 1", counts[RoleObserver])
	}
}

func TestRoleLabelsUseAccessibleSocialDeductionTheme(t *testing.T) {
	t.Parallel()

	if RoleLabels[RoleEngineer] != "🛡️ 守护者" {
		t.Fatalf("engineer label = %q, want 守护者 label", RoleLabels[RoleEngineer])
	}
	if RoleLabels[RoleSignalKeeper] != "📡 侦测者" {
		t.Fatalf("signal keeper label = %q, want 侦测者 label", RoleLabels[RoleSignalKeeper])
	}

	description := RoleDescription(RoleEngineer)
	if description == "" || containsAny(description, "工程师", "系统维护", "修复") {
		t.Fatalf("guardian description = %q, should avoid engineering theme", description)
	}
}

func containsAny(value string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}
