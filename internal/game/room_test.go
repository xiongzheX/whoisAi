package game

import (
	"strings"
	"testing"
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
