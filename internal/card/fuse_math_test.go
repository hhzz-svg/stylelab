package card_test

import (
	"strings"
	"testing"

	"stylelab/internal/card"
)

func TestBlendWeightedLevelAndFallbackDim(t *testing.T) {
	a := card.ValidFixture("extracted")
	a.ID = "crd_aaaaaaaaaaaaaaaa"
	a.Prohibitions = []string{"避免套话开场", "避免口号收束"}
	setLevel(&a, "sentence_rhythm", 80)
	setSummary(&a, "narrative_perspective", "A 视角")
	setLevel(&a, "narrative_perspective", 70)

	b := card.ValidFixture("extracted")
	b.ID = "crd_bbbbbbbbbbbbbbbb"
	b.Prohibitions = []string{"避免口号收束", "避免解释腔"}
	setLevel(&b, "sentence_rhythm", 20)
	setSummary(&b, "narrative_perspective", "B 视角")
	setLevel(&b, "narrative_perspective", 10)

	spec := []card.ParentRef{
		{
			CardID:  a.ID,
			Version: 1,
			Dims:    []string{"sentence_rhythm"},
			Weights: map[string]int{"sentence_rhythm": 60},
		},
		{
			CardID:  b.ID,
			Version: 1,
			Dims:    []string{"sentence_rhythm"},
			Weights: map[string]int{"sentence_rhythm": 40},
		},
	}

	dims, prohibitions, err := card.Blend([]card.Card{a, b}, spec)
	if err != nil {
		t.Fatalf("Blend: %v", err)
	}
	if len(dims) != len(card.DimensionKeys) {
		t.Fatalf("expected 9 dims, got %d", len(dims))
	}
	for _, key := range card.DimensionKeys {
		if _, ok := dims[key]; !ok {
			t.Fatalf("missing dim %s", key)
		}
	}
	if got := dims["sentence_rhythm"].Level; got != 56 {
		t.Fatalf("sentence_rhythm.level: got %d want 56", got)
	}
	np := dims["narrative_perspective"]
	if np.Level != 70 || np.Summary != "A 视角" {
		t.Fatalf("unselected dim should copy from A: %+v", np)
	}
	if len(prohibitions) != 3 {
		t.Fatalf("prohibitions: %v", prohibitions)
	}
	if prohibitions[0] != "避免套话开场" || prohibitions[1] != "避免口号收束" || prohibitions[2] != "避免解释腔" {
		t.Fatalf("prohibitions order: %v", prohibitions)
	}
}

func TestBlendFallbackTieUsesEarlierParent(t *testing.T) {
	a := card.ValidFixture("extracted")
	a.ID = "crd_aaaaaaaaaaaaaaaa"
	setLevel(&a, "tension_hook", 90)
	setSummary(&a, "tension_hook", "A hook")

	b := card.ValidFixture("extracted")
	b.ID = "crd_bbbbbbbbbbbbbbbb"
	setLevel(&b, "tension_hook", 20)
	setSummary(&b, "tension_hook", "B hook")

	spec := []card.ParentRef{
		{
			CardID:  a.ID,
			Version: 1,
			Dims:    []string{"sentence_rhythm"},
			Weights: map[string]int{"sentence_rhythm": 50},
		},
		{
			CardID:  b.ID,
			Version: 1,
			Dims:    []string{"sentence_rhythm"},
			Weights: map[string]int{"sentence_rhythm": 50},
		},
	}
	dims, _, err := card.Blend([]card.Card{a, b}, spec)
	if err != nil {
		t.Fatalf("Blend: %v", err)
	}
	if dims["tension_hook"].Summary != "A hook" || dims["tension_hook"].Level != 90 {
		t.Fatalf("tie should copy earlier parent A: %+v", dims["tension_hook"])
	}
}

func TestBlendRejectsParentCount(t *testing.T) {
	a := card.ValidFixture("extracted")
	a.ID = "crd_aaaaaaaaaaaaaaaa"
	_, _, err := card.Blend([]card.Card{a}, []card.ParentRef{{
		CardID: a.ID, Version: 1, Dims: []string{"sentence_rhythm"}, Weights: map[string]int{"sentence_rhythm": 100},
	}})
	if err == nil {
		t.Fatal("expected error for 1 parent")
	}

	parents := make([]card.Card, 5)
	spec := make([]card.ParentRef, 5)
	for i := 0; i < 5; i++ {
		c := card.ValidFixture("extracted")
		c.ID = "crd_" + strings.Repeat(string(rune('a'+i)), 16)
		parents[i] = c
		spec[i] = card.ParentRef{
			CardID:  c.ID,
			Version: 1,
			Dims:    []string{"sentence_rhythm"},
			Weights: map[string]int{"sentence_rhythm": 20},
		}
	}
	_, _, err = card.Blend(parents, spec)
	if err == nil {
		t.Fatal("expected error for 5 parents")
	}
}

func TestBlendRejectsWeightSumNot100(t *testing.T) {
	a := card.ValidFixture("extracted")
	a.ID = "crd_aaaaaaaaaaaaaaaa"
	b := card.ValidFixture("extracted")
	b.ID = "crd_bbbbbbbbbbbbbbbb"
	_, _, err := card.Blend([]card.Card{a, b}, []card.ParentRef{
		{CardID: a.ID, Version: 1, Dims: []string{"sentence_rhythm"}, Weights: map[string]int{"sentence_rhythm": 60}},
		{CardID: b.ID, Version: 1, Dims: []string{"sentence_rhythm"}, Weights: map[string]int{"sentence_rhythm": 30}},
	})
	if err == nil {
		t.Fatal("expected weight sum error")
	}
}

func TestBlendTechniquesWeightDescendingUniqueCapAndPad(t *testing.T) {
	a := card.ValidFixture("extracted")
	a.ID = "crd_aaaaaaaaaaaaaaaa"
	setTechs(&a, "sentence_rhythm", []string{"A1", "A2", "shared"})

	b := card.ValidFixture("extracted")
	b.ID = "crd_bbbbbbbbbbbbbbbb"
	setTechs(&b, "sentence_rhythm", []string{"B1", "B2", "B3", "B4", "shared"})

	spec := []card.ParentRef{
		{CardID: a.ID, Version: 1, Dims: []string{"sentence_rhythm"}, Weights: map[string]int{"sentence_rhythm": 30}},
		{CardID: b.ID, Version: 1, Dims: []string{"sentence_rhythm"}, Weights: map[string]int{"sentence_rhythm": 70}},
	}
	dims, _, err := card.Blend([]card.Card{a, b}, spec)
	if err != nil {
		t.Fatalf("Blend: %v", err)
	}
	got := dims["sentence_rhythm"].Techniques
	want := []string{"B1", "B2", "B3", "B4", "shared"}
	if len(got) != 5 {
		t.Fatalf("techniques len: %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("techniques: got %v want %v", got, want)
		}
	}

	c := card.ValidFixture("extracted")
	c.ID = "crd_cccccccccccccccc"
	setTechs(&c, "dialogue_density", []string{"only-c"})
	d := card.ValidFixture("extracted")
	d.ID = "crd_dddddddddddddddd"
	setTechs(&d, "dialogue_density", []string{"only-c"})
	dims, _, err = card.Blend([]card.Card{c, d}, []card.ParentRef{
		{CardID: c.ID, Version: 1, Dims: []string{"dialogue_density"}, Weights: map[string]int{"dialogue_density": 80}},
		{CardID: d.ID, Version: 1, Dims: []string{"dialogue_density"}, Weights: map[string]int{"dialogue_density": 20}},
	})
	if err != nil {
		t.Fatalf("pad Blend: %v", err)
	}
	pad := dims["dialogue_density"].Techniques
	if len(pad) < 3 {
		t.Fatalf("expected pad to 3 techniques, got %v", pad)
	}
	if pad[0] != "only-c" {
		t.Fatalf("pad techniques: %v", pad)
	}
}

func setLevel(c *card.Card, key string, level int) {
	d := c.Dimensions[key]
	d.Level = level
	c.Dimensions[key] = d
}

func setSummary(c *card.Card, key, summary string) {
	d := c.Dimensions[key]
	d.Summary = summary
	c.Dimensions[key] = d
}

func setTechs(c *card.Card, key string, techs []string) {
	d := c.Dimensions[key]
	d.Techniques = techs
	c.Dimensions[key] = d
}
