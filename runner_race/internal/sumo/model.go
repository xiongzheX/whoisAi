package sumo

import (
	"errors"
	"fmt"
	"strings"
)

const (
	RingRadius = 8.2
	TickRate   = 20.0
	DT         = 1.0 / TickRate
	MaxSeconds = 5.0
	MaxTicks   = int(MaxSeconds * TickRate)
)

const (
	StyleBalanced   = "balanced"
	StyleAggressive = "aggressive"
	StyleDefensive  = "defensive"
	StyleTrickster  = "trickster"
)

const (
	TraitIronFeet    = "iron_feet"
	TraitBullRush    = "bull_rush"
	TraitLowCenter   = "low_center"
	TraitCounterGrip = "counter_grip"
	TraitSoftStep    = "soft_step"
	TraitBigRoar     = "big_roar"
	TraitLuckyBelly  = "lucky_belly"
	TraitCalmBreath  = "calm_breath"
)

const (
	ArchetypePower     = "power"
	ArchetypeGuard     = "guard"
	ArchetypeAgile     = "agile"
	ArchetypeSpirit    = "spirit"
	ArchetypeEndurance = "endurance"
	ArchetypeBalanced  = "balanced"
)

var AllTraits = []string{
	TraitIronFeet,
	TraitBullRush,
	TraitLowCenter,
	TraitCounterGrip,
	TraitSoftStep,
	TraitBigRoar,
	TraitLuckyBelly,
	TraitCalmBreath,
}

type Stats struct {
	Power    int `json:"power"`
	Weight   int `json:"weight"`
	Balance  int `json:"balance"`
	Footwork int `json:"footwork"`
	Stamina  int `json:"stamina"`
	Spirit   int `json:"spirit"`
}

type Wrestler struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Stats Stats  `json:"stats"`
	Trait string `json:"trait"`
	Style string `json:"style"`
}

type wrestlerState struct {
	WrestlerID    string
	X             float64
	Y             float64
	VX            float64
	VY            float64
	Energy        float64
	StumbleTicks  int
	FocusTicks    int
	CounterTicks  int
	LockedTicks   int
	BreakerTicks  int
	DrainTicks    int
	RallyTicks    int
	LastCounterAt int
	LastRallyAt   int
	FinishReason  string
}

type WrestlerSnapshot struct {
	WrestlerID string  `json:"wrestlerId"`
	X          float64 `json:"x"`
	Y          float64 `json:"y"`
	Energy     float64 `json:"energy"`
	Stumbling  bool    `json:"stumbling"`
}

type ReplayFrame struct {
	Tick int              `json:"tick"`
	Time float64          `json:"time"`
	A    WrestlerSnapshot `json:"a"`
	B    WrestlerSnapshot `json:"b"`
}

type ReplayEvent struct {
	Tick     int     `json:"tick"`
	Time     float64 `json:"time"`
	Wrestler string  `json:"wrestler"`
	Type     string  `json:"type"`
	Message  string  `json:"message"`
}

type MatchResult struct {
	MatchID     string        `json:"matchId"`
	Seed        int64         `json:"seed"`
	Environment string        `json:"environment"`
	ArchetypeA  string        `json:"archetypeA"`
	ArchetypeB  string        `json:"archetypeB"`
	Winner      string        `json:"winner"`
	Reason      string        `json:"reason"`
	Frames      []ReplayFrame `json:"frames"`
	Events      []ReplayEvent `json:"events"`
}

func ValidateWrestler(w Wrestler) error {
	if strings.TrimSpace(w.ID) == "" {
		return errors.New("wrestler id is required")
	}
	if n := len([]rune(strings.TrimSpace(w.Name))); n < 1 || n > 20 {
		return errors.New("wrestler name must be 1-20 characters")
	}
	values := []int{w.Stats.Power, w.Stats.Weight, w.Stats.Balance, w.Stats.Footwork, w.Stats.Stamina, w.Stats.Spirit}
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
	if !validChoice(w.Trait, AllTraits) {
		return fmt.Errorf("unknown trait %q", w.Trait)
	}
	if !validChoice(w.Style, []string{StyleBalanced, StyleAggressive, StyleDefensive, StyleTrickster}) {
		return fmt.Errorf("unknown style %q", w.Style)
	}
	return nil
}

func validChoice(value string, choices []string) bool {
	for _, choice := range choices {
		if value == choice {
			return true
		}
	}
	return false
}
