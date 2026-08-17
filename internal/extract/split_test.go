package extract_test

import (
	"strings"
	"testing"

	"stylelab/internal/extract"
)

func TestSplitChaptersTwoHeadings(t *testing.T) {
	text := "第一章\n春风过境。\n\n第二章\n夏雨初歇。"
	got := extract.SplitChapters(text)
	if len(got) != 2 {
		t.Fatalf("got %d chapters, want 2: %#v", len(got), got)
	}
	if !strings.Contains(got[0], "春风过境") {
		t.Fatalf("chapter 0 missing body: %q", got[0])
	}
	if !strings.Contains(got[1], "夏雨初歇") {
		t.Fatalf("chapter 1 missing body: %q", got[1])
	}
}

func TestSplitChaptersMarkdownHeadings(t *testing.T) {
	text := "# 第一章 开端\n甲\n\n## 第二章 转折\n乙"
	got := extract.SplitChapters(text)
	if len(got) != 2 {
		t.Fatalf("got %d chapters, want 2: %#v", len(got), got)
	}
}

func TestSplitChaptersSingleParagraph(t *testing.T) {
	text := "只有一段没有标题的正文。"
	got := extract.SplitChapters(text)
	if len(got) != 1 {
		t.Fatalf("got %d chapters, want 1: %#v", len(got), got)
	}
	if got[0] != text {
		t.Fatalf("single chunk should be original text:\n got %q\nwant %q", got[0], text)
	}
}

func TestSplitChaptersBlankLineSeparators(t *testing.T) {
	text := "第一块\n\n\n第二块\n\n\n第三块"
	got := extract.SplitChapters(text)
	if len(got) != 3 {
		t.Fatalf("got %d chapters, want 3: %#v", len(got), got)
	}
	if strings.TrimSpace(got[0]) != "第一块" || strings.TrimSpace(got[1]) != "第二块" || strings.TrimSpace(got[2]) != "第三块" {
		t.Fatalf("blank-split bodies: %#v", got)
	}
}

func TestSplitChaptersDropsEmptyAndSingleFallback(t *testing.T) {
	text := "仅有一块\n\n\n\n\n"
	got := extract.SplitChapters(text)
	if len(got) != 1 {
		t.Fatalf("got %d chapters, want 1 fallback: %#v", len(got), got)
	}
	if got[0] != text {
		t.Fatalf("one remaining chunk should return original text, got %q", got[0])
	}
}
