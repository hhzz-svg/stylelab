package httpapi

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"stylelab/internal/extract"
	"stylelab/internal/ids"
)

const maxAssetBytes = 2 << 20

func (s *Server) handleUploadAsset(w http.ResponseWriter, r *http.Request) {
	userID, err := s.currentUserID(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}
	projectID := r.PathValue("id")
	if err := s.requireOwnedProject(r.Context(), projectID, userID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "not_found", "not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "invalid", "internal error")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxAssetBytes+128*1024)
	if err := r.ParseMultipartForm(maxAssetBytes + 128*1024); err != nil {
		writeError(w, http.StatusBadRequest, "invalid", "invalid upload")
		return
	}
	file, hdr, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid", "file is required")
		return
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, maxAssetBytes+1))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid", "invalid upload")
		return
	}
	if len(data) > maxAssetBytes {
		writeError(w, http.StatusBadRequest, "invalid", "file too large")
		return
	}
	if len(data) == 0 {
		writeError(w, http.StatusBadRequest, "invalid", "file is empty")
		return
	}
	if !utf8.Valid(data) {
		writeError(w, http.StatusBadRequest, "invalid", "file must be utf-8")
		return
	}

	filename := filepath.Base(hdr.Filename)
	ext := strings.ToLower(filepath.Ext(filename))
	if ext != ".txt" && ext != ".md" {
		writeError(w, http.StatusBadRequest, "invalid", "only .txt and .md are accepted")
		return
	}

	sum := sha256.Sum256(data)
	sumHex := hex.EncodeToString(sum[:])
	blobPath := filepath.Join(s.cfg.DataDir, "blobs", sumHex)
	if _, err := os.Stat(blobPath); err != nil {
		if !os.IsNotExist(err) {
			writeError(w, http.StatusInternalServerError, "invalid", "internal error")
			return
		}
		if err := os.WriteFile(blobPath, data, 0o644); err != nil {
			writeError(w, http.StatusInternalServerError, "invalid", "internal error")
			return
		}
	}

	text := string(data)
	chapters := extract.SplitChapters(text)
	id := ids.New("ast_")
	createdAt := time.Now().UTC().Format(time.RFC3339)
	runeCount := utf8.RuneCountInString(text)
	_, err = s.st.DB().ExecContext(
		r.Context(),
		`INSERT INTO assets (id, project_id, filename, sha256, rune_count, chapter_count, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, projectID, filename, sumHex, runeCount, len(chapters), createdAt,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "invalid", "internal error")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"id":            id,
		"filename":      filename,
		"sha256":        sumHex,
		"rune_count":    runeCount,
		"chapter_count": len(chapters),
	})
}

func (s *Server) handleListAssets(w http.ResponseWriter, r *http.Request) {
	userID, err := s.currentUserID(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}
	projectID := r.PathValue("id")
	if err := s.requireOwnedProject(r.Context(), projectID, userID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "not_found", "not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "invalid", "internal error")
		return
	}

	rows, err := s.st.DB().QueryContext(
		r.Context(),
		`SELECT id, filename, sha256, rune_count, chapter_count, created_at
		 FROM assets WHERE project_id = ? ORDER BY created_at DESC, id DESC`,
		projectID,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "invalid", "internal error")
		return
	}
	defer rows.Close()

	assets := make([]map[string]any, 0)
	for rows.Next() {
		var id, filename, sumHex, createdAt string
		var runeCount, chapterCount int
		if err := rows.Scan(&id, &filename, &sumHex, &runeCount, &chapterCount, &createdAt); err != nil {
			writeError(w, http.StatusInternalServerError, "invalid", "internal error")
			return
		}
		assets = append(assets, map[string]any{
			"id":            id,
			"filename":      filename,
			"sha256":        sumHex,
			"rune_count":    runeCount,
			"chapter_count": chapterCount,
			"created_at":    createdAt,
		})
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "invalid", "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"assets": assets})
}

func (s *Server) requireOwnedProject(ctx context.Context, projectID, userID string) error {
	var n int
	err := s.st.DB().QueryRowContext(
		ctx,
		`SELECT 1 FROM projects WHERE id = ? AND user_id = ?`,
		projectID, userID,
	).Scan(&n)
	if err != nil {
		return err
	}
	return nil
}
