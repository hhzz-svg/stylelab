package httpapi_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
)

// POST /api/projects/{id}/cards had no test, and did not work: it inserted an
// "id" column into style_card_versions, which is keyed by (card_id, version)
// and has no such column, so every call returned 500. That took out manual card
// creation and the whole official-preset import on the card library page.

func TestCreateCardFromScratch(t *testing.T) {
	srv, _ := newStudioServer(t)
	owner := registerUser(t, srv, "cards-create@example.com")
	projectID := createProject(t, srv, owner, "手动建卡")

	payload := map[string]any{
		"name": "凛冬叙事",
		"kind": "manual",
		"dimensions": map[string]any{
			"sentence_rhythm": map[string]any{
				"level": 70, "summary": "短句推进", "techniques": []string{"断句"},
			},
		},
		"prohibitions": []string{"不得模仿具名作者"},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	resp := postJSON(t, owner, srv.URL+"/api/projects/"+projectID+"/cards", string(raw))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201", resp.StatusCode)
	}
	body := decodeJSON(t, resp)

	cardID, _ := body["id"].(string)
	if cardID == "" {
		t.Fatalf("no card id in %v", body)
	}
	if got, _ := body["name"].(string); got != "凛冬叙事" {
		t.Fatalf("name = %q", got)
	}
	if got := intFromJSON(body["version"]); got != 1 {
		t.Fatalf("version = %d, want 1", got)
	}

	// The version row must actually be readable back, which is what the bad
	// INSERT prevented.
	get := mustGet(t, owner, srv.URL+"/api/cards/"+cardID)
	defer get.Body.Close()
	if get.StatusCode != http.StatusOK {
		t.Fatalf("get card: %d", get.StatusCode)
	}
	loaded := decodeJSON(t, get)
	dims, ok := loaded["dimensions"].(map[string]any)
	if !ok {
		t.Fatalf("dimensions missing: %v", loaded)
	}
	// Supplied dimension survives, and the nine-dimension set is filled out.
	rhythm, ok := dims["sentence_rhythm"].(map[string]any)
	if !ok {
		t.Fatalf("sentence_rhythm missing: %v", dims)
	}
	if got := intFromJSON(rhythm["level"]); got != 70 {
		t.Fatalf("sentence_rhythm level = %d, want 70", got)
	}
	if len(dims) < 9 {
		t.Fatalf("got %d dimensions, want the full nine filled in", len(dims))
	}
}

func TestCreateCardDefaultsAndOwnership(t *testing.T) {
	srv, _ := newStudioServer(t)
	owner := registerUser(t, srv, "cards-create-def@example.com")
	intruder := registerUser(t, srv, "cards-create-other@example.com")
	projectID := createProject(t, srv, owner, "默认值")

	// An empty body still produces a usable card with all nine dimensions.
	resp := postJSON(t, owner, srv.URL+"/api/projects/"+projectID+"/cards", `{}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201", resp.StatusCode)
	}
	body := decodeJSON(t, resp)
	if got, _ := body["kind"].(string); got != "manual" {
		t.Fatalf("kind = %q, want manual", got)
	}
	if got, _ := body["name"].(string); got == "" {
		t.Fatal("name should fall back to a placeholder")
	}

	other := postJSON(t, intruder, srv.URL+"/api/projects/"+projectID+"/cards", `{"name":"越权"}`)
	defer other.Body.Close()
	if other.StatusCode != http.StatusNotFound {
		t.Fatalf("cross-user status = %d, want 404", other.StatusCode)
	}
}

// The card library imports six built-in preset cards one at a time through the
// same route; this is that loop in miniature.
func TestCreateCardRepeatedlyForPresetImport(t *testing.T) {
	srv, _ := newStudioServer(t)
	owner := registerUser(t, srv, "cards-presets@example.com")
	projectID := createProject(t, srv, owner, "预设导入")

	for i := 1; i <= 6; i++ {
		resp := postJSON(t, owner, srv.URL+"/api/projects/"+projectID+"/cards",
			fmt.Sprintf(`{"name":"预设 %d","kind":"manual","dimensions":{},"prohibitions":[]}`, i))
		status := resp.StatusCode
		resp.Body.Close()
		if status != http.StatusCreated {
			t.Fatalf("preset %d: status = %d, want 201", i, status)
		}
	}

	list := mustGet(t, owner, srv.URL+"/api/projects/"+projectID+"/cards")
	defer list.Body.Close()
	cards, _ := decodeJSON(t, list)["cards"].([]any)
	if len(cards) != 6 {
		t.Fatalf("card count = %d, want 6", len(cards))
	}
}
