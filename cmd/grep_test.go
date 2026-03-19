package cmd

import (
	"testing"
)

func TestBuildMatcher(t *testing.T) {
	tests := []struct {
		name            string
		pattern         string
		isRegex         bool
		caseInsensitive bool
		input           string
		want            bool
	}{
		{"literal match", "HELLO", false, false, "  MOVE 'HELLO' TO WS-OUT", true},
		{"literal no match", "HELLO", false, false, "  MOVE 'WORLD' TO WS-OUT", false},
		{"case insensitive", "hello", false, true, "  MOVE 'HELLO' TO WS-OUT", true},
		{"case sensitive no match", "hello", false, false, "  MOVE 'HELLO' TO WS-OUT", false},
		{"regex match", "PERFORM.*PARA", true, false, "  PERFORM SECTION-PARA", true},
		{"regex no match", "^PERFORM$", true, false, "  PERFORM SOMETHING", false},
		{"regex case insensitive", "perform", true, true, "  PERFORM SOMETHING", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matcher, err := buildMatcher(tt.pattern, tt.isRegex, tt.caseInsensitive)
			if err != nil {
				t.Fatalf("buildMatcher error: %v", err)
			}
			if got := matcher(tt.input); got != tt.want {
				t.Errorf("matcher(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestBuildMatcherInvalidRegex(t *testing.T) {
	_, err := buildMatcher("[invalid", true, false)
	if err == nil {
		t.Error("expected error for invalid regex")
	}
}

func TestSearchLines(t *testing.T) {
	content := "LINE ONE\nLINE TWO\nLINE THREE\nLINE FOUR"
	matcher := func(line string) bool {
		return len(line) > 0 && (line == "LINE TWO" || line == "LINE FOUR")
	}

	matches := searchLines(content, matcher)

	if len(matches) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(matches))
	}

	if matches[0].Line != 2 || matches[0].Text != "LINE TWO" {
		t.Errorf("match 0: got line %d %q, want line 2 %q", matches[0].Line, matches[0].Text, "LINE TWO")
	}
	if matches[1].Line != 4 || matches[1].Text != "LINE FOUR" {
		t.Errorf("match 1: got line %d %q, want line 4 %q", matches[1].Line, matches[1].Text, "LINE FOUR")
	}
}

func TestParseGrepOutput(t *testing.T) {
	output := "10:  MOVE 'HELLO' TO WS-OUT\n42:  PERFORM PARA-X\n"
	matches := parseGrepOutput(output, "MYPROG")

	if len(matches) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(matches))
	}

	if matches[0].Line != 10 {
		t.Errorf("match 0 line: got %d, want 10", matches[0].Line)
	}
	if matches[0].Text != "  MOVE 'HELLO' TO WS-OUT" {
		t.Errorf("match 0 text: got %q", matches[0].Text)
	}
	if matches[1].Line != 42 {
		t.Errorf("match 1 line: got %d, want 42", matches[1].Line)
	}
}
