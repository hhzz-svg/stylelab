package card_test

import (
	"encoding/json"
	"testing"

	"stylelab/internal/card"
)

func TestToSimulationProfileMapsSpecKeys(t *testing.T) {
	c := card.ValidFixture("extracted")
	c.Prohibitions = []string{"避免套话开场", "避免口号收束"}
	setDim(&c, "sentence_rhythm", func(d *card.Dimension) {
		d.Summary = "短句收束，段落常单句落地"
		d.Techniques = []string{"三段递进后急收", "对白后不接神态", "少用长定语"}
	})

	raw := card.ToSimulationProfile(c)
	var profile map[string]any
	if err := json.Unmarshal(raw, &profile); err != nil {
		t.Fatalf("json: %v body=%s", err, raw)
	}
	if profile["version"] != "simulation_profile.v1" {
		t.Fatalf("version: %v", profile["version"])
	}

	corpus, _ := profile["corpus"].(map[string]any)
	sources, ok := corpus["sources"].([]any)
	if !ok {
		t.Fatalf("corpus.sources missing: %v", profile["corpus"])
	}
	if len(sources) != 0 {
		t.Fatalf("corpus.sources want empty, got %v", sources)
	}

	syn, _ := profile["synthesis"].(map[string]any)
	if syn == nil {
		t.Fatalf("synthesis missing: %v", profile)
	}
	style, _ := syn["style"].(map[string]any)
	if style == nil {
		t.Fatalf("synthesis.style missing: %v", syn)
	}
	for _, key := range []string{
		"sentence_rhythm",
		"perspective",
		"narrative_voice",
		"prose_texture",
		"mood",
		"do_not_copy",
	} {
		if _, ok := style[key]; !ok {
			t.Fatalf("missing synthesis.style.%s in %v", key, style)
		}
	}
	got, _ := style["do_not_copy"].([]any)
	if len(got) != len(c.Prohibitions) {
		t.Fatalf("do_not_copy: %v want %v", style["do_not_copy"], c.Prohibitions)
	}
	for i, p := range c.Prohibitions {
		if got[i] != p {
			t.Fatalf("do_not_copy[%d]=%v want %s", i, got[i], p)
		}
	}
	if _, ok := syn["pacing_density"]; !ok {
		t.Fatal("missing synthesis.pacing_density")
	}
	if _, ok := syn["hook_design"]; !ok {
		t.Fatal("missing synthesis.hook_design")
	}
	if _, ok := profile["facts"]; ok {
		t.Fatal("facts must not be written to the upstream schema")
	}
}
