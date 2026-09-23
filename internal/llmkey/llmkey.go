// Package llmkey loads a user's BYOK LLM credentials.
//
// Every job package and the HTTP layer used to carry its own copy of this
// loader -- eight identical copies. The HTTP layer calls it as a precheck
// before enqueueing and the job calls it again when it runs, so the two must
// agree on which key they pick; one shared implementation guarantees that.
package llmkey

import (
	"context"
	"errors"

	"stylelab/internal/cryptokey"
	"stylelab/internal/store"
)

// Key is a decrypted provider credential.
type Key struct {
	Provider string
	BaseURL  string
	APIKey   string
}

// ErrMissing reports that the user has stored no key. The "invalid:" prefix
// is the codebase's convention for errors a caller can fix; the HTTP layer
// strips it for display.
var ErrMissing = errors.New("invalid: missing llm key")

// preferred providers are chosen over the others when a user has several.
var preferred = map[string]bool{"chat": true, "response": true, "openai": true}

// Load returns the user's key. With several stored it picks the first
// preferred provider in provider-name order, else the first key.
func Load(ctx context.Context, st *store.Store, master []byte, userID string) (Key, error) {
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
		return Key{}, ErrMissing
	}
	for _, k := range keys {
		if preferred[k.Provider] {
			return k, nil
		}
	}
	return keys[0], nil
}
