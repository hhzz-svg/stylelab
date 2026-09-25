package insight

import (
	"math"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

// Scene segmentation after Hearst's TextTiling (1997), adapted to Chinese
// prose without a word segmenter:
//
//  1. Split into paragraphs. A paragraph of nothing but separator marks
//     (*** / ◇◇◇ / ———) is a hard boundary: the author drew it.
//  2. At each gap between paragraphs, compare the vocabulary on either
//     side -- vectors of adjacent Han-character pairs over ~200 runes --
//     by cosine similarity, and score how deep a valley the gap sits in.
//  3. Add weight where the next paragraph opens with a time or place
//     transition (次日, 三天后, 与此同时, 却说 ...) and where the cast
//     changes (Jaccard distance between the characters on each side).
//  4. Take gaps from the highest score down, above the mean plus half a
//     standard deviation, refusing any that would leave a scene shorter
//     than MinRunes.

// DefaultMinSceneRunes is the shortest scene a soft boundary may create.
const DefaultMinSceneRunes = 300

const (
	cohesionWindowRunes = 200
	transitionBonus     = 0.5
	castShiftWeight     = 0.3
	minValleyDepth      = 0.15
	thresholdSigma      = 0.5
	sceneTitleRunes     = 18
	sceneSummaryRunes   = 80
)

// Scene cues: why a scene starts where it does.
const (
	CueNone       = ""
	CueSeparator  = "separator"  // the author's separator line
	CueTransition = "transition" // a time or place transition phrase
	CueShift      = "shift"      // the vocabulary and cast change
)

// SceneOptions tunes a segmentation.
type SceneOptions struct {
	MinRunes   int        // 0 means DefaultMinSceneRunes
	Characters []CoEntity // for the cast signal and each scene's cast
	Locations  []CoEntity // for each scene's main location
}

// Scene is a stretch of a chapter, in rune offsets [Start, End) of the body.
type Scene struct {
	Index      int      `json:"index"`
	Start      int      `json:"start"`
	End        int      `json:"end"`
	Runes      int      `json:"runes"`
	Cue        string   `json:"cue"`
	CueText    string   `json:"cue_text"`
	Title      string   `json:"title"`
	Summary    string   `json:"summary"`
	Characters []string `json:"characters"` // ids, most mentioned first
	Location   string   `json:"location"`   // id of the most mentioned location, or ""
}

var transitionCue = regexp.MustCompile(`^(?:次日|翌日|第二天|第二日|隔日|当夜|当晚|入夜|深夜|半夜|夜里|夜深|清晨|拂晓|黎明|破晓|天明|天亮|黄昏|傍晚|日暮|午后|正午|与此同时|同一时间|同一时刻|另一边|另一头|另一方面|却说|且说|话分两头|转眼|一晃|不知过了多久|过了许久|[一二三四五六七八九十百千两几数半\d]+个?(?:天|日|夜|月|年|旬|时辰|炷香|盏茶)(?:之后|以后|后|过去))`)

var (
	separatorMarks = "*＊◆◇☆★#＃※○●◎§"
	separatorLines = "-—－=~～_·•━─"
)

type para struct {
	start, end int // rune offsets of the trimmed text
	text       string
	runes      int
	grams      map[string]int
	cast       map[int]int // character entity -> mentions
	places     map[int]int
	breakAfter bool // a separator line follows before the next paragraph
}

func isSeparator(rs []rune) bool {
	marks, lines := 0, 0
	for _, r := range rs {
		switch {
		case unicode.IsSpace(r):
		case strings.ContainsRune(separatorMarks, r):
			marks++
		case strings.ContainsRune(separatorLines, r):
			lines++
		default:
			return false
		}
	}
	return marks > 0 || lines >= 3
}

// splitParas returns the content paragraphs, noting separator lines as
// breaks after the paragraph before them.
func splitParas(body string) []para {
	rs := []rune(body)
	var out []para
	lineStart := 0
	for i := 0; i <= len(rs); i++ {
		if i < len(rs) && rs[i] != '\n' {
			continue
		}
		s, e := lineStart, i
		lineStart = i + 1
		for s < e && unicode.IsSpace(rs[s]) {
			s++
		}
		for e > s && unicode.IsSpace(rs[e-1]) {
			e--
		}
		if e == s {
			continue
		}
		if isSeparator(rs[s:e]) {
			if len(out) > 0 {
				out[len(out)-1].breakAfter = true
			}
			continue
		}
		out = append(out, para{start: s, end: e, text: string(rs[s:e]), runes: e - s})
	}
	return out
}

func bigrams(text string) map[string]int {
	g := map[string]int{}
	var prev rune
	for _, r := range text {
		if unicode.Is(unicode.Han, r) {
			if prev != 0 {
				g[string([]rune{prev, r})]++
			}
			prev = r
		} else {
			prev = 0
		}
	}
	return g
}

func cosine(a, b map[string]int) float64 {
	var dot, na, nb float64
	for k, v := range a {
		na += float64(v * v)
		if w, ok := b[k]; ok {
			dot += float64(v * w)
		}
	}
	for _, v := range b {
		nb += float64(v * v)
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / math.Sqrt(na*nb)
}

func countMatches(m *Matcher, text string) map[int]int {
	out := map[int]int{}
	for _, mt := range m.FindAll(text) {
		out[mt.Entity]++
	}
	return out
}

// window merges paragraphs outward from i (step -1 or +1) until it holds
// cohesionWindowRunes.
func window(ps []para, i, step int) (map[string]int, map[int]bool) {
	grams := map[string]int{}
	cast := map[int]bool{}
	runes := 0
	for ; i >= 0 && i < len(ps) && runes < cohesionWindowRunes; i += step {
		for k, v := range ps[i].grams {
			grams[k] += v
		}
		for e := range ps[i].cast {
			cast[e] = true
		}
		runes += ps[i].runes
	}
	return grams, cast
}

func jaccardDistance(a, b map[int]bool) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0 // one side is pure description: no evidence either way
	}
	inter := 0
	for e := range a {
		if b[e] {
			inter++
		}
	}
	return 1 - float64(inter)/float64(len(a)+len(b)-inter)
}

type gap struct {
	index   int // boundary between paragraph index and index+1
	hard    bool
	cue     string
	cueText string
	depth   float64
	score   float64
}

// SegmentScenes splits a chapter body into scenes. An empty body has none;
// any other body has at least one.
func SegmentScenes(body string, opt SceneOptions) []Scene {
	minRunes := opt.MinRunes
	if minRunes <= 0 {
		minRunes = DefaultMinSceneRunes
	}
	ps := splitParas(body)
	if len(ps) == 0 {
		return []Scene{}
	}
	chars := newEntityMatcher(opt.Characters)
	places := newEntityMatcher(opt.Locations)
	for i := range ps {
		ps[i].grams = bigrams(ps[i].text)
		ps[i].cast = countMatches(chars, ps[i].text)
		ps[i].places = countMatches(places, ps[i].text)
	}

	gaps := make([]gap, len(ps)-1)
	cohesion := make([]float64, len(gaps))
	shift := make([]float64, len(gaps))
	for g := range gaps {
		lg, lc := window(ps, g, -1)
		rg, rc := window(ps, g+1, 1)
		cohesion[g] = cosine(lg, rg)
		shift[g] = jaccardDistance(lc, rc)
		gaps[g] = gap{index: g, hard: ps[g].breakAfter}
		if m := transitionCue.FindString(ps[g+1].text); m != "" {
			gaps[g].cue, gaps[g].cueText = CueTransition, m
		}
	}
	for g := range gaps {
		lp, rp := cohesion[g], cohesion[g]
		for i := g - 1; i >= 0 && cohesion[i] >= lp; i-- {
			lp = cohesion[i]
		}
		for i := g + 1; i < len(cohesion) && cohesion[i] >= rp; i++ {
			rp = cohesion[i]
		}
		gaps[g].depth = lp + rp - 2*cohesion[g]
		gaps[g].score = gaps[g].depth + castShiftWeight*shift[g]
		if gaps[g].cue != "" {
			gaps[g].score += transitionBonus
		}
	}

	// The author's separators always cut.
	cut := make([]bool, len(gaps))
	var soft []gap
	for _, gp := range gaps {
		if gp.hard {
			cut[gp.index] = true
		} else {
			soft = append(soft, gp)
		}
	}

	if len(soft) > 0 {
		mean, sd := meanStd(soft)
		threshold := mean + thresholdSigma*sd
		var eligible []gap
		for _, gp := range soft {
			if gp.cue != "" || (gp.depth >= minValleyDepth && gp.score >= threshold) {
				eligible = append(eligible, gp)
			}
		}
		sort.SliceStable(eligible, func(a, b int) bool {
			if eligible[a].score != eligible[b].score {
				return eligible[a].score > eligible[b].score
			}
			return eligible[a].index < eligible[b].index
		})
		for _, gp := range eligible {
			if segmentRunes(ps, cut, gp.index, -1) >= minRunes && segmentRunes(ps, cut, gp.index, 1) >= minRunes {
				cut[gp.index] = true
			}
		}
	}

	return buildScenes(ps, gaps, cut, opt)
}

func newEntityMatcher(es []CoEntity) *Matcher {
	var ps []Pattern
	for i, e := range es {
		for _, n := range e.Names {
			ps = append(ps, Pattern{Text: n, Entity: i})
		}
	}
	return NewMatcher(ps)
}

func meanStd(gs []gap) (float64, float64) {
	var sum float64
	for _, g := range gs {
		sum += g.score
	}
	mean := sum / float64(len(gs))
	var sq float64
	for _, g := range gs {
		sq += (g.score - mean) * (g.score - mean)
	}
	return mean, math.Sqrt(sq / float64(len(gs)))
}

// segmentRunes measures the scene a cut at gap g would leave on one side
// (dir -1: paragraphs up to g; +1: from g+1), up to the nearest existing cut.
func segmentRunes(ps []para, cut []bool, g, dir int) int {
	n := 0
	if dir < 0 {
		for i := g; i >= 0; i-- {
			n += ps[i].runes
			if i > 0 && cut[i-1] {
				break
			}
		}
		return n
	}
	for i := g + 1; i < len(ps); i++ {
		n += ps[i].runes
		if i < len(cut) && cut[i] {
			break
		}
	}
	return n
}

func buildScenes(ps []para, gaps []gap, cut []bool, opt SceneOptions) []Scene {
	var out []Scene
	first := 0
	for i := range ps {
		if i < len(cut) && !cut[i] {
			continue
		}
		s := Scene{Index: len(out), Start: ps[first].start, End: ps[i].end, Characters: []string{}}
		if first > 0 {
			g := gaps[first-1]
			switch {
			case g.hard:
				s.Cue = CueSeparator
			case g.cue != "":
				s.Cue, s.CueText = g.cue, g.cueText
			default:
				s.Cue = CueShift
			}
		}
		cast := map[int]int{}
		place := map[int]int{}
		for k := first; k <= i; k++ {
			s.Runes += ps[k].runes
			for e, c := range ps[k].cast {
				cast[e] += c
			}
			for e, c := range ps[k].places {
				place[e] += c
			}
		}
		s.Title = sceneTitle(ps[first].text)
		s.Summary = sceneSummary(ps[first : i+1])
		s.Characters = rankedIDs(cast, opt.Characters)
		if ids := rankedIDs(place, opt.Locations); len(ids) > 0 {
			s.Location = ids[0]
		}
		out = append(out, s)
		first = i + 1
	}
	return out
}

func rankedIDs(counts map[int]int, es []CoEntity) []string {
	idx := make([]int, 0, len(counts))
	for e := range counts {
		idx = append(idx, e)
	}
	sort.Slice(idx, func(a, b int) bool {
		if counts[idx[a]] != counts[idx[b]] {
			return counts[idx[a]] > counts[idx[b]]
		}
		return es[idx[a]].ID < es[idx[b]].ID
	})
	out := make([]string, len(idx))
	for i, e := range idx {
		out[i] = es[e].ID
	}
	return out
}

// sceneTitle is the opening sentence, shortened.
func sceneTitle(text string) string {
	rs := []rune(text)
	for i, r := range rs {
		if strings.ContainsRune("。！？!?…", r) {
			rs = rs[:i]
			break
		}
	}
	if len(rs) > sceneTitleRunes {
		return string(rs[:sceneTitleRunes]) + "…"
	}
	return string(rs)
}

func sceneSummary(ps []para) string {
	var b []rune
	for _, p := range ps {
		if len(b) > 0 {
			b = append(b, ' ')
		}
		b = append(b, []rune(p.text)...)
		if len(b) >= sceneSummaryRunes {
			break
		}
	}
	if len(b) > sceneSummaryRunes {
		return string(b[:sceneSummaryRunes]) + "…"
	}
	return string(b)
}
