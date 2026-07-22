package realtime

import "testing"

func TestDecodePayloadFromSocketMap(t *testing.T) {
	t.Parallel()

	var payload joinRoomPayload
	err := decodePayload([]any{map[string]any{
		"roomId": "room1",
		"name":   "玩家1",
		"mode":   "solo",
	}}, &payload)
	if err != nil {
		t.Fatalf("decodePayload returned error: %v", err)
	}
	if payload.RoomID != "room1" || payload.Name != "玩家1" || payload.Mode != "solo" {
		t.Fatalf("payload = %+v", payload)
	}
}

func TestStablePlayerIDDoesNotExposeResumeToken(t *testing.T) {
	t.Parallel()

	const token = "player_0123456789abcdef"
	first := stablePlayerID("room1", token)
	if first == token || first == "" {
		t.Errorf("stablePlayerID(room1, token) = %q, want non-secret public ID", first)
	}
	if second := stablePlayerID("room1", token); second != first {
		t.Errorf("stablePlayerID(room1, token) = %q then %q, want stable value", first, second)
	}
	if otherRoom := stablePlayerID("room2", token); otherRoom == first {
		t.Errorf("stablePlayerID(room2, token) = %q, want room-scoped identity", otherRoom)
	}
}
