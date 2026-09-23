package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"stylelab/internal/ids"
	"stylelab/internal/job"
	"stylelab/internal/studio"
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
	if err := nodeRows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
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
	if err := edgeRows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
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
		// Pointers: the profile drawer saves without coordinates, and an
		// omitted x/y must keep the node where it is rather than reset it to 0.
		X *float64 `json:"x"`
		Y *float64 `json:"y"`
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
	// json.Marshal(nil map) is "null", never nil, so check the map itself.
	detailsBytes := []byte("{}")
	if req.Details != nil {
		detailsBytes, _ = json.Marshal(req.Details)
	}

	if nodeID == "" {
		nodeID = ids.New("node")
		_, err := s.st.DB().ExecContext(
			r.Context(),
			`INSERT INTO project_graph_nodes (id, project_id, name, kind, faction, summary, details_json, x, y, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			nodeID, projectID, req.Name, req.Kind, req.Faction, req.Summary, string(detailsBytes), coord(req.X), coord(req.Y), now, now,
		)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", err.Error())
			return
		}
	} else {
		res, err := s.st.DB().ExecContext(
			r.Context(),
			`UPDATE project_graph_nodes
			 SET name = ?, kind = ?, faction = ?, summary = ?, details_json = ?,
			     x = COALESCE(?, x), y = COALESCE(?, y), updated_at = ?
			 WHERE id = ? AND project_id = ?`,
			req.Name, req.Kind, req.Faction, req.Summary, string(detailsBytes), req.X, req.Y, now, nodeID, projectID,
		)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", err.Error())
			return
		}
		// An id that is not a node of this project used to come back as a
		// successful save of nothing.
		if n, err := res.RowsAffected(); err == nil && n == 0 {
			writeError(w, http.StatusNotFound, "not_found", "not found")
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

	// The node and its edges go together: the edge delete's error used to be
	// discarded, which could leave edges pointing at a deleted node.
	tx, err := s.st.DB().BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(r.Context(),
		`DELETE FROM project_graph_edges WHERE project_id = ? AND (source_id = ? OR target_id = ?)`,
		projectID, nodeID, nodeID,
	); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if _, err := tx.ExecContext(r.Context(),
		`DELETE FROM project_graph_nodes WHERE project_id = ? AND id = ?`,
		projectID, nodeID,
	); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if err := tx.Commit(); err != nil {
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
	// Both ends must be nodes of this project. The table has no foreign key on
	// them, so without this an edge could point at nothing, or at another
	// project's node id.
	var ends int
	if err := s.st.DB().QueryRowContext(r.Context(),
		`SELECT COUNT(*) FROM project_graph_nodes WHERE project_id = ? AND id IN (?, ?)`,
		projectID, req.SourceID, req.TargetID,
	).Scan(&ends); err != nil {
		writeError(w, http.StatusInternalServerError, "invalid", "internal error")
		return
	}
	if req.SourceID == req.TargetID {
		writeError(w, http.StatusBadRequest, "invalid", "an edge needs two different nodes")
		return
	}
	if ends != 2 {
		writeError(w, http.StatusBadRequest, "invalid", "source_id and target_id must be nodes in this project")
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
		res, err := s.st.DB().ExecContext(
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
		if n, err := res.RowsAffected(); err == nil && n == 0 {
			writeError(w, http.StatusNotFound, "not_found", "not found")
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

// handleExtractGraph queues AI extraction of the relationship graph. It reads
// the manuscript and calls the model, so like the other studio features it
// runs as a job; the client polls, then re-reads the graph.
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

	var req struct {
		Model string `json:"model"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	// Answer the two things the caller can fix with a 400 now, not a job that
	// fails a minute later.
	var material int
	if err := s.st.DB().QueryRowContext(r.Context(),
		`SELECT (SELECT COUNT(*) FROM chapters WHERE project_id = ?)
		      + (SELECT COUNT(*) FROM bible_entries WHERE project_id = ? AND status = 'active')`,
		projectID, projectID,
	).Scan(&material); err != nil {
		writeError(w, http.StatusInternalServerError, "invalid", "internal error")
		return
	}
	if material == 0 {
		writeError(w, http.StatusBadRequest, "invalid", trimInvalid(studio.ErrNothingToExtract))
		return
	}
	if _, err := s.loadUserLLMKey(r.Context(), userID); err != nil {
		writeError(w, http.StatusBadRequest, "invalid", "未配置 LLM 模型密钥，请前往设置配置")
		return
	}

	s.enqueueStudioJob(w, r, userID, projectID, job.KindGraphExtract, studio.GraphInput{
		ProjectID: projectID,
		Model:     req.Model,
	})
}

// coord is a node coordinate for INSERT, where an omitted one means 0.
func coord(v *float64) float64 {
	if v == nil {
		return 0
	}
	return *v
}
