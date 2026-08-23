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
