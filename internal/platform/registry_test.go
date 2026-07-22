package platform

import "testing"

func TestDefaultRegistryContainsCurrentGames(t *testing.T) {
	t.Parallel()

	games := DefaultRegistry().Games()
	wantIDs := []string{"who-is-ai", "bean-sprint", "dumpling-sumo"}
	if len(games) != len(wantIDs) {
		t.Fatalf("len(DefaultRegistry().Games()) = %d, want %d", len(games), len(wantIDs))
	}
	for i, wantID := range wantIDs {
		if games[i].ID != wantID {
			t.Errorf("DefaultRegistry().Games()[%d].ID = %q, want %q", i, games[i].ID, wantID)
		}
	}
}

func TestRegistryOrdersGamesAndReturnsCopies(t *testing.T) {
	t.Parallel()

	registry, err := NewRegistry([]GameDefinition{
		{ID: "later", Slug: "later", Name: "Later", Status: GameComingSoon, MinPlayers: 2, MaxPlayers: 4, SortOrder: 20, Tags: []string{"b"}},
		{ID: "first", Slug: "first", Name: "First", Status: GameActive, MinPlayers: 3, MaxPlayers: 6, SortOrder: 10, Tags: []string{"a"}},
	})
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	games := registry.Games()
	if got, want := games[0].ID, "first"; got != want {
		t.Errorf("Games()[0].ID = %q, want %q", got, want)
	}
	games[0].Tags[0] = "changed"
	game, _ := registry.Game("first")
	if got, want := game.Tags[0], "a"; got != want {
		t.Errorf("Game(first).Tags[0] = %q after caller mutation, want %q", got, want)
	}
}

func TestRegistryRejectsInvalidDefinitions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		definitions []GameDefinition
	}{
		{name: "missing id", definitions: []GameDefinition{{Slug: "x", Name: "X", Status: GameActive, MinPlayers: 1, MaxPlayers: 2}}},
		{name: "bad range", definitions: []GameDefinition{{ID: "x", Slug: "x", Name: "X", Status: GameActive, MinPlayers: 3, MaxPlayers: 2}}},
		{name: "duplicate id", definitions: []GameDefinition{
			{ID: "x", Slug: "x", Name: "X", Status: GameActive, MinPlayers: 1, MaxPlayers: 2},
			{ID: "x", Slug: "x-2", Name: "X2", Status: GameActive, MinPlayers: 1, MaxPlayers: 2},
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if _, err := NewRegistry(test.definitions); err == nil {
				t.Errorf("NewRegistry(%s) succeeded, want error", test.name)
			}
		})
	}
}
