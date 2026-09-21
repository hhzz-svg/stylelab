# Style Lab

Single-binary Go + React app for extracting, fusing, auditing, and sampling **abstract novel style cards**.

Style Lab describes techniques (rhythm, perspective, dialogue density, sensory language, pacing, emotion, rhetoric, lexical texture, tension). It does **not** imitate, recreate, or restore a named author. Upload only text you have the right to use. Source excerpts are not stored on the card — only counts, short technique notes, and prohibitions.

## Product boundary

- Cards capture nine technique dimensions. They are not author profiles.
- Prompts and UI talk about methods, not “write like person X”.
- BYOK: your LLM API key is encrypted at rest with `STYLELAB_MASTER_KEY`. The API never returns the plaintext key — only `provider` and last 4 characters.
- v1 is a standalone service (SQLite, in-process jobs). It does not call ainovel-cli as a subprocess.

## Prerequisites

- Go 1.22+
- Node 20+ (frontend build)
- Docker (optional)

## Master key

Production and Docker **must** set a 32-byte key as 64 hex characters:

```bash
export STYLELAB_MASTER_KEY="$(openssl rand -hex 32)"
```

Local/test only: `STYLELAB_DEV_INSECURE_KEY=1` uses 32 zero bytes. Do not use that in production.

## Run locally

Build the SPA, then start the binary (it embeds `web/dist` and serves the UI plus `/api`):

```bash
cd web && npm install && npm run build && cd ..
export STYLELAB_MASTER_KEY="$(openssl rand -hex 32)"
go run ./cmd/stylelab
```

Open `http://localhost:8080`. Health check: `GET /api/health` → `{"ok":true}`.

Frontend-only iteration:

```bash
# terminal 1
STYLELAB_DEV_INSECURE_KEY=1 go run ./cmd/stylelab

# terminal 2
cd web && npm run dev
```

Vite proxies `/api` to `http://127.0.0.1:8080`. After UI changes that you want inside the Go binary, run `cd web && npm run build` again.

`web/dist` is **committed and embedded into the binary** (`//go:embed all:dist`).
Two consequences worth internalising:

- If `web/dist` is missing, `go test` / `go run` will fail to compile the embed.
- If you change anything under `web/src` and do not rebuild, the binary keeps
  serving the old UI — and the Go test suite stays green while doing it. Always
  finish a frontend change with `cd web && npm run build`, and commit the
  regenerated `web/dist`. CI fails the build if the two drift apart.

## Docker

```bash
export STYLELAB_MASTER_KEY="$(openssl rand -hex 32)"
docker compose up --build
```

The image builds the SPA, then a static Go binary (`CGO_ENABLED=0`), listens on `8080`, and stores SQLite + blobs in the `/data` volume.

```yaml
# docker-compose.yml (excerpt)
services:
  stylelab:
    environment:
      STYLELAB_DATA_DIR: /data
      STYLELAB_MASTER_KEY: ${STYLELAB_MASTER_KEY}
    ports:
      - "8080:8080"
```

## Config

| Env | Default | Notes |
|---|---|---|
| `STYLELAB_ADDR` | `:8080` | Listen address |
| `STYLELAB_DATA_DIR` | `./data` | SQLite (`stylelab.db`) and blobs |
| `STYLELAB_MASTER_KEY` | required | 64 hex chars (32 bytes) |
| `STYLELAB_DEV_INSECURE_KEY` | off | `1` uses 32 zero bytes (test-only) |
| `STYLELAB_DEV_AUTO_LOGIN` | off | `1` enables auto-login |
| `STYLELAB_WORKERS` | `2` | In-process job concurrency |

## Workflow

1. **Account** — register / login. Session cookie: `stylelab_session` (httpOnly, 14 days).
2. **Settings → BYOK** — save an OpenAI, Anthropic, or OpenAI-compatible key. Only last 4 chars are shown later.
3. **Project** — create a project, upload `.txt` / `.md` samples (max 2 MiB each).
4. **Extract (抽离)** — `/p/:id`. Source shelf, selected-material tray, 100,000-rune cap, and a card mold. Job kind `extract` stays on this page; the new card appears in place with laboratory and fusion actions.
5. **Card library (牌库)** — `/p/:id/cards`. Search and filter `CardSummary` rows. Full nine-dimension data loads only when a card is opened.
6. **Lab** — `/p/:id/lab/:cardId`. Identity, radar, focused dimension, versions, constraints, and a sticky save bar. Saving still creates a new card version. Export JSON from the same bar.
7. **Fuse** — `/p/:id/fuse`. Four slots, 2–4 parents, balanced / dominant-70% / custom recipes. Per-dimension integer weights must total 100. A local level preview uses the versions frozen at selection. Job kind `fuse` stays on this page (`kind=fused`, lineage `fuse-v1`).
8. **Audit** — dual personas `commercial_web` and `literary_texture`. Job kind `audit`. Review strengths, risks, conflicts, recommended edits.
9. **Sample (试写)** — short chapter from a premise (≤ 80 runes), target 800–2000 runes. Job kind `sample`.
10. **Write** — `/p/:id/write` lists chapter briefs. `/p/:id/chapter/:chapterId` is a chapter workbench: body in the main column, bound style card and previous-chapter tail in the intel rail, sticky save / write / prev / next. The write API is unchanged.
11. **Story bible (设定集)** — `/p/:id/bible`. Register characters, settings, and threads (手动 CRUD). Entries are injected into the chapter generation prompt; after each chapter is written, an LLM pass incrementally maintains the bible (新增/更新，AI 不删除条目，失败不致命). For existing projects, `从已有章节同步` starts a `bible_sync` job that replays every written chapter in seq order to rebuild the bible. The chapter workbench intel rail also shows the current bible grouped by kind.
12. **Relationship graph (关系图谱)** — `/p/:id/graph`. Nodes and edges for characters, factions, and places, edited by hand or seeded from written chapters with `graph/extract`.
13. **Outline planner (智能大纲规划)** — on `/p/:id/write`. Generates a volume/chapter outline from a premise, then imports it as chapter briefs. Import appends after any chapter that already has prose: `替换现有章节目录` replaces only unwritten drafts and reports how many finished chapters it kept.
14. **Continuity radar (伏笔逻辑雷达)** — on `/p/:id/write`. Audits written chapters plus the story bible for power-scaling breaks, characterisation drift, and dangling foreshadowing. Issues carry a severity and a category, and both can be filtered. The report is not persisted, so each scan is a fresh pass.
15. **Branch simulator (灵感推演) and narration (沉浸朗读)** — on the chapter workbench. The simulator proposes plot branches from the current draft and can continue the chapter inline (the result lands in the editor unsaved, so save it deliberately). Narration reads the chapter aloud with the browser's own speech synthesis — no server, no TTS provider.
16. **Whole-novel export** — `导出全书` on `/p/:id/write` downloads the manuscript as TXT or Markdown, named after the project.

Steps 13–15 call the model **inline** rather than through a job, so those
requests stay open for 90–120 seconds instead of returning a job id. They have
no progress bar and cannot be cancelled; everything else long-running is a job.

Jobs are in-process (`queued` → `running` → `succeeded` / `failed` / `canceled`). Default concurrency is 2. A process restart marks leftover `running` rows `failed` with `interrupted`.

## Export to ainovel-cli

In Lab, **Export** downloads `simulation_profile.json` (`version: simulation_profile.v1`).

That file is a technique profile for ainovel-cli (or any consumer of the same schema). It maps the nine dimensions into `synthesis.style` / `pacing_density` / `hook_design`, copies prohibitions to `do_not_copy`, and leaves `corpus.sources` empty. It does not include author names or long source excerpts.

Use the downloaded JSON as a simulation profile input in ainovel-cli. Style Lab never shells out to ainovel-cli.

## CI

`.github/workflows/ci.yml` runs on every push and pull request:

| Check | What it catches |
|---|---|
| `gofmt -l internal cmd` | Unformatted Go, including a stray UTF-8 BOM |
| `go vet ./...` | Suspicious constructs |
| `go test ./... -count=1` | Behaviour, including the studio-route regression locks |
| `npm run lint:css` | `var(--x)` and `className` values with no definition in `styles.css` |
| `npm run build` | Type errors (`tsc --noEmit`) and a broken production build |
| `web/dist` diff | A frontend change that was never rebuilt, which would ship a stale UI |

Run the whole set locally before pushing:

```bash
gofmt -l internal cmd && go vet ./... && go test ./... -count=1
cd web && npm ci && npm run lint:css && npm run build
git status --porcelain -- web/dist    # must be empty
```

`npm run lint:css` compares against `web/scripts/css-baseline.json`, which
records class names that are used but have no rule today. The list is a record
of existing gaps, not approved ones — it must only ever shrink. Adding a *new*
undefined class fails the build; regenerate the file with
`npm run lint:css -- --write-baseline` only when you have deliberately added an
unstyled class, and the check will also tell you when an entry can be removed.

Undefined CSS custom properties have no baseline: there are none today, and any
new one fails the build.

## Layout

```
cmd/stylelab     HTTP binary (embeds web/dist)
internal/        API, jobs, cards, BYOK, LLM
web/             Vite + React SPA
Dockerfile       multi-stage: node → go → alpine
docker-compose.yml
```
