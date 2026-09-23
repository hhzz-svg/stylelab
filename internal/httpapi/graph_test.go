package httpapi_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"unicode/utf8"
)

// saveNode posts a node and returns the response status and id.
func saveNode(t *testing.T, srv *httptest.Server, c *http.Client, projectID, payload string) (int, string) {
	t.Helper()
	resp := postJSON(t, c, srv.URL+"/api/projects/"+projectID+"/graph/nodes", payload)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return resp.StatusCode, ""
	}
	id, _ := decodeJSON(t, resp)["id"].(string)
	return resp.StatusCode, id
}

func saveEdge(t *testing.T, srv *httptest.Server, c *http.Client, projectID, payload string) (int, string) {
	t.Helper()
	resp := postJSON(t, c, srv.URL+"/api/projects/"+projectID+"/graph/edges", payload)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return resp.StatusCode, ""
	}
	id, _ := decodeJSON(t, resp)["id"].(string)
	return resp.StatusCode, id
}

type graphView struct {
	Nodes []struct {
		ID      string  `json:"id"`
		Name    string  `json:"name"`
		Kind    string  `json:"kind"`
		Faction string  `json:"faction"`
		Summary string  `json:"summary"`
		X       float64 `json:"x"`
		Y       float64 `json:"y"`
	} `json:"nodes"`
	Edges []struct {
		ID       string `json:"id"`
		SourceID string `json:"source_id"`
		TargetID string `json:"target_id"`
		Relation string `json:"relation"`
	} `json:"edges"`
}

func getGraph(t *testing.T, srv *httptest.Server, c *http.Client, projectID string) graphView {
	t.Helper()
	resp := mustGet(t, c, srv.URL+"/api/projects/"+projectID+"/graph")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get graph: %d", resp.StatusCode)
	}
	var g graphView
	if err := json.NewDecoder(resp.Body).Decode(&g); err != nil {
		t.Fatalf("decode graph: %v", err)
	}
	return g
}

func mustDelete(t *testing.T, c *http.Client, rawURL string) int {
	t.Helper()
	resp, err := deleteReq(t, c, rawURL)
	if err != nil {
		t.Fatalf("DELETE %s: %v", rawURL, err)
	}
	resp.Body.Close()
	return resp.StatusCode
}

func TestGraphNodeAndEdgeCRUD(t *testing.T) {
	srv, _ := newStudioServer(t)
	owner := registerUser(t, srv, "graph-crud@example.com")
	projectID := createProject(t, srv, owner, "图谱")

	if g := getGraph(t, srv, owner, projectID); len(g.Nodes) != 0 || len(g.Edges) != 0 {
		t.Fatalf("new project graph = %+v, want empty", g)
	}

	_, a := saveNode(t, srv, owner, projectID, `{"name":"林远","kind":"character","faction":"青云宗","x":120,"y":80}`)
	_, b := saveNode(t, srv, owner, projectID, `{"name":"  苏晚  "}`)
	if a == "" || b == "" {
		t.Fatal("node ids missing")
	}
	if code, _ := saveNode(t, srv, owner, projectID, `{"name":"   "}`); code != http.StatusBadRequest {
		t.Fatalf("blank name: %d, want 400", code)
	}

	// Update in place by id.
	if code, id := saveNode(t, srv, owner, projectID,
		fmt.Sprintf(`{"id":%q,"name":"林远","kind":"character","faction":"魔门","summary":"叛出师门"}`, a)); code != http.StatusOK || id != a {
		t.Fatalf("update node: %d id=%q", code, id)
	}
	// An id that is not a node of this project is not silently "saved".
	if code, _ := saveNode(t, srv, owner, projectID, `{"id":"node_missing","name":"幽灵"}`); code != http.StatusNotFound {
		t.Fatalf("update unknown node: %d, want 404", code)
	}

	g := getGraph(t, srv, owner, projectID)
	if len(g.Nodes) != 2 {
		t.Fatalf("nodes = %d, want 2", len(g.Nodes))
	}
	byID := map[string]int{}
	for i, n := range g.Nodes {
		byID[n.ID] = i
	}
	// The profile drawer saves without x/y; the node must stay put.
	if n := g.Nodes[byID[a]]; n.Faction != "魔门" || n.Summary != "叛出师门" || n.X != 120 || n.Y != 80 {
		t.Fatalf("updated node = %+v", n)
	}
	if n := g.Nodes[byID[b]]; n.Name != "苏晚" || n.Kind != "character" {
		t.Fatalf("defaulted node = %+v", n)
	}

	_, e := saveEdge(t, srv, owner, projectID,
		fmt.Sprintf(`{"source_id":%q,"target_id":%q,"relation":" 宿敌 "}`, a, b))
	if e == "" {
		t.Fatal("edge id missing")
	}
	if code, _ := saveEdge(t, srv, owner, projectID,
		fmt.Sprintf(`{"id":"edge_missing","source_id":%q,"target_id":%q}`, a, b)); code != http.StatusNotFound {
		t.Fatalf("update unknown edge: %d, want 404", code)
	}
	g = getGraph(t, srv, owner, projectID)
	if len(g.Edges) != 1 || g.Edges[0].Relation != "宿敌" || g.Edges[0].SourceID != a {
		t.Fatalf("edges = %+v", g.Edges)
	}

	if code := mustDelete(t, owner, srv.URL+"/api/projects/"+projectID+"/graph/edges/"+e); code != http.StatusOK {
		t.Fatalf("delete edge: %d", code)
	}
	if g := getGraph(t, srv, owner, projectID); len(g.Edges) != 0 {
		t.Fatalf("edges after delete = %d", len(g.Edges))
	}

	// Deleting a node takes its edges with it.
	saveEdge(t, srv, owner, projectID, fmt.Sprintf(`{"source_id":%q,"target_id":%q,"relation":"师徒"}`, b, a))
	if code := mustDelete(t, owner, srv.URL+"/api/projects/"+projectID+"/graph/nodes/"+a); code != http.StatusOK {
		t.Fatalf("delete node: %d", code)
	}
	g = getGraph(t, srv, owner, projectID)
	if len(g.Nodes) != 1 || g.Nodes[0].ID != b || len(g.Edges) != 0 {
		t.Fatalf("after node delete: %+v", g)
	}
}

func TestGraphEdgeEndpointsMustBeProjectNodes(t *testing.T) {
	srv, _ := newStudioServer(t)
	owner := registerUser(t, srv, "graph-edges@example.com")
	projectID := createProject(t, srv, owner, "图谱甲")
	otherProject := createProject(t, srv, owner, "图谱乙")

	_, a := saveNode(t, srv, owner, projectID, `{"name":"甲"}`)
	_, b := saveNode(t, srv, owner, projectID, `{"name":"乙"}`)
	_, elsewhere := saveNode(t, srv, owner, otherProject, `{"name":"外人"}`)

	cases := []struct {
		name    string
		payload string
	}{
		{"missing ids", `{"relation":"盟友"}`},
		{"unknown node", fmt.Sprintf(`{"source_id":%q,"target_id":"node_nope"}`, a)},
		{"other project's node", fmt.Sprintf(`{"source_id":%q,"target_id":%q}`, a, elsewhere)},
		{"self loop", fmt.Sprintf(`{"source_id":%q,"target_id":%q}`, a, a)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if code, _ := saveEdge(t, srv, owner, projectID, tc.payload); code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", code)
			}
		})
	}
	if g := getGraph(t, srv, owner, projectID); len(g.Edges) != 0 {
		t.Fatalf("rejected edges were written: %+v", g.Edges)
	}
	if code, _ := saveEdge(t, srv, owner, projectID, fmt.Sprintf(`{"source_id":%q,"target_id":%q}`, a, b)); code != http.StatusOK {
		t.Fatalf("valid edge: %d", code)
	}
}

func TestGraphRoutesOtherUser404(t *testing.T) {
	srv, _ := newStudioServer(t)
	owner := registerUser(t, srv, "graph-owner@example.com")
	intruder := registerUser(t, srv, "graph-intruder@example.com")
	projectID := createProject(t, srv, owner, "私有图谱")
	_, a := saveNode(t, srv, owner, projectID, `{"name":"甲"}`)
	_, b := saveNode(t, srv, owner, projectID, `{"name":"乙"}`)
	_, e := saveEdge(t, srv, owner, projectID, fmt.Sprintf(`{"source_id":%q,"target_id":%q}`, a, b))
	base := srv.URL + "/api/projects/" + projectID + "/graph"

	resp := mustGet(t, intruder, base)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("GET graph: %d", resp.StatusCode)
	}
	if code, _ := saveNode(t, srv, intruder, projectID, fmt.Sprintf(`{"id":%q,"name":"篡改"}`, a)); code != http.StatusNotFound {
		t.Fatalf("POST node: %d", code)
	}
	if code, _ := saveEdge(t, srv, intruder, projectID, fmt.Sprintf(`{"source_id":%q,"target_id":%q}`, b, a)); code != http.StatusNotFound {
		t.Fatalf("POST edge: %d", code)
	}
	if code := mustDelete(t, intruder, base+"/edges/"+e); code != http.StatusNotFound {
		t.Fatalf("DELETE edge: %d", code)
	}
	if code := mustDelete(t, intruder, base+"/nodes/"+a); code != http.StatusNotFound {
		t.Fatalf("DELETE node: %d", code)
	}
	resp = postJSON(t, intruder, base+"/extract", `{}`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("extract: %d", resp.StatusCode)
	}

	g := getGraph(t, srv, owner, projectID)
	if len(g.Nodes) != 2 || len(g.Edges) != 1 || g.Nodes[0].Name == "篡改" || g.Nodes[1].Name == "篡改" {
		t.Fatalf("owner graph changed: %+v", g)
	}
}

func graphReply(nodes [][3]string, edges [][3]string) string {
	type node struct {
		Name    string `json:"name"`
		Kind    string `json:"kind"`
		Faction string `json:"faction"`
		Summary string `json:"summary"`
	}
	type edge struct {
		Source   string `json:"source"`
		Target   string `json:"target"`
		Relation string `json:"relation"`
	}
	out := struct {
		Nodes []node `json:"nodes"`
		Edges []edge `json:"edges"`
	}{Nodes: []node{}, Edges: []edge{}}
	for _, n := range nodes {
		out.Nodes = append(out.Nodes, node{Name: n[0], Kind: "character", Faction: n[1], Summary: n[2]})
	}
	for _, e := range edges {
		out.Edges = append(out.Edges, edge{Source: e[0], Target: e[1], Relation: e[2]})
	}
	raw, _ := json.Marshal(out)
	return "```json\n" + string(raw) + "\n```"
}

func TestGraphExtractRunsAsBoundedJob(t *testing.T) {
	srv, st := newStudioServer(t)
	owner := registerUser(t, srv, "graph-extract@example.com")
	projectID := createProject(t, srv, owner, "长篇")

	// A long book: 60 chapters of 2,000 runes each. The old inline handler sent
	// 1,500 runes of every one of them.
	const chapters = 60
	body := strings.Repeat("风", 2000)
	for i := 1; i <= chapters; i++ {
		id := createChapter(t, srv, owner, projectID, fmt.Sprintf("章%d", i), fmt.Sprintf("第%d章梗概", i))
		if _, err := st.DB().Exec(`UPDATE chapters SET body = ? WHERE id = ?`, body, id); err != nil {
			t.Fatalf("set body: %v", err)
		}
	}

	fake := newFakeLLM(t, graphReply(
		[][3]string{{"林远", "青云宗", "主角"}, {"苏晚", "", "师姐"}},
		[][3]string{{"林远", "苏晚", "同门"}, {"林远", "不存在的人", "宿敌"}, {"林远", "林远", "自省"}},
	))
	useFakeLLM(t, srv, owner, fake)

	path := "/api/projects/" + projectID + "/graph/extract"
	result := runStudioJob(t, srv, owner, path, `{}`)
	if got := intFromJSON(result["nodes_created"]); got != 2 {
		t.Fatalf("nodes_created = %d, want 2 (%v)", got, result)
	}
	// The edge to an unknown name and the self-loop are dropped.
	if got := intFromJSON(result["edges_created"]); got != 1 {
		t.Fatalf("edges_created = %d, want 1 (%v)", got, result)
	}
	if intFromJSON(result["chapters_read"]) != 30 || intFromJSON(result["chapters_total"]) != chapters {
		t.Fatalf("chapters read/total = %v/%v", result["chapters_read"], result["chapters_total"])
	}

	prompt := fake.lastUserPrompt(t)
	if n := utf8.RuneCountInString(prompt); n > 30000 {
		t.Fatalf("prompt is %d runes; extraction context is not bounded", n)
	}
	if !strings.Contains(prompt, "【第30章") || strings.Contains(prompt, "【第31章") {
		t.Fatal("prompt should carry exactly the first 30 chapters")
	}
	if !strings.Contains(prompt, "全书共 60 章") {
		t.Fatal("prompt should tell the model it only saw part of the book")
	}
	if model, _ := fake.lastCall(t)["model"].(string); strings.TrimSpace(model) == "" {
		t.Fatal("model sent to provider was empty")
	}

	g := getGraph(t, srv, owner, projectID)
	if len(g.Nodes) != 2 || len(g.Edges) != 1 || g.Edges[0].Relation != "同门" {
		t.Fatalf("graph after extract: %+v", g)
	}

	// The author fills in a faction by hand; a re-run whose reply leaves it
	// empty must neither wipe it nor duplicate the existing relation.
	var suWan string
	for _, n := range g.Nodes {
		if n.Name == "苏晚" {
			suWan = n.ID
		}
	}
	if code, _ := saveNode(t, srv, owner, projectID,
		fmt.Sprintf(`{"id":%q,"name":"苏晚","kind":"character","faction":"青云宗","summary":"师姐"}`, suWan)); code != http.StatusOK {
		t.Fatalf("hand edit: %d", code)
	}
	result = runStudioJob(t, srv, owner, path, `{}`)
	if intFromJSON(result["nodes_created"]) != 0 || intFromJSON(result["nodes_updated"]) != 2 || intFromJSON(result["edges_created"]) != 0 {
		t.Fatalf("re-run result = %v", result)
	}
	g = getGraph(t, srv, owner, projectID)
	if len(g.Nodes) != 2 || len(g.Edges) != 1 {
		t.Fatalf("re-run duplicated the graph: %+v", g)
	}
	for _, n := range g.Nodes {
		if n.Name == "苏晚" && n.Faction != "青云宗" {
			t.Fatalf("re-run wiped the hand-set faction: %+v", n)
		}
	}
}

func TestGraphExtractRejectsWhatTheCallerCanFix(t *testing.T) {
	srv, _ := newStudioServer(t)
	owner := registerUser(t, srv, "graph-extract-400@example.com")
	projectID := createProject(t, srv, owner, "空项目")
	path := srv.URL + "/api/projects/" + projectID + "/graph/extract"

	expect400 := func(t *testing.T, want string) {
		t.Helper()
		resp := postJSON(t, owner, path, `{}`)
		defer resp.Body.Close()
		raw, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusBadRequest || !strings.Contains(string(raw), want) {
			t.Fatalf("status %d body %s, want 400 mentioning %q", resp.StatusCode, raw, want)
		}
	}

	// Nothing to read.
	expect400(t, "暂无章节")
	// Material but no key.
	createChapter(t, srv, owner, projectID, "第一章", "开篇")
	expect400(t, "密钥")
}
