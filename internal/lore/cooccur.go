package lore

import (
	"context"
	"fmt"
	"strings"

	"stylelab/internal/insight"
	"stylelab/internal/store"
)

// Weights modes for the graph analysis.
const (
	WeightsGraph = "graph" // the relations the author drew (default)
	WeightsText  = "text"  // how often they share a paragraph in the prose
	WeightsBoth  = "both"
)

// ValidateWeights normalises the mode; "" means graph.
func ValidateWeights(mode string) (string, error) {
	switch strings.TrimSpace(mode) {
	case "", WeightsGraph:
		return WeightsGraph, nil
	case WeightsText:
		return WeightsText, nil
	case WeightsBoth:
		return WeightsBoth, nil
	}
	return "", fmt.Errorf("invalid: weights must be graph, text or both")
}

// Report thresholds.
const (
	// MinSuggestCount is how many shared units a pair needs before it is
	// offered as a relation the graph is missing.
	MinSuggestCount = 3
	// MaxReportPairs caps the pairs listed to the client.
	MaxReportPairs = 60
)

// Written is a chapter with prose.
type Written struct {
	ID    string
	Seq   int
	Title string
	Body  string
}

// LoadWritten reads the chapters that have a body, in order.
func LoadWritten(ctx context.Context, st *store.Store, projectID string) ([]Written, error) {
	rows, err := st.DB().QueryContext(ctx,
		`SELECT id, seq, title, body FROM chapters WHERE project_id = ? AND TRIM(body) != '' ORDER BY seq`,
		projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Written
	for rows.Next() {
		var w Written
		if err := rows.Scan(&w.ID, &w.Seq, &w.Title, &w.Body); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

// Paragraphs splits prose into its non-blank lines.
func Paragraphs(body string) []string {
	var out []string
	for _, line := range strings.Split(body, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			out = append(out, line)
		}
	}
	return out
}

// Aliases reads details.aliases: a list of strings, or one string split on
// the usual separators.
func Aliases(details map[string]any) []string {
	var raw []string
	switch v := details["aliases"].(type) {
	case []any:
		for _, x := range v {
			if s, ok := x.(string); ok {
				raw = append(raw, s)
			}
		}
	case string:
		raw = strings.FieldsFunc(v, func(r rune) bool {
			return r == ',' || r == '，' || r == '、' || r == ';' || r == '；' || r == '/' || r == ' ' || r == '\n'
		})
	}
	var out []string
	for _, s := range raw {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func (w World) coEntities() []insight.CoEntity {
	out := make([]insight.CoEntity, len(w.Nodes))
	for i, n := range w.Nodes {
		out[i] = insight.CoEntity{ID: n.ID, Names: append([]string{n.Name}, Aliases(n.Details)...)}
	}
	return out
}

// Unit kinds: what two entities must share to co-occur.
const (
	UnitParagraph = "paragraph"
	UnitScene     = "scene"
	UnitMixed     = "mixed" // scenes where a chapter has current ones, paragraphs elsewhere
)

// coChapters splits each written chapter into units: its scenes when it
// has been segmented and the scenes still match the body, else paragraphs.
func coChapters(ctx context.Context, st *store.Store, projectID string, written []Written) ([]insight.CoChapter, string, error) {
	scenes, err := freshSceneUnits(ctx, st, projectID, written)
	if err != nil {
		return nil, "", err
	}
	out := make([]insight.CoChapter, len(written))
	byScene := 0
	for i, c := range written {
		units, ok := scenes[c.ID]
		if ok {
			byScene++
		} else {
			units = Paragraphs(c.Body)
		}
		out[i] = insight.CoChapter{Seq: c.Seq, Units: units}
	}
	kind := UnitMixed
	switch byScene {
	case 0:
		kind = UnitParagraph
	case len(written):
		kind = UnitScene
	}
	return out, kind, nil
}

// CooccurrenceReport is what GET .../graph/cooccurrence returns.
type CooccurrenceReport struct {
	insight.Cooccurrence
	ChapterTitles []string `json:"chapter_titles"`
	// Suggestions are frequent pairs with no relation in the graph.
	Suggestions []insight.Pair `json:"suggestions"`
	UnitKind    string         `json:"unit_kind"` // what a shared unit is
}

// Cooccurrence analyses who appears where in the written chapters.
func Cooccurrence(ctx context.Context, st *store.Store, projectID string, absentAfter int) (CooccurrenceReport, error) {
	w, err := LoadWorld(ctx, st, projectID)
	if err != nil {
		return CooccurrenceReport{}, err
	}
	written, err := LoadWritten(ctx, st, projectID)
	if err != nil {
		return CooccurrenceReport{}, err
	}
	chapters, unitKind, err := coChapters(ctx, st, projectID, written)
	if err != nil {
		return CooccurrenceReport{}, err
	}
	co := insight.Cooccur(w.coEntities(), chapters, absentAfter)

	linked := map[[2]string]bool{}
	for _, l := range w.Links {
		a, b := l.SourceID, l.TargetID
		if a > b {
			a, b = b, a
		}
		linked[[2]string{a, b}] = true
	}
	suggestions := []insight.Pair{}
	for _, p := range co.Pairs {
		if p.Count >= MinSuggestCount && !linked[[2]string{p.A, p.B}] {
			suggestions = append(suggestions, p)
		}
	}
	if len(co.Pairs) > MaxReportPairs {
		co.Pairs = co.Pairs[:MaxReportPairs]
	}

	titles := make([]string, len(written))
	for i, c := range written {
		titles[i] = c.Title
	}
	return CooccurrenceReport{Cooccurrence: co, ChapterTitles: titles, Suggestions: suggestions, UnitKind: unitKind}, nil
}

// textEdges turns co-occurrence into edges weighted in (0, 1]: the most
// frequent pair weighs 1, as much as one relation the author drew.
func textEdges(pairs []insight.Pair) []insight.Edge {
	maxCount := 0
	for _, p := range pairs {
		if p.Count > maxCount {
			maxCount = p.Count
		}
	}
	out := make([]insight.Edge, len(pairs))
	for i, p := range pairs {
		out[i] = insight.Edge{Source: p.A, Target: p.B, Weight: float64(p.Count) / float64(maxCount)}
	}
	return out
}
