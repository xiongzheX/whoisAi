package duel

import (
	"encoding/json"
	"testing"
)

func TestSimulateReturnsSharedDuelResult(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		gameID string
		config json.RawMessage
	}{
		{
			name:   "bean_sprint",
			gameID: GameBeanSprint,
			config: json.RawMessage(`{"name":"豆豆","stats":{"burst":6,"speed":6,"stamina":6,"stability":6,"reaction":6,"grit":6},"strategy":{"start":"normal","middle":"steady","sprint":"normal"}}`),
		},
		{
			name:   "dumpling_sumo",
			gameID: GameDumplingSumo,
			config: json.RawMessage(`{"name":"团团","stats":{"power":6,"weight":6,"balance":6,"footwork":6,"stamina":6,"spirit":6},"style":"balanced"}`),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			result, err := Simulate(test.gameID, []PlayerInput{
				{ID: "p1", Name: "玩家1", Config: test.config},
				{ID: "p2", Name: "玩家2", Config: test.config},
			}, 42)
			if err != nil {
				t.Fatalf("Simulate(%q) error = %v", test.gameID, err)
			}
			if len(result.Players) == 0 || len(result.Match) == 0 {
				t.Errorf("Simulate(%q) returned empty payload: %+v", test.gameID, result)
			}
		})
	}
}
