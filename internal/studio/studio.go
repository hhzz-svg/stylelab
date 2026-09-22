// Package studio holds the three long-running planning features -- outline
// generation, the continuity audit and the branch simulator.
//
// They live together rather than in a package each because they share a shape
// no other feature has: the job's result IS the data the UI renders, not an id
// pointing at a row. extract/fuse/sample/audit all persist something and return
// its id; these three persist nothing and hand the parsed JSON straight back
// through job.Record.Result.
package studio

import (
	"context"
	"fmt"
	"strings"

	"stylelab/internal/cryptokey"
	"stylelab/internal/store"
)

const (
	// DefaultModel mirrors the per-package defaults used by the other job
	// features. An omitted model must never reach the provider as "".
	DefaultModel = "gpt-4o-mini"

	// Context bounds. These prompts are assembled from the whole manuscript,
	// so without caps a long project overruns the model's context window and
	// the reply comes back as truncated JSON. Raising MaxTokens does not help:
	// that caps the completion, not the prompt.
	auditChapterLimit = 60
	auditChapterRunes = 180
	auditBibleLimit   = 40
	auditBibleRunes   = 200

	outlineBibleLimit = 15

	branchPrevSummaryLimit = 3
	branchTailRunes        = 1200
	continueInstructionMax = 500
)

type llmKey struct {
	Provider string
	BaseURL  string
	APIKey   string
}

// resolveModel returns the first non-blank candidate, falling back to
// DefaultModel.
func resolveModel(candidates ...string) string {
	for _, c := range candidates {
		if c = strings.TrimSpace(c); c != "" {
			return c
		}
	}
	return DefaultModel
}

func headRunes(s string, n int) string {
	if n <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

func tailRunes(s string, n int) string {
	if n <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[len(r)-n:])
}

// cleanJSONMarkdown strips a ```json fence some providers wrap replies in.
func cleanJSONMarkdown(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```json") {
		s = strings.TrimPrefix(s, "```json")
		s = strings.TrimSuffix(s, "```")
	} else if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```")
		s = strings.TrimSuffix(s, "```")
	}
	return strings.TrimSpace(s)
}

// loadUserKey mirrors the identical helper in sample/write/extract/fuse/audit/
// bible. Consolidating the six copies is worth doing, but is a refactor across
// every job package rather than part of this change.
func loadUserKey(ctx context.Context, st *store.Store, master []byte, userID string) (llmKey, error) {
	rows, err := st.DB().QueryContext(
		ctx,
		`SELECT provider, base_url, encrypted_key FROM user_llm_keys WHERE user_id = ? ORDER BY provider`,
		userID,
	)
	if err != nil {
		return llmKey{}, err
	}
	defer rows.Close()

	var keys []llmKey
	for rows.Next() {
		var provider, baseURL string
		var blob []byte
		if err := rows.Scan(&provider, &baseURL, &blob); err != nil {
			return llmKey{}, err
		}
		plain, err := cryptokey.Open(master, blob)
		if err != nil {
			return llmKey{}, err
		}
		keys = append(keys, llmKey{Provider: provider, BaseURL: baseURL, APIKey: plain})
	}
	if err := rows.Err(); err != nil {
		return llmKey{}, err
	}
	if len(keys) == 0 {
		return llmKey{}, fmt.Errorf("invalid: missing llm key")
	}
	for _, k := range keys {
		if k.Provider == "chat" || k.Provider == "response" || k.Provider == "openai" {
			return k, nil
		}
	}
	return keys[0], nil
}

// ownsProject reports whether the project belongs to the user. The HTTP layer
// checks this before enqueueing, but a job runs later and off the request, so
// the handler re-checks rather than trusting its payload.
func ownsProject(ctx context.Context, st *store.Store, projectID, userID string) error {
	var got string
	err := st.DB().QueryRowContext(
		ctx, `SELECT id FROM projects WHERE id = ? AND user_id = ?`, projectID, userID,
	).Scan(&got)
	return err
}
