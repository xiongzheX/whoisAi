package main

import (
	"encoding/json"
	"fmt"
	"os"

	"runner_race/internal/runner"
)

func main() {
	a := runner.Runner{
		ID:    "runner_blue",
		Name:  "小蓝兔",
		Trait: runner.TraitRocketStart,
		Stats: runner.Stats{Burst: 8, Speed: 7, Stamina: 5, Stability: 6, Reaction: 7, Grit: 3},
		Strategy: runner.Strategy{
			Start:  runner.StartNormal,
			Middle: runner.MiddleSteady,
			Sprint: runner.SprintNormal,
		},
	}
	b := runner.Runner{
		ID:    "runner_red",
		Name:  "红豆",
		Trait: runner.TraitLateBeast,
		Stats: runner.Stats{Burst: 4, Speed: 7, Stamina: 8, Stability: 6, Reaction: 5, Grit: 6},
		Strategy: runner.Strategy{
			Start:  runner.StartSafe,
			Middle: runner.MiddleConserve,
			Sprint: runner.SprintLate,
		},
	}

	result, err := runner.Simulate(a, b, 20260623)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(result); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
