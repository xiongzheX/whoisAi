// Package duel exposes the runner_race simulation engines to the social-game
// host without leaking the submodule's internal packages.
package duel

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"runner_race/internal/runner"
	"runner_race/internal/sumo"
)

const (
	GameBeanSprint   = "bean-sprint"
	GameDumplingSumo = "dumpling-sumo"
)

type PlayerInput struct {
	ID     string
	Name   string
	Config json.RawMessage
}

type Result struct {
	GameID  string          `json:"gameId"`
	Players json.RawMessage `json:"players"`
	Match   json.RawMessage `json:"match"`
}

type runnerConfig struct {
	Name     string          `json:"name"`
	Stats    runner.Stats    `json:"stats"`
	Strategy runner.Strategy `json:"strategy"`
}

type sumoConfig struct {
	Name  string     `json:"name"`
	Stats sumo.Stats `json:"stats"`
	Style string     `json:"style"`
}

func ValidateConfig(gameID, playerName string, config json.RawMessage) error {
	input := PlayerInput{ID: "validation-player", Name: playerName, Config: config}
	switch gameID {
	case GameBeanSprint:
		_, err := buildRunner(input, 1)
		return err
	case GameDumplingSumo:
		_, err := buildWrestler(input, 1)
		return err
	default:
		return fmt.Errorf("unsupported duel game %q", gameID)
	}
}

func Simulate(gameID string, players []PlayerInput, seed int64) (Result, error) {
	if len(players) != 2 {
		return Result{}, fmt.Errorf("duel requires exactly 2 players, got %d", len(players))
	}
	if players[0].ID == players[1].ID {
		return Result{}, errors.New("duel players must be different")
	}

	switch gameID {
	case GameBeanSprint:
		return simulateRunners(players, seed)
	case GameDumplingSumo:
		return simulateSumo(players, seed)
	default:
		return Result{}, fmt.Errorf("unsupported duel game %q", gameID)
	}
}

func simulateRunners(inputs []PlayerInput, seed int64) (Result, error) {
	a, err := buildRunner(inputs[0], seed+101)
	if err != nil {
		return Result{}, fmt.Errorf("player A: %w", err)
	}
	b, err := buildRunner(inputs[1], seed+202)
	if err != nil {
		return Result{}, fmt.Errorf("player B: %w", err)
	}
	match, err := runner.Simulate(a, b, seed)
	if err != nil {
		return Result{}, err
	}
	return marshalResult(GameBeanSprint, []runner.Runner{a, b}, match)
}

func simulateSumo(inputs []PlayerInput, seed int64) (Result, error) {
	a, err := buildWrestler(inputs[0], seed+303)
	if err != nil {
		return Result{}, fmt.Errorf("player A: %w", err)
	}
	b, err := buildWrestler(inputs[1], seed+404)
	if err != nil {
		return Result{}, fmt.Errorf("player B: %w", err)
	}
	match, err := sumo.Simulate(a, b, seed)
	if err != nil {
		return Result{}, err
	}
	return marshalResult(GameDumplingSumo, []sumo.Wrestler{a, b}, match)
}

func buildRunner(input PlayerInput, traitSeed int64) (runner.Runner, error) {
	var config runnerConfig
	if err := json.Unmarshal(input.Config, &config); err != nil {
		return runner.Runner{}, errors.New("invalid runner config")
	}
	name := contestantName(config.Name, input.Name)
	created := runner.Runner{
		ID:       input.ID,
		Name:     name,
		Stats:    config.Stats,
		Trait:    runner.RandomTrait(runner.NewRNG(traitSeed)),
		Strategy: config.Strategy,
	}
	if err := runner.ValidateRunner(created); err != nil {
		return runner.Runner{}, err
	}
	return created, nil
}

func buildWrestler(input PlayerInput, traitSeed int64) (sumo.Wrestler, error) {
	var config sumoConfig
	if err := json.Unmarshal(input.Config, &config); err != nil {
		return sumo.Wrestler{}, errors.New("invalid sumo config")
	}
	name := contestantName(config.Name, input.Name)
	created := sumo.Wrestler{
		ID:    input.ID,
		Name:  name,
		Stats: config.Stats,
		Trait: sumo.RandomTrait(sumo.NewRNG(traitSeed)),
		Style: config.Style,
	}
	if err := sumo.ValidateWrestler(created); err != nil {
		return sumo.Wrestler{}, err
	}
	return created, nil
}

func contestantName(configured, fallback string) string {
	if name := strings.TrimSpace(configured); name != "" {
		return name
	}
	return strings.TrimSpace(fallback)
}

func marshalResult(gameID string, players, match any) (Result, error) {
	encodedPlayers, err := json.Marshal(players)
	if err != nil {
		return Result{}, fmt.Errorf("encode players: %w", err)
	}
	encodedMatch, err := json.Marshal(match)
	if err != nil {
		return Result{}, fmt.Errorf("encode match: %w", err)
	}
	return Result{GameID: gameID, Players: encodedPlayers, Match: encodedMatch}, nil
}
