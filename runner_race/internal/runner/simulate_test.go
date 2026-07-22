package runner

import "testing"

func TestSimulateFinishesWithinLimit(t *testing.T) {
	a := Runner{
		ID:    "a",
		Name:  "小蓝兔",
		Trait: TraitRocketStart,
		Stats: Stats{Burst: 8, Speed: 7, Stamina: 5, Stability: 6, Reaction: 7, Grit: 3},
		Strategy: Strategy{
			Start:  StartNormal,
			Middle: MiddleSteady,
			Sprint: SprintNormal,
		},
	}
	b := Runner{
		ID:    "b",
		Name:  "红豆",
		Trait: TraitLateBeast,
		Stats: Stats{Burst: 4, Speed: 7, Stamina: 8, Stability: 6, Reaction: 5, Grit: 6},
		Strategy: Strategy{
			Start:  StartSafe,
			Middle: MiddleConserve,
			Sprint: SprintLate,
		},
	}

	for seed := int64(1); seed <= 100; seed++ {
		result, err := Simulate(a, b, seed)
		if err != nil {
			t.Fatalf("simulate seed %d: %v", seed, err)
		}
		if result.Winner == "" {
			t.Fatalf("seed %d has no winner", seed)
		}
		if len(result.Frames) > MaxTicks {
			t.Fatalf("seed %d frame count = %d, want <= %d", seed, len(result.Frames), MaxTicks)
		}
		last := result.Frames[len(result.Frames)-1]
		if last.Time > MaxSeconds {
			t.Fatalf("seed %d last time = %f, want <= %f", seed, last.Time, MaxSeconds)
		}
	}
}

func TestSimulateIncludesSharedCourse(t *testing.T) {
	a := Runner{
		ID:    "a",
		Name:  "小蓝兔",
		Trait: TraitCleanSteps,
		Stats: Stats{Burst: 6, Speed: 6, Stamina: 6, Stability: 8, Reaction: 6, Grit: 4},
		Strategy: Strategy{
			Start:  StartNormal,
			Middle: MiddleSteady,
			Sprint: SprintNormal,
		},
	}
	b := Runner{
		ID:    "b",
		Name:  "红豆",
		Trait: TraitLightFeet,
		Stats: Stats{Burst: 7, Speed: 7, Stamina: 5, Stability: 5, Reaction: 7, Grit: 5},
		Strategy: Strategy{
			Start:  StartNormal,
			Middle: MiddleSteady,
			Sprint: SprintNormal,
		},
	}

	result, err := Simulate(a, b, 20260703)
	if err != nil {
		t.Fatalf("simulate: %v", err)
	}
	if len(result.Course.Obstacles) != 3 {
		t.Fatalf("obstacle count = %d, want 3", len(result.Course.Obstacles))
	}
	if len(result.Course.Curves) != 2 {
		t.Fatalf("curve count = %d, want 2", len(result.Course.Curves))
	}
	if result.Course.Shape == "" {
		t.Fatal("empty course shape")
	}
	if len(result.Course.Path) < 4 {
		t.Fatalf("path point count = %d, want >= 4", len(result.Course.Path))
	}
	for i, obstacle := range result.Course.Obstacles {
		if obstacle.Position <= 0 || obstacle.Position >= TrackLength {
			t.Fatalf("obstacle %d position = %f, want inside track", i, obstacle.Position)
		}
		if i > 0 && obstacle.Position <= result.Course.Obstacles[i-1].Position {
			t.Fatalf("obstacles are not sorted: %+v", result.Course.Obstacles)
		}
	}
	for i, curve := range result.Course.Curves {
		if curve.Start <= 0 || curve.End <= curve.Start || curve.End >= TrackLength {
			t.Fatalf("curve %d out of range: %+v", i, curve)
		}
	}
	for i, point := range result.Course.Path {
		if point.Meter < 0 || point.Meter > TrackLength || point.X < 0 || point.X > 100 || point.Y < 0 || point.Y > 100 {
			t.Fatalf("path point %d out of range: %+v", i, point)
		}
		if i > 0 && point.Meter <= result.Course.Path[i-1].Meter {
			t.Fatalf("path points are not sorted: %+v", result.Course.Path)
		}
	}
	hasObstacleEffect := false
	for _, event := range result.Events {
		if event.Type == ObstacleHurdle || event.Type == ObstaclePuddle || event.Type == ObstacleCone {
			if event.Effect == "" {
				t.Fatalf("obstacle event has empty effect: %+v", event)
			}
			hasObstacleEffect = true
		}
	}
	if !hasObstacleEffect {
		t.Fatal("expected at least one obstacle event with visible effect")
	}
}

func TestValidateRunnerRejectsTooManyPoints(t *testing.T) {
	r := Runner{
		ID:    "bad",
		Name:  "超模选手",
		Trait: TraitGambler,
		Stats: Stats{Burst: 10, Speed: 10, Stamina: 10, Stability: 10, Reaction: 10, Grit: 10},
		Strategy: Strategy{
			Start:  StartNormal,
			Middle: MiddleSteady,
			Sprint: SprintNormal,
		},
	}
	if err := ValidateRunner(r); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestSimulateCanEmitDramaEvent(t *testing.T) {
	a := Runner{
		ID:    "a",
		Name:  "小蓝兔",
		Trait: TraitChaser,
		Stats: Stats{Burst: 5, Speed: 6, Stamina: 7, Stability: 5, Reaction: 5, Grit: 8},
		Strategy: Strategy{
			Start:  StartSafe,
			Middle: MiddleConserve,
			Sprint: SprintLate,
		},
	}
	b := Runner{
		ID:    "b",
		Name:  "红豆",
		Trait: TraitRocketStart,
		Stats: Stats{Burst: 9, Speed: 7, Stamina: 5, Stability: 4, Reaction: 7, Grit: 4},
		Strategy: Strategy{
			Start:  StartRisky,
			Middle: MiddlePush,
			Sprint: SprintEarly,
		},
	}

	for seed := int64(1); seed <= 180; seed++ {
		result, err := Simulate(a, b, seed)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		for _, event := range result.Events {
			if event.Type == "surge" || event.Type == "pressure" {
				return
			}
		}
	}
	t.Fatal("expected at least one surge or pressure event across deterministic sample seeds")
}

func TestEarlyRaceDoesNotRevealWinnerTooOften(t *testing.T) {
	a := Runner{
		ID:    "a",
		Name:  "小蓝兔",
		Trait: TraitRocketStart,
		Stats: Stats{Burst: 8, Speed: 7, Stamina: 5, Stability: 6, Reaction: 7, Grit: 3},
		Strategy: Strategy{
			Start:  StartNormal,
			Middle: MiddleSteady,
			Sprint: SprintNormal,
		},
	}
	b := Runner{
		ID:    "b",
		Name:  "红豆",
		Trait: TraitLateBeast,
		Stats: Stats{Burst: 4, Speed: 7, Stamina: 8, Stability: 6, Reaction: 5, Grit: 6},
		Strategy: Strategy{
			Start:  StartSafe,
			Middle: MiddleConserve,
			Sprint: SprintLate,
		},
	}

	predictable := 0
	const samples = 120
	for seed := int64(1); seed <= samples; seed++ {
		result, err := Simulate(a, b, seed)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		winnerAhead := 0
		earlyFrames := 0
		for _, frame := range result.Frames {
			if frame.Time > 2.0 {
				break
			}
			earlyFrames++
			gap := frame.A.Position - frame.B.Position
			if result.Winner == b.ID {
				gap = -gap
			}
			if gap > 1.2 {
				winnerAhead++
			}
		}
		if earlyFrames > 0 && float64(winnerAhead)/float64(earlyFrames) > 0.70 {
			predictable++
		}
	}
	if predictable > 72 {
		t.Fatalf("winner visibly led too much in first two seconds for %d/%d seeds", predictable, samples)
	}
}
