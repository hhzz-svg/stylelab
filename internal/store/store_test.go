package store_test

import (
	"os"
	"path/filepath"
	"testing"

	"stylelab/internal/store"
)

func TestOpenCreatesTablesAndBlobDir(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() {
		if err := st.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	})

	want := []string{
		"users",
		"sessions",
		"user_llm_keys",
		"projects",
		"assets",
		"style_cards",
		"style_card_versions",
		"jobs",
		"audit_reports",
		"samples",
	}
	got := map[string]bool{}
	rows, err := st.DB().Query(`SELECT name FROM sqlite_master WHERE type='table'`)
	if err != nil {
		t.Fatalf("query tables: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan: %v", err)
		}
		got[name] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}
	for _, name := range want {
		if !got[name] {
			t.Errorf("missing table %s", name)
		}
	}

	info, err := os.Stat(filepath.Join(dir, "blobs"))
	if err != nil {
		t.Fatalf("blobs dir: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("blobs path is not a directory")
	}
}
