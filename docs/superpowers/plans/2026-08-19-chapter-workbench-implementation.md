# Chapter Workbench Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Turn `/p/:id/chapter/:chapterId` into a scannable chapter workbench with a body column, style-card and previous-chapter intel, and a sticky action bar, without changing the write API.

**Architecture:** Keep `PATCH /api/chapters/:id` and `POST /api/chapters/:id/write` unchanged. Load the current chapter, project chapter list, card summaries, bound card detail, and previous-chapter tail on the client. Compose the page from small presentational pieces plus the existing save/write/job flow in `Chapter.tsx`.

**Tech Stack:** React 18, React Router 6, existing `api` client, Lucide icons, shared `web/src/styles.css` tokens from the card workbench.

## Global Constraints

- Do not add, remove, or change backend routes, job payloads, or the chapters schema.
- Do not implement continue-writing, span rewrite, streaming, cancellation, or chapter reordering.
- Do not rewrite `web/src/pages/Write.tsx` beyond shared visual inheritance.
- Preserve existing validation: title ≤ 40, brief ≤ 200, note ≤ 200, target 2000–3500 step 100, previous-chapter tail 300 runes.
- Cards stay 3:4 with at most 8px radius. No aurora, glass, rarity, particles, or combat language.
- Every control remains usable by click, keyboard, and touch. Desktop drawers are optional, never the only path.
- Do not commit unrelated dirty worktree files. Append `progress.md` only for this work.
- Verify with `npx.cmd tsc --noEmit`. There is no frontend unit-test runner.

---

### Task 1: Chapter context helpers and load orchestration

**Files:**
- Create: `web/src/chapterContext.ts`
- Modify: `web/src/pages/Chapter.tsx`

**Interfaces:**
- Consumes: `Chapter`, `ChapterSummary`, `StyleCard` from `web/src/types.ts`; `api.getChapter`, `api.listChapters`, `api.listCards`, `api.getCard` from `web/src/api.ts`
- Produces:
  - `PREV_TAIL_RUNES = 300`
  - `lastRunes(text: string, n: number): string`
  - `neighbors(list: ChapterSummary[], currentId: string): { prev: ChapterSummary | null; next: ChapterSummary | null }`
  - `ChapterIntel` type used by later tasks

- [ ] **Step 1: Add helpers**

Create `web/src/chapterContext.ts`:

```ts
import type { Chapter, ChapterSummary, StyleCard } from './types'

export const PREV_TAIL_RUNES = 300

export type ChapterIntel = {
  card: StyleCard | null
  cardError: string
  prev: ChapterSummary | null
  next: ChapterSummary | null
  prevSummary: string
  prevTail: string
  prevError: string
}

export function lastRunes(text: string, n: number): string {
  const runes = Array.from((text ?? '').trim())
  if (runes.length <= n) return runes.join('')
  return runes.slice(-n).join('')
}

export function neighbors(list: ChapterSummary[], currentId: string) {
  const index = list.findIndex((item) => item.id === currentId)
  return {
    prev: index > 0 ? list[index - 1] : null,
    next: index >= 0 && index < list.length - 1 ? list[index + 1] : null,
  }
}
```

- [ ] **Step 2: Load intel in Chapter.tsx**

Keep existing chapter/card-summary load. After the current chapter and chapter list resolve:

1. Compute `neighbors(list, chapter.id)`.
2. If `chapter.card_id`, call `api.getCard(chapter.card_id)` and store the `StyleCard`. On failure set `cardError`.
3. If `prev` exists, call `api.getChapter(prev.id)` and store `summary` plus `lastRunes(body, PREV_TAIL_RUNES)`. On failure set `prevError` and keep `prev.has_summary` as a hint only.
4. Cache previous-chapter payloads in a `Record<string, Chapter>` keyed by chapter id so adjacent navigation can reuse them.

Do not change save or write request bodies.

- [ ] **Step 3: Type-check**

Run: `cd web && npx.cmd tsc --noEmit`

Expected: pass.

- [ ] **Step 4: Record progress, do not commit the whole dirty tree**

Append a short Task 1 note to root `progress.md`. Do not `git add` unrelated files.

---

### Task 2: Intel panels and mobile drawer

**Files:**
- Create: `web/src/components/ChapterIntelPanel.tsx`
- Modify: `web/src/styles.css`

**Interfaces:**
- Consumes: `ChapterIntel`, `PREV_TAIL_RUNES` from Task 1; `DIMENSION_KEYS`, `DIMENSION_LABELS`, `KIND_LABEL` from `web/src/types.ts`; existing `.dimension-bars` and `.deck-drawer` styles
- Produces: `ChapterIntelPanel` with props:
  - `projectId: string`
  - `intel: ChapterIntel`
  - `loadingCard?: boolean`
  - `loadingPrev?: boolean`
  - `onRetryCard?: () => void`
  - `onRetryPrev?: () => void`
  - `variant: 'aside' | 'drawer'`
  - `open?: boolean`
  - `onClose?: () => void`

- [ ] **Step 1: Build the panel**

`ChapterIntelPanel` renders two stacked `work-zone` blocks:

1. Style card: name, `KIND_LABEL`, version, nine `dimension-bar` rows, prohibition list, link to `/p/${projectId}/lab/${card.id}`. Empty copy: “还没有绑定风格卡” plus link to `/p/${projectId}/cards`. Error copy plus 重试 button when `cardError` is set.
2. Previous chapter: if `intel.prev` is null, “这是开篇，没有上一章。” Otherwise show `第 ${seq} 章 ${title}`, summary or “还没有摘要”, and the 300-rune tail. Error copy plus 重试 when `prevError` is set.

When `variant === 'drawer'` and `open` is false, return null. When open, reuse `.deck-scrim` and `.deck-drawer` so the drawer is viewport-bound like the card deck.

- [ ] **Step 2: Add only the CSS the panel needs**

Append after the laboratory sticky-bar rules:

```css
.chapter-intel {
  display: grid;
  gap: 1rem;
  align-content: start;
}

.chapter-prev-tail {
  margin: 0;
  padding: 0.7rem 0.8rem;
  border: 1px solid var(--line);
  border-radius: 8px;
  background: var(--sea);
  color: var(--ink-soft);
  font-size: 0.82rem;
  line-height: 1.7;
  white-space: pre-wrap;
}
```

Do not restyle Write.tsx.

- [ ] **Step 3: Type-check**

Run: `cd web && npx.cmd tsc --noEmit`

Expected: pass.

---

### Task 3: Chapter workspace layout and sticky actions

**Files:**
- Modify: `web/src/pages/Chapter.tsx`
- Modify: `web/src/styles.css`

**Interfaces:**
- Consumes: Task 1 intel state; Task 2 `ChapterIntelPanel`; existing `confirm`, `useDirtyGuard`, `JobProgress`, `registerJob`
- Produces: desktop two-column chapter workspace and mobile body-first drawer

- [ ] **Step 1: Restructure Chapter.tsx markup**

Replace the single `.card.stack` form with:

```tsx
<form className="chapter-workspace" onSubmit={onSave}>
  <section className="chapter-main">
    <details className="chapter-setup work-zone">
      <summary>第 {ch.seq} 章 · {title || ch.title} · {bodyCount}/{target}</summary>
      {/* existing title, card, model, brief, target, note fields */}
    </details>
    <label className="chapter-body-field">
      正文（{bodyCount} 字）
      <textarea className="chapter-body" value={body} rows={18} readOnly={jobActive} onChange={(e) => setBody(e.target.value)} />
    </label>
    <label>
      摘要（留给下一章）
      <textarea value={summary} rows={3} onChange={(e) => setSummary(e.target.value)} />
    </label>
  </section>
  <ChapterIntelPanel projectId={projectId} intel={intel} variant="aside" />
  <div className="chapter-sticky-bar">
    <button type="button" className="btn secondary" onClick={() => setIntelOpen(true)}>风格卡 / 前情</button>
    <button className="btn" type="submit" disabled={busy || !dirty}>{busy ? '保存中…' : '保存'}</button>
    <button className="btn secondary" type="button" onClick={() => void onWrite()} disabled={jobActive}>
      {jobActive ? '生成中…' : body.trim() ? '重写这一章' : '写这一章'}
    </button>
    <button className="btn secondary" type="button" disabled={!intel.prev} onClick={() => void goTo(intel.prev?.id)}>上一章</button>
    <button className="btn secondary" type="button" disabled={!intel.next} onClick={() => void goTo(intel.next?.id)}>下一章</button>
    <Link className="btn secondary" to={`/p/${projectId}/write`}>回章程</Link>
  </div>
  <ChapterIntelPanel variant="drawer" open={intelOpen} onClose={() => setIntelOpen(false)} ... />
</form>
```

`goTo(id)` must confirm when `dirty` is true, using the existing `confirm()` helper, then `navigate(/p/${projectId}/chapter/${id})`.

Keep rewrite confirmation, dirty save-before-write, and job registration exactly as they are.

- [ ] **Step 2: Workspace CSS**

```css
.chapter-workspace {
  display: grid;
  grid-template-columns: minmax(0, 1.4fr) minmax(280px, 0.72fr);
  gap: 1rem;
  align-items: start;
  padding-bottom: 5.5rem;
}

.chapter-main { display: grid; gap: 1rem; min-width: 0; }
.chapter-sticky-bar { position: sticky; bottom: 0.7rem; z-index: 8; display: flex; flex-wrap: wrap; gap: 0.45rem; grid-column: 1 / -1; padding: 0.7rem 0.9rem; border: 1px solid var(--line); border-radius: var(--radius); background: var(--sea-2); }
.chapter-workspace > .chapter-intel { grid-column: 2; }

@media (max-width: 900px) {
  .chapter-workspace { grid-template-columns: 1fr; }
  .chapter-workspace > .chapter-intel { display: none; }
  .chapter-sticky-bar { position: sticky; bottom: 0; margin: 0 -1.1rem; border-radius: 0; }
}

@media (min-width: 901px) {
  .chapter-sticky-bar .intel-open { display: none; }
}
```

- [ ] **Step 3: State matrix review**

Walk these states in code, then `npx.cmd tsc --noEmit`:

- first chapter (prev disabled)
- middle chapter (both neighbors enabled)
- last chapter (next disabled)
- unbound card empty intel
- card load error retry
- previous-chapter load error retry
- dirty navigate confirm
- job running: body read-only, write disabled, save still available if dirty

Expected: compile clean; no new API fields.

---

### Task 4: Verify, restyle inheritance, and package

**Files:**
- Modify: `web/src/pages/Chapter.tsx` only if review finds a defect
- Modify: `progress.md`
- Modify: `README.md` workflow bullet for Write if the chapter route behavior needs one sentence

- [ ] **Step 1: Verify**

Run:

```text
cd web && npx.cmd tsc --noEmit
cd .. && go test ./internal/write ./internal/httpapi -count=1
```

Expected: both pass. Write package tests must stay green because this task does not change Go.

- [ ] **Step 2: Layout check**

If a frontend server is available, open a chapter at 1440×900 and 390×844. Confirm no horizontal overflow and that the mobile drawer does not use `100vw`. If no live chapter data exists, record a CSS review instead of inventing seed data.

- [ ] **Step 3: Append progress**

Add a 2026-08-19 chapter-workbench entry to root `progress.md` with testing evidence, changed files, and a rollback that does not reset the dirty worktree.

---

## Spec coverage

- Desktop two-column body + intel: Task 3
- Mobile body-first drawer: Tasks 2 and 3
- Style card + previous chapter intel: Tasks 1 and 2
- Sticky save / write / prev / next / back: Task 3
- Existing API only, 300-rune tail, dirty confirm: Tasks 1 and 3
- Error/retry matrix: Tasks 2 and 3
- Visual tokens and reduced-motion inheritance: Task 2 CSS plus existing global reduced-motion rule
- Out of scope items are omitted on purpose
