package card

import "encoding/json"

const simulationProfileVersion = "simulation_profile.v1"

// ToSimulationProfile maps a style card to an ainovel-cli compatible
// simulation_profile.json payload. Facts are not exported.
func ToSimulationProfile(c Card) []byte {
	sr := c.Dimensions["sentence_rhythm"]
	np := c.Dimensions["narrative_perspective"]
	dd := c.Dimensions["dialogue_density"]
	sp := c.Dimensions["scene_pacing"]
	ee := c.Dimensions["emotional_expression"]
	rp := c.Dimensions["rhetoric_preference"]
	lt := c.Dimensions["lexical_texture"]
	th := c.Dimensions["tension_hook"]

	doNotCopy := c.Prohibitions
	if doNotCopy == nil {
		doNotCopy = []string{}
	}

	profile := map[string]any{
		"version":        simulationProfileVersion,
		"corpus":         map[string]any{"sources": []any{}},
		"source_reports": []any{},
		"synthesis": map[string]any{
			"style": map[string]any{
				"sentence_rhythm": dimLines(sr),
				"perspective":     dimLines(np),
				"narrative_voice": dimLines(np),
				"prose_texture":   append(dimLines(lt), dimLines(rp)...),
				"mood":            dimLines(ee),
				"do_not_copy":     doNotCopy,
			},
			"pacing_density": map[string]any{
				"scene_density":         dimLines(sp),
				"dialogue_action_ratio": dimLines(dd),
			},
			"hook_design": map[string]any{
				"hook_types": dimLines(th),
			},
		},
	}
	raw, err := json.Marshal(profile)
	if err != nil {
		return []byte(`{"version":"simulation_profile.v1","corpus":{"sources":[]},"synthesis":{}}`)
	}
	return raw
}

func dimLines(d Dimension) []string {
	out := make([]string, 0, 1+len(d.Techniques))
	if d.Summary != "" {
		out = append(out, d.Summary)
	}
	for _, t := range d.Techniques {
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}
