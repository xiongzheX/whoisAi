package main

import (
	"fmt"
	"sort"

	"runner_race/internal/sumo"
)

type preset struct {
	label    string
	wrestler sumo.Wrestler
}

func main() {
	presets := []preset{
		{
			label: "力量型",
			wrestler: sumo.Wrestler{
				ID: "power", Name: "力量型", Trait: sumo.TraitBullRush, Style: sumo.StyleAggressive,
				Stats: sumo.Stats{Power: 10, Weight: 5, Balance: 5, Footwork: 4, Stamina: 6, Spirit: 6},
			},
		},
		{
			label: "稳守型",
			wrestler: sumo.Wrestler{
				ID: "guard", Name: "稳守型", Trait: sumo.TraitIronFeet, Style: sumo.StyleDefensive,
				Stats: sumo.Stats{Power: 4, Weight: 8, Balance: 9, Footwork: 4, Stamina: 6, Spirit: 5},
			},
		},
		{
			label: "灵巧型",
			wrestler: sumo.Wrestler{
				ID: "agile", Name: "灵巧型", Trait: sumo.TraitSoftStep, Style: sumo.StyleTrickster,
				Stats: sumo.Stats{Power: 4, Weight: 4, Balance: 8, Footwork: 10, Stamina: 5, Spirit: 5},
			},
		},
		{
			label: "气势型",
			wrestler: sumo.Wrestler{
				ID: "spirit", Name: "气势型", Trait: sumo.TraitBigRoar, Style: sumo.StyleAggressive,
				Stats: sumo.Stats{Power: 6, Weight: 5, Balance: 5, Footwork: 5, Stamina: 5, Spirit: 10},
			},
		},
		{
			label: "消耗型",
			wrestler: sumo.Wrestler{
				ID: "endurance", Name: "消耗型", Trait: sumo.TraitCalmBreath, Style: sumo.StyleBalanced,
				Stats: sumo.Stats{Power: 5, Weight: 8, Balance: 5, Footwork: 4, Stamina: 9, Spirit: 5},
			},
		},
		{
			label: "均衡型",
			wrestler: sumo.Wrestler{
				ID: "balanced", Name: "均衡型", Trait: sumo.TraitCounterGrip, Style: sumo.StyleBalanced,
				Stats: sumo.Stats{Power: 6, Weight: 6, Balance: 6, Footwork: 6, Stamina: 6, Spirit: 6},
			},
		},
	}

	const seeds = 240
	fmt.Printf("preset win rates over %d mirrored seeds per matchup\n\n", seeds)
	fmt.Printf("%-10s", "")
	for _, defender := range presets {
		fmt.Printf("%10s", defender.label)
	}
	fmt.Println("   avg")

	averages := make(map[string]float64, len(presets))
	for _, attacker := range presets {
		rowTotal := 0.0
		rowCount := 0
		fmt.Printf("%-10s", attacker.label)
		for _, defender := range presets {
			if attacker.wrestler.ID == defender.wrestler.ID {
				fmt.Printf("%10s", "-")
				continue
			}
			rate := mirroredWinRate(attacker.wrestler, defender.wrestler, seeds)
			rowTotal += rate
			rowCount++
			fmt.Printf("%9.1f%%", rate*100)
		}
		avg := rowTotal / float64(rowCount)
		averages[attacker.label] = avg
		fmt.Printf("%7.1f%%\n", avg*100)
	}

	fmt.Println("\naverage ranking")
	rows := make([]string, 0, len(averages))
	for label := range averages {
		rows = append(rows, label)
	}
	sort.Slice(rows, func(i, j int) bool {
		return averages[rows[i]] > averages[rows[j]]
	})
	for _, label := range rows {
		fmt.Printf("%-10s %.1f%%\n", label, averages[label]*100)
	}
}

func mirroredWinRate(a, b sumo.Wrestler, seeds int64) float64 {
	wins := 0
	total := 0
	for seed := int64(1); seed <= seeds; seed++ {
		result, err := sumo.Simulate(a, b, seed)
		if err != nil {
			panic(err)
		}
		if result.Winner == a.ID {
			wins++
		}
		total++

		result, err = sumo.Simulate(b, a, seed+seeds)
		if err != nil {
			panic(err)
		}
		if result.Winner == a.ID {
			wins++
		}
		total++
	}
	return float64(wins) / float64(total)
}
