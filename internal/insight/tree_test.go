package insight

import (
	"fmt"
	"math"
	"math/rand"
	"reflect"
	"sort"
	"testing"
)

func TestHierarchyDirection(t *testing.T) {
	cases := map[string]int{
		"师徒": 1, "师父": 1, "父子": 1, "君臣": 1, "主仆": 1, "掌门": 1,
		"徒弟": -1, "弟子": -1, "子女": -1, "隶属": -1, "成员": -1, "下属": -1,
		"盟友": 0, "宿敌": 0, "同门": 0, "暗恋": 0, "": 0,
		" 师徒关系 ": 1,
	}
	for rel, want := range cases {
		if got := HierarchyDirection(rel); got != want {
			t.Errorf("HierarchyDirection(%q) = %d, want %d", rel, got, want)
		}
	}
}

func ch(id, faction string) TreeEntity {
	return TreeEntity{ID: id, Name: id, Kind: "character", Faction: faction}
}

func fa(id, name string) TreeEntity { return TreeEntity{ID: id, Name: name, Kind: "faction"} }

func link(src, tgt, rel string, order int) TreeLink {
	return TreeLink{Source: src, Target: tgt, Relation: rel, Weight: 1, Order: order}
}

// index walks the forest into id -> node and id -> parent id.
func index(l Lineage) (map[string]*LineageNode, map[string]string) {
	nodes := map[string]*LineageNode{}
	parents := map[string]string{}
	var walk func(n *LineageNode, p string)
	walk = func(n *LineageNode, p string) {
		nodes[n.ID] = n
		if p != "" {
			parents[n.ID] = p
		}
		for _, c := range n.Children {
			walk(c, n.ID)
		}
	}
	for _, r := range l.Roots {
		walk(r, "")
	}
	return nodes, parents
}

func TestLineageFactionsMastersAndDisciples(t *testing.T) {
	l := BuildLineage(
		[]TreeEntity{
			fa("sect", "青云宗"),
			ch("elder", "青云宗"), ch("lin", "青云宗"), ch("su", "青云宗"), ch("boy", "青云宗"),
			ch("rogue", "魔门"), // 魔门 has no node: a virtual root
			ch("nobody", ""),
			{ID: "sword", Name: "青霜剑", Kind: "artifact"}, // not part of the tree
		},
		[]TreeLink{
			link("elder", "lin", "师徒", 1), // elder is lin's master
			link("su", "elder", "师父", 2),  // forward word: su is elder's master
			link("boy", "lin", "徒弟", 3),   // reversed word: boy is lin's disciple
			link("lin", "su", "同门", 4),    // not hierarchical
		},
	)
	nodes, parents := index(l)
	want := map[string]string{
		"su":     "sect",
		"elder":  "su",
		"lin":    "elder",
		"boy":    "lin",
		"rogue":  VirtualFactionID("魔门"),
		"nobody": UnassignedID,
	}
	for id, p := range want {
		if parents[id] != p {
			t.Errorf("parent(%s) = %q, want %q", id, parents[id], p)
		}
	}
	if _, ok := nodes["sword"]; ok {
		t.Error("artifacts must not appear in the lineage")
	}
	if nodes["lin"].Relation != "师徒" || nodes["boy"].Relation != "徒弟" || nodes["su"].Relation != "" {
		t.Errorf("relations: lin=%q boy=%q su=%q", nodes["lin"].Relation, nodes["boy"].Relation, nodes["su"].Relation)
	}
	// Roots: the real faction (biggest), then the virtual one, then 未归属.
	var roots []string
	for _, r := range l.Roots {
		roots = append(roots, r.ID)
	}
	if !reflect.DeepEqual(roots, []string{"sect", VirtualFactionID("魔门"), UnassignedID}) {
		t.Fatalf("roots = %v", roots)
	}
	if !nodes[VirtualFactionID("魔门")].Virtual || nodes["sect"].Virtual {
		t.Error("virtual flags wrong")
	}
	if l.Depth != 4 || len(l.Cycles) != 0 {
		t.Fatalf("depth %d cycles %v", l.Depth, l.Cycles)
	}
}

func TestLineagePrefersSameFactionThenWeightThenAge(t *testing.T) {
	l := BuildLineage(
		[]TreeEntity{ch("kid", "青云宗"), ch("outsider", "魔门"), ch("strong", "青云宗"), ch("old", "青云宗"), ch("young", "青云宗")},
		[]TreeLink{
			{Source: "outsider", Target: "kid", Relation: "师徒", Weight: 9, Order: 0},
			{Source: "young", Target: "kid", Relation: "师徒", Weight: 1, Order: 3},
			{Source: "old", Target: "kid", Relation: "师徒", Weight: 1, Order: 2},
			{Source: "strong", Target: "kid", Relation: "师徒", Weight: 5, Order: 4},
		},
	)
	nodes, parents := index(l)
	if parents["kid"] != "strong" {
		t.Fatalf("parent = %q, want the strongest same-faction master", parents["kid"])
	}
	if !reflect.DeepEqual(nodes["kid"].ExtraParents, []string{"old", "young", "outsider"}) {
		t.Fatalf("extra parents = %v", nodes["kid"].ExtraParents)
	}
}

func TestLineageBreaksCycles(t *testing.T) {
	l := BuildLineage(
		[]TreeEntity{fa("sect", "青云宗"), ch("a", "青云宗"), ch("b", "青云宗"), ch("c", "青云宗")},
		[]TreeLink{link("a", "b", "师徒", 1), link("b", "c", "师徒", 2), link("c", "a", "师徒", 3)},
	)
	if !reflect.DeepEqual(l.Cycles, [][]string{{"a", "b", "c"}}) {
		t.Fatalf("cycles = %v", l.Cycles)
	}
	_, parents := index(l)
	// a's master link is dropped and a falls back to its faction.
	if parents["a"] != "sect" || parents["b"] != "a" || parents["c"] != "b" {
		t.Fatalf("parents = %v", parents)
	}
}

func TestLineageFactionsNamingEachOther(t *testing.T) {
	l := BuildLineage(
		[]TreeEntity{{ID: "x", Name: "东", Kind: "faction", Faction: "西"}, {ID: "y", Name: "西", Kind: "faction", Faction: "东"}},
		nil,
	)
	// Faction labels never close a loop: x (first by id) goes under y, and
	// y's label is then refused instead of looping back.
	if len(l.Cycles) != 0 || len(l.Roots) != 1 || l.Roots[0].ID != "y" || l.Roots[0].Children[0].ID != "x" {
		t.Fatalf("got roots %+v cycles %v", l.Roots, l.Cycles)
	}
}

func TestLineageEmpty(t *testing.T) {
	l := BuildLineage(nil, nil)
	if l.Roots == nil || len(l.Roots) != 0 || l.Cycles == nil {
		t.Fatalf("empty = %+v", l)
	}
}

// --- layout ---------------------------------------------------------------

func leaf(id string) *LineageNode { return &LineageNode{ID: id} }

func tree(id string, children ...*LineageNode) *LineageNode {
	return &LineageNode{ID: id, Children: children}
}

// checkTidy asserts the layout guarantees over a laid-out tree.
func checkTidy(t *testing.T, roots []*LineageNode) {
	t.Helper()
	levels := map[int][]float64{}
	var walk func(n *LineageNode)
	walk = func(n *LineageNode) {
		levels[n.Depth] = append(levels[n.Depth], n.X)
		if n.X < -1e-9 {
			t.Errorf("%s: negative x %v", n.ID, n.X)
		}
		if k := len(n.Children); k > 0 {
			mid := (n.Children[0].X + n.Children[k-1].X) / 2
			if math.Abs(n.X-mid) > 1e-9 {
				t.Errorf("%s at %v is not centred over its children (%v)", n.ID, n.X, mid)
			}
			for i := 1; i < k; i++ {
				if n.Children[i].X <= n.Children[i-1].X {
					t.Errorf("%s: children out of order", n.ID)
				}
			}
		}
		for _, c := range n.Children {
			if c.Depth != n.Depth+1 {
				t.Errorf("%s: depth %d under %d", c.ID, c.Depth, n.Depth)
			}
			walk(c)
		}
	}
	for _, r := range roots {
		walk(r)
	}
	for d, xs := range levels {
		sort.Float64s(xs)
		for i := 1; i < len(xs); i++ {
			if xs[i]-xs[i-1] < siblingGap-1e-9 {
				t.Errorf("level %d: nodes %v apart", d, xs[i]-xs[i-1])
			}
		}
	}
}

func TestLayoutParentOverThreeLeaves(t *testing.T) {
	r := tree("r", leaf("a"), leaf("b"), leaf("c"))
	layoutForest([]*LineageNode{r})
	got := []float64{r.Children[0].X, r.Children[1].X, r.Children[2].X, r.X}
	if !reflect.DeepEqual(got, []float64{0, 1, 2, 1}) {
		t.Fatalf("x = %v", got)
	}
}

// The case Walker's algorithm exists for: two small subtrees between two
// wide ones must be spread evenly, not bunched against the left one.
func TestLayoutSpreadsSmallSubtreesEvenly(t *testing.T) {
	wide := func(id string) *LineageNode {
		return tree(id, tree(id+"1", leaf(id+"1a"), leaf(id+"1b"), leaf(id+"1c")), tree(id+"2", leaf(id+"2a"), leaf(id+"2b"), leaf(id+"2c")))
	}
	r := tree("r", wide("A"), leaf("B"), leaf("C"), wide("D"))
	layoutForest([]*LineageNode{r})
	checkTidy(t, []*LineageNode{r})
	a, b, c, d := r.Children[0].X, r.Children[1].X, r.Children[2].X, r.Children[3].X
	if math.Abs((b-a)-(c-b)) > 1e-9 || math.Abs((c-b)-(d-c)) > 1e-9 {
		t.Fatalf("uneven: A=%v B=%v C=%v D=%v", a, b, c, d)
	}
}

func TestLayoutRandomTreesAreTidy(t *testing.T) {
	rng := rand.New(rand.NewSource(3))
	for trial := 0; trial < 50; trial++ {
		n := 2 + rng.Intn(60)
		nodes := []*LineageNode{leaf("n0")}
		for i := 1; i < n; i++ {
			c := leaf(fmt.Sprintf("n%d", i))
			p := nodes[rng.Intn(len(nodes))]
			p.Children = append(p.Children, c)
			nodes = append(nodes, c)
		}
		layoutForest([]*LineageNode{nodes[0]})
		checkTidy(t, []*LineageNode{nodes[0]})
		if t.Failed() {
			t.Fatalf("trial %d failed", trial)
		}
	}
}

func TestLayoutForestKeepsTreesApart(t *testing.T) {
	a := tree("a", leaf("a1"), leaf("a2"), leaf("a3"))
	b := tree("b", tree("b1", leaf("b11"), leaf("b12")))
	width, depth := layoutForest([]*LineageNode{a, b})
	checkTidy(t, []*LineageNode{a, b})
	if b.Children[0].Children[0].X <= a.Children[2].X {
		t.Fatalf("trees overlap: a ends at %v, b starts at %v", a.Children[2].X, b.Children[0].Children[0].X)
	}
	if width != b.Children[0].Children[1].X || depth != 2 {
		t.Fatalf("width %v depth %d", width, depth)
	}
}

// --- places ---------------------------------------------------------------

func place(id, name, region string) TreeEntity {
	return TreeEntity{ID: id, Name: name, Kind: "location", Faction: region}
}

func TestPlaceDirection(t *testing.T) {
	for rel, want := range map[string]int{
		"位于": -1, "坐落于": -1, "地处": -1, "属于": -1, "境内": -1,
		"包含": 1, "下辖": 1, "管辖": 1,
		"毗邻": 0, "通往": 0, "": 0,
	} {
		if got := PlaceDirection(rel); got != want {
			t.Errorf("PlaceDirection(%q) = %d, want %d", rel, got, want)
		}
	}
}

func TestPlacesFromRelationsAndRegionField(t *testing.T) {
	l := BuildPlaces(
		[]TreeEntity{
			place("east", "东洲", ""),
			place("mount", "青云山", ""),
			place("hall", "藏经阁", "青云山"), // region field names another place
			place("peak", "天剑峰", "青云宗"), // names a sect, not a place: ignored
			place("city", "临安城", ""),
			ch("lin", "青云宗"), // characters are not in the geography
			fa("sect", "青云宗"),
		},
		[]TreeLink{
			link("mount", "east", "位于", 1), // the mountain lies within 东洲
			link("east", "city", "下辖", 2),  // 东洲 contains the city
			link("peak", "mount", "毗邻", 3), // not containment
			link("lin", "mount", "位于", 4),  // a character: ignored
		},
	)
	nodes, parents := index(l)
	want := map[string]string{"mount": "east", "hall": "mount", "city": "east"}
	for id, p := range want {
		if parents[id] != p {
			t.Errorf("parent(%s) = %q, want %q", id, parents[id], p)
		}
	}
	if _, ok := parents["peak"]; ok {
		t.Errorf("天剑峰 should be a root, got parent %q", parents["peak"])
	}
	for _, id := range []string{"lin", "sect", VirtualFactionID("青云宗"), UnassignedID} {
		if _, ok := nodes[id]; ok {
			t.Errorf("%s must not be in the geography", id)
		}
	}
	if len(l.Roots) != 2 || l.Roots[0].ID != "east" || l.Depth != 2 {
		t.Fatalf("roots %v depth %d", l.Roots, l.Depth)
	}
}

func TestPlacesBreakLoops(t *testing.T) {
	l := BuildPlaces(
		[]TreeEntity{place("a", "甲地", ""), place("b", "乙地", "")},
		[]TreeLink{link("a", "b", "位于", 1), link("b", "a", "位于", 2)},
	)
	if len(l.Cycles) != 1 || len(l.Roots) != 1 {
		t.Fatalf("cycles %v roots %d", l.Cycles, len(l.Roots))
	}
}
