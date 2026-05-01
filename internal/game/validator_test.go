package game

import "testing"

func TestValidateRoomID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "trims spaces", input: "  room-1  ", want: "room-1"},
		{name: "rejects empty", input: "", wantErr: true},
		{name: "rejects long id", input: "abcdefghijklmnopqrstu", wantErr: true},
		{name: "rejects symbols", input: "room@1", wantErr: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ValidateRoomID(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ValidateRoomID(%q) succeeded, want error", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("ValidateRoomID(%q) error = %v", tt.input, err)
			}
			if got != tt.want {
				t.Fatalf("ValidateRoomID(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestValidatePlayerName(t *testing.T) {
	t.Parallel()

	got, err := ValidatePlayerName(" <b>玩家1</b> ")
	if err != nil {
		t.Fatalf("ValidatePlayerName returned error: %v", err)
	}
	if got != "b玩家1/b" {
		t.Fatalf("ValidatePlayerName sanitized to %q, want %q", got, "b玩家1/b")
	}

	if _, err := ValidatePlayerName("admin"); err == nil {
		t.Fatal("ValidatePlayerName accepted a reserved name")
	}
}

func TestValidateChatMessage(t *testing.T) {
	t.Parallel()

	got, err := ValidateChatMessage("<b>消息</b>")
	if err != nil {
		t.Fatalf("ValidateChatMessage returned error: %v", err)
	}
	if got != "b消息/b" {
		t.Fatalf("ValidateChatMessage sanitized to %q, want %q", got, "b消息/b")
	}

	if _, err := ValidateChatMessage(""); err == nil {
		t.Fatal("ValidateChatMessage accepted an empty message")
	}
}
