package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveSourceLocal(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	os.WriteFile(path, []byte("hello\nworld\n"), 0644)

	content, err := resolveSource(nil, path)
	if err != nil {
		t.Fatalf("resolveSource error: %v", err)
	}
	if content != "hello\nworld\n" {
		t.Errorf("got %q, want %q", content, "hello\nworld\n")
	}
}

func TestResolveSourceLocalRelative(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rel.txt")
	os.WriteFile(path, []byte("data"), 0644)

	// Use ./ prefix with absolute path won't work, but test the
	// fallback path that checks os.Stat for non-prefixed paths
	content, err := resolveSource(nil, path)
	if err != nil {
		t.Fatalf("resolveSource error: %v", err)
	}
	if content != "data" {
		t.Errorf("got %q, want %q", content, "data")
	}
}

func TestResolveSourceDSNNoConn(t *testing.T) {
	_, err := resolveSource(nil, "USER.SOURCE(PROG1)")
	if err == nil {
		t.Error("expected error for DSN without connection")
	}
}

func TestSplitLines(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{"empty", "", 0},
		{"one line no newline", "hello", 1},
		{"one line with newline", "hello\n", 1},
		{"two lines", "one\ntwo\n", 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitLines(tt.input)
			if len(got) != tt.want {
				t.Errorf("splitLines(%q) len = %d, want %d", tt.input, len(got), tt.want)
			}
		})
	}
}

func TestIsLocalFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "exists.txt")
	os.WriteFile(path, []byte("x"), 0644)

	tests := []struct {
		name   string
		source string
		want   bool
	}{
		{"relative dot", "./foo.txt", true},
		{"relative dotdot", "../bar.txt", true},
		{"dsn", "USER.SOURCE(PROG1)", false},
		{"existing file", path, true},
		{"nonexistent", "/u/user/nofile", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isLocalFile(tt.source); got != tt.want {
				t.Errorf("isLocalFile(%q) = %v, want %v", tt.source, got, tt.want)
			}
		})
	}
}
