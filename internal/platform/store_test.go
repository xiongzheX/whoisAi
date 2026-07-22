package platform

import (
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestMemoryStoreRoomAndReconnectLifecycle(t *testing.T) {
	t.Parallel()

	store := NewMemoryStore(DefaultRegistry())
	room, err := store.CreateRoom(CreateRoomInput{
		Code:     "party",
		HostID:   "host",
		HostName: "房主",
		GameID:   "who-is-ai",
	})
	if err != nil {
		t.Fatalf("CreateRoom() error = %v", err)
	}
	if got, want := room.HostMemberID, "host"; got != want {
		t.Errorf("CreateRoom().HostMemberID = %q, want %q", got, want)
	}

	joined, reconnected, err := store.JoinRoom("party", "player", "玩家")
	if err != nil {
		t.Fatalf("JoinRoom(new player) error = %v", err)
	}
	if reconnected {
		t.Error("JoinRoom(new player) reconnected = true, want false")
	}
	if got, want := len(joined.Members), 2; got != want {
		t.Errorf("JoinRoom(new player) member count = %d, want %d", got, want)
	}

	if _, err := store.SetConnection("party", "player", MemberOffline); err != nil {
		t.Fatalf("SetConnection(offline) error = %v", err)
	}
	rejoined, reconnected, err := store.JoinRoom("party", "player", "新名字")
	if err != nil {
		t.Fatalf("JoinRoom(reconnect) error = %v", err)
	}
	if !reconnected {
		t.Error("JoinRoom(reconnect) reconnected = false, want true")
	}
	if got, want := rejoined.Members[1].DisplayName, "新名字"; got != want {
		t.Errorf("JoinRoom(reconnect) name = %q, want %q", got, want)
	}
}

func TestMemoryStoreEnforcesCatalogPlayerLimit(t *testing.T) {
	t.Parallel()

	store := NewMemoryStore(DefaultRegistry())
	if _, err := store.CreateRoom(CreateRoomInput{
		Code: "duel", HostID: "p1", HostName: "玩家1", GameID: "bean-sprint",
	}); err != nil {
		t.Fatalf("CreateRoom(bean-sprint) error = %v", err)
	}
	if _, _, err := store.JoinRoom("duel", "p2", "玩家2"); err != nil {
		t.Fatalf("JoinRoom(p2) error = %v", err)
	}
	if _, _, err := store.JoinRoom("duel", "p3", "玩家3"); !errors.Is(err, ErrRoomFull) {
		t.Errorf("JoinRoom(p3) error = %v, want %v", err, ErrRoomFull)
	}
}

func TestMemoryStoreGeneratedRoomAndLeaveReleaseSeat(t *testing.T) {
	t.Parallel()

	store := NewMemoryStore(DefaultRegistry())
	room, err := store.CreateGeneratedRoom("host", "房主", "bean-sprint")
	if err != nil {
		t.Fatalf("CreateGeneratedRoom() error = %v", err)
	}
	if room.Code == "" || room.Code == "race1" {
		t.Fatalf("CreateGeneratedRoom().Code = %q, want generated code", room.Code)
	}
	if _, _, err := store.JoinRoom(room.Code, "guest", "队友"); err != nil {
		t.Fatalf("JoinRoom(guest) error = %v", err)
	}
	remaining, err := store.LeaveRoom(room.Code, "host")
	if err != nil {
		t.Fatalf("LeaveRoom(host) error = %v", err)
	}
	if got, want := remaining.HostMemberID, "guest"; got != want {
		t.Errorf("LeaveRoom(host).HostMemberID = %q, want %q", got, want)
	}
	if got, want := remaining.Members[0].Role, MemberHost; got != want {
		t.Errorf("LeaveRoom(host).Members[0].Role = %q, want %q", got, want)
	}
	if _, err := store.LeaveRoom(room.Code, "guest"); err != nil {
		t.Fatalf("LeaveRoom(last member) error = %v", err)
	}
	if _, exists := store.RoomByCode(room.Code); exists {
		t.Error("RoomByCode() found empty room after last member left")
	}
}

func TestMemoryStoreExpiresOfflineSeat(t *testing.T) {
	t.Parallel()

	store := NewMemoryStore(DefaultRegistry())
	room, err := store.CreateRoom(CreateRoomInput{
		Code: "stale", HostID: "host", HostName: "房主", GameID: "bean-sprint",
	})
	if err != nil {
		t.Fatalf("CreateRoom() error = %v", err)
	}
	if _, _, err := store.JoinRoom(room.Code, "guest", "队友"); err != nil {
		t.Fatalf("JoinRoom() error = %v", err)
	}
	if _, err := store.SetConnection(room.Code, "guest", MemberOffline); err != nil {
		t.Fatalf("SetConnection() error = %v", err)
	}
	if got := store.ExpireOfflineMembers(time.Now().UTC().Add(2*time.Minute), OfflineSeatGrace); got != 1 {
		t.Fatalf("ExpireOfflineMembers() = %d, want 1", got)
	}
	updated, _ := store.RoomByCode(room.Code)
	if got, want := len(updated.Members), 1; got != want {
		t.Errorf("expired room member count = %d, want %d", got, want)
	}
	if _, _, err := store.JoinRoom(room.Code, "replacement", "新队友"); err != nil {
		t.Fatalf("JoinRoom(replacement) error = %v", err)
	}
}

func TestMemoryStoreListsOnlyJoinableWaitingRooms(t *testing.T) {
	t.Parallel()

	store := NewMemoryStore(DefaultRegistry())
	if _, err := store.CreateRoom(CreateRoomInput{
		Code: "joinable", HostID: "host-1", HostName: "小蓝", GameID: "bean-sprint",
	}); err != nil {
		t.Fatalf("CreateRoom(joinable) error = %v", err)
	}
	if _, err := store.CreateRoom(CreateRoomInput{
		Code: "full", HostID: "host-2", HostName: "小绿", GameID: "bean-sprint",
	}); err != nil {
		t.Fatalf("CreateRoom(full) error = %v", err)
	}
	if _, _, err := store.JoinRoom("full", "player-2", "小红"); err != nil {
		t.Fatalf("JoinRoom(full) error = %v", err)
	}
	if _, err := store.CreateRoom(CreateRoomInput{
		Code: "offline", HostID: "host-3", HostName: "小灰", GameID: "bean-sprint",
	}); err != nil {
		t.Fatalf("CreateRoom(offline) error = %v", err)
	}
	if _, err := store.SetConnection("offline", "host-3", MemberOffline); err != nil {
		t.Fatalf("SetConnection(offline) error = %v", err)
	}
	if _, err := store.CreateRoom(CreateRoomInput{
		Code: "sumo", HostID: "host-4", HostName: "团子", GameID: "dumpling-sumo",
	}); err != nil {
		t.Fatalf("CreateRoom(sumo) error = %v", err)
	}

	rooms, err := store.WaitingRooms("bean-sprint")
	if err != nil {
		t.Fatalf("WaitingRooms(bean-sprint) error = %v", err)
	}
	if got, want := len(rooms), 1; got != want {
		t.Fatalf("len(WaitingRooms(bean-sprint)) = %d, want %d: %+v", got, want, rooms)
	}
	room := rooms[0]
	if got, want := room.Code, "joinable"; got != want {
		t.Errorf("WaitingRooms(bean-sprint)[0].Code = %q, want %q", got, want)
	}
	if got, want := room.HostName, "小蓝"; got != want {
		t.Errorf("WaitingRooms(bean-sprint)[0].HostName = %q, want %q", got, want)
	}
	if got, want := room.PlayerCount, 1; got != want {
		t.Errorf("WaitingRooms(bean-sprint)[0].PlayerCount = %d, want %d", got, want)
	}
	if got, want := room.OpenSeats, 1; got != want {
		t.Errorf("WaitingRooms(bean-sprint)[0].OpenSeats = %d, want %d", got, want)
	}
	if _, err := store.WaitingRooms("missing"); !errors.Is(err, ErrGameNotFound) {
		t.Errorf("WaitingRooms(missing) error = %v, want %v", err, ErrGameNotFound)
	}
}

func TestMemoryStoreRejectsSwitchToGameBelowCurrentRoomCapacity(t *testing.T) {
	t.Parallel()

	store := NewMemoryStore(DefaultRegistry())
	if _, err := store.CreateRoom(CreateRoomInput{
		Code: "party-switch", HostID: "p1", HostName: "玩家1", GameID: "who-is-ai",
	}); err != nil {
		t.Fatalf("CreateRoom(who-is-ai) error = %v", err)
	}
	for _, playerID := range []string{"p2", "p3"} {
		if _, _, err := store.JoinRoom("party-switch", playerID, playerID); err != nil {
			t.Fatalf("JoinRoom(%q) error = %v", playerID, err)
		}
	}
	if _, err := store.SelectGame("party-switch", "p1", "bean-sprint"); err == nil {
		t.Error("SelectGame(bean-sprint) with 3 members succeeded, want capacity error")
	}
}

func TestMemoryStoreCreatesIsolatedSessions(t *testing.T) {
	t.Parallel()

	store := NewMemoryStore(DefaultRegistry())
	createTestRoom(t, store)
	participants := []ParticipantInput{
		{ParticipantKey: "host", MemberID: "host", DisplayName: "房主", Kind: ParticipantHuman, Seat: 0},
		{ParticipantKey: "p2", MemberID: "p2", DisplayName: "玩家2", Kind: ParticipantHuman, Seat: 1},
		{ParticipantKey: "p3", MemberID: "p3", DisplayName: "玩家3", Kind: ParticipantHuman, Seat: 2},
		{ParticipantKey: "bot1", DisplayName: "AI 1", Kind: ParticipantBot, Seat: 3},
		{ParticipantKey: "bot2", DisplayName: "AI 2", Kind: ParticipantBot, Seat: 4},
	}

	first, err := store.StartSession(StartSessionInput{
		RoomCode: "party", HostID: "host", Mode: "normal", Participants: participants,
	})
	if err != nil {
		t.Fatalf("StartSession(first) error = %v", err)
	}
	if _, err := store.StartSession(StartSessionInput{
		RoomCode: "party", HostID: "host", Mode: "normal", Participants: participants,
	}); !errors.Is(err, ErrActiveSession) {
		t.Fatalf("StartSession(while active) error = %v, want %v", err, ErrActiveSession)
	}
	if _, err := store.FinishSession("party", json.RawMessage(`{"winner":"engineer"}`)); err != nil {
		t.Fatalf("FinishSession() error = %v", err)
	}

	second, err := store.StartSession(StartSessionInput{
		RoomCode: "party", HostID: "host", Mode: "normal", Participants: participants,
	})
	if err != nil {
		t.Fatalf("StartSession(second) error = %v", err)
	}
	if second.ID == first.ID {
		t.Errorf("second session ID = first session ID %q, want isolated IDs", first.ID)
	}
	if got, want := second.Sequence, first.Sequence+1; got != want {
		t.Errorf("second.Sequence = %d, want %d", got, want)
	}
}

func TestMemoryStoreSnapshotVersionAndPrivacy(t *testing.T) {
	t.Parallel()

	store := NewMemoryStore(DefaultRegistry())
	createTestRoom(t, store)
	session, err := store.StartSession(StartSessionInput{
		RoomCode: "party",
		HostID:   "host",
		Mode:     "test",
		Participants: []ParticipantInput{
			{ParticipantKey: "host", MemberID: "host", DisplayName: "房主", Kind: ParticipantHuman, Seat: 0},
			{ParticipantKey: "bot1", DisplayName: "AI 1", Kind: ParticipantBot, Seat: 1},
			{ParticipantKey: "bot2", DisplayName: "AI 2", Kind: ParticipantBot, Seat: 2},
			{ParticipantKey: "bot3", DisplayName: "AI 3", Kind: ParticipantBot, Seat: 3},
			{ParticipantKey: "bot4", DisplayName: "AI 4", Kind: ParticipantBot, Seat: 4},
		},
	})
	if err != nil {
		t.Fatalf("StartSession() error = %v", err)
	}

	publicState := json.RawMessage(`{"phase":"propose"}`)
	privateState := map[string]json.RawMessage{"host": json.RawMessage(`{"role":"engineer"}`)}
	snapshot, err := store.CommitSnapshot(session.ID, 0, publicState, privateState)
	if err != nil {
		t.Fatalf("CommitSnapshot(version 0) error = %v", err)
	}
	if got, want := snapshot.Version, uint64(1); got != want {
		t.Errorf("CommitSnapshot(version 0).Version = %d, want %d", got, want)
	}
	if _, err := store.CommitSnapshot(session.ID, 0, publicState, nil); !errors.Is(err, ErrVersionConflict) {
		t.Errorf("CommitSnapshot(stale version) error = %v, want %v", err, ErrVersionConflict)
	}

	forOtherPlayer, ok := store.LatestSnapshot(session.ID, "bot1")
	if !ok {
		t.Fatal("LatestSnapshot() missing committed snapshot")
	}
	if len(forOtherPlayer.PrivateState) != 0 {
		t.Errorf("LatestSnapshot(bot1).PrivateState = %v, want no host secret", forOtherPlayer.PrivateState)
	}
	forHost, _ := store.LatestSnapshot(session.ID, "host")
	if got := string(forHost.PrivateState["host"]); got != `{"role":"engineer"}` {
		t.Errorf("LatestSnapshot(host) private state = %s, want host role", got)
	}
}

func TestMemoryStoreRejectsDuplicateAction(t *testing.T) {
	t.Parallel()

	store := NewMemoryStore(DefaultRegistry())
	createTestRoom(t, store)
	session, err := store.StartSession(StartSessionInput{
		RoomCode: "party", HostID: "host", Mode: "normal",
		Participants: []ParticipantInput{
			{ParticipantKey: "host", MemberID: "host", DisplayName: "房主", Kind: ParticipantHuman, Seat: 0},
			{ParticipantKey: "p2", MemberID: "p2", DisplayName: "玩家2", Kind: ParticipantHuman, Seat: 1},
			{ParticipantKey: "p3", MemberID: "p3", DisplayName: "玩家3", Kind: ParticipantHuman, Seat: 2},
			{ParticipantKey: "bot1", DisplayName: "AI 1", Kind: ParticipantBot, Seat: 3},
			{ParticipantKey: "bot2", DisplayName: "AI 2", Kind: ParticipantBot, Seat: 4},
		},
	})
	if err != nil {
		t.Fatalf("StartSession() error = %v", err)
	}
	event := EventRecord{
		SessionID: session.ID, ActorParticipantID: "host", ActionID: "action-1",
		Type: "team_vote", Scope: EventPublic, Payload: json.RawMessage(`{"approve":true}`),
	}
	if _, err := store.AppendEvent(event); err != nil {
		t.Fatalf("AppendEvent(first) error = %v", err)
	}
	if _, err := store.AppendEvent(event); !errors.Is(err, ErrDuplicateAction) {
		t.Errorf("AppendEvent(duplicate) error = %v, want %v", err, ErrDuplicateAction)
	}
}

func createTestRoom(t *testing.T, store *MemoryStore) {
	t.Helper()
	if _, err := store.CreateRoom(CreateRoomInput{
		Code: "party", HostID: "host", HostName: "房主", GameID: "who-is-ai",
	}); err != nil {
		t.Fatalf("CreateRoom() error = %v", err)
	}
	for _, memberID := range []string{"p2", "p3"} {
		if _, _, err := store.JoinRoom("party", memberID, memberID); err != nil {
			t.Fatalf("JoinRoom(%q) error = %v", memberID, err)
		}
	}
}
