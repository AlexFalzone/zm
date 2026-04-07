package cmd

import (
	"testing"

	"zm/internal/connection"
)

func TestSortJobs(t *testing.T) {
	makeJobs := func() []connection.JobStatus {
		return []connection.JobStatus{
			{JobID: "JOB00003", JobName: "COMPILE", Owner: "SMITH", Status: "OUTPUT", RetCode: "CC 0004"},
			{JobID: "JOB00001", JobName: "ASSEMBLE", Owner: "FALZONE", Status: "ACTIVE", RetCode: ""},
			{JobID: "JOB00002", JobName: "BIND", Owner: "JONES", Status: "INPUT", RetCode: "CC 0000"},
		}
	}

	tests := []struct {
		name  string
		field string
		want  []string // expected order by JobID
	}{
		{
			name:  "sort by jobid",
			field: "jobid",
			want:  []string{"JOB00001", "JOB00002", "JOB00003"},
		},
		{
			name:  "sort by jobname",
			field: "jobname",
			want:  []string{"JOB00001", "JOB00002", "JOB00003"},
		},
		{
			name:  "sort by owner",
			field: "owner",
			want:  []string{"JOB00001", "JOB00002", "JOB00003"},
		},
		{
			name:  "sort by status",
			field: "status",
			want:  []string{"JOB00001", "JOB00002", "JOB00003"},
		},
		{
			name:  "sort by rc",
			field: "rc",
			want:  []string{"JOB00001", "JOB00002", "JOB00003"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jobs := makeJobs()
			sortJobs(jobs, tt.field)
			for i, wantID := range tt.want {
				if jobs[i].JobID != wantID {
					t.Errorf("position %d: got %s, want %s", i, jobs[i].JobID, wantID)
				}
			}
		})
	}
}

func TestSortJobsReverse(t *testing.T) {
	jobs := []connection.JobStatus{
		{JobID: "JOB00001"},
		{JobID: "JOB00003"},
		{JobID: "JOB00002"},
	}

	sortJobs(jobs, "jobid")

	// Reverse in-place (same logic as runJobs)
	for i, j := 0, len(jobs)-1; i < j; i, j = i+1, j-1 {
		jobs[i], jobs[j] = jobs[j], jobs[i]
	}

	want := []string{"JOB00003", "JOB00002", "JOB00001"}
	for i, wantID := range want {
		if jobs[i].JobID != wantID {
			t.Errorf("position %d: got %s, want %s", i, jobs[i].JobID, wantID)
		}
	}
}
