package bible

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"stylelab/internal/cryptokey"
	"stylelab/internal/job"
	"stylelab/internal/llm"
	"stylelab/internal/store"
)

// SyncInput 是回填 job 的 payload。按章序遍历项目里所有已写章节，
// 逐章提取设定集操作并立即落库，取消或失败时已完成的章节保留。
type SyncInput struct {
	Model string `json:"model"`
}

type SyncResult struct {
	ChaptersSynced int `json:"chapters_synced"`
	Entries        int `json:"entries"`
}

func JobHandler(st *store.Store, client *llm.Client, master []byte) job.Handler {
	return func(ctx context.Context, rec job.Record, prog func(int, string)) (json.RawMessage, error) {
		var in SyncInput
		if err := json.Unmarshal(rec.Payload, &in); err != nil {
			return nil, err
		}
		out, err := RunSync(ctx, st, client, master, rec.UserID, rec.ProjectID, in, prog)
		if err != nil {
			return nil, err
		}
		return json.Marshal(out)
	}
}

type syncChapterRow struct {
	ID    string
	Seq   int
	Title string
	Body  string
}

func RunSync(ctx context.Context, st *store.Store, client *llm.Client, master []byte, userID, projectID string, in SyncInput, prog func(int, string)) (SyncResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if prog == nil {
		prog = func(int, string) {}
	}

	prog(5, "llm_key")
	key, err := loadUserKey(ctx, st, master, userID)
	if err != nil {
		return SyncResult{}, err
	}
	model := strings.TrimSpace(in.Model)
	if model == "" {
		model = defaultSyncModel
	}

	prog(10, "load_chapters")
	rows, err := st.DB().QueryContext(
		ctx,
		`SELECT c.id, c.seq, c.title, c.body
		 FROM chapters c
		 JOIN projects p ON p.id = c.project_id
		 WHERE c.project_id = ? AND p.user_id = ? AND c.status = ? AND c.body != ''
		 ORDER BY c.seq ASC`,
		projectID, userID, "written",
	)
	if err != nil {
		return SyncResult{}, err
	}
	var chapters []syncChapterRow
	for rows.Next() {
		var ch syncChapterRow
		if err := rows.Scan(&ch.ID, &ch.Seq, &ch.Title, &ch.Body); err != nil {
			rows.Close()
			return SyncResult{}, err
		}
		chapters = append(chapters, ch)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return SyncResult{}, err
	}
	rows.Close()
	if len(chapters) == 0 {
		return SyncResult{}, fmt.Errorf("invalid: no written chapters")
	}

	for i, ch := range chapters {
		if err := ctx.Err(); err != nil {
			return SyncResult{}, err
		}
		entries, err := ListByProject(ctx, st, userID, projectID)
		if err != nil {
			return SyncResult{}, err
		}
		ops, err := SyncChapter(ctx, client, key, model, entries, ch.Seq, ch.Title, ch.Body)
		if err != nil {
			return SyncResult{}, fmt.Errorf("chapter %d (%s): %w", ch.Seq, ch.Title, err)
		}
		if _, _, err := ApplyOps(ctx, st, userID, projectID, ch.Seq, ops); err != nil {
			return SyncResult{}, err
		}
		prog(10+85*(i+1)/len(chapters), fmt.Sprintf("chapter %d/%d", i+1, len(chapters)))
	}

	entries, err := ListByProject(ctx, st, userID, projectID)
	if err != nil {
		return SyncResult{}, err
	}
	prog(100, "done")
	return SyncResult{ChaptersSynced: len(chapters), Entries: len(entries)}, nil
}

func loadUserKey(ctx context.Context, st *store.Store, master []byte, userID string) (Key, error) {
	rows, err := st.DB().QueryContext(
		ctx,
		`SELECT provider, base_url, encrypted_key FROM user_llm_keys WHERE user_id = ? ORDER BY provider`,
		userID,
	)
	if err != nil {
		return Key{}, err
	}
	defer rows.Close()

	var keys []Key
	for rows.Next() {
		var provider, baseURL string
		var blob []byte
		if err := rows.Scan(&provider, &baseURL, &blob); err != nil {
			return Key{}, err
		}
		plain, err := cryptokey.Open(master, blob)
		if err != nil {
			return Key{}, err
		}
		keys = append(keys, Key{Provider: provider, BaseURL: baseURL, APIKey: plain})
	}
	if err := rows.Err(); err != nil {
		return Key{}, err
	}
	if len(keys) == 0 {
		return Key{}, fmt.Errorf("invalid: missing llm key")
	}
	for _, k := range keys {
		if k.Provider == "chat" || k.Provider == "response" || k.Provider == "openai" {
			return k, nil
		}
	}
	return keys[0], nil
}
