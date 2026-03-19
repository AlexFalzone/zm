package connection

import "strings"

func parseJobLine(line string) JobStatus {
	// Format: JOBNAME  JOBID    OWNER    STATUS CLASS
	// Example: MYJOB    JOB12345 FALZONE  OUTPUT A    RC=0000
	fields := strings.Fields(line)
	if len(fields) < 4 {
		return JobStatus{}
	}

	job := JobStatus{
		JobName: fields[0],
		JobID:   fields[1],
		Owner:   fields[2],
		Status:  fields[3],
	}

	if len(fields) >= 5 {
		job.Class = fields[4]
	}

	// Look for return code
	for _, f := range fields {
		if strings.HasPrefix(f, "RC=") {
			job.RetCode = "CC " + strings.TrimPrefix(f, "RC=")
		} else if strings.HasPrefix(f, "ABEND=") {
			job.RetCode = "ABEND " + strings.TrimPrefix(f, "ABEND=")
		}
	}

	return job
}

func parseJobLines(lines []string) []JobStatus {
	jobs := make([]JobStatus, 0, len(lines))
	for _, line := range lines {
		if strings.Contains(line, "JOBNAME") && strings.Contains(line, "JOBID") {
			continue
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		job := parseJobLine(line)
		if job.JobID != "" {
			jobs = append(jobs, job)
		}
	}
	return jobs
}

// parseStatusOutput parses SSH tsocmd "STATUS" output.
// Format: "JOB JOBNAME(JOBID) STATUS..."
func parseStatusOutput(output, owner string) []JobStatus {
	lines := strings.Split(output, "\n")
	var jobs []JobStatus
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "JOB ") {
			continue
		}

		rest := strings.TrimPrefix(line, "JOB ")
		parenOpen := strings.Index(rest, "(")
		parenClose := strings.Index(rest, ")")
		if parenOpen == -1 || parenClose == -1 || parenClose <= parenOpen {
			continue
		}

		jobName := strings.TrimSpace(rest[:parenOpen])
		jobID := rest[parenOpen+1 : parenClose]
		statusPart := strings.TrimSpace(rest[parenClose+1:])

		status := "UNKNOWN"
		if strings.Contains(strings.ToUpper(statusPart), "OUTPUT") {
			status = "OUTPUT"
		} else if strings.Contains(strings.ToUpper(statusPart), "EXECUTING") || strings.Contains(strings.ToUpper(statusPart), "ACTIVE") {
			status = "ACTIVE"
		} else if strings.Contains(strings.ToUpper(statusPart), "INPUT") {
			status = "INPUT"
		}

		jobs = append(jobs, JobStatus{
			JobID:   jobID,
			JobName: jobName,
			Owner:   owner,
			Status:  status,
		})
	}
	return jobs
}

func isJobID(s string) bool {
	if !strings.HasPrefix(s, "JOB") || len(s) < 4 {
		return false
	}
	for _, c := range s[3:] {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
