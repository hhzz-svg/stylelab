package lore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"stylelab/internal/ids"
	"stylelab/internal/store"
)

const (
	maxVolumeTitleRunes = 60
	maxVolumeBriefRunes = 1000
)

// ErrVolumeNotFound is returned for a volume that is missing or not the
// caller's.
var ErrVolumeNotFound = errors.New("volume not found")

// ErrVolumeStartTaken: two volumes cannot begin at the same chapter.
var ErrVolumeStartTaken = errors.New("invalid: 这一章已经是另一卷的开头")

// Volume groups the chapters from StartSeq up to the next volume.
type Volume struct {
	ID        string `json:"id"`
	StartSeq  int    `json:"start_seq"`
	Title     string `json:"title"`
	Brief     string `json:"brief"`
	UpdatedAt string `json:"updated_at"`
}

// VolumeInput creates or edits a volume; nil fields are left as they are.
type VolumeInput struct {
	StartSeq *int    `json:"start_seq"`
	Title    *string `json:"title"`
	Brief    *string `json:"brief"`
}

func validateVolume(v *Volume) error {
	v.Title = strings.TrimSpace(v.Title)
	v.Brief = strings.TrimSpace(v.Brief)
	if v.Title == "" {
		return fmt.Errorf("invalid: 卷名不能为空")
	}
	if utf8.RuneCountInString(v.Title) > maxVolumeTitleRunes {
		return fmt.Errorf("invalid: 卷名最多 %d 字", maxVolumeTitleRunes)
	}
	if utf8.RuneCountInString(v.Brief) > maxVolumeBriefRunes {
		return fmt.Errorf("invalid: 卷简介最多 %d 字", maxVolumeBriefRunes)
	}
	if v.StartSeq < 1 {
		return fmt.Errorf("invalid: 起始章节必须是正整数")
	}
	return nil
}

func isUnique(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE")
}

// ListVolumes returns a project's volumes in order.
func ListVolumes(ctx context.Context, st *store.Store, projectID string) ([]Volume, error) {
	rows, err := st.DB().QueryContext(ctx,
		`SELECT id, start_seq, title, brief, updated_at FROM volumes WHERE project_id = ? ORDER BY start_seq`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Volume{}
	for rows.Next() {
		var v Volume
		if err := rows.Scan(&v.ID, &v.StartSeq, &v.Title, &v.Brief, &v.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// CreateVolume starts a volume at a chapter number.
func CreateVolume(ctx context.Context, st *store.Store, projectID string, in VolumeInput) (Volume, error) {
	v := Volume{}
	if in.StartSeq != nil {
		v.StartSeq = *in.StartSeq
	}
	if in.Title != nil {
		v.Title = *in.Title
	}
	if in.Brief != nil {
		v.Brief = *in.Brief
	}
	if err := validateVolume(&v); err != nil {
		return Volume{}, err
	}
	v.ID = ids.New("vol")
	v.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	_, err := st.DB().ExecContext(ctx,
		`INSERT INTO volumes (id, project_id, start_seq, title, brief, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		v.ID, projectID, v.StartSeq, v.Title, v.Brief, v.UpdatedAt, v.UpdatedAt)
	if isUnique(err) {
		return Volume{}, ErrVolumeStartTaken
	}
	return v, err
}

// UpdateVolume edits a volume the user owns.
func UpdateVolume(ctx context.Context, st *store.Store, userID, volumeID string, in VolumeInput) (Volume, error) {
	var v Volume
	err := st.DB().QueryRowContext(ctx,
		`SELECT v.id, v.start_seq, v.title, v.brief, v.updated_at
		 FROM volumes v JOIN projects p ON p.id = v.project_id
		 WHERE v.id = ? AND p.user_id = ?`, volumeID, userID,
	).Scan(&v.ID, &v.StartSeq, &v.Title, &v.Brief, &v.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Volume{}, ErrVolumeNotFound
	}
	if err != nil {
		return Volume{}, err
	}
	if in.StartSeq != nil {
		v.StartSeq = *in.StartSeq
	}
	if in.Title != nil {
		v.Title = *in.Title
	}
	if in.Brief != nil {
		v.Brief = *in.Brief
	}
	if err := validateVolume(&v); err != nil {
		return Volume{}, err
	}
	v.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	_, err = st.DB().ExecContext(ctx,
		`UPDATE volumes SET start_seq = ?, title = ?, brief = ?, updated_at = ? WHERE id = ?`,
		v.StartSeq, v.Title, v.Brief, v.UpdatedAt, v.ID)
	if isUnique(err) {
		return Volume{}, ErrVolumeStartTaken
	}
	return v, err
}

// DeleteVolume removes a volume; its chapters join the volume before it.
func DeleteVolume(ctx context.Context, st *store.Store, userID, volumeID string) error {
	res, err := st.DB().ExecContext(ctx,
		`DELETE FROM volumes WHERE id = ? AND project_id IN (SELECT id FROM projects WHERE user_id = ?)`,
		volumeID, userID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrVolumeNotFound
	}
	return nil
}

// OutlineVolume is a volume in an imported outline: a title, a brief and
// how many of the imported chapters it holds.
type OutlineVolume struct {
	Title        string `json:"title"`
	Brief        string `json:"brief"`
	ChapterCount int    `json:"chapter_count"`
}

// ValidateOutlineVolumes checks an outline's volumes against the number of
// chapters imported with it.
func ValidateOutlineVolumes(vols []OutlineVolume, chapters int) error {
	total := 0
	for i, v := range vols {
		if v.ChapterCount < 0 {
			return fmt.Errorf("invalid: 第 %d 卷的章节数不能为负", i+1)
		}
		total += v.ChapterCount
	}
	if len(vols) > 0 && total != chapters {
		return fmt.Errorf("invalid: 分卷共 %d 章，与导入的 %d 章不符", total, chapters)
	}
	return nil
}

// ReplaceVolumesFrom is used by outline import inside its transaction:
// volumes starting at or after startSeq pointed at chapters that no longer
// exist or are being replaced, so they go; then each imported volume starts
// where its chapters do. Volumes with no chapters are skipped.
func ReplaceVolumesFrom(ctx context.Context, tx *sql.Tx, projectID string, startSeq int, vols []OutlineVolume) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM volumes WHERE project_id = ? AND start_seq >= ?`, projectID, startSeq); err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	seq := startSeq
	for i, ov := range vols {
		if ov.ChapterCount == 0 {
			continue
		}
		v := Volume{StartSeq: seq, Title: ov.Title, Brief: ov.Brief}
		if strings.TrimSpace(v.Title) == "" {
			v.Title = fmt.Sprintf("第%d卷", i+1)
		}
		v.Title = headRunes(strings.TrimSpace(v.Title), maxVolumeTitleRunes)
		v.Brief = headRunes(strings.TrimSpace(v.Brief), maxVolumeBriefRunes)
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO volumes (id, project_id, start_seq, title, brief, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			ids.New("vol"), projectID, v.StartSeq, v.Title, v.Brief, now, now); err != nil {
			return err
		}
		seq += ov.ChapterCount
	}
	return nil
}

func headRunes(s string, n int) string {
	rs := []rune(s)
	if len(rs) <= n {
		return s
	}
	return string(rs[:n])
}

// ShiftVolumesAfterDelete keeps volumes on their chapters when chapter seq
// is deleted and every later chapter moves up one. Call it inside the
// deleting transaction.
//
// A volume that started at seq keeps its start: the chapter that moves
// into seq was its own second chapter. If that volume held only the
// deleted chapter -- the next volume starts right after -- it is removed.
// Later volumes move up by one, via negative values so the unique
// (project, start) index never sees two volumes on the same chapter
// mid-update.
func ShiftVolumesAfterDelete(ctx context.Context, tx *sql.Tx, projectID string, seq int) error {
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM volumes WHERE project_id = ? AND start_seq = ?
		   AND EXISTS (SELECT 1 FROM volumes n WHERE n.project_id = ? AND n.start_seq = ?)`,
		projectID, seq, projectID, seq+1); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE volumes SET start_seq = -(start_seq - 1) WHERE project_id = ? AND start_seq > ?`, projectID, seq); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx,
		`UPDATE volumes SET start_seq = -start_seq WHERE project_id = ? AND start_seq < 0`, projectID)
	return err
}
