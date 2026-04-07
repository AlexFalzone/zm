package connection

import (
	"testing"
)

func TestParseJobLine(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		expected JobStatus
	}{
		{
			name: "output with RC",
			line: "MYJOB    JOB12345 USER  OUTPUT A    RC=0000",
			expected: JobStatus{
				JobName: "MYJOB",
				JobID:   "JOB12345",
				Owner:   "USER",
				Status:  "OUTPUT",
				Class:   "A",
				RetCode: "CC 0000",
			},
		},
		{
			name: "active job",
			line: "TESTJOB  JOB00001 USER1    ACTIVE A",
			expected: JobStatus{
				JobName: "TESTJOB",
				JobID:   "JOB00001",
				Owner:   "USER1",
				Status:  "ACTIVE",
				Class:   "A",
			},
		},
		{
			name: "abend",
			line: "BADJOB   JOB99999 USER2    OUTPUT A    ABEND=S0C7",
			expected: JobStatus{
				JobName: "BADJOB",
				JobID:   "JOB99999",
				Owner:   "USER2",
				Status:  "OUTPUT",
				Class:   "A",
				RetCode: "ABEND S0C7",
			},
		},
		{
			name:     "too few fields",
			line:     "JOB ONLY",
			expected: JobStatus{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseJobLine(tt.line)
			if got != tt.expected {
				t.Errorf("parseJobLine(%q) = %+v, want %+v", tt.line, got, tt.expected)
			}
		})
	}
}

func TestParseJobLines(t *testing.T) {
	lines := []string{
		"JOBNAME  JOBID    OWNER    STATUS CLASS",
		"JOB1     JOB00001 USER1    OUTPUT A    RC=0000",
		"JOB2     JOB00002 USER1    ACTIVE B",
		"",
	}

	jobs := parseJobLines(lines)
	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}

	if jobs[0].JobName != "JOB1" {
		t.Errorf("first job name = %q, want JOB1", jobs[0].JobName)
	}
	if jobs[1].Status != "ACTIVE" {
		t.Errorf("second job status = %q, want ACTIVE", jobs[1].Status)
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
			owner:  "USER",
			want: []JobStatus{
				{JobID: "JOB12345", JobName: "MYJOB", Owner: "USER", Status: "OUTPUT"},
			},
		},
		{
			name:   "executing",
			output: "JOB TESTJOB(JOB00001) EXECUTING\n",
			owner:  "USER",
			want: []JobStatus{
				{JobID: "JOB00001", JobName: "TESTJOB", Owner: "USER", Status: "ACTIVE"},
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
			owner:  "USER",
			want:   nil,
		},
		{
			name:   "input queue",
			output: "JOB WAIT(JOB99999) ON INPUT QUEUE\n",
			owner:  "USER",
			want: []JobStatus{
				{JobID: "JOB99999", JobName: "WAIT", Owner: "USER", Status: "INPUT"},
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
