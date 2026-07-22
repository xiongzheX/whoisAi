package runner

import "math"

func initState(r Runner, rng *RNG) runnerState {
	delay := 0.20 - float64(r.Stats.Reaction)*0.010
	switch r.Strategy.Start {
	case StartSafe:
		delay += 0.04
	case StartRisky:
		delay -= 0.04
	}
	if delay < 0.02 {
		delay = 0.02
	}

	energy := 70 + float64(r.Stats.Stamina)*2.5 + float64(r.Stats.Stability)*0.6 + traitEnergyBonus(r)
	return runnerState{
		RunnerID:      r.ID,
		Energy:        energy,
		ReactionDelay: delay,
		FinishTime:    -1,
	}
}

func phaseSpeed(r Runner, s runnerState) float64 {
	switch {
	case s.Position < 25:
		return 18 +
			float64(r.Stats.Speed)*0.10 +
			float64(r.Stats.Burst)*1.15 +
			float64(r.Stats.Reaction)*0.60 +
			float64(r.Stats.Stability)*0.18
	case s.Position < 70:
		return 19 +
			float64(r.Stats.Speed)*0.52 +
			float64(r.Stats.Stability)*0.45 +
			float64(r.Stats.Stamina)*0.28 +
			float64(r.Stats.Reaction)*0.08
	default:
		return 18.5 +
			float64(r.Stats.Speed)*0.10 +
			float64(r.Stats.Stamina)*0.55 +
			float64(r.Stats.Grit)*0.95 +
			float64(r.Stats.Burst)*0.12
	}
}

func middleModifier(strategy string) (speedBonus float64, energyMultiplier float64) {
	switch strategy {
	case MiddleConserve:
		return -1.0, 0.70
	case MiddlePush:
		return 1.2, 1.35
	default:
		return 0, 1.0
	}
}

func sprintStart(strategy string) float64 {
	switch strategy {
	case SprintEarly:
		return 70
	case SprintLate:
		return 90
	default:
		return 80
	}
}

func sprintBonus(strategy string, position float64) (float64, bool) {
	if position < sprintStart(strategy) {
		return 0, false
	}

	switch strategy {
	case SprintEarly:
		return 2.0, true
	case SprintLate:
		return 3.0, true
	default:
		return 2.4, true
	}
}

func fatiguePenalty(energy float64, r Runner) float64 {
	penalty := math.Max(0, 24-energy) * 0.15
	if r.Trait == TraitIronLungs {
		penalty *= 0.60
	}
	return penalty
}

func noise(r Runner, s runnerState, rng *RNG) float64 {
	width := (11 - float64(r.Stats.Stability)) * 0.085 * traitNoiseMultiplier(r, s)
	return rng.Range(-width, width)
}

func mistakeChance(r Runner, environment string) float64 {
	chance := 0.002 + float64(10-r.Stats.Stability)*0.001
	chance *= traitMistakeChanceMultiplier(r)
	if r.Strategy.Start == StartRisky {
		chance += 0.003
		if r.Trait == TraitCleanSteps {
			chance -= 0.0015
		}
	}
	if chance < 0 {
		return 0
	}
	if environment == EnvWetTrack {
		chance += 0.004 + float64(10-r.Stats.Stability)*0.0005
	}
	return chance
}

func mistakePenalty(r Runner) float64 {
	penalty := 1.8
	if r.Trait == TraitLightFeet {
		penalty *= 1.20
	}
	if r.Trait == TraitCleanSteps {
		penalty *= 0.65
	}
	return penalty
}

func traitEnergyBonus(r Runner) float64 {
	if r.Trait == TraitIronLungs {
		return 12
	}
	return 0
}

func traitSpeedBonus(r Runner, self runnerState, opponent runnerState) float64 {
	p := self.Position
	switch r.Trait {
	case TraitRocketStart:
		if p < 20 {
			return 2.2
		}
		if p < 30 {
			return 0.8
		}
		if p >= 70 {
			return -0.4
		}
	case TraitLateBeast:
		if p < 20 {
			return -0.5
		}
		if p >= 90 {
			return 2.6
		}
		if p >= 70 {
			return 1.8
		}
	case TraitSteadyMachine:
		return -0.35
	case TraitRhythmMaster:
		if p < 15 {
			return -0.3
		}
		if p >= 30 && p < 70 {
			return 0.9
		}
	case TraitLightFeet:
		return 0.45
	case TraitFrontRunner:
		if self.Position-opponent.Position >= 1 {
			return 0.6
		}
		if opponent.Position-self.Position > 3 {
			return -0.4
		}
	case TraitChaser:
		behind := opponent.Position - self.Position
		if behind > 5 {
			return 1.1
		}
		if behind >= 1 {
			return 0.7
		}
		if self.Position-opponent.Position > 2 {
			return -0.3
		}
	case TraitPhotoFinish:
		if p < 50 {
			return -0.25
		}
		if TrackLength-p <= 10 {
			bonus := 1.4
			if math.Abs(self.Position-opponent.Position) < 1 {
				bonus += 0.7
			}
			return bonus
		}
	}
	return 0
}

func traitNoiseMultiplier(r Runner, s runnerState) float64 {
	switch r.Trait {
	case TraitSteadyMachine:
		return 0.35
	case TraitGambler:
		return 2.2
	case TraitRhythmMaster:
		if s.Position >= 30 && s.Position < 70 {
			return 0.5
		}
	}
	return 1
}

func traitMistakeChanceMultiplier(r Runner) float64 {
	switch r.Trait {
	case TraitSteadyMachine:
		return 0.30
	case TraitCleanSteps:
		return 0.55
	}
	return 1
}

func traitEnergyCostMultiplier(r Runner, sprinting bool) float64 {
	if r.Trait == TraitIronLungs && sprinting {
		return 0.65
	}
	return 1
}

func finalKickBonus(r Runner) float64 {
	switch r.Trait {
	case TraitPhotoFinish:
		return 0.08
	default:
		return 0
	}
}

func environmentSpeedBonus(environment string, r Runner, s runnerState) float64 {
	switch environment {
	case EnvTailwind:
		if s.Position >= 70 {
			return 1.0
		}
		return 0.45
	case EnvWetTrack:
		if s.Position < 25 {
			return -0.45 + float64(r.Stats.Stability)*0.035
		}
		return -0.20 + float64(r.Stats.Stability)*0.015
	case EnvLoudCrowd:
		if s.Position >= 55 {
			return float64(r.Stats.Grit)*0.12 + 0.15
		}
		return float64(r.Stats.Reaction) * 0.035
	default:
		return 0
	}
}

func environmentEnergyMultiplier(environment string, s runnerState) float64 {
	switch environment {
	case EnvTailwind:
		if s.Position >= 70 {
			return 0.90
		}
	case EnvWetTrack:
		return 1.08
	case EnvLoudCrowd:
		if s.Position >= 55 {
			return 1.06
		}
	}
	return 1
}
