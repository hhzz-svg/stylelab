package write

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"stylelab/internal/bible"
	"stylelab/internal/card"
	"stylelab/internal/job"
	"stylelab/internal/llm"
	"stylelab/internal/llmkey"
	"stylelab/internal/lore"
	"stylelab/internal/store"
)

const (
	Prefix             = "chp_"
	StatusDraft        = "draft"
	StatusWriting      = "writing"
	StatusWritten      = "written"
	StatusFailed       = "failed"
	defaultTargetRunes = 2500
	minTargetRunes     = 2000
	maxTargetRunes     = 3500
	maxBriefRunes      = 200
	maxNoteRunes       = 200
	maxTitleRunes      = 40
	maxSummaryRunes    = 120
	prevTailRunes      = 300
	prevSummaryLimit   = 3
	defaultWriteModel  = "gpt-4o-mini"
	writeChatTemp      = 0.75
	writeMaxTokens     = 8192
	summaryMaxTokens   = 400
)

type Input struct {
	ChapterID string `json:"chapter_id"`
	Model     string `json:"model"`
	Target    int    `json:"target_runes"`
	Note      string `json:"note"`
}

type Chapter struct {
	ID          string `json:"id"`
	ProjectID   string `json:"project_id"`
	CardID      string `json:"card_id"`
	CardVersion int    `json:"card_version"`
	Seq         int    `json:"seq"`
	Title       string `json:"title"`
	Brief       string `json:"brief"`
	Body        string `json:"body"`
	Summary     string `json:"summary"`
	Status      string `json:"status"`
	TargetRunes int    `json:"target_runes"`
	Model       string `json:"model"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type ChapterSummary struct {
	ID          string `json:"id"`
	Seq         int    `json:"seq"`
	Title       string `json:"title"`
	Brief       string `json:"brief"`
	Status      string `json:"status"`
	RuneCount   int    `json:"rune_count"`
	HasSummary  bool   `json:"has_summary"`
	CardID      string `json:"card_id"`
	TargetRunes int    `json:"target_runes"`
	UpdatedAt   string `json:"updated_at"`
}

func ClampTarget(n int) int {
	if n == 0 {
		return defaultTargetRunes
	}
	if n < minTargetRunes {
		return minTargetRunes
	}
	if n > maxTargetRunes {
		return maxTargetRunes
	}
	return n
}

func ValidateTitle(title string) (string, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return "", fmt.Errorf("invalid: title required")
	}
	if utf8.RuneCountInString(title) > maxTitleRunes {
		return "", fmt.Errorf("invalid: title too long")
	}
	return title, nil
}

func ValidateBrief(brief string) (string, error) {
	brief = strings.TrimSpace(brief)
	if brief == "" {
		return "", fmt.Errorf("invalid: brief required")
	}
	if utf8.RuneCountInString(brief) > maxBriefRunes {
		return "", fmt.Errorf("invalid: brief too long")
	}
	return brief, nil
}

func ValidateNote(note string) (string, error) {
	note = strings.TrimSpace(note)
	if utf8.RuneCountInString(note) > maxNoteRunes {
		return "", fmt.Errorf("invalid: note too long")
	}
	return note, nil
}

func JobHandler(st *store.Store, client *llm.Client, master []byte) job.Handler {
	return func(ctx context.Context, rec job.Record, prog func(int, string)) (json.RawMessage, error) {
		var in Input
		if err := json.Unmarshal(rec.Payload, &in); err != nil {
			return nil, err
		}
		if in.ChapterID == "" {
			return nil, fmt.Errorf("invalid: chapter_id required")
		}
		out, err := Run(ctx, st, client, master, rec.UserID, in, prog)
		if err != nil {
			_ = markStatus(ctx, st, in.ChapterID, rec.UserID, StatusFailed)
			return nil, err
		}
		return json.Marshal(map[string]string{"chapter_id": out.ID})
	}
}

func Run(ctx context.Context, st *store.Store, client *llm.Client, master []byte, userID string, in Input, prog func(int, string)) (Chapter, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if prog == nil {
		prog = func(int, string) {}
	}

	note, err := ValidateNote(in.Note)
	if err != nil {
		return Chapter{}, err
	}
	target := ClampTarget(in.Target)

	prog(10, "load")
	ch, err := LoadOwned(ctx, st, userID, in.ChapterID)
	if err != nil {
		return Chapter{}, err
	}
	if strings.TrimSpace(ch.CardID) == "" {
		return Chapter{}, fmt.Errorf("invalid: card_id required")
	}
	c, err := loadOwnedCard(ctx, st, userID, ch.CardID)
	if err != nil {
		return Chapter{}, err
	}
	siblings, err := ListByProject(ctx, st, userID, ch.ProjectID)
	if err != nil {
		return Chapter{}, err
	}
	prev, err := loadPrevious(ctx, st, userID, ch.ProjectID, ch.Seq)
	if err != nil {
		return Chapter{}, err
	}
	entries, err := bible.ListByProject(ctx, st, userID, ch.ProjectID)
	if err != nil {
		return Chapter{}, err
	}
	bibleBlock := bible.RenderForPrompt(entries)

	prog(20, "llm_key")
	key, err := llmkey.Load(ctx, st, master, userID)
	if err != nil {
		return Chapter{}, err
	}

	model := strings.TrimSpace(in.Model)
	if model == "" {
		model = strings.TrimSpace(ch.Model)
	}
	if model == "" {
		model = defaultWriteModel
	}

	prog(55, "generate")
	body, err := chatChapter(ctx, client, key, model, c, ch, siblings, prev, bibleBlock, note, target)
	if err != nil {
		return Chapter{}, err
	}

	prog(80, "summarize")
	summary := ""
	if s, sumErr := chatSummary(ctx, client, key, model, ch.Title, body); sumErr == nil {
		summary = s
	}

	now := time.Now().UTC().Format(time.RFC3339)
	ch.CardID = c.ID
	ch.CardVersion = c.Version
	ch.Body = body
	ch.Summary = summary
	ch.Status = StatusWritten
	ch.TargetRunes = target
	ch.Model = model
	ch.UpdatedAt = now

	prog(95, "persist")
	if err := persistGenerated(ctx, st, ch); err != nil {
		return Chapter{}, err
	}

	prog(97, "bible_sync")
	syncBible(ctx, st, client, key, model, userID, ch.ProjectID, ch.Seq, ch.Title, body)

	prog(100, "done")
	return ch, nil
}

// syncBible 用同一把 LLM key 把刚写完的一章交给设定集管理员维护；
// 失败不致命（同 summary 先例）——章节已持久化，设定集未更新也不影响本章。
func syncBible(ctx context.Context, st *store.Store, client *llm.Client, key llmkey.Key, model, userID, projectID string, seq int, title, body string) {
	entries, err := bible.ListByProject(ctx, st, userID, projectID)
	if err != nil {
		return
	}
	ops, err := bible.SyncChapter(ctx, client, key, model, entries, seq, title, body)
	if err != nil {
		return
	}
	_, _, _ = bible.ApplyOps(ctx, st, userID, projectID, seq, ops)
}

func chatChapter(ctx context.Context, client *llm.Client, key llmkey.Key, model string, c card.Card, ch Chapter, siblings []ChapterSummary, prev []Chapter, bibleBlock, note string, target int) (string, error) {
	cardJSON, err := json.Marshal(map[string]any{
		"id":           c.ID,
		"name":         c.Name,
		"kind":         c.Kind,
		"version":      c.Version,
		"dimensions":   c.Dimensions,
		"prohibitions": c.Prohibitions,
	})
	if err != nil {
		return "", err
	}
	req := llm.Request{
		Provider:  key.Provider,
		BaseURL:   key.BaseURL,
		APIKey:    key.APIKey,
		Model:     model,
		Temp:      writeChatTemp,
		MaxTokens: writeMaxTokens,
		Messages: []llm.Message{
			{Role: "system", Content: llm.ChapterSystem(string(cardJSON))},
			{Role: "user", Content: ChapterUserPrompt(ch, siblings, prev, bibleBlock, note, target)},
		},
	}
	body, err := client.Chat(ctx, req)
	if err != nil {
		return "", err
	}
	body = strings.TrimSpace(body)
	if body == "" {
		return "", fmt.Errorf("invalid: empty chapter body")
	}
	if llm.ForbiddenCopyCheck(body) {
		return "", fmt.Errorf("invalid: chapter contains banned copy")
	}
	return body, nil
}

func ChapterUserPrompt(ch Chapter, siblings []ChapterSummary, prev []Chapter, bibleBlock, note string, target int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "本章序号：第 %d 章\n标题：%s\n概括：%s\n目标字数：%d\n", ch.Seq, ch.Title, ch.Brief, target)
	if note != "" {
		fmt.Fprintf(&b, "这一次补充：%s\n", note)
	}
	b.WriteString("\n全书目录：\n")
	if len(siblings) == 0 {
		b.WriteString("（仅本章）\n")
	} else {
		for _, s := range siblings {
			mark := ""
			if s.ID == ch.ID {
				mark = " ← 本章"
			}
			fmt.Fprintf(&b, "%d. %s%s\n", s.Seq, s.Title, mark)
		}
	}
	if bibleBlock != "" {
		b.WriteString("\n")
		b.WriteString(bibleBlock)
	}
	if len(prev) > 0 {
		b.WriteString("\n前情摘要：\n")
		for _, p := range prev {
			if strings.TrimSpace(p.Summary) == "" {
				continue
			}
			fmt.Fprintf(&b, "第 %d 章《%s》：%s\n", p.Seq, p.Title, p.Summary)
		}
		last := prev[len(prev)-1]
		if tail := lastRunes(last.Body, prevTailRunes); tail != "" {
			fmt.Fprintf(&b, "\n上一章文末：\n%s\n", tail)
		}
	}
	b.WriteString("\n现在写本章正文。")
	return b.String()
}

func chatSummary(ctx context.Context, client *llm.Client, key llmkey.Key, model, title, body string) (string, error) {
	req := llm.Request{
		Provider:  key.Provider,
		BaseURL:   key.BaseURL,
		APIKey:    key.APIKey,
		Model:     model,
		Temp:      0.2,
		MaxTokens: summaryMaxTokens,
		Messages: []llm.Message{
			{Role: "system", Content: llm.ChapterSummarySystem()},
			{Role: "user", Content: fmt.Sprintf("章标题：%s\n\n正文：\n%s", title, body)},
		},
	}
	text, err := client.Chat(ctx, req)
	if err != nil {
		return "", err
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return "", fmt.Errorf("empty summary")
	}
	if llm.ForbiddenCopyCheck(text) {
		return "", fmt.Errorf("banned summary")
	}
	if utf8.RuneCountInString(text) > maxSummaryRunes {
		text = string([]rune(text)[:maxSummaryRunes])
	}
	return text, nil
}

func lastRunes(s string, n int) string {
	rs := []rune(strings.TrimSpace(s))
	if len(rs) <= n {
		return string(rs)
	}
	return string(rs[len(rs)-n:])
}

func persistGenerated(ctx context.Context, st *store.Store, ch Chapter) error {
	_, err := st.DB().ExecContext(
		ctx,
		`UPDATE chapters SET card_id=?, card_version=?, body=?, summary=?, status=?, target_runes=?, model=?, updated_at=?
		 WHERE id=?`,
		ch.CardID, ch.CardVersion, ch.Body, ch.Summary, ch.Status, ch.TargetRunes, ch.Model, ch.UpdatedAt, ch.ID,
	)
	return err
}

func markStatus(ctx context.Context, st *store.Store, chapterID, userID, status string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := st.DB().ExecContext(
		ctx,
		`UPDATE chapters SET status=?, updated_at=?
		 WHERE id=? AND project_id IN (SELECT id FROM projects WHERE user_id=?)`,
		status, now, chapterID, userID,
	)
	return err
}

func MarkWriting(ctx context.Context, st *store.Store, chapterID, userID string) error {
	return markStatus(ctx, st, chapterID, userID, StatusWriting)
}

func LoadOwned(ctx context.Context, st *store.Store, userID, chapterID string) (Chapter, error) {
	var ch Chapter
	err := st.DB().QueryRowContext(
		ctx,
		`SELECT c.id, c.project_id, c.card_id, c.card_version, c.seq, c.title, c.brief, c.body, c.summary,
		        c.status, c.target_runes, c.model, c.created_at, c.updated_at
		 FROM chapters c
		 JOIN projects p ON p.id = c.project_id
		 WHERE c.id = ? AND p.user_id = ?`,
		chapterID, userID,
	).Scan(
		&ch.ID, &ch.ProjectID, &ch.CardID, &ch.CardVersion, &ch.Seq, &ch.Title, &ch.Brief, &ch.Body, &ch.Summary,
		&ch.Status, &ch.TargetRunes, &ch.Model, &ch.CreatedAt, &ch.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return Chapter{}, fmt.Errorf("not found")
		}
		return Chapter{}, err
	}
	return ch, nil
}

func ListByProject(ctx context.Context, st *store.Store, userID, projectID string) ([]ChapterSummary, error) {
	rows, err := st.DB().QueryContext(
		ctx,
		`SELECT c.id, c.seq, c.title, c.brief, c.status, c.body,
		        CASE WHEN c.summary != '' THEN 1 ELSE 0 END, c.card_id, c.target_runes, c.updated_at
		 FROM chapters c
		 JOIN projects p ON p.id = c.project_id
		 WHERE c.project_id = ? AND p.user_id = ?
		 ORDER BY c.seq ASC`,
		projectID, userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ChapterSummary
	for rows.Next() {
		var s ChapterSummary
		var has int
		var body string
		if err := rows.Scan(&s.ID, &s.Seq, &s.Title, &s.Brief, &s.Status, &body, &has, &s.CardID, &s.TargetRunes, &s.UpdatedAt); err != nil {
			return nil, err
		}
		s.RuneCount = utf8.RuneCountInString(body)
		s.HasSummary = has == 1
		out = append(out, s)
	}
	return out, rows.Err()
}

func NextSeq(ctx context.Context, st *store.Store, projectID string) (int, error) {
	var max sql.NullInt64
	err := st.DB().QueryRowContext(ctx, `SELECT MAX(seq) FROM chapters WHERE project_id = ?`, projectID).Scan(&max)
	if err != nil {
		return 0, err
	}
	if !max.Valid {
		return 1, nil
	}
	return int(max.Int64) + 1, nil
}

func Insert(ctx context.Context, st *store.Store, ch Chapter) error {
	_, err := st.DB().ExecContext(
		ctx,
		`INSERT INTO chapters (id, project_id, card_id, card_version, seq, title, brief, body, summary, status, target_runes, model, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		ch.ID, ch.ProjectID, ch.CardID, ch.CardVersion, ch.Seq, ch.Title, ch.Brief, ch.Body, ch.Summary, ch.Status, ch.TargetRunes, ch.Model, ch.CreatedAt, ch.UpdatedAt,
	)
	return err
}

func UpdateFields(ctx context.Context, st *store.Store, ch Chapter) error {
	_, err := st.DB().ExecContext(
		ctx,
		`UPDATE chapters SET card_id=?, card_version=?, title=?, brief=?, body=?, summary=?, status=?, target_runes=?, model=?, updated_at=?
		 WHERE id=?`,
		ch.CardID, ch.CardVersion, ch.Title, ch.Brief, ch.Body, ch.Summary, ch.Status, ch.TargetRunes, ch.Model, ch.UpdatedAt, ch.ID,
	)
	return err
}

func DeleteAndRenumber(ctx context.Context, st *store.Store, userID, chapterID string) error {
	ch, err := LoadOwned(ctx, st, userID, chapterID)
	if err != nil {
		return err
	}
	tx, err := st.DB().BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM chapters WHERE id=?`, ch.ID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE chapters SET seq = seq - 1 WHERE project_id=? AND seq > ?`, ch.ProjectID, ch.Seq); err != nil {
		return err
	}
	// Volumes are keyed by the chapter they start at; move them with it.
	if err := lore.ShiftVolumesAfterDelete(ctx, tx, ch.ProjectID, ch.Seq); err != nil {
		return err
	}
	return tx.Commit()
}

func ManuscriptMarkdown(ctx context.Context, st *store.Store, userID, projectID, projectName string) (string, error) {
	rows, err := st.DB().QueryContext(
		ctx,
		`SELECT c.seq, c.title, c.body
		 FROM chapters c
		 JOIN projects p ON p.id = c.project_id
		 WHERE c.project_id = ? AND p.user_id = ?
		 ORDER BY c.seq ASC`,
		projectID, userID,
	)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	var b strings.Builder
	if projectName != "" {
		fmt.Fprintf(&b, "# %s\n\n", projectName)
	}
	n := 0
	for rows.Next() {
		var seq int
		var title, body string
		if err := rows.Scan(&seq, &title, &body); err != nil {
			return "", err
		}
		n++
		fmt.Fprintf(&b, "## 第 %d 章 %s\n\n", seq, title)
		if strings.TrimSpace(body) == "" {
			b.WriteString("（未写）\n\n")
			continue
		}
		b.WriteString(strings.TrimSpace(body))
		b.WriteString("\n\n")
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	if n == 0 {
		b.WriteString("（还没有章节）\n")
	}
	return b.String(), nil
}

func loadPrevious(ctx context.Context, st *store.Store, userID, projectID string, seq int) ([]Chapter, error) {
	rows, err := st.DB().QueryContext(
		ctx,
		`SELECT c.id, c.project_id, c.card_id, c.card_version, c.seq, c.title, c.brief, c.body, c.summary,
		        c.status, c.target_runes, c.model, c.created_at, c.updated_at
		 FROM chapters c
		 JOIN projects p ON p.id = c.project_id
		 WHERE c.project_id = ? AND p.user_id = ? AND c.seq < ?
		 ORDER BY c.seq DESC
		 LIMIT ?`,
		projectID, userID, seq, prevSummaryLimit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var rev []Chapter
	for rows.Next() {
		var ch Chapter
		if err := rows.Scan(
			&ch.ID, &ch.ProjectID, &ch.CardID, &ch.CardVersion, &ch.Seq, &ch.Title, &ch.Brief, &ch.Body, &ch.Summary,
			&ch.Status, &ch.TargetRunes, &ch.Model, &ch.CreatedAt, &ch.UpdatedAt,
		); err != nil {
			return nil, err
		}
		rev = append(rev, ch)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i, j := 0, len(rev)-1; i < j; i, j = i+1, j-1 {
		rev[i], rev[j] = rev[j], rev[i]
	}
	return rev, nil
}

func loadOwnedCard(ctx context.Context, st *store.Store, userID, cardID string) (card.Card, error) {
	var (
		id, projectID, name, kind                    string
		version                                      int
		dimsJSON, prohibJSON, factsJSON, lineageJSON sql.NullString
	)
	err := st.DB().QueryRowContext(
		ctx,
		`SELECT c.id, c.project_id, c.name, c.kind, v.version,
		        v.dimensions_json, v.prohibitions_json, v.facts_json, v.lineage_json
		 FROM style_cards c
		 JOIN projects p ON p.id = c.project_id
		 JOIN style_card_versions v ON v.card_id = c.id AND v.version = c.current_version
		 WHERE c.id = ? AND p.user_id = ?`,
		cardID, userID,
	).Scan(&id, &projectID, &name, &kind, &version, &dimsJSON, &prohibJSON, &factsJSON, &lineageJSON)
	if err != nil {
		if err == sql.ErrNoRows {
			return card.Card{}, fmt.Errorf("invalid: card not found")
		}
		return card.Card{}, err
	}
	out := card.Card{
		ID:        id,
		ProjectID: projectID,
		Name:      name,
		Kind:      kind,
		Version:   version,
		Facts:     json.RawMessage(`{}`),
	}
	if err := json.Unmarshal([]byte(dimsJSON.String), &out.Dimensions); err != nil {
		return card.Card{}, err
	}
	if err := json.Unmarshal([]byte(prohibJSON.String), &out.Prohibitions); err != nil {
		return card.Card{}, err
	}
	return out, nil
}
