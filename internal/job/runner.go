package job

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"stylelab/internal/ids"
	"stylelab/internal/store"
)

type Runner struct {
	st          *store.Store
	concurrency int

	mu       sync.Mutex
	handlers map[Kind]Handler
	cancels  map[string]context.CancelFunc
	canceled map[string]struct{}
	wake     chan struct{}
	start    sync.Once
}

func NewRunner(st *store.Store, concurrency int) *Runner {
	if concurrency < 1 {
		concurrency = 1
	}
	return &Runner{
		st:          st,
		concurrency: concurrency,
		handlers:    make(map[Kind]Handler),
		cancels:     make(map[string]context.CancelFunc),
		canceled:    make(map[string]struct{}),
		wake:        make(chan struct{}, 1),
	}
}

func (r *Runner) Register(k Kind, h Handler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[k] = h
}

func (r *Runner) Start(ctx context.Context) {
	r.start.Do(func() {
		for i := 0; i < r.concurrency; i++ {
			go r.worker(ctx)
		}
	})
}

func (r *Runner) Enqueue(ctx context.Context, rec Record) (string, error) {
	if rec.ID == "" {
		rec.ID = ids.New("job_")
	}
	if rec.Payload == nil {
		rec.Payload = json.RawMessage(`{}`)
	}
	_, err := r.st.DB().ExecContext(
		ctx,
		`INSERT INTO jobs (id, user_id, project_id, kind, status, progress, stage, payload_json, created_at)
		 VALUES (?, ?, ?, ?, ?, 0, '', ?, ?)`,
		rec.ID, rec.UserID, rec.ProjectID, string(rec.Kind), string(StatusQueued), string(rec.Payload),
		time.Now().UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return "", err
	}
	r.nudge()
	return rec.ID, nil
}

func (r *Runner) Cancel(ctx context.Context, id, userID string) error {
	if _, err := r.Get(ctx, id, userID); err != nil {
		return err
	}
	r.mu.Lock()
	r.canceled[id] = struct{}{}
	if cancel, ok := r.cancels[id]; ok {
		cancel()
	}
	r.mu.Unlock()

	_, err := r.st.DB().ExecContext(
		ctx,
		`UPDATE jobs SET status = ?, finished_at = ? WHERE id = ? AND user_id = ? AND status IN (?, ?)`,
		string(StatusCanceled), time.Now().UTC().Format(time.RFC3339Nano),
		id, userID, string(StatusQueued), string(StatusRunning),
	)
	return err
}

func (r *Runner) Get(ctx context.Context, id, userID string) (Record, error) {
	rec, err := scanRecord(r.st.DB().QueryRowContext(
		ctx,
		`SELECT id, user_id, project_id, kind, status, progress, stage, payload_json, result_json, error
		 FROM jobs WHERE id = ? AND user_id = ?`,
		id, userID,
	))
	if err != nil {
		if err == sql.ErrNoRows {
			return Record{}, ErrNotFound
		}
		return Record{}, err
	}
	return rec, nil
}

func (r *Runner) RecoverInterrupted(ctx context.Context) (int, error) {
	res, err := r.st.DB().ExecContext(
		ctx,
		`UPDATE jobs SET status = ?, error = 'interrupted', finished_at = ? WHERE status = ?`,
		string(StatusFailed), time.Now().UTC().Format(time.RFC3339Nano), string(StatusRunning),
	)
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	return int(n), err
}

func (r *Runner) worker(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			return
		}
		rec, ok, err := r.claim(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(50 * time.Millisecond):
			}
			continue
		}
		if ok {
			r.execute(ctx, rec)
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-r.wake:
		case <-time.After(50 * time.Millisecond):
		}
	}
}

func (r *Runner) claim(ctx context.Context) (Record, bool, error) {
	tx, err := r.st.DB().BeginTx(ctx, nil)
	if err != nil {
		return Record{}, false, err
	}
	defer tx.Rollback()

	startedAt := time.Now().UTC().Format(time.RFC3339Nano)
	rec, err := scanRecord(tx.QueryRowContext(
		ctx,
		`UPDATE jobs SET status='running', started_at=?
		 WHERE id=(SELECT id FROM jobs WHERE status='queued' ORDER BY created_at LIMIT 1)
		 RETURNING id, user_id, project_id, kind, status, progress, stage, payload_json, result_json, error`,
		startedAt,
	))
	if err == sql.ErrNoRows {
		if err := tx.Commit(); err != nil {
			return Record{}, false, err
		}
		return Record{}, false, nil
	}
	if err != nil {
		return Record{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return Record{}, false, err
	}
	return rec, true, nil
}

func (r *Runner) execute(ctx context.Context, rec Record) {
	r.mu.Lock()
	if _, done := r.canceled[rec.ID]; done {
		r.mu.Unlock()
		return
	}
	jobCtx, cancel := context.WithCancel(ctx)
	r.cancels[rec.ID] = cancel
	h := r.handlers[rec.Kind]
	r.mu.Unlock()

	defer func() {
		cancel()
		r.mu.Lock()
		delete(r.cancels, rec.ID)
		delete(r.canceled, rec.ID)
		r.mu.Unlock()
	}()

	if h == nil {
		r.finish(rec.ID, nil, fmt.Errorf("no handler for kind %s", rec.Kind))
		return
	}

	result, err := h(jobCtx, rec, func(progress int, stage string) {
		if progress < 0 {
			progress = 0
		}
		if progress > 100 {
			progress = 100
		}
		_, _ = r.st.DB().ExecContext(
			context.Background(),
			`UPDATE jobs SET progress = ?, stage = ? WHERE id = ? AND status = ?`,
			progress, stage, rec.ID, string(StatusRunning),
		)
	})
	r.finish(rec.ID, result, err)
}

func (r *Runner) finish(id string, result json.RawMessage, err error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if err != nil {
		_, _ = r.st.DB().Exec(
			`UPDATE jobs SET status = ?, error = ?, result_json = ?, finished_at = ? WHERE id = ? AND status = ?`,
			string(StatusFailed), err.Error(), nullJSON(result), now, id, string(StatusRunning),
		)
		return
	}
	_, _ = r.st.DB().Exec(
		`UPDATE jobs SET status = ?, result_json = ?, progress = 100, error = NULL, finished_at = ? WHERE id = ? AND status = ?`,
		string(StatusSucceeded), nullJSON(result), now, id, string(StatusRunning),
	)
}

func (r *Runner) nudge() {
	select {
	case r.wake <- struct{}{}:
	default:
	}
}

func nullJSON(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}
	return string(raw)
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanRecord(row rowScanner) (Record, error) {
	var rec Record
	var kind, status, payload string
	var result, errStr sql.NullString
	err := row.Scan(
		&rec.ID, &rec.UserID, &rec.ProjectID, &kind, &status,
		&rec.Progress, &rec.Stage, &payload, &result, &errStr,
	)
	if err != nil {
		return Record{}, err
	}
	rec.Kind = Kind(kind)
	rec.Status = Status(status)
	rec.Payload = json.RawMessage(payload)
	if result.Valid {
		rec.Result = json.RawMessage(result.String)
	}
	if errStr.Valid {
		rec.Error = errStr.String
	}
	return rec, nil
}
