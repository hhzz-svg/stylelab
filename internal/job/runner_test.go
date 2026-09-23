package job_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"stylelab/internal/ids"
	"stylelab/internal/job"
	"stylelab/internal/store"
)

func TestRunnerTwoJobsSucceed(t *testing.T) {
	st := openStore(t)
	uid, pid := seedUserProject(t, st)
	r := job.NewRunner(st, 2)
	r.Register(job.KindExtract, func(ctx context.Context, rec job.Record, prog func(int, string)) (json.RawMessage, error) {
		prog(50, "halfway")
		return json.RawMessage(`{"ok":true}`), nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() { cancel(); r.Wait() })

	id1, err := r.Enqueue(ctx, job.Record{
		UserID:    uid,
		ProjectID: pid,
		Kind:      job.KindExtract,
		Payload:   json.RawMessage(`{"n":1}`),
	})
	if err != nil {
		t.Fatalf("enqueue 1: %v", err)
	}
	id2, err := r.Enqueue(ctx, job.Record{
		UserID:    uid,
		ProjectID: pid,
		Kind:      job.KindExtract,
		Payload:   json.RawMessage(`{"n":2}`),
	})
	if err != nil {
		t.Fatalf("enqueue 2: %v", err)
	}
	if !ids.Valid(id1, "job_") || !ids.Valid(id2, "job_") {
		t.Fatalf("ids: %s %s", id1, id2)
	}

	r.Start(ctx)
	a := waitJob(t, r, id1, uid, func(rec job.Record) bool { return rec.Status == job.StatusSucceeded })
	b := waitJob(t, r, id2, uid, func(rec job.Record) bool { return rec.Status == job.StatusSucceeded })
	if string(a.Result) != `{"ok":true}` || string(b.Result) != `{"ok":true}` {
		t.Fatalf("results: %s %s", a.Result, b.Result)
	}
}

func TestRunnerCancelBlockingHandler(t *testing.T) {
	st := openStore(t)
	uid, pid := seedUserProject(t, st)
	r := job.NewRunner(st, 1)
	started := make(chan struct{})
	r.Register(job.KindExtract, func(ctx context.Context, rec job.Record, prog func(int, string)) (json.RawMessage, error) {
		close(started)
		<-ctx.Done()
		return nil, ctx.Err()
	})

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() { cancel(); r.Wait() })

	id, err := r.Enqueue(ctx, job.Record{
		UserID:    uid,
		ProjectID: pid,
		Kind:      job.KindExtract,
		Payload:   json.RawMessage(`{}`),
	})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	r.Start(ctx)

	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("handler did not start")
	}

	if err := r.Cancel(ctx, id, uid); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	rec := waitJob(t, r, id, uid, func(rec job.Record) bool {
		return rec.Status == job.StatusCanceled || rec.Status == job.StatusFailed
	})
	if rec.Status == job.StatusRunning {
		t.Fatalf("job left running: %+v", rec)
	}
}

func TestRecoverInterrupted(t *testing.T) {
	st := openStore(t)
	uid, pid := seedUserProject(t, st)
	r := job.NewRunner(st, 1)

	id := ids.New("job_")
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := st.DB().Exec(
		`INSERT INTO jobs (id, user_id, project_id, kind, status, progress, stage, payload_json, created_at, started_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, uid, pid, string(job.KindExtract), string(job.StatusRunning), 40, "work", `{}`, now, now,
	)
	if err != nil {
		t.Fatalf("insert running: %v", err)
	}

	n, err := r.RecoverInterrupted(context.Background())
	if err != nil {
		t.Fatalf("RecoverInterrupted: %v", err)
	}
	if n != 1 {
		t.Fatalf("recovered count: %d", n)
	}
	rec, err := r.Get(context.Background(), id, uid)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if rec.Status != job.StatusFailed {
		t.Fatalf("status: %s", rec.Status)
	}
	if rec.Error != "interrupted" {
		t.Fatalf("error: %q", rec.Error)
	}
}

func openStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() {
		if err := st.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	})
	return st
}

func seedUserProject(t *testing.T, st *store.Store) (string, string) {
	t.Helper()
	uid := ids.New("usr_")
	pid := ids.New("prj_")
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := st.DB().Exec(
		`INSERT INTO users (id, email, password_hash, created_at) VALUES (?, ?, ?, ?)`,
		uid, uid+"@example.com", "x", now,
	)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
	_, err = st.DB().Exec(
		`INSERT INTO projects (id, user_id, name, created_at) VALUES (?, ?, ?, ?)`,
		pid, uid, "p", now,
	)
	if err != nil {
		t.Fatalf("insert project: %v", err)
	}
	return uid, pid
}

func waitJob(t *testing.T, r *job.Runner, id, userID string, pred func(job.Record) bool) job.Record {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	var last job.Record
	for time.Now().Before(deadline) {
		rec, err := r.Get(context.Background(), id, userID)
		if err == nil {
			last = rec
			if pred(rec) {
				return rec
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for job %s last=%+v", id, last)
	return last
}

// Jobs must be claimed in the order they were queued. created_at is written as
// RFC3339Nano, which trims trailing zeros, so those strings do not sort
// chronologically: "…05.1Z" (earlier) sorts after "…05.12Z" (later), because
// '2' < 'Z'. Ordering the queue by created_at therefore ran such jobs out of
// order.
func TestRunnerClaimsInEnqueueOrder(t *testing.T) {
	st := openStore(t)
	uid, pid := seedUserProject(t, st)

	queued := []struct{ id, createdAt string }{
		{"job_000000000000000a", "2026-09-23T10:00:05.1Z"},  // first queued
		{"job_000000000000000b", "2026-09-23T10:00:05.12Z"}, // second queued
	}
	for _, q := range queued {
		if _, err := st.DB().Exec(
			`INSERT INTO jobs (id, user_id, project_id, kind, status, progress, stage, payload_json, created_at)
			 VALUES (?, ?, ?, ?, 'queued', 0, '', '{}', ?)`,
			q.id, uid, pid, string(job.KindExtract), q.createdAt,
		); err != nil {
			t.Fatalf("insert %s: %v", q.id, err)
		}
	}

	order := make(chan string, len(queued))
	r := job.NewRunner(st, 1) // one worker, so claim order is run order
	r.Register(job.KindExtract, func(ctx context.Context, rec job.Record, prog func(int, string)) (json.RawMessage, error) {
		order <- rec.ID
		return json.RawMessage(`{}`), nil
	})
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() { cancel(); r.Wait() })
	r.Start(ctx)

	for i, want := range queued {
		select {
		case got := <-order:
			if got != want.id {
				t.Fatalf("claim %d = %s, want %s (queued first)", i, got, want.id)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("claim %d: timed out", i)
		}
	}
}
