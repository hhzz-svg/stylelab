package card

import (
	"fmt"
	"math"
	"sort"
)

const (
	minBlendParents = 2
	maxBlendParents = 4
)

func Blend(parents []Card, spec []ParentRef) (map[string]Dimension, []string, error) {
	if n := len(spec); n < minBlendParents || n > maxBlendParents {
		return nil, nil, invalid("fuse requires 2-4 parents")
	}

	byID := make(map[string]Card, len(parents))
	for _, p := range parents {
		byID[p.ID] = p
	}

	ordered := make([]Card, 0, len(spec))
	for _, ref := range spec {
		p, ok := byID[ref.CardID]
		if !ok {
			return nil, nil, invalid("parent card not found: " + ref.CardID)
		}
		if err := validateParentRef(ref); err != nil {
			return nil, nil, err
		}
		ordered = append(ordered, p)
	}

	selected := make(map[string][]parentWeight, len(DimensionKeys))
	totals := make([]int, len(spec))
	for i, ref := range spec {
		for _, key := range ref.Dims {
			w, ok := ref.Weights[key]
			if !ok {
				return nil, nil, invalid("missing weight for " + key)
			}
			if w < minLevel || w > maxLevel {
				return nil, nil, invalid("weight for " + key + " must be 0-100")
			}
			selected[key] = append(selected[key], parentWeight{index: i, weight: w})
			totals[i] += w
		}
	}

	for key, contribs := range selected {
		sum := 0
		for _, c := range contribs {
			sum += c.weight
		}
		if sum != 100 {
			return nil, nil, invalid(fmt.Sprintf("weights for %s must sum to 100", key))
		}
	}

	fallback := 0
	for i := 1; i < len(spec); i++ {
		if totals[i] > totals[fallback] {
			fallback = i
		}
	}

	out := make(map[string]Dimension, len(DimensionKeys))
	for _, key := range DimensionKeys {
		contribs, ok := selected[key]
		if !ok {
			out[key] = copyDimension(ordered[fallback].Dimensions[key])
			continue
		}
		dim, err := blendSelected(ordered, contribs, key)
		if err != nil {
			return nil, nil, err
		}
		out[key] = dim
	}

	return out, unionProhibitions(ordered), nil
}

type parentWeight struct {
	index  int
	weight int
}

func validateParentRef(ref ParentRef) error {
	seen := make(map[string]struct{}, len(ref.Dims))
	for _, key := range ref.Dims {
		if !knownDimension(key) {
			return invalid("unknown dimension " + key)
		}
		if _, dup := seen[key]; dup {
			return invalid("duplicate dimension " + key)
		}
		seen[key] = struct{}{}
	}
	return nil
}

func knownDimension(key string) bool {
	for _, k := range DimensionKeys {
		if k == key {
			return true
		}
	}
	return false
}

func blendSelected(parents []Card, contribs []parentWeight, key string) (Dimension, error) {
	sort.SliceStable(contribs, func(i, j int) bool {
		if contribs[i].weight != contribs[j].weight {
			return contribs[i].weight > contribs[j].weight
		}
		return contribs[i].index < contribs[j].index
	})

	var weighted float64
	for _, c := range contribs {
		dim, ok := parents[c.index].Dimensions[key]
		if !ok {
			return Dimension{}, invalid("parent missing dimension " + key)
		}
		weighted += float64(dim.Level) * float64(c.weight) / 100.0
	}

	top := parents[contribs[0].index].Dimensions[key]
	return Dimension{
		Level:      int(math.Round(weighted)),
		Summary:    top.Summary,
		Techniques: blendTechniques(parents, contribs, key),
	}, nil
}

func blendTechniques(parents []Card, contribs []parentWeight, key string) []string {
	seen := make(map[string]struct{})
	out := make([]string, 0, maxTechniques)
	for _, c := range contribs {
		for _, tech := range parents[c.index].Dimensions[key].Techniques {
			if tech == "" {
				continue
			}
			if _, ok := seen[tech]; ok {
				continue
			}
			seen[tech] = struct{}{}
			out = append(out, tech)
			if len(out) == maxTechniques {
				return out
			}
		}
	}
	if len(out) >= minTechniques || len(contribs) == 0 {
		return out
	}
	src := parents[contribs[0].index].Dimensions[key].Techniques
	for _, tech := range src {
		if len(out) >= minTechniques {
			break
		}
		if tech == "" {
			continue
		}
		if _, ok := seen[tech]; ok {
			continue
		}
		seen[tech] = struct{}{}
		out = append(out, tech)
	}
	i := 0
	for len(out) < minTechniques && len(src) > 0 {
		tech := src[i%len(src)]
		if tech != "" {
			out = append(out, tech)
		}
		i++
		if i > 32 {
			break
		}
	}
	return out
}

func unionProhibitions(parents []Card) []string {
	seen := make(map[string]struct{})
	out := make([]string, 0)
	for _, p := range parents {
		for _, item := range p.Prohibitions {
			if item == "" {
				continue
			}
			if _, ok := seen[item]; ok {
				continue
			}
			seen[item] = struct{}{}
			out = append(out, item)
		}
	}
	return out
}

func copyDimension(d Dimension) Dimension {
	techs := make([]string, len(d.Techniques))
	copy(techs, d.Techniques)
	d.Techniques = techs
	return d
}
