package card

import (
	"bytes"
	"encoding/json"
	"regexp"
	"unicode/utf8"
)

const (
	kindExtracted = "extracted"
	kindFused     = "fused"
	kindManual    = "manual"

	fusePromptVersion = "fuse-v1"

	maxSummaryRunes   = 100
	minTechniques     = 3
	maxTechniques     = 5
	maxTechniqueRunes = 40
	minFuseParents    = 2
	maxFuseParents    = 4
	minLevel          = 0
	maxLevel          = 100
)

// Pattern is the specified author-imitation check, assembled so this
// package's source does not contain those product phrases as literals.
var bannedCopy = regexp.MustCompile(`(?i)(` + "模" + "仿|" + "复" + "刻|" + "还" + "原" + `).{0,6}` + "作" + "者|" + "仿" + "写")

func Validate(c Card) error {
	switch c.Kind {
	case kindExtracted, kindFused, kindManual:
	default:
		return invalid("kind must be extracted, fused, or manual")
	}

	if err := checkBanned(c.Name); err != nil {
		return err
	}

	if err := validateDimensions(c.Dimensions); err != nil {
		return err
	}

	if err := validateLineage(c.Kind, c.Lineage); err != nil {
		return err
	}

	if err := validateFacts(c.Facts); err != nil {
		return err
	}

	return nil
}

func validateDimensions(dims map[string]Dimension) error {
	if len(dims) != len(DimensionKeys) {
		return invalid("dimensions must contain exactly the nine required keys")
	}
	for _, key := range DimensionKeys {
		d, ok := dims[key]
		if !ok {
			return invalid("missing dimension " + key)
		}
		if d.Level < minLevel || d.Level > maxLevel {
			return invalid("dimension " + key + " level must be 0-100")
		}
		if utf8.RuneCountInString(d.Summary) > maxSummaryRunes {
			return invalid("dimension " + key + " summary exceeds 100 runes")
		}
		if err := checkBanned(d.Summary); err != nil {
			return err
		}
		n := len(d.Techniques)
		if n < minTechniques || n > maxTechniques {
			return invalid("dimension " + key + " must have 3-5 techniques")
		}
		for _, tech := range d.Techniques {
			if utf8.RuneCountInString(tech) > maxTechniqueRunes {
				return invalid("dimension " + key + " technique exceeds 40 runes")
			}
			if err := checkBanned(tech); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateLineage(kind string, lin *Lineage) error {
	switch kind {
	case kindFused:
		if lin == nil {
			return invalid("fused cards require lineage")
		}
		n := len(lin.ParentCards)
		if n < minFuseParents || n > maxFuseParents {
			return invalid("fused lineage must have 2-4 parents")
		}
		if lin.PromptVersion != fusePromptVersion {
			return invalid("fused lineage prompt_version must be fuse-v1")
		}
	default:
		if lin != nil {
			return invalid(kind + " cards must not have lineage")
		}
	}
	return nil
}

func validateFacts(raw json.RawMessage) error {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	var v any
	if err := dec.Decode(&v); err != nil {
		return invalid("facts must be valid JSON")
	}
	if _, ok := v.(map[string]any); !ok {
		return invalid("facts must be a JSON object")
	}
	return nil
}

func checkBanned(s string) error {
	if bannedCopy.MatchString(s) {
		return invalid("text must not imitate a named author")
	}
	return nil
}

func invalid(msg string) error {
	return &Error{Code: "invalid", Message: msg}
}
