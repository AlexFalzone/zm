package diff

import (
	"strings"
	"testing"
)

func TestDiffIdentical(t *testing.T) {
	lines := []string{"line one", "line two", "line three"}
	hunks := Diff(lines, lines, 3)
	if len(hunks) != 0 {
		t.Errorf("expected no hunks for identical input, got %d", len(hunks))
	}
}

func TestDiffAdded(t *testing.T) {
	a := []string{"line one", "line three"}
	b := []string{"line one", "line two", "line three"}
	hunks := Diff(a, b, 3)

	if len(hunks) != 1 {
		t.Fatalf("expected 1 hunk, got %d", len(hunks))
	}

	hasInsert := false
	for _, line := range hunks[0].Lines {
		if line.Op == OpInsert && line.Text == "line two" {
			hasInsert = true
		}
	}
	if !hasInsert {
		t.Error("expected insert of 'line two'")
	}
}

func TestDiffDeleted(t *testing.T) {
	a := []string{"line one", "line two", "line three"}
	b := []string{"line one", "line three"}
	hunks := Diff(a, b, 3)

	if len(hunks) != 1 {
		t.Fatalf("expected 1 hunk, got %d", len(hunks))
	}

	hasDelete := false
	for _, line := range hunks[0].Lines {
		if line.Op == OpDelete && line.Text == "line two" {
			hasDelete = true
		}
	}
	if !hasDelete {
		t.Error("expected delete of 'line two'")
	}
}

func TestDiffChanged(t *testing.T) {
	a := []string{"line one", "line OLD", "line three"}
	b := []string{"line one", "line NEW", "line three"}
	hunks := Diff(a, b, 3)

	if len(hunks) != 1 {
		t.Fatalf("expected 1 hunk, got %d", len(hunks))
	}

	hasDelete, hasInsert := false, false
	for _, line := range hunks[0].Lines {
		if line.Op == OpDelete && line.Text == "line OLD" {
			hasDelete = true
		}
		if line.Op == OpInsert && line.Text == "line NEW" {
			hasInsert = true
		}
	}
	if !hasDelete || !hasInsert {
		t.Errorf("expected delete of 'line OLD' and insert of 'line NEW', got delete=%v insert=%v", hasDelete, hasInsert)
	}
}

func TestFormatUnified(t *testing.T) {
	a := []string{"alpha", "beta", "gamma"}
	b := []string{"alpha", "BETA", "gamma"}
	hunks := Diff(a, b, 3)

	out := FormatUnified("file_a", "file_b", hunks)

	if !strings.HasPrefix(out, "--- file_a\n+++ file_b\n") {
		t.Error("missing unified diff header")
	}
	if !strings.Contains(out, "@@") {
		t.Error("missing hunk header")
	}
	if !strings.Contains(out, "-beta") {
		t.Error("missing delete line")
	}
	if !strings.Contains(out, "+BETA") {
		t.Error("missing insert line")
	}
}

func TestFormatUnifiedEmpty(t *testing.T) {
	out := FormatUnified("a", "b", nil)
	if out != "" {
		t.Errorf("expected empty output for nil hunks, got %q", out)
	}
}

func TestDiffContext(t *testing.T) {
	a := []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10"}
	b := []string{"1", "2", "3", "4", "CHANGED", "6", "7", "8", "9", "10"}

	hunks := Diff(a, b, 1)
	if len(hunks) != 1 {
		t.Fatalf("expected 1 hunk, got %d", len(hunks))
	}

	// With context=1, hunk should include lines 4,5,6 from A
	if hunks[0].CountA != 3 {
		t.Errorf("expected CountA=3, got %d", hunks[0].CountA)
	}
}
