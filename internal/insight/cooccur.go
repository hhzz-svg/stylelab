package insight

import (
	"math"
	"sort"
)

// CoEntity is something to look for in the text: a node and its names.
type CoEntity struct {
	ID    string
	Names []string // name first, then aliases
}

// CoChapter is a written chapter split into units -- paragraphs, or scenes
// where a chapter has been segmented. Two entities co-occur when they are
// mentioned in the same unit.
type CoChapter struct {
	Seq   int
	Units []string
}

// Appearance is one entity's presence across the book.
type Appearance struct {
	ID string `json:"id"`
	// PerChapter counts mentions, aligned with Cooccurrence.Chapters.
	PerChapter []int `json:"per_chapter"`
	Total      int   `json:"total"`
	Units      int   `json:"units"` // units mentioning it
	FirstSeq   int   `json:"first_seq"`
	LastSeq    int   `json:"last_seq"`
}

// Pair is how often two entities share a unit.
type Pair struct {
	A     string `json:"a"`
	B     string `json:"b"`
	Count int    `json:"count"`
	// Jaccard: shared units over units mentioning either.
	Jaccard float64 `json:"jaccard"`
	// PMI: log2 of how much more often they share a unit than chance
	// would put them together. High even for minor pairs that always meet.
	PMI float64 `json:"pmi"`
}

// Absence is an entity mentioned often enough to matter that has not
// appeared for a while.
type Absence struct {
	ID            string `json:"id"`
	LastSeq       int    `json:"last_seq"`
	ChaptersSince int    `json:"chapters_since"`
}

// Cooccurrence is the whole analysis.
type Cooccurrence struct {
	Chapters    []int        `json:"chapters"`
	Units       int          `json:"units"`
	Appearances []Appearance `json:"appearances"` // mentioned at least once, by first appearance
	Pairs       []Pair       `json:"pairs"`       // strongest first, all with MinPairCount or more
	Absent      []Absence    `json:"absent"`
}

// Thresholds for the derived lists.
const (
	MinPairCount       = 2  // shared units before a pair counts; one meeting is noise
	MinAbsentMentions  = 5  // mentions before a disappearance matters
	DefaultAbsentAfter = 10 // chapters of silence that count as gone
)

// Cooccur counts every entity's mentions chapter by chapter and every
// pair's shared units, and lists who has been gone for absentAfter or more
// written chapters.
func Cooccur(entities []CoEntity, chapters []CoChapter, absentAfter int) Cooccurrence {
	if absentAfter < 1 {
		absentAfter = DefaultAbsentAfter
	}
	var patterns []Pattern
	for i, e := range entities {
		for _, n := range e.Names {
			patterns = append(patterns, Pattern{Text: n, Entity: i})
		}
	}
	m := NewMatcher(patterns)

	chs := append([]CoChapter(nil), chapters...)
	sort.SliceStable(chs, func(a, b int) bool { return chs[a].Seq < chs[b].Seq })

	out := Cooccurrence{Chapters: make([]int, len(chs)), Appearances: []Appearance{}, Pairs: []Pair{}, Absent: []Absence{}}
	per := make([][]int, len(entities))
	for i := range per {
		per[i] = make([]int, len(chs))
	}
	unitsOf := make([]int, len(entities))
	firstCh := make([]int, len(entities))
	lastCh := make([]int, len(entities))
	for i := range firstCh {
		firstCh[i] = -1
	}
	shared := map[[2]int]int{}

	for ci, ch := range chs {
		out.Chapters[ci] = ch.Seq
		for _, unit := range ch.Units {
			out.Units++
			present := map[int]bool{}
			for _, mt := range m.FindAll(unit) {
				per[mt.Entity][ci]++
				present[mt.Entity] = true
			}
			ids := make([]int, 0, len(present))
			for e := range present {
				ids = append(ids, e)
				unitsOf[e]++
				if firstCh[e] < 0 {
					firstCh[e] = ci
				}
				lastCh[e] = ci
			}
			sort.Ints(ids)
			for a := 0; a < len(ids); a++ {
				for b := a + 1; b < len(ids); b++ {
					shared[[2]int{ids[a], ids[b]}]++
				}
			}
		}
	}

	for i, e := range entities {
		if unitsOf[i] == 0 {
			continue
		}
		total := 0
		for _, c := range per[i] {
			total += c
		}
		out.Appearances = append(out.Appearances, Appearance{
			ID: e.ID, PerChapter: per[i], Total: total, Units: unitsOf[i],
			FirstSeq: chs[firstCh[i]].Seq, LastSeq: chs[lastCh[i]].Seq,
		})
		since := len(chs) - 1 - lastCh[i]
		if total >= MinAbsentMentions && since >= absentAfter {
			out.Absent = append(out.Absent, Absence{ID: e.ID, LastSeq: chs[lastCh[i]].Seq, ChaptersSince: since})
		}
	}
	sort.SliceStable(out.Appearances, func(a, b int) bool {
		x, y := out.Appearances[a], out.Appearances[b]
		if x.FirstSeq != y.FirstSeq {
			return x.FirstSeq < y.FirstSeq
		}
		if x.Total != y.Total {
			return x.Total > y.Total
		}
		return x.ID < y.ID
	})
	sort.SliceStable(out.Absent, func(a, b int) bool {
		if out.Absent[a].ChaptersSince != out.Absent[b].ChaptersSince {
			return out.Absent[a].ChaptersSince > out.Absent[b].ChaptersSince
		}
		return out.Absent[a].ID < out.Absent[b].ID
	})

	n := float64(out.Units)
	for k, c := range shared {
		if c < MinPairCount {
			continue
		}
		a, b := k[0], k[1]
		na, nb := float64(unitsOf[a]), float64(unitsOf[b])
		idA, idB := entities[a].ID, entities[b].ID
		if idA > idB {
			idA, idB = idB, idA
		}
		out.Pairs = append(out.Pairs, Pair{
			A: idA, B: idB, Count: c,
			Jaccard: float64(c) / (na + nb - float64(c)),
			PMI:     math.Log2(float64(c) * n / (na * nb)),
		})
	}
	sort.Slice(out.Pairs, func(a, b int) bool {
		x, y := out.Pairs[a], out.Pairs[b]
		if x.Count != y.Count {
			return x.Count > y.Count
		}
		if !nearlyEqual(x.PMI, y.PMI) {
			return x.PMI > y.PMI
		}
		if x.A != y.A {
			return x.A < y.A
		}
		return x.B < y.B
	})
	return out
}
