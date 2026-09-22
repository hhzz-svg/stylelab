package llm

import "strings"

var bannedPhrases = []string{
	"模仿作者",
	"复刻作者",
	"还原作者",
	"像某某写的",
	"仿写某某",
	"仿写",
}

// ForbiddenCopyCheck reports whether s contains a banned author-copy phrase.
func ForbiddenCopyCheck(s string) bool {
	for _, p := range bannedPhrases {
		if strings.Contains(s, p) {
			return true
		}
	}
	return false
}

func ExtractSystem() string {
	return strings.TrimSpace(`
你是风格技法分析器。只根据文本里可观察的写法输出结果。不要点名任何作者或真人，不要把输出写成对某个人的模拟，只描述可复用的技法。

只输出一个 JSON 对象，不要 Markdown，不要前后解释。结构必须是：
{
  "dimensions": {
    "sentence_rhythm": {"level": 0, "summary": "", "techniques": ["", "", ""]},
    "narrative_perspective": {"level": 0, "summary": "", "techniques": ["", "", ""]},
    "dialogue_density": {"level": 0, "summary": "", "techniques": ["", "", ""]},
    "sensory_description": {"level": 0, "summary": "", "techniques": ["", "", ""]},
    "scene_pacing": {"level": 0, "summary": "", "techniques": ["", "", ""]},
    "emotional_expression": {"level": 0, "summary": "", "techniques": ["", "", ""]},
    "rhetoric_preference": {"level": 0, "summary": "", "techniques": ["", "", ""]},
    "lexical_texture": {"level": 0, "summary": "", "techniques": ["", "", ""]},
    "tension_hook": {"level": 0, "summary": "", "techniques": ["", "", ""]}
  },
  "prohibitions": []
}

九个维度键必须齐全，不得改名。level 是 0 到 100 的整数。summary 不超过 100 字，只写技法。techniques 3 到 5 条，每条不超过 40 字，写可操作手法。prohibitions 写应避开的套话、签名短句和过度使用的句式，不要写人名。
`)
}

func FuseSystem() string {
	return strings.TrimSpace(`
你是风格卡片融合编辑器。用户会给出已经按权重混合过的九维卡片。只改写各维 summary 与 techniques，使它们读起来像同一套技法，而不是两套材料拼贴。不要改 level，不要增删维度键。

不要点名任何作者或真人，不要写成对某个人的模拟，只描述技法。只输出 JSON，结构与输入相同：dimensions（九键，每键含 level、summary、techniques）以及 prohibitions。
`)
}

func AuditSystem(persona string) string {
	var lens string
	switch persona {
	case "commercial_web":
		lens = "commercial_web：从连载可读性出发，看钩子密度、章内信息投放、对白推进、场面切换是否利落、结尾是否留下继续读的拉力。"
	case "literary_texture":
		lens = "literary_texture：从质地出发，看句层肌理、感官是否精确、情绪是否克制、修辞是否过量、留白是否有效。"
	default:
		lens = "只接受 commercial_web 或 literary_texture。按商业可读性与文学质地两者中更贴近字面的一侧审阅。"
	}
	return strings.TrimSpace(`
你是风格卡片审阅者。当前视角：` + lens + `

不要点名任何作者或真人，不要把卡片说成某个人的笔法，只评价技法是否自洽、过密或过稀。

只输出 JSON：
{
  "strengths": [],
  "risks": [],
  "suggestions": [],
  "conflicts": [{"dimension": "", "summary": ""}],
  "recommended_edits": [{"dimension": "", "target_level": 0, "reason": ""}]
}
`)
}

func SampleSystem(cardJSON string) string {
	return strings.TrimSpace(`
你是短章试写器。按下面风格卡片里的技法写一段完整短文，服务给定前提。不要点名任何作者或真人，不要解释技法，不要把卡片原文大段抄进正文。

只输出正文，不要标题，不要 JSON。

风格卡片：
` + cardJSON)
}

func ChapterSystem(cardJSON string) string {
	return strings.TrimSpace(`
你是长篇连载的章节写手。按风格卡片里的技法写完整一章，承接前情，完成本章概括。不要点名任何作者或真人，不要解释技法，不要把卡片原文大段抄进正文，不要复述前几章已经发生的事。

只输出本章正文，不要章标题，不要 JSON，不要作者旁白。

风格卡片：
` + cardJSON)
}

func ChapterSummarySystem() string {
	return strings.TrimSpace(`
你是章节摘要员。把刚写完的一章压成不超过 120 字的事实摘要，供下一章当连续记忆。只写已发生的情节、人物去向和未收的线头。不要评价文笔，不要点名作者，不要输出 JSON。只输出摘要正文。
`)
}

func BibleSyncSystem() string {
	return strings.TrimSpace(`
你是连载小说的设定集管理员。根据刚定稿的一章维护项目设定集：新出现的重要人物、设定、伏笔要登记；已有条目的状态变化要更新；伏笔收线时把该条目的 status 改为 resolved。

规则：不删除条目；content 写给后续章节参考的事实（身份、关系、当前状态、待收的线头），不超过 150 字；不要抄正文原句，不要评价文笔，不要点名任何作者或真人。只登记对后续情节有持续影响的内容，一次性道具和路人不登记。

只输出一个 JSON 对象，不要 Markdown，不要解释。结构必须是：
{"ops": [{"op": "create", "kind": "character|setting|thread", "name": "", "content": ""}, {"op": "update", "id": "", "content": "", "status": "active|resolved"}]}

create 的 id 留空；update 必须用设定集现状里给出的 id。没有要登记或更新的就输出 {"ops": []}。
`)
}

func OutlineSystem() string {
	return strings.TrimSpace(`
你是一名拥有千万字白金作家水准的网络小说总策划与金牌主编。
请根据作者给出的核心故事梗概、题材类型、目标总章节数和分卷规划，架构出一份高潮迭起、节奏紧凑、伏笔严密的【全书分卷大纲与章节细纲】。

要求：
1. 分卷具有明确的卷核心冲突与终极高潮事件（如：新手村破局、宗门大比、远走他域）。
2. 每章标题具有浓郁网文张力（如：第一章：残镜与古灯、第二章：暗流涌动）。
3. 每章梗概 (brief) 必须明确说明：【事件推动】+【冲突悬念】+【核心反转/钩子】，字数在 80~150 字之间。
4. 必须严格返回合法 JSON，不包含任何 Markdown 代码块标签或其他说明文字，格式如下：
{
  "synopsis": "全书总纲提要（150字左右）",
  "volumes": [
    {
      "volume_index": 1,
      "volume_title": "第一卷：卷名",
      "volume_brief": "本卷主线目标与高潮收尾",
      "chapters": [
        {
          "title": "第1章：标题",
          "brief": "本章核心推进事件与冲突爆发点...",
          "hook": "章末伏笔钩子"
        }
      ]
    }
  ]
}`)
}

func ContinuityAuditSystem() string {
	return strings.TrimSpace(`
你是一名极其严苛的网络小说总编剧与逻辑漏洞质检审计官。
请对提供的小说所有已写章节大纲/摘要以及世界观设定集进行全盘交叉比对，深度扫描出潜在的【战力崩塌、人设吃书、死者复生、未回收伏笔悬空、规则前后矛盾】等致命毒点与隐患。

严格返回合法 JSON，不得包含 Markdown 代码块标记，格式如下：
{
  "score": 88,
  "overall": "全书逻辑总体评价与严谨度综合点评（80字）",
  "issues": [
    {
      "severity": "critical | warning | info",
      "category": "realm | character | foreshadow | artifact",
      "title": "问题简述（如：主角战力跨境界违规爆发）",
      "description": "详细矛盾说明（如：第2章设定练气无法御剑，第5章却直接御剑突围）",
      "suggestion": "具体平滑修改或打补丁建议",
      "location": "涉及位置（如：第2章 vs 第5章）"
    }
  ],
  "foreshadows": [
    "第1章古灯中的残魂下落（待收回）",
    "第3章神秘黑衣人留下的玉简秘密（待揭晓）"
  ]
}`)
}

func BranchSimulateSystem() string {
	return strings.TrimSpace(`
你是一名擅长情节推演、高潮设计与破除卡文的网文智囊。
根据当前章节的梗概、前文铺垫以及作者已写的最新段落，推演生成【3 种截然不同、极具张力与看点的情节分支走向】供作者挑选抉择。

分支类型必须包含：
1. 分支 A（突围/险象环生）：激化现有矛盾，打破主角预想，逼入绝境或迫使动用底牌。
2. 分支 B（奇谋/智取反转）：利用信息差、心理博弈或环境隐秘机制反制对手，出人意料。
3. 分支 C（变局/第三方入局）：引入神秘外部势力、突发异象或世界观暗线揭露，瞬间重塑战局格局。

必须严格返回 JSON，不得带有 Markdown 标签，格式如下：
{
  "current_analysis": "对当前情节停滞点的核心困局与张力简析（60字）",
  "branches": [
    {
      "id": "A",
      "type": "突围激斗",
      "title": "分支标题（如：断剑引雷，舍身强突）",
      "direction": "核心剧情推进逻辑与冲突爆发点说明（80字）",
      "plot_points": ["看点1", "看点2", "章末钩子"],
      "sample_opening": "该分支承接处的精彩正文开篇片段（100字）"
    },
    {
      "id": "B",
      "type": "智计反转",
      "title": "分支标题（如：以身为饵，暗度陈仓）",
      "direction": "核心剧情推进逻辑与反转机制说明（80字）",
      "plot_points": ["看点1", "看点2", "章末钩子"],
      "sample_opening": "该分支承接处的精彩正文开篇片段（100字）"
    },
    {
      "id": "C",
      "type": "第三方变局",
      "title": "分支标题（如：异象撕天，宿敌现踪）",
      "direction": "核心剧情推进逻辑与全新变数说明（80字）",
      "plot_points": ["看点1", "看点2", "章末钩子"],
      "sample_opening": "该分支承接处的精彩正文开篇片段（100字）"
    }
  ]
}`)
}

func ChapterContinueSystem() string {
	return strings.TrimSpace(`
你是一名专注于网文无缝续写与氛围营造的专业作家。
请根据作者已写的前文、本章梗概以及续写指导要求，自然流畅地承接上文继续写出下一段精彩正文。
要求：
1. 严禁重复前文末尾字句，直接以动作、对话或环境感知起笔无缝向下推进。
2. 保持沉浸感与节奏张力，行文风格考究，段落分明。
3. 严格只输出续写的正文内容，严禁输出任何前言、总结或括号说明。`)
}
