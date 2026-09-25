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
12. **Relationship graph (关系图谱)** — `/p/:id/graph`. Nodes and edges for characters, factions, and places, edited by hand or seeded from written chapters with `graph/extract`. Extraction reads the first 30 chapters (600 runes of each) plus up to 40 bible entries and tells the model when it saw only part of the book. Re-running it updates nodes by name, never duplicates a relation, and leaves a field the model returned empty as the author set it. The **insight panel** beside the graph ranks entities by weighted PageRank (who the important people are closely tied to) and flags **hubs** by Brandes betweenness centrality (the go-betweens that link separate circles); node size follows the score. It also splits the network into **communities** with Louvain modularity optimisation, names each by the faction most of its members carry, and points out members labelled with a different faction (a secret ally, a spy, or a stale label); the graph can be coloured by these detected groups. `GET /api/projects/{id}/graph/analysis`. The **谱系树** tab lays the characters out as a lineage forest: factions on top (a faction whose own faction field names another nests under it; a faction named only on characters becomes a dashed stand-in root), members beneath, masters above disciples and parents above children. Hierarchical relations (师徒, 父子, 君臣, 主仆, …) read the edge's source as the superior, and words naming the junior side (徒弟, 子女, 隶属) the other way; the profile drawer's ⇄ swaps an edge's direction. With several superiors the tree edge goes to the same-faction one, then the strongest, then the oldest, and the rest are drawn dashed; a loop of superiors is reported and cut. Positions come from the Buchheim–Walker tidy-tree algorithm. `GET /api/projects/{id}/graph/lineage`. The **出场时间线** tab scans the written chapters for every entity's name and aliases (an Aho–Corasick automaton, leftmost-longest, so 林远山 is not also counted as 林远) and shows a chapter-by-chapter heat map, who has been missing for ten or more chapters after being mentioned at least five times, and pairs that share three or more paragraphs with no relation drawn (one click draws it). Pairs are scored by count, Jaccard and PMI. The insight panel can base its ranking and communities on the drawn relations, on this prose co-occurrence, or on both. Aliases live in a node's `details.aliases` and can be edited in the profile drawer; AI extraction fills them too. `GET /api/projects/{id}/graph/cooccurrence?absent_after=10`, `GET .../graph/analysis?weights=graph|text|both`. The chapter workbench can **split a chapter into scenes** (本章场景 → 切分场景): a TextTiling-style segmentation adapted to Chinese — lexical cohesion of adjacent Han-character pairs on either side of each paragraph gap, deeper valleys scoring higher, plus weight for a time/place transition at the start of a paragraph (次日, 三天后, 与此同时, 却说 …) and for a change of cast; the author's separator lines (***, ◇◇◇, ———) always cut, and no soft cut leaves a scene under 300 characters. Each scene records its cast, main location and why it starts where it does; clicking one selects its text in the editor, and its title can be renamed. A scene set remembers the body it was cut from and reports itself stale when the text changes. Once a chapter has current scenes, co-occurrence counts by scene instead of by paragraph; the timeline can split the whole book at once. `GET/POST /api/chapters/{id}/scenes[/split]`, `PATCH /api/scenes/{id}`, `POST /api/projects/{id}/scenes/split-all`. The chapter list on `/p/:id/write` is the book's **structure tree**, volume → chapter → scene: a volume runs from the chapter it starts at up to the next volume (start one from any chapter row, edit or delete it from its header, collapse it), each chapter row shows its scenes, and importing a generated outline now keeps its volumes. Deleting a chapter renumbers the ones after it, and the volumes move with them. `GET /api/projects/{id}/structure`, `POST /api/projects/{id}/volumes`, `PATCH`/`DELETE /api/volumes/{id}`; `outline/import` accepts `volumes: [{title, brief, chapter_count}]`. The analyses live in `internal/insight`, are deterministic, call no model, and answer synchronously.
13. **Outline planner (智能大纲规划)** — on `/p/:id/write`. Generates a volume/chapter outline from a premise, then imports it as chapter briefs. Import appends after any chapter that already has prose: `替换现有章节目录` replaces only unwritten drafts and reports how many finished chapters it kept.
14. **Continuity radar (伏笔逻辑雷达)** — on `/p/:id/write`. Audits written chapters plus the story bible for power-scaling breaks, characterisation drift, and dangling foreshadowing. Issues carry a severity and a category, and both can be filtered. The report is not persisted, so each scan is a fresh pass.
15. **Branch simulator (灵感推演) and narration (沉浸朗读)** — on the chapter workbench. The simulator proposes plot branches from the current draft and can continue the chapter inline (the result lands in the editor unsaved, so save it deliberately). Narration reads the chapter aloud with the browser's own speech synthesis — no server, no TTS provider.
16. **Whole-novel export** — `导出全书` on `/p/:id/write` downloads the manuscript as TXT or Markdown, named after the project.

Graph extraction (step 12) and steps 13–15 are **jobs**, like everything
else long-running here. `POST` to those routes answers `202 {"job_id": ...}` and the client polls
`GET /api/jobs/{id}`; they show a progress bar, can be cancelled, survive a
refresh via the job dock, and are recovered on restart.

They differ from the other job kinds in one way worth knowing: their result
**is** the data the UI renders (the outline, the audit report, the branches),
carried in `job.result`, rather than an id pointing at a persisted row.
Graph extraction is the exception: it writes the graph itself, and its
result is a summary (`nodes_created`, `nodes_updated`, `edges_created`,
`chapters_read`, `chapters_total`); the page re-reads the graph when it lands.

The newest successful result is reused rather than regenerated:
`GET /api/projects/{id}/studio/latest?kind=outline_generate|continuity_audit`
and `GET /api/chapters/{id}/studio/latest?kind=branch_simulate` return it, or
`{"latest": null}` if there has been none. Re-opening the continuity radar
shows the last report with its timestamp instead of paying for a new scan
(重新扫描 still runs one); the outline planner offers the last outline; the
branch drawer shows the chapter's last simulation. These are read straight
from the `jobs` table, which is never pruned -- anything that ever prunes it
must keep the newest succeeded job per kind, or move these to their own table.

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
| `npm run test:e2e` | UI regressions unit tests cannot see: overlays clipped or under the sidebar, invisible progress bars, a job lost on reload, duplicate toasts |

Run the whole set locally before pushing:

```bash
gofmt -l internal cmd && go vet ./... && go test ./... -count=1
cd web && npm ci && npm run lint:css && npm run build
git status --porcelain -- web/dist    # must be empty
cd web && npm run test:e2e             # after npm run build
```

The browser smoke tests (`web/e2e/`) start the real Go server -- which embeds
`web/dist`, so build first -- and a stub model provider that users' BYOK keys
point at, then drive Chromium through them. Run `npx playwright install
chromium` once, or point `E2E_CHROMIUM` at a Chromium binary you already have.

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
