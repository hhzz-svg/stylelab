package insight

import (
	"reflect"
	"strings"
	"testing"
)

// Three stretches of prose about different things, in different places,
// with different people.
var (
	valley = []string{
		"林远在山谷中练剑，剑光如水，剑气纵横，惊起一片飞鸟。",
		"他收剑而立，山风拂过山谷，谷中松涛阵阵，剑穗轻摇。",
		"林远再次拔剑，剑锋所指，山石崩裂，碎石纷飞，剑气久久不散。",
		"剑意越来越凝练，林远的呼吸也越来越沉稳，剑与人仿佛合为一体。",
		"山谷深处的瀑布轰鸣，水雾打湿了他的衣衫，他却浑然不觉，只管练剑。",
		"一套剑法使完，林远收剑入鞘，望着山谷里被剑气削平的巨石出神。",
	}
	library = []string{
		"苏晚在藏经阁里翻阅古籍，书页泛黄，墨香扑鼻，烛火映着她的侧脸。",
		"她一卷卷地查找，指尖拂过书脊，古籍上的字迹早已模糊不清。",
		"藏经阁的书架高耸入顶，苏晚踩着木梯，从最上层抽出一本残破的典籍。",
		"典籍里记载着失传的阵法，苏晚看得入神，连烛泪流尽都没有察觉。",
		"她把要紧的段落抄在纸上，又将古籍小心放回原处，拂去书上的灰尘。",
		"夜读至此，藏经阁外传来更鼓声，苏晚合上书卷，揉了揉酸涩的眼睛。",
	}
	palace = []string{
		"魔宫深处，魔尊端坐王座，血色灯火摇曳，照得殿中一片猩红。",
		"殿下跪着数名魔将，无人敢抬头，魔宫里只听得见灯芯爆裂的声音。",
		"魔尊缓缓开口，声音冰冷，命魔将们三日之内踏平正道的山门。",
		"魔将们领命退下，魔宫的大门轰然关闭，血色的雾气翻涌不息。",
		"魔尊独自坐在王座上，指尖敲着扶手，眼中闪过一丝阴鸷的杀意。",
		"王座之后的血池翻滚冒泡，魔尊的影子在猩红灯火中拉得很长。",
	}
)

func entities() SceneOptions {
	return SceneOptions{
		MinRunes: 150,
		Characters: []CoEntity{
			{ID: "lin", Names: []string{"林远"}},
			{ID: "su", Names: []string{"苏晚"}},
			{ID: "mo", Names: []string{"魔尊"}},
		},
		Locations: []CoEntity{
			{ID: "valley", Names: []string{"山谷"}},
			{ID: "library", Names: []string{"藏经阁"}},
			{ID: "palace", Names: []string{"魔宫"}},
		},
	}
}

func join(parts ...[]string) string {
	var all []string
	for _, p := range parts {
		all = append(all, p...)
	}
	return strings.Join(all, "\n")
}

func prefixed(prefix string, ps []string) []string {
	out := append([]string(nil), ps...)
	out[0] = prefix + out[0]
	return out
}

// text returns the scene's slice of the body.
func text(body string, s Scene) string { return string([]rune(body)[s.Start:s.End]) }

func TestScenesSplitOnTransitions(t *testing.T) {
	body := join(valley, prefixed("次日，", library), prefixed("与此同时，", palace))
	scenes := SegmentScenes(body, entities())
	if len(scenes) != 3 {
		t.Fatalf("got %d scenes: %+v", len(scenes), scenes)
	}
	if scenes[1].Cue != CueTransition || scenes[1].CueText != "次日" || scenes[2].CueText != "与此同时" {
		t.Fatalf("cues = %q %q / %q", scenes[1].Cue, scenes[1].CueText, scenes[2].CueText)
	}
	if !strings.HasPrefix(text(body, scenes[1]), "次日，苏晚在藏经阁") || !strings.HasSuffix(text(body, scenes[0]), "出神。") {
		t.Fatalf("boundary misplaced: %q", text(body, scenes[1])[:12])
	}
	want := []struct {
		cast  []string
		place string
	}{{[]string{"lin"}, "valley"}, {[]string{"su"}, "library"}, {[]string{"mo"}, "palace"}}
	for i, w := range want {
		if !reflect.DeepEqual(scenes[i].Characters, w.cast) || scenes[i].Location != w.place {
			t.Errorf("scene %d: cast %v place %q", i, scenes[i].Characters, scenes[i].Location)
		}
	}
	if scenes[0].Title != "林远在山谷中练剑，剑光如水，剑气纵横…" {
		t.Errorf("title = %q", scenes[0].Title)
	}
}

// Without any cue words the change of vocabulary and cast alone finds the
// same boundaries.
func TestScenesSplitOnTopicShift(t *testing.T) {
	body := join(valley, library, palace)
	scenes := SegmentScenes(body, entities())
	if len(scenes) != 3 {
		t.Fatalf("got %d scenes", len(scenes))
	}
	for i, prefix := range []string{"林远在山谷", "苏晚在藏经阁", "魔宫深处"} {
		if !strings.HasPrefix(text(body, scenes[i]), prefix) {
			t.Errorf("scene %d starts %q", i, []rune(text(body, scenes[i]))[:6])
		}
		if i > 0 && scenes[i].Cue != CueShift {
			t.Errorf("scene %d cue = %q", i, scenes[i].Cue)
		}
	}
}

func TestScenesUniformTextStaysWhole(t *testing.T) {
	body := join(valley, valley, valley)
	if got := SegmentScenes(body, entities()); len(got) != 1 {
		t.Fatalf("one continuous scene split into %d", len(got))
	}
}

func TestScenesSeparatorsAlwaysCut(t *testing.T) {
	// Two short stretches, each well under MinRunes, split by the author.
	body := valley[0] + "\n\n　　＊　＊　＊\n\n" + library[0] + "\n———\n" + palace[0]
	scenes := SegmentScenes(body, entities())
	if len(scenes) != 3 || scenes[1].Cue != CueSeparator || scenes[2].Cue != CueSeparator {
		t.Fatalf("got %+v", scenes)
	}
	for _, s := range scenes {
		if strings.ContainsAny(text(body, s), "＊—") {
			t.Fatalf("separator inside a scene: %q", text(body, s))
		}
	}
	// Separators at the very start or end do not create empty scenes, and a
	// dash inside prose is not a separator.
	if got := SegmentScenes("***\n"+valley[0]+"——他想。\n***", entities()); len(got) != 1 {
		t.Fatalf("edge separators: %+v", got)
	}
}

func TestScenesRespectMinimumLength(t *testing.T) {
	// A transition cue near the end would leave a one-line scene.
	body := join(valley, library, []string{"次日，林远醒来。"})
	scenes := SegmentScenes(body, entities())
	last := scenes[len(scenes)-1]
	if strings.HasPrefix(text(body, last), "次日") {
		t.Fatalf("a %d-rune scene was cut off", last.Runes)
	}
	for _, s := range scenes {
		if s.Runes < 150 {
			t.Fatalf("scene of %d runes", s.Runes)
		}
	}
}

func TestScenesEmptyAndSingle(t *testing.T) {
	if got := SegmentScenes(" \n\n ", SceneOptions{}); got == nil || len(got) != 0 {
		t.Fatalf("empty: %v", got)
	}
	got := SegmentScenes("  林远出门了。  ", SceneOptions{})
	if len(got) != 1 || got[0].Start != 2 || got[0].End != 8 || got[0].Runes != 6 || got[0].Cue != CueNone {
		t.Fatalf("single: %+v", got)
	}
}

func TestTransitionCues(t *testing.T) {
	for _, s := range []string{"次日清晨", "三天后", "半月之后", "数日以后", "两个时辰后", "与此同时", "却说", "10年过去"} {
		if !transitionCue.MatchString(s) {
			t.Errorf("%q should be a transition", s)
		}
	}
	for _, s := range []string{"他三天没睡", "日后再说", "天色渐暗", "说罢"} {
		if transitionCue.MatchString(s) {
			t.Errorf("%q should not be a transition", s)
		}
	}
}
