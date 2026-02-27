package connection

import (
	"testing"
)

func TestParseZOSMFJobs(t *testing.T) {
	items := []jobsListResponse{
		{
			JobID:   "JOB12345",
			JobName: "MYJOB",
			Owner:   "FALZONE",
			Status:  "OUTPUT",
			RetCode: "CC 0000",
			Class:   "A",
		},
		{
			JobID:   "JOB12346",
			JobName: "MYJOB2",
			Owner:   "FALZONE",
			Status:  "ACTIVE",
			RetCode: "",
			Class:   "B",
		},
	}

	jobs := parseZOSMFJobs(items)

	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}

	if jobs[0].JobID != "JOB12345" {
		t.Errorf("JobID = %q, want JOB12345", jobs[0].JobID)
	}
	if jobs[0].JobName != "MYJOB" {
		t.Errorf("JobName = %q, want MYJOB", jobs[0].JobName)
	}
	if jobs[0].Status != "OUTPUT" {
		t.Errorf("Status = %q, want OUTPUT", jobs[0].Status)
	}
	if jobs[0].RetCode != "CC 0000" {
		t.Errorf("RetCode = %q, want CC 0000", jobs[0].RetCode)
	}

	if jobs[1].Status != "ACTIVE" {
		t.Errorf("Status = %q, want ACTIVE", jobs[1].Status)
	}
	if jobs[1].RetCode != "" {
		t.Errorf("RetCode = %q, want empty", jobs[1].RetCode)
	}
}

func TestParseZOSMFJobsEmpty(t *testing.T) {
	jobs := parseZOSMFJobs(nil)
	if len(jobs) != 0 {
		t.Errorf("expected 0 jobs, got %d", len(jobs))
	}
}

func TestNewZOSMFConnection(t *testing.T) {
	conn := NewZOSMFConnection("host.example.com", 443, "user", "pass")
	if conn.host != "host.example.com" {
		t.Errorf("host = %q, want host.example.com", conn.host)
	}
	if conn.port != 443 {
		t.Errorf("port = %d, want 443", conn.port)
	}
}

func TestNewConnection(t *testing.T) {
	tests := []struct {
		name     string
		protocol string
		wantErr  bool
	}{
		{"zosmf", "zosmf", false},
		{"ftp", "ftp", false},
		{"invalid", "telnet", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewConnection("host", 443, "user", "pass", tt.protocol)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewConnection() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
