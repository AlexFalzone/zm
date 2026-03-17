package cmd

import (
	"bytes"
	"fmt"
	"os"
	"testing"
)

func TestParseDSN(t *testing.T) {
	tests := []struct {
		name        string
		dsn         string
		wantDataset string
		wantMember  string
		wantErr     bool
	}{
		{
			name:        "simple",
			dsn:         "USER.SOURCE(MYPROG)",
			wantDataset: "USER.SOURCE",
			wantMember:  "MYPROG",
		},
		{
			name:        "with quotes",
			dsn:         "'USER.SOURCE(MYPROG)'",
			wantDataset: "USER.SOURCE",
			wantMember:  "MYPROG",
		},
		{
			name:        "long qualifier",
			dsn:         "SYS1.MACLIB(ABEND)",
			wantDataset: "SYS1.MACLIB",
			wantMember:  "ABEND",
		},
		{
			name:        "multiple qualifiers",
			dsn:         "USER.TEST.COBOL(PROG001)",
			wantDataset: "USER.TEST.COBOL",
			wantMember:  "PROG001",
		},
		{
			name:    "no member",
			dsn:     "USER.SOURCE",
			wantErr: true,
		},
		{
			name:    "empty parentheses",
			dsn:     "USER.SOURCE()",
			wantErr: true,
		},
		{
			name:    "missing close paren",
			dsn:     "USER.SOURCE(MEMBER",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dataset, member, err := parseDSN(tt.dsn)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseDSN(%q) error = %v, wantErr %v", tt.dsn, err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}
			if dataset != tt.wantDataset {
				t.Errorf("dataset = %q, want %q", dataset, tt.wantDataset)
			}
			if member != tt.wantMember {
				t.Errorf("member = %q, want %q", member, tt.wantMember)
			}
		})
	}
}

func TestTrimQuotes(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"'quoted'", "quoted"},
		{"noquotes", "noquotes"},
		{"'single", "'single"},
		{"single'", "single'"},
		{"''", ""},
		{"", ""},
		{"'a'", "a"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := trimQuotes(tt.input); got != tt.want {
				t.Errorf("trimQuotes(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestMatchWildcard(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		pattern string
		want    bool
	}{
		{"exact match", "MYPROG", "MYPROG", true},
		{"star suffix", "PROG001", "PROG*", true},
		{"star prefix", "PROG001", "*001", true},
		{"star middle", "ABCDEF", "A*F", true},
		{"no match", "MYPROG", "OTHER*", false},
		{"question mark", "PROG1", "PROG?", true},
		{"case insensitive via uppercase", "MYPROG", "MYPROG", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchWildcard(tt.input, tt.pattern); got != tt.want {
				t.Errorf("matchWildcard(%q, %q) = %v, want %v", tt.input, tt.pattern, got, tt.want)
			}
		})
	}
}

func TestFilterByDD(t *testing.T) {
	input := "--- DD: JESMSGLG (Step: JES2) ---\nlog line 1\nlog line 2\n--- DD: JESJCL (Step: JES2) ---\njcl line 1\n--- DD: SYSPRINT (Step: STEP1) ---\nprint line 1\n"

	tests := []struct {
		name string
		dd   string
		want string
	}{
		{
			name: "filter JESMSGLG",
			dd:   "JESMSGLG",
			want: "--- DD: JESMSGLG (Step: JES2) ---\nlog line 1\nlog line 2\n",
		},
		{
			name: "filter SYSPRINT",
			dd:   "SYSPRINT",
			want: "--- DD: SYSPRINT (Step: STEP1) ---\nprint line 1\n\n",
		},
		{
			name: "case insensitive",
			dd:   "jesjcl",
			want: "--- DD: JESJCL (Step: JES2) ---\njcl line 1\n",
		},
		{
			name: "no match",
			dd:   "NOTFOUND",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := filterByDD(input, tt.dd); got != tt.want {
				t.Errorf("filterByDD dd=%q:\ngot:  %q\nwant: %q", tt.dd, got, tt.want)
			}
		})
	}
}

func captureStdout(f func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	f()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	return buf.String()
}

func TestPrintContent(t *testing.T) {
	content := "line one\nline two\nline three\nline four\nline five\n"

	tests := []struct {
		name  string
		lines bool
		head  int
		tail  int
		want  string
	}{
		{
			name: "plain output",
			want: "line one\nline two\nline three\nline four\nline five\n",
		},
		{
			name:  "with line numbers",
			lines: true,
			want:  "     1  line one\n     2  line two\n     3  line three\n     4  line four\n     5  line five\n",
		},
		{
			name: "head 2",
			head: 2,
			want: "line one\nline two\n",
		},
		{
			name: "tail 2",
			tail: 2,
			want: "line four\nline five\n",
		},
		{
			name:  "head with line numbers",
			lines: true,
			head:  3,
			want:  fmt.Sprintf("%6d  line one\n%6d  line two\n%6d  line three\n", 1, 2, 3),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := captureStdout(func() {
				printContent(content, tt.lines, tt.head, tt.tail)
			})
			if got != tt.want {
				t.Errorf("printContent:\ngot:  %q\nwant: %q", got, tt.want)
			}
		})
	}
}
