package studio

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"stylelab/internal/ids"
	"stylelab/internal/job"
	"stylelab/internal/llm"
	"stylelab/internal/llmkey"
	"stylelab/internal/store"
)

// GraphInput is the job payload for AI relationship-graph extraction.
type GraphInput struct {
	ProjectID string `json:"project_id"`
	Model     string `json:"model"`
}

// GraphResult reports what extraction changed. Unlike the other studio kinds
// this one writes the graph itself, so the result is a summary; the client
// re-reads the graph afterwards.
type GraphResult struct {
	NodesCreated  int `json:"nodes_created"`
	NodesUpdated  int `json:"nodes_updated"`
	EdgesCreated  int `json:"edges_created"`
	ChaptersRead  int `json:"chapters_read"`
	ChaptersTotal int `json:"chapters_total"`
}

const (
	// The extraction prompt used to carry every chapter with 1,500 runes of
	// body plus the whole bible, with no cap: a 50-chapter book overran the
	// context window. Bounded the same way as the continuity audit.
	graphChapterLimit  = 30
	graphChapterRunes  = 600
	graphBriefRunes    = 200
	graphBibleLimit    = 40
	graphBibleRunes    = 200
	graphTemp          = 0.2
	graphMaxTokens     = 4096
	defaultNodeKind    = "character"
	defaultRelationTxt = "关联"
)

// ErrNothingToExtract is returned when the project has no chapters and no
// bible entries.
var ErrNothingToExtract = fmt.Errorf("invalid: 项目中暂无章节或设定内容，无法提炼实体图谱")

type extractedGraph struct {
	Nodes []struct {
		Name    string         `json:"name"`
		Kind    string         `json:"kind"`
		Faction string         `json:"faction"`
		Summary string         `json:"summary"`
		Details map[string]any `json:"details"`
	} `json:"nodes"`
	Edges []struct {
		Source      string `json:"source"`
		Target      string `json:"target"`
		Relation    string `json:"relation"`
		Description string `json:"description"`
	} `json:"edges"`
}

func GraphJobHandler(st *store.Store, client *llm.Client, master []byte) job.Handler {
	return func(ctx context.Context, rec job.Record, prog func(int, string)) (json.RawMessage, error) {
		var in GraphInput
		if err := json.Unmarshal(rec.Payload, &in); err != nil {
			return nil, err
		}
		out, err := RunGraph(ctx, st, client, master, rec.UserID, in, prog)
		if err != nil {
			return nil, err
		}
		return json.Marshal(out)
	}
}

func RunGraph(
	ctx context.Context, st *store.Store, client *llm.Client, master []byte,
	userID string, in GraphInput, prog func(int, string),
) (GraphResult, error) {
	if prog == nil {
		prog = func(int, string) {}
	}
	if err := ownsProject(ctx, st, in.ProjectID, userID); err != nil {
		return GraphResult{}, err
	}

	prog(10, "collect")
	chapters, read, total, err := graphChapters(ctx, st, in.ProjectID)
	if err != nil {
		return GraphResult{}, err
	}
	bible, err := graphBible(ctx, st, in.ProjectID)
	if err != nil {
		return GraphResult{}, err
	}
	if len(chapters) == 0 && len(bible) == 0 {
		return GraphResult{}, ErrNothingToExtract
	}

	key, err := llmkey.Load(ctx, st, master, userID)
	if err != nil {
		return GraphResult{}, err
	}

	scopeNote := ""
	if total > read {
		scopeNote = fmt.Sprintf("\n\n（注：全书共 %d 章，因篇幅所限本次只读取了前 %d 章。）", total, read)
	}
	userPrompt := strings.Join(chapters, "\n\n") + "\n\n世界设定参考：\n" + strings.Join(bible, "\n") + scopeNote

	prog(35, "extract")
	outText, err := client.Chat(ctx, llm.Request{
		Provider:  key.Provider,
		BaseURL:   key.BaseURL,
		APIKey:    key.APIKey,
		Model:     resolveModel(in.Model),
		Temp:      graphTemp,
		MaxTokens: graphMaxTokens,
		Messages: []llm.Message{
			{Role: "system", Content: llm.GraphExtractSystem()},
			{Role: "user", Content: userPrompt},
		},
	})
	if err != nil {
		return GraphResult{}, fmt.Errorf("AI 提炼失败: %w", err)
	}

	prog(80, "parse")
	var extracted extractedGraph
	if err := json.Unmarshal([]byte(cleanJSONMarkdown(outText)), &extracted); err != nil {
		return GraphResult{}, fmt.Errorf("解析 AI 提炼结果失败: %w", err)
	}

	prog(90, "save")
	res, err := saveExtractedGraph(ctx, st, in.ProjectID, extracted)
	if err != nil {
		return GraphResult{}, err
	}
	res.ChaptersRead, res.ChaptersTotal = read, total
	prog(100, "done")
	return res, nil
}

func graphChapters(ctx context.Context, st *store.Store, projectID string) (out []string, read, total int, err error) {
	rows, err := st.DB().QueryContext(ctx,
		`SELECT seq, title, brief, body FROM chapters WHERE project_id = ? ORDER BY seq ASC`,
		projectID,
	)
	if err != nil {
		return nil, 0, 0, err
	}
	defer rows.Close()
	for rows.Next() {
		var seq int
		var title, brief, body string
		if err := rows.Scan(&seq, &title, &brief, &body); err != nil {
			return nil, 0, 0, err
		}
		total++
		if len(out) >= graphChapterLimit {
			continue
		}
		excerpt := headRunes(body, graphChapterRunes)
		if excerpt != body {
			excerpt += "…"
		}
		out = append(out, fmt.Sprintf("【第%d章 %s】\n概要：%s\n正文节选：%s",
			seq, title, headRunes(brief, graphBriefRunes), excerpt))
	}
	if err := rows.Err(); err != nil {
		return nil, 0, 0, err
	}
	return out, len(out), total, nil
}

func graphBible(ctx context.Context, st *store.Store, projectID string) ([]string, error) {
	rows, err := st.DB().QueryContext(ctx,
		`SELECT kind, name, content FROM bible_entries
		 WHERE project_id = ? AND status = 'active' LIMIT ?`,
		projectID, graphBibleLimit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var kind, name, content string
		if err := rows.Scan(&kind, &name, &content); err != nil {
			return nil, err
		}
		out = append(out, fmt.Sprintf("[%s] %s: %s", kind, name, headRunes(content, graphBibleRunes)))
	}
	return out, rows.Err()
}

// saveExtractedGraph upserts nodes by name and adds edges that are not already
// there, all in one transaction. It used to write row by row ignoring every
// error, so a failure part-way left a half-written graph; and it inserted
// every edge unconditionally, so re-running extraction doubled each relation.
func saveExtractedGraph(ctx context.Context, st *store.Store, projectID string, g extractedGraph) (GraphResult, error) {
	var res GraphResult
	tx, err := st.DB().BeginTx(ctx, nil)
	if err != nil {
		return res, err
	}
	defer tx.Rollback()

	nameToID, err := existingNodes(ctx, tx, projectID)
	if err != nil {
		return res, err
	}
	now := time.Now().UTC().Format(time.RFC3339)

	for _, n := range g.Nodes {
		name := strings.TrimSpace(n.Name)
		if name == "" {
			continue
		}
		kind := strings.TrimSpace(n.Kind)
		if kind == "" {
			kind = defaultNodeKind
		}
		details := []byte("{}")
		if n.Details != nil {
			if b, err := json.Marshal(n.Details); err == nil {
				details = b
			}
		}
		if id, ok := nameToID[name]; ok {
			// A field the model left empty keeps what is there, so a re-run
			// does not wipe a faction or summary the author typed in.
			if _, err := tx.ExecContext(ctx,
				`UPDATE project_graph_nodes
				 SET kind = ?, faction = COALESCE(NULLIF(?, ''), faction),
				     summary = COALESCE(NULLIF(?, ''), summary),
				     details_json = COALESCE(NULLIF(?, '{}'), details_json), updated_at = ?
				 WHERE id = ? AND project_id = ?`,
				kind, n.Faction, n.Summary, string(details), now, id, projectID,
			); err != nil {
				return res, err
			}
			res.NodesUpdated++
			continue
		}
		id := ids.New("node")
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO project_graph_nodes (id, project_id, name, kind, faction, summary, details_json, x, y, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, 0, 0, ?, ?)`,
			id, projectID, name, kind, n.Faction, n.Summary, string(details), now, now,
		); err != nil {
			return res, err
		}
		nameToID[name] = id
		res.NodesCreated++
	}

	seen, err := existingEdges(ctx, tx, projectID)
	if err != nil {
		return res, err
	}
	for _, e := range g.Edges {
		src, ok1 := nameToID[strings.TrimSpace(e.Source)]
		tgt, ok2 := nameToID[strings.TrimSpace(e.Target)]
		if !ok1 || !ok2 || src == tgt {
			continue
		}
		relation := strings.TrimSpace(e.Relation)
		if relation == "" {
			relation = defaultRelationTxt
		}
		k := edgeKey(src, tgt, relation)
		if seen[k] {
			continue
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO project_graph_edges (id, project_id, source_id, target_id, relation, description, strength, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?, 1.0, ?, ?)`,
			ids.New("edge"), projectID, src, tgt, relation, e.Description, now, now,
		); err != nil {
			return res, err
		}
		seen[k] = true
		res.EdgesCreated++
	}

	return res, tx.Commit()
}

func existingNodes(ctx context.Context, tx *sql.Tx, projectID string) (map[string]string, error) {
	rows, err := tx.QueryContext(ctx, `SELECT id, name FROM project_graph_nodes WHERE project_id = ?`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var id, name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		out[name] = id
	}
	return out, rows.Err()
}

func existingEdges(ctx context.Context, tx *sql.Tx, projectID string) (map[string]bool, error) {
	rows, err := tx.QueryContext(ctx,
		`SELECT source_id, target_id, relation FROM project_graph_edges WHERE project_id = ?`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var s, t, r string
		if err := rows.Scan(&s, &t, &r); err != nil {
			return nil, err
		}
		out[edgeKey(s, t, r)] = true
	}
	return out, rows.Err()
}

func edgeKey(src, tgt, relation string) string { return src + "\x00" + tgt + "\x00" + relation }
