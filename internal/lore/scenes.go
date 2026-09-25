package lore

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"stylelab/internal/ids"
	"stylelab/internal/insight"
	"stylelab/internal/store"
)

// Scene origins.
const (
	OriginAuto   = "auto"   // as the segmenter produced it
	OriginEdited = "edited" // the author changed its title or summary
)

const (
	maxSceneTitleRunes   = 60
	maxSceneSummaryRunes = 500
)

// ErrNoBody is returned when splitting a chapter that has no prose yet.
var ErrNoBody = errors.New("invalid: 这一章还没有正文，无法切分场景")

// ErrSceneNotFound is returned for a scene that is missing or not the
// caller's.
var ErrSceneNotFound = errors.New("scene not found")

// SceneRecord is a stored scene.
type SceneRecord struct {
	ID         string   `json:"id"`
	ChapterID  string   `json:"chapter_id"`
	Index      int      `json:"index"`
	Start      int      `json:"start"` // rune offsets into the chapter body
	End        int      `json:"end"`
	Runes      int      `json:"runes"`
	Title      string   `json:"title"`
	Summary    string   `json:"summary"`
	Location   string   `json:"location"`   // graph node id, or ""
	Characters []string `json:"characters"` // graph node ids, most mentioned first
	Cue        string   `json:"cue"`
	CueText    string   `json:"cue_text"`
	Origin     string   `json:"origin"`
	UpdatedAt  string   `json:"updated_at"`
}

// ChapterScenes is a chapter's scenes and whether they still fit its body.
type ChapterScenes struct {
	Scenes []SceneRecord `json:"scenes"`
	// Stale: the body changed after the split, so offsets may be off.
	Stale bool `json:"stale"`
}

// BodyHash fingerprints a chapter body.
func BodyHash(body string) string {
	sum := sha256.Sum256([]byte(body))
	return hex.EncodeToString(sum[:8])
}

func (w World) entitiesOfKind(kind string) []insight.CoEntity {
	var out []insight.CoEntity
	for _, n := range w.Nodes {
		if n.Kind == kind {
			out = append(out, insight.CoEntity{ID: n.ID, Names: append([]string{n.Name}, Aliases(n.Details)...)})
		}
	}
	return out
}

// SplitChapter segments a chapter's body and replaces its stored scenes.
// Edited titles and summaries are replaced too: the offsets they belonged
// to no longer exist.
func SplitChapter(ctx context.Context, st *store.Store, projectID, chapterID, body string) ([]SceneRecord, error) {
	if strings.TrimSpace(body) == "" {
		return nil, ErrNoBody
	}
	w, err := LoadWorld(ctx, st, projectID)
	if err != nil {
		return nil, err
	}
	scenes := insight.SegmentScenes(body, insight.SceneOptions{
		Characters: w.entitiesOfKind("character"),
		Locations:  w.entitiesOfKind("location"),
	})

	tx, err := st.DB().BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err := replaceScenes(ctx, tx, projectID, chapterID, body, scenes); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	out, err := LoadScenes(ctx, st, chapterID, body)
	return out.Scenes, err
}

func replaceScenes(ctx context.Context, tx *sql.Tx, projectID, chapterID, body string, scenes []insight.Scene) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM chapter_scenes WHERE chapter_id = ?`, chapterID); err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	hash := BodyHash(body)
	for _, s := range scenes {
		cast, _ := json.Marshal(s.Characters)
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO chapter_scenes (id, project_id, chapter_id, idx, start_rune, end_rune, runes, title, summary,
			   location_id, characters_json, cue, cue_text, origin, body_hash, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			ids.New("scene"), projectID, chapterID, s.Index, s.Start, s.End, s.Runes, s.Title, s.Summary,
			s.Location, string(cast), s.Cue, s.CueText, OriginAuto, hash, now, now,
		); err != nil {
			return err
		}
	}
	return nil
}

var sceneCols = []string{"id", "chapter_id", "idx", "start_rune", "end_rune", "runes", "title", "summary",
	"location_id", "characters_json", "cue", "cue_text", "origin", "body_hash", "updated_at"}

// sceneColumns lists the scene columns, each prefixed with table.
func sceneColumns(table string) string {
	cols := make([]string, len(sceneCols))
	for i, c := range sceneCols {
		cols[i] = table + c
	}
	return strings.Join(cols, ", ")
}

type sceneRow struct {
	SceneRecord
	hash string
}

func scanScene(scan func(...any) error) (sceneRow, error) {
	var r sceneRow
	var cast string
	err := scan(&r.ID, &r.ChapterID, &r.Index, &r.Start, &r.End, &r.Runes, &r.Title, &r.Summary, &r.Location,
		&cast, &r.Cue, &r.CueText, &r.Origin, &r.hash, &r.UpdatedAt)
	if err != nil {
		return r, err
	}
	if err := json.Unmarshal([]byte(cast), &r.Characters); err != nil || r.Characters == nil {
		r.Characters = []string{}
	}
	return r, nil
}

// LoadScenes reads a chapter's scenes; body is its current text.
func LoadScenes(ctx context.Context, st *store.Store, chapterID, body string) (ChapterScenes, error) {
	rows, err := st.DB().QueryContext(ctx,
		`SELECT `+sceneColumns("")+` FROM chapter_scenes WHERE chapter_id = ? ORDER BY idx`, chapterID)
	if err != nil {
		return ChapterScenes{}, err
	}
	defer rows.Close()
	out := ChapterScenes{Scenes: []SceneRecord{}}
	hash := BodyHash(body)
	for rows.Next() {
		r, err := scanScene(rows.Scan)
		if err != nil {
			return ChapterScenes{}, err
		}
		if r.hash != hash {
			out.Stale = true
		}
		out.Scenes = append(out.Scenes, r.SceneRecord)
	}
	return out, rows.Err()
}

// SceneEdit changes a scene's title or summary; nil leaves a field as is.
type SceneEdit struct {
	Title   *string `json:"title"`
	Summary *string `json:"summary"`
}

// UpdateScene applies an edit to a scene the user owns.
func UpdateScene(ctx context.Context, st *store.Store, userID, sceneID string, edit SceneEdit) (SceneRecord, error) {
	row := st.DB().QueryRowContext(ctx,
		`SELECT `+sceneColumns("s.")+`
		 FROM chapter_scenes s JOIN projects p ON p.id = s.project_id
		 WHERE s.id = ? AND p.user_id = ?`, sceneID, userID)
	r, err := scanScene(row.Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return SceneRecord{}, ErrSceneNotFound
	}
	if err != nil {
		return SceneRecord{}, err
	}
	if edit.Title != nil {
		t := strings.TrimSpace(*edit.Title)
		if t == "" {
			return SceneRecord{}, fmt.Errorf("invalid: 场景标题不能为空")
		}
		if utf8.RuneCountInString(t) > maxSceneTitleRunes {
			return SceneRecord{}, fmt.Errorf("invalid: 场景标题最多 %d 字", maxSceneTitleRunes)
		}
		r.Title = t
	}
	if edit.Summary != nil {
		s := strings.TrimSpace(*edit.Summary)
		if utf8.RuneCountInString(s) > maxSceneSummaryRunes {
			return SceneRecord{}, fmt.Errorf("invalid: 场景摘要最多 %d 字", maxSceneSummaryRunes)
		}
		r.Summary = s
	}
	r.Origin = OriginEdited
	r.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	if _, err := st.DB().ExecContext(ctx,
		`UPDATE chapter_scenes SET title = ?, summary = ?, origin = ?, updated_at = ? WHERE id = ?`,
		r.Title, r.Summary, r.Origin, r.UpdatedAt, r.ID); err != nil {
		return SceneRecord{}, err
	}
	return r.SceneRecord, nil
}

// SplitAll segments every written chapter of a project.
func SplitAll(ctx context.Context, st *store.Store, projectID string) (chapters, scenes int, err error) {
	written, err := LoadWritten(ctx, st, projectID)
	if err != nil {
		return 0, 0, err
	}
	for _, c := range written {
		got, err := SplitChapter(ctx, st, projectID, c.ID, c.Body)
		if err != nil {
			return chapters, scenes, err
		}
		chapters++
		scenes += len(got)
	}
	return chapters, scenes, nil
}

// freshSceneUnits maps chapter id to the texts of its scenes, for chapters
// whose scenes still match their body.
func freshSceneUnits(ctx context.Context, st *store.Store, projectID string, written []Written) (map[string][]string, error) {
	bodies := map[string]string{}
	for _, c := range written {
		bodies[c.ID] = c.Body
	}
	rows, err := st.DB().QueryContext(ctx,
		`SELECT chapter_id, start_rune, end_rune, body_hash FROM chapter_scenes WHERE project_id = ? ORDER BY chapter_id, idx`,
		projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	units := map[string][]string{}
	stale := map[string]bool{}
	runes := map[string][]rune{}
	for rows.Next() {
		var chapterID, hash string
		var start, end int
		if err := rows.Scan(&chapterID, &start, &end, &hash); err != nil {
			return nil, err
		}
		body, ok := bodies[chapterID]
		if !ok || stale[chapterID] || hash != BodyHash(body) {
			stale[chapterID] = true
			continue
		}
		rs, ok := runes[chapterID]
		if !ok {
			rs = []rune(body)
			runes[chapterID] = rs
		}
		if start < 0 || end > len(rs) || start >= end {
			stale[chapterID] = true
			continue
		}
		units[chapterID] = append(units[chapterID], string(rs[start:end]))
	}
	for id := range stale {
		delete(units, id)
	}
	return units, rows.Err()
}
