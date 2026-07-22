package platform

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

type Registry struct {
	games map[string]GameDefinition
}

func NewRegistry(definitions []GameDefinition) (*Registry, error) {
	registry := &Registry{games: make(map[string]GameDefinition, len(definitions))}
	for _, definition := range definitions {
		if err := validateGameDefinition(definition); err != nil {
			return nil, err
		}
		if _, exists := registry.games[definition.ID]; exists {
			return nil, fmt.Errorf("duplicate game id %q", definition.ID)
		}
		definition.Tags = append([]string(nil), definition.Tags...)
		definition.Manifest = append([]byte(nil), definition.Manifest...)
		registry.games[definition.ID] = definition
	}
	return registry, nil
}

func DefaultRegistry() *Registry {
	registry, err := NewRegistry([]GameDefinition{
		{
			ID:          "who-is-ai",
			Slug:        "who-is-ai",
			Name:        "谁是 AI",
			Description: "在 AI 干扰中追踪组队、发言和任务证据。",
			Status:      GameActive,
			MinPlayers:  5,
			MaxPlayers:  8,
			SupportsAI:  true,
			Route:       "/#who-is-ai-room",
			Tags:        []string{"社交推理", "隐藏身份", "AI"},
			SortOrder:   10,
		},
		{
			ID:          "bean-sprint",
			Slug:        "bean-sprint",
			Name:        "豆豆百米赛",
			Description: "两人各自调配选手，在同一条随机赛道上冲线。",
			Status:      GameActive,
			MinPlayers:  2,
			MaxPlayers:  2,
			SupportsAI:  false,
			Route:       "/games/bean-sprint/",
			Tags:        []string{"双人", "轻策略", "竞速"},
			SortOrder:   20,
		},
		{
			ID:          "dumpling-sumo",
			Slug:        "dumpling-sumo",
			Name:        "团子相扑",
			Description: "两名力士配好属性与风格，同场看完五秒攻防。",
			Status:      GameActive,
			MinPlayers:  2,
			MaxPlayers:  2,
			SupportsAI:  false,
			Route:       "/games/dumpling-sumo/",
			Tags:        []string{"双人", "配点", "对抗"},
			SortOrder:   30,
		},
	})
	if err != nil {
		panic(err)
	}
	return registry
}

func (r *Registry) Game(id string) (GameDefinition, bool) {
	definition, ok := r.games[id]
	if !ok {
		return GameDefinition{}, false
	}
	return cloneGameDefinition(definition), true
}

func (r *Registry) Games() []GameDefinition {
	definitions := make([]GameDefinition, 0, len(r.games))
	for _, definition := range r.games {
		definitions = append(definitions, cloneGameDefinition(definition))
	}
	sort.Slice(definitions, func(i, j int) bool {
		if definitions[i].SortOrder != definitions[j].SortOrder {
			return definitions[i].SortOrder < definitions[j].SortOrder
		}
		return definitions[i].ID < definitions[j].ID
	})
	return definitions
}

func validateGameDefinition(definition GameDefinition) error {
	if strings.TrimSpace(definition.ID) == "" {
		return errors.New("game id is required")
	}
	if strings.TrimSpace(definition.Slug) == "" {
		return fmt.Errorf("game %q slug is required", definition.ID)
	}
	if strings.TrimSpace(definition.Name) == "" {
		return fmt.Errorf("game %q name is required", definition.ID)
	}
	if definition.Status != GameActive && definition.Status != GameComingSoon && definition.Status != GameDisabled {
		return fmt.Errorf("game %q has invalid status %q", definition.ID, definition.Status)
	}
	if definition.MinPlayers <= 0 || definition.MaxPlayers < definition.MinPlayers {
		return fmt.Errorf("game %q has invalid player range %d-%d", definition.ID, definition.MinPlayers, definition.MaxPlayers)
	}
	return nil
}

func cloneGameDefinition(definition GameDefinition) GameDefinition {
	definition.Tags = append([]string(nil), definition.Tags...)
	definition.Manifest = append([]byte(nil), definition.Manifest...)
	return definition
}
