package studio

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"stylelab/internal/job"
	"stylelab/internal/store"
)

// Latest is the most recent successful result of a studio job.
//
// There is no table for these: the jobs table already persists result_json and
// finished_at and nothing deletes from it, so the newest succeeded row of a
// kind IS the saved result. That ties the results' lifetime to the jobs table
// -- anything that ever prunes jobs must keep the newest succeeded row per
// (project, kind[, chapter]) or move these to a table of their own first.
type Latest struct {
	JobID      string          `json:"job_id"`
	FinishedAt string          `json:"finished_at"`
	Result     json.RawMessage `json:"result"`
}

// ErrUnknownKind is returned for a kind that has no reusable result: the
// continue job's output is written straight into the editor, and the other
// job kinds persist rows of their own.
var ErrUnknownKind = errors.New("invalid: unsupported kind")

// ProjectScoped reports whether kind is looked up per project; BranchKind is
// per chapter.
func ProjectScoped(kind job.Kind) bool {
	return kind == job.KindOutline || kind == job.KindContinuity
}

// LatestResult returns the newest successful result, or nil when there is
// none. chapterID is required for job.KindBranch and ignored otherwise.
//
// Newest means most recently requested, by insertion order (rowid). The
// timestamps are RFC3339Nano, which trims trailing zeros, so they do not sort
// correctly as strings.
func LatestResult(
	ctx context.Context, st *store.Store,
	userID, projectID string, kind job.Kind, chapterID string,
) (*Latest, error) {
	query := `SELECT id, COALESCE(finished_at, ''), result_json FROM jobs
		 WHERE user_id = ? AND project_id = ? AND kind = ? AND status = ?
		   AND result_json IS NOT NULL`
	args := []any{userID, projectID, string(kind), string(job.StatusSucceeded)}

	switch {
	case ProjectScoped(kind):
	case kind == job.KindBranch:
		if chapterID == "" {
			return nil, fmt.Errorf("invalid: chapter required")
		}
		query += ` AND json_extract(payload_json, '$.chapter_id') = ?`
		args = append(args, chapterID)
	default:
		return nil, ErrUnknownKind
	}
	query += ` ORDER BY rowid DESC LIMIT 1`

	var out Latest
	var raw string
	err := st.DB().QueryRowContext(ctx, query, args...).Scan(&out.JobID, &out.FinishedAt, &raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	out.Result = json.RawMessage(raw)
	return &out, nil
}
