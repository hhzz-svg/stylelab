package insight

import (
	"fmt"
	"math/rand"
	"reflect"
	"testing"
)

// clique returns the edges of a complete graph over ids.
func clique(ids ...string) []Edge {
	var out []Edge
	for i := range ids {
		for j := i + 1; j < len(ids); j++ {
			out = append(out, e(ids[i], ids[j]))
		}
	}
	return out
}

func twoCliques() ([]string, []Edge) {
	ids := []string{"a1", "a2", "a3", "a4", "b1", "b2", "b3", "b4"}
	edges := append(clique("a1", "a2", "a3", "a4"), clique("b1", "b2", "b3", "b4")...)
	edges = append(edges, e("a4", "b1"))
	return ids, edges
}

// Two K4s and a bridge: 13 edges, so 2m = 26. Each side has 6 internal
// edges (12 counted both ways) and total degree 3*4 + 1 = 13, so
// Q = 2 * (12/26 - (13/26)^2).
func TestLouvainTwoCliquesByHand(t *testing.T) {
	ids, edges := twoCliques()
	p := Louvain(NewGraph(ids, edges))
	want := [][]string{{"a1", "a2", "a3", "a4"}, {"b1", "b2", "b3", "b4"}}
	if !reflect.DeepEqual(p.Communities, want) {
		t.Fatalf("communities = %v, want %v", p.Communities, want)
	}
	approx(t, "Q", p.Modularity, 2*(12.0/26-(13.0/26)*(13.0/26)), 1e-12)
}

// A ring of six K4s, neighbours joined by one edge: the textbook case
// Louvain must split into the six cliques.
func TestLouvainRingOfCliques(t *testing.T) {
	var ids []string
	var edges []Edge
	for c := 0; c < 6; c++ {
		var members []string
		for k := 0; k < 4; k++ {
			members = append(members, fmt.Sprintf("c%d_%d", c, k))
		}
		ids = append(ids, members...)
		edges = append(edges, clique(members...)...)
		edges = append(edges, e(fmt.Sprintf("c%d_3", c), fmt.Sprintf("c%d_0", (c+1)%6)))
	}
	p := Louvain(NewGraph(ids, edges))
	if len(p.Communities) != 6 {
		t.Fatalf("got %d communities: %v", len(p.Communities), p.Communities)
	}
	for _, members := range p.Communities {
		if len(members) != 4 || members[0][:2] != members[3][:2] {
			t.Fatalf("community mixes cliques: %v", members)
		}
	}
	if p.Modularity < 0.6 {
		t.Fatalf("Q = %.3f, want a clear split (> 0.6)", p.Modularity)
	}
}

func TestLouvainWeightsDecide(t *testing.T) {
	// A path a-b-c-d. Heavy a-b and c-d, light b-c: two pairs.
	g := NewGraph([]string{"a", "b", "c", "d"}, []Edge{
		{Source: "a", Target: "b", Weight: 10},
		{Source: "b", Target: "c", Weight: 1},
		{Source: "c", Target: "d", Weight: 10},
	})
	want := [][]string{{"a", "b"}, {"c", "d"}}
	if got := Louvain(g).Communities; !reflect.DeepEqual(got, want) {
		t.Fatalf("communities = %v, want %v", got, want)
	}
}

func TestLouvainIsolatedNodesStayAlone(t *testing.T) {
	ids, edges := twoCliques()
	ids = append(ids, "hermit", "wanderer")
	p := Louvain(NewGraph(ids, edges))
	if len(p.Communities) != 4 {
		t.Fatalf("communities = %v", p.Communities)
	}
	if !reflect.DeepEqual(p.Communities[2:], [][]string{{"hermit"}, {"wanderer"}}) {
		t.Fatalf("singletons = %v", p.Communities[2:])
	}
}

func TestLouvainNoEdgesAndEmpty(t *testing.T) {
	p := Louvain(NewGraph([]string{"x", "y"}, nil))
	if !reflect.DeepEqual(p.Communities, [][]string{{"x"}, {"y"}}) || p.Modularity != 0 {
		t.Fatalf("no edges: %+v", p)
	}
	if p := Louvain(NewGraph(nil, nil)); len(p.Communities) != 0 || p.Communities == nil {
		t.Fatalf("empty: %+v", p)
	}
}

func TestLouvainIgnoresInputOrder(t *testing.T) {
	ids, edges := twoCliques()
	ids = append(ids, "c1", "c2", "c3")
	edges = append(edges, e("c1", "c2"), e("c2", "c3"), e("c3", "c1"), e("c1", "b4"))
	want := Louvain(NewGraph(ids, edges))
	rng := rand.New(rand.NewSource(11))
	for trial := 0; trial < 30; trial++ {
		rng.Shuffle(len(ids), func(i, j int) { ids[i], ids[j] = ids[j], ids[i] })
		rng.Shuffle(len(edges), func(i, j int) { edges[i], edges[j] = edges[j], edges[i] })
		got := Louvain(NewGraph(ids, edges))
		if !reflect.DeepEqual(got.Communities, want.Communities) {
			t.Fatalf("trial %d: %v, want %v", trial, got.Communities, want.Communities)
		}
	}
}

func TestModularityOfTrivialSplits(t *testing.T) {
	ids, edges := twoCliques()
	g := NewGraph(ids, edges)
	// Everything in one community: Q = 1 - 1 = 0.
	approx(t, "Q(all)", Modularity(g, [][]string{ids}), 0, 1e-12)
	// Louvain never does worse than leaving each node alone.
	if Modularity(g, nil) >= Louvain(g).Modularity {
		t.Fatal("singletons should score below the Louvain split")
	}
}
