package httpapi_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

type analysisView struct {
	NodeCount int `json:"node_count"`
	EdgeCount int `json:"edge_count"`
	Ranking   []struct {
		ID          string  `json:"id"`
		Score       int     `json:"score"`
		Betweenness float64 `json:"betweenness"`
		Degree      int     `json:"degree"`
		Rank        int     `json:"rank"`
		Role        string  `json:"role"`
	} `json:"ranking"`
}

func getAnalysis(t *testing.T, srv *httptest.Server, c *http.Client, projectID string) (int, analysisView) {
	t.Helper()
	resp := mustGet(t, c, srv.URL+"/api/projects/"+projectID+"/graph/analysis")
	defer resp.Body.Close()
	var a analysisView
	if resp.StatusCode == http.StatusOK {
		if err := json.NewDecoder(resp.Body).Decode(&a); err != nil {
			t.Fatalf("decode analysis: %v", err)
		}
	}
	return resp.StatusCode, a
}

func TestGraphAnalysisRanksTheStarCentre(t *testing.T) {
	srv, _ := newStudioServer(t)
	owner := registerUser(t, srv, "analysis@example.com")
	projectID := createProject(t, srv, owner, "重要度")

	_, centre := saveNode(t, srv, owner, projectID, `{"name":"林远"}`)
	var leaves []string
	for i := 1; i <= 4; i++ {
		_, id := saveNode(t, srv, owner, projectID, fmt.Sprintf(`{"name":"配角%d"}`, i))
		leaves = append(leaves, id)
		saveEdge(t, srv, owner, projectID, fmt.Sprintf(`{"source_id":%q,"target_id":%q,"relation":"同门"}`, centre, id))
	}
	_, loner := saveNode(t, srv, owner, projectID, `{"name":"路人"}`)

	code, a := getAnalysis(t, srv, owner, projectID)
	if code != http.StatusOK {
		t.Fatalf("status %d", code)
	}
	if a.NodeCount != 6 || a.EdgeCount != 4 || len(a.Ranking) != 6 {
		t.Fatalf("counts = %d nodes, %d edges, %d ranked", a.NodeCount, a.EdgeCount, len(a.Ranking))
	}
	top := a.Ranking[0]
	if top.ID != centre || top.Rank != 1 || top.Score != 100 || top.Role != "core" || top.Degree != 4 {
		t.Fatalf("top = %+v, want the centre as core", top)
	}
	roles := map[string]string{}
	for _, r := range a.Ranking {
		roles[r.ID] = r.Role
	}
	for _, id := range leaves {
		if roles[id] != "peripheral" {
			t.Fatalf("leaf role = %q", roles[id])
		}
	}
	if roles[loner] != "isolated" {
		t.Fatalf("loner role = %q", roles[loner])
	}
}

func TestGraphAnalysisEmptyAndOtherUser(t *testing.T) {
	srv, _ := newStudioServer(t)
	owner := registerUser(t, srv, "analysis-empty@example.com")
	intruder := registerUser(t, srv, "analysis-intruder@example.com")
	projectID := createProject(t, srv, owner, "空图谱")

	code, a := getAnalysis(t, srv, owner, projectID)
	if code != http.StatusOK || a.Ranking == nil || len(a.Ranking) != 0 {
		t.Fatalf("empty project: status %d ranking %v (want [] not null)", code, a.Ranking)
	}
	if code, _ := getAnalysis(t, srv, intruder, projectID); code != http.StatusNotFound {
		t.Fatalf("other user: %d, want 404", code)
	}
}
