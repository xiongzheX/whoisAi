package duel

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"whoisai/internal/platform"
)

func TestServiceStartsOnlyAfterBothPlayersReady(t *testing.T) {
	t.Parallel()

	room := duelRoom()
	service := NewService()
	config := json.RawMessage(`{"name":"豆豆","stats":{"burst":6,"speed":6,"stamina":6,"stability":6,"reaction":6,"grit":6},"strategy":{"start":"normal","middle":"steady","sprint":"normal"}}`)

	first, ready, err := service.Ready(room, "p1", config)
	if err != nil {
		t.Fatalf("Ready(p1) error = %v", err)
	}
	if ready {
		t.Error("Ready(p1) allReady = true, want false")
	}
	if first.Players[0].Ready != true || first.Players[1].Ready != false {
		t.Errorf("Ready(p1) players = %+v, want only p1 ready", first.Players)
	}
	publicState, err := json.Marshal(first)
	if err != nil {
		t.Fatalf("json.Marshal(Ready(p1)) error = %v", err)
	}
	if strings.Contains(string(publicState), `"stats"`) || strings.Contains(string(publicState), `"strategy"`) {
		t.Errorf("Ready(p1) public state leaked contestant config: %s", publicState)
	}

	_, ready, err = service.Ready(room, "p2", config)
	if err != nil {
		t.Fatalf("Ready(p2) error = %v", err)
	}
	if !ready {
		t.Fatal("Ready(p2) allReady = false, want true")
	}
	settings, err := service.SessionSettings(room)
	if err != nil {
		t.Fatalf("SessionSettings() error = %v", err)
	}
	if !json.Valid(settings) || !jsonContainsPlayerConfigs(settings, "p1", "p2") {
		t.Errorf("SessionSettings() = %s, want both frozen player configs", settings)
	}

	finished, err := service.Start(room, "session-1")
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if got, want := finished.Phase, PhaseFinished; got != want {
		t.Errorf("Start().Phase = %q, want %q", got, want)
	}
	if finished.Result == nil {
		t.Fatal("Start().Result = nil, want shared result")
	}
	if got, want := finished.SessionID, "session-1"; got != want {
		t.Errorf("Start().SessionID = %q, want %q", got, want)
	}
}

func jsonContainsPlayerConfigs(settings json.RawMessage, playerIDs ...string) bool {
	var decoded struct {
		PlayerConfigs map[string]json.RawMessage `json:"playerConfigs"`
	}
	if err := json.Unmarshal(settings, &decoded); err != nil {
		return false
	}
	for _, playerID := range playerIDs {
		if len(decoded.PlayerConfigs[playerID]) == 0 {
			return false
		}
	}
	return true
}

func TestServiceRematchClearsBothReadyFlags(t *testing.T) {
	t.Parallel()

	room := duelRoom()
	service := NewService()
	config := json.RawMessage(`{"name":"豆豆","stats":{"burst":6,"speed":6,"stamina":6,"stability":6,"reaction":6,"grit":6},"strategy":{"start":"normal","middle":"steady","sprint":"normal"}}`)
	_, _, _ = service.Ready(room, "p1", config)
	_, _, _ = service.Ready(room, "p2", config)
	if _, err := service.Start(room, "session-1"); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	state, allReady, err := service.Ready(room, "p1", config)
	if err != nil {
		t.Fatalf("Ready(rematch p1) error = %v", err)
	}
	if allReady {
		t.Error("Ready(rematch p1) allReady = true, want false")
	}
	if state.Result != nil || state.SessionID != "" {
		t.Errorf("Ready(rematch p1) retained previous result/session: %+v", state)
	}
	if !state.Players[0].Ready || state.Players[1].Ready {
		t.Errorf("Ready(rematch p1) players = %+v, want only p1 ready", state.Players)
	}
}

func TestServiceCancelReadyUnlocksConfiguration(t *testing.T) {
	t.Parallel()

	room := duelRoom()
	service := NewService()
	config := json.RawMessage(`{"name":"豆豆","stats":{"burst":6,"speed":6,"stamina":6,"stability":6,"reaction":6,"grit":6},"strategy":{"start":"normal","middle":"steady","sprint":"normal"}}`)
	if _, _, err := service.Ready(room, "p1", config); err != nil {
		t.Fatalf("Ready() error = %v", err)
	}
	state, err := service.CancelReady(room, "p1")
	if err != nil {
		t.Fatalf("CancelReady() error = %v", err)
	}
	if state.Players[0].Ready {
		t.Error("CancelReady().Players[0].Ready = true, want false")
	}
	if _, err := service.SessionSettings(room); !errors.Is(err, ErrNotReady) {
		t.Errorf("SessionSettings() error = %v, want %v", err, ErrNotReady)
	}
}

func TestServiceRemovePlayerResetsFinishedRound(t *testing.T) {
	t.Parallel()

	room := duelRoom()
	service := NewService()
	config := json.RawMessage(`{"name":"豆豆","stats":{"burst":6,"speed":6,"stamina":6,"stability":6,"reaction":6,"grit":6},"strategy":{"start":"normal","middle":"steady","sprint":"normal"}}`)
	_, _, _ = service.Ready(room, "p1", config)
	_, _, _ = service.Ready(room, "p2", config)
	if _, err := service.Start(room, "session-1"); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	service.RemovePlayer(room.Code, "p2")
	room.Members = room.Members[:1]
	state := service.State(room)
	if state.Phase != PhaseConfiguring || state.Result != nil || state.SessionID != "" {
		t.Errorf("State() after finished player leave = %+v, want clean configuring state", state)
	}
	if state.Players[0].Ready {
		t.Error("remaining player stayed ready after finished player leave")
	}
}

func duelRoom() *platform.PartyRoom {
	return &platform.PartyRoom{
		Code:           "duel-room",
		SelectedGameID: "bean-sprint",
		Members: []platform.RoomMember{
			{ID: "p1", DisplayName: "玩家1", Seat: 0, ConnectionStatus: platform.MemberOnline},
			{ID: "p2", DisplayName: "玩家2", Seat: 1, ConnectionStatus: platform.MemberOnline},
		},
	}
}
