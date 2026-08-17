package stylestat_test

import (
	"strings"
	"testing"
	"unicode/utf8"

	"stylelab/internal/stylestat"
)

func TestComputeSampleTooSmall(t *testing.T) {
	got := stylestat.Compute(stylestat.Input{
		Chapters: []string{"一", "二", "三", "四"},
	})
	if got == nil {
		t.Fatal("expected stats, got nil")
	}
	if got.Chapters != 4 {
		t.Fatalf("Chapters: got %d want 4", got.Chapters)
	}
	if !got.SampleTooSmall {
		t.Fatal("SampleTooSmall: want true")
	}
}

func TestComputeCorrectionPattern(t *testing.T) {
	chapters := make([]string, 5)
	for i := range chapters {
		chapters[i] = "不是快乐，而是悲伤。"
	}
	got := stylestat.Compute(stylestat.Input{Chapters: chapters})
	if got == nil {
		t.Fatal("expected stats, got nil")
	}
	const name = "矫正句『不是…(而)是…』"
	for _, p := range got.Patterns {
		if p.Name == name {
			if p.Total < 5 {
				t.Fatalf("%s Total: got %d want ≥ 5", name, p.Total)
			}
			return
		}
	}
	t.Fatalf("Patterns missing %s: %+v", name, got.Patterns)
}

func TestComputeEndingShortRatio(t *testing.T) {
	short := strings.Repeat("短", 10)
	long := strings.Repeat("长", 80)
	if utf8.RuneCountInString(short) != 10 {
		t.Fatalf("short fixture: %d runes", utf8.RuneCountInString(short))
	}
	if utf8.RuneCountInString(long) != 80 {
		t.Fatalf("long fixture: %d runes", utf8.RuneCountInString(long))
	}

	// 3 short (≤30) + 2 long (>30) → ShortRatio 0.6
	got := stylestat.Compute(stylestat.Input{
		Chapters: []string{short, short, short, long, long},
	})
	if got == nil {
		t.Fatal("expected stats, got nil")
	}
	if got.Ending.ShortRatio != 0.6 {
		t.Fatalf("3 short + 2 long ShortRatio: got %v want 0.6", got.Ending.ShortRatio)
	}

	// all short endings
	allShort := stylestat.Compute(stylestat.Input{
		Chapters: []string{short, short, short, short, short},
	})
	if allShort.Ending.ShortRatio != 1 {
		t.Fatalf("all-short ShortRatio: got %v want 1", allShort.Ending.ShortRatio)
	}

	// all long endings
	allLong := stylestat.Compute(stylestat.Input{
		Chapters: []string{long, long, long, long, long},
	})
	if allLong.Ending.ShortRatio != 0 {
		t.Fatalf("all-long ShortRatio: got %v want 0", allLong.Ending.ShortRatio)
	}
}
