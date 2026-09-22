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

type BranchInput struct {
	ChapterID   string `json:"chapter_id"`
	CurrentText string `json:"current_text"`
	Model       string `json:"model"`
}

type PlotBranch struct {
	ID            string   `json:"id"`
	Type          string   `json:"type"`
	Title         string   `json:"title"`
	Direction     string   `json:"direction"`
	PlotPoints    []string `json:"plot_points"`
	SampleOpening string   `json:"sample_opening"`
}

type BranchResult struct {
	CurrentAnalysis string       `json:"current_analysis"`
	Branches        []PlotBranch `json:"branches"`
}

type ContinueInput struct {
	ChapterID   string `json:"chapter_id"`
	CurrentText string `json:"current_text"`
	Instruction string `json:"instruction"`
	TargetRunes int    `json:"target_runes"`
	Model       string `json:"model"`
}

type ContinueResult struct {
	ContinuedText string `json:"continued_text"`
}

const (
	branchTemp          = 0.75
	branchMaxTokens     = 3000
	continueTemp        = 0.7
	continueMaxTokens   = 2048
	continueTargetMin   = 1
	continueTargetMax   = 3000
	continueTargetPlain = 600
)

// chapterCtx is the chapter row both branch features read.
type chapterCtx struct {
	ProjectID string
	Seq       int
	Title     string
	Brief     string
	Body      string
	Model     string
}

// loadOwnedChapter scopes by projects.user_id so another user's chapter is
// indistinguishable from a missing one.
func loadOwnedChapter(ctx context.Context, st *store.Store, chapterID, userID string) (chapterCtx, error) {
	var c chapterCtx
	err := st.DB().QueryRowContext(ctx,
		`SELECT c.project_id, c.seq, c.title, c.brief, c.body, c.model
		 FROM chapters c
		 JOIN projects p ON p.id = c.project_id
		 WHERE c.id = ? AND p.user_id = ?`,
		chapterID, userID,
	).Scan(&c.ProjectID, &c.Seq, &c.Title, &c.Brief, &c.Body, &c.Model)
	return c, err
}

// prevSummaries returns the most recent non-empty summaries before seq, in
// reading order.
func prevSummaries(ctx context.Context, st *store.Store, projectID string, seq int) ([]string, error) {
	rows, err := st.DB().QueryContext(ctx,
		`SELECT seq, title, summary FROM chapters
		 WHERE project_id = ? AND seq < ? AND TRIM(summary) <> ''
		 ORDER BY seq DESC LIMIT ?`,
		projectID, seq, branchPrevSummaryLimit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var sSeq int
		var sTitle, sSummary string
		if err := rows.Scan(&sSeq, &sTitle, &sSummary); err != nil {
			return nil, err
		}
		out = append(out, fmt.Sprintf("第%d章《%s》：%s", sSeq, sTitle, sSummary))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// The query walks backwards to take the newest; the prompt wants oldest first.
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}

func BranchJobHandler(st *store.Store, client *llm.Client, master []byte) job.Handler {
	return func(ctx context.Context, rec job.Record, prog func(int, string)) (json.RawMessage, error) {
		var in BranchInput
		if err := json.Unmarshal(rec.Payload, &in); err != nil {
			return nil, err
		}
		out, err := RunBranch(ctx, st, client, master, rec.UserID, in, prog)
		if err != nil {
			return nil, err
		}
		return json.Marshal(out)
	}
}

func RunBranch(
	ctx context.Context, st *store.Store, client *llm.Client, master []byte,
	userID string, in BranchInput, prog func(int, string),
) (BranchResult, error) {
	if prog == nil {
		prog = func(int, string) {}
	}
	ch, err := loadOwnedChapter(ctx, st, in.ChapterID, userID)
	if err != nil {
		return BranchResult{}, err
	}

	activeText := strings.TrimSpace(in.CurrentText)
	if activeText == "" {
		activeText = strings.TrimSpace(ch.Body)
	}
	// Only the tail decides where the plot goes next, and the client posts its
	// entire editor buffer.
	activeText = tailRunes(activeText, branchTailRunes)

	prog(15, "collect")
	prev, err := prevSummaries(ctx, st, ch.ProjectID, ch.Seq)
	if err != nil {
		return BranchResult{}, err
	}

	key, err := loadUserKey(ctx, st, master, userID)
	if err != nil {
		return BranchResult{}, err
	}

	userPrompt := fmt.Sprintf(
		"【本章序号与标题】\n第 %d 章：%s\n\n【本章预定大纲梗概】\n%s\n\n【前文梗概回顾】\n%s\n\n【作者当前已写正文节选（末尾）】\n%s",
		ch.Seq, ch.Title, ch.Brief, strings.Join(prev, "\n"), activeText,
	)

	prog(40, "simulate")
	respText, err := client.Chat(ctx, llm.Request{
		Provider:  key.Provider,
		BaseURL:   key.BaseURL,
		APIKey:    key.APIKey,
		Model:     resolveModel(in.Model, ch.Model),
		Temp:      branchTemp,
		MaxTokens: branchMaxTokens,
		Messages: []llm.Message{
			{Role: "system", Content: llm.BranchSimulateSystem()},
			{Role: "user", Content: userPrompt},
		},
	})
	if err != nil {
		return BranchResult{}, fmt.Errorf("情节推演失败: %w", err)
	}

	prog(85, "parse")
	var out BranchResult
	if err := json.Unmarshal([]byte(cleanJSONMarkdown(respText)), &out); err != nil {
		return BranchResult{}, fmt.Errorf("解析推演结果失败: %w", err)
	}
	prog(100, "done")
	return out, nil
}

// ClampContinueTarget keeps the requested length inside the band the prompt is
// written for.
func ClampContinueTarget(n int) int {
	if n < continueTargetMin || n > continueTargetMax {
		return continueTargetPlain
	}
	return n
}

func ContinueJobHandler(st *store.Store, client *llm.Client, master []byte) job.Handler {
	return func(ctx context.Context, rec job.Record, prog func(int, string)) (json.RawMessage, error) {
		var in ContinueInput
		if err := json.Unmarshal(rec.Payload, &in); err != nil {
			return nil, err
		}
		out, err := RunContinue(ctx, st, client, master, rec.UserID, in, prog)
		if err != nil {
			return nil, err
		}
		return json.Marshal(out)
	}
}

func RunContinue(
	ctx context.Context, st *store.Store, client *llm.Client, master []byte,
	userID string, in ContinueInput, prog func(int, string),
) (ContinueResult, error) {
	if prog == nil {
		prog = func(int, string) {}
	}
	ch, err := loadOwnedChapter(ctx, st, in.ChapterID, userID)
	if err != nil {
		return ContinueResult{}, err
	}

	target := ClampContinueTarget(in.TargetRunes)
	// The client posts its whole editor buffer; only the tail is continued
	// from, and a runaway instruction should not crowd out the text.
	currentText := tailRunes(strings.TrimSpace(in.CurrentText), branchTailRunes)
	instruction := headRunes(strings.TrimSpace(in.Instruction), continueInstructionMax)

	key, err := loadUserKey(ctx, st, master, userID)
	if err != nil {
		return ContinueResult{}, err
	}

	userPrompt := fmt.Sprintf(
		"【章节】第 %d 章：%s\n【本章大纲梗概】\n%s\n\n【续写意图/走向指导】\n%s\n\n【前文末尾内容】\n%s\n\n请直接续写约 %d 字正文：",
		ch.Seq, ch.Title, ch.Brief, instruction, currentText, target,
	)

	prog(40, "write")
	respText, err := client.Chat(ctx, llm.Request{
		Provider:  key.Provider,
		BaseURL:   key.BaseURL,
		APIKey:    key.APIKey,
		Model:     resolveModel(in.Model, ch.Model),
		Temp:      continueTemp,
		MaxTokens: continueMaxTokens,
		Messages: []llm.Message{
			{Role: "system", Content: llm.ChapterContinueSystem()},
			{Role: "user", Content: userPrompt},
		},
	})
	if err != nil {
		return ContinueResult{}, fmt.Errorf("续写失败: %w", err)
	}

	prog(100, "done")
	return ContinueResult{ContinuedText: strings.TrimSpace(respText)}, nil
}
