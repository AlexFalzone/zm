package cmd

import "testing"

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
