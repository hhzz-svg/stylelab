package httpapi

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"
)

type exportNovelChapter struct {
	Seq       int    `json:"seq"`
	Title     string `json:"title"`
	Brief     string `json:"brief"`
	Body      string `json:"body"`
	Summary   string `json:"summary"`
	Status    string `json:"status"`
	RuneCount int    `json:"rune_count"`
}

func (s *Server) handleExportNovelMulti(w http.ResponseWriter, r *http.Request) {
	userID, err := s.currentUserID(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}
	projectID := r.PathValue("id")
	var projectName string
	err = s.st.DB().QueryRowContext(
		r.Context(),
		`SELECT name FROM projects WHERE id = ? AND user_id = ?`,
		projectID, userID,
	).Scan(&projectName)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "not_found", "project not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "invalid", "internal error")
		return
	}

	format := strings.ToLower(r.URL.Query().Get("format"))
	if format == "" {
		format = "txt"
	}

	rows, err := s.st.DB().QueryContext(
		r.Context(),
		`SELECT seq, title, brief, body, summary, status FROM chapters WHERE project_id = ? ORDER BY seq ASC`,
		projectID,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "invalid", "internal error")
		return
	}
	defer rows.Close()

	var chapters []exportNovelChapter
	totalRunes := 0
	writtenCount := 0

	for rows.Next() {
		var c exportNovelChapter
		if err := rows.Scan(&c.Seq, &c.Title, &c.Brief, &c.Body, &c.Summary, &c.Status); err == nil {
			c.RuneCount = utf8.RuneCountInString(c.Body)
			totalRunes += c.RuneCount
			if c.Status == "written" {
				writtenCount++
			}
			chapters = append(chapters, c)
		}
	}

	switch format {
	case "report":
		writeJSON(w, http.StatusOK, map[string]any{
			"project_id":     projectID,
			"project_name":   projectName,
			"total_runes":    totalRunes,
			"total_chapters": len(chapters),
			"written_count":  writtenCount,
			"chapters":       chapters,
		})
		return

	case "md":
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("# 《%s》\n\n", projectName))
		sb.WriteString(fmt.Sprintf("> 全书总字数：%d 字 · 共 %d 章（已成篇 %d 章）\n", totalRunes, len(chapters), writtenCount))
		sb.WriteString(fmt.Sprintf("> 导出时间：%s\n\n", time.Now().Format("2006-01-02 15:04")))
		sb.WriteString("## 目录索引\n\n")
		for _, ch := range chapters {
			sb.WriteString(fmt.Sprintf("- [第 %d 章 %s](#chapter-%d) (%d 字)\n", ch.Seq, ch.Title, ch.Seq, ch.RuneCount))
		}
		sb.WriteString("\n---\n\n")

		for _, ch := range chapters {
			sb.WriteString(fmt.Sprintf("<a id=\"chapter-%d\"></a>\n", ch.Seq))
			sb.WriteString(fmt.Sprintf("## 第 %d 章 %s\n\n", ch.Seq, ch.Title))
			if ch.Brief != "" {
				sb.WriteString(fmt.Sprintf("*【大纲梗概】%s*\n\n", ch.Brief))
			}
			if strings.TrimSpace(ch.Body) != "" {
				sb.WriteString(ch.Body + "\n\n")
			} else {
				sb.WriteString("*（本章尚未成篇）*\n\n")
			}
			sb.WriteString("---\n\n")
		}

		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.md\"", projectName))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(sb.String()))
		return

	default: // standard txt
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("《%s》\n", projectName))
		sb.WriteString(fmt.Sprintf("全书总字数：%d 字 | 总章节数：%d 章\n", totalRunes, len(chapters)))
		sb.WriteString(fmt.Sprintf("导出时间：%s\n", time.Now().Format("2006-01-02 15:04")))
		sb.WriteString("================================================================\n\n")

		for _, ch := range chapters {
			sb.WriteString(fmt.Sprintf("第%d章 %s\n\n", ch.Seq, ch.Title))
			if strings.TrimSpace(ch.Body) != "" {
				lines := strings.Split(ch.Body, "\n")
				for _, line := range lines {
					trimmed := strings.TrimSpace(line)
					if trimmed != "" {
						sb.WriteString("    " + trimmed + "\n\n")
					}
				}
			} else {
				sb.WriteString("    （本章尚未撰写正文）\n\n")
			}
			sb.WriteString("\n----------------------------------------------------------------\n\n")
		}

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.txt\"", projectName))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(sb.String()))
		return
	}
}
