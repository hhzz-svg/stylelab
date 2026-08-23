# Card Workbench UI Implementation Plan

## Global Constraints

- Preserve all existing backend APIs, database schema, authentication, job protocol, and user-owned working-tree changes.
- Cover the project library, extraction workbench, card library, fusion workbench, laboratory, and shared navigation. Writing and audit flows receive shared visual styling only.
- Use a mixed light/dark professional tool aesthetic. Cards have a stable 3:4 ratio and at most 8px corner radius. Remove decorative aurora blobs and heavy glass effects.
- Every workflow must be completable by click, keyboard, and touch. Desktop drag-and-drop is an optional equivalent, never the only path.
- Extraction and fusion success remain on the originating workbench, reveal the resulting card in place, refresh card data, and expose explicit actions to enter the laboratory or continue to fusion.
- Fusion accepts 2-4 cards. Every dimension has at least one source and integer weights totaling exactly 100. Presets are balanced, dominant-card 70%, and custom.
- Card lists load `CardSummary` only. Full card data is loaded and cached only when inspecting or selecting a card.
- Do not add rarity, particles, currencies, combat language, speculative backend behavior, task cancellation, or card deletion.

## Task 1: Stabilize Shared Shell and Card Library

- Finish the Lucide navigation, `/p/:id/cards` route, fixed-ratio card tile, searchable/filterable card library, lazy inspector, and reusable deck drawer.
- Fix current TypeScript errors without changing extraction or fusion behavior beyond the shared job-result interface.
- Verify with `npx tsc --noEmit`.

## Task 2: Complete Extraction Workbench

- Present source shelf, selected-material tray, 100,000-rune total, 2 MiB/file-type preflight, advanced model setting, card mold, inline task progress, and in-place result reveal.
- Refresh project cards on success and expose `Enter laboratory` and `Add to fusion` actions.
- Verify empty, selected, over-limit, running, failed, and succeeded states.

## Task 3: Complete Fusion Workbench

- Implement four fixed slots, card-library drawer, click and optional drag selection, balanced/dominant/custom recipes, per-dimension contribution bars, exact integer rebalancing, local level preview, and in-place result reveal.
- Keep selected card versions fixed and serialize the final draft to the existing `ParentRef[]` contract.
- Verify 2/3/4-card presets, fifth-card rejection, last-source protection, custom weight edits, and submission payloads.

## Task 4: Refine Laboratory and Visual System

- Make the card identity, radar, focused dimension, detail copy, constraints, versions, and sticky save actions scan as one card-editing workspace.
- Apply the mixed light/dark token system, stable control sizes, responsive drawers/slots, visible focus, reduced-motion behavior, and shared styling to adjacent pages without changing their workflows.
- Verify 1440x900, 1024x768, and 390x844 layouts with no overlap or horizontal overflow.

## Task 5: Verify, Document, and Package

- Run TypeScript, production build, Go tests, scoped diff checks, and Playwright desktop/mobile workflow checks.
- Rebuild `web/dist`, update usage documentation, and append the required task entry to root `progress.md` with testing evidence, changed-file notes, and a rollback strategy that preserves pre-existing dirty changes.
- Perform a whole-branch code review and resolve load-bearing findings before handoff.
