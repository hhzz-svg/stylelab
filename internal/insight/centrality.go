package insight

import (
	"container/heap"
	"math"
	"sort"
)

// Roles assigned by RankNodes.
const (
	RoleCore       = "core"       // top of the PageRank ranking
	RoleHub        = "hub"        // bridges groups: high betweenness, not core
	RolePeripheral = "peripheral" // a single connection
	RoleIsolated   = "isolated"   // no connections
	RoleNone       = ""
)

const (
	pageRankDamping   = 0.85
	pageRankTolerance = 1e-10
	pageRankMaxIter   = 200
	// Share of connected nodes labelled core or hub; at least one of each
	// when the graph has any edges.
	roleShare = 0.1
)

// PageRank is weighted PageRank on the undirected graph: a node is
// important when important nodes are strongly connected to it. The result
// sums to 1. Isolated nodes spread their mass evenly, as in the standard
// treatment of dangling nodes.
func PageRank(g *Graph) []float64 {
	n := g.Len()
	if n == 0 {
		return nil
	}
	pr := make([]float64, n)
	for i := range pr {
		pr[i] = 1 / float64(n)
	}
	next := make([]float64, n)
	for iter := 0; iter < pageRankMaxIter; iter++ {
		dangling := 0.0
		for i := 0; i < n; i++ {
			if g.strength[i] == 0 {
				dangling += pr[i]
			}
		}
		base := (1-pageRankDamping)/float64(n) + pageRankDamping*dangling/float64(n)
		for i := range next {
			next[i] = base
		}
		for j := 0; j < n; j++ {
			if g.strength[j] == 0 {
				continue
			}
			share := pageRankDamping * pr[j] / g.strength[j]
			for _, a := range g.adj[j] {
				next[a.to] += share * a.weight
			}
		}
		diff := 0.0
		for i := range pr {
			diff += math.Abs(next[i] - pr[i])
		}
		pr, next = next, pr
		if diff < pageRankTolerance {
			break
		}
	}
	return pr
}

// Betweenness is Brandes' betweenness centrality on the weighted graph,
// with an edge's length taken as 1/weight so that stronger ties are
// shorter. A node scores high when many shortest paths between other nodes
// run through it -- the go-between who links otherwise separate circles.
// Normalised to [0, 1] by the number of pairs not involving the node.
func Betweenness(g *Graph) []float64 {
	n := g.Len()
	cb := make([]float64, n)
	if n < 3 {
		return cb
	}

	dist := make([]float64, n)
	sigma := make([]float64, n)
	delta := make([]float64, n)
	preds := make([][]int, n)
	order := make([]int, 0, n)

	for s := 0; s < n; s++ {
		for i := 0; i < n; i++ {
			dist[i] = math.Inf(1)
			sigma[i] = 0
			delta[i] = 0
			preds[i] = preds[i][:0]
		}
		order = order[:0]
		dist[s] = 0
		sigma[s] = 1

		pq := &distHeap{{node: s, dist: 0}}
		done := make([]bool, n)
		for pq.Len() > 0 {
			item := heap.Pop(pq).(distItem)
			v := item.node
			if done[v] || item.dist > dist[v] {
				continue
			}
			done[v] = true
			order = append(order, v)
			for _, a := range g.adj[v] {
				alt := dist[v] + 1/a.weight
				switch {
				case nearlyEqual(alt, dist[a.to]):
					sigma[a.to] += sigma[v]
					preds[a.to] = append(preds[a.to], v)
				case alt < dist[a.to]:
					dist[a.to] = alt
					sigma[a.to] = sigma[v]
					preds[a.to] = append(preds[a.to][:0], v)
					heap.Push(pq, distItem{node: a.to, dist: alt})
				}
			}
		}

		for k := len(order) - 1; k >= 0; k-- {
			w := order[k]
			for _, v := range preds[w] {
				delta[v] += sigma[v] / sigma[w] * (1 + delta[w])
			}
			if w != s {
				cb[w] += delta[w]
			}
		}
	}

	// Each undirected path was counted from both ends.
	norm := float64((n - 1) * (n - 2))
	for i := range cb {
		cb[i] /= norm
	}
	return cb
}

func nearlyEqual(a, b float64) bool {
	if math.IsInf(a, 0) || math.IsInf(b, 0) {
		return false
	}
	return math.Abs(a-b) <= 1e-9*math.Max(1, math.Max(math.Abs(a), math.Abs(b)))
}

type distItem struct {
	node int
	dist float64
}

type distHeap []distItem

func (h distHeap) Len() int { return len(h) }
func (h distHeap) Less(i, j int) bool {
	if h[i].dist != h[j].dist {
		return h[i].dist < h[j].dist
	}
	return h[i].node < h[j].node
}
func (h distHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }
func (h *distHeap) Push(x any)   { *h = append(*h, x.(distItem)) }
func (h *distHeap) Pop() any {
	old := *h
	it := old[len(old)-1]
	*h = old[:len(old)-1]
	return it
}

// NodeRank is one node's standing in the graph.
type NodeRank struct {
	ID          string  `json:"id"`
	PageRank    float64 `json:"pagerank"`
	Score       int     `json:"score"` // PageRank scaled so the top node is 100
	Betweenness float64 `json:"betweenness"`
	Degree      int     `json:"degree"`
	Rank        int     `json:"rank"` // 1-based, by PageRank
	Role        string  `json:"role"`
}

// RankNodes scores every node and labels its role, ordered by rank.
// Ties in PageRank are broken by betweenness, then by id.
func RankNodes(g *Graph) []NodeRank {
	n := g.Len()
	if n == 0 {
		return []NodeRank{}
	}
	pr := PageRank(g)
	bc := Betweenness(g)

	out := make([]NodeRank, n)
	maxPR := 0.0
	for i := 0; i < n; i++ {
		maxPR = math.Max(maxPR, pr[i])
		out[i] = NodeRank{ID: g.ID(i), PageRank: pr[i], Betweenness: bc[i], Degree: g.Degree(i)}
	}
	for i := range out {
		if maxPR > 0 {
			out[i].Score = int(math.Round(out[i].PageRank / maxPR * 100))
		}
	}
	sort.SliceStable(out, func(a, b int) bool {
		if !nearlyEqual(out[a].PageRank, out[b].PageRank) {
			return out[a].PageRank > out[b].PageRank
		}
		if !nearlyEqual(out[a].Betweenness, out[b].Betweenness) {
			return out[a].Betweenness > out[b].Betweenness
		}
		return out[a].ID < out[b].ID
	})
	for i := range out {
		out[i].Rank = i + 1
	}
	assignRoles(out)
	return out
}

func assignRoles(ranked []NodeRank) {
	connected := 0
	for _, r := range ranked {
		if r.Degree > 0 {
			connected++
		}
	}
	if connected == 0 {
		for i := range ranked {
			ranked[i].Role = RoleIsolated
		}
		return
	}
	quota := int(math.Ceil(float64(connected) * roleShare))

	// Core: the top of the ranking among connected nodes. A node tied with
	// the last one admitted is admitted too: two symmetric leads must not
	// be told apart by their ids.
	core, lastPR := 0, 0.0
	for i := range ranked {
		if ranked[i].Degree == 0 {
			continue
		}
		if core < quota || nearlyEqual(ranked[i].PageRank, lastPR) {
			ranked[i].Role = RoleCore
			lastPR = ranked[i].PageRank
			core++
		}
	}

	// Hub: the highest betweenness outside the core. Only a node that
	// actually lies on others' shortest paths qualifies.
	byBetween := make([]int, 0, len(ranked))
	for i := range ranked {
		if ranked[i].Role == RoleNone && ranked[i].Betweenness > 0 {
			byBetween = append(byBetween, i)
		}
	}
	sort.SliceStable(byBetween, func(a, b int) bool {
		return ranked[byBetween[a]].Betweenness > ranked[byBetween[b]].Betweenness
	})
	lastBC := 0.0
	for k, i := range byBetween {
		if k >= quota && !nearlyEqual(ranked[i].Betweenness, lastBC) {
			break
		}
		ranked[i].Role = RoleHub
		lastBC = ranked[i].Betweenness
	}

	for i := range ranked {
		if ranked[i].Role != RoleNone {
			continue
		}
		switch ranked[i].Degree {
		case 0:
			ranked[i].Role = RoleIsolated
		case 1:
			ranked[i].Role = RolePeripheral
		}
	}
}
