package lore

import (
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
