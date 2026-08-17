package card_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"stylelab/internal/card"
)

func TestValidateAcceptsValidFixtures(t *testing.T) {
	for _, kind := range []string{"extracted", "fused", "manual"} {
		c := card.ValidFixture(kind)
		if err := card.Validate(c); err != nil {
			t.Fatalf("kind %s: unexpected error: %v", kind, err)
		}
	}
}

func TestValidateRejects(t *testing.T) {
	tests := []struct {
		name string
		mut  func(*card.Card)
	}{
		{
			name: "unknown kind",
			mut:  func(c *card.Card) { c.Kind = "blend" },
		},
		{
			name: "missing dimension key",
			mut: func(c *card.Card) {
				delete(c.Dimensions, "tension_hook")
			},
		},
		{
			name: "extra dimension key",
			mut: func(c *card.Card) {
				c.Dimensions["voice_color"] = c.Dimensions["sentence_rhythm"]
			},
		},
		{
			name: "level below 0",
			mut: func(c *card.Card) {
				setDim(c, "sentence_rhythm", func(d *card.Dimension) { d.Level = -1 })
			},
		},
		{
			name: "level above 100",
			mut: func(c *card.Card) {
				setDim(c, "scene_pacing", func(d *card.Dimension) { d.Level = 101 })
			},
		},
		{
			name: "summary longer than 100 runes",
			mut: func(c *card.Card) {
				setDim(c, "sensory_description", func(d *card.Dimension) {
					d.Summary = strings.Repeat("感", 101)
				})
			},
		},
		{
			name: "too few techniques",
			mut: func(c *card.Card) {
				setDim(c, "dialogue_density", func(d *card.Dimension) {
					d.Techniques = []string{"对白短", "少解释"}
				})
			},
		},
		{
			name: "too many techniques",
			mut: func(c *card.Card) {
				setDim(c, "dialogue_density", func(d *card.Dimension) {
					d.Techniques = []string{"对白短", "少解释", "留白", "打断", "叠句", "旁白"}
				})
			},
		},
		{
			name: "technique longer than 40 runes",
			mut: func(c *card.Card) {
				setDim(c, "lexical_texture", func(d *card.Dimension) {
					d.Techniques = []string{"短词", "口语", strings.Repeat("词", 41)}
				})
			},
		},
		{
			name: "name matches author-imitation pattern",
			mut:  func(c *card.Card) { c.Name = "模仿作者风格" },
		},
		{
			name: "summary matches author-imitation pattern",
			mut: func(c *card.Card) {
				setDim(c, "rhetoric_preference", func(d *card.Dimension) {
					d.Summary = "复刻那位作者的比喻密度"
				})
			},
		},
		{
			name: "technique contains 仿写",
			mut: func(c *card.Card) {
				setDim(c, "emotional_expression", func(d *card.Dimension) {
					d.Techniques = []string{"克制抒情", "仿写名家收束", "留白"}
				})
			},
		},
		{
			name: "fused missing lineage",
			mut: func(c *card.Card) {
				*c = card.ValidFixture("fused")
				c.Lineage = nil
			},
		},
		{
			name: "fused with one parent",
			mut: func(c *card.Card) {
				*c = card.ValidFixture("fused")
				c.Lineage.ParentCards = c.Lineage.ParentCards[:1]
			},
		},
		{
			name: "fused with five parents",
			mut: func(c *card.Card) {
				*c = card.ValidFixture("fused")
				extra := c.Lineage.ParentCards[0]
				c.Lineage.ParentCards = append(c.Lineage.ParentCards, extra, extra, extra)
			},
		},
		{
			name: "fused prompt_version not fuse-v1",
			mut: func(c *card.Card) {
				*c = card.ValidFixture("fused")
				c.Lineage.PromptVersion = "fuse-v0"
			},
		},
		{
			name: "extracted has lineage",
			mut: func(c *card.Card) {
				*c = card.ValidFixture("extracted")
				c.Lineage = &card.Lineage{
					ParentCards: []card.ParentRef{
						{CardID: "crd_aaaaaaaaaaaaaaaa", Version: 1},
						{CardID: "crd_bbbbbbbbbbbbbbbb", Version: 1},
					},
					PromptVersion: "fuse-v1",
				}
			},
		},
		{
			name: "manual has lineage",
			mut: func(c *card.Card) {
				*c = card.ValidFixture("manual")
				c.Lineage = &card.Lineage{
					ParentCards: []card.ParentRef{
						{CardID: "crd_aaaaaaaaaaaaaaaa", Version: 1},
						{CardID: "crd_bbbbbbbbbbbbbbbb", Version: 1},
					},
					PromptVersion: "fuse-v1",
				}
			},
		},
		{
			name: "facts is a JSON array",
			mut:  func(c *card.Card) { c.Facts = json.RawMessage(`[]`) },
		},
		{
			name: "facts is invalid JSON",
			mut:  func(c *card.Card) { c.Facts = json.RawMessage(`{`) },
		},
		{
			name: "facts is a JSON string",
			mut:  func(c *card.Card) { c.Facts = json.RawMessage(`"nope"`) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := card.ValidFixture("extracted")
			tt.mut(&c)
			err := card.Validate(c)
			if err == nil {
				t.Fatal("expected error")
			}
			var ve *card.Error
			if !errors.As(err, &ve) {
				t.Fatalf("want *card.Error, got %T %v", err, err)
			}
			if ve.Code != "invalid" {
				t.Fatalf("code %q, want invalid", ve.Code)
			}
		})
	}
}

func TestValidateAcceptsEmptyFactsAsObject(t *testing.T) {
	c := card.ValidFixture("manual")
	c.Facts = nil
	if err := card.Validate(c); err != nil {
		t.Fatalf("nil facts: %v", err)
	}
	c.Facts = json.RawMessage{}
	if err := card.Validate(c); err != nil {
		t.Fatalf("empty facts: %v", err)
	}
}

func TestValidateAcceptsBoundaryLengths(t *testing.T) {
	c := card.ValidFixture("extracted")
	setDim(&c, "sentence_rhythm", func(d *card.Dimension) {
		d.Level = 0
		d.Summary = strings.Repeat("句", 100)
		d.Techniques = []string{
			strings.Repeat("技", 40),
			"第二技法",
			"第三技法",
			"第四技法",
			"第五技法",
		}
	})
	setDim(&c, "tension_hook", func(d *card.Dimension) { d.Level = 100 })
	if utf8.RuneCountInString(c.Dimensions["sentence_rhythm"].Summary) != 100 {
		t.Fatal("fixture setup")
	}
	if err := card.Validate(c); err != nil {
		t.Fatalf("boundary card: %v", err)
	}
}

func TestPackageSourcesOmitBannedPhrases(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	banned := []string{"模仿作者", "复刻作者", "还原作者", "像某某写的", "仿写某某", "仿写"}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		body, err := os.ReadFile(filepath.Join(".", e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		text := string(body)
		for _, phrase := range banned {
			if strings.Contains(text, phrase) {
				t.Errorf("%s contains banned phrase %q", e.Name(), phrase)
			}
		}
	}
}

func setDim(c *card.Card, key string, fn func(*card.Dimension)) {
	d := c.Dimensions[key]
	fn(&d)
	c.Dimensions[key] = d
}
