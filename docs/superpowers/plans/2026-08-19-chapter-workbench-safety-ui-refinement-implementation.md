# Chapter Workbench Safety and UI Refinement — Implementation Tracking

Date: 2026-08-19

Status: 进行中

Source of truth: [章节工作台安全与 UI 加固设计](../specs/2026-08-19-chapter-workbench-safety-ui-refinement-design.md)

## Goal

在不修改写作 API、数据库、生成规则或全局路由的前提下，消除章节错位提交、写操作竞态和重复生成风险，并把章节页调整为正文优先、操作始终可达、情报抽屉可访问的工作台。

## Scope amendment already accepted

- 章节页拥有的上一章、下一章、回章程和情报链接统一经过离开确认。
- 刷新和关闭页面继续使用 `useDirtyGuard`。
- 普通 `BrowserRouter` 下的浏览器后退是已知限制；本轮不迁移 Data Router，也不实现 `popstate` 回跳。

## Dependency order

```text
T1 请求身份与降级加载
 └─> T2 写操作生命周期与任务终态
       └─> T3 离开确认与临时生成草稿
             └─> T4 页面结构与命令栏
                   └─> T5 情报内容与模态 dialog
                         └─> T6 章节专用响应式样式
                               └─> T7 工程验证
                                     └─> T8 浏览器状态矩阵
                                           └─> T9 文档与进度闭环
```

同一文件默认串行施工。`Chapter.tsx` 涉及 T1–T4，因此这些任务不能由多个代理并行修改。

## Tracking

| ID | Task | Depends on | Status | Primary files | Completion evidence |
|---|---|---|---|---|---|
| T0 | 规格边界修正 | — | 已完成 | design spec | 明确 BrowserRouter 限制与 `JobProgress` 兼容扩展 |
| T1 | 请求身份与降级加载 | T0 | 已完成 | `Chapter.tsx`, optional `chapterContext.ts` | 过期响应不落地，辅助失败不阻断正文，TypeScript 通过 |
| T2 | 写操作生命周期与任务终态 | T1 | 已完成 | `Chapter.tsx`, `JobProgress.tsx` | 单一阶段、重复提交拦截、真实任务失败与轮询失败分离 |
| T3 | 离开确认与临时生成草稿 | T1, T2 | 已完成 | `Chapter.tsx`, `ChapterIntelPanel.tsx` | 章节自有导航统一确认，note/model 语义正确 |
| T4 | 页面结构与顶部命令栏 | T2, T3 | 进行中 | `Chapter.tsx` | 正文首屏可见，命令栏显示阶段与准备状态 |
| T5 | 情报内容与模态 dialog | T1, T4 | 待开发 | `ChapterIntelPanel.tsx`, `Chapter.tsx` | 状态互斥、3×3 九维、Esc/焦点恢复可用 |
| T6 | 章节专用响应式样式 | T4, T5 | 待开发 | `styles.css` | 1180px 断点、无 410px 空白、无底部遮挡 |
| T7 | 工程验证 | T1–T6 | 待开发 | scoped frontend files | TypeScript、Go 回归、diff check 通过 |
| T8 | 浏览器状态矩阵 | T7 | 待开发 | no source changes unless defect found | 三个视口和关键竞态均有实机证据 |
| T9 | 文档与进度闭环 | T8 | 待开发 | this plan, `progress.md` | 追踪状态更新，记录测试、文件清单和回滚点 |

## T1 — Request identity and degraded loading

依据：设计规格 §5.1、§5.2、§8。

改动范围：

- `web/src/pages/Chapter.tsx`
- `web/src/chapterContext.ts`，仅当一个纯函数能真实减少页面分支时使用

Implementation actions:

- [x] 用递增 request epoch 和目标 `projectId/chapterId` 标识每轮加载。
- [x] 路由参数生效后，先清空旧 `ch`、旧情报、旧任务状态和上一章临时草稿，再开始新加载。
- [x] 写操作只允许使用已加载且与当前路由匹配的 `ch.id`。
- [x] 将当前章节、卡片摘要列表、章节摘要列表并行发起但独立处理；只有当前章节失败才进入整页错误。
- [x] 分离整页错误、卡片列表错误、邻章列表错误和操作错误，禁止继续复用一个 `error` 字符串承载所有语义。
- [x] `loadCard` 和 `loadPrev` 绑定请求目标 ID 与 epoch；开始新请求时退出旧内容的正常展示。
- [x] 卡片和前情的 loading、error、empty、content 状态保持互斥。
- [x] 项目或章节切换时清理仅属于旧目标的页面缓存；不改后端缓存或全局缓存。

Verify:

- [x] `cd web; npx.cmd tsc --noEmit`
- [x] 静态路径检查：A 请求后返回时若当前目标已是 B，所有 `setCh/setIntel/applyChapter` 均不得执行。
- [x] 静态路径检查：`listCards` 或 `listChapters` 失败时，已成功取得的当前章节仍进入编辑器。

## T2 — Write operation lifecycle and job terminal states

依据：设计规格 §5.3、§5.5、§8。

改动范围：

- `web/src/pages/Chapter.tsx`
- `web/src/components/JobProgress.tsx`

Implementation actions:

- [x] 用 `idle | saving | submitting | generating | refreshing` 替换 `busy` 与 `jobActive` 的重叠语义，并用同步 ref 防止同一事件循环内双击。
- [x] 保存、补保存、写作提交和生成后刷新均以当前 `ch.id` 为目标；身份不匹配时直接拒绝。
- [x] 保存、提交和刷新期间锁定编辑控件；生成期间锁定会与生成结果冲突的编辑控件，但不阻止安全离开。
- [x] 写作按钮在第一个 `await` 前进入 `submitting`。
- [x] 写作前只镜像既有服务端条件：风格卡已选、章概括非空。失败时展开设定并聚焦首个缺失字段。
- [x] 重写判断使用服务器正文和本地正文的并集，并在任何补保存前确认。
- [x] 任务受理后设置 `jobId`、注册 JobDock 记录并清空一次性补充；提交失败保留补充。
- [x] `JobProgress` 设置 `autoNavigate={false}`；成功后进入 `refreshing`，最新章节应用成功后才回到 `idle`。
- [x] 为 `JobProgress` 增加可选的轮询读取失败回调；章节页传入该回调时，不把 `useJobPoll` 的读取失败伪装成任务失败，而是从实际 `job.status` 转发真实状态并单独报告轮询错误。未传入时保持现有调用方行为不变，不修改 `hooks.ts`。
- [x] 轮询读取失败时保持生成锁定，并由章节页提供重新挂载进度读取的重试；真实任务失败或取消才解除生成锁定。
- [x] 页面加载到 `status === 'writing'` 且没有本地 `jobId` 时，用现有章节读取接口轮询到终态；不增加活动任务 API。

Verify:

- [x] `cd web; npx.cmd tsc --noEmit`
- [x] 连续触发保存或写作处理函数时，只能进入一个请求路径。
- [x] `JobProgress` 现有调用不需要修改即可编译，默认自动跳转行为不变。
- [x] 轮询网络错误不会被当成任务 `failed` 并开放再次生成。

## T3 — Leave confirmation and transient generation drafts

依据：设计规格 §5.1、§5.4。

改动范围：

- `web/src/pages/Chapter.tsx`
- `web/src/components/ChapterIntelPanel.tsx`

Implementation actions:

- [x] 分开计算 persisted dirty 与 generation-draft dirty；保存按钮只看前者，`useDirtyGuard` 与页面离开确认看两者并集。
- [x] 模型标签改为“本次生成模型（留空使用默认）”。
- [x] 共用一个异步离开确认入口处理上一章、下一章和回章程；用户确认前不调用 `navigate`、不清状态。
- [x] 情报区内部的牌库和实验室链接通过回调使用同一离开确认；保留修饰键打开新标签的正常浏览器行为，因为它不会离开当前页。
- [x] 情报 dialog 内需要确认的链接先暂时关闭原生模态，再调用全局确认框；取消后重新打开并恢复焦点，确认后直接导航。其他入口仍在确认后才导航，新路由参数生效后再由 T1 清旧状态。
- [x] 刷新和关闭继续使用 `useDirtyGuard`；不修改全局 Router，不声称拦截浏览器后退。

Verify:

- [x] persisted dirty、只填写 note、只修改 model 三种情况都触发页面离开保护。
- [x] 用户取消确认后 URL、章节内容、note/model 和情报状态均不改变。
- [x] 无修改时章节自有导航直接执行，不多弹一次确认。

## T4 — Body-first page structure and command bar

依据：设计规格 §6.1–§6.3。

改动范围：

- `web/src/pages/Chapter.tsx`

Implementation actions:

- [ ] 将上一章、回章程、下一章放入标题区紧凑导航；去掉底部操作栏中的对应控件。
- [ ] 将命令栏放到工作区内容之前，显示当前阶段、情报入口、保存和写/重写。
- [ ] 情报入口只在常驻情报栏隐藏时显示。
- [ ] 本章设定摘要显示风格卡、章概括和目标字数的准备状态。
- [ ] 为设定折叠区、风格卡和章概括准备 ref，支持 T2 的展开与聚焦。
- [ ] 将工作台主栏类名改为章节专用名称，消除 `.chapter-main` 与章程列表链接的冲突。
- [ ] 所有表单控件依据 T2 阶段显示正确的 disabled/readOnly 状态；状态原因在命令栏可见。
- [ ] 整页、辅助、保存、提交、任务和刷新错误在对应位置展示，不让 toast 成为唯一反馈。

Verify:

- [ ] `cd web; npx.cmd tsc --noEmit`
- [ ] DOM 顺序为标题导航 → 命令栏 → 折叠设定 → 正文 → 摘要；键盘 Tab 顺序与视觉顺序一致。

## T5 — Intel content and native modal dialog

依据：设计规格 §6.4、§7、§8。

改动范围：

- `web/src/components/ChapterIntelPanel.tsx`
- `web/src/pages/Chapter.tsx`，只负责 opener ref、open/close 和导航回调

Implementation actions:

- [ ] 保持 aside 与 dialog 共用同一情报正文，避免复制数据逻辑。
- [ ] 风格卡九维改为紧凑 3×3 名称/数值格；保留卡片元数据、约束和实验室入口。
- [ ] loading、error、empty、content 使用互斥分支；前情读取失败时不再同时显示“还没有摘要”。
- [ ] dialog 通过 `createPortal(..., document.body)` 脱离带 transform 的 `.page`。
- [ ] 使用原生 `<dialog>` 和 `showModal()`；标题与关闭按钮固定，内容独立滚动。
- [ ] 支持原生 `cancel`/Esc、关闭按钮和遮罩点击；关闭后把焦点恢复到情报 opener。
- [ ] 验证情报链接的全局离开确认不会被原生模态层遮挡；取消时 dialog 重新打开，确认时保持关闭并导航。
- [ ] 以 `aria-labelledby` 关联标题；所有 dialog 内按钮声明正确 `type`。

Verify:

- [ ] `cd web; npx.cmd tsc --noEmit`
- [ ] dialog 打开后计算位置为视口顶部，页面滚动不改变其 viewport 定位。
- [ ] Esc、关闭按钮和遮罩均关闭；关闭后 `document.activeElement` 是 opener。

## T6 — Scoped responsive CSS

依据：设计规格 §6、§7。

改动范围：

- `web/src/styles.css`

Implementation actions:

- [ ] 只重写 chapter-workbench 范围的选择器，不改牌库、融合、实验室和共享 deck drawer。
- [ ] 为章节设定和情报块覆盖 `.work-zone { min-height: 410px; }`，折叠区不再留下空面板。
- [ ] 新的章节主栏类维持 `gap: 1rem`，不受章程列表 `.chapter-main` 后置规则影响。
- [ ] ≥1180px 使用正文/情报两栏；<1180px 隐藏常驻情报并显示命令栏入口。
- [ ] ≤900px 将命令栏粘在移动顶栏下方，禁止底部 fixed/sticky 与 JobDock 竞争。
- [ ] 添加紧凑九维格和章节 dialog 样式；dialog header 固定、body 滚动、宽度使用容器内百分比而非 `100vw`。
- [ ] 390px 下按钮不横向溢出；必要时只允许章节标题导航自身换行，不增加横向滚动容器。
- [ ] 继续继承全局 `prefers-reduced-motion`，不新增动效。

Verify:

- [ ] `git diff --check -- web/src/styles.css`
- [ ] CSS 搜索确认没有修改 `.deck-drawer`、`.work-zone` 通用定义或其他工作台断点。

## T7 — Engineering verification

依据：设计规格 §9.1。

- [ ] `cd web; npx.cmd tsc --noEmit`
- [ ] `go test ./internal/write ./internal/httpapi -count=1`
- [ ] 对本轮源文件执行 scoped `git diff --check`。
- [ ] 检查本轮 diff，确认没有 API、schema、prompt、`Write.tsx`、全局 Router 或其他工作台改动。

任何失败必须先修复并重跑，不得直接进入浏览器验收。

## T8 — Browser state matrix

依据：设计规格 §9.2。

使用 Playwright CLI，不新增测试框架。浏览器内模拟 API 只存在于会话中，不写入产品代码。

- [ ] 1440×900：正文和情报常驻，正文首屏可见，九维与前情可扫描。
- [ ] 1024×768：正文单栏，命令栏情报入口可用，无正文窄列。
- [ ] 390×844：移动顶栏、命令栏、正文和 dialog 无横向溢出或 JobDock 遮挡。
- [ ] 延迟 A 章并先返回 B 章：最终只能显示和提交 B。
- [ ] 卡片列表失败、章节列表失败：当前正文仍可编辑保存。
- [ ] 双击保存、双击写作：各只有一个网络请求。
- [ ] 服务器有正文而本地清空：仍显示重写并确认。
- [ ] 缺卡、缺概括：不发写作请求，设定展开并聚焦。
- [ ] note 提交失败保留、任务受理清空、切章前提醒。
- [ ] 任务成功停留本章，刷新完成前保持锁定。
- [ ] 任务真实失败解除锁定；任务进度读取失败保持生成锁定并可重试。
- [ ] dialog 顶部、内部滚动、Esc、遮罩、焦点约束和焦点恢复通过。

若浏览器检查发现缺陷，只回到对应 T1–T6 修复，不顺带改其他页面。

## T9 — Documentation and progress closure

依据：仓库 `AGENTS.md` 记录规范和设计规格 §9.3。

- [ ] 将 T1–T9 的实际状态更新到本追踪表。
- [ ] 在 `progress.md` 末尾追加一轮记录，包含业务结果、可信测试证据、全部改动文件和可执行回滚点。
- [ ] 若有验证未执行或失败，明确写为缺口，不使用“已可用”措辞。
- [ ] 不提交、格式化或清理与本任务无关的脏工作树文件。

## Rollback boundary

## Implementation status update — 2026-08-20

- T1–T3: completed before this implementation pass; the accepted native-dialog navigation rule is recorded in the design spec and applied by `ChapterIntelPanel`/`Chapter.tsx`.
- T4: completed. The chapter route now presents title navigation, a top command bar, collapsible setup, body, and summary in that order; the chapter editor uses a chapter-specific class.
- T5: completed. Intel loading/error/empty/content branches are mutually exclusive, the nine dimensions use a compact 3×3 grid, and the portal/native-dialog focus behavior is implemented.
- T6: completed. Chapter-scoped rules cover the 1180px/900px/560px breakpoints, remove the shared 410px minimum from chapter zones, and keep the mobile command bar below the mobile top bar.
- T7: completed. TypeScript, Vite build, Go write/httpapi regression tests, and scoped diff checks pass.
- T8: partially completed. Live checks cover 1440×900, 1024×768, 390×844, dialog Esc/focus, dirty-leave cancel, and missing-card focus. Delayed-response, auxiliary-request failure injection, duplicate-request counting, and real job terminal/poll-error flows remain verification gaps.
- T9: completed. The progress record is appended, verification gaps are called out explicitly, and the final branch status is reported without staging or committing unrelated worktree files.

实施完成后的回滚必须只针对本计划实际修改的 hunks。不得使用 `git reset --hard`、`git checkout --` 或覆盖整个 `styles.css`。若最终创建独立提交，优先使用对应提交的 `git revert`；否则按 `progress.md` 中记录的文件与 hunk 逐项回退。
