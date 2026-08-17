package extract

import (
	"regexp"
	"strings"
)

var (
	chapterHeadingRe = regexp.MustCompile(`(?m)^#{0,2}\s*第[零〇一二三四五六七八九十百千万0-9]+章`)
	blankSplitRe     = regexp.MustCompile(`\n\n\n+`)
)

func SplitChapters(text string) []string {
	locs := chapterHeadingRe.FindAllStringIndex(text, -1)
	var parts []string
	if len(locs) >= 2 {
		starts := make([]int, 0, len(locs)+1)
		if locs[0][0] > 0 {
			starts = append(starts, 0)
		}
		for _, loc := range locs {
			starts = append(starts, loc[0])
		}
		for i, start := range starts {
			end := len(text)
			if i+1 < len(starts) {
				end = starts[i+1]
			}
			parts = append(parts, text[start:end])
		}
	} else {
		parts = blankSplitRe.Split(text, -1)
	}

	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			out = append(out, p)
		}
	}
	if len(out) <= 1 {
		return []string{text}
	}
	return out
}
