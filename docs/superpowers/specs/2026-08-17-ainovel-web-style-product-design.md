# ainovel-cli 衍生 Web 产品设计：风格实验室

日期：2026-08-17  
状态：已确认  
产品代号：Style Lab（工作名，可改）

## 1. 背景与边界

ainovel-cli（https://github.com/voocel/ainovel-cli）是 Go 写的全自动 AI 长篇小说创作引擎，已有 Architect / Writer / Editor / Arbiter、TUI + Headless、外部小说导入、Voice Layer、styles 预设、rules、七维 Editor 评审、eval/stylestat。

本产品不做 CLI 包装，做独立 Web 产品（方案 B）：有账号、项目库、上传素材、在线保存风格卡片。后端独立实现，契约兼容 ainovel-cli，后续再接入其引擎。

### 1.1 硬边界

- 不做“可识别复刻某个具体作者”的仿写器。
- “风格抽离”只输出高层技法画像：句式节奏、叙述视角、对白密度、感官描写、场景推进、情绪表达、修辞偏好、禁用倾向等。
- “风格融合”目标是生成新的混合风格，不输出“模仿某某作者”。
- 当代作者素材的产品文案和提示词禁止“还原 / 复刻 / 模仿作者本人”的表达。
- 卡片不保存原文大段引用。`techniques` 只存抽象描述；`facts` 只存统计数字。签名短语只进 `prohibitions`（作为“避免滥用”清单），不做“特色词库供复用”。

### 1.2 已确认决策

| 决策 | 选择 |
|---|---|
| 与 ainovel-cli 关系 | 独立服务，契约兼容。v1 不运行 ainovel-cli 代码。schema 对齐 `simulation_profile.json`，确定性统计移植 stylestat。 |
| 技术栈 | Go 后端 + React 前端 |
| LLM 费用 | 用户自带 Key（BYOK），加密存储，仅本人可用 |
| 架构 | 方案 A：模块化单体 + 内嵌 job 队列。单 Go 二进制，`go:embed` 承载前端。开发用 SQLite，生产可切 Postgres。 |
| v1 不做 | 风格市场、多人协作、复杂权限、在线训练、自定义 Agent 编排、平台代付/计费、OAuth 第三方登录 |

### 1.3 上游可复用事实

- `internal/domain/simulation.go` 的 `SimulationProfile` 已是结构化写作画像：Style / Lexicon / PlotDesign / HookDesign / PacingDensity / ReaderEngagement / DoNotCopy。
- `internal/stylestat` 是自包含纯函数：正则统计 AI 文风 tic、高频短语、跨章复读、章末形态、开篇时间锚点率。零 LLM、零外部依赖。
- Go `internal/` 不能被外部 module import。v1 不 fork，只移植纯函数并保持输出语义一致。
- ainovel-cli 是单机单用户、本地文件 store。不能直接当 Web 多租户引擎用。

## 2. 产品形态

### 2.1 用户流

```
登录/注册 → 项目库 → 项目工作区
                     ├─ 素材页        上传 txt/md → 分章 + 统计概览
                     ├─ 卡片库        抽离卡 / 融合卡 / 手动卡
                     ├─ 卡片详情      实验室：雷达图 + 逐维调节 + 版本历史 + 导出
                     ├─ 融合工作台    选 2–4 张卡 → 勾维度 → 拉权重 → 生成融合卡
                     ├─ 审计面板      选卡 → 双人格报告
                     └─ 试写预览      选卡 + 一句话梗概 → 800–2000 字样章 + 统计回显
设置页：API Key 管理（加密存储）
```

所有慢操作（抽离 / 融合 / 审计 / 试写）统一为：提交 → job → 轮询或 SSE 进度 → 结果。页面共用一个进度组件。

### 2.2 成功标准（v1）

1. 用户上传一份中文小说样本文本，能得到一张可保存、可调节的风格卡片。
2. 用户选 2–4 张卡、勾选维度、调权重，能得到一张带 lineage 的融合卡。
3. 用户能在实验室里改维度、保存新版本、回溯旧版本。
4. 用户能对一张卡跑双人格审计，看到优点、冲突、风险、建议。
5. 用户能用一张卡试写 800–2000 字样章，并看到 stylestat 回显。
6. 卡片能导出为兼容 `simulation_profile.json` 的文件。
7. 产品文案和提示词中不出现“模仿/复刻/还原某某作者”。

## 3. 风格卡片契约

卡片是全产品的核心对象。9 个可调维度 + 1 个禁用清单。每维结构一致，才能按维加权融合。

### 3.1 维度清单（固定，v1 不可增删改名）

| key | 中文名 |
|---|---|
| `sentence_rhythm` | 句式节奏 |
| `narrative_perspective` | 叙述视角 |
| `dialogue_density` | 对白密度 |
| `sensory_description` | 感官描写 |
| `scene_pacing` | 场景推进 |
| `emotional_expression` | 情绪表达 |
| `rhetoric_preference` | 修辞偏好 |
| `lexical_texture` | 用词质感 |
| `tension_hook` | 钩子与张力 |

### 3.2 JSON 形状

```json
{
  "id": "crd_...",
  "project_id": "prj_...",
  "name": "冷峻短句·样本A",
  "kind": "extracted",
  "version": 3,
  "dimensions": {
    "sentence_rhythm": {
      "level": 72,
      "summary": "短句为主，段落常以单句成段收束",
      "techniques": ["三段递进后急收", "对白后不接神态描写"]
    }
  },
  "prohibitions": ["不使用排比抒情收尾", "避免'眼中闪过'类神态模板"],
  "facts": {},
  "lineage": null
}
```

字段约束：

- `kind`：`extracted` | `fused` | `manual`
- `version`：从 1 起。每次手动调节保存产生新版本，旧版本只读。
- `level`：整数 0–100。
- `summary`：不超过 100 个汉字（按 rune 计）。
- `techniques`：3–5 条抽象技法，每条不超过 40 个汉字。禁止出现作者名、书名、可识别原文摘抄。
- `prohibitions`：字符串数组。融合时取并集去重。
- `facts`：仅抽离卡必填。存放移植 stylestat 的确定性统计。融合卡 / 手动卡可为空对象。
- `lineage`：仅融合卡必填。结构：

```json
{
  "parent_cards": [
    {
      "card_id": "crd_...",
      "version": 2,
      "dims": ["sentence_rhythm", "dialogue_density"],
      "weights": { "sentence_rhythm": 60, "dialogue_density": 40 }
    }
  ],
  "prompt_version": "fuse-v1"
}
```

### 3.3 导出映射

卡片可导出 `simulation_profile.json`（`version = "simulation_profile.v1"`）：

| 本产品 | 上游 |
|---|---|
| `sentence_rhythm.summary/techniques` | `synthesis.style.sentence_rhythm` |
| `narrative_perspective` | `synthesis.style.perspective` + `narrative_voice` |
| `lexical_texture` + `rhetoric_preference` | `synthesis.style.prose_texture` |
| `emotional_expression` | `synthesis.style.mood` |
| `prohibitions` | `synthesis.style.do_not_copy` |
| `scene_pacing` + `dialogue_density` | `synthesis.pacing_density` |
| `tension_hook` | `synthesis.hook_design` |
| `facts` | 不写入上游 schema；仅本产品保留 |

导出文件不含作者名、书名、原文摘抄。`corpus.sources` 只写用户上传文件的相对名和哈希，不写外部来源标注。

## 4. AI 管线

所有管线都是 job：提交返回 `job_id`，worker 写进度，完成后写结果 ID。

### 4.1 抽离（extract）

输入：项目内 1 个或多个素材（合计最多 100,000 汉字）。  
输出：一张 `kind=extracted` 的卡片，version=1。

步骤：

1. 读素材纯文本，按空行 / `第N章` 标题切章。不足 5 章时 stylestat 的全书级统计返回空，但抽离仍继续（facts 标记 `sample_too_small: true`）。
2. 跑移植后的 stylestat，结果写入 `facts`。
3. 将 facts 摘要 + 抽样段落（每章最多 400 字，合计不超过 6,000 字）送给 LLM。prompt 禁止出现“模仿/复刻/还原作者”，只要求输出 9 维 + prohibitions 的 JSON。
4. 校验 JSON：9 维齐全、level 0–100、techniques 3–5 条、无作者名/书名模式。失败则重试最多 2 次，仍失败则 job 失败。
5. 规则层把 stylestat 命中的通用 AI tic 追加进 `prohibitions`（去重）。

LLM：temperature 0.3。用户自己的 Key。

### 4.2 融合（fuse）

输入：2–4 张同项目卡片 + 每张卡参与的维度集合 + 每维权重 0–100。同一维度上参与卡的权重之和必须为 100。  
输出：一张 `kind=fused` 的卡片，version=1。

步骤：

1. 纯计算：对每个被选维度，`new_level = round(Σ parent_level × weight / 100)`。未参与融合的维度不出现在结果卡中——不允许。v1 规定：结果卡必须仍有完整 9 维。未勾选的维度取权重最高的那张父卡对应维；若并列取先加入的父卡。
2. `techniques` 先按父卡权重拼接去重，截到最多 5 条。
3. `prohibitions` 取并集去重。
4. LLM 二次裁定：输入父卡被选维的 summary/techniques + 计算后的 level，输出新的 summary/techniques，并标 `conflicts[]`（字符串数组，写入 job 结果，不进卡片本体）。
5. 写 `lineage`。`prompt_version` 固定 `"fuse-v1"`。

### 4.3 双人格审计（audit）

输入：一张卡片 + 两个固定人格 ID。v1 只提供两个内置人格，不可自定义：

- `commercial_web`：商业网文编辑
- `literary_texture`：文学质感编辑

输出：一份审计报告，存 `audit_reports`。

报告结构（JSON，前端渲染为两栏 + 汇总）：

```json
{
  "card_id": "crd_...",
  "card_version": 3,
  "personas": ["commercial_web", "literary_texture"],
  "by_persona": {
    "commercial_web": {
      "strengths": ["..."],
      "risks": ["..."],
      "suggestions": ["..."]
    },
    "literary_texture": {
      "strengths": ["..."],
      "risks": ["..."],
      "suggestions": ["..."]
    }
  },
  "conflicts": [
    {
      "dimension": "sentence_rhythm",
      "summary": "商业侧要更快的短句推进，文学侧认为会削掉余韵"
    }
  ],
  "recommended_edits": [
    { "dimension": "sentence_rhythm", "target_level": 58, "reason": "..." }
  ]
}
```

两个 persona 的 LLM 调用并行。temperature 0.2。提示词只评价技法画像，禁止评价“像不像某作者”。

### 4.4 试写（sample）

输入：一张卡片 + 一句话场景梗概（最多 80 字）+ 目标字数 800–2000（默认 1200）。  
输出：样章正文 + 对该正文跑 stylestat 的 facts（单章时全书级统计可能为空，仍返回句式计数）。

v1 自研简化 prompt：把 9 维 summary/techniques + prohibitions 编进系统提示，用户提示只有场景梗概。不调用 ainovel-cli host 引擎。

## 5. 实验室界面

分栏：

- 左：本项目卡片列表 + 当前卡版本下拉。
- 中：9 个维度滑块（改 level）+ 每维 summary 只读、techniques 只读。v1 不在滑块页直接改文案；改文案走“保存为新版本”时可选“让模型按新 level 重写 summary”（默认关闭，只改数字）。
- 右：雷达图（当前版本 vs 上一版本，或融合预览时 vs 父卡）+ 禁用清单。

融合工作台是独立页，不是弹窗：选 2–4 张卡，每张卡 9 个维度复选框 + 权重滑块，提交生成融合卡。

审计、试写是卡片详情页上的两个动作，结果各自一页可回看。

进度：进行中的 job 在顶栏显示，支持取消（协作式 cancel，worker 在阶段边界检查）。

## 6. 数据与存储

开发：SQLite。生产：同一套 SQL，可切 Postgres。素材文件存本地磁盘目录（v1 不引入 MinIO/S3）。路径 `{data_dir}/blobs/{sha256}`，数据库只存哈希、原文件名、字数、分章数。

### 6.1 表

- `users`：id, email, password_hash, created_at
- `user_llm_keys`：user_id, provider (`openai` | `anthropic` | `compatible`), base_url, encrypted_key, created_at, updated_at。v1 每用户每 provider 一条。
- `projects`：id, user_id, name, created_at
- `assets`：id, project_id, filename, sha256, rune_count, chapter_count, created_at
- `style_cards`：id, project_id, name, kind, current_version, created_at, updated_at
- `style_card_versions`：card_id, version, dimensions_json, prohibitions_json, facts_json, lineage_json, created_at
- `jobs`：id, user_id, project_id, kind (`extract`|`fuse`|`audit`|`sample`), status (`queued`|`running`|`succeeded`|`failed`|`canceled`), progress (0–100), stage, payload_json, result_json, error, created_at, started_at, finished_at
- `audit_reports`：id, project_id, card_id, card_version, report_json, created_at
- `samples`：id, project_id, card_id, card_version, premise, body, facts_json, created_at

删除项目为硬删除，级联删该项目下素材元数据、卡片、报告、样章。blob 若无其他引用再删文件。

### 6.2 认证与 Key

- 邮箱 + 密码。密码用 bcrypt。session 用 httpOnly cookie + 随机 token，存 `sessions` 表（id, user_id, token_hash, expires_at）。有效期 14 天。
- LLM Key 用 AES-256-GCM 加密，主密钥来自环境变量 `STYLELAB_MASTER_KEY`（32 字节 hex）。进程不落盘明文 Key。API 回读 Key 时只返回后 4 位和 provider。
- v1 无邮箱验证、无重置密码邮件。本地开发可用 `STYLELAB_DEV_AUTO_LOGIN` 跳过，生产必须关闭。

## 7. 架构

单个 Go 模块 `stylelab`：

```
cmd/stylelab/main.go          HTTP + 内嵌 worker + embed 前端
internal/httpapi              路由与 handler
internal/auth                 注册登录 session
internal/store                SQL + blob
internal/job                  队列、状态机、worker
internal/stylestat            从 ainovel-cli 移植的纯函数
internal/card                 StyleCard 校验、融合纯计算、导出 mapping
internal/llm                  带重试的 chat completions 客户端
internal/extract              抽离编排
internal/fuse                 融合编排
internal/audit                审计编排
internal/sample               试写编排
web/                          React SPA，build 后 embed
```

Job runner：进程内 goroutine pool，并发上限默认 2（可用配置改）。任务状态落库。进程重启后 `running` 标为 `failed`（error=`interrupted`），`queued` 重新领取。前端 GET `/api/jobs/:id` 轮询；可选 GET `/api/jobs/:id/events` SSE。

LLM 客户端：OpenAI 兼容 `/v1/chat/completions`。Anthropic 走其 messages API。超时 120s，最多重试 2 次（仅 429/5xx）。不实现流式试写以外的流；试写 v1 也可以非流式，整段返回。

## 8. 合规与文案

产品内固定禁用词（UI 和系统提示词）：`模仿作者`、`复刻作者`、`还原作者`、`像某某写的`、`仿写某某`。

上传页提示：请确保你有权使用该文本；系统只抽取抽象技法，不保存可识别原文片段到卡片。

## 9. 测试与验收

- stylestat：用 ainovel-cli 仓库中公开测试文本或自备中文短篇，对比关键计数字段语义一致（允许因移植产生的字段名差异，不允许同一输入下模式计数对不上）。
- card 校验：缺维、level 越界、techniques 含作者名模式，必须拒绝。
- fuse 纯计算：给定两张卡和权重，level 结果有固定夹具。
- job：提交后能查到 queued→running→succeeded；取消后 worker 在下一阶段退出。
- 认证：未登录访问 `/api/projects` 返回 401；Key 回读不含明文。

## 10. 非目标（再次声明）

v1 不做：风格市场、分享链接、多人协作、RBAC、OAuth、计费、配额、MinIO、独立 worker 进程、接入 ainovel-cli headless、自定义人格、自定义维度、在线微调、长篇连载生成。
