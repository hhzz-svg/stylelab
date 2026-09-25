package insight

import (
	"sort"
)

// Partition is a split of the graph's nodes into communities.
type Partition struct {
	// Communities lists node ids, largest community first; ties by the
	// smallest member id. Members are sorted by id.
	Communities [][]string
	// Modularity is Newman's Q for this split: the share of edge weight
	// inside communities minus what random wiring with the same degrees
	// would put there. Above ~0.3 the groups are clearly real.
	Modularity float64
}

// louvainEpsilon is the smallest modularity gain worth a move; it keeps
// float noise from shuffling nodes between equally good communities.
const louvainEpsilon = 1e-12

// level is one layer of the Louvain hierarchy: after each round the
// communities found become the nodes of the next level.
type level struct {
	adj  [][]arc   // neighbours, excluding self
	self []float64 // self-loop weight (inside a merged community, counted both ways)
	k    []float64 // strength, including self
	m2   float64   // total strength: twice the edge weight
}

// Louvain finds communities by greedy modularity optimisation (Blondel et
// al. 2008): move each node to the neighbouring community that raises
// modularity most until nothing moves, merge each community into a single
// node, and repeat on the smaller graph until no merge helps.
//
// Nodes are visited in id order and ties go to the lowest community, so
// the same graph always yields the same partition. Isolated nodes stay on
// their own.
func Louvain(g *Graph) Partition {
	n := g.Len()
	if n == 0 {
		return Partition{Communities: [][]string{}}
	}

	lv := &level{adj: g.adj, self: make([]float64, n), k: append([]float64(nil), g.strength...)}
	for _, s := range lv.k {
		lv.m2 += s
	}

	// membership[i] is the current-level node that original node i sits in.
	membership := make([]int, n)
	for i := range membership {
		membership[i] = i
	}

	if lv.m2 > 0 {
		for {
			comm, moved := lv.localMoves()
			if !moved {
				break
			}
			var count int
			comm, count = renumber(comm)
			for i := range membership {
				membership[i] = comm[membership[i]]
			}
			if count == len(lv.k) {
				break
			}
			lv = lv.aggregate(comm, count)
		}
	}

	groups := make(map[int][]string)
	for i, c := range membership {
		groups[c] = append(groups[c], g.ID(i))
	}
	out := make([][]string, 0, len(groups))
	for _, members := range groups {
		sort.Strings(members)
		out = append(out, members)
	}
	sort.Slice(out, func(a, b int) bool {
		if len(out[a]) != len(out[b]) {
			return len(out[a]) > len(out[b])
		}
		return out[a][0] < out[b][0]
	})
	return Partition{Communities: out, Modularity: Modularity(g, out)}
}

// localMoves runs Louvain's first phase on this level. comm[i] is node i's
// community, labelled by a node index.
func (lv *level) localMoves() (comm []int, movedAny bool) {
	n := len(lv.k)
	comm = make([]int, n)
	tot := make([]float64, n) // total strength of each community
	for i := range comm {
		comm[i] = i
		tot[i] = lv.k[i]
	}

	weightTo := make([]float64, n) // scratch: edge weight from i into each community
	touched := make([]int, 0, 16)

	for {
		moves := 0
		for i := 0; i < n; i++ {
			own := comm[i]
			touched = touched[:0]
			for _, a := range lv.adj[i] {
				c := comm[a.to]
				if weightTo[c] == 0 {
					touched = append(touched, c)
				}
				weightTo[c] += a.weight
			}

			// Take i out of its community, then put it back wherever the
			// gain k_i,in - tot_c * k_i / 2m is largest.
			tot[own] -= lv.k[i]
			gain := func(c int) float64 { return weightTo[c] - tot[c]*lv.k[i]/lv.m2 }

			best, bestGain := own, gain(own)
			sort.Ints(touched)
			for _, c := range touched {
				if c == own {
					continue
				}
				if gc := gain(c); gc > bestGain+louvainEpsilon {
					best, bestGain = c, gc
				}
			}
			tot[best] += lv.k[i]
			if best != own {
				comm[i] = best
				moves++
				movedAny = true
			}
			for _, c := range touched {
				weightTo[c] = 0
			}
		}
		if moves == 0 {
			return comm, movedAny
		}
	}
}

// aggregate builds the next level: one node per community.
func (lv *level) aggregate(comm []int, count int) *level {
	next := &level{
		self: make([]float64, count),
		k:    make([]float64, count),
		m2:   lv.m2,
	}
	between := make(map[[2]int]float64)
	for i := range lv.k {
		ci := comm[i]
		next.k[ci] += lv.k[i]
		next.self[ci] += lv.self[i]
		for _, a := range lv.adj[i] {
			cj := comm[a.to]
			if ci == cj {
				next.self[ci] += a.weight // each internal edge is seen from both ends
				continue
			}
			if ci < cj {
				between[[2]int{ci, cj}] += a.weight
			}
		}
	}
	next.adj = make([][]arc, count)
	for key, w := range between {
		next.adj[key[0]] = append(next.adj[key[0]], arc{to: key[1], weight: w})
		next.adj[key[1]] = append(next.adj[key[1]], arc{to: key[0], weight: w})
	}
	for i := range next.adj {
		sort.Slice(next.adj[i], func(x, y int) bool { return next.adj[i][x].to < next.adj[i][y].to })
	}
	return next
}

// renumber relabels communities 0..count-1 in order of first appearance.
func renumber(comm []int) ([]int, int) {
	ids := make(map[int]int)
	out := make([]int, len(comm))
	for i, c := range comm {
		id, ok := ids[c]
		if !ok {
			id = len(ids)
			ids[c] = id
		}
		out[i] = id
	}
	return out, len(ids)
}

// Modularity is Newman's Q of a partition of g, given as groups of ids.
// Nodes left out of every group count as singletons.
func Modularity(g *Graph, communities [][]string) float64 {
	m2 := 0.0
	for _, s := range g.strength {
		m2 += s
	}
	if m2 == 0 {
		return 0
	}
	of := make([]int, g.Len())
	for i := range of {
		of[i] = -1
	}
	for c, members := range communities {
		for _, id := range members {
			if i, ok := g.index[id]; ok {
				of[i] = c
			}
		}
	}
	next := len(communities)
	for i := range of {
		if of[i] < 0 {
			of[i] = next
			next++
		}
	}
	in := make([]float64, next)
	tot := make([]float64, next)
	for i := range of {
		tot[of[i]] += g.strength[i]
		for _, a := range g.adj[i] {
			if of[a.to] == of[i] {
				in[of[i]] += a.weight
			}
		}
	}
	q := 0.0
	for c := range in {
		q += in[c]/m2 - (tot[c]/m2)*(tot[c]/m2)
	}
	return q
}
