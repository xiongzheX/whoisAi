package runner

import (
	"errors"
	"fmt"
	"strings"
)

const (
	TrackLength = 100.0
	TickRate    = 20.0
	DT          = 1.0 / TickRate
	MaxSeconds  = 10.0
	MaxTicks    = int(MaxSeconds * TickRate)
)

const (
	StartSafe   = "safe"
	StartNormal = "normal"
	StartRisky  = "risky"

	MiddleConserve = "conserve"
	MiddleSteady   = "steady"
	MiddlePush     = "push"

	SprintEarly  = "early"
	SprintNormal = "normal"
	SprintLate   = "late"
)

const (
	TraitRocketStart   = "rocket_start"
	TraitLateBeast     = "late_beast"
	TraitSteadyMachine = "steady_machine"
	TraitBigHeart      = "big_heart"
	TraitGambler       = "gambler"
	TraitRhythmMaster  = "rhythm_master"
	TraitLightFeet     = "light_feet"
	TraitIronLungs     = "iron_lungs"
	TraitFrontRunner   = "front_runner"
	TraitChaser        = "chaser"
	TraitCleanSteps    = "clean_steps"
	TraitPhotoFinish   = "photo_finish"
)

const (
	ObstacleHurdle = "hurdle"
	ObstaclePuddle = "puddle"
	ObstacleCone   = "cone"
)

var AllTraits = []string{
	TraitRocketStart,
	TraitLateBeast,
	TraitSteadyMachine,
	TraitBigHeart,
	TraitGambler,
	TraitRhythmMaster,
	TraitLightFeet,
	TraitIronLungs,
	TraitFrontRunner,
	TraitChaser,
	TraitCleanSteps,
	TraitPhotoFinish,
}

type Stats struct {
	Burst     int `json:"burst"`
	Speed     int `json:"speed"`
	Stamina   int `json:"stamina"`
	Stability int `json:"stability"`
	Reaction  int `json:"reaction"`
	Grit      int `json:"grit"`
}

type Strategy struct {
	Start  string `json:"start"`
	Middle string `json:"middle"`
	Sprint string `json:"sprint"`
}

type Runner struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Stats    Stats    `json:"stats"`
	Trait    string   `json:"trait"`
	Strategy Strategy `json:"strategy"`
}

type runnerState struct {
	RunnerID      string
	Position      float64
	Speed         float64
	Energy        float64
	ReactionDelay float64
	NextObstacle  int
	CurveMask     uint8
	MistakeTicks  int
	GritUsed      bool
	GritTicks     int
	GamblerTicks  int
	GamblerBonus  float64
	SurgeUsed     bool
	SurgeTicks    int
	PressureUsed  bool
	PressureTicks int
	FinishTime    float64
}

type CourseObstacle struct {
	Position float64 `json:"position"`
	Kind     string  `json:"kind"`
}

type CourseCurve struct {
	Start     float64 `json:"start"`
	End       float64 `json:"end"`
	Direction string  `json:"direction"`
}

type CoursePathPoint struct {
	Meter float64 `json:"meter"`
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
}

type Course struct {
	Obstacles []CourseObstacle  `json:"obstacles"`
	Curves    []CourseCurve     `json:"curves"`
	Shape     string            `json:"shape"`
	Path      []CoursePathPoint `json:"path"`
}

type RunnerSnapshot struct {
	RunnerID string  `json:"runnerId"`
	Position float64 `json:"position"`
	Speed    float64 `json:"speed"`
	Energy   float64 `json:"energy"`
}

type ReplayFrame struct {
	Tick int            `json:"tick"`
	Time float64        `json:"time"`
	A    RunnerSnapshot `json:"a"`
	B    RunnerSnapshot `json:"b"`
}

type ReplayEvent struct {
	Tick      int     `json:"tick"`
	Time      float64 `json:"time"`
	Runner    string  `json:"runner"`
	Type      string  `json:"type"`
	Message   string  `json:"message"`
	Effect    string  `json:"effect,omitempty"`
	Intensity float64 `json:"intensity,omitempty"`
}

type MatchResult struct {
	MatchID     string        `json:"matchId"`
	Seed        int64         `json:"seed"`
	Environment string        `json:"environment"`
	Course      Course        `json:"course"`
	Winner      string        `json:"winner"`
	Reason      string        `json:"reason"`
	Frames      []ReplayFrame `json:"frames"`
	Events      []ReplayEvent `json:"events"`
}

func ValidateRunner(r Runner) error {
	if strings.TrimSpace(r.ID) == "" {
		return errors.New("runner id is required")
	}
	name := strings.TrimSpace(r.Name)
	if len([]rune(name)) < 1 || len([]rune(name)) > 20 {
		return errors.New("runner name must be 1-20 characters")
	}

	values := []int{
		r.Stats.Burst,
		r.Stats.Speed,
		r.Stats.Stamina,
		r.Stats.Stability,
		r.Stats.Reaction,
		r.Stats.Grit,
	}

	total := 0
	for _, v := range values {
		if v < 1 || v > 10 {
			return errors.New("each stat must be between 1 and 10")
		}
		total += v
	}
	if total > 36 {
		return fmt.Errorf("total stats must be <= 36, got %d", total)
	}
	if !validTrait(r.Trait) {
		return fmt.Errorf("unknown trait %q", r.Trait)
	}
	if !validChoice(r.Strategy.Start, []string{StartSafe, StartNormal, StartRisky}) {
		return fmt.Errorf("unknown start strategy %q", r.Strategy.Start)
	}
	if !validChoice(r.Strategy.Middle, []string{MiddleConserve, MiddleSteady, MiddlePush}) {
		return fmt.Errorf("unknown middle strategy %q", r.Strategy.Middle)
	}
	if !validChoice(r.Strategy.Sprint, []string{SprintEarly, SprintNormal, SprintLate}) {
		return fmt.Errorf("unknown sprint strategy %q", r.Strategy.Sprint)
	}
	return nil
}

func validTrait(trait string) bool {
	return validChoice(trait, AllTraits)
}

func validChoice(value string, choices []string) bool {
	for _, choice := range choices {
		if value == choice {
			return true
		}
	}
	return false
}
