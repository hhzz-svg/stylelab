package studio

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"stylelab/internal/job"
	"stylelab/internal/llm"
	"stylelab/internal/llmkey"
	"stylelab/internal/store"
)

type ContinuityInput struct {
	ProjectID string `json:"project_id"`
	Model     string `json:"model"`
}

type ContinuityIssue struct {
	Severity    string `json:"severity"` // critical | warning | info
	Category    string `json:"category"` // realm | character | foreshadow | artifact
	Title       string `json:"title"`
	Description string `json:"description"`
	Suggestion  string `json:"suggestion"`
	Location    string `json:"location"`
}

type ContinuityResult struct {
	Score       int               `json:"score"`
	Overall     string            `json:"overall"`
	Issues      []ContinuityIssue `json:"issues"`
	Foreshadows []string          `json:"foreshadows"`
}

const (
	continuityTemp      = 0.4
	continuityMaxTokens = 3500
)

// ErrNoChapters is returned when there is nothing to audit.
var ErrNoChapters = fmt.Errorf("invalid: 作品尚无任何章节内容，无法进行伏笔逻辑审计")

func ContinuityJobHandler(st *store.Store, client *llm.Client, master []byte) job.Handler {
	return func(ctx context.Context, rec job.Record, prog func(int, string)) (json.RawMessage, error) {
		var in ContinuityInput
		if err := json.Unmarshal(rec.Payload, &in); err != nil {
			return nil, err
		}
		out, err := RunContinuity(ctx, st, client, master, rec.UserID, in, prog)
		if err != nil {
			return nil, err
		}
		return json.Marshal(out)
	}
}

func RunContinuity(
	ctx context.Context, st *store.Store, client *llm.Client, master []byte,
	userID string, in ContinuityInput, prog func(int, string),
) (ContinuityResult, error) {
	if prog == nil {
		prog = func(int, string) {}
	}
	if err := ownsProject(ctx, st, in.ProjectID, userID); err != nil {
		return ContinuityResult{}, err
	}

	prog(10, "collect")
	chapterList, totalChapters, err := continuityChapters(ctx, st, in.ProjectID)
	if err != nil {
		return ContinuityResult{}, err
	}
	if len(chapterList) == 0 {
		return ContinuityResult{}, ErrNoChapters
	}

	bibleList, err := continuityBible(ctx, st, in.ProjectID)
	if err != nil {
		return ContinuityResult{}, err
	}

	key, err := llmkey.Load(ctx, st, master, userID)
	if err != nil {
		return ContinuityResult{}, err
	}

	scopeNote := ""
	if totalChapters > len(chapterList) {
		scopeNote = fmt.Sprintf("\n\n（注：全书共 %d 章，因篇幅所限本次仅审计前 %d 章。）",
			totalChapters, len(chapterList))
	}
	userPrompt := fmt.Sprintf("【全书章节大纲与内容摘要】\n%s\n\n【世界设定与人物档案】\n%s%s",
		strings.Join(chapterList, "\n\n"), strings.Join(bibleList, "\n"), scopeNote,
	)

	prog(40, "audit")
	respText, err := client.Chat(ctx, llm.Request{
		Provider:  key.Provider,
		BaseURL:   key.BaseURL,
		APIKey:    key.APIKey,
		Model:     resolveModel(in.Model),
		Temp:      continuityTemp,
		MaxTokens: continuityMaxTokens,
		Messages: []llm.Message{
			{Role: "system", Content: llm.ContinuityAuditSystem()},
			{Role: "user", Content: userPrompt},
		},
	})
	if err != nil {
		return ContinuityResult{}, fmt.Errorf("逻辑雷达审计失败: %w", err)
	}

	prog(85, "parse")
	var out ContinuityResult
	if err := json.Unmarshal([]byte(cleanJSONMarkdown(respText)), &out); err != nil {
		return ContinuityResult{}, fmt.Errorf("解析审计结果失败: %w", err)
	}
	prog(100, "done")
	return out, nil
}

// continuityChapters returns at most auditChapterLimit rendered entries plus
// the true chapter count, so the prompt can say it was truncated.
func continuityChapters(ctx context.Context, st *store.Store, projectID string) ([]string, int, error) {
	rows, err := st.DB().QueryContext(ctx,
		`SELECT seq, title, brief, summary, status FROM chapters
		 WHERE project_id = ? ORDER BY seq ASC`,
		projectID,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var list []string
	total := 0
	for rows.Next() {
		var seq int
		var title, brief, summary, status string
		if err := rows.Scan(&seq, &title, &brief, &summary, &status); err != nil {
			return nil, 0, err
		}
		total++
		if len(list) >= auditChapterLimit {
			continue
		}
		content := summary
		if content == "" {
			content = brief
		}
		list = append(list, fmt.Sprintf("【第%d章 %s】(状态:%s)\n梗概与摘要：%s",
			seq, title, status, headRunes(content, auditChapterRunes)))
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func continuityBible(ctx context.Context, st *store.Store, projectID string) ([]string, error) {
	rows, err := st.DB().QueryContext(ctx,
		`SELECT kind, name, content FROM bible_entries
		 WHERE project_id = ? AND status = 'active' LIMIT ?`,
		projectID, auditBibleLimit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []string
	for rows.Next() {
		var k, n, c string
		if err := rows.Scan(&k, &n, &c); err != nil {
			return nil, err
		}
		list = append(list, fmt.Sprintf("[%s] %s: %s", k, n, headRunes(c, auditBibleRunes)))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return list, nil
}
