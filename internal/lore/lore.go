// Package lore loads a project's story world -- the relationship graph,
// and later its chapters and scenes -- and runs the insight analyses over
// it. The algorithms themselves live in internal/insight and never see the
// database.
package lore

import (
	"context"
	"encoding/json"

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
}

// Analyze runs the graph analyses over a project's world.
func Analyze(ctx context.Context, st *store.Store, projectID string) (Analysis, error) {
	w, err := LoadWorld(ctx, st, projectID)
	if err != nil {
		return Analysis{}, err
	}
	g := w.Graph()
	return Analysis{
		NodeCount: g.Len(),
		EdgeCount: g.EdgeCount(),
		Ranking:   insight.RankNodes(g),
	}, nil
}
