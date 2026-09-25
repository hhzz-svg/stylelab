package insight

import (
	"reflect"
	"strings"
	"testing"
)

func names(ms []Match, text string, entities []string) []string {
	rs := []rune(text)
	var out []string
	for _, m := range ms {
		out = append(out, entities[m.Entity]+":"+string(rs[m.Start:m.End]))
	}
	return out
}

func TestMatcherLongestWinsAndAliasesMap(t *testing.T) {
	entities := []string{"林远", "林远山", "苏晚"}
	m := NewMatcher([]Pattern{
		{Text: "林远", Entity: 0},
		{Text: "林师兄", Entity: 0}, // alias
		{Text: "林远山", Entity: 1},
		{Text: "苏晚", Entity: 2},
		{Text: "晚", Entity: 2}, // too short: dropped
	})
	text := "林远山见林远与苏晚同行，林师兄笑了。晚风起。"
	got := names(m.FindAll(text), text, entities)
	want := []string{"林远山:林远山", "林远:林远", "苏晚:苏晚", "林远:林师兄"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestMatcherOverlapsAndFailLinks(t *testing.T) {
	// Classic Aho-Corasick case: "he", "she", "hers" in "ushers".
	entities := []string{"he", "she", "hers", "his"}
	m := NewMatcher([]Pattern{{"he", 0}, {"she", 1}, {"hers", 2}, {"his", 3}})
	text := "ushers"
	// All hits are she(1-4), he(2-4), hers(2-6); leftmost wins "she", then
	// "hers" overlaps it and is dropped.
	if got := names(m.FindAll(text), text, entities); !reflect.DeepEqual(got, []string{"she:she"}) {
		t.Fatalf("got %v", got)
	}
	text = "hishers"
	if got := names(m.FindAll(text), text, entities); !reflect.DeepEqual(got, []string{"his:his", "hers:hers"}) {
		t.Fatalf("got %v", got)
	}
}

func TestMatcherOffsetsAreRunes(t *testing.T) {
	m := NewMatcher([]Pattern{{Text: "苏晚", Entity: 0}})
	got := m.FindAll("一二三苏晚")
	if len(got) != 1 || got[0].Start != 3 || got[0].End != 5 {
		t.Fatalf("got %+v", got)
	}
}

func TestMatcherSharedNameKeepsFirst(t *testing.T) {
	m := NewMatcher([]Pattern{{Text: "师兄", Entity: 0}, {Text: "师兄", Entity: 1}})
	if got := m.FindAll("师兄"); len(got) != 1 || got[0].Entity != 0 {
		t.Fatalf("got %+v", got)
	}
}

func TestMatcherEmpty(t *testing.T) {
	if got := NewMatcher(nil).FindAll("任何文字"); got != nil {
		t.Fatalf("got %v", got)
	}
}

// Brute force agrees with the automaton on a larger mixed text.
func TestMatcherAgreesWithBruteForce(t *testing.T) {
	pats := []string{"林远", "林远山", "远山", "苏晚", "晚风", "青云宗", "青云", "云宗主"}
	var ps []Pattern
	for i, p := range pats {
		ps = append(ps, Pattern{Text: p, Entity: i})
	}
	text := strings.Repeat("林远山下青云宗主苏晚风起林远青云", 7)
	rs := []rune(text)

	var all []Match
	for i := range rs {
		for e, p := range pats {
			pr := []rune(p)
			if i+len(pr) <= len(rs) && string(rs[i:i+len(pr)]) == p {
				all = append(all, Match{Entity: e, Start: i, End: i + len(pr)})
			}
		}
	}
	// leftmost-longest by hand
	var want []Match
	end := 0
	for i := 0; i < len(rs); i++ {
		best := -1
		for k, mt := range all {
			if mt.Start == i && mt.Start >= end && (best < 0 || mt.End > all[best].End) {
				best = k
			}
		}
		if best >= 0 {
			want = append(want, all[best])
			end = all[best].End
		}
	}
	if got := NewMatcher(ps).FindAll(text); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v\nwant %v", got, want)
	}
}
