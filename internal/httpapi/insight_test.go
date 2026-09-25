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
	Communities []struct {
		Index    int      `json:"index"`
		Faction  string   `json:"faction"`
		Members  []string `json:"members"`
		Outliers []struct {
			ID      string `json:"id"`
			Faction string `json:"faction"`
		} `json:"outliers"`
	} `json:"communities"`
	Modularity float64 `json:"modularity"`
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

func TestGraphAnalysisFindsFactionsAndTheSpy(t *testing.T) {
	srv, _ := newStudioServer(t)
	owner := registerUser(t, srv, "communities@example.com")
	projectID := createProject(t, srv, owner, "阵营")

	add := func(name, faction string) string {
		_, id := saveNode(t, srv, owner, projectID, fmt.Sprintf(`{"name":%q,"faction":%q}`, name, faction))
		return id
	}
	link := func(a, b string) {
		saveEdge(t, srv, owner, projectID, fmt.Sprintf(`{"source_id":%q,"target_id":%q,"relation":"往来"}`, a, b))
	}
	// 青云宗: three disciples, plus a man labelled 魔门 who spends all his
	// time with them. 魔门: three members. One thin tie between the camps.
	q := []string{add("林远", "青云宗"), add("苏晚", "青云宗"), add("赵长老", "青云宗"), add("卧底", "魔门")}
	m := []string{add("魔尊", "魔门"), add("血影", "魔门"), add("鬼婆", "魔门")}
	for _, group := range [][]string{q, m} {
		for i := range group {
			for j := i + 1; j < len(group); j++ {
				link(group[i], group[j])
			}
		}
	}
	link(q[0], m[0])

	code, a := getAnalysis(t, srv, owner, projectID)
	if code != http.StatusOK {
		t.Fatalf("status %d", code)
	}
	if len(a.Communities) != 2 {
		t.Fatalf("communities = %+v", a.Communities)
	}
	qc, mc := a.Communities[0], a.Communities[1]
	if qc.Faction != "青云宗" || len(qc.Members) != 4 || mc.Faction != "魔门" || len(mc.Members) != 3 {
		t.Fatalf("got %+v / %+v", qc, mc)
	}
	if len(qc.Outliers) != 1 || qc.Outliers[0].ID != q[3] || qc.Outliers[0].Faction != "魔门" {
		t.Fatalf("the spy should stand out: %+v", qc.Outliers)
	}
	if len(mc.Outliers) != 0 {
		t.Fatalf("魔门 outliers = %+v", mc.Outliers)
	}
	if a.Modularity < 0.3 {
		t.Fatalf("modularity %.3f", a.Modularity)
	}
}

type lineageNodeView struct {
	ID       string            `json:"id"`
	Virtual  bool              `json:"virtual"`
	Name     string            `json:"name"`
	Relation string            `json:"relation"`
	X        float64           `json:"x"`
	Depth    int               `json:"depth"`
	Children []lineageNodeView `json:"children"`
}

func getLineage(t *testing.T, srv *httptest.Server, c *http.Client, projectID string) (int, []lineageNodeView) {
	t.Helper()
	resp := mustGet(t, c, srv.URL+"/api/projects/"+projectID+"/graph/lineage")
	defer resp.Body.Close()
	var body struct {
		Roots []lineageNodeView `json:"roots"`
	}
	if resp.StatusCode == http.StatusOK {
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatalf("decode lineage: %v", err)
		}
	}
	return resp.StatusCode, body.Roots
}

// path returns the names from the root down to the node called name.
func lineagePath(roots []lineageNodeView, name string) []string {
	for _, r := range roots {
		if r.Name == name {
			return []string{r.Name}
		}
		if p := lineagePath(r.Children, name); p != nil {
			return append([]string{r.Name}, p...)
		}
	}
	return nil
}

func TestGraphLineageFollowsMastersAndSwaps(t *testing.T) {
	srv, _ := newStudioServer(t)
	owner := registerUser(t, srv, "lineage@example.com")
	intruder := registerUser(t, srv, "lineage-intruder@example.com")
	projectID := createProject(t, srv, owner, "谱系")

	_, _ = saveNode(t, srv, owner, projectID, `{"name":"青云宗","kind":"faction"}`)
	_, master := saveNode(t, srv, owner, projectID, `{"name":"赵长老","faction":"青云宗"}`)
	_, disciple := saveNode(t, srv, owner, projectID, `{"name":"林远","faction":"青云宗"}`)
	_, edge := saveEdge(t, srv, owner, projectID, fmt.Sprintf(`{"source_id":%q,"target_id":%q,"relation":"师徒"}`, master, disciple))

	code, roots := getLineage(t, srv, owner, projectID)
	if code != http.StatusOK {
		t.Fatalf("status %d", code)
	}
	if got := lineagePath(roots, "林远"); fmt.Sprint(got) != "[青云宗 赵长老 林远]" {
		t.Fatalf("path = %v", got)
	}

	// Swapping the edge's direction puts the other one on top.
	if code, _ := saveEdge(t, srv, owner, projectID,
		fmt.Sprintf(`{"id":%q,"source_id":%q,"target_id":%q,"relation":"师徒"}`, edge, disciple, master)); code != http.StatusOK {
		t.Fatalf("swap: %d", code)
	}
	_, roots = getLineage(t, srv, owner, projectID)
	if got := lineagePath(roots, "赵长老"); fmt.Sprint(got) != "[青云宗 林远 赵长老]" {
		t.Fatalf("after swap path = %v", got)
	}

	if code, _ := getLineage(t, srv, intruder, projectID); code != http.StatusNotFound {
		t.Fatalf("other user: %d", code)
	}
}
