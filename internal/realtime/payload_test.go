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
