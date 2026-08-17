package card

import "encoding/json"

var DimensionKeys = []string{
	"sentence_rhythm", "narrative_perspective", "dialogue_density",
	"sensory_description", "scene_pacing", "emotional_expression",
	"rhetoric_preference", "lexical_texture", "tension_hook",
}

type Dimension struct {
	Level      int      `json:"level"`
	Summary    string   `json:"summary"`
	Techniques []string `json:"techniques"`
}

type ParentRef struct {
	CardID  string         `json:"card_id"`
	Version int            `json:"version"`
	Dims    []string       `json:"dims"`
	Weights map[string]int `json:"weights"`
}

type Lineage struct {
	ParentCards   []ParentRef `json:"parent_cards"`
	PromptVersion string      `json:"prompt_version"`
}

type Card struct {
	ID           string               `json:"id"`
	ProjectID    string               `json:"project_id"`
	Name         string               `json:"name"`
	Kind         string               `json:"kind"`
	Version      int                  `json:"version"`
	Dimensions   map[string]Dimension `json:"dimensions"`
	Prohibitions []string             `json:"prohibitions"`
	Facts        json.RawMessage      `json:"facts"`
	Lineage      *Lineage             `json:"lineage"`
}

type Error struct {
	Code    string
	Message string
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Code == "" {
		return e.Message
	}
	if e.Message == "" {
		return e.Code
	}
	return e.Code + ": " + e.Message
}

// ValidFixture returns a card that satisfies Validate for kind
// extracted, fused, or manual.
func ValidFixture(kind string) Card {
	dims := make(map[string]Dimension, len(DimensionKeys))
	for _, key := range DimensionKeys {
		dims[key] = Dimension{
			Level:      50,
			Summary:    "节奏平稳，信息按场景推进。",
			Techniques: []string{"短句收束", "场景切换", "感官点到为止"},
		}
	}
	c := Card{
		ID:           "crd_0123456789abcdef",
		ProjectID:    "prj_0123456789abcdef",
		Name:         "节奏样本",
		Kind:         kind,
		Version:      1,
		Dimensions:   dims,
		Prohibitions: []string{"避免套话开场"},
		Facts:        json.RawMessage(`{}`),
	}
	if kind == "fused" {
		c.Lineage = &Lineage{
			ParentCards: []ParentRef{
				{
					CardID:  "crd_aaaaaaaaaaaaaaaa",
					Version: 1,
					Dims:    []string{"sentence_rhythm"},
					Weights: map[string]int{"sentence_rhythm": 60},
				},
				{
					CardID:  "crd_bbbbbbbbbbbbbbbb",
					Version: 1,
					Dims:    []string{"sentence_rhythm"},
					Weights: map[string]int{"sentence_rhythm": 40},
				},
			},
			PromptVersion: "fuse-v1",
		}
	}
	return c
}
