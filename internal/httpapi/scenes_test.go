package httpapi_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// A chapter of three scenes: the valley, 次日 the library, 与此同时 the
// palace. Each is written out twice to reach a realistic length: the
// default minimum scene is 300 runes.
var (
	valleyParas = []string{
		"林远在山谷中练剑，剑光如水，剑气纵横，惊起一片飞鸟。",
		"他收剑而立，山风拂过山谷，谷中松涛阵阵，剑穗轻摇。",
		"林远再次拔剑，剑锋所指，山石崩裂，碎石纷飞，剑气久久不散。",
		"剑意越来越凝练，林远的呼吸也越来越沉稳，剑与人仿佛合为一体。",
		"山谷深处的瀑布轰鸣，水雾打湿了他的衣衫，他却浑然不觉，只管练剑。",
		"一套剑法使完，林远收剑入鞘，望着山谷里被剑气削平的巨石出神。",
	}
	libraryParas = []string{
		"苏晚在藏经阁里翻阅古籍，书页泛黄，墨香扑鼻，烛火映着她的侧脸。",
		"她一卷卷地查找，指尖拂过书脊，古籍上的字迹早已模糊不清。",
		"藏经阁的书架高耸入顶，苏晚踩着木梯，从最上层抽出一本残破的典籍。",
		"典籍里记载着失传的阵法，苏晚看得入神，连烛泪流尽都没有察觉。",
		"她把要紧的段落抄在纸上，又将古籍小心放回原处，拂去书上的灰尘。",
		"夜读至此，藏经阁外传来更鼓声，苏晚合上书卷，揉了揉酸涩的眼睛。",
	}
	palaceParas = []string{
		"魔宫深处，魔尊端坐王座，血色灯火摇曳，照得殿中一片猩红。",
		"殿下跪着数名魔将，无人敢抬头，魔宫里只听得见灯芯爆裂的声音。",
		"魔尊缓缓开口，声音冰冷，命魔将们三日之内踏平正道的山门。",
		"魔将们领命退下，魔宫的大门轰然关闭，血色的雾气翻涌不息。",
		"魔尊独自坐在王座上，指尖敲着扶手，眼中闪过一丝阴鸷的杀意。",
		"王座之后的血池翻滚冒泡，魔尊的影子在猩红灯火中拉得很长。",
	}
	threeSceneBody = strings.Join(sceneSection("", valleyParas), "\n") + "\n" +
		strings.Join(sceneSection("次日，", libraryParas), "\n") + "\n" +
		strings.Join(sceneSection("与此同时，", palaceParas), "\n")
)

// sceneSection is the paragraphs twice over, the first opening with cue.
func sceneSection(cue string, paras []string) []string {
	out := append(append([]string(nil), paras...), paras...)
	out[0] = cue + out[0]
	return out
}

type scenesView struct {
	Scenes []struct {
		ID         string   `json:"id"`
		Index      int      `json:"index"`
		Start      int      `json:"start"`
		End        int      `json:"end"`
		Title      string   `json:"title"`
		Summary    string   `json:"summary"`
		Location   string   `json:"location"`
		Characters []string `json:"characters"`
		Cue        string   `json:"cue"`
		CueText    string   `json:"cue_text"`
		Origin     string   `json:"origin"`
	} `json:"scenes"`
	Stale bool `json:"stale"`
}

func decodeScenes(t *testing.T, resp *http.Response) scenesView {
	t.Helper()
	defer resp.Body.Close()
	var v scenesView
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		t.Fatalf("decode scenes: %v", err)
	}
	return v
}

func setBody(t *testing.T, srv *httptest.Server, c *http.Client, chapterID, body string) {
	t.Helper()
	raw, _ := json.Marshal(map[string]string{"body": body})
	resp := patchJSON(t, c, srv.URL+"/api/chapters/"+chapterID, string(raw))
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("patch body: %d", resp.StatusCode)
	}
}

func unitKind(t *testing.T, srv *httptest.Server, c *http.Client, projectID string) string {
	t.Helper()
	resp := mustGet(t, c, srv.URL+"/api/projects/"+projectID+"/graph/cooccurrence")
	defer resp.Body.Close()
	var r struct {
		UnitKind string `json:"unit_kind"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&r)
	return r.UnitKind
}

func TestSplitChapterIntoScenes(t *testing.T) {
	srv, st := newStudioServer(t)
	owner := registerUser(t, srv, "scenes@example.com")
	projectID := createProject(t, srv, owner, "场景")

	ids := map[string]string{}
	for _, n := range []struct{ name, kind string }{
		{"林远", "character"}, {"苏晚", "character"}, {"魔尊", "character"},
		{"山谷", "location"}, {"藏经阁", "location"}, {"魔宫", "location"},
	} {
		_, ids[n.name] = saveNode(t, srv, owner, projectID, fmt.Sprintf(`{"name":%q,"kind":%q}`, n.name, n.kind))
	}
	chapterID := createChapter(t, srv, owner, projectID, "第一章", "梗概")
	setBody(t, srv, owner, chapterID, threeSceneBody)
	base := srv.URL + "/api/chapters/" + chapterID + "/scenes"

	if v := decodeScenes(t, mustGet(t, owner, base)); len(v.Scenes) != 0 || v.Stale {
		t.Fatalf("before split: %+v", v)
	}
	if got := unitKind(t, srv, owner, projectID); got != "paragraph" {
		t.Fatalf("unit kind before split = %q", got)
	}

	resp := postJSON(t, owner, base+"/split", `{}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("split: %d", resp.StatusCode)
	}
	v := decodeScenes(t, resp)
	if len(v.Scenes) != 3 {
		t.Fatalf("scenes = %+v", v.Scenes)
	}
	body := []rune(threeSceneBody)
	for i, want := range []struct{ person, place, opening, cueText string }{
		{"林远", "山谷", "林远在山谷", ""}, {"苏晚", "藏经阁", "次日，苏晚", "次日"}, {"魔尊", "魔宫", "与此同时，魔宫", "与此同时"},
	} {
		s := v.Scenes[i]
		if s.Index != i || len(s.Characters) != 1 || s.Characters[0] != ids[want.person] || s.Location != ids[want.place] {
			t.Errorf("scene %d: %+v", i, s)
		}
		if !strings.HasPrefix(string(body[s.Start:s.End]), want.opening) || s.CueText != want.cueText || s.Origin != "auto" {
			t.Errorf("scene %d opens %q cue %q", i, string(body[s.Start:s.Start+5]), s.CueText)
		}
	}
	if got := unitKind(t, srv, owner, projectID); got != "scene" {
		t.Fatalf("unit kind after split = %q", got)
	}

	// Editing a scene's title marks it edited.
	resp = patchJSON(t, owner, srv.URL+"/api/scenes/"+v.Scenes[1].ID, `{"title":"  藏经阁夜读  "}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("patch scene: %d", resp.StatusCode)
	}
	edited := decodeJSON(t, resp)
	resp.Body.Close()
	if edited["title"] != "藏经阁夜读" || edited["origin"] != "edited" || edited["summary"] != v.Scenes[1].Summary {
		t.Fatalf("edited = %v", edited)
	}
	resp = patchJSON(t, owner, srv.URL+"/api/scenes/"+v.Scenes[1].ID, `{"title":"   "}`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("blank title: %d", resp.StatusCode)
	}

	// Changing the body makes the scenes stale, and co-occurrence falls
	// back to paragraphs rather than trust the old offsets.
	setBody(t, srv, owner, chapterID, "开头加了一句。\n"+threeSceneBody)
	if v := decodeScenes(t, mustGet(t, owner, base)); !v.Stale || len(v.Scenes) != 3 {
		t.Fatalf("after edit: stale=%v scenes=%d", v.Stale, len(v.Scenes))
	}
	if got := unitKind(t, srv, owner, projectID); got != "paragraph" {
		t.Fatalf("unit kind with stale scenes = %q", got)
	}

	// Deleting the chapter takes its scenes with it.
	if code := mustDelete(t, owner, srv.URL+"/api/chapters/"+chapterID); code != http.StatusOK && code != http.StatusNoContent {
		t.Fatalf("delete chapter: %d", code)
	}
	var left int
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM chapter_scenes WHERE chapter_id = ?`, chapterID).Scan(&left); err != nil || left != 0 {
		t.Fatalf("scenes left after delete: %d (%v)", left, err)
	}
}

func TestScenesRejectionsAndSplitAll(t *testing.T) {
	srv, _ := newStudioServer(t)
	owner := registerUser(t, srv, "scenes-all@example.com")
	intruder := registerUser(t, srv, "scenes-intruder@example.com")
	projectID := createProject(t, srv, owner, "全书切分")

	empty := createChapter(t, srv, owner, projectID, "未写", "梗概")
	resp := postJSON(t, owner, srv.URL+"/api/chapters/"+empty+"/scenes/split", `{}`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("split empty chapter: %d", resp.StatusCode)
	}

	a := createChapter(t, srv, owner, projectID, "甲", "梗概")
	setBody(t, srv, owner, a, threeSceneBody)
	b := createChapter(t, srv, owner, projectID, "乙", "梗概")
	setBody(t, srv, owner, b, "林远出门了。")

	resp = postJSON(t, owner, srv.URL+"/api/projects/"+projectID+"/scenes/split-all", `{}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("split-all: %d", resp.StatusCode)
	}
	got := decodeJSON(t, resp)
	resp.Body.Close()
	if intFromJSON(got["chapters"]) != 2 || intFromJSON(got["scenes"]) != 4 {
		t.Fatalf("split-all = %v", got)
	}
	sceneID := decodeScenes(t, mustGet(t, owner, srv.URL+"/api/chapters/"+b+"/scenes")).Scenes[0].ID

	for name, code := range map[string]int{
		"GET scenes": func() int {
			r := mustGet(t, intruder, srv.URL+"/api/chapters/"+a+"/scenes")
			r.Body.Close()
			return r.StatusCode
		}(),
		"split": func() int {
			r := postJSON(t, intruder, srv.URL+"/api/chapters/"+a+"/scenes/split", `{}`)
			r.Body.Close()
			return r.StatusCode
		}(),
		"split-all": func() int {
			r := postJSON(t, intruder, srv.URL+"/api/projects/"+projectID+"/scenes/split-all", `{}`)
			r.Body.Close()
			return r.StatusCode
		}(),
		"PATCH scene": func() int {
			r := patchJSON(t, intruder, srv.URL+"/api/scenes/"+sceneID, `{"title":"篡改"}`)
			r.Body.Close()
			return r.StatusCode
		}(),
	} {
		if code != http.StatusNotFound {
			t.Errorf("%s by another user: %d, want 404", name, code)
		}
	}
}

func TestGraphPlacesWithTheirScenes(t *testing.T) {
	srv, _ := newStudioServer(t)
	owner := registerUser(t, srv, "places@example.com")
	intruder := registerUser(t, srv, "places-intruder@example.com")
	projectID := createProject(t, srv, owner, "地理")

	_, east := saveNode(t, srv, owner, projectID, `{"name":"东洲","kind":"location"}`)
	_, mount := saveNode(t, srv, owner, projectID, `{"name":"青云山","kind":"location"}`)
	_, library := saveNode(t, srv, owner, projectID, `{"name":"藏经阁","kind":"location","faction":"青云山"}`)
	_, palace := saveNode(t, srv, owner, projectID, `{"name":"魔宫","kind":"location"}`)
	saveNode(t, srv, owner, projectID, `{"name":"苏晚","kind":"character"}`)
	saveEdge(t, srv, owner, projectID, fmt.Sprintf(`{"source_id":%q,"target_id":%q,"relation":"位于"}`, mount, east))

	chapterID := createChapter(t, srv, owner, projectID, "第一章", "梗概")
	setBody(t, srv, owner, chapterID, threeSceneBody)
	resp := postJSON(t, owner, srv.URL+"/api/chapters/"+chapterID+"/scenes/split", `{}`)
	resp.Body.Close()

	get := func(c *http.Client) (int, map[string]json.RawMessage) {
		r := mustGet(t, c, srv.URL+"/api/projects/"+projectID+"/graph/places")
		defer r.Body.Close()
		var body map[string]json.RawMessage
		if r.StatusCode == http.StatusOK {
			_ = json.NewDecoder(r.Body).Decode(&body)
		}
		return r.StatusCode, body
	}
	code, body := get(owner)
	if code != http.StatusOK {
		t.Fatalf("status %d", code)
	}
	var roots []lineageNodeView
	_ = json.Unmarshal(body["roots"], &roots)
	if got := lineagePath(roots, "藏经阁"); fmt.Sprint(got) != "[东洲 青云山 藏经阁]" {
		t.Fatalf("path = %v", got)
	}
	if got := lineagePath(roots, "苏晚"); got != nil {
		t.Fatalf("a character in the geography: %v", got)
	}
	var scenes map[string][]struct {
		ChapterID string `json:"chapter_id"`
		Seq       int    `json:"seq"`
		Index     int    `json:"index"`
	}
	_ = json.Unmarshal(body["scenes"], &scenes)
	if s := scenes[library]; len(s) != 1 || s[0].ChapterID != chapterID || s[0].Seq != 1 || s[0].Index != 1 {
		t.Fatalf("藏经阁 scenes = %+v", s)
	}
	if s := scenes[palace]; len(s) != 1 || s[0].Index != 2 {
		t.Fatalf("魔宫 scenes = %+v", s)
	}
	if code, _ := get(intruder); code != http.StatusNotFound {
		t.Fatalf("other user: %d", code)
	}
}
