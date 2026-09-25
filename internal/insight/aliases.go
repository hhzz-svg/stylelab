package insight

import (
	"sort"
	"strings"
)

// Alias discovery: the prose calls 林远 "林师兄", "远儿", "小远" far more
// often than it writes 林远, and those mentions are invisible to the
// co-occurrence analysis until they are registered as aliases. Candidates
// come from how Chinese fiction forms such names -- surname plus a title,
// and pet forms of the given name -- and are counted in the prose with the
// same leftmost-longest matcher, so a candidate inside a longer known name
// is not counted. When two characters share a surname, "林师兄" belongs to
// whichever one's name keeps turning up in the same or the neighbouring
// paragraph.

// Titles that follow a surname to address or refer to someone.
var aliasTitles = []string{
	"师兄", "师弟", "师姐", "师妹", "师父", "师尊", "师叔", "师伯", "长老", "公子", "姑娘", "小姐",
	"兄", "大哥", "前辈", "道友", "掌门", "宗主", "少侠", "大侠", "夫人", "老爷", "少爷", "大人",
	"先生", "仙子", "真人", "道长",
}

// Two-character surnames; anything else is taken to be one character.
var compoundSurnames = []string{
	"欧阳", "司马", "上官", "诸葛", "慕容", "东方", "南宫", "独孤", "令狐", "西门", "公孙", "皇甫",
	"轩辕", "长孙", "宇文", "尉迟", "端木", "夏侯", "司徒", "澹台",
}

// Discovery thresholds.
const (
	MinAliasCount      = 2   // mentions before a candidate is worth showing
	MinAliasConfidence = 0.6 // below this share the owner is uncertain
	aliasExamples      = 3
	aliasSnippetRunes  = 16 // context either side of an example mention
)

// AliasSuggestion is a name the prose seems to use for an entity.
type AliasSuggestion struct {
	EntityID string `json:"entity_id"`
	Alias    string `json:"alias"`
	Count    int    `json:"count"`
	// Confidence is the owner's share of the context evidence: 1 when no
	// one else could be meant.
	Confidence float64 `json:"confidence"`
	// Ambiguous when another character is about as likely to be meant.
	Ambiguous bool     `json:"ambiguous"`
	Examples  []string `json:"examples"`
}

// splitName returns a Chinese name's surname and given name, or ok=false
// when it is too short or too long to be one.
func splitName(name string) (surname, given string, ok bool) {
	rs := []rune(strings.TrimSpace(name))
	if len(rs) < 2 || len(rs) > 4 {
		return "", "", false
	}
	for _, c := range compoundSurnames {
		if strings.HasPrefix(string(rs), c) && len(rs) >= 3 {
			return c, string(rs[2:]), true
		}
	}
	return string(rs[:1]), string(rs[1:]), true
}

// aliasCandidates lists the names the prose might use for someone called
// name.
func aliasCandidates(name string) []string {
	surname, given, ok := splitName(name)
	if !ok {
		return nil
	}
	var out []string
	for _, t := range aliasTitles {
		out = append(out, surname+t)
	}
	g := []rune(given)
	last := string(g[len(g)-1:])
	out = append(out, last+"儿", "小"+last, "阿"+last)
	if len(g) >= 2 {
		out = append(out, given)
	}
	return out
}

type aliasHit struct {
	chapter, unit int
	start         int
}

// DiscoverAliases proposes aliases for the characters from the prose.
// Units are paragraphs, chapter by chapter, in reading order.
func DiscoverAliases(characters []CoEntity, chapters []CoChapter) []AliasSuggestion {
	// Every name anyone already goes by, and who it belongs to.
	known := map[string]int{}
	for i, c := range characters {
		for _, n := range c.Names {
			if _, dup := known[n]; !dup {
				known[n] = i
			}
		}
	}
	// Candidate strings and the characters whose names generate them.
	owners := map[string][]int{}
	for i, c := range characters {
		if len(c.Names) == 0 {
			continue
		}
		for _, cand := range aliasCandidates(c.Names[0]) {
			if _, taken := known[cand]; taken {
				continue
			}
			if len(owners[cand]) == 0 || owners[cand][len(owners[cand])-1] != i {
				owners[cand] = append(owners[cand], i)
			}
		}
	}
	if len(owners) == 0 {
		return []AliasSuggestion{}
	}

	// One matcher over known names (entity = character index) and
	// candidates (entity = len(characters) + candidate index), so a
	// candidate inside a longer known name is not counted.
	cands := make([]string, 0, len(owners))
	for c := range owners {
		cands = append(cands, c)
	}
	sort.Strings(cands)
	var patterns []Pattern
	for n, i := range known {
		patterns = append(patterns, Pattern{Text: n, Entity: i})
	}
	sort.Slice(patterns, func(a, b int) bool { return patterns[a].Text < patterns[b].Text })
	for k, c := range cands {
		patterns = append(patterns, Pattern{Text: c, Entity: len(characters) + k})
	}
	m := NewMatcher(patterns)

	// Which characters are named (by a known name) in each unit, and where
	// each candidate occurs.
	present := make([][]map[int]bool, len(chapters))
	hits := make([][]aliasHit, len(cands))
	for ci, ch := range chapters {
		present[ci] = make([]map[int]bool, len(ch.Units))
		for ui, u := range ch.Units {
			present[ci][ui] = map[int]bool{}
			for _, mt := range m.FindAll(u) {
				if mt.Entity < len(characters) {
					present[ci][ui][mt.Entity] = true
				} else {
					k := mt.Entity - len(characters)
					hits[k] = append(hits[k], aliasHit{chapter: ci, unit: ui, start: mt.Start})
				}
			}
		}
	}
	near := func(h aliasHit, who int) bool {
		for d := -1; d <= 1; d++ {
			u := h.unit + d
			if u >= 0 && u < len(present[h.chapter]) && present[h.chapter][u][who] {
				return true
			}
		}
		return false
	}

	out := []AliasSuggestion{}
	for k, cand := range cands {
		hs := hits[k]
		if len(hs) < MinAliasCount {
			continue
		}
		who := owners[cand]
		best, confidence := who[0], 1.0
		if len(who) > 1 {
			// Evidence: mentions with that character named alongside.
			score := make([]int, len(who))
			total := 0
			for _, h := range hs {
				for j, w := range who {
					if near(h, w) {
						score[j]++
						total++
					}
				}
			}
			bi := 0
			for j := range who {
				if score[j] > score[bi] {
					bi = j
				}
			}
			best = who[bi]
			confidence = 0
			if total > 0 {
				confidence = float64(score[bi]) / float64(total)
			}
		}
		s := AliasSuggestion{
			EntityID:   characters[best].ID,
			Alias:      cand,
			Count:      len(hs),
			Confidence: confidence,
			Ambiguous:  confidence < MinAliasConfidence,
			Examples:   []string{},
		}
		for _, h := range hs {
			if len(s.Examples) == aliasExamples {
				break
			}
			s.Examples = append(s.Examples, snippet(chapters[h.chapter].Units[h.unit], h.start, len([]rune(cand))))
		}
		out = append(out, s)
	}
	sort.SliceStable(out, func(a, b int) bool {
		if out[a].Ambiguous != out[b].Ambiguous {
			return !out[a].Ambiguous
		}
		if out[a].Count != out[b].Count {
			return out[a].Count > out[b].Count
		}
		return out[a].Alias < out[b].Alias
	})
	return out
}

// snippet is the text around a mention, with … where it was cut.
func snippet(unit string, start, length int) string {
	rs := []rune(unit)
	from, to := start-aliasSnippetRunes, start+length+aliasSnippetRunes
	prefix, suffix := "…", "…"
	if from <= 0 {
		from, prefix = 0, ""
	}
	if to >= len(rs) {
		to, suffix = len(rs), ""
	}
	return prefix + string(rs[from:to]) + suffix
}
