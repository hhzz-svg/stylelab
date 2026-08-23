# Story Bible（设定集）实施计划

## 背景与目标

当前章节生成的全部上下文只有「前 3 章摘要（每章 ≤120 字）+ 上一章文末 300 字」，长篇写到几十章后人物/设定/伏笔必然漂移。本计划新增项目级**设定集（story bible）**：

1. 三类条目：**人物（character）/ 设定（setting）/ 伏笔（thread）**，单表存储，手动 CRUD；
2. **写章时注入**：章节生成的 user prompt 中加入设定集块（全量注入，每条截断 + 总量上限）；
3. **写完自动更新**：write job 持久化章节后追加一步 LLM 增量维护（新增/更新条目，**AI 不删条目**；失败不致命，同 summary 先例）；
4. **回填**：新增 job kind `bible_sync`，一键按序遍历已有已写章节重建设定集（老项目立即可用）。

## 关键设计决策

- **表结构**（新表，不动旧表——现有迁移机制只支持加新表）：
```sql
CREATE TABLE IF NOT EXISTS bible_entries (
  id TEXT PRIMARY KEY,
  project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  kind TEXT NOT NULL,              -- character | setting | thread
  name TEXT NOT NULL,
  content TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'active',   -- thread 可标 resolved（已收线）
  origin TEXT NOT NULL DEFAULT 'manual',   -- manual | auto，UI 显示信任来源
  source_seq INTEGER NOT NULL DEFAULT 0,   -- 最近一次 AI 更新来自第几章
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
```
- **注入格式**（`ChapterUserPrompt` 内，位于「全书目录」之后、「前情摘要」之前；设定集为空则整块省略，存量测试不受影响）：按 人物/设定/伏笔 分组，每条 content 截 240 runes，整块上限 4000 runes，resolved 伏笔标注（已收线）。
- **AI 更新协议**：system prompt 要求只输出 `{"ops":[{"op":"create","kind":"","name":"","content":""},{"op":"update","id":"","content":"","status":""}]}`；`ApplyOps` 事务内执行，create 遇同名同 kind 自动降级为 update，未知 id 忽略，每次上限 30 条 ops，content 钳到 400 runes。temp 0.2 / maxTokens 2500，解析失败重试 3 次（仿 `chatExtract`）。
- **失败策略**：写章后的 bible 同步失败 → 静默跳过（章节照常 written）；回填 job 单章失败 → 整个 job fail（每章 ApplyOps 后即落库，取消可保留部分进度）。

## 实施任务（按序执行）

### Task 1 — 存储层
- `internal/store/migrate.go`：`const schema` 追加上述 DDL。
- `internal/store/store_test.go`：表名断言 want 列表加 `bible_entries`（顺手补上缺失的 `chapters`）。

### Task 2 — `internal/bible` 领域包（新文件 bible.go + bible_test.go）
- 常量：`Prefix = "bib_"`、kind/status/origin 常量；校验函数（name ≤40 runes 非空、content ≤2000、kind/status 枚举）。
- 类型：`Entry`（全字段）、`EntrySummary`（无 content）、`Op`。
- 存取（包级函数，仿 `internal/write`）：`ListByProject` / `LoadOwned`（JOIN projects）/ `Insert` / `UpdateFields` / `Delete`。
- `RenderForPrompt(entries) string`：分组渲染 + 截断。
- `SyncChapter(...)`：组装「设定集现状 JSON + 章序/标题/正文」user prompt，调 LLM，解析 ops（本地实现剥 ```json 围栏的逻辑，同 extract）。
- `ApplyOps(ctx, st, projectID, seq, ops)`：事务 upsert，去重降级、钳制、origin/source_seq 维护；返回 (created, updated)。
- `internal/llm/prompts.go` 加 `BibleSyncSystem()`（设定集管理员角色，只输出 ops JSON，不删条目，content ≤150 字，不抄正文原句）。
- 单元测试：校验、RenderForPrompt 分组/截断/已收线标注、SyncChapter+ApplyOps 用 fake LLM（仿 `write_test.go` 的 `insertLLMKey` + httptest 模式）测 create/update/同名降级/坏 JSON 重试。

### Task 3 — 写作管线接入（`internal/write/write.go`）
- `Run` 在 loadPrevious 之后加载 `bible.ListByProject`，渲染注入块；`ChapterUserPrompt` 签名加 `bibleBlock string` 参数（更新现有调用与测试）。
- `persist`（prog 95）之后加 `prog(97, "bible_sync")`：复用已解出的 key/model 调 `bible.SyncChapter` + `ApplyOps`，错误忽略。
- `write_test.go` 增测：①captured LLM 请求体含设定集条目文本；②写章成功后 bible_entries 落库 AI 条目；③同步步骤返回垃圾时章节仍为 written。

### Task 4 — HTTP API + 回填 job
- `internal/job/job.go`：`KindBibleSync Kind = "bible_sync"`。
- `internal/bible`：`JobHandler`（回填 Run：按 seq 遍历 status=written 且 body 非空的章节，逐章 Sync+Apply，prog 按章推进，result `{"chapters_synced":N,"entries":M}`）。
- `cmd/stylelab/main.go`：`runner.Register(job.KindBibleSync, bible.JobHandler(...))`。
- 新文件 `internal/httpapi/bible.go` + `server.go` 路由：
  - `GET /api/projects/{id}/bible` → `{entries: [...]}`（requireOwnedProject）
  - `POST /api/projects/{id}/bible` → 201 Entry
  - `PATCH /api/bible/{id}`（指针字段部分更新，成功后 origin=manual）→ 200
  - `DELETE /api/bible/{id}` → 204
  - `POST /api/projects/{id}/bible/sync` → 202 `{job_id}`
- `internal/httpapi/httpapi_test.go`：`newTestServer` 注册 KindBibleSync fake；新增 `TestBibleCRUDAndOwnership`（含跨用户 404）、`TestBibleSyncStartsJob`（轮询到 succeeded）。

### Task 5 — 前端：设定集页面
- `types.ts`：`BibleEntryKind/Status`、`BibleEntry`、`BibleEntrySummary`。
- `api.ts`：`listBible / createBible / patchBible / deleteBible / syncBible`。
- `jobs.ts`/`JobDock`：检查空 route 的处理（resultRoute 返回 ''），确保回填 job 的完成 toast 不渲染无效「查看」按钮，或加分支跳 `/p/:id/bible`。
- 新页 `pages/Bible.tsx`（路由 `/p/:id/bible`，`App.tsx` 加 Route + Sidebar SideLink「设定集」，lucide 图标）：仿 Cards.tsx 骨架——`.library-tools`（搜索/类型过滤/排序）+ 左侧条目行列表（kind 徽标 + 名称 + 状态 + origin 标签）+ 右侧 sticky 编辑面板（复用 `.card-inspector` 风格，双模式：选中编辑/新建）；页头状态 chips（人物 N / 设定 N / 伏笔未收 N）；「从已有章节同步」按钮 → 提交 job + `registerJob` + `JobProgress autoNavigate=false onSucceeded` 原位刷新。
- `styles.css`：条目行、kind 徽标、编辑面板样式（沿用现有 token，≤900px 单栏）。

### Task 6 — 章节工作台侧栏
- `chapterContext.ts`：`ChapterIntel` 加 `bible: BibleEntrySummary[]` / `bibleError`；`emptyIntel` 同步。
- `Chapter.tsx`：`loadBible(projectId)` fire-and-forget（仿 loadCard），aside/drawer 两处传参。
- `ChapterIntelPanel.tsx`：新增「设定集」work-zone（zone-index「志」），按三类分组展示名称+状态+短摘要，底部「去设定集管理」链接。

### Task 7 — 收尾验证
- `go test ./...`、`npx.cmd tsc --noEmit`、`cd web && npx.cmd vite build`（重建 `web/dist` 供 embed）。
- README Workflow 加设定集一节；`progress.md` 追加本次记录。
- **不提交 git**（当前工作树有大量未提交改动，遵循 progress.md 既有惯例，只实现不 commit）。

## 验证标准

- `go test ./...` 全绿（含新增 bible/write/httpapi 测试）；tsc 无错；vite build 成功。
- 行为断言：写章 prompt 含设定集块；写章后 AI 条目落库且 origin=auto/source_seq=章序；同步失败不影响章节状态；CRUD 跨用户 404；回填 job 202→succeeded。

## 规模估算

后端 ~5 个文件改动 + 2 个新文件（bible.go、httpapi/bible.go）+ 2 个测试文件；前端 ~6 个文件改动 + 1 个新页面。预计一个会话内可完成。