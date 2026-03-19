package connection

import (
	"testing"
)

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

func TestParseUSSLine(t *testing.T) {
	tests := []struct {
		name string
		line string
		want USSFile
	}{
		{
			name: "directory",
			line: "drwxr-xr-x   2 FALZONE  SYS1        8192 Mar 12 10:20 analyzer",
			want: USSFile{Name: "analyzer", Type: "directory", Size: 8192, Mode: "drwxr-xr-x", User: "FALZONE", Group: "SYS1", Mtime: "Mar 12 10:20"},
		},
		{
			name: "regular file",
			line: "-rw-r--r--   1 FALZONE  SYS1        1884 Mar 12 10:16 Makefile",
			want: USSFile{Name: "Makefile", Type: "file", Size: 1884, Mode: "-rw-r--r--", User: "FALZONE", Group: "SYS1", Mtime: "Mar 12 10:16"},
		},
		{
			name: "symlink",
			line: "lrwxrwxrwx   1 FALZONE  SYS1          15 Jan 20 09:00 link -> target",
			want: USSFile{Name: "link", Type: "symlink", Size: 15, Mode: "lrwxrwxrwx", User: "FALZONE", Group: "SYS1", Mtime: "Jan 20 09:00"},
		},
		{
			name: "filename with space",
			line: "-rw-r--r--   1 FALZONE  SYS1         100 Jan 20 09:00 my file.txt",
			want: USSFile{Name: "my file.txt", Type: "file", Size: 100, Mode: "-rw-r--r--", User: "FALZONE", Group: "SYS1", Mtime: "Jan 20 09:00"},
		},
		{
			name: "too few fields",
			line: "drwx  2 FALZONE",
			want: USSFile{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseUSSLine(tt.line)
			if got != tt.want {
				t.Errorf("parseUSSLine(%q)\n  got:  %+v\n  want: %+v", tt.line, got, tt.want)
			}
		})
	}
}
