package lore

import (
	"fmt"
	"reflect"
	"testing"

	"stylelab/internal/insight"
)

func node(id, faction string) Node {
	return Node{ID: id, Name: id, Kind: "character", Faction: faction}
}

func TestDescribeCommunitiesNamesAndOutliers(t *testing.T) {
	nodes := []Node{
		node("lin", "青云宗"), node("su", "青云宗"), node("zhao", "青云宗"),
		node("spy", "魔门"), node("drifter", ""),
		{ID: "sect", Name: "青云宗", Kind: "faction"},
	}
	part := insight.Partition{Communities: [][]string{
		{"drifter", "lin", "sect", "spy", "su", "zhao"},
		{"alone"},
	}}
	ranking := []insight.NodeRank{
		{ID: "lin", Rank: 1}, {ID: "su", Rank: 2}, {ID: "spy", Rank: 3},
		{ID: "zhao", Rank: 4}, {ID: "sect", Rank: 5}, {ID: "drifter", Rank: 6}, {ID: "alone", Rank: 7},
	}
	got := describeCommunities(part, nodes, ranking)
	if len(got) != 1 {
		t.Fatalf("singletons should be dropped: %+v", got)
	}
	c := got[0]
	// The faction node counts for its own name: 4 of 5 labelled say 青云宗.
	if c.Faction != "青云宗" {
		t.Fatalf("faction = %q", c.Faction)
	}
	if !reflect.DeepEqual(c.Members, []string{"lin", "su", "spy", "zhao", "sect", "drifter"}) {
		t.Fatalf("members not by rank: %v", c.Members)
	}
	if !reflect.DeepEqual(c.Outliers, []Outlier{{ID: "spy", Faction: "魔门"}}) {
		t.Fatalf("outliers = %+v", c.Outliers)
	}
}

func TestDescribeCommunitiesWithoutClearFaction(t *testing.T) {
	nodes := []Node{node("a", "东"), node("b", "西"), node("c", "南"), node("d", "")}
	part := insight.Partition{Communities: [][]string{{"a", "b", "c", "d"}}}
	got := describeCommunities(part, nodes, nil)
	// No faction holds half of the three labelled members: no name, and so
	// nobody is an outlier either.
	if got[0].Faction != "" || len(got[0].Outliers) != 0 {
		t.Fatalf("got %+v", got[0])
	}
}

func TestDescribeCommunitiesHalfIsEnough(t *testing.T) {
	nodes := []Node{node("a", "东"), node("b", "东"), node("c", "西"), node("d", "南")}
	part := insight.Partition{Communities: [][]string{{"a", "b", "c", "d"}}}
	got := describeCommunities(part, nodes, nil)
	if got[0].Faction != "东" || len(got[0].Outliers) != 2 {
		t.Fatalf("got %+v", got[0])
	}
}

func TestAliasesAcceptListOrString(t *testing.T) {
	if got := Aliases(map[string]any{"aliases": []any{" 林师兄 ", "", 3, "远哥"}}); !reflect.DeepEqual(got, []string{"林师兄", "远哥"}) {
		t.Fatalf("list: %v", got)
	}
	if got := Aliases(map[string]any{"aliases": "林师兄、远哥, 小林；林少"}); !reflect.DeepEqual(got, []string{"林师兄", "远哥", "小林", "林少"}) {
		t.Fatalf("string: %v", got)
	}
	if got := Aliases(nil); got != nil {
		t.Fatalf("none: %v", got)
	}
}

func TestParagraphsDropBlankLines(t *testing.T) {
	got := Paragraphs("  第一段。\n\n\t第二段。\r\n   \n第三段")
	if !reflect.DeepEqual(got, []string{"第一段。", "第二段。", "第三段"}) {
		t.Fatalf("got %q", got)
	}
}

func TestValidateWeights(t *testing.T) {
	for in, want := range map[string]string{"": "graph", "graph": "graph", "text": "text", " both ": "both"} {
		if got, err := ValidateWeights(in); err != nil || got != want {
			t.Errorf("%q -> %q, %v", in, got, err)
		}
	}
	if _, err := ValidateWeights("pagerank"); err == nil {
		t.Error("unknown mode accepted")
	}
}

func chapterSeqs(n VolumeNode) []int {
	var out []int
	for _, c := range n.Chapters {
		out = append(out, c.Seq)
	}
	return out
}

func TestGroupByVolume(t *testing.T) {
	vols := []Volume{{ID: "v1", StartSeq: 3, Title: "上卷"}, {ID: "v2", StartSeq: 6, Title: "下卷"}, {ID: "v3", StartSeq: 20, Title: "空卷"}}
	var chapters []ChapterNode
	for _, seq := range []int{1, 2, 3, 4, 5, 7, 9} { // 6 and 8 were deleted
		runes := 0
		if seq%2 == 1 {
			runes = 1000
		}
		chapters = append(chapters, ChapterNode{ID: fmt.Sprint(seq), Seq: seq, Runes: runes})
	}
	got := groupByVolume(vols, chapters)
	if len(got) != 4 || got[0].ID != "" || got[1].ID != "v1" || got[2].ID != "v2" || got[3].ID != "v3" {
		t.Fatalf("groups = %+v", got)
	}
	for i, want := range [][]int{{1, 2}, {3, 4, 5}, {7, 9}, nil} {
		if !reflect.DeepEqual(chapterSeqs(got[i]), want) {
			t.Errorf("group %d = %v, want %v", i, chapterSeqs(got[i]), want)
		}
	}
	// v2 starts at the deleted chapter 6 and still collects 7 and 9.
	if got[2].Runes != 2000 || got[2].Written != 2 || got[1].Written != 2 {
		t.Fatalf("totals: v1 %d/%d v2 %d/%d", got[1].Runes, got[1].Written, got[2].Runes, got[2].Written)
	}
	if got[3].Chapters == nil {
		t.Fatal("an empty volume must list [] not null")
	}

	// No leading group when the first volume starts at chapter 1.
	got = groupByVolume([]Volume{{ID: "v", StartSeq: 1}}, chapters)
	if len(got) != 1 || len(got[0].Chapters) != len(chapters) {
		t.Fatalf("single volume: %+v", got)
	}
	// No volumes: one unnamed group.
	if got = groupByVolume(nil, chapters); len(got) != 1 || got[0].ID != "" {
		t.Fatalf("no volumes: %+v", got)
	}
}

func TestValidateOutlineVolumes(t *testing.T) {
	vols := []OutlineVolume{{Title: "一", ChapterCount: 2}, {Title: "二", ChapterCount: 3}}
	if err := ValidateOutlineVolumes(vols, 5); err != nil {
		t.Fatalf("matching counts: %v", err)
	}
	if err := ValidateOutlineVolumes(vols, 4); err == nil {
		t.Fatal("mismatched counts accepted")
	}
	if err := ValidateOutlineVolumes([]OutlineVolume{{ChapterCount: -1}}, -1); err == nil {
		t.Fatal("negative count accepted")
	}
	if err := ValidateOutlineVolumes(nil, 7); err != nil {
		t.Fatalf("no volumes is fine: %v", err)
	}
}
