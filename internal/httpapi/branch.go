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

type branchSimulateRequest struct {
	CurrentText string `json:"current_text"`
	Model       string `json:"model"`
}

type plotBranch struct {
	ID            string   `json:"id"`
	Type          string   `json:"type"`
	Title         string   `json:"title"`
	Direction     string   `json:"direction"`
	PlotPoints    []string `json:"plot_points"`
	SampleOpening string   `json:"sample_opening"`
}

type branchSimulateResponse struct {
	CurrentAnalysis string       `json:"current_analysis"`
	Branches        []plotBranch `json:"branches"`
}

type chapterContinueRequest struct {
	CurrentText string `json:"current_text"`
	Instruction string `json:"instruction"`
	TargetRunes int    `json:"target_runes"`
	Model       string `json:"model"`
}

func (s *Server) handleBranchSimulate(w http.ResponseWriter, r *http.Request) {
	userID, err := s.currentUserID(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}
	chapterID := r.PathValue("id")
	var projectID string
	var seq int
	var title, brief, body, cardID string
	var cardVersion int
	err = s.st.DB().QueryRowContext(
		r.Context(),
		`SELECT c.project_id, c.seq, c.title, c.brief, c.body, c.card_id, c.card_version 
		 FROM chapters c
		 JOIN projects p ON p.id = c.project_id
		 WHERE c.id = ? AND p.user_id = ?`,
		chapterID, userID,
	).Scan(&projectID, &seq, &title, &brief, &body, &cardID, &cardVersion)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "not_found", "chapter not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "invalid", "internal error")
		return
	}

	var req branchSimulateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid", "invalid json")
		return
	}

	activeText := strings.TrimSpace(req.CurrentText)
	if activeText == "" {
		activeText = strings.TrimSpace(body)
	}

	// 收集前文摘要
	var prevSummaries []string
	rows, err := s.st.DB().QueryContext(r.Context(),
		`SELECT seq, title, summary FROM chapters WHERE project_id = ? AND seq < ? ORDER BY seq ASC`,
		projectID, seq,
	)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var sSeq int
			var sTitle, sSummary string
			if err := rows.Scan(&sSeq, &sTitle, &sSummary); err == nil && sSummary != "" {
				prevSummaries = append(prevSummaries, fmt.Sprintf("第%d章《%s》：%s", sSeq, sTitle, sSummary))
			}
		}
	}

	key, err := s.loadUserLLMKey(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid", "未配置 LLM 模型密钥，请前往设置配置")
		return
	}

	systemPrompt := `你是一名擅长情节推演、高潮设计与破除卡文的网文智囊。
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
}`

	userPrompt := fmt.Sprintf("【本章序号与标题】\n第 %d 章：%s\n\n【本章预定大纲梗概】\n%s\n\n【前文梗概回顾】\n%s\n\n【作者当前已写正文节选（末尾）】\n%s",
		seq, title, brief, strings.Join(prevSummaries, "\n"), activeText,
	)

	llmReq := llm.Request{
		Provider:  key.provider,
		BaseURL:   key.baseURL,
		APIKey:    key.apiKey,
		Model:     req.Model,
		Temp:      0.75,
		MaxTokens: 3000,
		Messages: []llm.Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
	}

	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()

	respText, err := s.llm.Chat(ctx, llmReq)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "llm_failed", fmt.Sprintf("剧情推演失败: %v", err))
		return
	}

	cleanJSON := cleanJSONMarkdown(respText)
	var resp branchSimulateResponse
	if err := json.Unmarshal([]byte(cleanJSON), &resp); err != nil {
		writeError(w, http.StatusInternalServerError, "parse_failed", fmt.Sprintf("解析推演结果失败: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleChapterContinue(w http.ResponseWriter, r *http.Request) {
	userID, err := s.currentUserID(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}
	chapterID := r.PathValue("id")
	var projectID string
	var seq int
	var title, brief, cardID string
	var cardVersion int
	err = s.st.DB().QueryRowContext(
		r.Context(),
		`SELECT c.project_id, c.seq, c.title, c.brief, c.card_id, c.card_version 
		 FROM chapters c
		 JOIN projects p ON p.id = c.project_id
		 WHERE c.id = ? AND p.user_id = ?`,
		chapterID, userID,
	).Scan(&projectID, &seq, &title, &brief, &cardID, &cardVersion)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "not_found", "chapter not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "invalid", "internal error")
		return
	}

	var req chapterContinueRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid", "invalid json")
		return
	}

	if req.TargetRunes <= 0 || req.TargetRunes > 3000 {
		req.TargetRunes = 600
	}

	key, err := s.loadUserLLMKey(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid", "未配置 LLM 模型密钥，请前往设置配置")
		return
	}

	systemPrompt := `你是一名专注于网文无缝续写与氛围营造的专业作家。
请根据作者已写的前文、本章梗概以及续写指导要求，自然流畅地承接上文继续写出下一段精彩正文。
要求：
1. 严禁重复前文末尾字句，直接以动作、对话或环境感知起笔无缝向下推进。
2. 保持沉浸感与节奏张力，行文风格考究，段落分明。
3. 严格只输出续写的正文内容，严禁输出任何前言、总结或括号说明。`

	userPrompt := fmt.Sprintf("【章节】第 %d 章：%s\n【本章大纲梗概】\n%s\n\n【续写意图/走向指导】\n%s\n\n【前文末尾内容】\n%s\n\n请直接续写约 %d 字正文：",
		seq, title, brief, req.Instruction, req.CurrentText, req.TargetRunes,
	)

	llmReq := llm.Request{
		Provider:  key.provider,
		BaseURL:   key.baseURL,
		APIKey:    key.apiKey,
		Model:     req.Model,
		Temp:      0.7,
		MaxTokens: 2048,
		Messages: []llm.Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
	}

	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()

	continuedText, err := s.llm.Chat(ctx, llmReq)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "llm_failed", fmt.Sprintf("AI 续写失败: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":             true,
		"continued_text": strings.TrimSpace(continuedText),
	})
}
