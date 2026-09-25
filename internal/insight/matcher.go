package insight

import (
	"sort"
	"strings"
	"unicode/utf8"
)

// MinNameRunes is the shortest name or alias the matcher accepts. A
// one-character name ("林") would match inside every other word.
const MinNameRunes = 2

// Pattern is a name or alias standing for an entity.
type Pattern struct {
	Text   string
	Entity int
}

// Match is one occurrence of an entity's name, in rune offsets [Start, End).
type Match struct {
	Entity     int
	Start, End int
}

type acState struct {
	next map[rune]int
	fail int
	// out is the longest pattern ending here (-1 if none); dict links to
	// the nearest state down the fail chain that ends a pattern.
	out  int
	dict int
}

// Matcher finds entity names in text with the Aho-Corasick automaton: one
// pass over the text however many names there are. Overlapping hits are
// resolved leftmost-longest, so with both 林远 and 林远山 known, "林远山"
// is one mention of 林远山 and none of 林远.
type Matcher struct {
	states   []acState
	patterns []Pattern
	lengths  []int
}

// NewMatcher builds the automaton. Patterns shorter than MinNameRunes are
// dropped; when two entities share a name, the first one keeps it.
func NewMatcher(patterns []Pattern) *Matcher {
	m := &Matcher{states: []acState{{next: map[rune]int{}, out: -1, dict: -1}}}
	seen := map[string]bool{}
	for _, p := range patterns {
		text := strings.TrimSpace(p.Text)
		if utf8.RuneCountInString(text) < MinNameRunes || seen[text] {
			continue
		}
		seen[text] = true
		m.add(Pattern{Text: text, Entity: p.Entity})
	}
	m.link()
	return m
}

func (m *Matcher) add(p Pattern) {
	s := 0
	n := 0
	for _, r := range p.Text {
		nx, ok := m.states[s].next[r]
		if !ok {
			m.states = append(m.states, acState{next: map[rune]int{}, out: -1, dict: -1})
			nx = len(m.states) - 1
			m.states[s].next[r] = nx
		}
		s = nx
		n++
	}
	m.states[s].out = len(m.patterns)
	m.patterns = append(m.patterns, p)
	m.lengths = append(m.lengths, n)
}

// link computes failure and dictionary links breadth-first.
func (m *Matcher) link() {
	queue := []int{}
	for _, r := range sortedRunes(m.states[0].next) {
		child := m.states[0].next[r]
		m.states[child].fail = 0
		queue = append(queue, child)
	}
	for len(queue) > 0 {
		s := queue[0]
		queue = queue[1:]
		for _, r := range sortedRunes(m.states[s].next) {
			child := m.states[s].next[r]
			f := m.states[s].fail
			for f != 0 {
				if _, ok := m.states[f].next[r]; ok {
					break
				}
				f = m.states[f].fail
			}
			if nx, ok := m.states[f].next[r]; ok && nx != child {
				f = nx
			} else {
				f = 0
			}
			m.states[child].fail = f
			if m.states[f].out >= 0 {
				m.states[child].dict = f
			} else {
				m.states[child].dict = m.states[f].dict
			}
			queue = append(queue, child)
		}
	}
}

func sortedRunes(next map[rune]int) []rune {
	rs := make([]rune, 0, len(next))
	for r := range next {
		rs = append(rs, r)
	}
	sort.Slice(rs, func(i, j int) bool { return rs[i] < rs[j] })
	return rs
}

// FindAll returns the non-overlapping mentions in text, leftmost-longest,
// in order.
func (m *Matcher) FindAll(text string) []Match {
	if len(m.patterns) == 0 {
		return nil
	}
	var all []Match
	s, pos := 0, 0
	for _, r := range text {
		for {
			if nx, ok := m.states[s].next[r]; ok {
				s = nx
				break
			}
			if s == 0 {
				break
			}
			s = m.states[s].fail
		}
		pos++
		for o := s; o > 0; {
			if p := m.states[o].out; p >= 0 {
				all = append(all, Match{Entity: m.patterns[p].Entity, Start: pos - m.lengths[p], End: pos})
			}
			o = m.states[o].dict
			if o < 0 {
				break
			}
		}
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].Start != all[j].Start {
			return all[i].Start < all[j].Start
		}
		return all[i].End > all[j].End
	})
	out := all[:0]
	end := 0
	for _, mt := range all {
		if mt.Start >= end {
			out = append(out, mt)
			end = mt.End
		}
	}
	return out
}
