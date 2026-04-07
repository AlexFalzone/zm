package connection

import (
	"testing"
)

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
	members, err := parseMemberListFromDebug(debug)
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
	members, err := parseMemberListFromDebug(debug)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	if len(members) != 0 {
		t.Errorf("expected 0 members, got %d", len(members))
	}
}

func TestParseMemberListFromDebugNoStartMarker(t *testing.T) {
	debug := `< 220-FTP server ready
> USER testuser
< 230 User logged in
`
	_, err := parseMemberListFromDebug(debug)
	if err == nil {
		t.Error("expected error for missing start marker, got nil")
	}
}

func TestParseMemberListFromDebug150Marker(t *testing.T) {
	debug := `> LIST
< 150 Opening ASCII mode
MEMBER1   01.00 2024/01/01 2024/01/15 09:00    10    10     0 USER1
< 226 Transfer complete
`
	members, err := parseMemberListFromDebug(debug)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	if len(members) != 1 {
		t.Fatalf("expected 1 member, got %d", len(members))
	}
	if members[0].Name != "MEMBER1" {
		t.Errorf("member = %q, want MEMBER1", members[0].Name)
	}
}

func TestParseMemberListFromDebugNoEndMarker(t *testing.T) {
	debug := `< 125 List started
PROG1     01.00 2024/01/01 2024/01/15 09:00    10    10     0 USER1
`
	members, err := parseMemberListFromDebug(debug)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	if len(members) != 1 {
		t.Fatalf("expected 1 member, got %d", len(members))
	}
	if members[0].Name != "PROG1" {
		t.Errorf("member = %q, want PROG1", members[0].Name)
	}
}

func TestParseUSSListFromDebugNoStartMarker(t *testing.T) {
	debug := `< 220-FTP server ready
> USER testuser
< 230 User logged in
`
	_, err := parseUSSListFromDebug(debug)
	if err == nil {
		t.Error("expected error for missing start marker, got nil")
	}
}

func TestParseUSSListFromDebugEmptyData(t *testing.T) {
	debug := `< 125 List started
total 0
< 250 List completed
`
	files, err := parseUSSListFromDebug(debug)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("expected 0 files, got %d", len(files))
	}
}

func TestParseUSSListFromDebugRealOutput(t *testing.T) {
	debug := `> LIST -a
< 125 List started
total 48
drwxr-xr-x   2 USER  SYS1        8192 Mar 12 10:20 src
-rw-r--r--   1 USER  SYS1        1884 Mar 12 10:16 Makefile
< 250 List completed
`
	files, err := parseUSSListFromDebug(debug)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}
	if files[0].Name != "src" || files[0].Type != "directory" {
		t.Errorf("first file = %+v, want src/directory", files[0])
	}
	if files[1].Name != "Makefile" || files[1].Type != "file" {
		t.Errorf("second file = %+v, want Makefile/file", files[1])
	}
}
