package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"stylelab/internal/llm"
)

type continuityAuditItem struct {
	Severity    string `json:"severity"` // critical | warning | info
	Category    string `json:"category"` // realm (战力境界) | character (人物人设) | foreshadow (伏笔悬念) | artifact (物品法宝)
	Title       string `json:"title"`
	Description string `json:"description"`
	Suggestion  string `json:"suggestion"`
	Location    string `json:"location"` // e.g. "第3章 vs 第7章" 或 "世界设定: 灵石体系"
}

type continuityAuditResponse struct {
	Score       int                   `json:"score"`   // 0 - 100
	Overall     string                `json:"overall"` // 总体评价
	Issues      []continuityAuditItem `json:"issues"`
	Foreshadows []string              `json:"foreshadows"` // 已埋下待收回的重要伏笔清单
}

// Context bounds for the continuity audit. The prompt is assembled from every
// chapter in the project, so on a long manuscript an unbounded build overruns
// the model's context window and the reply comes back as truncated JSON that
// fails to parse. Raising MaxTokens does not help: that caps the completion,
// not the prompt.
const (
	auditChapterLimit = 60
	auditChapterRunes = 180
	auditBibleLimit   = 40
	auditBibleRunes   = 200
)

func (s *Server) handleContinuityAudit(w http.ResponseWriter, r *http.Request) {
	userID, err := s.currentUserID(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}
	projectID := r.PathValue("id")
	if err := s.requireOwnedProject(r.Context(), projectID, userID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "not_found", "not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "invalid", "internal error")
		return
	}

	var req struct {
		Model string `json:"model"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	// 1. 获取所有章节目录与摘要
	var chapterList []string
	var totalChapters int
	rows, err := s.st.DB().QueryContext(r.Context(),
		`SELECT seq, title, brief, summary, status FROM chapters WHERE project_id = ? ORDER BY seq ASC`,
		projectID,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	defer rows.Close()
	for rows.Next() {
		var seq int
		var title, brief, summary, status string
		if err := rows.Scan(&seq, &title, &brief, &summary, &status); err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "internal error")
			return
		}
		totalChapters++
		if len(chapterList) >= auditChapterLimit {
			continue
		}
		content := summary
		if content == "" {
			content = brief
		}
		chapterList = append(chapterList, fmt.Sprintf("【第%d章 %s】(状态:%s)\n梗概与摘要：%s", seq, title, status, headRunes(content, auditChapterRunes)))
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}

	if len(chapterList) == 0 {
		writeError(w, http.StatusBadRequest, "invalid", "作品尚无任何章节内容，无法进行伏笔逻辑审计")
		return
	}

	// 2. 获取世界观与人物设定
	var bibleList []string
	bRows, err := s.st.DB().QueryContext(r.Context(),
		`SELECT kind, name, content FROM bible_entries WHERE project_id = ? AND status = 'active' LIMIT ?`,
		projectID, auditBibleLimit,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	defer bRows.Close()
	for bRows.Next() {
		var k, n, c string
		if err := bRows.Scan(&k, &n, &c); err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "internal error")
			return
		}
		bibleList = append(bibleList, fmt.Sprintf("[%s] %s: %s", k, n, headRunes(c, auditBibleRunes)))
	}
	if err := bRows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}

	key, err := s.loadUserLLMKey(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid", "未配置 LLM 模型密钥，请前往设置配置")
		return
	}

	systemPrompt := `你是一名极其严苛的网络小说总编剧与逻辑漏洞质检审计官。
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
}`

	scopeNote := ""
	if totalChapters > len(chapterList) {
		scopeNote = fmt.Sprintf("\n\n（注：全书共 %d 章，因篇幅所限本次仅审计前 %d 章。）", totalChapters, len(chapterList))
	}
	userPrompt := fmt.Sprintf("【全书章节大纲与内容摘要】\n%s\n\n【世界设定与人物档案】\n%s%s",
		strings.Join(chapterList, "\n\n"), strings.Join(bibleList, "\n"), scopeNote,
	)

	llmReq := llm.Request{
		Provider:  key.provider,
		BaseURL:   key.baseURL,
		APIKey:    key.apiKey,
		Model:     resolveModel(req.Model),
		Temp:      0.4,
		MaxTokens: 3500,
		Messages: []llm.Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
	}

	ctx, cancel := context.WithTimeout(r.Context(), 100*time.Second)
	defer cancel()

	respText, err := s.llm.Chat(ctx, llmReq)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "llm_failed", fmt.Sprintf("逻辑雷达审计失败: %v", err))
		return
	}

	cleanJSON := cleanJSONMarkdown(respText)
	var resp continuityAuditResponse
	if err := json.Unmarshal([]byte(cleanJSON), &resp); err != nil {
		writeError(w, http.StatusInternalServerError, "parse_failed", fmt.Sprintf("解析审计结果失败: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, resp)
}
