package job

import (
	"context"
	"encoding/json"
	"errors"
)

type Kind string

const (
	KindExtract Kind = "extract"
	KindFuse    Kind = "fuse"
	KindAudit   Kind = "audit"
	KindSample  Kind = "sample"
)

type Status string

const (
	StatusQueued    Status = "queued"
	StatusRunning   Status = "running"
	StatusSucceeded Status = "succeeded"
	StatusFailed    Status = "failed"
	StatusCanceled  Status = "canceled"
)

type Record struct {
	ID        string          `json:"id"`
	UserID    string          `json:"user_id"`
	ProjectID string          `json:"project_id"`
	Kind      Kind            `json:"kind"`
	Status    Status          `json:"status"`
	Progress  int             `json:"progress"`
	Stage     string          `json:"stage"`
	Payload   json.RawMessage `json:"payload"`
	Result    json.RawMessage `json:"result,omitempty"`
	Error     string          `json:"error,omitempty"`
}

type Handler func(ctx context.Context, rec Record, prog func(progress int, stage string)) (result json.RawMessage, err error)

var ErrNotFound = errors.New("not found")
