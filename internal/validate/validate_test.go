package validate

import "testing"

func TestDSN(t *testing.T) {
	valid := []string{
		"USER.SOURCE",
		"SYS1.PARMLIB",
		"A.B.C#D",
		"DSN*",
		"HLQ.@MEMBER",
		"A.B$C",
		"DATASET(MEMBER)",
	}
	for _, name := range valid {
		if err := DSN(name); err != nil {
			t.Errorf("DSN(%q) should be valid, got: %v", name, err)
		}
	}

	invalid := []string{
		"",
		"; rm -rf",
		"DSN\r\nQUIT",
		"DSN WITH SPACES",
		"DSN|PIPE",
		"DSN`CMD`",
		"DSN&BG",
		"path/slash",
	}
	for _, name := range invalid {
		if err := DSN(name); err == nil {
			t.Errorf("DSN(%q) should be invalid", name)
		}
	}
}

func TestUSSPath(t *testing.T) {
	valid := []string{
		"/u/user/file.txt",
		"/tmp/test",
		"/u/user/my file.txt",
		"/u/user/.hidden",
	}
	for _, path := range valid {
		if err := USSPath(path); err != nil {
			t.Errorf("USSPath(%q) should be valid, got: %v", path, err)
		}
	}

	invalid := []string{
		"",
		"/tmp/$(cmd)",
		"/tmp/file;rm",
		"/tmp/`whoami`",
		"/tmp/a|b",
		"/tmp/a&b",
		"/tmp/a>b",
		"/tmp/a<b",
		"/tmp/a\rb",
		"/tmp/a\nb",
	}
	for _, path := range invalid {
		if err := USSPath(path); err == nil {
			t.Errorf("USSPath(%q) should be invalid", path)
		}
	}
}

func TestJobID(t *testing.T) {
	valid := []string{
		"JOB12345",
		"JOB00001",
		"JOB0001",
	}
	for _, id := range valid {
		if err := JobID(id); err != nil {
			t.Errorf("JobID(%q) should be valid, got: %v", id, err)
		}
	}

	invalid := []string{
		"",
		"JOB",
		"NOTAJOB",
		"JOB123AB",
		"TSU12345",
		"job",
	}
	for _, id := range invalid {
		if err := JobID(id); err == nil {
			t.Errorf("JobID(%q) should be invalid", id)
		}
	}
}

func TestOwner(t *testing.T) {
	valid := []string{
		"USER",
		"USER01",
		"A",
		"USR@HCL",
		"USR#01",
		"USR$X",
		"*",
	}
	for _, owner := range valid {
		if err := Owner(owner); err != nil {
			t.Errorf("Owner(%q) should be valid, got: %v", owner, err)
		}
	}

	invalid := []string{
		"",
		"USER\r\n",
		"; DROP",
		"USER NAME",
		"USER|CMD",
	}
	for _, owner := range invalid {
		if err := Owner(owner); err == nil {
			t.Errorf("Owner(%q) should be invalid", owner)
		}
	}
}
