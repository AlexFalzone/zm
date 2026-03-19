package connection

import (
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
