package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"stylelab/internal/ids"
	"stylelab/internal/llm"
)

type GraphNode struct {
	ID        string         `json:"id"`
	ProjectID string         `json:"project_id"`
	Name      string         `json:"name"`
	Kind      string         `json:"kind"` // character | faction | artifact | location
	Faction   string         `json:"faction"`
	Summary   string         `json:"summary"`
	Details   map[string]any `json:"details"`
	X         float64        `json:"x"`
	Y         float64        `json:"y"`
	CreatedAt string         `json:"created_at"`
	UpdatedAt string         `json:"updated_at"`
}

type GraphEdge struct {
	ID          string  `json:"id"`
	ProjectID   string  `json:"project_id"`
	SourceID    string  `json:"source_id"`
	TargetID    string  `json:"target_id"`
	Relation    string  `json:"relation"` // 盟友, 宿敌, 师徒, 暗恋, 君臣, 仇敌, 同门, etc.
	Description string  `json:"description"`
	Strength    float64 `json:"strength"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

type GraphResponse struct {
	Nodes []GraphNode `json:"nodes"`
	Edges []GraphEdge `json:"edges"`
}

func (s *Server) handleGetGraph(w http.ResponseWriter, r *http.Request) {
	userID, err := s.currentUserID(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}
	projectID := r.PathValue("id")
	if err := s.requireOwnedProject(r.Context(), projectID, userID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "not_found", "not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "invalid", "internal error")
		return
	}

	// 1. Query nodes
	nodeRows, err := s.st.DB().QueryContext(
		r.Context(),
		`SELECT id, project_id, name, kind, faction, summary, details_json, x, y, created_at, updated_at
		 FROM project_graph_nodes WHERE project_id = ? ORDER BY created_at ASC`,
		projectID,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	defer nodeRows.Close()

	nodes := make([]GraphNode, 0)
	for nodeRows.Next() {
		var n GraphNode
		var detailsJSON string
		if err := nodeRows.Scan(&n.ID, &n.ProjectID, &n.Name, &n.Kind, &n.Faction, &n.Summary, &detailsJSON, &n.X, &n.Y, &n.CreatedAt, &n.UpdatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "internal", err.Error())
			return
		}
		if detailsJSON != "" {
			_ = json.Unmarshal([]byte(detailsJSON), &n.Details)
		}
		if n.Details == nil {
			n.Details = make(map[string]any)
		}
		nodes = append(nodes, n)
	}

	// 2. Query edges
	edgeRows, err := s.st.DB().QueryContext(
		r.Context(),
		`SELECT id, project_id, source_id, target_id, relation, description, strength, created_at, updated_at
		 FROM project_graph_edges WHERE project_id = ? ORDER BY created_at ASC`,
		projectID,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	defer edgeRows.Close()

	edges := make([]GraphEdge, 0)
	for edgeRows.Next() {
		var e GraphEdge
		if err := edgeRows.Scan(&e.ID, &e.ProjectID, &e.SourceID, &e.TargetID, &e.Relation, &e.Description, &e.Strength, &e.CreatedAt, &e.UpdatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "internal", err.Error())
			return
		}
		edges = append(edges, e)
	}

	writeJSON(w, http.StatusOK, GraphResponse{Nodes: nodes, Edges: edges})
}

func (s *Server) handleSaveGraphNode(w http.ResponseWriter, r *http.Request) {
	userID, err := s.currentUserID(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}
	projectID := r.PathValue("id")
	if err := s.requireOwnedProject(r.Context(), projectID, userID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "not_found", "not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "invalid", "internal error")
		return
	}

	var req struct {
		ID      string         `json:"id"`
		Name    string         `json:"name"`
		Kind    string         `json:"kind"`
		Faction string         `json:"faction"`
		Summary string         `json:"summary"`
		Details map[string]any `json:"details"`
		X       float64        `json:"x"`
		Y       float64        `json:"y"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid", "invalid json")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "invalid", "name is required")
		return
	}
	if req.Kind == "" {
		req.Kind = "character"
	}
	nodeID := strings.TrimSpace(req.ID)
	now := time.Now().UTC().Format(time.RFC3339)
	detailsBytes, _ := json.Marshal(req.Details)
	if detailsBytes == nil {
		detailsBytes = []byte("{}")
	}

	if nodeID == "" {
		nodeID = ids.New("node")
		_, err := s.st.DB().ExecContext(
			r.Context(),
			`INSERT INTO project_graph_nodes (id, project_id, name, kind, faction, summary, details_json, x, y, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			nodeID, projectID, req.Name, req.Kind, req.Faction, req.Summary, string(detailsBytes), req.X, req.Y, now, now,
		)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", err.Error())
			return
		}
	} else {
		_, err := s.st.DB().ExecContext(
			r.Context(),
			`UPDATE project_graph_nodes
			 SET name = ?, kind = ?, faction = ?, summary = ?, details_json = ?, x = ?, y = ?, updated_at = ?
			 WHERE id = ? AND project_id = ?`,
			req.Name, req.Kind, req.Faction, req.Summary, string(detailsBytes), req.X, req.Y, now, nodeID, projectID,
		)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", err.Error())
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{"id": nodeID, "ok": true})
}

func (s *Server) handleDeleteGraphNode(w http.ResponseWriter, r *http.Request) {
	userID, err := s.currentUserID(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}
	projectID := r.PathValue("id")
	nodeID := r.PathValue("nodeId")
	if err := s.requireOwnedProject(r.Context(), projectID, userID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "not_found", "not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "invalid", "internal error")
		return
	}

	// Delete incident edges first
	_, _ = s.st.DB().ExecContext(
		r.Context(),
		`DELETE FROM project_graph_edges WHERE project_id = ? AND (source_id = ? OR target_id = ?)`,
		projectID, nodeID, nodeID,
	)

	// Delete node
	_, err = s.st.DB().ExecContext(
		r.Context(),
		`DELETE FROM project_graph_nodes WHERE project_id = ? AND id = ?`,
		projectID, nodeID,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleSaveGraphEdge(w http.ResponseWriter, r *http.Request) {
	userID, err := s.currentUserID(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}
	projectID := r.PathValue("id")
	if err := s.requireOwnedProject(r.Context(), projectID, userID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "not_found", "not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "invalid", "internal error")
		return
	}

	var req struct {
		ID          string  `json:"id"`
		SourceID    string  `json:"source_id"`
		TargetID    string  `json:"target_id"`
		Relation    string  `json:"relation"`
		Description string  `json:"description"`
		Strength    float64 `json:"strength"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid", "invalid json")
		return
	}
	if req.SourceID == "" || req.TargetID == "" {
		writeError(w, http.StatusBadRequest, "invalid", "source_id and target_id are required")
		return
	}
	req.Relation = strings.TrimSpace(req.Relation)
	if req.Relation == "" {
		req.Relation = "关联"
	}
	if req.Strength <= 0 {
		req.Strength = 1.0
	}
	edgeID := strings.TrimSpace(req.ID)
	now := time.Now().UTC().Format(time.RFC3339)

	if edgeID == "" {
		edgeID = ids.New("edge")
		_, err := s.st.DB().ExecContext(
			r.Context(),
			`INSERT INTO project_graph_edges (id, project_id, source_id, target_id, relation, description, strength, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			edgeID, projectID, req.SourceID, req.TargetID, req.Relation, req.Description, req.Strength, now, now,
		)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", err.Error())
			return
		}
	} else {
		_, err := s.st.DB().ExecContext(
			r.Context(),
			`UPDATE project_graph_edges
			 SET source_id = ?, target_id = ?, relation = ?, description = ?, strength = ?, updated_at = ?
			 WHERE id = ? AND project_id = ?`,
			req.SourceID, req.TargetID, req.Relation, req.Description, req.Strength, now, edgeID, projectID,
		)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", err.Error())
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{"id": edgeID, "ok": true})
}

func (s *Server) handleDeleteGraphEdge(w http.ResponseWriter, r *http.Request) {
	userID, err := s.currentUserID(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}
	projectID := r.PathValue("id")
	edgeID := r.PathValue("edgeId")
	if err := s.requireOwnedProject(r.Context(), projectID, userID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "not_found", "not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "invalid", "internal error")
		return
	}

	_, err = s.st.DB().ExecContext(
		r.Context(),
		`DELETE FROM project_graph_edges WHERE project_id = ? AND id = ?`,
		projectID, edgeID,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleExtractGraph(w http.ResponseWriter, r *http.Request) {
	userID, err := s.currentUserID(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}
	projectID := r.PathValue("id")
	if err := s.requireOwnedProject(r.Context(), projectID, userID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "not_found", "not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "invalid", "internal error")
		return
	}

	// 1. Gather all chapters body & briefs and bible content
	var chapterSnippets []string
	rows, err := s.st.DB().QueryContext(
		r.Context(),
		`SELECT seq, title, brief, body FROM chapters WHERE project_id = ? ORDER BY seq ASC`,
		projectID,
	)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var seq int
			var title, brief, body string
			if err := rows.Scan(&seq, &title, &brief, &body); err == nil {
				text := body
				if len([]rune(text)) > 1500 {
					text = string([]rune(text)[:1500]) + "..."
				}
				chapterSnippets = append(chapterSnippets, fmt.Sprintf("【第%d章 %s】\n概要：%s\n正文节选：%s", seq, title, brief, text))
			}
		}
	}

	var bibleSnippets []string
	bRows, err := s.st.DB().QueryContext(
		r.Context(),
		`SELECT kind, name, content FROM bible_entries WHERE project_id = ? AND status = 'active'`,
		projectID,
	)
	if err == nil {
		defer bRows.Close()
		for bRows.Next() {
			var kind, name, content string
			if err := bRows.Scan(&kind, &name, &content); err == nil {
				bibleSnippets = append(bibleSnippets, fmt.Sprintf("[%s] %s: %s", kind, name, content))
			}
		}
	}

	combinedContext := strings.Join(chapterSnippets, "\n\n") + "\n\n世界设定参考：\n" + strings.Join(bibleSnippets, "\n")
	if strings.TrimSpace(combinedContext) == "" {
		writeError(w, http.StatusBadRequest, "invalid", "项目中暂无章节或设定内容，无法提炼实体图谱")
		return
	}

	key, err := s.loadUserLLMKey(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid", "未配置 LLM 模型密钥，请前往设置配置")
		return
	}

	// 2. Call LLM to extract entities and relations
	systemPrompt := `你是一名资深网络小说架构师与世界观实体图谱分析专家。
请仔细阅读提供的小说章节节选和世界设定，抽离出关键的【实体节点】（人物、门派势力、法宝神器、关键地理）以及彼此之间的【关系网络】（盟友、宿敌、师徒、暗恋、君臣、道侣、同门、死敌等）。

必须严格返回 JSON 格式，不要包含任何 markdown 标记或附加说明，格式如下：
{
  "nodes": [
    {
      "name": "实体名称（如：韩立、落云宗、掌天瓶）",
      "kind": "character | faction | artifact | location",
      "faction": "所属势力/阵营名称（如：落云宗、魔道六宗、散修）",
      "summary": "一句话核心身份/定位与特征",
      "details": {
        "realm": "当前境界/等级（若适用）",
        "temperament": "性格/脾气与为人准则",
        "secrets": "秘密/动机/底牌"
      }
    }
  ],
  "edges": [
    {
      "source": "源实体名称（必须与 nodes 中的 name 一致）",
      "target": "目标实体名称（必须与 nodes 中的 name 一致）",
      "relation": "关系简短名称（如：盟友 / 宿敌 / 师徒 / 暗恋 / 主仆 / 仇怨）",
      "description": "详细关系渊源或冲突焦点"
    }
  ]
}`

	outText, err := s.llm.Chat(r.Context(), llm.Request{
		Provider: key.provider,
		BaseURL:  key.baseURL,
		APIKey:   key.apiKey,
		Model:    resolveModel(""),
		Temp:     0.2,
		Messages: []llm.Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: combinedContext},
		},
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "llm_error", "AI 提炼失败: "+err.Error())
		return
	}

	outText = cleanJSONMarkdown(outText)

	var extracted struct {
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
	if err := json.Unmarshal([]byte(outText), &extracted); err != nil {
		writeError(w, http.StatusInternalServerError, "llm_error", "解析 AI 提炼结果失败: "+err.Error())
		return
	}

	// 3. Save extracted nodes and map names to IDs
	nameToID := make(map[string]string)
	now := time.Now().UTC().Format(time.RFC3339)

	// Fetch existing nodes to avoid duplicates
	existingRows, _ := s.st.DB().QueryContext(r.Context(), `SELECT id, name FROM project_graph_nodes WHERE project_id = ?`, projectID)
	if existingRows != nil {
		for existingRows.Next() {
			var id, name string
			_ = existingRows.Scan(&id, &name)
			nameToID[name] = id
		}
		existingRows.Close()
	}

	for _, n := range extracted.Nodes {
		n.Name = strings.TrimSpace(n.Name)
		if n.Name == "" {
			continue
		}
		if n.Kind == "" {
			n.Kind = "character"
		}
		detailsBytes, _ := json.Marshal(n.Details)
		if detailsBytes == nil {
			detailsBytes = []byte("{}")
		}

		if existingID, exists := nameToID[n.Name]; exists {
			// Update
			_, _ = s.st.DB().ExecContext(
				r.Context(),
				`UPDATE project_graph_nodes
				 SET kind = ?, faction = ?, summary = ?, details_json = ?, updated_at = ?
				 WHERE id = ? AND project_id = ?`,
				n.Kind, n.Faction, n.Summary, string(detailsBytes), now, existingID, projectID,
			)
		} else {
			// Insert
			newID := ids.New("node")
			nameToID[n.Name] = newID
			_, _ = s.st.DB().ExecContext(
				r.Context(),
				`INSERT INTO project_graph_nodes (id, project_id, name, kind, faction, summary, details_json, x, y, created_at, updated_at)
				 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				newID, projectID, n.Name, n.Kind, n.Faction, n.Summary, string(detailsBytes), 0, 0, now, now,
			)
		}
	}

	// 4. Save extracted edges
	for _, e := range extracted.Edges {
		srcID, ok1 := nameToID[strings.TrimSpace(e.Source)]
		tgtID, ok2 := nameToID[strings.TrimSpace(e.Target)]
		if !ok1 || !ok2 || srcID == tgtID {
			continue
		}
		e.Relation = strings.TrimSpace(e.Relation)
		if e.Relation == "" {
			e.Relation = "关联"
		}
		edgeID := ids.New("edge")
		_, _ = s.st.DB().ExecContext(
			r.Context(),
			`INSERT INTO project_graph_edges (id, project_id, source_id, target_id, relation, description, strength, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			edgeID, projectID, srcID, tgtID, e.Relation, e.Description, 1.0, now, now,
		)
	}

	// 5. Return updated graph
	s.handleGetGraph(w, r)
}

func cleanJSONMarkdown(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```json") {
		s = strings.TrimPrefix(s, "```json")
		s = strings.TrimSuffix(s, "```")
	} else if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```")
		s = strings.TrimSuffix(s, "```")
	}
	return strings.TrimSpace(s)
}
