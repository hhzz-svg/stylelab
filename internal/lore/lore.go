// Package lore loads a project's story world -- the relationship graph,
// and later its chapters and scenes -- and runs the insight analyses over
// it. The algorithms themselves live in internal/insight and never see the
// database.
package lore

import (
	"context"
	"encoding/json"
	"sort"

	"stylelab/internal/insight"
	"stylelab/internal/store"
)

// Node is a graph entity as the analyses need it.
type Node struct {
	ID      string
	Name    string
	Kind    string
	Faction string
	Details map[string]any
}

// Link is a graph relation.
type Link struct {
	ID        string
	SourceID  string
	TargetID  string
	Relation  string
	Strength  float64
	CreatedAt string
}

// World is a project's relationship graph.
type World struct {
	Nodes []Node
	Links []Link
}

// LoadWorld reads the project's graph. The caller checks ownership.
func LoadWorld(ctx context.Context, st *store.Store, projectID string) (World, error) {
	var w World
	rows, err := st.DB().QueryContext(ctx,
		`SELECT id, name, kind, faction, details_json FROM project_graph_nodes WHERE project_id = ? ORDER BY rowid`,
		projectID)
	if err != nil {
		return w, err
	}
	defer rows.Close()
	for rows.Next() {
		var n Node
		var details string
		if err := rows.Scan(&n.ID, &n.Name, &n.Kind, &n.Faction, &details); err != nil {
			return w, err
		}
		_ = json.Unmarshal([]byte(details), &n.Details)
		w.Nodes = append(w.Nodes, n)
	}
	if err := rows.Err(); err != nil {
		return w, err
	}

	rows, err = st.DB().QueryContext(ctx,
		`SELECT id, source_id, target_id, relation, strength, created_at FROM project_graph_edges WHERE project_id = ? ORDER BY rowid`,
		projectID)
	if err != nil {
		return w, err
	}
	defer rows.Close()
	for rows.Next() {
		var l Link
		if err := rows.Scan(&l.ID, &l.SourceID, &l.TargetID, &l.Relation, &l.Strength, &l.CreatedAt); err != nil {
			return w, err
		}
		w.Links = append(w.Links, l)
	}
	return w, rows.Err()
}

// Graph is the world as an undirected weighted graph, each relation
// weighted by its strength.
func (w World) Graph() *insight.Graph {
	ids := make([]string, len(w.Nodes))
	for i, n := range w.Nodes {
		ids[i] = n.ID
	}
	edges := make([]insight.Edge, len(w.Links))
	for i, l := range w.Links {
		edges[i] = insight.Edge{Source: l.SourceID, Target: l.TargetID, Weight: l.Strength}
	}
	return insight.NewGraph(ids, edges)
}

// Analysis is what GET .../graph/analysis returns.
type Analysis struct {
	NodeCount int                `json:"node_count"`
	EdgeCount int                `json:"edge_count"`
	Ranking   []insight.NodeRank `json:"ranking"`
	// Communities found by Louvain, largest first. Nodes in no community
	// here were left on their own.
	Communities []Community `json:"communities"`
	Modularity  float64     `json:"modularity"`
}

// Community is a group Louvain found, described against the factions the
// author labelled.
type Community struct {
	Index int `json:"index"`
	// Faction is the labelled faction most of the group's labelled members
	// share, or "" when the group has no clear one.
	Faction string `json:"faction"`
	// Members are node ids, most important first.
	Members []string `json:"members"`
	// Outliers are members labelled with a different faction from the
	// group's: someone whose ties say otherwise -- a secret ally, a spy,
	// or a label that is out of date.
	Outliers []Outlier `json:"outliers"`
}

// Outlier is a member whose labelled faction differs from its group's.
type Outlier struct {
	ID      string `json:"id"`
	Faction string `json:"faction"`
}

// Analyze runs the graph analyses over a project's world.
func Analyze(ctx context.Context, st *store.Store, projectID string) (Analysis, error) {
	w, err := LoadWorld(ctx, st, projectID)
	if err != nil {
		return Analysis{}, err
	}
	g := w.Graph()
	ranking := insight.RankNodes(g)
	part := insight.Louvain(g)
	return Analysis{
		NodeCount:   g.Len(),
		EdgeCount:   g.EdgeCount(),
		Ranking:     ranking,
		Communities: describeCommunities(part, w.Nodes, ranking),
		Modularity:  part.Modularity,
	}, nil
}

// factionOf is the faction a node is labelled with. A faction node with
// no faction of its own stands for itself.
func factionOf(n Node) string {
	if n.Faction != "" {
		return n.Faction
	}
	if n.Kind == "faction" {
		return n.Name
	}
	return ""
}

// describeCommunities keeps the groups of two or more and names each by
// the faction at least half its labelled members share.
func describeCommunities(p insight.Partition, nodes []Node, ranking []insight.NodeRank) []Community {
	byID := make(map[string]Node, len(nodes))
	for _, n := range nodes {
		byID[n.ID] = n
	}
	rank := make(map[string]int, len(ranking))
	for _, r := range ranking {
		rank[r.ID] = r.Rank
	}

	out := []Community{}
	for _, group := range p.Communities {
		if len(group) < 2 {
			continue
		}
		members := append([]string(nil), group...)
		sort.SliceStable(members, func(a, b int) bool { return rank[members[a]] < rank[members[b]] })

		// Count labelled factions; ties go to the one whose first member
		// ranks higher.
		counts := map[string]int{}
		var order []string
		labelled := 0
		for _, id := range members {
			f := factionOf(byID[id])
			if f == "" {
				continue
			}
			labelled++
			if counts[f] == 0 {
				order = append(order, f)
			}
			counts[f]++
		}
		dominant := ""
		for _, f := range order {
			if counts[f] > counts[dominant] {
				dominant = f
			}
		}
		if dominant != "" && counts[dominant]*2 < labelled {
			dominant = ""
		}

		c := Community{Index: len(out), Faction: dominant, Members: members, Outliers: []Outlier{}}
		if dominant != "" {
			for _, id := range members {
				if f := factionOf(byID[id]); f != "" && f != dominant {
					c.Outliers = append(c.Outliers, Outlier{ID: id, Faction: f})
				}
			}
		}
		out = append(out, c)
	}
	return out
}

// Lineage builds the project's lineage forest: factions, their members,
// and masters above disciples. Relations are read in creation order.
func Lineage(ctx context.Context, st *store.Store, projectID string) (insight.Lineage, error) {
	w, err := LoadWorld(ctx, st, projectID)
	if err != nil {
		return insight.Lineage{}, err
	}
	entities := make([]insight.TreeEntity, len(w.Nodes))
	for i, n := range w.Nodes {
		entities[i] = insight.TreeEntity{ID: n.ID, Name: n.Name, Kind: n.Kind, Faction: n.Faction}
	}
	links := make([]insight.TreeLink, len(w.Links))
	for i, l := range w.Links {
		links[i] = insight.TreeLink{Source: l.SourceID, Target: l.TargetID, Relation: l.Relation, Weight: l.Strength, Order: i}
	}
	return insight.BuildLineage(entities, links), nil
}
