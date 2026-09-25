// Package insight holds the deterministic analyses behind the story tools:
// centrality, community detection, lineage trees, co-occurrence and scene
// segmentation. It never touches the database or calls a model, so every
// result is reproducible and unit-testable; callers load the data and pass
// plain values in.
package insight

import (
	"sort"
)

// Edge is an undirected, weighted connection between two node ids.
type Edge struct {
	Source string
	Target string
	Weight float64
}

type arc struct {
	to     int
	weight float64
}

// Graph is an undirected weighted graph over a fixed set of node ids.
// Parallel edges are merged by summing their weights; self-loops and edges
// to unknown ids are dropped. Node order is sorted by id, which is what
// makes every algorithm here independent of input order.
type Graph struct {
	ids   []string
	index map[string]int
	adj   [][]arc
	// strength[i] is the total weight of node i's edges.
	strength []float64
	edges    int
}

// NewGraph builds a graph. A non-positive weight counts as 1.
func NewGraph(nodeIDs []string, edges []Edge) *Graph {
	ids := append([]string(nil), nodeIDs...)
	sort.Strings(ids)
	ids = dedupeSorted(ids)

	g := &Graph{ids: ids, index: make(map[string]int, len(ids))}
	for i, id := range ids {
		g.index[id] = i
	}

	merged := make(map[[2]int]float64)
	for _, e := range edges {
		a, okA := g.index[e.Source]
		b, okB := g.index[e.Target]
		if !okA || !okB || a == b {
			continue
		}
		if a > b {
			a, b = b, a
		}
		w := e.Weight
		if w <= 0 {
			w = 1
		}
		merged[[2]int{a, b}] += w
	}

	g.adj = make([][]arc, len(ids))
	g.strength = make([]float64, len(ids))
	for k, w := range merged {
		a, b := k[0], k[1]
		g.adj[a] = append(g.adj[a], arc{to: b, weight: w})
		g.adj[b] = append(g.adj[b], arc{to: a, weight: w})
		g.strength[a] += w
		g.strength[b] += w
	}
	for i := range g.adj {
		sort.Slice(g.adj[i], func(x, y int) bool { return g.adj[i][x].to < g.adj[i][y].to })
	}
	g.edges = len(merged)
	return g
}

// Len is the number of nodes.
func (g *Graph) Len() int { return len(g.ids) }

// EdgeCount is the number of distinct undirected edges after merging.
func (g *Graph) EdgeCount() int { return g.edges }

// ID returns the id of node i.
func (g *Graph) ID(i int) string { return g.ids[i] }

// Degree is the number of distinct neighbours of node i.
func (g *Graph) Degree(i int) int { return len(g.adj[i]) }

func dedupeSorted(s []string) []string {
	out := s[:0]
	for i, v := range s {
		if i > 0 && v == s[i-1] {
			continue
		}
		out = append(out, v)
	}
	return out
}
