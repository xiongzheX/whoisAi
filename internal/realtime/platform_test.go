package realtime

import (
	"encoding/json"
	"strconv"
	"testing"

	siosocket "github.com/zishang520/socket.io/socket"

	"whoisai/internal/game"
	"whoisai/internal/platform"
)

func TestPlatformSessionTracksLegacyGameLifecycle(t *testing.T) {
	t.Parallel()

	gameStore := game.NewStore()
	gameStore.CreateRoom("party")
	registry := platform.DefaultRegistry()
	platformStore := platform.NewMemoryStore(registry)
	if _, err := platformStore.CreateRoom(platform.CreateRoomInput{
		Code: "party", HostID: "p1", HostName: "玩家1", GameID: "who-is-ai",
	}); err != nil {
		t.Fatalf("CreateRoom() error = %v", err)
	}
	for index := 1; index <= 5; index++ {
		id := playerID(index)
		if _, err := gameStore.AddPlayer("party", id, "玩家"+id, game.ModeNormal); err != nil {
			t.Fatalf("AddPlayer(%q) error = %v", id, err)
		}
		if index > 1 {
			if _, _, err := platformStore.JoinRoom("party", id, "玩家"+id); err != nil {
				t.Fatalf("JoinRoom(%q) error = %v", id, err)
			}
		}
	}
	service := game.NewService(gameStore)
	room, _, err := service.StartGame("party", "p1")
	if err != nil {
		t.Fatalf("StartGame() error = %v", err)
	}
	server := &Server{store: gameStore, platform: platformStore}

	session, err := server.ensurePlatformSession(room, "p1")
	if err != nil {
		t.Fatalf("ensurePlatformSession() error = %v", err)
	}
	if got, want := session.GameID, "who-is-ai"; got != want {
		t.Errorf("ensurePlatformSession().GameID = %q, want %q", got, want)
	}
	if err := server.validateActiveSession("party", session.ID); err != nil {
		t.Errorf("validateActiveSession(current) error = %v", err)
	}
	if err := server.validateActiveSession("party", "session_old"); err == nil {
		t.Error("validateActiveSession(stale) succeeded, want error")
	}

	if _, err := platformStore.FinishSession("party", nil); err != nil {
		t.Fatalf("FinishSession() error = %v", err)
	}
	if err := server.validateActiveSession("party", session.ID); err == nil {
		t.Error("validateActiveSession(finished) succeeded, want error")
	}
	if _, ok := platformStore.ActiveSession("party"); ok {
		t.Error("ActiveSession() exists after finish")
	}
	stored, ok := platformStore.Session(session.ID)
	if !ok || stored.Status != platform.SessionFinished {
		t.Errorf("Session(%q) = %+v, %v, want finished session", session.ID, stored, ok)
	}
}

func TestDuelGamesCreateAndFinishPlatformSessions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		gameID string
		config json.RawMessage
	}{
		{
			gameID: "bean-sprint",
			config: json.RawMessage(`{"name":"豆豆","stats":{"burst":6,"speed":6,"stamina":6,"stability":6,"reaction":6,"grit":6},"strategy":{"start":"normal","middle":"steady","sprint":"normal"}}`),
		},
		{
			gameID: "dumpling-sumo",
			config: json.RawMessage(`{"name":"团团","stats":{"power":6,"weight":6,"balance":6,"footwork":6,"stamina":6,"spirit":6},"style":"balanced"}`),
		},
	}

	for _, test := range tests {
		t.Run(test.gameID, func(t *testing.T) {
			t.Parallel()
			registry := platform.DefaultRegistry()
			platformStore := platform.NewMemoryStore(registry)
			room, err := platformStore.CreateRoom(platform.CreateRoomInput{
				Code: test.gameID, HostID: "p1", HostName: "玩家1", GameID: test.gameID,
			})
			if err != nil {
				t.Fatalf("CreateRoom(%q) error = %v", test.gameID, err)
			}
			room, _, err = platformStore.JoinRoom(test.gameID, "p2", "玩家2")
			if err != nil {
				t.Fatalf("JoinRoom(%q) error = %v", test.gameID, err)
			}

			io := siosocket.NewServer(nil, nil)
			gameStore := game.NewStore()
			server := NewWithPlatform(io, gameStore, game.NewService(gameStore), platformStore, registry)
			if _, _, err := server.duels.Ready(room, "p1", test.config); err != nil {
				t.Fatalf("Ready(p1, %q) error = %v", test.gameID, err)
			}
			if _, allReady, err := server.duels.Ready(room, "p2", test.config); err != nil || !allReady {
				t.Fatalf("Ready(p2, %q) = allReady %v, error %v; want true, nil", test.gameID, allReady, err)
			}
			if err := server.startDuel(room); err != nil {
				t.Fatalf("startDuel(%q) error = %v", test.gameID, err)
			}

			updatedRoom, _ := platformStore.RoomByCode(test.gameID)
			state := server.duels.State(updatedRoom)
			if state.Result == nil || state.SessionID == "" {
				t.Fatalf("startDuel(%q) state = %+v, want result and session", test.gameID, state)
			}
			if _, ok := platformStore.ActiveSession(test.gameID); ok {
				t.Errorf("ActiveSession(%q) exists after result", test.gameID)
			}
			storedSession, ok := platformStore.Session(state.SessionID)
			if !ok || storedSession.Status != platform.SessionFinished {
				t.Errorf("Session(%q) = %+v, %v; want finished", state.SessionID, storedSession, ok)
			}
			var settings struct {
				PlayerConfigs map[string]json.RawMessage `json:"playerConfigs"`
			}
			if err := json.Unmarshal(storedSession.Settings, &settings); err != nil {
				t.Errorf("Session(%q).Settings = %s, invalid JSON: %v", state.SessionID, storedSession.Settings, err)
			} else if got, want := len(settings.PlayerConfigs), 2; got != want {
				t.Errorf("Session(%q) player config count = %d, want %d", state.SessionID, got, want)
			}
			if snapshot, ok := platformStore.LatestSnapshot(state.SessionID, ""); !ok || snapshot.Version != 1 {
				t.Errorf("LatestSnapshot(%q) = %+v, %v; want version 1", state.SessionID, snapshot, ok)
			}
		})
	}
}

func playerID(index int) string {
	return "p" + strconv.Itoa(index)
}
