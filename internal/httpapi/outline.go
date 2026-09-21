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

	"stylelab/internal/ids"
	"stylelab/internal/llm"
	"stylelab/internal/write"
)

// Outline sizing caps. maxOutlineBriefRunes matches write.maxBriefRunes, which
// is what handleWriteChapter later validates an imported brief against.
const (
	maxOutlineChapters   = 100
	maxOutlineBriefRunes = 200
)

type generateOutlineRequest struct {
	Premise        string `json:"premise"`
	Genre          string `json:"genre"`
	TargetChapters int    `json:"target_chapters"`
	VolumeCount    int    `json:"volume_count"`
	Model          string `json:"model"`
	CardID         string `json:"card_id"`
}

type importOutlineRequest struct {
	Chapters        []outlineChapterItem `json:"chapters"`
	ReplaceExisting bool                 `json:"replace_existing"`
	CardID          string               `json:"card_id"`
}

type outlineChapterItem struct {
	Title string `json:"title"`
	Brief string `json:"brief"`
	Hook  string `json:"hook"`
}

type outlineVolumeItem struct {
	VolumeIndex int                  `json:"volume_index"`
	VolumeTitle string               `json:"volume_title"`
	VolumeBrief string               `json:"volume_brief"`
	Chapters    []outlineChapterItem `json:"chapters"`
}

type generateOutlineResponse struct {
	Synopsis string              `json:"synopsis"`
	Volumes  []outlineVolumeItem `json:"volumes"`
}

func (s *Server) handleGenerateOutline(w http.ResponseWriter, r *http.Request) {
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

	var req generateOutlineRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid", "invalid json")
		return
	}

	premise := strings.TrimSpace(req.Premise)
	if premise == "" {
		writeError(w, http.StatusBadRequest, "invalid", "故事核心梗概 (premise) 不能为空")
		return
	}
	if req.TargetChapters <= 0 || req.TargetChapters > maxOutlineChapters {
		req.TargetChapters = 15
	}
	if req.VolumeCount <= 0 {
		req.VolumeCount = 2
	}
	if req.Genre == "" {
		req.Genre = "玄幻修真/都市异能/科幻末世"
	}

	key, err := s.loadUserLLMKey(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid", "未配置 LLM 模型密钥，请前往设置配置")
		return
	}

	// 收集已有设定集辅助
	var bibleSnippets []string
	bRows, err := s.st.DB().QueryContext(r.Context(),
		`SELECT kind, name, content FROM bible_entries WHERE project_id = ? AND status = 'active' LIMIT 15`,
		projectID,
	)
	if err == nil {
		defer bRows.Close()
		for bRows.Next() {
			var k, n, c string
			if err := bRows.Scan(&k, &n, &c); err == nil {
				bibleSnippets = append(bibleSnippets, fmt.Sprintf("[%s] %s: %s", k, n, c))
			}
		}
	}
	bibleContext := ""
	if len(bibleSnippets) > 0 {
		bibleContext = "\n\n现有世界观设定参考：\n" + strings.Join(bibleSnippets, "\n")
	}

	systemPrompt := `你是一名拥有千万字白金作家水准的网络小说总策划与金牌主编。
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
}`

	userPrompt := fmt.Sprintf("【故事核心梗概】\n%s\n\n【题材类型】\n%s\n\n【规划参数】\n目标总章节数：%d 章，分卷数：%d 卷%s",
		premise, req.Genre, req.TargetChapters, req.VolumeCount, bibleContext,
	)

	llmReq := llm.Request{
		Provider:  key.provider,
		BaseURL:   key.baseURL,
		APIKey:    key.apiKey,
		Model:     resolveModel(req.Model),
		Temp:      0.7,
		MaxTokens: 4096,
		Messages: []llm.Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
	}

	ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
	defer cancel()

	respText, err := s.llm.Chat(ctx, llmReq)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "llm_failed", fmt.Sprintf("AI 生成大纲失败: %v", err))
		return
	}

	cleanJSON := cleanJSONMarkdown(respText)
	var outline generateOutlineResponse
	if err := json.Unmarshal([]byte(cleanJSON), &outline); err != nil {
		writeError(w, http.StatusInternalServerError, "parse_failed", fmt.Sprintf("解析大纲失败: %v (原始响应: %s)", err, respText))
		return
	}

	writeJSON(w, http.StatusOK, outline)
}

func (s *Server) handleImportOutline(w http.ResponseWriter, r *http.Request) {
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

	var req importOutlineRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid", "invalid json")
		return
	}
	if len(req.Chapters) == 0 {
		writeError(w, http.StatusBadRequest, "invalid", "chapters list is empty")
		return
	}
	if len(req.Chapters) > maxOutlineChapters {
		writeError(w, http.StatusBadRequest, "invalid", fmt.Sprintf("一次最多导入 %d 章", maxOutlineChapters))
		return
	}

	cardID := strings.TrimSpace(req.CardID)
	cardVersion := 0
	if cardID != "" {
		c, err := s.loadOwnedCard(r, userID, cardID, 0)
		if err != nil {
			writeCardError(w, err)
			return
		}
		if c.ProjectID != projectID {
			writeError(w, http.StatusNotFound, "not_found", "not found")
			return
		}
		cardID = c.ID
		cardVersion = c.Version
	}

	// Validate every chapter before opening the transaction so a bad item
	// cannot leave a half-imported outline behind. An empty brief matters:
	// handleWriteChapter rejects one, so such a chapter could never be written.
	type preparedChapter struct{ title, brief string }
	prepared := make([]preparedChapter, 0, len(req.Chapters))
	for i, ch := range req.Chapters {
		title, err := write.ValidateTitle(ch.Title)
		if err != nil {
			title = fmt.Sprintf("第%d章", i+1)
		}
		brief := strings.TrimSpace(ch.Brief)
		if hook := strings.TrimSpace(ch.Hook); hook != "" && !strings.Contains(brief, hook) {
			brief = strings.TrimSpace(brief + " (伏笔: " + hook + ")")
		}
		// Clamp before validating: an over-long brief from the model is worth
		// truncating, but an empty one is a hard error.
		brief, err = write.ValidateBrief(headRunes(brief, maxOutlineBriefRunes))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid", fmt.Sprintf("第 %d 章缺少章节梗概", i+1))
			return
		}
		prepared = append(prepared, preparedChapter{title: title, brief: brief})
	}

	tx, err := s.st.DB().BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	defer tx.Rollback()

	// "Replace" must never discard finished work: drop only chapters that carry
	// no prose. Surviving chapters keep their seq, so the import appends after
	// them -- chapters has UNIQUE (project_id, seq), so renumbering around them
	// is not an option.
	protectedCount := 0
	if req.ReplaceExisting {
		if err := tx.QueryRowContext(r.Context(),
			`SELECT COUNT(*) FROM chapters WHERE project_id = ? AND (status = ? OR TRIM(body) <> '')`,
			projectID, write.StatusWritten,
		).Scan(&protectedCount); err != nil {
			writeError(w, http.StatusInternalServerError, "internal", err.Error())
			return
		}
		if _, err := tx.ExecContext(r.Context(),
			`DELETE FROM chapters WHERE project_id = ? AND status <> ? AND TRIM(body) = ''`,
			projectID, write.StatusWritten,
		); err != nil {
			writeError(w, http.StatusInternalServerError, "internal", err.Error())
			return
		}
	}

	startSeq := 1
	var maxSeq sql.NullInt64
	if err := tx.QueryRowContext(r.Context(), `SELECT MAX(seq) FROM chapters WHERE project_id = ?`, projectID).Scan(&maxSeq); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if maxSeq.Valid {
		startSeq = int(maxSeq.Int64) + 1
	}

	now := time.Now().UTC().Format(time.RFC3339)
	insertedCount := 0

	for i, ch := range prepared {
		seq := startSeq + i
		chapterID := ids.New(write.Prefix)
		_, err := tx.ExecContext(
			r.Context(),
			`INSERT INTO chapters (id, project_id, card_id, card_version, seq, title, brief, body, summary, status, target_runes, model, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, '', '', ?, ?, '', ?, ?)`,
			chapterID, projectID, cardID, cardVersion, seq, ch.title, ch.brief, write.StatusDraft, write.ClampTarget(0), now, now,
		)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", fmt.Sprintf("插入第 %d 章失败: %v", seq, err))
			return
		}
		insertedCount++
	}

	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":              true,
		"inserted_count":  insertedCount,
		"protected_count": protectedCount,
	})
}
