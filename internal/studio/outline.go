package studio

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"stylelab/internal/job"
	"stylelab/internal/llm"
	"stylelab/internal/store"
)

// OutlineInput is the job payload for outline generation.
type OutlineInput struct {
	ProjectID      string `json:"project_id"`
	Premise        string `json:"premise"`
	Genre          string `json:"genre"`
	TargetChapters int    `json:"target_chapters"`
	VolumeCount    int    `json:"volume_count"`
	Model          string `json:"model"`
}

type OutlineChapterItem struct {
	Title string `json:"title"`
	Brief string `json:"brief"`
	Hook  string `json:"hook"`
}

type OutlineVolumeItem struct {
	VolumeIndex int                  `json:"volume_index"`
	VolumeTitle string               `json:"volume_title"`
	VolumeBrief string               `json:"volume_brief"`
	Chapters    []OutlineChapterItem `json:"chapters"`
}

// OutlineResult is both the parsed model reply and the job's stored result.
type OutlineResult struct {
	Synopsis string              `json:"synopsis"`
	Volumes  []OutlineVolumeItem `json:"volumes"`
}

const (
	MaxOutlineChapters = 100
	defaultGenre       = "玄幻修真/都市异能/科幻末世"
	outlineTemp        = 0.7
	outlineMaxTokens   = 4096
)

// ValidateOutlineInput normalises the request and reports anything the caller
// must fix. Run at enqueue time so the user gets a 400 instead of a job that
// fails a minute later.
func ValidateOutlineInput(in *OutlineInput) error {
	in.Premise = strings.TrimSpace(in.Premise)
	if in.Premise == "" {
		return fmt.Errorf("invalid: 故事核心梗概 (premise) 不能为空")
	}
	if in.TargetChapters <= 0 || in.TargetChapters > MaxOutlineChapters {
		in.TargetChapters = 15
	}
	if in.VolumeCount <= 0 {
		in.VolumeCount = 2
	}
	if strings.TrimSpace(in.Genre) == "" {
		in.Genre = defaultGenre
	}
	return nil
}

func OutlineJobHandler(st *store.Store, client *llm.Client, master []byte) job.Handler {
	return func(ctx context.Context, rec job.Record, prog func(int, string)) (json.RawMessage, error) {
		var in OutlineInput
		if err := json.Unmarshal(rec.Payload, &in); err != nil {
			return nil, err
		}
		out, err := RunOutline(ctx, st, client, master, rec.UserID, in, prog)
		if err != nil {
			return nil, err
		}
		return json.Marshal(out)
	}
}

func RunOutline(
	ctx context.Context, st *store.Store, client *llm.Client, master []byte,
	userID string, in OutlineInput, prog func(int, string),
) (OutlineResult, error) {
	if prog == nil {
		prog = func(int, string) {}
	}
	if err := ValidateOutlineInput(&in); err != nil {
		return OutlineResult{}, err
	}
	if err := ownsProject(ctx, st, in.ProjectID, userID); err != nil {
		return OutlineResult{}, err
	}

	prog(10, "collect")
	bibleContext, err := outlineBibleContext(ctx, st, in.ProjectID)
	if err != nil {
		return OutlineResult{}, err
	}

	key, err := loadUserKey(ctx, st, master, userID)
	if err != nil {
		return OutlineResult{}, err
	}

	userPrompt := fmt.Sprintf(
		"【故事核心梗概】\n%s\n\n【题材类型】\n%s\n\n【规划参数】\n目标总章节数：%d 章，分卷数：%d 卷%s",
		in.Premise, in.Genre, in.TargetChapters, in.VolumeCount, bibleContext,
	)

	prog(35, "generate")
	respText, err := client.Chat(ctx, llm.Request{
		Provider:  key.Provider,
		BaseURL:   key.BaseURL,
		APIKey:    key.APIKey,
		Model:     resolveModel(in.Model),
		Temp:      outlineTemp,
		MaxTokens: outlineMaxTokens,
		Messages: []llm.Message{
			{Role: "system", Content: llm.OutlineSystem()},
			{Role: "user", Content: userPrompt},
		},
	})
	if err != nil {
		return OutlineResult{}, fmt.Errorf("AI 生成大纲失败: %w", err)
	}

	prog(85, "parse")
	var out OutlineResult
	if err := json.Unmarshal([]byte(cleanJSONMarkdown(respText)), &out); err != nil {
		return OutlineResult{}, fmt.Errorf("解析大纲失败: %w", err)
	}
	prog(100, "done")
	return out, nil
}

func outlineBibleContext(ctx context.Context, st *store.Store, projectID string) (string, error) {
	rows, err := st.DB().QueryContext(ctx,
		`SELECT kind, name, content FROM bible_entries
		 WHERE project_id = ? AND status = 'active' LIMIT ?`,
		projectID, outlineBibleLimit,
	)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	var snippets []string
	for rows.Next() {
		var k, n, c string
		if err := rows.Scan(&k, &n, &c); err != nil {
			return "", err
		}
		snippets = append(snippets, fmt.Sprintf("[%s] %s: %s", k, n, c))
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	if len(snippets) == 0 {
		return "", nil
	}
	return "\n\n现有世界观设定参考：\n" + strings.Join(snippets, "\n"), nil
}
