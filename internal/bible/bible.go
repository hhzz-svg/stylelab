package bible

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"stylelab/internal/ids"
	"stylelab/internal/llm"
	"stylelab/internal/store"
)

const (
	Prefix = "bib_"

	KindCharacter = "character"
	KindSetting    = "setting"
	KindThread     = "thread"

	StatusActive   = "active"
	StatusResolved = "resolved"

	OriginManual = "manual"
	OriginAuto   = "auto"

	maxNameRunes      = 40
	maxContentRunes   = 2000
	maxOps            = 30
	clampContentRunes = 400
	summaryRunes      = 80
	promptEntryRunes  = 240
	promptTotalRunes  = 4000
	syncKeyRunes      = 200
	syncChatTemp      = 0.2
	syncMaxTokens     = 2500
	syncParseRetries  = 2
	defaultSyncModel  = "gpt-4o-mini"
)

type Entry struct {
	ID        string `json:"id"`
	ProjectID string `json:"project_id"`
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Content   string `json:"content"`
	Status    string `json:"status"`
	Origin    string `json:"origin"`
	SourceSeq int    `json:"source_seq"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type EntrySummary struct {
	ID              string `json:"id"`
	Kind            string `json:"kind"`
	Name            string `json:"name"`
	Status          string `json:"status"`
	Origin          string `json:"origin"`
	SourceSeq       int    `json:"source_seq"`
	ContentPreview  string `json:"content_preview"`
	UpdatedAt       string `json:"updated_at"`
}

// Op 是设定集的一次 AI 维护操作。AI 只能新增或更新，不能删除。
type Op struct {
	Op      string `json:"op"`
	ID      string `json:"id,omitempty"`
	Kind    string `json:"kind,omitempty"`
	Name    string `json:"name,omitempty"`
	Content string `json:"content,omitempty"`
	Status  string `json:"status,omitempty"`
}

type Key struct {
	Provider string
	BaseURL  string
	APIKey   string
}

func ValidateKind(kind string) (string, error) {
	switch strings.TrimSpace(kind) {
	case KindCharacter:
		return KindCharacter, nil
	case KindSetting:
		return KindSetting, nil
	case KindThread:
		return KindThread, nil
	}
	return "", fmt.Errorf("invalid: kind must be character, setting, or thread")
}

func ValidateStatus(status string) (string, error) {
	switch strings.TrimSpace(status) {
	case StatusActive:
		return StatusActive, nil
	case StatusResolved:
		return StatusResolved, nil
	}
	return "", fmt.Errorf("invalid: status must be active or resolved")
}

func ValidateName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("invalid: name required")
	}
	if utf8.RuneCountInString(name) > maxNameRunes {
		return "", fmt.Errorf("invalid: name too long")
	}
	return name, nil
}

func ValidateContent(content string) (string, error) {
	content = strings.TrimSpace(content)
	if utf8.RuneCountInString(content) > maxContentRunes {
		return "", fmt.Errorf("invalid: content too long")
	}
	return content, nil
}

func clampRunes(s string, n int) string {
	rs := []rune(strings.TrimSpace(s))
	if len(rs) <= n {
		return string(rs)
	}
	return string(rs[:n])
}

func ListByProject(ctx context.Context, st *store.Store, userID, projectID string) ([]Entry, error) {
	rows, err := st.DB().QueryContext(
		ctx,
		`SELECT e.id, e.project_id, e.kind, e.name, e.content, e.status, e.origin, e.source_seq, e.created_at, e.updated_at
		 FROM bible_entries e
		 JOIN projects p ON p.id = e.project_id
		 WHERE e.project_id = ? AND p.user_id = ?
		 ORDER BY CASE e.kind WHEN 'character' THEN 1 WHEN 'setting' THEN 2 ELSE 3 END, e.name COLLATE NOCASE`,
		projectID, userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Entry
	for rows.Next() {
		var e Entry
		if err := rows.Scan(&e.ID, &e.ProjectID, &e.Kind, &e.Name, &e.Content, &e.Status, &e.Origin, &e.SourceSeq, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func ListSummaries(ctx context.Context, st *store.Store, userID, projectID string) ([]EntrySummary, error) {
	entries, err := ListByProject(ctx, st, userID, projectID)
	if err != nil {
		return nil, err
	}
	out := make([]EntrySummary, 0, len(entries))
	for _, e := range entries {
		out = append(out, EntrySummary{
			ID:             e.ID,
			Kind:           e.Kind,
			Name:           e.Name,
			Status:         e.Status,
			Origin:         e.Origin,
			SourceSeq:      e.SourceSeq,
			ContentPreview: clampRunes(e.Content, summaryRunes),
			UpdatedAt:      e.UpdatedAt,
		})
	}
	return out, nil
}

func LoadOwned(ctx context.Context, st *store.Store, userID, entryID string) (Entry, error) {
	var e Entry
	err := st.DB().QueryRowContext(
		ctx,
		`SELECT e.id, e.project_id, e.kind, e.name, e.content, e.status, e.origin, e.source_seq, e.created_at, e.updated_at
		 FROM bible_entries e
		 JOIN projects p ON p.id = e.project_id
		 WHERE e.id = ? AND p.user_id = ?`,
		entryID, userID,
	).Scan(&e.ID, &e.ProjectID, &e.Kind, &e.Name, &e.Content, &e.Status, &e.Origin, &e.SourceSeq, &e.CreatedAt, &e.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return Entry{}, fmt.Errorf("not found")
		}
		return Entry{}, err
	}
	return e, nil
}

func Insert(ctx context.Context, st *store.Store, e Entry) error {
	_, err := st.DB().ExecContext(
		ctx,
		`INSERT INTO bible_entries (id, project_id, kind, name, content, status, origin, source_seq, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.ID, e.ProjectID, e.Kind, e.Name, e.Content, e.Status, e.Origin, e.SourceSeq, e.CreatedAt, e.UpdatedAt,
	)
	return err
}

func UpdateFields(ctx context.Context, st *store.Store, e Entry) error {
	_, err := st.DB().ExecContext(
		ctx,
		`UPDATE bible_entries SET kind=?, name=?, content=?, status=?, origin=?, source_seq=?, updated_at=?
		 WHERE id=?`,
		e.Kind, e.Name, e.Content, e.Status, e.Origin, e.SourceSeq, e.UpdatedAt, e.ID,
	)
	return err
}

func Delete(ctx context.Context, st *store.Store, userID, entryID string) error {
	res, err := st.DB().ExecContext(
		ctx,
		`DELETE FROM bible_entries
		 WHERE id=? AND project_id IN (SELECT id FROM projects WHERE user_id=?)`,
		entryID, userID,
	)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("not found")
	}
	return nil
}

// RenderForPrompt 把设定集渲染成注入章节 user prompt 的文本块。
// 按人物/设定/伏笔分组，每条截断到 promptEntryRunes 字，整块上限 promptTotalRunes 字；
// 已收线的伏笔仍注入（标注），避免后文误把已收的线头再写活。空设定集返回空串。
func RenderForPrompt(entries []Entry) string {
	type section struct {
		title   string
		entries []Entry
	}
	sections := []section{
		{title: "【人物】"},
		{title: "【设定】"},
		{title: "【伏笔】"},
	}
	byKind := map[string]int{KindCharacter: 0, KindSetting: 1, KindThread: 2}
	for _, e := range entries {
		idx, ok := byKind[e.Kind]
		if !ok {
			continue
		}
		sections[idx].entries = append(sections[idx].entries, e)
	}

	var b strings.Builder
	wrote := 0
	for _, sec := range sections {
		if len(sec.entries) == 0 {
			continue
		}
		if wrote == 0 {
			b.WriteString("设定集（既定事实，写作时保持一致，勿在正文中复述这些条目）：\n")
		}
		b.WriteString(sec.title)
		b.WriteString("\n")
		for _, e := range sec.entries {
			line := "- " + e.Name + "：" + clampRunes(e.Content, promptEntryRunes)
			if e.Kind == KindThread && e.Status == StatusResolved {
				line = "- " + e.Name + "（已收线）：" + clampRunes(e.Content, promptEntryRunes)
			}
			if wrote+utf8.RuneCountInString(line) > promptTotalRunes {
				return b.String()
			}
			b.WriteString(line)
			b.WriteString("\n")
			wrote += utf8.RuneCountInString(line)
		}
	}
	return b.String()
}

// SyncChapter 用 LLM 从一章正文提取设定集维护操作。
func SyncChapter(ctx context.Context, client *llm.Client, key Key, model string, entries []Entry, seq int, title, body string) ([]Op, error) {
	if strings.TrimSpace(model) == "" {
		model = defaultSyncModel
	}
	req := llm.Request{
		Provider: key.Provider,
		BaseURL:  key.BaseURL,
		APIKey:   key.APIKey,
		Model:    model,
		Temp:     syncChatTemp,
		MaxTokens: syncMaxTokens,
		Messages: []llm.Message{
			{Role: "system", Content: llm.BibleSyncSystem()},
			{Role: "user", Content: SyncUserPrompt(entries, seq, title, body)},
		},
	}
	var lastErr error
	for attempt := 0; attempt < 1+syncParseRetries; attempt++ {
		raw, err := client.Chat(ctx, req)
		if err != nil {
			return nil, err
		}
		ops, err := ParseOps(raw)
		if err == nil {
			return ops, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

// SyncUserPrompt 组装设定集同步的 user prompt（导出供测试断言注入内容）。
func SyncUserPrompt(entries []Entry, seq int, title, body string) string {
	var b strings.Builder
	b.WriteString("设定集现状：\n")
	if len(entries) == 0 {
		b.WriteString("（空）\n")
	} else {
		type slim struct {
			ID      string `json:"id"`
			Kind    string `json:"kind"`
			Name    string `json:"name"`
			Content string `json:"content"`
			Status  string `json:"status"`
		}
		list := make([]slim, 0, len(entries))
		for _, e := range entries {
			list = append(list, slim{ID: e.ID, Kind: e.Kind, Name: e.Name, Content: clampRunes(e.Content, syncKeyRunes), Status: e.Status})
		}
		enc, err := json.Marshal(list)
		if err != nil {
			b.WriteString("（空）\n")
		} else {
			b.Write(enc)
			b.WriteString("\n")
		}
	}
	fmt.Fprintf(&b, "\n第 %d 章《%s》正文：\n%s\n", seq, title, body)
	return b.String()
}

// ParseOps 解析 LLM 输出的 ops JSON；非法条目被丢弃而不是报错，
// 只有整体不是合法 JSON 时才失败（触发上层重试）。
func ParseOps(raw string) ([]Op, error) {
	payload := extractJSONObject(raw)
	var out struct {
		Ops []Op `json:"ops"`
	}
	if err := json.Unmarshal(payload, &out); err != nil {
		return nil, fmt.Errorf("invalid: bible ops json: %w", err)
	}
	ops := make([]Op, 0, len(out.Ops))
	for _, op := range out.Ops {
		switch strings.TrimSpace(op.Op) {
		case "create":
			kind, err := ValidateKind(op.Kind)
			if err != nil {
				continue
			}
			name, err := ValidateName(op.Name)
			if err != nil {
				continue
			}
			op.Kind = kind
			op.Name = name
			op.Content = strings.TrimSpace(op.Content)
			op.Status = ""
			ops = append(ops, op)
		case "update":
			if strings.TrimSpace(op.ID) == "" || !ids.Valid(strings.TrimSpace(op.ID), Prefix) {
				continue
			}
			op.ID = strings.TrimSpace(op.ID)
			op.Content = strings.TrimSpace(op.Content)
			if op.Status != "" {
				status, err := ValidateStatus(op.Status)
				if err != nil {
					op.Status = ""
				} else {
					op.Status = status
				}
			}
			ops = append(ops, op)
		}
	}
	return ops, nil
}

func extractJSONObject(raw string) []byte {
	s := strings.TrimSpace(raw)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```JSON")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	s = strings.TrimSpace(s)
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start >= 0 && end > start {
		s = s[start : end+1]
	}
	return bytes.TrimSpace([]byte(s))
}

// ApplyOps 在一个事务里执行 AI 维护操作：create 遇到同名同类的既有条目自动降级为
// update（防重复建条），update 只接受本项目已有 id，AI 写入的 content 钳到
// clampContentRunes 字。返回 (新建数, 更新数)。
func ApplyOps(ctx context.Context, st *store.Store, userID, projectID string, seq int, ops []Op) (int, int, error) {
	if len(ops) > maxOps {
		ops = ops[:maxOps]
	}
	existing, err := ListByProject(ctx, st, userID, projectID)
	if err != nil {
		return 0, 0, err
	}
	byID := make(map[string]Entry, len(existing))
	byName := make(map[string]Entry, len(existing))
	for _, e := range existing {
		byID[e.ID] = e
		byName[e.Kind+"\x00"+e.Name] = e
	}

	tx, err := st.DB().BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, err
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now().UTC().Format(time.RFC3339)
	created, updated := 0, 0
	for _, op := range ops {
		switch op.Op {
		case "create":
			if prev, ok := byName[op.Kind+"\x00"+op.Name]; ok {
				// 同名同类：降级为更新，保留原 id 与状态
				if _, err := tx.ExecContext(
					ctx,
					`UPDATE bible_entries SET content=?, origin=?, source_seq=?, updated_at=? WHERE id=?`,
					clampRunes(op.Content, clampContentRunes), OriginAuto, seq, now, prev.ID,
				); err != nil {
					return 0, 0, err
				}
				updated++
				continue
			}
			e := Entry{
				ID:        ids.New(Prefix),
				ProjectID: projectID,
				Kind:      op.Kind,
				Name:      op.Name,
				Content:   clampRunes(op.Content, clampContentRunes),
				Status:    StatusActive,
				Origin:    OriginAuto,
				SourceSeq: seq,
				CreatedAt: now,
				UpdatedAt: now,
			}
			if _, err := tx.ExecContext(
				ctx,
				`INSERT INTO bible_entries (id, project_id, kind, name, content, status, origin, source_seq, created_at, updated_at)
				 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				e.ID, e.ProjectID, e.Kind, e.Name, e.Content, e.Status, e.Origin, e.SourceSeq, e.CreatedAt, e.UpdatedAt,
			); err != nil {
				return 0, 0, err
			}
			byID[e.ID] = e
			byName[e.Kind+"\x00"+e.Name] = e
			created++
		case "update":
			prev, ok := byID[op.ID]
			if !ok {
				continue
			}
			content := prev.Content
			if op.Content != "" {
				content = clampRunes(op.Content, clampContentRunes)
			}
			status := prev.Status
			if op.Status != "" {
				status = op.Status
			}
			if _, err := tx.ExecContext(
				ctx,
				`UPDATE bible_entries SET content=?, status=?, origin=?, source_seq=?, updated_at=? WHERE id=?`,
				content, status, OriginAuto, seq, now, prev.ID,
			); err != nil {
				return 0, 0, err
			}
			updated++
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, 0, err
	}
	return created, updated, nil
}
