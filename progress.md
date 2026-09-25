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
- Confirmed green on GitHub Actions (run 35628489422, 1m23s total): Go job 79s (gofmt / vet / `go test` 37s), Web job 22s with all seven steps passing.
- The `web/dist` freshness step passing on the Linux runner is the meaningful result: it rebuilt the SPA and got byte-identical output to the copy committed from a Windows worktree, so the check is reliable across platforms rather than merely reliable locally.

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

## 2026-09-22 - Task: Repair the stylesheet, and move the studio features onto the job system

### What was done
Two things, in the scope the user picked: UI tier 1+2, and the long-running LLM calls converted to jobs.

**UI.** The lint added last round found 58 class names used in `web/src` with no rule in `styles.css`. The cause was not sloppy authoring: `8920c32` rewrote the component tree without the stylesheet following it, leaving 67 orphaned rules under old names and 58 unstyled names under new ones — two vocabularies for the same UI. They line up almost one to one, so most of the fix was widening a selector (`.dock-track` → also `.job-track`, `.fuse-mixer-card` → also `.fuse-control-panel`/`.fuse-slot-panel`, `.fuse-ignite-bar` → also `.fuse-command-bar`, `.crumb span.sep` → `.crumb-sep`, `.toast-x` → also `.dock-x`) rather than writing new CSS.

What was actually broken: every job progress bar was a zero-height invisible div; the card deck drawer rendered as a block at the foot of the page with a zero-size scrim; `.sr-only` was undefined so six labels showed as text and pushed the search icon out of line; skeletons were blank holes; spinners painted nothing; `.icon-btn` had no hit area in 8 files; the Fuse two-column layout collapsed with no panel chrome.

That work also exposed a containing-block bug: `.page` used `animation: pageRise ... both`, and `fill-mode: both` leaves the final keyframe's transform applied forever. Even an identity transform makes the element the containing block for `position: fixed` descendants, so every overlay inside a page — including the studio modals repaired two rounds ago — was confined to the content area. The final keyframe equals the element's natural state, so the fill-mode bought nothing. Measured: `.modal-backdrop` went from 1086×465 at (309,35) to 1440×1000 at (0,0).

**Functionality.** Outline generation, the continuity radar, the branch simulator and chapter continuation were synchronous 90–120s HTTP requests. They are now jobs: new `internal/studio` package (one package, not three, because these three share a shape no other feature has — the job result *is* the data, not an id), prompts moved into `internal/llm/prompts.go`, four new job kinds registered in `main.go`, and the four handlers reduced to validate-then-enqueue returning 202. No new tables: `job.Record.Result` carries the payload. The frontend submits, registers the job with the dock, and renders `JobProgress` with `autoNavigate={false}`.

### Also fixed along the way
- **`POST /api/projects/{id}/cards` always returned 500** (committed separately). It inserted an `id` column into `style_card_versions`, which is keyed by `(card_id, version)` and has no such column. Manual card creation and the official-preset import had therefore never worked. Found by clicking 一键导入此卡 in a real browser and reading the toast: "internal error".
- The job dock only toasted on completion when the result mapped to a route. The studio kinds have no route, so a job finishing while the user was on another page disappeared from the dock silently. It now notifies without the 查看 action.

### Testing
- `gofmt -l internal cmd` clean, `go vet ./...`, `go test ./... -count=1` (17 packages) all pass.
- Studio tests rewritten from synchronous 200 to 202 + poll, keeping the same regression properties (non-empty model, bounded context, cross-user 404 without reaching the LLM, 400 with no key) and adding: the job result carries its data, a non-JSON reply fails the job with a message, and a job can be cancelled.
- New tests for `POST /api/projects/{id}/cards`, verified to fail with "status = 500, want 201" against the old insert.
- `npm run lint:css` passes; baseline 58 → 13.
- **Driven in a real browser (Playwright + the pre-installed Chromium), not just asserted in tests**: `.sr-only` spans measure 1×1; the preset modal backdrop is 1440×1000 at (0,0); the deck drawer is fixed, 460 wide, full height, with a full-viewport scrim; `.fuse-layout` is a two-column grid with both children on the same row; `.contribution-row` is a 4-column grid; the progress bar renders 4px tall and animates with real job progress ("generate · 35%"); a refresh mid-job leaves the dock holding the task; navigating away mid-job still produces the completion toast.
- Not verified: behaviour against a real provider (the LLM is stubbed through the BYOK `base_url` throughout), and mobile widths were not re-checked after the Fuse layout change beyond the CSS media queries.

### Notes
- `internal/studio/{studio,outline,continuity,branch}.go` - new domain package.
- `internal/llm/prompts.go` - `OutlineSystem`, `ContinuityAuditSystem`, `BranchSimulateSystem`, `ChapterContinueSystem`.
- `internal/job/job.go`, `cmd/stylelab/main.go` - four new kinds, registered.
- `internal/httpapi/{outline,continuity,branch}.go` - reduced to validate + enqueue; `helpers.go` gained `enqueueStudioJob`/`trimInvalid`.
- `internal/httpapi/cards.go` - the 500 fix; `cards_create_test.go` new.
- `web/src/styles.css` - the reconciliation layer; `web/src/{api,jobs,types}.ts`, `components/{JobProgress,JobDock,OutlinePlannerModal,ContinuityRadarModal,BranchSimulationDrawer}.tsx`, `pages/Chapter.tsx`.
- `README.md` - steps 13–15 are now jobs, and the response is 202 not 200.
- Rollback: the studio conversion and the CSS work are separate commits and revert independently.

### Known gaps
- `loadUserKey` is now duplicated in seven packages. Worth consolidating, but it touches every job package.
- Outline/audit/branch results are still not persisted, so re-opening the radar costs another model call. Storing them is the natural next step now that they are jobs.
- `.dock-card.failed`, `.preset-tag`, `.muted-row`, `.ok`/`.warn` are dynamic or compound names the lint cannot see statically; they remain unchecked.

## 2026-09-23 - Task: Close out the three open items

### What was done
The three gaps left at the end of the previous entry, each its own commit.

**One BYOK key loader instead of eight.** sample, write, extract, fuse, audit, bible, studio and httpapi each had a `loadUserKey`; diffed, all identical (bible only renamed the type). The duplication had become a correctness risk: the HTTP layer calls its copy as a precheck before enqueueing and the job calls its own when it runs, so the two must pick the same key. `internal/llmkey` is now the only implementation, logic verbatim. `bible.Key` is an alias so its API and tests are untouched; httpapi keeps `loadUserLLMKey` as a one-line wrapper. No existing test changed; new tests pin the selection rule.

**The CSS lint reads className expressions.** It now brace-matches `className={...}` and collects its string literals after dropping comparison operands (`mode === 'custom' ? 'on' : ''` yields only `on`). That found eight undefined classes, four of them signals the user could not see: no drop target when dragging a card onto a fusion slot (`.drop-over` — the earlier progress entry said this was added; it wasn't), no colour on the NN/100 "can I submit" total (`.ok`/`.warn`), no red on the over-limit rune counter (`.danger-text`), and failed dock jobs looking like running ones (`.dock-card.failed`). Plus one the lint structurally cannot catch: the active blend recipe uses `.btn.secondary.on`, and `.on` counts as defined because `.dim-pill.on` exists — nothing styled it on a button. The lint header now states that limit.

**Studio results are saved and reused.** Re-opening the continuity radar used to start a new paid scan every time. The newest successful result of each studio kind is now served from the `jobs` table (which already stores `result_json` and is never pruned) via `GET .../studio/latest`. The radar shows the last report with "扫描于 X" and only scans when there has never been one; the outline planner offers the last outline behind a 载入 button rather than overwriting the form; the branch drawer shows the chapter's last simulation. "Newest" is by `rowid`, because the RFC3339Nano timestamps trim trailing zeros and do not sort correctly as strings.

### Also fixed
- The branch drawer stays mounted while moving between chapters, so its results were keyed to nothing; it now resets on chapter change.
- `studio_test.go`'s header comment still described the pre-job design.

### Testing
- `go test ./... -count=1` (18 packages incl. new `llmkey`), `go vet`, `gofmt` clean; `npm run lint:css`, `npm run build`, `web/dist` clean.
- New tests: saved result round-trips; a newer run replaces it; a failed run does not; reading it does not call the model; branch results are per chapter (which also proves `json_extract` works in the bundled SQLite 3.49.1); kind validation and cross-user 404.
- Lint probed both ways: a new dynamic class fails the check; a comparison operand is not reported.
- Browser (Playwright, counting stub LLM): radar first open scans (1 call); reopen and full reload show the saved report with the stub still at 1 call; outline banner appears after reload and 载入 restores it; branch results stay per chapter across both full reloads and in-app 下一章 navigation; all five new state styles checked against real renders, the drop target via a dispatched `dragover`.

### Known gaps
- Saved results live and die with the `jobs` table. Nothing prunes it today; if something ever does, it must keep the newest succeeded job per kind.
- Queued jobs are claimed with `ORDER BY created_at` over the same RFC3339Nano strings, so two jobs queued in the same second can run out of order. Out of scope here; raised as a separate task.

## 2026-09-23 - Task: "Is it complete enough?" — close the remaining gaps

### What was done
Answered "not yet" with a list, then closed the four items in it.

**Relationship-graph extraction is a job with a bounded prompt.** It was the last model call still made inside an HTTP request, and it had every problem the other four studio features had before they were converted: the browser's 30 s timeout, and a prompt with no ceiling — every chapter with 1,500 runes of body, so a 60-chapter book sent ~90,000 runes. It now lives in `internal/studio/graph.go` as job kind `graph_extract`: first 30 chapters × 600 runes plus 40 bible entries, with a note to the model when it saw part of the book (measured: 18,884 runes on 60 chapters). Saving is one transaction with every error checked (it used to write row by row ignoring errors), edges are de-duplicated against the existing graph (a re-run used to double every relation), and a field the model returns empty no longer wipes what the author typed. The page registers the job with the dock, shows progress, and re-attaches to a running extraction after a refresh so the graph still reloads when it lands.

**Tests for the 9 routes that had none** — the whole graph API, `POST /api/chapters/{id}/write`, `manuscript.md` and the job SSE stream. 56 routes, 0 untested (counted by matching route patterns against test URLs; the same script reports the original 9 with the new files excluded). As with every earlier batch of untested routes, they had bugs:
- Updating a graph node or edge by an id that is not in the project answered 200 and changed nothing; now 404.
- Editing a node's profile reset its position: the drawer sends no `x`/`y`, and the update wrote 0. Omitted coordinates now keep their stored value.
- Deleting a node discarded the error from deleting its edges and was not atomic; it is one transaction now.
- `json.Marshal(nil map)` is `"null"`, not nil, so a node saved without details stored `null`; now `{}`.
- Graph GET did not check `rows.Err()`.

**Edge endpoints are validated.** Both ends must be nodes of this project and different from each other; the table has no foreign key, so an edge could point at nothing or at another project's node id.

**The job queue runs jobs in the order they were queued** (`ORDER BY rowid`, not the RFC3339Nano `created_at` strings, which trim trailing zeros and mis-sort). Reproduced with a test first.

### Also fixed
- **Every toast in the app appeared twice.** `ToastHost` was mounted both in `main.tsx` and inside `App`. Found by counting `.toast` elements in the browser run.
- `internal/job` tests failed about 1 run in 200 with "TempDir RemoveAll: directory not empty": cancelling the runner did not wait for its workers, so a worker could still be writing a job's final status while the store closed. `Runner.Wait()` added; the job tests and the studio test server stop the runner and wait before the store closes. 500 consecutive runs pass.

### Testing
- `gofmt`, `go vet ./...`, `go test ./... -count=1` clean; `npm run lint:css`, `npm run build`, `web/dist` committed.
- Browser (Playwright, stub LLM that logs each prompt, 60 chapters × 2,000 runes): extract answers 202; the progress bar moves; after a mid-job reload both the page and the dock hold the job; the three nodes render without a reload; one toast from the dock and one with the counts ("读取了前 30 / 60 章"); a second run updates 3 nodes and adds 0 edges.

### Known gaps
- The frontend still has no automated tests; the browser checks above are hand-run scripts that CI does not repeat.
- Nothing has been run against a real model provider; the LLM is stubbed through the BYOK `base_url` everywhere.
- Rewriting a card's dimension summaries is still an inline model call (short prompt, only the changed dimensions).
- A completed studio job shows two toasts when its page is open — the dock's generic one and the page's detailed one. This predates this round and applies to all studio features.

## 2026-09-23 - Task: Lock the UI fixes into CI

### What was done
Answered "is it complete?" a second time: the backend is (every route tested, every long model call a job, CI green); what was still missing was a guard for the UI. Every UI fix so far was verified only by hand-run Playwright scripts that CI never repeated.

**Browser smoke tests in CI.** `web/e2e/` with `@playwright/test`: the real Go server (embedding the built `web/dist`) on a throwaway data dir, plus a stub model provider that the test users' BYOK keys point at, so the real job handlers run end to end. Five tests, each pinning a bug that shipped once: graph extraction runs as a job with a visible progress bar, survives a reload and is announced exactly once; a job whose page was left is announced by the dock; studio modals cover the whole viewport and sit on top; the card deck is a fixed full-height drawer whose scrim closes it; on a phone the opened sidebar is above the page. New CI job `e2e`, uploading the report and traces on failure.

Each test was checked against the bug it guards by putting the bug back: `ToastHost` mounted twice, `.page` animation `fill-mode: both`, the page not claiming its job, and `.table`'s `z-index` each make the intended tests fail and nothing else.

**One toast per finished job.** A studio job finishing with its page open produced two toasts: the page's, with the result, and the dock's generic "X 完成". The page's `JobProgress` now claims a job it shows the result of; the dock waits one poll past the end and stays quiet for a claimed job. With the page closed nobody claims it and the dock still announces it.

### Also fixed
- **Every modal and drawer painted under the sidebar.** `.table` had `z-index: 2`, making it a stacking context: overlays inside a page (z-index 95–100) were confined to it, below the sidebar (40), which stayed lit and clickable while a modal was open. Earlier checks measured the overlays' size, not what a click would hit; the new tests hit-test. The mobile sidebar was checked not to fall under the page as a result.
- The first test runs measured layout mid-animation; the tests now wait for finite animations to end (spinners excluded).
- Counting toasts on screen missed duplicates that had already dismissed themselves; the tests record every toast as it appears.

### Testing
- `npx playwright test --repeat-each=3`: 15/15. Go and lint/build checks unchanged and clean.

### Known gaps
- Still nothing run against a real model provider.
- The smoke suite covers the fixed bugs, not every page.

## 2026-09-25 - Task: Story structure and graph algorithms, phase 1 — importance ranking

Plan (six phases, each committed and pushed on its own): importance ranking, community detection, lineage tree, text co-occurrence, scene segmentation, narrative tree (volume → chapter → scene).

### What was done
- New package `internal/insight`: pure, deterministic graph algorithms with no database or model access. `Graph` merges parallel edges, drops self-loops and sorts node ids so no result depends on input order.
- Weighted **PageRank** (undirected, damping 0.85, dangling mass spread evenly) and **Brandes betweenness** (edge length = 1/weight, Dijkstra, ties split between equal shortest paths). Roles: core (top 10% by PageRank), hub (top 10% by betweenness outside the core), peripheral (one tie), isolated.
- New package `internal/lore` loads a project's graph and runs the analyses; `GET /api/projects/{id}/graph/analysis` answers synchronously.
- Graph page: insight panel with the top-ten ranking (click to open the profile) and the hubs; node size follows the score (toggle).

### Found while testing
- Nodes tied at the core cutoff were split by id: in two mirrored triangles only one of the two symmetric leads was "core". Nodes tied with the last one admitted are now admitted too.

### Testing
- Unit tests check PageRank and betweenness against hand-worked values (a path; a square where shortest paths split), a star, the bridge between two triangles, weight sensitivity, merging, empty and single-node graphs, and 20 shuffles of input order.
- Route tests: a star ranks its centre first as core, its leaves peripheral, a lone node isolated; an empty project returns `[]`; another user gets 404.
- e2e: the ranking lists the centre first with score 100; its node is drawn 1.3× and returns to 1× when the toggle is off; clicking the row opens the profile.

## 2026-09-25 - Phase 2 — community detection

### What was done
- `insight.Louvain`: greedy modularity optimisation (Blondel et al.), local moves then aggregation until no merge helps. Nodes are visited in id order and ties go to the lowest community, so a graph always yields the same partition. `insight.Modularity` computes Newman's Q.
- `lore` names each group of two or more by the faction at least half its labelled members share (a faction node counts for its own name) and lists members labelled otherwise as outliers.
- Graph page: the insight panel lists the groups with the modularity and a verdict, and flags the outliers ("标注为「魔门」，却与青云宗往来密切"); a toolbar toggle colours nodes by detected group instead of labelled faction.

### Testing
- Unit: two K4s joined by a bridge split exactly, with Q equal to the hand-worked 2·(12/26 − (13/26)²); a ring of six K4s splits into the six; edge weights decide a path's split; isolated nodes stay alone; 30 input shuffles give the same partition; naming, the half-rule and outliers.
- Route: two camps with a spy labelled 魔门 among 青云宗 → two groups named 青云宗 and 魔门, the spy the only outlier, Q > 0.3.
- e2e: the panel shows both groups and the spy; toggling colours gives the spy 林远's colour instead of 魔尊's.

## 2026-09-25 - Phase 3 — lineage tree

### What was done
- Direction convention for hierarchical relations: the source is the superior (师徒, 师父, 父子, 君臣, 主仆, 掌门, …); words naming the junior side (徒弟, 弟子, 子女, 隶属, 成员, …) read the other way. The graph-extraction prompt now states it, and the profile drawer gains ⇄ to swap an edge's direction.
- `insight.BuildLineage`: characters and factions only. Tree parent = superior of the same faction, then strongest tie, then oldest; other superiors kept as `extra_parents`. Anyone without a superior hangs from their faction's node, a virtual root when the faction has no node, or 未归属. A faction's own faction field nests it under another. Loops of superiors are detected (three-colour walk), one link cut, and the node falls back to its faction if that closes no new loop; faction labels never close a loop.
- `insight` layout: Buchheim–Walker tidy tree (linear-time Walker), forest laid side by side.
- `GET /api/projects/{id}/graph/lineage`; graph page gains a 关系网 / 谱系树 switch with an SVG tree (elbow links, relation labels, dashed extra masters, cycle warnings; click a node for its profile).

### Found while testing
- A master–disciple loop inside one sect dropped the cut node into 未归属 instead of its sect; it now falls back to its faction.

### Testing
- Unit: relation vocabulary both ways; a sect with a master chain, a virtual faction and an unaffiliated character; parent preference order and extra parents; a three-node loop reported and cut; two factions naming each other; layout — three leaves under a parent at 0/1/2 with the parent at 1, two leaves between two wide subtrees spread evenly (checked to fail with the shift-spreading disabled), 50 random trees keep order, centring and one-unit spacing, and forests do not overlap.
- Route: 青云宗 → 赵长老 → 林远; swapping the edge puts 林远 on top; another user gets 404.
- e2e: the master is drawn above the disciple with the 师徒 label, 魔门 appears as a virtual root, and ⇄ in the drawer flips them.

## 2026-09-25 - Phase 4 — text co-occurrence

### What was done
- `insight.Matcher`: Aho–Corasick over runes, leftmost-longest and non-overlapping (林远山 is not also 林远), names under two characters dropped, a name shared by two entities kept by the first.
- `insight.Cooccur`: per entity, mentions per chapter, units mentioning it, first and last chapter; per pair, shared units with Jaccard and PMI (pairs sharing one unit are noise and dropped); entities mentioned at least five times but absent for N chapters.
- Aliases: `details.aliases` (list, or a string split on the usual separators), editable in the profile drawer; the extraction prompt asks for them.
- `lore`: written chapters split into paragraphs; suggestions = pairs sharing three or more paragraphs with no relation drawn. `GET .../graph/cooccurrence?absent_after=`; `GET .../graph/analysis?weights=graph|text|both` feeds the prose ties (normalised so the strongest weighs as much as one drawn relation) into PageRank, betweenness and Louvain.
- Graph page: 出场时间线 tab — heat map (sticky names and chapter numbers), 久未出场, 潜在关系 with a one-click 建立关系; the insight panel gains a 图谱关系 / 正文共现 / 两者 switch.

### Found while testing
- **Any wide content widened the whole app.** The app grid's column was `1fr`, whose minimum is the content's min-content width, so a 60-chapter heat map pushed the page 354px past the viewport and the header off screen. Now `minmax(0, 1fr)`; an e2e test fails with the old value.
- Suggested pairs were shown in id order ("苏晚 × 林远"); the more-mentioned one now comes first, and is the source of the drawn relation.

### Testing
- Unit: the matcher against the classic he/she/hers case, rune offsets, and brute force over a mixed text; co-occurrence counts, Jaccard and PMI by hand, the absence threshold both ways, empty input; alias parsing and paragraph splitting.
- Route: the report counts aliases, skips unwritten chapters, lists the one pair and the missing 魔尊, stops suggesting once the relation is drawn, and validates `absent_after`; text weighting finds edges where none are drawn; bad modes 400; another user 404.
- e2e: aliases count in the heat map; adding a nickname in the drawer raises the count and surfaces the suggestion; 建立关系 draws it; 正文共现 ranks 林远 first.

## 2026-09-25 - Phase 5 — scene segmentation

### What was done
- `insight.SegmentScenes`, after Hearst's TextTiling, for Chinese without a word segmenter: paragraphs; separator-only lines are hard cuts; at each gap, cosine similarity of Han-character-pair vectors over ~200 runes each side and the valley depth; +0.5 when the next paragraph opens with a time/place transition (次日, N 天后, 与此同时, 却说, …); +0.3 × the Jaccard distance of the cast either side; soft cuts from the highest score down, above mean + ½σ with a depth of at least 0.15 (or a transition), refusing any that leaves a scene under 300 runes. Each scene: rune offsets, cast by mentions, main location, cue, a title from its first sentence, a summary from its opening.
- New table `chapter_scenes` (cascades with the chapter) storing the scenes with a hash of the body they were cut from, so a changed body reads as stale.
- `GET /api/chapters/{id}/scenes`, `POST .../scenes/split`, `PATCH /api/scenes/{id}` (title/summary; marks the scene edited, and re-splitting asks before overwriting), `POST /api/projects/{id}/scenes/split-all`. Co-occurrence now counts by scene for chapters with current scenes (`unit_kind` paragraph / scene / mixed) and falls back to paragraphs when the scenes are stale.
- Chapter workbench: 本章场景 panel under the editor — split, cards with cue, length, location and cast; clicking selects the scene's text in the editor (rune offsets converted to UTF-16); ✎ renames. The timeline offers 全书切分场景.

### Found while testing
- Renaming by double-click could never work: the first click focuses the editor, which scrolls the page, so the second click lands elsewhere. Renaming is an explicit ✎ button.

### Testing
- Unit: three topical sections split at the right paragraphs with transitions, and again with no cue words at all (vocabulary and cast alone — the similarity curve bottoms out at 0.02 and 0.01 exactly at the breaks); uniform text stays whole; separators always cut and never appear inside a scene, edge separators and a dash inside prose do not cut; the minimum length refuses a one-line tail; empty and single-paragraph bodies; the transition pattern both ways.
- Route: split → 3 scenes with the right cast, location and cue; renaming; a changed body reads stale and co-occurrence falls back to paragraphs; deleting the chapter deletes its scenes; an empty chapter is 400; split-all counts; another user gets 404 on all four routes.
- e2e: split in the workbench, the selection covers exactly scene 2's text, a rename survives a reload, the timeline counts by scene; split-all from the timeline.

## 2026-09-25 - Phase 6 — narrative structure tree

### What was done
- New table `volumes` (project, `start_seq`, title, brief; unique start). A volume holds the chapters from its start up to the next volume's, so adding chapters needs no volume update.
- `lore`: volume create / edit / delete with validation; `groupByVolume` (pure) and `LoadStructure` build volume → chapter → scene, with per-volume chapter, written and rune counts and per-chapter scene staleness; chapters before the first volume form an unnamed leading group.
- Outline import takes `volumes: [{title, brief, chapter_count}]` (counts must add up, checked before anything is written), creates them where their chapters land, and drops volumes that pointed at replaced or non-existent chapters. The outline planner now sends them instead of flattening.
- Write page: the chapter list is the tree — collapsible volume headers with range and totals, ✎ / delete, "从这一章开始新的一卷" on each row, and each chapter's scenes as chips.

### Found while testing
- **Deleting a chapter renumbers every later one, which would have left each later volume one chapter too late** (and the last one empty). `write.DeleteAndRenumber` now shifts volumes in the same transaction: a volume that held only the deleted chapter is removed, and later volumes move up via negative values so the unique start index never sees two volumes on one chapter mid-update (the test creates volumes last-first to exercise that). The test fails with the shift removed.

### Testing
- Unit: grouping with a leading group, a volume starting at a deleted chapter, an empty volume, totals; outline volume validation.
- Route: volumes shape the tree; duplicate start, blank title and start < 1 are 400; moving and deleting volumes; scenes hang under their chapter and go stale with the body; another user gets 404 on all four routes; outline import with volumes, a mismatched import writes nothing, a replacing import replaces the volumes; volumes follow chapter deletes.
- e2e: build two volumes from the rows, collapse one, see scenes under chapter 1, delete a chapter and see the second volume still start at 入城; import a generated outline and get its volume.
