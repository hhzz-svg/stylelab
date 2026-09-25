package insight

import (
	"math"
	"reflect"
	"testing"
)

func book() ([]CoEntity, []CoChapter) {
	entities := []CoEntity{
		{ID: "lin", Names: []string{"林远", "林师兄"}},
		{ID: "su", Names: []string{"苏晚"}},
		{ID: "mo", Names: []string{"魔尊"}},
		{ID: "ghost", Names: []string{"幽灵"}}, // never mentioned
	}
	chapters := []CoChapter{
		// Out of order on purpose: chapters are sorted by seq.
		{Seq: 2, Units: []string{"林师兄护着苏晚。", "林远独自练剑。魔尊冷笑，魔尊出关。"}},
		{Seq: 1, Units: []string{"林远与苏晚同行。", "魔尊闭关，魔尊不语，魔尊入定。"}},
	}
	for s := 3; s <= 12; s++ {
		chapters = append(chapters, CoChapter{Seq: s, Units: []string{"林远赶路。"}})
	}
	return entities, chapters
}

func TestCooccurCountsByHand(t *testing.T) {
	entities, chapters := book()
	c := Cooccur(entities, chapters, 10)

	if c.Units != 14 || len(c.Chapters) != 12 || c.Chapters[0] != 1 || c.Chapters[11] != 12 {
		t.Fatalf("units %d chapters %v", c.Units, c.Chapters)
	}
	app := map[string]Appearance{}
	for _, a := range c.Appearances {
		app[a.ID] = a
	}
	if _, ok := app["ghost"]; ok || len(c.Appearances) != 3 {
		t.Fatalf("appearances = %+v", c.Appearances)
	}
	// 林远: ch1 once, ch2 twice (one via the alias 林师兄), then once a chapter.
	lin := app["lin"]
	if lin.PerChapter[0] != 1 || lin.PerChapter[1] != 2 || lin.PerChapter[11] != 1 || lin.Total != 13 || lin.Units != 13 {
		t.Fatalf("lin = %+v", lin)
	}
	if lin.FirstSeq != 1 || lin.LastSeq != 12 {
		t.Fatalf("lin seqs = %d..%d", lin.FirstSeq, lin.LastSeq)
	}
	mo := app["mo"]
	if mo.Total != 5 || mo.Units != 2 || mo.LastSeq != 2 {
		t.Fatalf("mo = %+v", mo)
	}
	// Ordered by first appearance, then by mentions.
	if c.Appearances[0].ID != "lin" || c.Appearances[1].ID != "mo" || c.Appearances[2].ID != "su" {
		t.Fatalf("order = %s %s %s", c.Appearances[0].ID, c.Appearances[1].ID, c.Appearances[2].ID)
	}

	// 林远 and 苏晚 share 2 units; 林远 is in 13, 苏晚 in 2, of 14.
	var pair *Pair
	for i := range c.Pairs {
		if c.Pairs[i].A == "lin" && c.Pairs[i].B == "su" {
			pair = &c.Pairs[i]
		}
	}
	if pair == nil || pair.Count != 2 {
		t.Fatalf("pairs = %+v", c.Pairs)
	}
	approx(t, "jaccard", pair.Jaccard, 2.0/13, 1e-12)
	approx(t, "pmi", pair.PMI, math.Log2(2.0*14/(13*2)), 1e-12)
	// 林远 and 魔尊 share only the one unit in chapter 2: below the listing bar.
	if len(c.Pairs) != 1 {
		t.Fatalf("pairs = %+v", c.Pairs)
	}

	// 魔尊: 5 mentions, last seen in chapter 2, ten chapters ago.
	if !reflect.DeepEqual(c.Absent, []Absence{{ID: "mo", LastSeq: 2, ChaptersSince: 10}}) {
		t.Fatalf("absent = %+v", c.Absent)
	}
}

func TestCooccurAbsenceThreshold(t *testing.T) {
	entities, chapters := book()
	if got := Cooccur(entities, chapters, 11).Absent; len(got) != 0 {
		t.Fatalf("ten chapters is not eleven: %+v", got)
	}
	// 苏晚 has only 2 mentions: never listed however long she is gone.
	for _, a := range Cooccur(entities, chapters, 1).Absent {
		if a.ID == "su" {
			t.Fatal("a minor mention should not count as a disappearance")
		}
	}
}

func TestCooccurEmpty(t *testing.T) {
	c := Cooccur(nil, nil, 0)
	if c.Units != 0 || c.Appearances == nil || c.Pairs == nil || c.Absent == nil {
		t.Fatalf("empty = %+v", c)
	}
}
