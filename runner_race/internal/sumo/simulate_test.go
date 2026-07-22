package sumo

import (
	"math"
	"strings"
	"testing"
)

func TestSimulateProducesWinnerWithinLimit(t *testing.T) {
	a := Wrestler{
		ID:    "a",
		Name:  "团子力士",
		Trait: TraitIronFeet,
		Style: StyleBalanced,
		Stats: Stats{Power: 7, Weight: 7, Balance: 7, Footwork: 5, Stamina: 5, Spirit: 5},
	}
	b := Wrestler{
		ID:    "b",
		Name:  "年糕丸",
		Trait: TraitBullRush,
		Style: StyleAggressive,
		Stats: Stats{Power: 8, Weight: 5, Balance: 5, Footwork: 7, Stamina: 5, Spirit: 6},
	}
	for seed := int64(1); seed <= 100; seed++ {
		result, err := Simulate(a, b, seed)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		if result.Winner == "" {
			t.Fatalf("seed %d has no winner", seed)
		}
		if result.Reason == "judge" {
			t.Fatalf("seed %d ended by judge without a visible push-out", seed)
		}
		if len(result.Frames) > MaxTicks {
			t.Fatalf("seed %d frames = %d", seed, len(result.Frames))
		}
		for _, frame := range result.Frames {
			gap := math.Hypot(frame.B.X-frame.A.X, frame.B.Y-frame.A.Y)
			if gap < 1.20 {
				t.Fatalf("seed %d tick %d gap = %.3f, wrestlers overlap", seed, frame.Tick, gap)
			}
		}
		last := result.Frames[len(result.Frames)-1]
		loser := last.A
		if result.Winner == last.A.WrestlerID {
			loser = last.B
		}
		if math.Hypot(loser.X, loser.Y) < RingRadius {
			t.Fatalf("seed %d loser is still in ring: %.3f", seed, math.Hypot(loser.X, loser.Y))
		}
	}
}

func TestArchetypeCounterRules(t *testing.T) {
	tests := []struct {
		name     string
		wrestler Wrestler
		want     string
	}{
		{
			name: "agile",
			wrestler: Wrestler{
				ID: "agile", Name: "跳跳团", Trait: TraitSoftStep, Style: StyleTrickster,
				Stats: Stats{Power: 4, Weight: 4, Balance: 8, Footwork: 10, Stamina: 5, Spirit: 5},
			},
			want: ArchetypeAgile,
		},
		{
			name: "power",
			wrestler: Wrestler{
				ID: "power", Name: "猛推团", Trait: TraitBullRush, Style: StyleAggressive,
				Stats: Stats{Power: 10, Weight: 6, Balance: 5, Footwork: 4, Stamina: 6, Spirit: 5},
			},
			want: ArchetypePower,
		},
		{
			name: "guard",
			wrestler: Wrestler{
				ID: "guard", Name: "稳稳团", Trait: TraitIronFeet, Style: StyleDefensive,
				Stats: Stats{Power: 4, Weight: 8, Balance: 8, Footwork: 4, Stamina: 6, Spirit: 6},
			},
			want: ArchetypeGuard,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Archetype(tt.wrestler); got != tt.want {
				t.Fatalf("Archetype() = %q, want %q", got, tt.want)
			}
		})
	}

	if !counters(ArchetypeAgile, ArchetypePower) {
		t.Fatal("agile should counter power")
	}
	if counters(ArchetypePower, ArchetypeAgile) {
		t.Fatal("power should not counter agile")
	}
}

func TestSimulateCanEmitCounterEvent(t *testing.T) {
	agile := Wrestler{
		ID: "agile", Name: "跳跳团", Trait: TraitSoftStep, Style: StyleTrickster,
		Stats: Stats{Power: 4, Weight: 4, Balance: 8, Footwork: 10, Stamina: 5, Spirit: 5},
	}
	power := Wrestler{
		ID: "power", Name: "猛推团", Trait: TraitBullRush, Style: StyleAggressive,
		Stats: Stats{Power: 10, Weight: 6, Balance: 5, Footwork: 4, Stamina: 6, Spirit: 5},
	}

	for seed := int64(1); seed <= 160; seed++ {
		result, err := Simulate(agile, power, seed)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		if result.ArchetypeA != ArchetypeAgile || result.ArchetypeB != ArchetypePower {
			t.Fatalf("seed %d archetypes = %q/%q", seed, result.ArchetypeA, result.ArchetypeB)
		}
		for _, event := range result.Events {
			if strings.HasPrefix(event.Type, "counter_") {
				return
			}
		}
	}
	t.Fatal("expected at least one counter event across deterministic sample seeds")
}

func TestSimulateCanEmitRallyEvent(t *testing.T) {
	steady := Wrestler{
		ID: "steady", Name: "稳住团", Trait: TraitCalmBreath, Style: StyleDefensive,
		Stats: Stats{Power: 5, Weight: 6, Balance: 8, Footwork: 6, Stamina: 7, Spirit: 4},
	}
	charger := Wrestler{
		ID: "charger", Name: "猛推团", Trait: TraitBullRush, Style: StyleAggressive,
		Stats: Stats{Power: 10, Weight: 5, Balance: 5, Footwork: 4, Stamina: 6, Spirit: 6},
	}

	for seed := int64(1); seed <= 220; seed++ {
		result, err := Simulate(steady, charger, seed)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		for _, event := range result.Events {
			if event.Type == "rally" || event.Type == "edge_turn" {
				return
			}
		}
	}
	t.Fatal("expected at least one rally or edge turn event across deterministic sample seeds")
}

func TestFinishVariantsAreRandomized(t *testing.T) {
	a := Wrestler{
		ID: "a", Name: "团团", Trait: TraitCounterGrip, Style: StyleBalanced,
		Stats: Stats{Power: 6, Weight: 6, Balance: 6, Footwork: 6, Stamina: 6, Spirit: 6},
	}
	b := Wrestler{
		ID: "b", Name: "圆圆", Trait: TraitLowCenter, Style: StyleBalanced,
		Stats: Stats{Power: 6, Weight: 6, Balance: 6, Footwork: 6, Stamina: 6, Spirit: 6},
	}

	counts := map[string]int{}
	const samples = 240
	for seed := int64(1); seed <= samples; seed++ {
		result, err := Simulate(a, b, seed)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		var finishType string
		for _, event := range result.Events {
			if event.Type == "finish_push" || event.Type == "edge_turn" {
				finishType = event.Message
			}
		}
		counts[finishType]++
	}

	direct := counts["顺势推出"]
	pressure := counts["回身推出"]
	edge := counts["回身借力"]
	if direct < 70 || direct > 120 {
		t.Fatalf("direct finish count = %d, want roughly 40%% of %d; counts=%v", direct, samples, counts)
	}
	if pressure < 45 || pressure > 95 {
		t.Fatalf("pressure finish count = %d, want roughly 30%% of %d; counts=%v", pressure, samples, counts)
	}
	if edge < 45 || edge > 95 {
		t.Fatalf("edge turn finish count = %d, want roughly 30%% of %d; counts=%v", edge, samples, counts)
	}
}

func TestDramaticFinishPushesAcrossRingWhenTriggered(t *testing.T) {
	a := Wrestler{
		ID: "a", Name: "团团", Trait: TraitCounterGrip, Style: StyleBalanced,
		Stats: Stats{Power: 6, Weight: 6, Balance: 6, Footwork: 6, Stamina: 6, Spirit: 6},
	}
	b := Wrestler{
		ID: "b", Name: "圆圆", Trait: TraitLowCenter, Style: StyleBalanced,
		Stats: Stats{Power: 6, Weight: 6, Balance: 6, Footwork: 6, Stamina: 6, Spirit: 6},
	}

	var result MatchResult
	for seed := int64(1); seed <= 80; seed++ {
		got, err := Simulate(a, b, seed)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		for _, event := range got.Events {
			if event.Type == "near_throw" {
				result = got
				break
			}
		}
		if result.Winner != "" {
			break
		}
	}
	if result.Winner == "" {
		t.Fatal("expected at least one dramatic finish in sample seeds")
	}

	nearThrowTick := -1
	finishTick := -1
	for _, event := range result.Events {
		switch event.Type {
		case "near_throw":
			nearThrowTick = event.Tick
		case "finish_push", "edge_turn":
			finishTick = event.Tick
		}
	}
	if nearThrowTick < 0 || finishTick < 0 {
		t.Fatalf("expected near_throw and finish_push events, got %+v", result.Events)
	}

	var dangerFrame ReplayFrame
	for _, frame := range result.Frames {
		if frame.Tick == finishTick-1 {
			dangerFrame = frame
			break
		}
	}
	last := result.Frames[len(result.Frames)-1]
	winnerDanger := dangerFrame.A
	loserFinal := last.B
	if result.Winner == dangerFrame.B.WrestlerID {
		winnerDanger = dangerFrame.B
		loserFinal = last.A
	}

	dangerX, dangerY := unit(winnerDanger.X, winnerDanger.Y)
	finalX, finalY := unit(loserFinal.X, loserFinal.Y)
	if dangerX*finalX+dangerY*finalY > -0.75 {
		t.Fatalf("loser final direction should be opposite winner danger edge, dot = %.3f", dangerX*finalX+dangerY*finalY)
	}
}

func TestPresetArchetypesStayBalanced(t *testing.T) {
	presets := []Wrestler{
		{
			ID: "power", Name: "力量型", Trait: TraitBullRush, Style: StyleAggressive,
			Stats: Stats{Power: 10, Weight: 5, Balance: 5, Footwork: 4, Stamina: 6, Spirit: 6},
		},
		{
			ID: "guard", Name: "稳守型", Trait: TraitIronFeet, Style: StyleDefensive,
			Stats: Stats{Power: 4, Weight: 8, Balance: 9, Footwork: 4, Stamina: 6, Spirit: 5},
		},
		{
			ID: "agile", Name: "灵巧型", Trait: TraitSoftStep, Style: StyleTrickster,
			Stats: Stats{Power: 4, Weight: 4, Balance: 8, Footwork: 10, Stamina: 5, Spirit: 5},
		},
		{
			ID: "spirit", Name: "气势型", Trait: TraitBigRoar, Style: StyleAggressive,
			Stats: Stats{Power: 6, Weight: 5, Balance: 5, Footwork: 5, Stamina: 5, Spirit: 10},
		},
		{
			ID: "endurance", Name: "消耗型", Trait: TraitCalmBreath, Style: StyleBalanced,
			Stats: Stats{Power: 5, Weight: 8, Balance: 5, Footwork: 4, Stamina: 9, Spirit: 5},
		},
		{
			ID: "balanced", Name: "均衡型", Trait: TraitCounterGrip, Style: StyleBalanced,
			Stats: Stats{Power: 6, Weight: 6, Balance: 6, Footwork: 6, Stamina: 6, Spirit: 6},
		},
	}

	for _, wrestler := range presets {
		wins := 0
		total := 0
		for _, opponent := range presets {
			if wrestler.ID == opponent.ID {
				continue
			}
			for seed := int64(1); seed <= 50; seed++ {
				result, err := Simulate(wrestler, opponent, seed)
				if err != nil {
					t.Fatalf("%s vs %s seed %d: %v", wrestler.ID, opponent.ID, seed, err)
				}
				if result.Winner == wrestler.ID {
					wins++
				}
				total++

				result, err = Simulate(opponent, wrestler, seed+50)
				if err != nil {
					t.Fatalf("%s vs %s mirrored seed %d: %v", opponent.ID, wrestler.ID, seed, err)
				}
				if result.Winner == wrestler.ID {
					wins++
				}
				total++
			}
		}
		rate := float64(wins) / float64(total)
		if rate < 0.35 || rate > 0.65 {
			t.Fatalf("%s average win rate = %.1f%%, want 35%%-65%%", wrestler.ID, rate*100)
		}
	}
}
