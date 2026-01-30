package connection

import (
	"strings"
	"testing"
)

func TestParseMemberLine(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		expected Member
	}{
		{
			name: "standard line",
			line: "HSISAPIE  01.82 2024/04/16 2025/12/10 20:18     5    27     0 FALZONE",
			expected: Member{
				Name:    "HSISAPIE",
				VV:      1,
				MM:      82,
				Created: "2024/04/16",
				Changed: "2025/12/10 20:18",
				Size:    5,
				Init:    27,
				Mod:     0,
				User:    "FALZONE",
			},
		},
		{
			name: "different version",
			line: "MYPROG    02.01 2023/01/01 2024/06/15 10:30   100   100    10 USER123",
			expected: Member{
				Name:    "MYPROG",
				VV:      2,
				MM:      1,
				Created: "2023/01/01",
				Changed: "2024/06/15 10:30",
				Size:    100,
				Init:    100,
				Mod:     10,
				User:    "USER123",
			},
		},
		{
			name:     "too few fields",
			line:     "MEMBER 01.00",
			expected: Member{},
		},
		{
			name:     "empty line",
			line:     "",
			expected: Member{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseMemberLine(tt.line)
			if got != tt.expected {
				t.Errorf("parseMemberLine(%q) = %+v, want %+v", tt.line, got, tt.expected)
			}
		})
	}
}

func TestParseMemberList(t *testing.T) {
	input := ` Name     VV.MM   Created       Changed      Size  Init   Mod   Id
MEMBER1   01.00 2024/01/01 2024/01/15 09:00    10    10     0 USER1
MEMBER2   02.05 2023/06/01 2024/02/20 14:30    50    40    10 USER2

`
	f := &FTPConnection{}
	members, err := f.parseMemberList(strings.NewReader(input))
	if err != nil {
		t.Fatalf("parseMemberList error: %v", err)
	}

	if len(members) != 2 {
		t.Fatalf("expected 2 members, got %d", len(members))
	}

	if members[0].Name != "MEMBER1" {
		t.Errorf("first member name = %q, want MEMBER1", members[0].Name)
	}
	if members[1].Name != "MEMBER2" {
		t.Errorf("second member name = %q, want MEMBER2", members[1].Name)
	}
	if members[1].VV != 2 || members[1].MM != 5 {
		t.Errorf("second member version = %d.%d, want 2.5", members[1].VV, members[1].MM)
	}
}

func TestParseMemberListFromDebug(t *testing.T) {
	debug := `< 220-FTP server ready
> USER testuser
< 331 Password required
> PASS ****
< 230 User logged in
> CWD 'TEST.PDS'
< 250 Directory changed
> LIST
< 125 List started
 Name     VV.MM   Created       Changed      Size  Init   Mod   Id
PROG1     01.00 2024/01/01 2024/01/15 09:00    10    10     0 USER1
PROG2     01.05 2024/02/01 2024/03/15 10:00    20    15     5 USER2
< 250 List completed
> QUIT
< 221 Goodbye
`
	f := &FTPConnection{}
	members, err := f.parseMemberListFromDebug(debug)
	if err != nil {
		t.Fatalf("parseMemberListFromDebug error: %v", err)
	}

	if len(members) != 2 {
		t.Fatalf("expected 2 members, got %d", len(members))
	}

	if members[0].Name != "PROG1" {
		t.Errorf("first member = %q, want PROG1", members[0].Name)
	}
	if members[1].Name != "PROG2" {
		t.Errorf("second member = %q, want PROG2", members[1].Name)
	}
}

func TestParseMemberListFromDebugEmpty(t *testing.T) {
	debug := `< 125 List started
 Name     VV.MM   Created       Changed      Size  Init   Mod   Id
< 250 List completed
`
	f := &FTPConnection{}
	members, err := f.parseMemberListFromDebug(debug)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	if len(members) != 0 {
		t.Errorf("expected 0 members, got %d", len(members))
	}
}

func TestParseJobLine(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		expected JobStatus
	}{
		{
			name: "output with RC",
			line: "MYJOB    JOB12345 FALZONE  OUTPUT A    RC=0000",
			expected: JobStatus{
				JobName: "MYJOB",
				JobID:   "JOB12345",
				Owner:   "FALZONE",
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

func TestParsePASV(t *testing.T) {
	tests := []struct {
		resp    string
		want    string
		wantErr bool
	}{
		{
			resp: "227 Entering Passive Mode (192,168,1,1,4,1)",
			want: "192.168.1.1:1025",
		},
		{
			resp: "227 Entering Passive Mode (10,0,0,1,39,16)",
			want: "10.0.0.1:10000",
		},
		{
			resp:    "500 Invalid command",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.resp, func(t *testing.T) {
			got, err := parsePASV(tt.resp)
			if (err != nil) != tt.wantErr {
				t.Errorf("parsePASV() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("parsePASV() = %q, want %q", got, tt.want)
			}
		})
	}
}
