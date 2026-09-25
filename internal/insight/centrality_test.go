package insight

import (
	"math"
	"math/rand"
	"testing"
)

func approx(t *testing.T, name string, got, want, tol float64) {
	t.Helper()
	if math.Abs(got-want) > tol {
		t.Fatalf("%s = %.6f, want %.6f", name, got, want)
	}
}

func e(a, b string) Edge { return Edge{Source: a, Target: b, Weight: 1} }

func byID(ranked []NodeRank) map[string]NodeRank {
	m := make(map[string]NodeRank, len(ranked))
	for _, r := range ranked {
		m[r.ID] = r
	}
	return m
}

// Path a-b-c, worked by hand: with d = 0.85 the ends satisfy
// x = 0.05 + 0.425y and the middle y = 0.05 + 1.7x, so x = 0.07125/0.2775.
func TestPageRankPathByHand(t *testing.T) {
	g := NewGraph([]string{"a", "b", "c"}, []Edge{e("a", "b"), e("b", "c")})
	pr := PageRank(g)
	x := 0.07125 / 0.2775
	approx(t, "PR(a)", pr[0], x, 1e-8)
	approx(t, "PR(b)", pr[1], 0.05+1.7*x, 1e-8)
	approx(t, "PR(c)", pr[2], x, 1e-8)
	approx(t, "sum", pr[0]+pr[1]+pr[2], 1, 1e-9)
}

func TestPageRankSumsToOneWithIsolatedNodes(t *testing.T) {
	g := NewGraph([]string{"a", "b", "c", "lonely"}, []Edge{e("a", "b"), e("b", "c")})
	sum := 0.0
	for _, v := range PageRank(g) {
		sum += v
	}
	approx(t, "sum", sum, 1, 1e-9)
}

func TestStrongerTiesWeighMore(t *testing.T) {
	// hub is tied to a strongly and to b weakly; a should outrank b.
	g := NewGraph([]string{"a", "b", "hub"}, []Edge{
		{Source: "hub", Target: "a", Weight: 5},
		{Source: "hub", Target: "b", Weight: 1},
	})
	r := byID(RankNodes(g))
	if r["a"].PageRank <= r["b"].PageRank {
		t.Fatalf("PR(a)=%v should exceed PR(b)=%v", r["a"].PageRank, r["b"].PageRank)
	}
}

func TestBetweennessPathByHand(t *testing.T) {
	// Only b lies between a and c: one pair of the one possible, so 1.
	g := NewGraph([]string{"a", "b", "c"}, []Edge{e("a", "b"), e("b", "c")})
	bc := Betweenness(g)
	approx(t, "BC(a)", bc[0], 0, 1e-12)
	approx(t, "BC(b)", bc[1], 1, 1e-12)
	approx(t, "BC(c)", bc[2], 0, 1e-12)
}

func TestBetweennessSplitsEqualPaths(t *testing.T) {
	// Square a-b-d-c-a: a and d are joined by two shortest paths, through b
	// and through c, so each gets half of that pair; likewise b-c via a or d.
	g := NewGraph([]string{"a", "b", "c", "d"}, []Edge{e("a", "b"), e("b", "d"), e("d", "c"), e("c", "a")})
	bc := Betweenness(g)
	// Each node: 0.5 (one opposite pair) / ((4-1)(4-2)/2 = 3 pairs).
	for i, v := range bc {
		approx(t, "BC["+g.ID(i)+"]", v, 0.5/3, 1e-12)
	}
}

// Two triangles joined through a bridge node: the bridge is the hub.
func bridgeGraph() *Graph {
	return NewGraph(
		[]string{"a1", "a2", "a3", "bridge", "b1", "b2", "b3"},
		[]Edge{
			e("a1", "a2"), e("a2", "a3"), e("a3", "a1"),
			e("b1", "b2"), e("b2", "b3"), e("b3", "b1"),
			e("a1", "bridge"), e("bridge", "b1"),
		},
	)
}

func TestBridgeHasHighestBetweenness(t *testing.T) {
	g := bridgeGraph()
	bc := Betweenness(g)
	top := 0
	for i := range bc {
		if bc[i] > bc[top] {
			top = i
		}
	}
	if g.ID(top) != "bridge" {
		t.Fatalf("highest betweenness is %s, want bridge", g.ID(top))
	}
}

func TestStarCentreRanksFirstAndIsCore(t *testing.T) {
	ids := []string{"centre", "l1", "l2", "l3", "l4", "l5"}
	var edges []Edge
	for _, l := range ids[1:] {
		edges = append(edges, e("centre", l))
	}
	ranked := RankNodes(NewGraph(ids, edges))
	if ranked[0].ID != "centre" || ranked[0].Rank != 1 || ranked[0].Score != 100 || ranked[0].Role != RoleCore {
		t.Fatalf("top = %+v", ranked[0])
	}
	for _, r := range ranked[1:] {
		if r.Role != RolePeripheral {
			t.Fatalf("leaf %s role = %q, want peripheral", r.ID, r.Role)
		}
		if r.Score >= 100 {
			t.Fatalf("leaf %s score = %d", r.ID, r.Score)
		}
	}
}

func TestRolesHubAndIsolated(t *testing.T) {
	g := NewGraph(
		append([]string{"alone"}, bridgeGraph().ids...),
		[]Edge{
			e("a1", "a2"), e("a2", "a3"), e("a3", "a1"),
			e("b1", "b2"), e("b2", "b3"), e("b3", "b1"),
			e("a1", "bridge"), e("bridge", "b1"),
		},
	)
	r := byID(RankNodes(g))
	if r["alone"].Role != RoleIsolated || r["alone"].Degree != 0 {
		t.Fatalf("alone = %+v", r["alone"])
	}
	// a1 and b1 tie for the top of PageRank. The quota (10% of 7, rounded
	// up) is one, but symmetric nodes must get the same role, so both are
	// core. The bridge ranks low but lies on every path between the
	// triangles: the hub.
	if r["a1"].Role != RoleCore || r["b1"].Role != RoleCore {
		t.Fatalf("a1 = %q, b1 = %q, want both core", r["a1"].Role, r["b1"].Role)
	}
	if r["bridge"].Role != RoleHub {
		t.Fatalf("bridge role = %q, want hub", r["bridge"].Role)
	}
	for _, id := range []string{"a2", "a3", "b2", "b3"} {
		if r[id].Role != RoleNone {
			t.Fatalf("%s role = %q, want none", id, r[id].Role)
		}
	}
}

func TestParallelEdgesMergeAndSelfLoopsDrop(t *testing.T) {
	g := NewGraph([]string{"a", "b"}, []Edge{e("a", "b"), e("b", "a"), e("a", "a"), e("a", "ghost")})
	if g.EdgeCount() != 1 {
		t.Fatalf("edges = %d, want 1", g.EdgeCount())
	}
	if g.strength[0] != 2 {
		t.Fatalf("merged weight = %v, want 2", g.strength[0])
	}
}

func TestEmptyAndSingleNode(t *testing.T) {
	if got := RankNodes(NewGraph(nil, nil)); len(got) != 0 {
		t.Fatalf("empty = %v", got)
	}
	got := RankNodes(NewGraph([]string{"solo"}, nil))
	if len(got) != 1 || got[0].Role != RoleIsolated || got[0].Rank != 1 {
		t.Fatalf("single = %+v", got)
	}
}

func TestRankingIgnoresInputOrder(t *testing.T) {
	ids := []string{"a1", "a2", "a3", "bridge", "b1", "b2", "b3", "x"}
	edges := []Edge{
		e("a1", "a2"), e("a2", "a3"), e("a3", "a1"),
		e("b1", "b2"), e("b2", "b3"), e("b3", "b1"),
		e("a1", "bridge"), e("bridge", "b1"), e("x", "b3"),
	}
	want := RankNodes(NewGraph(ids, edges))
	rng := rand.New(rand.NewSource(7))
	for trial := 0; trial < 20; trial++ {
		rng.Shuffle(len(ids), func(i, j int) { ids[i], ids[j] = ids[j], ids[i] })
		rng.Shuffle(len(edges), func(i, j int) { edges[i], edges[j] = edges[j], edges[i] })
		got := RankNodes(NewGraph(ids, edges))
		for i := range want {
			if got[i].ID != want[i].ID || got[i].Role != want[i].Role {
				t.Fatalf("trial %d position %d: %+v, want %+v", trial, i, got[i], want[i])
			}
		}
	}
}
