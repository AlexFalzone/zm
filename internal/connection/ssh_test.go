package connection

import (
	"testing"

	"zm/internal/validate"
)

func TestParseListDatasetsOutput(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   []string
	}{
		{
			name: "standard LISTDS output",
			output: `READY
USER.JCL
USER.SOURCE
USER.LOAD
READY
END`,
			want: []string{"USER.JCL", "USER.SOURCE", "USER.LOAD"},
		},
		{
			name: "LISTCAT output",
			output: `THE FOLLOWING WAS FOUND
USER.TEST.DATA
USER.TEST.JCL
READY`,
			want: []string{"USER.TEST.DATA", "USER.TEST.JCL"},
		},
		{
			name:   "empty output",
			output: "READY\nEND\n",
			want:   nil,
		},
		{
			name: "with dashes and status lines",
			output: `LISTDS 'USER.*'
---RECFM-LRECL-BLKSIZE
USER.DATA
READY`,
			want: []string{"USER.DATA"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseListDatasetsOutput(tt.output)
			if len(got) != len(tt.want) {
				t.Fatalf("parseListDatasetsOutput() returned %d items, want %d\ngot: %v", len(got), len(tt.want), got)
			}
			for i, g := range got {
				if g != tt.want[i] {
					t.Errorf("item[%d] = %q, want %q", i, g, tt.want[i])
				}
			}
		})
	}
}

func TestParseSubmitOutput(t *testing.T) {
	tests := []struct {
		name    string
		output  string
		want    string
		wantErr bool
	}{
		{
			name:   "standard submit response",
			output: "JOB JOB12345 submitted",
			want:   "JOB12345",
		},
		{
			name:   "with extra text",
			output: "Some preamble\nJOB JOB00001 submitted from /tmp/test.jcl\n",
			want:   "JOB00001",
		},
		{
			name:    "no job ID",
			output:  "Error: file not found",
			wantErr: true,
		},
		{
			name:    "empty output",
			output:  "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseSubmitOutput(tt.output)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseSubmitOutput() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("parseSubmitOutput() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestValidateDSN(t *testing.T) {
	tests := []struct {
		name    string
		dsn     string
		wantErr bool
	}{
		{name: "valid simple", dsn: "USER.JCL", wantErr: false},
		{name: "valid with special chars", dsn: "SYS1.@MACRO#.$DATA", wantErr: false},
		{name: "valid with parens", dsn: "USER.SOURCE(MEMBER)", wantErr: false},
		{name: "valid with wildcard", dsn: "USER.*", wantErr: false},
		{name: "invalid semicolon", dsn: "USER;rm -rf /", wantErr: true},
		{name: "invalid pipe", dsn: "USER|cat /etc/passwd", wantErr: true},
		{name: "invalid backtick", dsn: "USER`id`", wantErr: true},
		{name: "invalid space", dsn: "USER SOURCE", wantErr: true},
		{name: "empty", dsn: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate.DSN(tt.dsn)
			if (err != nil) != tt.wantErr {
				t.Errorf("validate.DSN(%q) error = %v, wantErr %v", tt.dsn, err, tt.wantErr)
			}
		})
	}
}

func TestValidateUSSPath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{name: "valid path", path: "/u/user/.bashrc", wantErr: false},
		{name: "valid with subdirs", path: "/u/user/src/main.c", wantErr: false},
		{name: "invalid semicolon", path: "/u/user; rm -rf /", wantErr: true},
		{name: "invalid command sub", path: "/u/$(whoami)/file", wantErr: true},
		{name: "invalid backtick", path: "/u/`id`/file", wantErr: true},
		{name: "invalid pipe", path: "/u/user | cat", wantErr: true},
		{name: "invalid ampersand", path: "/u/user & echo", wantErr: true},
		{name: "invalid redirect", path: "/u/user > /tmp/out", wantErr: true},
		{name: "empty", path: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate.USSPath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("validate.USSPath(%q) error = %v, wantErr %v", tt.path, err, tt.wantErr)
			}
		})
	}
}

func TestIsDatasetName(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"USER.JCL", true},
		{"SYS1.MACLIB", true},
		{"A.B.C.D", true},
		{"DATA@SET#1.$X", true},
		{"lowercase", false},
		{"HAS SPACE", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := isDatasetName(tt.input); got != tt.want {
				t.Errorf("isDatasetName(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
