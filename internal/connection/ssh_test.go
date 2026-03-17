package connection

import (
	"testing"
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
FALZONE.JCL
FALZONE.SOURCE
FALZONE.LOAD
READY
END`,
			want: []string{"FALZONE.JCL", "FALZONE.SOURCE", "FALZONE.LOAD"},
		},
		{
			name: "LISTCAT output",
			output: `THE FOLLOWING WAS FOUND
FALZONE.TEST.DATA
FALZONE.TEST.JCL
READY`,
			want: []string{"FALZONE.TEST.DATA", "FALZONE.TEST.JCL"},
		},
		{
			name:   "empty output",
			output: "READY\nEND\n",
			want:   nil,
		},
		{
			name: "with dashes and status lines",
			output: `LISTDS 'FALZONE.*'
---RECFM-LRECL-BLKSIZE
FALZONE.DATA
READY`,
			want: []string{"FALZONE.DATA"},
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

func TestParseListMembersOutput(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   []string
	}{
		{
			name: "standard output with members",
			output: `FALZONE.SOURCE
--RECFM-LRECL-BLKSIZE-DSORG
FB    80    27920   PO
--MEMBERS--
PROG1
PROG2
MAIN
READY`,
			want: []string{"PROG1", "PROG2", "MAIN"},
		},
		{
			name: "empty members",
			output: `FALZONE.EMPTY
--RECFM-LRECL-BLKSIZE-DSORG
FB    80    27920   PO
--MEMBERS--
READY`,
			want: nil,
		},
		{
			name:   "no members section",
			output: "FALZONE.SEQ\nREADY\n",
			want:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseListMembersOutput(tt.output)
			if len(got) != len(tt.want) {
				t.Fatalf("parseListMembersOutput() returned %d members, want %d", len(got), len(tt.want))
			}
			for i, m := range got {
				if m.Name != tt.want[i] {
					t.Errorf("member[%d].Name = %q, want %q", i, m.Name, tt.want[i])
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

func TestParseStatusOutput(t *testing.T) {
	tests := []struct {
		name   string
		output string
		owner  string
		want   []JobStatus
	}{
		{
			name:   "output queue",
			output: "JOB MYJOB(JOB12345) ON OUTPUT QUEUE\n",
			owner:  "FALZONE",
			want: []JobStatus{
				{JobID: "JOB12345", JobName: "MYJOB", Owner: "FALZONE", Status: "OUTPUT"},
			},
		},
		{
			name:   "executing",
			output: "JOB TESTJOB(JOB00001) EXECUTING\n",
			owner:  "FALZONE",
			want: []JobStatus{
				{JobID: "JOB00001", JobName: "TESTJOB", Owner: "FALZONE", Status: "ACTIVE"},
			},
		},
		{
			name: "multiple jobs",
			output: `READY
JOB BUILD(JOB00100) ON OUTPUT QUEUE
JOB COMPILE(JOB00101) EXECUTING
READY`,
			owner: "USER1",
			want: []JobStatus{
				{JobID: "JOB00100", JobName: "BUILD", Owner: "USER1", Status: "OUTPUT"},
				{JobID: "JOB00101", JobName: "COMPILE", Owner: "USER1", Status: "ACTIVE"},
			},
		},
		{
			name:   "no jobs",
			output: "READY\nEND\n",
			owner:  "FALZONE",
			want:   nil,
		},
		{
			name:   "input queue",
			output: "JOB WAIT(JOB99999) ON INPUT QUEUE\n",
			owner:  "FALZONE",
			want: []JobStatus{
				{JobID: "JOB99999", JobName: "WAIT", Owner: "FALZONE", Status: "INPUT"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseStatusOutput(tt.output, tt.owner)
			if len(got) != len(tt.want) {
				t.Fatalf("parseStatusOutput() returned %d jobs, want %d\ngot: %+v", len(got), len(tt.want), got)
			}
			for i, j := range got {
				if j != tt.want[i] {
					t.Errorf("job[%d] = %+v, want %+v", i, j, tt.want[i])
				}
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
		{name: "valid simple", dsn: "FALZONE.JCL", wantErr: false},
		{name: "valid with special chars", dsn: "SYS1.@MACRO#.$DATA", wantErr: false},
		{name: "valid with parens", dsn: "FALZONE.SOURCE(MEMBER)", wantErr: false},
		{name: "valid with wildcard", dsn: "FALZONE.*", wantErr: false},
		{name: "invalid semicolon", dsn: "FALZONE;rm -rf /", wantErr: true},
		{name: "invalid pipe", dsn: "FALZONE|cat /etc/passwd", wantErr: true},
		{name: "invalid backtick", dsn: "FALZONE`id`", wantErr: true},
		{name: "invalid space", dsn: "FALZONE SOURCE", wantErr: true},
		{name: "empty", dsn: "", wantErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDSN(tt.dsn)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateDSN(%q) error = %v, wantErr %v", tt.dsn, err, tt.wantErr)
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
		{name: "valid path", path: "/u/falzone/.bashrc", wantErr: false},
		{name: "valid with subdirs", path: "/u/falzone/src/main.c", wantErr: false},
		{name: "invalid semicolon", path: "/u/falzone; rm -rf /", wantErr: true},
		{name: "invalid command sub", path: "/u/$(whoami)/file", wantErr: true},
		{name: "invalid backtick", path: "/u/`id`/file", wantErr: true},
		{name: "invalid pipe", path: "/u/falzone | cat", wantErr: true},
		{name: "invalid ampersand", path: "/u/falzone & echo", wantErr: true},
		{name: "invalid redirect", path: "/u/falzone > /tmp/out", wantErr: true},
		{name: "empty", path: "", wantErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateUSSPath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateUSSPath(%q) error = %v, wantErr %v", tt.path, err, tt.wantErr)
			}
		})
	}
}

func TestIsJobID(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"JOB12345", true},
		{"JOB00001", true},
		{"JOB1", true},
		{"JOB", false},
		{"JOBNAME", false},
		{"TSU12345", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := isJobID(tt.input); got != tt.want {
				t.Errorf("isJobID(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsDatasetName(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"FALZONE.JCL", true},
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
