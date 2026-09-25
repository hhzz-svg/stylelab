package insight

import (
	"reflect"
	"strings"
	"testing"
)

func TestSplitNameAndCandidates(t *testing.T) {
	for name, want := range map[string][2]string{
		"林远": {"林", "远"}, "林远山": {"林", "远山"}, "欧阳锋": {"欧阳", "锋"}, "欧阳": {"欧", "阳"},
	} {
		s, g, ok := splitName(name)
		if !ok || s != want[0] || g != want[1] {
			t.Errorf("splitName(%q) = %q %q %v", name, s, g, ok)
		}
	}
	for _, bad := range []string{"林", "一个很长的名字"} {
		if _, _, ok := splitName(bad); ok {
			t.Errorf("splitName(%q) should fail", bad)
		}
	}
	c := aliasCandidates("林远山")
	for _, want := range []string{"林师兄", "林姑娘", "山儿", "小山", "阿山", "远山"} {
		found := false
		for _, x := range c {
			found = found || x == want
		}
		if !found {
			t.Errorf("candidates of 林远山 lack %q", want)
		}
	}
	for _, x := range aliasCandidates("林远") {
		if x == "远" {
			t.Error("a one-character given name alone is too loose to propose")
		}
	}
}

func byAlias(ss []AliasSuggestion) map[string]AliasSuggestion {
	m := map[string]AliasSuggestion{}
	for _, s := range ss {
		m[s.Alias] = s
	}
	return m
}

func TestDiscoverAliases(t *testing.T) {
	characters := []CoEntity{
		{ID: "lin", Names: []string{"林远"}},
		{ID: "wan", Names: []string{"林婉"}},
		{ID: "su", Names: []string{"苏晚", "晚儿"}}, // already has her pet name
		{ID: "ouyang", Names: []string{"欧阳锋"}},
	}
	chapters := []CoChapter{
		{Seq: 1, Units: []string{
			"林远在山门外练剑。",
			"林师兄的剑越来越快，远儿这孩子天分极高。", // 林远 named either side
			"夜深了，林远收剑。",
			"林婉提着灯笼走来。",
			"林师妹，天凉。",         // 林婉 named either side
			"林师兄笑道：林婉，你怎么来了。", // only 林婉 nearby
		}},
		{Seq: 2, Units: []string{
			"林远独自下山，林师兄一路无话。",
			"远儿，路上小心。晚儿在身后喊。",
			"林婉追出山门，林师妹三个字卡在喉头。",
			"欧阳兄别来无恙？欧阳锋冷笑。",
			"小远只出现这一次。",
			"林远与林婉并肩，林姑娘低声道。",
			"林姑娘又道：林远，林婉，走吧。",
		}},
		{Seq: 3, Units: []string{"欧阳兄，请。"}},
	}
	got := byAlias(DiscoverAliases(characters, chapters))

	check := func(alias, entity string, count int, ambiguous bool) {
		t.Helper()
		s, ok := got[alias]
		if !ok {
			t.Errorf("%s not suggested", alias)
			return
		}
		if s.EntityID != entity || s.Count != count || s.Ambiguous != ambiguous {
			t.Errorf("%s = %+v, want %s ×%d ambiguous=%v", alias, s, entity, count, ambiguous)
		}
	}
	// 林师兄 could be 林远 or 林婉 (both surnamed 林). Two mentions have
	// 林远 named in the same or a neighbouring paragraph, one has 林婉:
	// 林远, with 2 of 3 pieces of evidence.
	check("林师兄", "lin", 3, false)
	if s := got["林师兄"]; s.Confidence != 2.0/3 {
		t.Errorf("林师兄 confidence = %v, want 2/3", s.Confidence)
	}
	check("林师妹", "wan", 2, false)
	check("远儿", "lin", 2, false) // only 林远 generates it
	check("欧阳兄", "ouyang", 2, false)
	// 林姑娘 appears only where both are named: a coin toss.
	check("林姑娘", "lin", 2, true)

	if _, ok := got["晚儿"]; ok {
		t.Error("an existing alias must not be suggested again")
	}
	if _, ok := got["小远"]; ok {
		t.Error("a single mention is below the bar")
	}
	if s := got["远儿"]; len(s.Examples) != 2 || !strings.Contains(s.Examples[0], "远儿这孩子") {
		t.Errorf("examples = %q", s.Examples)
	}
	// Confident suggestions first, then by count.
	ordered := DiscoverAliases(characters, chapters)
	if ordered[len(ordered)-1].Alias != "林姑娘" || ordered[0].Alias != "林师兄" {
		t.Errorf("order: first %s, last %s", ordered[0].Alias, ordered[len(ordered)-1].Alias)
	}
}

func TestDiscoverAliasesSkipsCandidatesInsideLongerNames(t *testing.T) {
	// 远山 is a candidate for 林远山, but inside "林远山" it is the full name.
	characters := []CoEntity{{ID: "a", Names: []string{"林远山"}}}
	chapters := []CoChapter{{Units: []string{"林远山来了。", "林远山走了。", "远山笑了。"}}}
	if got := DiscoverAliases(characters, chapters); len(got) != 0 {
		t.Fatalf("got %+v", got)
	}
	chapters[0].Units = append(chapters[0].Units, "远山又笑了。")
	got := DiscoverAliases(characters, chapters)
	if !reflect.DeepEqual([]string{got[0].Alias}, []string{"远山"}) || got[0].Count != 2 {
		t.Fatalf("got %+v", got)
	}
}

func TestSnippet(t *testing.T) {
	long := strings.Repeat("甲", 30) + "林师兄" + strings.Repeat("乙", 30)
	s := snippet(long, 30, 3)
	if !strings.HasPrefix(s, "…") || !strings.HasSuffix(s, "…") || len([]rune(s)) != 2+16*2+3 {
		t.Fatalf("snippet = %q", s)
	}
	if got := snippet("林师兄", 0, 3); got != "林师兄" {
		t.Fatalf("short = %q", got)
	}
}
