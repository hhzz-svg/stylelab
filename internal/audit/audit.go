package audit

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"stylelab/internal/card"
	"stylelab/internal/cryptokey"
	"stylelab/internal/ids"
	"stylelab/internal/job"
	"stylelab/internal/llm"
	"stylelab/internal/store"
)

const (
	PersonaCommercial = "commercial_web"
	PersonaLiterary   = "literary_texture"
	defaultAuditModel = "gpt-4o-mini"
	auditChatTemp     = 0.3
	auditParseRetries = 2
)

type Report struct {
	ID               string                 `json:"id"`
	CardID           string                 `json:"card_id"`
	CardVersion      int                    `json:"card_version"`
	Personas         []string               `json:"personas"`
	ByPersona        map[string]PersonaView `json:"by_persona"`
	Conflicts        []Conflict             `json:"conflicts"`
	RecommendedEdits []Edit                 `json:"recommended_edits"`
}

type PersonaView struct {
	Strengths   []string `json:"strengths"`
	Risks       []string `json:"risks"`
	Suggestions []string `json:"suggestions"`
}

type Conflict struct {
	Dimension string `json:"dimension"`
	Summary   string `json:"summary"`
}

type Edit struct {
	Dimension   string `json:"dimension"`
	TargetLevel int    `json:"target_level"`
	Reason      string `json:"reason"`
}

type llmKey struct {
	Provider string
	BaseURL  string
	APIKey   string
}

type personaResult struct {
	View      PersonaView
	Conflicts []Conflict
	Edits     []Edit
}

func JobHandler(st *store.Store, client *llm.Client, master []byte) job.Handler {
	return func(ctx context.Context, rec job.Record, prog func(int, string)) (json.RawMessage, error) {
		var in struct {
			CardID string `json:"card_id"`
			Model  string `json:"model"`
		}
		if err := json.Unmarshal(rec.Payload, &in); err != nil {
			return nil, err
		}
		if in.CardID == "" {
			return nil, fmt.Errorf("invalid: card_id required")
		}
		rep, err := Run(ctx, st, client, master, rec.UserID, in.CardID, in.Model, prog)
		if err != nil {
			return nil, err
		}
		return json.Marshal(map[string]string{"audit_id": rep.ID})
	}
}

func Run(ctx context.Context, st *store.Store, client *llm.Client, master []byte, userID, cardID, model string, prog func(int, string)) (Report, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if prog == nil {
		prog = func(int, string) {}
	}

	prog(10, "load_card")
	c, err := loadOwnedCard(ctx, st, userID, cardID)
	if err != nil {
		return Report{}, err
	}

	prog(25, "llm_key")
	key, err := loadUserKey(ctx, st, master, userID)
	if err != nil {
		return Report{}, err
	}

	prog(45, "audit")
	personas := []string{PersonaCommercial, PersonaLiterary}
	results := make(map[string]personaResult, len(personas))
	var mu sync.Mutex
	g, gctx := errgroup.WithContext(ctx)
	for _, persona := range personas {
		persona := persona
		g.Go(func() error {
			res, err := chatPersona(gctx, client, key, model, persona, c)
			if err != nil {
				return err
			}
			mu.Lock()
			results[persona] = res
			mu.Unlock()
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return Report{}, err
	}

	byPersona := make(map[string]PersonaView, len(personas))
	var conflicts []Conflict
	var edits []Edit
	seenConflict := map[string]struct{}{}
	seenEdit := map[string]struct{}{}
	for _, persona := range personas {
		res := results[persona]
		byPersona[persona] = res.View
		for _, cf := range res.Conflicts {
			k := cf.Dimension + "\n" + cf.Summary
			if _, ok := seenConflict[k]; ok {
				continue
			}
			seenConflict[k] = struct{}{}
			conflicts = append(conflicts, cf)
		}
		for _, ed := range res.Edits {
			k := fmt.Sprintf("%s\n%d\n%s", ed.Dimension, ed.TargetLevel, ed.Reason)
			if _, ok := seenEdit[k]; ok {
				continue
			}
			seenEdit[k] = struct{}{}
			edits = append(edits, ed)
		}
	}
	if conflicts == nil {
		conflicts = []Conflict{}
	}
	if edits == nil {
		edits = []Edit{}
	}

	rep := Report{
		ID:               ids.New("aud_"),
		CardID:           c.ID,
		CardVersion:      c.Version,
		Personas:         personas,
		ByPersona:        byPersona,
		Conflicts:        conflicts,
		RecommendedEdits: edits,
	}

	prog(85, "persist")
	if err := persistReport(ctx, st, c.ProjectID, rep); err != nil {
		return Report{}, err
	}
	prog(100, "done")
	return rep, nil
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
	if factsJSON.Valid && strings.TrimSpace(factsJSON.String) != "" {
		out.Facts = json.RawMessage(factsJSON.String)
	}
	if lineageJSON.Valid && strings.TrimSpace(lineageJSON.String) != "" && lineageJSON.String != "null" {
		var lin card.Lineage
		if err := json.Unmarshal([]byte(lineageJSON.String), &lin); err != nil {
			return card.Card{}, err
		}
		out.Lineage = &lin
	}
	return out, nil
}

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
		if k.Provider == "openai" {
			return k, nil
		}
	}
	return keys[0], nil
}

func chatPersona(ctx context.Context, client *llm.Client, key llmKey, model, persona string, c card.Card) (personaResult, error) {
	if strings.TrimSpace(model) == "" {
		model = defaultAuditModel
	}
	userContent, err := buildAuditUserContent(c)
	if err != nil {
		return personaResult{}, err
	}
	req := llm.Request{
		Provider: key.Provider,
		BaseURL:  key.BaseURL,
		APIKey:   key.APIKey,
		Model:    model,
		Temp:     auditChatTemp,
		Messages: []llm.Message{
			{Role: "system", Content: llm.AuditSystem(persona)},
			{Role: "user", Content: userContent},
		},
	}
	var lastErr error
	for attempt := 0; attempt < 1+auditParseRetries; attempt++ {
		raw, err := client.Chat(ctx, req)
		if err != nil {
			return personaResult{}, err
		}
		res, err := parsePersonaJSON(raw)
		if err == nil {
			return res, nil
		}
		lastErr = err
	}
	return personaResult{}, lastErr
}

func buildAuditUserContent(c card.Card) (string, error) {
	payload, err := json.Marshal(map[string]any{
		"card_id":      c.ID,
		"card_version": c.Version,
		"name":         c.Name,
		"kind":         c.Kind,
		"dimensions":   c.Dimensions,
		"prohibitions": c.Prohibitions,
	})
	if err != nil {
		return "", err
	}
	return "审阅这张风格卡片的技法是否自洽。只评价技法密度与风险，不要点名作者。\n" + string(payload), nil
}

func parsePersonaJSON(raw string) (personaResult, error) {
	payload := extractJSONObject(raw)
	var out struct {
		Strengths        []string   `json:"strengths"`
		Risks            []string   `json:"risks"`
		Suggestions      []string   `json:"suggestions"`
		Conflicts        []Conflict `json:"conflicts"`
		RecommendedEdits []Edit     `json:"recommended_edits"`
	}
	if err := json.Unmarshal(payload, &out); err != nil {
		return personaResult{}, fmt.Errorf("invalid: audit json: %w", err)
	}
	blob := string(payload)
	if llm.ForbiddenCopyCheck(blob) {
		return personaResult{}, fmt.Errorf("invalid: audit json contains banned copy")
	}
	if out.Strengths == nil {
		out.Strengths = []string{}
	}
	if out.Risks == nil {
		out.Risks = []string{}
	}
	if out.Suggestions == nil {
		out.Suggestions = []string{}
	}
	if out.Conflicts == nil {
		out.Conflicts = []Conflict{}
	}
	if out.RecommendedEdits == nil {
		out.RecommendedEdits = []Edit{}
	}
	return personaResult{
		View: PersonaView{
			Strengths:   out.Strengths,
			Risks:       out.Risks,
			Suggestions: out.Suggestions,
		},
		Conflicts: out.Conflicts,
		Edits:     out.RecommendedEdits,
	}, nil
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

func persistReport(ctx context.Context, st *store.Store, projectID string, rep Report) error {
	body, err := json.Marshal(rep)
	if err != nil {
		return err
	}
	_, err = st.DB().ExecContext(
		ctx,
		`INSERT INTO audit_reports (id, project_id, card_id, card_version, report_json, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		rep.ID, projectID, rep.CardID, rep.CardVersion, string(body),
		time.Now().UTC().Format(time.RFC3339),
	)
	return err
}
