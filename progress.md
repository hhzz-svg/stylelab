## 2026-08-18 - Task: First-pass UI/workflow cleanup for the style lab
### What was done
Reworked the main product entry pages so the app reads more like a working studio than a stack of forms: the projects view now presents a clearer entry point, the project workspace shows explicit status and next actions, the lab highlights version/state/editing controls, and the fuse view surfaces readiness before submission.

### Testing
- `npx.cmd tsc --noEmit`
- `go test ./...`

### Notes
- `web/src/pages/Projects.tsx` — added a stronger landing/creation layout and clearer empty-state copy.
- `web/src/pages/ProjectHome.tsx` — added project status summary, clearer upload/extract guidance, and better card empty-state framing.
- `web/src/pages/Lab.tsx` — turned the card editor into a more explicit studio panel with state summary and separated sample workflow.
- `web/src/pages/Fuse.tsx` — added a clearer fusion readiness header and tightened the submission workspace.
- `web/src/styles.css` — added the shared layout classes needed by the new workspace structure.
- Rollback point: revert the five files above together to return the UI to the previous layout.

## 2026-08-19 - Task: Stabilize shared shell and card library
### What was done
Completed the shared card-library presentation layer: navigation now uses the common icon set throughout, cards retain a fixed 3:4 footprint, and the library, inspector, and reusable deck drawer have responsive layouts that preserve click, keyboard, and touch access. Resolved the project-home result-card nullability errors without changing extraction or fusion requests, responses, or result handling.

### Testing
- `npx.cmd tsc --noEmit` (passed)
- `git diff --check -- web/src/App.tsx web/src/pages/ProjectHome.tsx web/src/styles.css` (passed; Git only reported existing LF-to-CRLF conversion warnings)

### Notes
- `web/src/App.tsx` - replaced the mobile menu's hand-drawn SVG with the Lucide menu icon while retaining the shared shell and card-library route.
- `web/src/pages/ProjectHome.tsx` - narrowed the result-card render branch so TypeScript can safely use the returned card identifier.
- `web/src/styles.css` - added the scoped card-library, fixed-ratio tile, inspector, deck-drawer, and responsive layout rules.
- `progress.md` - appended this task record.
- `.superpowers/sdd/2026-08-19-card-workbench-ui-implementation/task-1-report.md` - recorded implementation, verification, review, and rollback context.
- Rollback point: remove this appended entry and revert only the Task 1 hunks in the three web source files; do not reset the worktree because it contains unrelated user changes.

## 2026-08-19 - Task: Complete extraction workbench
### What was done
Reviewed the extraction workbench against the full state matrix (empty, selected, over-limit, running, failed, succeeded) and confirmed the preflight limits match the backend (2 MiB per asset, 100,000 selected runes). Fixed two succeeded-state defects: the result reveal used to permanently replace the card-mold form with no way to start another extraction, and the revealed card tile was a button that did nothing. The result panel now offers `继续抽离下一张` to dismiss the reveal and restore the form, and the tile navigates to the laboratory.

### Testing
- `npx.cmd tsc --noEmit` (passed)
- Cross-checked `MAX_FILE_BYTES`/`MAX_SAMPLE_RUNES` against `internal/httpapi/assets.go` and `internal/extract/extract.go`

### Notes
- `web/src/pages/ProjectHome.tsx` - added result dismissal, navigable result tile, and `useNavigate`.
- `.superpowers/sdd/2026-08-19-card-workbench-ui-implementation/task-2-report.md` - implementation, verification, review, and rollback context.
- Rollback point: revert only the Task 2 hunks in `web/src/pages/ProjectHome.tsx`; live-browser checks are deferred to the Task 5 Playwright pass.

## 2026-08-19 - Task: Complete fusion workbench
### What was done
Closed the remaining fusion-bench gaps without changing the `ParentRef[]` contract. Selected cards now freeze their version at selection time. Balanced, dominant-card 70%, and custom presets are explicit; custom weight edits still rebalance the rest of a dimension to an exact integer 100. A local nine-dimension level preview is computed from cached parent versions with the same rounded weighted sum as `internal/card.Blend`. Desktop drag from the deck drawer into a slot is an optional equivalent of click-to-add.

### Testing
- `npx.cmd tsc --noEmit` (passed)
- Code-path review of 2/3/4-card presets, fifth-card rejection, last-source protection, custom rebalance, and submission payload shape

### Notes
- `web/src/pages/Fuse.tsx` - version freeze, presets, local preview, drop targets, last-source repair.
- `web/src/components/CardDeckDrawer.tsx` - optional drag payload in selection mode.
- `web/src/styles.css` - preview panel, preset chips, drop-over state.
- `.superpowers/sdd/2026-08-19-card-workbench-ui-implementation/task-3-report.md` - implementation, verification, review, and rollback context.
- Rollback point: revert only the Task 3 hunks in the three files above.

## 2026-08-19 - Task: Refine laboratory and visual system
### What was done
Turned the laboratory into one scannable card-editing workspace: identity, radar, focused dimension copy, versions, constraints, and a sticky save/export/audit/sample/write bar. Replaced decorative aurora blobs and heavy glass surfaces with opaque mixed-light/dark panels, a shared 2.5rem control height, a visible gold focus ring, and a global reduced-motion rule. Adjacent pages inherit the visual tokens only.

### Testing
- `npx.cmd tsc --noEmit` (passed)
- CSS review of 1440×900, 1024×768, and 390×844 stacking; live screenshots deferred to Task 5

### Notes
- `web/src/pages/Lab.tsx` - workspace structure and sticky actions.
- `web/src/styles.css` - token restyle, aurora/glass removal, laboratory layout, overflow-safe drawer.
- `.superpowers/sdd/2026-08-19-card-workbench-ui-implementation/task-4-report.md` - implementation, verification, review, and rollback context.
- Rollback point: revert only the Task 4 hunks in the two files above.

## 2026-08-19 - Task: Verify, document, and package card workbench UI
### What was done
Finished the remaining plan work and packaged it without committing the dirty worktree. TypeScript, the Vite production build, and `go test ./...` all passed. `web/dist` was rebuilt. README now describes the extract, library, laboratory, and fusion workbenches. A laboratory missing-card bug found during live checks (infinite skeleton) was fixed to a retryable error. Desktop and mobile empty-state checks were run against the Vite SPA because the process on `:8080` is an older embedded binary.

### Testing
- `npx.cmd tsc --noEmit` (passed)
- `npx.cmd vite build` (passed; `index-BhMUnfjC.js`, `index-qT16ll5S.css`)
- `go test ./...` (passed)
- `git diff --check` on scoped frontend files (passed; LF-to-CRLF warnings only)
- Browser empty-state checks at 1440×900 and 390×844 on `http://127.0.0.1:5173`

### Notes
- `web/src/pages/Lab.tsx` - missing-card error/retry.
- `web/dist/**` - rebuilt SPA.
- `README.md` - workbench usage.
- `.superpowers/sdd/2026-08-19-card-workbench-ui-implementation/task-5-report.md` - verification, review, and rollback context.
- Rollback point: revert the Task 5 hunks in Lab and README, then restore the previous `web/dist` assets if needed. Do not reset the worktree.

## 2026-08-19 - Task: Deploy card workbench to stylelab.aihzcc.top
### What was done
Rebuilt the Go binary with the new embedded SPA, replaced the stale local `stylelab.exe` on `:8080`, and published it through a Cloudflare Tunnel. `https://stylelab.aihzcc.top` now serves the new UI. Session cookies set `Secure` when Cloudflare forwards `X-Forwarded-Proto: https`. Master key and tunnel credentials stay in gitignored local files.

### Testing
- `go test ./internal/httpapi -run TestSessionCookie` (passed)
- `GET https://stylelab.aihzcc.top/api/health` → `{"ok":true}`
- Public HTML references `index-BhMUnfjC.js`
- HTTPS login sets `stylelab_session` with `HttpOnly; Secure; SameSite=Lax`
- Authenticated `GET /api/me` over the public host succeeds
- Tunnel readyConnections=2, status healthy

### Notes
- Live URL: `https://stylelab.aihzcc.top`
- Tunnel id: `e8e3752a-30c9-4d26-b011-c0397c0d84ee`
- DNS: `stylelab.aihzcc.top` CNAME → `<tunnel-id>.cfargotunnel.com` (proxied)
- Local processes: `stylelab.exe` on `:8080`, `cloudflared tunnel --config .cloudflared/config.yml run`

## 2026-08-19 - Task: Chapter workbench
### What was done
Turned the chapter page into a two-column editing workspace. The body stays in the main column. A side rail shows the bound style card (nine dimensions and constraints) and the previous chapter's summary plus last 300 runes. On narrow screens the rail becomes a drawer. Save, write/rewrite, previous/next chapter, and back-to-outline sit in a sticky bar. Write APIs and generation behavior are unchanged.

### Testing
- `npx.cmd tsc --noEmit` (passed)
- `go test ./internal/write ./internal/httpapi -count=1` (passed)
- `go test ./...` (passed)

### Notes
- `web/src/chapterContext.ts` - neighbor and tail helpers.
- `web/src/components/ChapterIntelPanel.tsx` - style-card and previous-chapter intel, aside or drawer.
- `web/src/pages/Chapter.tsx` - workspace load/orchestration and sticky actions.
- `web/src/styles.css` - chapter workspace, sticky bar, mobile drawer stacking.
- `docs/superpowers/specs/2026-08-19-chapter-workbench-design.md`
- `docs/superpowers/plans/2026-08-19-chapter-workbench-implementation.md`
- Rollback: revert the four web files above together. Do not reset the worktree.
- Rollback: stop the new processes, restore `stylelab.old.exe` as `stylelab.exe`, and delete the `stylelab` tunnel/DNS record if the hostname should go away.

## 2026-08-19 - Task: Design chapter workbench safety and UI refinement
### What was done
Captured the approved chapter-workbench refinement as a bounded frontend design. The specification prioritizes chapter identity safety and write-operation consistency, then defines a body-first responsive layout, an accessible intel dialog, localized failure handling, and a browser state matrix without changing write APIs or generation rules.

### Testing
- Completed the specification self-review for placeholders, internal consistency, scope, and ambiguity; clarified that leave confirmation must complete before route state is cleared.
- `git diff --no-index --check -- NUL docs/superpowers/specs/2026-08-19-chapter-workbench-safety-ui-refinement-design.md` (passed; existing LF-to-CRLF conversion warning only)
- Cross-checked the existing `JobProgress` `autoNavigate` contract and TypeScript DOM library support for the specified dialog behavior.
- Verified the design commit staged exactly one file before creating commit `0bf28ee`.

### Notes
- `docs/superpowers/specs/2026-08-19-chapter-workbench-safety-ui-refinement-design.md` - added the confirmed scope, state model, responsive layout, dialog behavior, error matrix, and verification criteria.
- `progress.md` - appended this design and verification record without rewriting prior entries.
- Rollback point: run `git revert 0bf28ee` to remove the committed specification, then remove only this appended progress block.

## 2026-08-19 - Task: Story bible (设定集)
### What was done
Added a project-level story bible so long-form generation keeps characters, settings, and threads consistent across chapters. Three entry kinds (character / setting / thread) live in a single `bible_entries` table (manual CRUD + GET/PATCH/DELETE). Entries are injected into the chapter generation user prompt (grouped, per-entry clamped, total-clamped, resolved threads marked). After each chapter is persisted, a non-fatal LLM pass incrementally maintains the bible via an `{"ops":[...]}` protocol (create dedupes to update by name+kind, AI never deletes, content clamped). For existing projects, a `bible_sync` job replays every written chapter in seq order to rebuild the bible. A new `/p/:id/bible` page manages entries (search/filter/sort + side editor), the sidebar links it, and the chapter workbench intel rail gains a 设定集 section.

### Testing
- `go test ./...` (passed)
- `npx.cmd tsc --noEmit` (passed)
- `cd web && npx.cmd vite build` (passed; `index-C7fCuh5c.js`, `index-COzTcOtj.css`)

### Notes
- `internal/store/migrate.go` - added `bible_entries` DDL.
- `internal/store/store_test.go` - added `bible_entries` (and missing `chapters`) to the table-name assertion.
- `internal/bible/bible.go` - Entry/EntrySummary/Op types, validation, ListByProject/ListSummaries/LoadOwned/Insert/UpdateFields/Delete, RenderForPrompt (grouped + clamped + resolved marker), SyncChapter + ParseOps + ApplyOps (transactional upsert with name-dedupe).
- `internal/bible/sync.go` - `bible_sync` job: RunSync loads written chapters in seq order, per-chapter SyncChapter + ApplyOps, progress per chapter, result `{chapters_synced, entries}`.
- `internal/llm/prompts.go` - added `BibleSyncSystem` (ops-only JSON, no deletes, content ≤150 字).
- `internal/write/write.go` - load bible after previous chapters, inject `bible.RenderForPrompt` block into `ChapterUserPrompt` (between 全书目录 and 前情摘要, empty block omitted); `ChapterUserPrompt` and `chatChapter` gained a `bibleBlock` param; after `persist` a non-fatal `bible_sync` step reuses the same LLM key/model.
- `internal/job/job.go` - added `KindBibleSync`.
- `internal/httpapi/bible.go` - GET/POST /api/projects/{id}/bible, GET/PATCH/DELETE /api/bible/{id}, POST /api/projects/{id}/bible/sync; ownership via requireOwnedProject + LoadOwned, cross-user 404.
- `internal/httpapi/server.go` - registered bible routes.
- `cmd/stylelab/main.go` - registered `KindBibleSync` handler.
- `web/src/types.ts` - `BibleEntry` / `BibleEntrySummary` / `BibleEntryKind` / `BibleEntryStatus`.
- `web/src/api.ts` - listBible / getBible / createBible / patchBible / deleteBible / syncBible.
- `web/src/pages/Bible.tsx` - new page: sync bar (model + 从已有章节同步), search/filter/sort, entry rows with kind badges + origin/seq tags, side editor (create/edit/delete), status chips.
- `web/src/App.tsx` - route `/p/:id/bible`, sidebar 设定集 link.
- `web/src/chapterContext.ts` - ChapterIntel gained `bible` / `bibleError`; `emptyIntel` updated.
- `web/src/pages/Chapter.tsx` - `loadBible` fire-and-forget (epoch-guarded like loadCard), reset bumps bibleRequestRef, both intel panel instances pass `onRetryBible`.
- `web/src/components/ChapterIntelPanel.tsx` - new 设定集 work-zone (zone-index 志) grouping entries by kind with resolved markers and a 去设定集管理 link.
- `web/src/styles.css` - bible page (sync bar, rows, kind badges, editor) + chapter intel bible list styles; 900px single-column rules.
- `README.md` - Workflow gained Story bible step.
- `internal/bible/bible_test.go` - validation, RenderForPrompt (grouping/clamp/resolved), ParseOps (dedupe/bad-id/bad-status), SyncChapter+ApplyOps (create/update/name-dedupe), RunSync backfill (two chapters, progressive state), RunSync no-written-chapters error.
- `internal/write/write_test.go` - prompt contains bible entries; write persists AI bible entries (origin=auto, source_seq); bible sync failure does not break the write.
- `internal/httpapi/httpapi_test.go` - TestBibleCRUDAndOwnership (create/list/get/patch/delete + cross-user 404), TestBibleSyncStartsJob (202 → succeeded, kind=bible_sync).
- Rollback point: revert the appended progress block and the changed/new files above; the `bible_entries` table is additive and existing tables are untouched, so no data migration is needed.

## 2026-08-20 - Task: Harden chapter workbench safety and UI
### What was done
Completed the chapter-detail workbench pass within the approved frontend scope. Chapter requests now keep their route identity through auxiliary loads and retries, story-bible loading has an explicit mutually-exclusive loading state, and post-save card refreshes retain the originating epoch. The chapter page uses a body-first command-bar layout with responsive intel behavior, while the intel panel uses a compact 3×3 dimension grid and a portal-backed native dialog with Esc, backdrop, and opener-focus restoration. Leave confirmation remains shared across chapter-owned navigation, with the approved close-before-confirm behavior for dialog links.

### Testing
- `cd web; npx.cmd tsc --noEmit` (passed)
- `cd web; npm.cmd run build` (passed; Vite production build)
- `go test ./internal/write ./internal/httpapi -count=1` (passed)
- Scoped `git diff --check` for the chapter source, component, styles, and chapter docs (passed; Git reported only existing LF-to-CRLF warnings)
- Playwright live checks at 1440×900, 1024×768, and 390×844 (passed for layout, responsive command-bar/intel behavior, dialog Esc/focus restoration, dirty-leave cancel, and missing-card focus)
- Verification gap: delayed A/B response injection, auxiliary-request failure injection, duplicate-request counting, and real job terminal/poll-error flows were not available in this live session and remain unclaimed.

### Notes
- `web/src/pages/Chapter.tsx` - added bible loading state, stale-request guards, and epoch-bound auxiliary refreshes; retains the existing write API and operation lifecycle.
- `web/src/components/ChapterIntelPanel.tsx` - made bible loading mutually exclusive and kept aside/dialog content shared through one render path.
- `web/src/components/JobProgress.tsx` - retains the optional polling-error/status compatibility extension from the prior pass.
- `web/src/styles.css` - added chapter-scoped command-bar, responsive, dimension-grid, and native-dialog rules without changing shared drawer/work-zone definitions.
- `docs/superpowers/specs/2026-08-19-chapter-workbench-safety-ui-refinement-design.md` - retains the approved dialog close-before-confirm boundary.
- `docs/superpowers/plans/2026-08-19-chapter-workbench-safety-ui-refinement-implementation.md` - appended the implementation status and verification-gap record.
- Rollback point: revert only the chapter hunks in the five files above plus the appended plan/progress blocks; do not reset the dirty worktree or remove unrelated build assets.

## 2026-09-21 - Task: Repair the studio feature batch

### What was done
Commit `9cf4fb7` shipped five features (Master Outline Planner, Continuity Radar, Branching Simulator, Multi-Format Exporter, Audio Narration) that compiled and left `go test ./...` green but could not actually be used. This pass fixed them without adding features and without converting the inline LLM calls to the job system.

The headline defect was that all four LLM endpoints passed `req.Model` to the provider with no default while no frontend caller sent one, so every call shipped `{"model":""}` and failed. The second was that `web/src/styles.css` was never touched by that commit, so `.modal-backdrop`, `.modal-panel`, `.modal-head`, `.btn.icon-only`, `@keyframes spin` and `@keyframes fadeInUp` had no definitions — with no `position: fixed` the two Write-page modals rendered as ordinary blocks at the foot of the page and every spinner sat frozen. The components also used `var(--border)` / `var(--text)` / `var(--muted)`, none of which exist in this design system.

Also fixed: `replace_existing` outline import ran an unguarded `DELETE FROM chapters` and destroyed finished manuscript bodies; export swallowed scan errors and never checked `rows.Err()`, so a mid-iteration failure returned a silently truncated novel under HTTP 200, and the project name went raw into `Content-Disposition`; the continuity and branch prompts were assembled from the entire manuscript with no cap; the client aborted at 30s against 90–120s server budgets, and the abort's `NetworkError` was swallowed by `instanceof APIError` checks.

### Testing
- `gofmt -l internal/ cmd/` (clean; two BOMs stripped first)
- `go vet ./...` (passed)
- `go test ./... -count=1` (passed, including the 12 new studio tests)
- Regression locks verified by temporarily reverting the fixes: the model and import tests fail with "model sent to provider was empty" and "written chapter is gone: 404", then pass again once restored
- `cd web && npm ci && npm run build` (`tsc --noEmit && vite build`, passed)
- Verified the new CSS is present in the built bundle, not just the source, since `web/dist` is what the binary embeds
- Not verified: live browser checks of the repaired modals, and no real-provider call was made — the tests stub the LLM through the BYOK `base_url`

### Notes
- `internal/httpapi/helpers.go` - new: `resolveModel`, `headRunes`, `tailRunes`, and an RFC 5987 `contentDispositionAttachment`.
- `internal/httpapi/outline.go` - model default, batch cap, pre-transaction validation (rejects the empty brief that would make a chapter permanently unwritable), `write.Prefix` / `write.ClampTarget`, card-ownership 404, and a replace that preserves written chapters and appends after them (forced by `UNIQUE (project_id, seq)`), reporting `protected_count`.
- `internal/httpapi/continuity.go`, `branch.go` - model default with a chapter-model fallback, bounded prompt context, `rows.Err()` checks.
- `internal/httpapi/graph.go` - same empty-model bug fixed; uses the server's shared `llm.Client`.
- `internal/httpapi/export.go` - scan/`rows.Err()` handling and a parseable `Content-Disposition`.
- `internal/httpapi/studio_test.go` - new: 12 tests over the six previously untested routes; stubs the LLM by pointing a stored BYOK `base_url` at an `httptest` server, which also makes the request body assertable.
- `web/src/styles.css` - shared modal layer, spinner and `fadeInUp` keyframes; also repairs `Cards.tsx` and `Graph.tsx`, which used the same undefined classes.
- `web/src/api.ts` - per-route LLM timeouts, `errMessage`, `Content-Disposition`-aware `downloadNovel`; removed the now-callerless `downloadManuscript` (the Go route remains).
- `web/src/components/*`, `web/src/pages/Write.tsx` - real design tokens, z-index folded onto the existing scale, StrictMode audit guard, category filter, `plot_points` rendering, voice picker, unsupported-TTS message, export-menu dismissal, modals unmount on close.
- `web/dist/**` - rebuilt.
- `README.md` - workflow steps 12–16, and a note that steps 13–15 are inline LLM calls rather than jobs.
- Rollback point: revert commits `aa0764e`, `43e4392`, `8dd4e84`, `f217264` and this block. All changes are additive to the schema-free surface; no migration is involved.

### Known gaps (deliberately out of scope this pass)
- The inline 90–120s LLM calls still bypass the job system, so they have no progress, no cancel, and no recovery across restart.
- `outline.go` / `continuity.go` / `branch.go` still hold business logic, raw SQL and ~100 lines of prompt text in the httpapi layer instead of a domain package with prompts in `internal/llm/prompts.go` — this is the root cause of the ID-prefix and validation drift fixed above.
- There is no CI. A `go test` + `gofmt` + `tsc` + `vite build` gate would have caught most of this batch, especially a `web/src` change shipped without rebuilding `web/dist`.

## 2026-09-21 - Task: CI gate

### What was done
Added `.github/workflows/ci.yml` so the failure modes from the previous two passes cannot reach the trunk again. No business code changed.

The point of this pass is that the last batch shipped with a fully green `go test ./...`: the empty-model bug, the undefined `.modal-backdrop` / `.spin` rules, the three non-existent design tokens, the UTF-8 BOMs, and a `web/dist` that could silently fall behind `web/src` were all invisible to the toolchain. CI now covers each of them.

Two jobs run in parallel, pinned to the same versions as `Dockerfile` (Go 1.22 / Node 20):
- **Go** — `gofmt -l internal cmd` (fails on any output), `go vet ./...`, `go test ./... -count=1`.
- **Web** — `npm ci`, `npm run lint:css`, `npm run build` (`tsc --noEmit && vite build`), then a `web/dist` freshness check.

`web/scripts/check-css.mjs` is a zero-dependency Node script covering the two CSS failures specifically: undefined `var(--x)` (zero baseline — there are none today, any new one fails) and `className` values with no rule in `styles.css` (58 pre-existing gaps recorded in `web/scripts/css-baseline.json`; only new ones fail, and the check also reports when a baseline entry has been fixed so the list keeps shrinking). It only sees static class names; dynamically built ones are out of reach and the script says so.

`.gitattributes` pins the working tree to LF. This is a prerequisite for the `web/dist` check rather than tidying: the repo is developed on Windows, and a CRLF worktree feeding the build while CI feeds LF could produce different bytes and a false failure. `git add --renormalize .` produced no churn, confirming the index was already all-LF.

### Correction to the previous entry
The `chore: strip UTF-8 BOMs` commit message claims "gofmt does not remove it". That is wrong: `gofmt -w` does strip a BOM, and `gofmt -l` flags a BOM'd file even when it is otherwise clean. The BOMs were real and are gone, but a separate BOM check would be redundant, so CI relies on the gofmt gate alone.

### Testing
- Ran every gate locally: gofmt clean, `go vet` clean, `go test ./... -count=1` passed, `npm run lint:css` passed, `npm run build` passed, `web/dist` clean.
- Confirmed the vite build is deterministic — a rebuild reproduced the identical content hashes (`index-B5A6nmLf.css`, `index-Ch7qEWDB.js`), so the dist check will not be flaky.
- **Proved each gate fails when it should**, then restored: bad indentation → gofmt step fails; `var(--border)` → CSS check fails naming the file; `className="brand-new-thing"` → CSS check fails and passes once baselined; a `styles.css` edit followed by a rebuild → dist check reports the changed bundle.
- Not verified: the workflow has not yet run on GitHub Actions; the YAML parses and every step was executed locally, but runner behaviour is unconfirmed until the first push.

### Notes
- `.github/workflows/ci.yml` - new.
- `.gitattributes` - new, `* text=auto eol=lf` plus binary asset exclusions.
- `web/scripts/check-css.mjs`, `web/scripts/css-baseline.json` - new.
- `web/package.json` - added `lint:css`.
- `README.md` - a CI section, and a much more prominent statement of the `web/dist` rebuild rule.
- Rollback point: delete the four new files and revert the `package.json` / `README.md` hunks. Nothing in `internal/` or `web/src` was touched.

### Finding: 58 undefined class names, two of them live defects
The new check surfaced a pre-existing problem larger than the one fixed last pass. These are recorded in the baseline, not fixed:
- **Every job progress bar is invisible.** `JobProgress.tsx:57-72` uses `.job-bar` / `.job-track` / `.job-fill`; all three appear **zero times** in `styles.css`. `.job-fill` receives only an inline `width: X%` — with no height or background it renders nothing. Affects extract, fuse, audit, sample, write and bible_sync.
- **`.sr-only` is undefined, so screen-reader-only labels render as visible text**: `Bible.tsx:264,269,278` and `Cards.tsx:222,227,234` (六处「搜索条目 / 条目类型 / 排序方式 / 搜索卡片 / 卡片类型」).
- Others include `.skeleton`, `.spinner`, `.toast-msg`, `.tooltip-bubble`, `.deck-drawer`, `.icon-btn`, `.notfound`, `.offline-banner`.
Recommended next pass: fix `.job-track` / `.job-fill` and `.sr-only` first, then work the baseline down.
