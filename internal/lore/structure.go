package lore

import (
	"context"

	"stylelab/internal/store"
)

// SceneBrief is a scene as the structure tree shows it.
type SceneBrief struct {
	ID      string `json:"id"`
	Index   int    `json:"index"`
	Title   string `json:"title"`
	Cue     string `json:"cue"`
	CueText string `json:"cue_text"`
	Runes   int    `json:"runes"`
}

// ChapterNode is a chapter in the structure tree.
type ChapterNode struct {
	ID          string       `json:"id"`
	Seq         int          `json:"seq"`
	Title       string       `json:"title"`
	Status      string       `json:"status"`
	Runes       int          `json:"runes"`
	Scenes      []SceneBrief `json:"scenes"`
	ScenesStale bool         `json:"scenes_stale"`
}

// VolumeNode is a volume and its chapters. The leading group of chapters
// before the first volume has no ID ("未分卷").
type VolumeNode struct {
	Volume
	Chapters []ChapterNode `json:"chapters"`
	Runes    int           `json:"runes"`
	Written  int           `json:"written"` // chapters with prose
}

// Structure is the book as a tree: volume -> chapter -> scene.
type Structure struct {
	Volumes []VolumeNode `json:"volumes"`
}

// groupByVolume puts each chapter (in seq order) under the last volume that
// starts at or before it. Chapters before every volume form a leading group
// without an id; it is omitted when empty. Every volume is kept, even one
// whose chapters were deleted.
func groupByVolume(volumes []Volume, chapters []ChapterNode) []VolumeNode {
	out := []VolumeNode{}
	lead := VolumeNode{Chapters: []ChapterNode{}}
	nodes := make([]VolumeNode, len(volumes))
	for i, v := range volumes {
		nodes[i] = VolumeNode{Volume: v, Chapters: []ChapterNode{}}
	}
	vi := -1
	for _, c := range chapters {
		for vi+1 < len(volumes) && volumes[vi+1].StartSeq <= c.Seq {
			vi++
		}
		target := &lead
		if vi >= 0 {
			target = &nodes[vi]
		}
		target.Chapters = append(target.Chapters, c)
		target.Runes += c.Runes
		if c.Runes > 0 {
			target.Written++
		}
	}
	if len(lead.Chapters) > 0 {
		out = append(out, lead)
	}
	return append(out, nodes...)
}

// LoadStructure builds the project's tree.
func LoadStructure(ctx context.Context, st *store.Store, projectID string) (Structure, error) {
	volumes, err := ListVolumes(ctx, st, projectID)
	if err != nil {
		return Structure{}, err
	}
	rows, err := st.DB().QueryContext(ctx,
		`SELECT id, seq, title, status, body FROM chapters WHERE project_id = ? ORDER BY seq`, projectID)
	if err != nil {
		return Structure{}, err
	}
	defer rows.Close()
	var chapters []ChapterNode
	hashes := map[string]string{}
	for rows.Next() {
		var c ChapterNode
		var body string
		if err := rows.Scan(&c.ID, &c.Seq, &c.Title, &c.Status, &body); err != nil {
			return Structure{}, err
		}
		c.Runes = len([]rune(body))
		c.Scenes = []SceneBrief{}
		hashes[c.ID] = BodyHash(body)
		chapters = append(chapters, c)
	}
	if err := rows.Err(); err != nil {
		return Structure{}, err
	}

	srows, err := st.DB().QueryContext(ctx,
		`SELECT chapter_id, id, idx, title, cue, cue_text, runes, body_hash FROM chapter_scenes WHERE project_id = ? ORDER BY chapter_id, idx`,
		projectID)
	if err != nil {
		return Structure{}, err
	}
	defer srows.Close()
	scenes := map[string][]SceneBrief{}
	stale := map[string]bool{}
	for srows.Next() {
		var chapterID, hash string
		var s SceneBrief
		if err := srows.Scan(&chapterID, &s.ID, &s.Index, &s.Title, &s.Cue, &s.CueText, &s.Runes, &hash); err != nil {
			return Structure{}, err
		}
		scenes[chapterID] = append(scenes[chapterID], s)
		if hash != hashes[chapterID] {
			stale[chapterID] = true
		}
	}
	if err := srows.Err(); err != nil {
		return Structure{}, err
	}
	for i := range chapters {
		if s, ok := scenes[chapters[i].ID]; ok {
			chapters[i].Scenes = s
			chapters[i].ScenesStale = stale[chapters[i].ID]
		}
	}
	return Structure{Volumes: groupByVolume(volumes, chapters)}, nil
}
