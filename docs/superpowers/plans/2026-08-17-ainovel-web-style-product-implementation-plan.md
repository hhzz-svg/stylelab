# Style Lab Web MVP Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a single-binary Go + React web app where a user can upload novel samples, extract/fuse/edit style cards, run dual-persona audits, and generate a short sample chapter — without imitating named authors.

**Architecture:** One Go module `stylelab`. `cmd/stylelab` serves HTTP, runs an in-process job pool, and embeds the Vite-built SPA. SQLite in dev (same SQL works on Postgres later). LLM calls use the user's encrypted BYOK key via an OpenAI-compatible client. Style cards are the core persisted object; extract/fuse/audit/sample are job kinds.

**Tech Stack:** Go 1.22+, `net/http` + `chi`, `modernc.org/sqlite`, `golang.org/x/crypto`, React 18 + TypeScript + Vite + Tailwind. No Gin, no GORM, no Redis, no MinIO in v1.

## Global Constraints

- Module path: `stylelab`. Go version in `go.mod`: `1.22`.
- Product copy and every LLM system prompt MUST NOT contain: `模仿作者`, `复刻作者`, `还原作者`, `像某某写的`, `仿写某某`.
- Style card dimensions are exactly these nine keys, never renamed: `sentence_rhythm`, `narrative_perspective`, `dialogue_density`, `sensory_description`, `scene_pacing`, `emotional_expression`, `rhetoric_preference`, `lexical_texture`, `tension_hook`.
- Dimension `level` is an integer 0–100. `summary` ≤ 100 runes. `techniques` has 3–5 items, each ≤ 40 runes. `kind` is `extracted` | `fused` | `manual`.
- Cards must not store long source excerpts. `facts` holds counts only. Signature phrases go only into `prohibitions`.
- LLM keys are AES-256-GCM encrypted with `STYLELAB_MASTER_KEY` (64 hex chars = 32 bytes). API never returns the plaintext key; only `provider` and last 4 chars.
- Jobs: kinds `extract` | `fuse` | `audit` | `sample`. Statuses `queued` | `running` | `succeeded` | `failed` | `canceled`. In-process worker pool, default concurrency 2. On process start, any `running` row becomes `failed` with error `interrupted`.
- Auth: email + bcrypt password, session token in httpOnly cookie `stylelab_session`, 14-day expiry, token stored as SHA-256 hex in `sessions`.
- IDs: prefix + 16 lowercase hex chars from crypto/rand. Prefixes: `usr_`, `prj_`, `ast_`, `crd_`, `job_`, `aud_`, `smp_`, `ses_`.
- Data dir default `./data`. Blobs at `{data_dir}/blobs/{sha256}`. SQLite at `{data_dir}/stylelab.db`.
- HTTP JSON errors: `{"error":{"code":"unauthorized|forbidden|not_found|invalid|conflict|job_failed","message":"..."}}`.
- v1 must not add OAuth, billing, Redis, MinIO, GORM, Gin, custom personas, custom dimensions, or ainovel-cli process calls.
- Work happens in `E:\小说agent`. Commit after each task. Do not commit secrets.

---

## File map

Create these files across the plan. Do not invent parallel trees.

```
go.mod
cmd/stylelab/main.go
internal/ids/ids.go
internal/ids/ids_test.go
internal/config/config.go
internal/config/config_test.go
internal/store/db.go
internal/store/migrate.go
internal/store/store_test.go
internal/auth/password.go
internal/auth/password_test.go
internal/auth/session.go
internal/auth/service.go
internal/auth/service_test.go
internal/cryptokey/cryptokey.go
internal/cryptokey/cryptokey_test.go
internal/httpapi/server.go
internal/httpapi/auth.go
internal/httpapi/projects.go
internal/httpapi/assets.go
internal/httpapi/cards.go
internal/httpapi/jobs.go
internal/httpapi/keys.go
internal/httpapi/middleware.go
internal/httpapi/httpapi_test.go
internal/card/card.go
internal/card/validate.go
internal/card/validate_test.go
internal/card/fuse_math.go
internal/card/fuse_math_test.go
internal/card/export.go
internal/card/export_test.go
internal/stylestat/stylestat.go
internal/stylestat/stylestat_test.go
internal/llm/client.go
internal/llm/client_test.go
internal/llm/prompts.go
internal/job/job.go
internal/job/runner.go
internal/job/runner_test.go
internal/extract/extract.go
internal/extract/extract_test.go
internal/fuse/fuse.go
internal/fuse/fuse_test.go
internal/audit/audit.go
internal/audit/audit_test.go
internal/sample/sample.go
internal/sample/sample_test.go
web/package.json
web/tsconfig.json
web/vite.config.ts
web/index.html
web/src/main.tsx
web/src/App.tsx
web/src/api.ts
web/src/types.ts
web/src/pages/Login.tsx
web/src/pages/Projects.tsx
web/src/pages/ProjectHome.tsx
web/src/pages/Lab.tsx
web/src/pages/Fuse.tsx
web/src/pages/Audit.tsx
web/src/pages/Sample.tsx
web/src/pages/Settings.tsx
web/src/components/JobProgress.tsx
web/src/components/DimensionSliders.tsx
Dockerfile
docker-compose.yml
README.md
```

---

### Task 1: IDs, config, and a bootable HTTP binary

**Files:**
- Create: `go.mod`
- Create: `internal/ids/ids.go`
- Create: `internal/ids/ids_test.go`
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`
- Create: `cmd/stylelab/main.go`
- Create: `README.md`

**Interfaces:**
- Consumes: nothing
- Produces:
  - `ids.New(prefix string) string` — `prefix` + 16 hex chars
  - `ids.Valid(id, prefix string) bool`
  - `config.Config` with fields `Addr string`, `DataDir string`, `MasterKey []byte`, `DevAutoLogin bool`, `WorkerConcurrency int`
  - `config.Load() (Config, error)` reads `STYLELAB_ADDR` (default `:8080`), `STYLELAB_DATA_DIR` (default `./data`), `STYLELAB_MASTER_KEY` (required unless `STYLELAB_DEV_INSECURE_KEY=1`, which uses 32 zero bytes and is test-only), `STYLELAB_DEV_AUTO_LOGIN` (`1` = true), `STYLELAB_WORKERS` (default `2`)
  - `cmd/stylelab` listens and serves `GET /api/health` → `{"ok":true}`

- [ ] **Step 1: Write the failing ID tests**

```go
package ids_test

import (
	"strings"
	"testing"

	"stylelab/internal/ids"
)

func TestNewHasPrefixAndHexLength(t *testing.T) {
	id := ids.New("usr_")
	if !strings.HasPrefix(id, "usr_") {
		t.Fatalf("prefix: %s", id)
	}
	if !ids.Valid(id, "usr_") {
		t.Fatalf("expected valid %s", id)
	}
	a, b := ids.New("usr_"), ids.New("usr_")
	if a == b {
		t.Fatal("expected unique ids")
	}
}

func TestValidRejectsBad(t *testing.T) {
	if ids.Valid("usr_zz", "usr_") {
		t.Fatal("short/non-hex should be invalid")
	}
	if ids.Valid("prj_0123456789abcdef", "usr_") {
		t.Fatal("wrong prefix should be invalid")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ids/ -count=1`
Expected: FAIL — package or symbols not found.

- [ ] **Step 3: Implement IDs**

```go
package ids

import (
	"crypto/rand"
	"encoding/hex"
)

func New(prefix string) string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return prefix + hex.EncodeToString(b[:])
}

func Valid(id, prefix string) bool {
	if !stringsHasPrefix(id, prefix) {
		return false
	}
	rest := id[len(prefix):]
	if len(rest) != 16 {
		return false
	}
	for i := 0; i < len(rest); i++ {
		c := rest[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

func stringsHasPrefix(s, p string) bool {
	return len(s) >= len(p) && s[:len(p)] == p
}
```

Use `"strings"` in the real file instead of the local helper if you prefer; either is fine.

- [ ] **Step 4: Write config tests then implement `config.Load`**

Test cases:
- unset env + `STYLELAB_DEV_INSECURE_KEY=1` → Addr `:8080`, DataDir `./data`, Workers 2, MasterKey 32 zero bytes
- `STYLELAB_MASTER_KEY` not 64 hex chars and insecure flag off → error
- `STYLELAB_WORKERS=0` → error
- `STYLELAB_DEV_AUTO_LOGIN=1` → DevAutoLogin true

- [ ] **Step 5: `cmd/stylelab/main.go` serves health**

`GET /api/health` returns 200 and `{"ok":true}`. Load config; if Load fails, print error and exit 1. Create DataDir with `mkdirAll` 0o755.

- [ ] **Step 6: Run tests and a one-shot health check**

Run: `go test ./... -count=1`
Expected: PASS.

Then: `STYLELAB_DEV_INSECURE_KEY=1 go run ./cmd/stylelab` in background, `curl -s localhost:8080/api/health`, stop the process.

- [ ] **Step 7: Commit**

```bash
git add go.mod go.sum cmd/stylelab/main.go internal/ids internal/config README.md
git commit -m "feat: bootstrap stylelab binary, ids, and config"
```

---

### Task 2: SQLite store and schema

**Files:**
- Create: `internal/store/db.go`
- Create: `internal/store/migrate.go`
- Create: `internal/store/store_test.go`

**Interfaces:**
- Consumes: `config.Config.DataDir`
- Produces:
  - `store.Open(dataDir string) (*Store, error)`
  - `func (s *Store) Close() error`
  - `func (s *Store) DB() *sql.DB`
  - tables created by `migrate`: `users`, `sessions`, `user_llm_keys`, `projects`, `assets`, `style_cards`, `style_card_versions`, `jobs`, `audit_reports`, `samples`
  - blob dir `{dataDir}/blobs` created

Schema (SQLite):

```sql
CREATE TABLE users (
  id TEXT PRIMARY KEY,
  email TEXT NOT NULL UNIQUE,
  password_hash TEXT NOT NULL,
  created_at TEXT NOT NULL
);
CREATE TABLE sessions (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token_hash TEXT NOT NULL UNIQUE,
  expires_at TEXT NOT NULL
);
CREATE TABLE user_llm_keys (
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  provider TEXT NOT NULL,
  base_url TEXT NOT NULL DEFAULT '',
  encrypted_key BLOB NOT NULL,
  last4 TEXT NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  PRIMARY KEY (user_id, provider)
);
CREATE TABLE projects (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  created_at TEXT NOT NULL
);
CREATE TABLE assets (
  id TEXT PRIMARY KEY,
  project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  filename TEXT NOT NULL,
  sha256 TEXT NOT NULL,
  rune_count INTEGER NOT NULL,
  chapter_count INTEGER NOT NULL,
  created_at TEXT NOT NULL
);
CREATE TABLE style_cards (
  id TEXT PRIMARY KEY,
  project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  kind TEXT NOT NULL,
  current_version INTEGER NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
CREATE TABLE style_card_versions (
  card_id TEXT NOT NULL REFERENCES style_cards(id) ON DELETE CASCADE,
  version INTEGER NOT NULL,
  dimensions_json TEXT NOT NULL,
  prohibitions_json TEXT NOT NULL,
  facts_json TEXT NOT NULL,
  lineage_json TEXT,
  created_at TEXT NOT NULL,
  PRIMARY KEY (card_id, version)
);
CREATE TABLE jobs (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  kind TEXT NOT NULL,
  status TEXT NOT NULL,
  progress INTEGER NOT NULL DEFAULT 0,
  stage TEXT NOT NULL DEFAULT '',
  payload_json TEXT NOT NULL,
  result_json TEXT,
  error TEXT,
  created_at TEXT NOT NULL,
  started_at TEXT,
  finished_at TEXT
);
CREATE TABLE audit_reports (
  id TEXT PRIMARY KEY,
  project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  card_id TEXT NOT NULL,
  card_version INTEGER NOT NULL,
  report_json TEXT NOT NULL,
  created_at TEXT NOT NULL
);
CREATE TABLE samples (
  id TEXT PRIMARY KEY,
  project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  card_id TEXT NOT NULL,
  card_version INTEGER NOT NULL,
  premise TEXT NOT NULL,
  body TEXT NOT NULL,
  facts_json TEXT NOT NULL,
  created_at TEXT NOT NULL
);
```

Enable `PRAGMA foreign_keys = ON` and `PRAGMA busy_timeout = 5000`.

- [ ] **Step 1: Write `TestOpenCreatesTablesAndBlobDir`**

Open a temp dir, `store.Open`, query `SELECT name FROM sqlite_master WHERE type='table'`, assert all 10 table names exist, assert `blobs` dir exists, Close.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/store/ -count=1`
Expected: FAIL.

- [ ] **Step 3: Implement Open + migrate**

Use `modernc.org/sqlite` driver name `"sqlite"`. DSN: `file:{dataDir}/stylelab.db?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)`.

- [ ] **Step 4: Run tests**

Run: `go test ./internal/store/ -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/store go.mod go.sum
git commit -m "feat: sqlite schema and blob directory"
```

---

### Task 3: Auth service and HTTP routes

**Files:**
- Create: `internal/auth/password.go`
- Create: `internal/auth/password_test.go`
- Create: `internal/auth/session.go`
- Create: `internal/auth/service.go`
- Create: `internal/auth/service_test.go`
- Create: `internal/httpapi/server.go`
- Create: `internal/httpapi/auth.go`
- Create: `internal/httpapi/middleware.go`
- Create: `internal/httpapi/httpapi_test.go`
- Modify: `cmd/stylelab/main.go` to construct store + httpapi and mount routes

**Interfaces:**
- Consumes: `store.Store`, `ids.New`
- Produces:
  - `auth.HashPassword(pw string) (string, error)` bcrypt cost 12
  - `auth.CheckPassword(hash, pw string) bool`
  - `auth.HashToken(raw string) string` SHA-256 hex
  - `auth.Service` methods:
    - `Register(ctx, email, password) (userID string, err error)`
    - `Login(ctx, email, password) (rawToken string, userID string, err error)`
    - `Logout(ctx, rawToken) error`
    - `UserIDFromToken(ctx, rawToken) (userID string, error)`
  - Register rejects password shorter than 8 runes, email missing `@`, duplicate email (`err = auth.ErrInvalid` or `auth.ErrConflict`)
  - HTTP:
    - `POST /api/auth/register` `{"email","password"}` → 201 `{"user_id"}` + Set-Cookie
    - `POST /api/auth/login` → 200 `{"user_id"}` + Set-Cookie
    - `POST /api/auth/logout` → 204 + clear cookie
    - `GET /api/me` → 200 `{"user_id","email"}` or 401
  - Cookie: name `stylelab_session`, Path `/`, HttpOnly, SameSite=Lax, MaxAge 14 days. Secure only if request TLS.

- [ ] **Step 1: Password + service tests (table-driven)**

Cover: hash/check match; wrong password false; register then login returns token that UserIDFromToken accepts; bad email; short password; duplicate email; expired session rejected (insert session with expires_at in the past).

- [ ] **Step 2: Run tests — expect FAIL**

Run: `go test ./internal/auth/ -count=1`

- [ ] **Step 3: Implement auth package**

Session raw token: 32 random bytes hex (64 chars). Store `ids.New("ses_")` + token_hash + expires RFC3339 UTC.

- [ ] **Step 4: HTTP tests with `httptest`**

Use `STYLELAB_DEV_INSECURE_KEY=1`, temp data dir, `httpapi.New(store, cfg)`. Register → login → GET /api/me 200. GET /api/me without cookie 401. Logout then /api/me 401.

Error body must match `{"error":{"code":"...","message":"..."}}`.

- [ ] **Step 5: Wire main.go**

`store.Open`, `httpapi.New`, `http.ListenAndServe(cfg.Addr, handler)`.

- [ ] **Step 6: Run `go test ./... -count=1` — PASS**

- [ ] **Step 7: Commit**

```bash
git add internal/auth internal/httpapi cmd/stylelab/main.go
git commit -m "feat: email/password auth and session cookie"
```

---

### Task 4: BYOK LLM key encrypt/decrypt and routes

**Files:**
- Create: `internal/cryptokey/cryptokey.go`
- Create: `internal/cryptokey/cryptokey_test.go`
- Create: `internal/httpapi/keys.go`
- Modify: `internal/httpapi/server.go` to mount key routes
- Modify: `internal/httpapi/httpapi_test.go`

**Interfaces:**
- Consumes: `config.Config.MasterKey`
- Produces:
  - `cryptokey.Seal(master []byte, plaintext string) ([]byte, error)`
  - `cryptokey.Open(master []byte, blob []byte) (string, error)`
  - AES-256-GCM, nonce prepended to ciphertext
  - `PUT /api/me/llm-keys` auth required, body `{"provider":"openai|anthropic|compatible","base_url":"","api_key":"..."}` → 200 `{"provider","last4"}`
  - `GET /api/me/llm-keys` → 200 `{"keys":[{"provider","base_url","last4"}]}` — never `api_key`
  - `DELETE /api/me/llm-keys/{provider}` → 204
  - provider not in the three values → 400 `invalid`
  - empty api_key → 400

- [ ] **Step 1: Seal/Open roundtrip test + tamper test**

- [ ] **Step 2: Expect FAIL, then implement cryptokey**

- [ ] **Step 3: HTTP tests: PUT then GET must not contain the raw key string; last4 equals last 4 runes of the key**

- [ ] **Step 4: `go test ./internal/cryptokey/ ./internal/httpapi/ -count=1` PASS**

- [ ] **Step 5: Commit**

```bash
git add internal/cryptokey internal/httpapi
git commit -m "feat: encrypted BYOK LLM keys"
```

---

### Task 5: Projects CRUD

**Files:**
- Create: `internal/httpapi/projects.go`
- Modify: `internal/httpapi/server.go`
- Modify: `internal/httpapi/httpapi_test.go`

**Interfaces:**
- Produces:
  - `POST /api/projects` `{"name"}` → 201 `{"id","name","created_at"}`. name 1–80 runes.
  - `GET /api/projects` → 200 `{"projects":[...]}` only caller's rows
  - `GET /api/projects/{id}` → 200 or 404. other user's id → 404 (do not leak)
  - `DELETE /api/projects/{id}` → 204, cascade via FK

- [ ] **Step 1: Write HTTP tests for the four routes including cross-user 404**

- [ ] **Step 2: Implement handlers + `go test ./internal/httpapi/ -count=1` PASS**

- [ ] **Step 3: Commit**

```bash
git add internal/httpapi
git commit -m "feat: project CRUD"
```

---

### Task 6: Asset upload, chapter split, blob store

**Files:**
- Create: `internal/httpapi/assets.go`
- Create: `internal/extract/split.go`
- Create: `internal/extract/split_test.go`
- Modify: `internal/httpapi/httpapi_test.go`

**Interfaces:**
- Produces:
  - `extract.SplitChapters(text string) []string`
    - Split on lines matching `^#{0,2}\s*第[零〇一二三四五六七八九十百千万0-9]+章` OR on `\n\n\n+` if fewer than 2 chapter headings.
    - Drop empty chapters. If still 1 chunk, return `[]string{text}`.
  - `POST /api/projects/{id}/assets` `multipart/form-data` field `file`. Accept `.txt` or `.md`. Max 2 MiB. Decode as UTF-8. Reject empty. Compute sha256 of bytes, write `{dataDir}/blobs/{hex}` if missing. Insert `assets` row. Response 201 `{"id","filename","sha256","rune_count","chapter_count"}`.
  - `GET /api/projects/{id}/assets` → list without file bodies.
  - Sum of `rune_count` for a project used later; upload itself does not yet enforce 100_000 — extract job will.

- [ ] **Step 1: SplitChapters tests**

Input with two `第一章` / `第二章` headings → 2 chapters. Input with no headings and one paragraph → 1 chapter. Input with `\n\n\n` separators → multiple.

- [ ] **Step 2: Implement split + upload + list. HTTP test uploads a small txt.**

- [ ] **Step 3: Commit**

```bash
git add internal/extract internal/httpapi
git commit -m "feat: asset upload and chapter split"
```

---

### Task 7: Style card types and validation

**Files:**
- Create: `internal/card/card.go`
- Create: `internal/card/validate.go`
- Create: `internal/card/validate_test.go`

**Interfaces:**
- Produces:

```go
package card

var DimensionKeys = []string{
	"sentence_rhythm", "narrative_perspective", "dialogue_density",
	"sensory_description", "scene_pacing", "emotional_expression",
	"rhetoric_preference", "lexical_texture", "tension_hook",
}

type Dimension struct {
	Level      int      `json:"level"`
	Summary    string   `json:"summary"`
	Techniques []string `json:"techniques"`
}

type ParentRef struct {
	CardID  string         `json:"card_id"`
	Version int            `json:"version"`
	Dims    []string       `json:"dims"`
	Weights map[string]int `json:"weights"`
}

type Lineage struct {
	ParentCards   []ParentRef `json:"parent_cards"`
	PromptVersion string      `json:"prompt_version"`
}

type Card struct {
	ID            string                `json:"id"`
	ProjectID     string                `json:"project_id"`
	Name          string                `json:"name"`
	Kind          string                `json:"kind"`
	Version       int                   `json:"version"`
	Dimensions    map[string]Dimension  `json:"dimensions"`
	Prohibitions  []string              `json:"prohibitions"`
	Facts         json.RawMessage       `json:"facts"`
	Lineage       *Lineage              `json:"lineage"`
}

func Validate(c Card) error // returns *Error with Code
```

`Validate` rules:
- kind in extracted|fused|manual
- all 9 keys present
- each level 0–100
- summary rune count ≤ 100
- techniques length 3–5, each ≤ 40 runes
- techniques/summary/name must not match `(?i)(模仿|复刻|还原).{0,6}作者` or `仿写`
- fused requires non-nil lineage with 2–4 parents and `prompt_version == "fuse-v1"`
- extracted/manual must have nil lineage
- facts must be valid JSON object (default `{}`)

- [ ] **Step 1: Tests for valid fixture + each rejection rule**

Provide `card.ValidFixture(kind string) Card` in the test file (or as `testdata` helper in card package `func TestCard() Card` used only by tests).

- [ ] **Step 2: Implement Validate**

- [ ] **Step 3: Commit**

```bash
git add internal/card
git commit -m "feat: style card schema validation"
```

---

### Task 8: Port stylestat as a pure function

**Files:**
- Create: `internal/stylestat/stylestat.go`
- Create: `internal/stylestat/stylestat_test.go`

**Interfaces:**
- Produces the same public shapes as ainovel-cli `internal/stylestat` for v1:

```go
type Input struct {
	Chapters  []string
	Titles    []string
	Stopwords []string
}

type Stats struct {
	Chapters          int            `json:"chapters"`
	SampleTooSmall    bool           `json:"sample_too_small,omitempty"`
	Patterns          []PatternStat  `json:"patterns,omitempty"`
	TopPhrases        []PhraseStat   `json:"top_phrases,omitempty"`
	RepeatedSentences []SentenceStat `json:"repeated_sentences,omitempty"`
	Ending            EndingStat     `json:"ending"`
	OpeningTimeRate   float64        `json:"opening_time_rate"`
}

func Compute(in Input) *Stats
```

Port pattern regexes verbatim from https://raw.githubusercontent.com/voocel/ainovel-cli/main/internal/stylestat/stylestat.go (the eight `patternDefs`). If `len(Chapters) < 5`, return `&Stats{Chapters: n, SampleTooSmall: true}` — this is the one intentional difference from upstream (`nil`), because extract still needs a facts object.

- [ ] **Step 1: Write tests**

- 4 chapters → SampleTooSmall true, Chapters 4
- 5 chapters each containing `不是快乐，而是悲伤` → Patterns contains name `矫正句『不是…(而)是…』` with Total ≥ 5
- chapter ending of 10 runes vs 80 runes: Ending.ShortRatio behaves (short = ≤30 runes)

- [ ] **Step 2: Port implementation (copy logic, keep package `stylestat` under `stylelab`)**

Do not import ainovel-cli.

- [ ] **Step 3: Commit**

```bash
git add internal/stylestat
git commit -m "feat: port stylestat deterministic metrics"
```

---

### Task 9: In-process job runner

**Files:**
- Create: `internal/job/job.go`
- Create: `internal/job/runner.go`
- Create: `internal/job/runner_test.go`
- Create: `internal/httpapi/jobs.go`
- Modify: `internal/httpapi/server.go`, `cmd/stylelab/main.go`

**Interfaces:**

```go
package job

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
	ID, UserID, ProjectID string
	Kind Kind
	Status Status
	Progress int
	Stage string
	Payload json.RawMessage
	Result json.RawMessage
	Error string
}

type Handler func(ctx context.Context, rec Record, prog func(progress int, stage string)) (result json.RawMessage, err error)

type Runner struct{}

func NewRunner(st *store.Store, concurrency int) *Runner
func (r *Runner) Register(k Kind, h Handler)
func (r *Runner) Start(ctx context.Context)
func (r *Runner) Enqueue(ctx context.Context, rec Record) (id string, err error) // fills ID if empty, status queued
func (r *Runner) Cancel(ctx context.Context, id, userID string) error
func (r *Runner) Get(ctx context.Context, id, userID string) (Record, error)
func (r *Runner) RecoverInterrupted(ctx context.Context) (int, error)
```

HTTP:
- `GET /api/jobs/{id}` → record without leaking other users (404)
- `POST /api/jobs/{id}/cancel` → 204
- `GET /api/jobs/{id}/events` SSE: send `event: progress` with `{"progress","stage","status"}` every update and a final event. If SSE is too large for this task, polling-only is acceptable **only if** GET returns fresh progress; add SSE in this same task if under ~80 extra lines.

Behavior:
- Start N workers. Claim with `UPDATE jobs SET status='running', started_at=? WHERE id=(SELECT id FROM jobs WHERE status='queued' ORDER BY created_at LIMIT 1)` inside a transaction.
- Handler ctx canceled on Cancel or parent shutdown.
- Handler error → status failed, error=err.Error()
- RecoverInterrupted: `UPDATE jobs SET status='failed', error='interrupted' WHERE status='running'`

- [ ] **Step 1: Runner tests with a fake handler**

- enqueue two jobs, Start, wait until both succeeded
- cancel a job whose handler blocks on ctx.Done() → canceled or failed after ctx; assert not left running
- insert a running row, RecoverInterrupted → failed/interrupted

- [ ] **Step 2: Implement runner + HTTP get/cancel**

- [ ] **Step 3: main.go calls RecoverInterrupted then Start**

- [ ] **Step 4: Commit**

```bash
git add internal/job internal/httpapi cmd/stylelab/main.go
git commit -m "feat: in-process job runner and job API"
```

---

### Task 10: LLM client

**Files:**
- Create: `internal/llm/client.go`
- Create: `internal/llm/client_test.go`
- Create: `internal/llm/prompts.go`

**Interfaces:**

```go
package llm

type Message struct{ Role, Content string }

type Request struct {
	Provider string // openai | anthropic | compatible
	BaseURL  string
	APIKey   string
	Model    string
	Temp     float64
	Messages []Message
}

type Client struct {
	HTTP *http.Client // 120s timeout
}

func (c *Client) Chat(ctx context.Context, req Request) (string, error)
```

- openai/compatible: POST `{BaseURL or https://api.openai.com}/v1/chat/completions` body `model,temperature,messages`. Read `choices[0].message.content`.
- anthropic: POST `https://api.anthropic.com/v1/messages` with `x-api-key` and `anthropic-version: 2023-06-01`. System message pulled out to `system`. Read `content[0].text`.
- Retry twice on 429 and 5xx with 500ms then 1500ms sleep. No retry on 4xx other than 429.
- `prompts.go` exports:
  - `prompts.ForbiddenCopyCheck(s string) bool` true if s contains any banned phrase
  - `prompts.ExtractSystem()` string — instructs model to output only JSON of 9 dimensions + prohibitions; explicitly: do not name authors; do not imitate a person; describe techniques only
  - `prompts.FuseSystem()`, `prompts.AuditSystem(persona string)`, `prompts.SampleSystem(cardJSON string)`
  - persona only `commercial_web` or `literary_texture`

Banned phrases used in ForbiddenCopyCheck: `模仿作者`, `复刻作者`, `还原作者`, `像某某写的`, `仿写某某`, `仿写`.

- [ ] **Step 1: httptest server tests for openai success, 500 then success (retry), 400 no retry**

- [ ] **Step 2: ForbiddenCopyCheck unit tests; ExtractSystem must not contain banned phrases**

- [ ] **Step 3: Implement**

- [ ] **Step 4: Commit**

```bash
git add internal/llm
git commit -m "feat: BYOK chat client and style prompts"
```

---

### Task 11: Extract pipeline + API

**Files:**
- Create: `internal/extract/extract.go`
- Create: `internal/extract/extract_test.go`
- Modify: `internal/httpapi/assets.go` or new `internal/httpapi/extract.go`
- Modify: `cmd/stylelab/main.go` to `runner.Register(job.KindExtract, ...)`

**Interfaces:**

```go
type ExtractInput struct {
	ProjectID string   `json:"project_id"`
	AssetIDs  []string `json:"asset_ids"`
	Name      string   `json:"name"`
	Model     string   `json:"model"`
}

func Run(ctx context.Context, st *store.Store, llm *llm.Client, userID string, in ExtractInput, prog func(int,string)) (card.Card, error)
```

Steps inside Run:
1. Load assets; must belong to project and user. Concat text from blobs. If total runes > 100000 → error `invalid: sample too large`.
2. SplitChapters. Compute stylestat. Facts = json of Stats.
3. Build sample for LLM: first 400 runes of each chapter, stop at 6000 runes.
4. Load user's openai-or-first-available key via Open(master). If none → error `invalid: missing llm key`.
5. Chat with ExtractSystem + user content (facts summary + samples). Temp 0.3. Parse JSON into dimensions + prohibitions. Retry parse up to 2 extra Chat calls.
6. Append stylestat pattern names into prohibitions (prefix `避免过度使用：`).
7. Validate card. Insert style_cards + style_card_versions version=1 kind=extracted.
8. Return card.

HTTP: `POST /api/projects/{id}/extract` body ExtractInput minus ProjectID → 202 `{"job_id"}`. Job payload is ExtractInput. Handler calls Run, result_json `{"card_id":"..."}`.

- [ ] **Step 1: Test Run with a fake HTTP LLM returning a valid 9-dim JSON; assert card persisted and facts.sample_too_small for <5 chapters**

- [ ] **Step 2: Test missing key and oversize text**

- [ ] **Step 3: Implement + register + HTTP 202 test**

- [ ] **Step 4: Commit**

```bash
git add internal/extract internal/httpapi cmd/stylelab
git commit -m "feat: style extraction job"
```

---

### Task 12: Fuse math, fuse pipeline, card read API

**Files:**
- Create: `internal/card/fuse_math.go`
- Create: `internal/card/fuse_math_test.go`
- Create: `internal/fuse/fuse.go`
- Create: `internal/fuse/fuse_test.go`
- Create: `internal/httpapi/cards.go`
- Modify: server + main register KindFuse

**Interfaces:**

```go
type FuseSpec struct {
	Name    string      `json:"name"`
	Parents []card.ParentRef `json:"parents"`
	Model   string      `json:"model"`
}

func Blend(parents []card.Card, spec []card.ParentRef) (map[string]card.Dimension, []string, error)
```

Blend rules (verbatim from spec):
- 2–4 parents
- For each selected dim on a parent, weight 0–100; for each dim key, weights of parents that selected it must sum to 100, else error
- `new_level = int(math.Round(sum(level*weight/100.0)))`
- techniques: append in parent order by that dim's weight descending, unique, cap 5; if fewer than 3, pad by copying remaining from highest-weight parent until 3
- prohibitions: union, unique, stable order
- dims not selected by anyone: copy from the parent with the highest *total* weight across its selected dims; tie → earlier parent
- result must have all 9 keys

`fuse.Run` calls Blend, then LLM FuseSystem to rewrite summaries/techniques (not levels), collects `conflicts []string` into job result only. Lineage PromptVersion `fuse-v1`. Persist kind=fused.

HTTP:
- `GET /api/projects/{id}/cards` list `{id,name,kind,current_version,updated_at}`
- `GET /api/cards/{id}` full current version
- `GET /api/cards/{id}/versions/{n}` that version
- `POST /api/projects/{id}/fuse` → 202 job

- [ ] **Step 1: fuse_math tests with two cards**

Card A sentence_rhythm.level=80, Card B=20, weights 60/40 → level 56. Unselected dim copies from A if A listed first with higher total weight.

- [ ] **Step 2: fuse.Run test with fake LLM**

- [ ] **Step 3: Implement HTTP list/get**

- [ ] **Step 4: Commit**

```bash
git add internal/card internal/fuse internal/httpapi cmd/stylelab
git commit -m "feat: weighted style fusion and card reads"
```

---

### Task 13: Manual edit versions and export

**Files:**
- Create: `internal/card/export.go`
- Create: `internal/card/export_test.go`
- Modify: `internal/httpapi/cards.go`

**Interfaces:**
- `POST /api/cards/{id}/versions` body `{"name?":"...","levels":{"sentence_rhythm":70,...},"rewrite_summaries":false,"model":""}`  
  - copies current dimensions, applies any provided levels, kind stays the same (if fused, lineage copied; kind remains fused)  
  - if rewrite_summaries true, one LLM call to refresh summary/techniques for changed dims only  
  - increments current_version, inserts new style_card_versions row  
  - 201 full card
- `GET /api/cards/{id}/export` → `application/json` attachment `simulation_profile.json`  
  `export.ToSimulationProfile(c card.Card) []byte` mapping from spec §3.3. version field `simulation_profile.v1`. No author names. corpus.sources empty array.

- [ ] **Step 1: export fixture test checks mapped keys exist and do_not_copy == prohibitions**

- [ ] **Step 2: version POST test: level change persists as version 2, GET version 1 unchanged**

- [ ] **Step 3: Commit**

```bash
git add internal/card internal/httpapi
git commit -m "feat: card versioning and simulation_profile export"
```

---

### Task 14: Dual-persona audit

**Files:**
- Create: `internal/audit/audit.go`
- Create: `internal/audit/audit_test.go`
- Modify: httpapi + main

**Interfaces:**

```go
type Report struct {
	CardID     string                    `json:"card_id"`
	CardVersion int                      `json:"card_version"`
	Personas   []string                  `json:"personas"`
	ByPersona  map[string]PersonaView    `json:"by_persona"`
	Conflicts  []Conflict                `json:"conflicts"`
	RecommendedEdits []Edit              `json:"recommended_edits"`
}
type PersonaView struct {
	Strengths    []string `json:"strengths"`
	Risks        []string `json:"risks"`
	Suggestions  []string `json:"suggestions"`
}
type Conflict struct {
	Dimension string `json:"dimension"`
	Summary   string `json:"summary"`
}
type Edit struct {
	Dimension   string `json:"dimension"`
	TargetLevel int    `json:"target_level"`
	Reason      string `json:"reason"`
}

func Run(ctx, st, llm, userID, cardID, model string, prog) (Report, error)
```

Personas always both `commercial_web` and `literary_texture`. Two Chat calls in parallel (`errgroup`). A third Chat may synthesize conflicts+recommended_edits from the two views; or parse them if each persona returns a `conflicts` field — pick one approach and test it. Persist `audit_reports`. Job result `{"audit_id"}`.

HTTP:
- `POST /api/cards/{id}/audit` `{"model":""}` → 202 job
- `GET /api/audits/{id}` report JSON

- [ ] **Step 1: Fake LLM returns two persona JSON blobs; assert report saved and both keys present**

- [ ] **Step 2: Implement + commit**

```bash
git add internal/audit internal/httpapi cmd/stylelab
git commit -m "feat: dual-persona style audit"
```

---

### Task 15: Sample chapter job

**Files:**
- Create: `internal/sample/sample.go`
- Create: `internal/sample/sample_test.go`
- Modify: httpapi + main

**Interfaces:**

```go
type Input struct {
	CardID  string `json:"card_id"`
	Premise string `json:"premise"`
	Target  int    `json:"target_runes"` // default 1200, min 800, max 2000
	Model   string `json:"model"`
}
```

Reject premise empty or >80 runes. SampleSystem includes card JSON. After body returned, `stylestat.Compute` on `[]string{body}`. Persist `samples`. Job result `{"sample_id"}`.

HTTP:
- `POST /api/cards/{id}/sample` → 202
- `GET /api/samples/{id}` `{premise,body,facts,card_id,card_version}`

- [ ] **Step 1: Tests for premise length, default target, persist**

- [ ] **Step 2: Implement + commit**

```bash
git add internal/sample internal/httpapi cmd/stylelab
git commit -m "feat: short sample chapter job"
```

---

### Task 16: React SPA — shell, auth, projects, assets

**Files:**
- Create the `web/` tree listed in the file map for: package.json, vite.config.ts, tsconfig.json, index.html, main.tsx, App.tsx, api.ts, types.ts, Login.tsx, Projects.tsx, ProjectHome.tsx, Settings.tsx, JobProgress.tsx

**Interfaces:**
- Vite dev proxy `/api` → `http://127.0.0.1:8080`
- `api.ts` uses `fetch` with `credentials: 'include'`
- Routes: `/login`, `/register`, `/`, `/p/:id`, `/p/:id/lab/:cardId`, `/p/:id/fuse`, `/p/:id/audit/:auditId`, `/p/:id/sample/:sampleId`, `/settings`
- Unauthenticated `/api/me` failure redirects to `/login` except on login/register
- Projects page: list + create
- Project home: upload txt/md, list assets, button “抽离风格” (starts extract job, shows JobProgress polling GET /api/jobs/:id every 1s, on success navigate to lab)
- Settings: PUT/GET/DELETE llm keys; input type password; show last4 only after save

Banned UI strings: do not use 模仿作者 / 复刻 / 还原作者. Upload helper text: `请确保你有权使用该文本。系统只抽取抽象技法，不会把大段原文写入风格卡片。`

- [ ] **Step 1: `npm create` / write files; `npm install`; `npm run build` succeeds**

- [ ] **Step 2: Commit**

```bash
git add web
git commit -m "feat: web shell, auth, projects, and uploads"
```

---

### Task 17: Lab, fuse workbench, audit and sample pages

**Files:**
- Create: `web/src/pages/Lab.tsx`
- Create: `web/src/pages/Fuse.tsx`
- Create: `web/src/pages/Audit.tsx`
- Create: `web/src/pages/Sample.tsx`
- Create: `web/src/components/DimensionSliders.tsx`

**Interfaces:**
- Lab: load card; nine sliders bound to levels; radar can be a simple CSS/SVG nonagon using the nine levels (no extra chart library required). Version select. Save calls POST `/versions` with rewrite_summaries unchecked by default. Export downloads `/export`. Buttons 审计 / 试写.
- Fuse: pick 2–4 cards from project; per card, 9 checkboxes + weight number input. Client-side check: for each dim, selected weights sum to 100 before submit. POST fuse, poll job, open new card in Lab.
- Audit page: two columns for personas, list conflicts and recommended_edits. No author-likeness language.
- Sample page: premise input (80), target slider 800–2000, show body + facts JSON.

- [ ] **Step 1: Typecheck `npx tsc --noEmit` and `npm run build`**

- [ ] **Step 2: Commit**

```bash
git add web
git commit -m "feat: lab, fusion workbench, audit and sample UI"
```

---

### Task 18: Embed SPA, Docker, README

**Files:**
- Modify: `cmd/stylelab/main.go` to `//go:embed all:dist` from `web/dist` **or** embed via `web/embed.go` in package web that copies dist. Preferred:

```go
// web/embed.go
package web
import "embed"
//go:embed all:dist
var Dist embed.FS
```

If `web/dist` is empty in git, commit a `.gitkeep` is not enough for embed. Plan: build in Docker; for local, README says `cd web && npm run build` then `go run ./cmd/stylelab`. For tests that import embed, generate a minimal `web/dist/index.html` in this task so `go test ./...` still works without npm.

- Create: `Dockerfile` multi-stage: node build web → golang build cgo-free binary (`CGO_ENABLED=0`) → distroless or alpine, expose 8080, volume `/data`
- Create: `docker-compose.yml` service `stylelab` env `STYLELAB_DATA_DIR=/data` `STYLELAB_MASTER_KEY` from env, port 8080:8080
- Modify: `README.md` with: product boundary (no author imitation), BYOK, how to run locally, how to set master key (`openssl rand -hex 32`), extract/fuse/audit/sample flow, export to ainovel-cli

SPA fallback: any non-`/api` GET returns `index.html`.

- [ ] **Step 1: `go test ./...` still passes with embedded stub dist**

- [ ] **Step 2: Documented docker-compose config is valid YAML**

- [ ] **Step 3: Commit**

```bash
git add cmd/stylelab web/embed.go web/dist/index.html Dockerfile docker-compose.yml README.md
git commit -m "feat: embed SPA and containerize stylelab"
```

---

## Self-review

**Spec coverage**
- Account / project / upload / card library / lab / fuse / audit / sample / BYOK / job progress / export → Tasks 3–18.
- Nine dimensions + prohibitions + lineage + facts → Tasks 7, 11–13.
- stylestat port + extract facts → Tasks 8, 11.
- Dual personas commercial_web + literary_texture → Task 14.
- No author-imitation copy → Global Constraints + Tasks 10, 16, 17.
- Independent service, no ainovel-cli process → Global Constraints.
- SQLite, in-process jobs, no Redis/MinIO/OAuth/market → Tasks 2, 9, 18.

**Placeholders:** none intended. No TBD/TODO steps.

**Type consistency:** `card.Card`, `card.Dimension`, `card.Lineage`, `job.Kind`/`Status`, ID prefixes, job kinds, persona IDs, `fuse-v1`, cookie name `stylelab_session` are reused as written.

**Scope:** one MVP plan. Frontend is two tasks after the API exists so each still produces a buildable increment.
