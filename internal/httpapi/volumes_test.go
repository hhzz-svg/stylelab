package httpapi_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

type structureView struct {
	Volumes []struct {
		ID       string `json:"id"`
		Title    string `json:"title"`
		StartSeq int    `json:"start_seq"`
		Written  int    `json:"written"`
		Chapters []struct {
			ID          string `json:"id"`
			Seq         int    `json:"seq"`
			ScenesStale bool   `json:"scenes_stale"`
			Scenes      []struct {
				Title string `json:"title"`
			} `json:"scenes"`
		} `json:"chapters"`
	} `json:"volumes"`
}

func getStructure(t *testing.T, srv *httptest.Server, c *http.Client, projectID string) (int, structureView) {
	t.Helper()
	resp := mustGet(t, c, srv.URL+"/api/projects/"+projectID+"/structure")
	defer resp.Body.Close()
	var v structureView
	if resp.StatusCode == http.StatusOK {
		if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
			t.Fatalf("decode structure: %v", err)
		}
	}
	return resp.StatusCode, v
}

// shape renders the tree as "title:seq,seq|title:seq" for comparison.
func shape(v structureView) string {
	out := ""
	for i, vol := range v.Volumes {
		if i > 0 {
			out += "|"
		}
		out += vol.Title + ":"
		for j, c := range vol.Chapters {
			if j > 0 {
				out += ","
			}
			out += fmt.Sprint(c.Seq)
		}
	}
	return out
}

func createVolume(t *testing.T, srv *httptest.Server, c *http.Client, projectID, payload string) (int, string) {
	t.Helper()
	resp := postJSON(t, c, srv.URL+"/api/projects/"+projectID+"/volumes", payload)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		return resp.StatusCode, ""
	}
	id, _ := decodeJSON(t, resp)["id"].(string)
	return resp.StatusCode, id
}

func TestVolumesShapeTheStructure(t *testing.T) {
	srv, _ := newStudioServer(t)
	owner := registerUser(t, srv, "volumes@example.com")
	intruder := registerUser(t, srv, "volumes-intruder@example.com")
	projectID := createProject(t, srv, owner, "分卷")

	var chapters []string
	for i := 1; i <= 5; i++ {
		chapters = append(chapters, createChapter(t, srv, owner, projectID, fmt.Sprintf("第%d章", i), "梗概"))
	}
	if _, v := getStructure(t, srv, owner, projectID); shape(v) != ":1,2,3,4,5" || v.Volumes[0].ID != "" {
		t.Fatalf("no volumes: %q", shape(v))
	}

	_, first := createVolume(t, srv, owner, projectID, `{"start_seq":1,"title":"上卷","brief":"开局"}`)
	_, second := createVolume(t, srv, owner, projectID, `{"start_seq":4,"title":" 下卷 "}`)
	if _, v := getStructure(t, srv, owner, projectID); shape(v) != "上卷:1,2,3|下卷:4,5" {
		t.Fatalf("two volumes: %q", shape(v))
	}
	for payload, why := range map[string]string{
		`{"start_seq":4,"title":"又一卷"}`: "start taken",
		`{"start_seq":2,"title":"  "}`:  "blank title",
		`{"start_seq":0,"title":"零"}`:   "start below 1",
	} {
		if code, _ := createVolume(t, srv, owner, projectID, payload); code != http.StatusBadRequest {
			t.Errorf("%s: %d, want 400", why, code)
		}
	}

	// Move the second volume's start back one chapter.
	resp := patchJSON(t, owner, srv.URL+"/api/volumes/"+second, `{"start_seq":3}`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("patch volume: %d", resp.StatusCode)
	}
	if _, v := getStructure(t, srv, owner, projectID); shape(v) != "上卷:1,2|下卷:3,4,5" {
		t.Fatalf("after move: %q", shape(v))
	}

	// Scenes hang under their chapter, and go stale with the body.
	setBody(t, srv, owner, chapters[2], threeSceneBody)
	resp = postJSON(t, owner, srv.URL+"/api/chapters/"+chapters[2]+"/scenes/split", `{}`)
	resp.Body.Close()
	_, v := getStructure(t, srv, owner, projectID)
	if c := v.Volumes[1].Chapters[0]; len(c.Scenes) != 3 || c.ScenesStale || v.Volumes[1].Written != 1 {
		t.Fatalf("chapter 3 in tree: %+v, written %d", c, v.Volumes[1].Written)
	}
	setBody(t, srv, owner, chapters[2], threeSceneBody+"\n尾声。")
	if _, v := getStructure(t, srv, owner, projectID); !v.Volumes[1].Chapters[0].ScenesStale {
		t.Fatal("scenes should read stale after the body changed")
	}

	// Another user can do nothing with any of it.
	if code, _ := getStructure(t, srv, intruder, projectID); code != http.StatusNotFound {
		t.Errorf("structure: %d", code)
	}
	if code, _ := createVolume(t, srv, intruder, projectID, `{"start_seq":2,"title":"入侵"}`); code != http.StatusNotFound {
		t.Errorf("create: %d", code)
	}
	resp = patchJSON(t, intruder, srv.URL+"/api/volumes/"+first, `{"title":"篡改"}`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("patch: %d", resp.StatusCode)
	}
	if code := mustDelete(t, intruder, srv.URL+"/api/volumes/"+first); code != http.StatusNotFound {
		t.Errorf("delete: %d", code)
	}

	// Deleting the first volume leaves its chapters unassigned.
	if code := mustDelete(t, owner, srv.URL+"/api/volumes/"+first); code != http.StatusOK {
		t.Fatalf("delete: %d", code)
	}
	if _, v := getStructure(t, srv, owner, projectID); shape(v) != ":1,2|下卷:3,4,5" {
		t.Fatalf("after delete: %q", shape(v))
	}
}

func TestOutlineImportKeepsVolumes(t *testing.T) {
	srv, _ := newStudioServer(t)
	owner := registerUser(t, srv, "outline-volumes@example.com")
	projectID := createProject(t, srv, owner, "大纲分卷")
	importURL := srv.URL + "/api/projects/" + projectID + "/outline/import"
	chapters := `[{"title":"一","brief":"甲"},{"title":"二","brief":"乙"},{"title":"三","brief":"丙"}]`

	// Counts that do not add up are refused before anything is written.
	resp := postJSON(t, owner, importURL, `{"chapters":`+chapters+`,"volumes":[{"title":"卷一","chapter_count":2}]}`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("mismatched volumes: %d", resp.StatusCode)
	}
	if n := len(listChapters(t, srv, owner, projectID)); n != 0 {
		t.Fatalf("a refused import wrote %d chapters", n)
	}

	resp = postJSON(t, owner, importURL,
		`{"chapters":`+chapters+`,"volumes":[{"title":"卷一","brief":"开局","chapter_count":1},{"title":"","chapter_count":0},{"title":"卷三","chapter_count":2}]}`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("import: %d", resp.StatusCode)
	}
	// The empty volume is skipped.
	if _, v := getStructure(t, srv, owner, projectID); shape(v) != "卷一:1|卷三:2,3" {
		t.Fatalf("imported: %q", shape(v))
	}

	// Replacing the (unwritten) outline replaces its volumes too.
	resp = postJSON(t, owner, importURL,
		`{"chapters":[{"title":"新一","brief":"甲"},{"title":"新二","brief":"乙"}],"replace_existing":true,"volumes":[{"title":"新卷","chapter_count":2}]}`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("replace: %d", resp.StatusCode)
	}
	if _, v := getStructure(t, srv, owner, projectID); shape(v) != "新卷:1,2" {
		t.Fatalf("replaced: %q", shape(v))
	}
}

// Deleting a chapter renumbers the ones after it; volumes must move with
// them, and a volume left with no chapter must go.
func TestVolumesFollowChapterDeletes(t *testing.T) {
	srv, _ := newStudioServer(t)
	owner := registerUser(t, srv, "volumes-delete@example.com")
	projectID := createProject(t, srv, owner, "删章")
	var chapters []string
	for i := 1; i <= 6; i++ {
		chapters = append(chapters, createChapter(t, srv, owner, projectID, fmt.Sprintf("第%d章", i), "梗概"))
	}
	// Created last-first, so a naive one-row-at-a-time shift would collide
	// on the unique start.
	for _, v := range []string{`{"start_seq":6,"title":"丁"}`, `{"start_seq":4,"title":"丙"}`, `{"start_seq":3,"title":"乙"}`, `{"start_seq":1,"title":"甲"}`} {
		if code, _ := createVolume(t, srv, owner, projectID, v); code != http.StatusCreated {
			t.Fatalf("create %s: %d", v, code)
		}
	}
	if _, v := getStructure(t, srv, owner, projectID); shape(v) != "甲:1,2|乙:3|丙:4,5|丁:6" {
		t.Fatalf("before: %q", shape(v))
	}

	// Chapter 3 was all of 乙: 乙 goes, 丙 and 丁 move up.
	if code := mustDelete(t, owner, srv.URL+"/api/chapters/"+chapters[2]); code != http.StatusOK && code != http.StatusNoContent {
		t.Fatalf("delete chapter 3: %d", code)
	}
	if _, v := getStructure(t, srv, owner, projectID); shape(v) != "甲:1,2|丙:3,4|丁:5" {
		t.Fatalf("after deleting 3: %q", shape(v))
	}

	// Deleting 甲's first chapter keeps 甲 on what was its second.
	if code := mustDelete(t, owner, srv.URL+"/api/chapters/"+chapters[0]); code != http.StatusOK && code != http.StatusNoContent {
		t.Fatalf("delete chapter 1: %d", code)
	}
	_, v := getStructure(t, srv, owner, projectID)
	if shape(v) != "甲:1|丙:2,3|丁:4" || v.Volumes[0].Chapters[0].ID != chapters[1] {
		t.Fatalf("after deleting 1: %q", shape(v))
	}
}
