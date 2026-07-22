package game

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
)

func TestGameSnapshotSeparatesPublicAndPrivateState(t *testing.T) {
	t.Parallel()

	store := NewStore()
	store.CreateRoom("snapshot")
	players := []struct{ id, name string }{
		{"host", "房主"}, {"p2", "玩家2"}, {"p3", "玩家3"}, {"p4", "玩家4"}, {"p5", "玩家5"},
	}
	for _, player := range players {
		if _, err := store.AddPlayer("snapshot", player.id, player.name, ModeNormal); err != nil {
			t.Fatalf("AddPlayer(%q) error = %v", player.id, err)
		}
	}
	service := NewService(store)
	room, _, err := service.StartGame("snapshot", "host")
	if err != nil {
		t.Fatalf("StartGame() error = %v", err)
	}

	public, private, err := store.GameSnapshot("snapshot", "host")
	if err != nil {
		t.Fatalf("GameSnapshot() error = %v", err)
	}
	if got, want := public.StartedAtMillis, room.StartedAtMillis; got != want || got == 0 {
		t.Errorf("GameSnapshot().Public.StartedAt = %d, want non-zero %d", got, want)
	}
	if private.Role == "" {
		t.Error("GameSnapshot().Private.Role is empty")
	}
	if private.Role != room.Roles["host"] {
		t.Errorf("GameSnapshot().Private.Role = %q, want %q", private.Role, room.Roles["host"])
	}
	encoded, err := json.Marshal(public)
	if err != nil {
		t.Fatalf("json.Marshal(public snapshot) error = %v", err)
	}
	for _, secretField := range []string{"original", "possessed", "hasPossession"} {
		if strings.Contains(string(encoded), `"`+secretField+`"`) {
			t.Errorf("public snapshot contains private field %q: %s", secretField, encoded)
		}
	}
}

func TestChatHistorySurvivesRoundChange(t *testing.T) {
	t.Parallel()

	store := NewStore()
	store.CreateRoom("history")
	for index := 1; index <= 5; index++ {
		id := "p" + strconv.Itoa(index)
		if _, err := store.AddPlayer("history", id, id, ModeNormal); err != nil {
			t.Fatalf("AddPlayer(%q) error = %v", id, err)
		}
	}
	service := NewService(store)
	if _, _, err := service.StartGame("history", "p1"); err != nil {
		t.Fatalf("StartGame() error = %v", err)
	}
	if _, _, err := service.Chat("history", "p1", "第一轮证据"); err != nil {
		t.Fatalf("Chat() error = %v", err)
	}

	store.mu.Lock()
	store.rooms["history"].CurrentRound = 2
	store.mu.Unlock()

	public, _, err := store.GameSnapshot("history", "p1")
	if err != nil {
		t.Fatalf("GameSnapshot() error = %v", err)
	}
	if got, want := len(public.ChatMessages), 1; got != want {
		t.Errorf("GameSnapshot().Public.ChatMessages length = %d, want %d", got, want)
	}
	if got, want := public.ChatMessages[0].Round, 1; got != want {
		t.Errorf("ChatMessages[0].Round = %d, want %d", got, want)
	}
}
