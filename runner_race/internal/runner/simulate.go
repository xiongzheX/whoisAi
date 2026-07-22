package runner

import (
	"fmt"
	"math"
)

const raceSpeedScale = 0.4

func Simulate(a, b Runner, seed int64) (MatchResult, error) {
	if err := ValidateRunner(a); err != nil {
		return MatchResult{}, fmt.Errorf("runner A: %w", err)
	}
	if err := ValidateRunner(b); err != nil {
		return MatchResult{}, fmt.Errorf("runner B: %w", err)
	}

	rng := NewRNG(seed)
	environment := randomEnvironment(rng)
	course := randomCourse(rng)
	stateA := initState(a, rng)
	stateB := initState(b, rng)

	result := MatchResult{
		MatchID:     fmt.Sprintf("match_%d", seed),
		Seed:        seed,
		Environment: environment,
		Course:      course,
		Frames:      make([]ReplayFrame, 0, MaxTicks),
		Events:      make([]ReplayEvent, 0, 16),
	}

	for tick := 0; tick < MaxTicks; tick++ {
		t := float64(tick) * DT
		stepRunner(&stateA, a, stateB, environment, course, t, tick, rng, &result)
		stepRunner(&stateB, b, stateA, environment, course, t, tick, rng, &result)

		result.Frames = append(result.Frames, ReplayFrame{
			Tick: tick,
			Time: round3(t),
			A:    snapshot(stateA),
			B:    snapshot(stateB),
		})

		aFinished := stateA.Position >= TrackLength
		bFinished := stateB.Position >= TrackLength
		if aFinished || bFinished {
			result.Winner = pickFinishWinner(stateA, stateB)
			result.Reason = "finish"
			return result, nil
		}
	}

	result.Winner = pickTieBreakWinner(stateA, stateB, a, b, rng)
	result.Reason = "tiebreak"
	return result, nil
}

func stepRunner(s *runnerState, r Runner, opponent runnerState, environment string, course Course, t float64, tick int, rng *RNG, result *MatchResult) {
	if s.Position >= TrackLength {
		return
	}
	if t < s.ReactionDelay {
		s.Speed = 0
		return
	}

	speed := phaseSpeed(r, *s)
	speed += environmentSpeedBonus(environment, r, *s)

	if s.Position >= 25 && s.Position < 70 {
		bonus, _ := middleModifier(r.Strategy.Middle)
		speed += bonus
	}

	sprint, sprinting := sprintBonus(r.Strategy.Sprint, s.Position)
	speed += sprint
	speed += traitSpeedBonus(r, *s, opponent)
	speed += packTensionBonus(r, *s, opponent, t)
	speed += curveSpeedModifier(s, r, course, tick, t, result)

	if r.Trait == TraitBigHeart && !s.GritUsed && s.Position >= 50 && opponent.Position-s.Position > 3 {
		if rng.Float64() < float64(r.Stats.Grit)*0.004*1.80 {
			s.GritUsed = true
			s.GritTicks = 10
			addEvent(result, tick, t, r.ID, "grit", "咬牙追赶")
		}
	}
	if !s.GritUsed && s.Position >= 55 && opponent.Position-s.Position > 4 {
		if rng.Float64() < float64(r.Stats.Grit)*0.004 {
			s.GritUsed = true
			s.GritTicks = 10
			addEvent(result, tick, t, r.ID, "grit", "落后爆发")
		}
	}
	if s.GritTicks > 0 {
		speed += 2.5
		if r.Trait == TraitBigHeart {
			speed += 0.8
		}
		s.GritTicks--
	}
	if !s.SurgeUsed && s.Position >= 35 && opponent.Position-s.Position > 2.2 {
		chance := 0.006 + float64(r.Stats.Grit)*0.0022 + float64(r.Stats.Stamina)*0.0012
		if r.Trait == TraitChaser || r.Trait == TraitBigHeart {
			chance += 0.012
		}
		if rng.Float64() < chance {
			s.SurgeUsed = true
			s.SurgeTicks = 12
			addEvent(result, tick, t, r.ID, "surge", "找回节奏")
		}
	}
	if s.SurgeTicks > 0 {
		speed += 1.9 + float64(r.Stats.Grit)*0.05
		s.SurgeTicks--
	}
	if !s.PressureUsed && s.Position >= 38 && s.Position-opponent.Position > 2.8 {
		chance := 0.006 + float64(10-r.Stats.Stability)*0.0018 + float64(10-r.Stats.Grit)*0.0010
		if r.Strategy.Sprint == SprintEarly {
			chance += 0.006
		}
		if r.Trait == TraitSteadyMachine || r.Trait == TraitCleanSteps {
			chance *= 0.45
		}
		if rng.Float64() < chance {
			s.PressureUsed = true
			s.PressureTicks = 8
			addEvent(result, tick, t, r.ID, "pressure", "脚下发紧")
		}
	}
	if s.PressureTicks > 0 {
		speed -= 1.7
		s.PressureTicks--
	}

	if r.Trait == TraitGambler {
		updateGambler(s, r, tick, t, rng, result)
	}
	if s.GamblerTicks > 0 {
		speed += s.GamblerBonus
		s.GamblerTicks--
	}

	speed += noise(r, *s, rng)

	if s.MistakeTicks > 0 {
		speed -= mistakePenalty(r)
		s.MistakeTicks--
	} else if rng.Float64() < mistakeChance(r, environment) {
		s.MistakeTicks = 5
		addEvent(result, tick, t, r.ID, "mistake", "脚步乱了")
	}

	speed -= fatiguePenalty(s.Energy, r)
	if t > 8.0 {
		speed += (t - 8.0) * 14
	}
	speed *= raceSpeedScale
	speed -= obstaclePenalty(s, r, course, speed, tick, t, result)
	if speed < 4.4 {
		speed = 4.4
	}

	cost := 0.45
	if s.Position >= 25 && s.Position < 70 {
		_, energyMul := middleModifier(r.Strategy.Middle)
		cost *= energyMul
	}
	if sprinting {
		cost += 0.45
	}
	cost *= environmentEnergyMultiplier(environment, *s)
	cost *= traitEnergyCostMultiplier(r, sprinting)
	s.Energy -= cost
	if s.Energy < 0 {
		s.Energy = 0
	}

	previous := s.Position
	s.Speed = speed
	s.Position += speed * DT
	if s.Position >= TrackLength && s.FinishTime < 0 {
		over := s.Position - TrackLength
		portion := 1.0
		if s.Position > previous {
			portion = 1 - over/(s.Position-previous)
		}
		s.FinishTime = t + DT*portion
		addEvent(result, tick, t, r.ID, "finish", "冲过终点")
	}
}

func curveSpeedModifier(s *runnerState, r Runner, course Course, tick int, t float64, result *MatchResult) float64 {
	modifier := 0.0
	for i, curve := range course.Curves {
		if s.Position < curve.Start || s.Position > curve.End {
			continue
		}
		if s.CurveMask&(1<<uint(i)) == 0 {
			s.CurveMask |= 1 << uint(i)
			addEvent(result, tick, t, r.ID, "curve", "进入弯道")
		}
		penalty := 0.46 +
			float64(10-r.Stats.Stability)*0.04 +
			math.Max(0, float64(r.Stats.Speed-r.Stats.Stability))*0.035
		if curve.Direction == "right" {
			penalty += float64(10-r.Stats.Reaction) * 0.012
		}
		switch r.Trait {
		case TraitRhythmMaster, TraitSteadyMachine:
			penalty *= 0.72
		case TraitLightFeet, TraitCleanSteps:
			penalty *= 0.82
		}
		modifier -= penalty
	}
	return modifier
}

func obstaclePenalty(s *runnerState, r Runner, course Course, speed float64, tick int, t float64, result *MatchResult) float64 {
	for s.NextObstacle < len(course.Obstacles) && s.Position >= course.Obstacles[s.NextObstacle].Position {
		s.NextObstacle++
	}
	if s.NextObstacle >= len(course.Obstacles) {
		return 0
	}

	obstacle := course.Obstacles[s.NextObstacle]
	projected := s.Position + speed*DT
	if projected < obstacle.Position {
		return 0
	}
	s.NextObstacle++

	penalty := obstaclePenaltyValue(obstacle.Kind, r)
	addEventWithEffect(result, tick, t, r.ID, obstacle.Kind, obstacleMessage(obstacle.Kind, r, penalty), obstacleEffect(obstacle.Kind, r, penalty), penalty)
	return penalty
}

func obstaclePenaltyValue(kind string, r Runner) float64 {
	var penalty float64
	switch kind {
	case ObstacleHurdle:
		penalty = 1.08 - float64(r.Stats.Burst)*0.045 - float64(r.Stats.Reaction)*0.025
	case ObstaclePuddle:
		penalty = 1.18 - float64(r.Stats.Stability)*0.055 - float64(r.Stats.Reaction)*0.018
	case ObstacleCone:
		penalty = 1.00 - float64(r.Stats.Reaction)*0.05 - float64(r.Stats.Stability)*0.025
	default:
		penalty = 0.7
	}
	switch r.Trait {
	case TraitCleanSteps:
		penalty *= 0.62
	case TraitLightFeet, TraitRhythmMaster:
		penalty *= 0.78
	case TraitSteadyMachine:
		penalty *= 0.84
	}
	return math.Max(0.22, penalty)
}

func obstacleEffect(kind string, r Runner, penalty float64) string {
	if penalty <= 0.34 {
		if kind == ObstaclePuddle && (r.Trait == TraitCleanSteps || r.Stats.Stability >= 8) {
			return "clean"
		}
		if kind == ObstacleCone && (r.Trait == TraitLightFeet || r.Stats.Reaction >= 8) {
			return "dodge"
		}
		return "hop"
	}
	if penalty >= 0.72 {
		if kind == ObstaclePuddle {
			return "slip"
		}
		return "stumble"
	}
	switch kind {
	case ObstacleHurdle:
		return "hop"
	case ObstaclePuddle:
		return "splash"
	case ObstacleCone:
		return "dodge"
	default:
		return "bump"
	}
}

func obstacleMessage(kind string, r Runner, penalty float64) string {
	effect := obstacleEffect(kind, r, penalty)
	switch kind {
	case ObstacleHurdle:
		if effect == "stumble" {
			return "被小栏绊了一下"
		}
		if effect == "hop" && (r.Stats.Burst >= 8 || r.Trait == TraitRocketStart) {
			return "爆发跳过小栏"
		}
		return "跨过小栏"
	case ObstaclePuddle:
		if effect == "clean" {
			return "干净踩过水坑"
		}
		if effect == "slip" {
			return "水坑里打滑"
		}
		return "踩出水花"
	case ObstacleCone:
		if effect == "dodge" && (r.Stats.Reaction >= 8 || r.Trait == TraitLightFeet) {
			return "灵巧绕开路标"
		}
		if effect == "stumble" {
			return "绕路标时踉跄"
		}
		return "绕过路标"
	default:
		return "通过障碍"
	}
}

func packTensionBonus(r Runner, self, opponent runnerState, t float64) float64 {
	if self.Position >= 72 || opponent.Position >= 76 {
		return 0
	}
	gap := opponent.Position - self.Position
	if math.Abs(gap) < 0.8 {
		return 0
	}

	phase := 1.0
	if t < 2.0 {
		phase = 1.22
	} else if t > 3.0 {
		phase = math.Max(0.35, 1.0-(t-3.0)*0.55)
	}

	if gap > 0 {
		chase := math.Min(2.7, (gap-0.8)*0.82)
		chase *= 0.78 + float64(r.Stats.Stamina)*0.025 + float64(r.Stats.Grit)*0.030
		if r.Trait == TraitChaser || r.Trait == TraitBigHeart {
			chase += 0.35
		}
		return chase * phase
	}

	lead := math.Min(1.55, (-gap-0.8)*0.46)
	lead *= 1.18 - float64(r.Stats.Stability)*0.035
	if r.Trait == TraitSteadyMachine || r.Trait == TraitFrontRunner {
		lead *= 0.72
	}
	return -lead * phase
}

func updateGambler(s *runnerState, r Runner, tick int, t float64, rng *RNG, result *MatchResult) {
	if s.GamblerTicks > 0 {
		return
	}
	if rng.Float64() < 0.006 {
		s.GamblerTicks = 5
		s.GamblerBonus = 2.0
		addEvent(result, tick, t, r.ID, "gambler_high", "赌徒爆发")
		return
	}
	if rng.Float64() < 0.005 {
		s.GamblerTicks = 5
		s.GamblerBonus = -1.6
		addEvent(result, tick, t, r.ID, "gambler_low", "赌徒失速")
	}
}

func pickFinishWinner(a, b runnerState) string {
	if a.FinishTime >= 0 && b.FinishTime >= 0 {
		if a.FinishTime <= b.FinishTime {
			return a.RunnerID
		}
		return b.RunnerID
	}
	if a.FinishTime >= 0 {
		return a.RunnerID
	}
	return b.RunnerID
}

func pickTieBreakWinner(a, b runnerState, runnerA, runnerB Runner, rng *RNG) string {
	scoreA := a.Position +
		float64(runnerA.Stats.Speed)*0.03 +
		float64(runnerA.Stats.Stamina)*0.025 +
		float64(runnerA.Stats.Grit)*0.04 +
		finalKickBonus(runnerA) +
		rng.Range(0, 0.02)
	scoreB := b.Position +
		float64(runnerB.Stats.Speed)*0.03 +
		float64(runnerB.Stats.Stamina)*0.025 +
		float64(runnerB.Stats.Grit)*0.04 +
		finalKickBonus(runnerB) +
		rng.Range(0, 0.02)
	if scoreA >= scoreB {
		return a.RunnerID
	}
	return b.RunnerID
}

func snapshot(s runnerState) RunnerSnapshot {
	return RunnerSnapshot{
		RunnerID: s.RunnerID,
		Position: round3(math.Min(s.Position, TrackLength)),
		Speed:    round3(s.Speed),
		Energy:   round3(s.Energy),
	}
}

func addEvent(result *MatchResult, tick int, t float64, runnerID, eventType, message string) {
	addEventWithEffect(result, tick, t, runnerID, eventType, message, "", 0)
}

func addEventWithEffect(result *MatchResult, tick int, t float64, runnerID, eventType, message, effect string, intensity float64) {
	result.Events = append(result.Events, ReplayEvent{
		Tick:      tick,
		Time:      round3(t),
		Runner:    runnerID,
		Type:      eventType,
		Message:   message,
		Effect:    effect,
		Intensity: round3(intensity),
	})
}

func round3(v float64) float64 {
	return math.Round(v*1000) / 1000
}
