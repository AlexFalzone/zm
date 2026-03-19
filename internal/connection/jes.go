package connection

import (
	"bufio"
	"fmt"
	"net"
	"strconv"
	"strings"
)

type jesClient struct {
	ftpBase
}

func newJESClient(host string, port int, user, password string) (*jesClient, error) {
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	conn, err := net.DialTimeout("tcp", addr, ftpTimeout)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	c := &jesClient{
		ftpBase: ftpBase{
			conn:   conn,
			reader: bufio.NewReader(conn),
		},
	}

	// Read welcome
	if _, err := c.readResponse(); err != nil {
		conn.Close()
		return nil, err
	}

	// Login
	if err := c.cmd("USER %s", user); err != nil {
		conn.Close()
		return nil, err
	}
	if err := c.cmd("PASS %s", password); err != nil {
		conn.Close()
		return nil, err
	}

	// Enter JES mode
	if err := c.cmd("SITE FILETYPE=JES"); err != nil {
		conn.Close()
		return nil, err
	}

	return c, nil
}

func (c *jesClient) setOwner(owner string) error {
	if strings.ContainsAny(owner, "\r\n") {
		return fmt.Errorf("invalid owner: contains control characters")
	}
	if err := c.cmd("SITE JESOWNER=%s", owner); err != nil {
		return err
	}
	return c.cmd("SITE JESJOBNAME=*")
}

func (c *jesClient) listJobs() ([]JobStatus, error) {
	lines, err := c.retrData("LIST", "")
	if err != nil {
		return nil, err
	}
	return parseJobLines(lines), nil
}

func (c *jesClient) getJobStatus(jobid string) (*JobStatus, error) {
	if strings.ContainsAny(jobid, "\r\n") {
		return nil, fmt.Errorf("invalid jobid: contains control characters")
	}
	// LIST with jobid arg filters server-side — avoids fetching all jobs
	lines, err := c.retrData("LIST", jobid)
	if err != nil {
		return nil, fmt.Errorf("job %s not found: %w", jobid, err)
	}
	jobs := parseJobLines(lines)
	if len(jobs) == 0 {
		return nil, fmt.Errorf("job %s not found", jobid)
	}
	return &jobs[0], nil
}

func (c *jesClient) submitJCL(jcl []byte) (string, error) {
	if err := c.cmd("TYPE A"); err != nil {
		return "", fmt.Errorf("failed to set ASCII mode: %w", err)
	}

	lines, err := c.storData("STOR SUBMIT", jcl)
	if err != nil {
		return "", fmt.Errorf("failed to submit JCL: %w", err)
	}

	// z/OS FTP responds with job ID in the 250 response, e.g.:
	// "250 It is known to JES as JOB12345"
	for _, line := range lines {
		if idx := strings.Index(line, "JOB"); idx != -1 {
			// Extract JOBxxxxx
			field := line[idx:]
			if sp := strings.IndexByte(field, ' '); sp != -1 {
				field = field[:sp]
			}
			return strings.TrimSpace(field), nil
		}
	}

	return "", fmt.Errorf("could not parse job ID from submit response")
}

func (c *jesClient) purgeJob(jobid string) error {
	if strings.ContainsAny(jobid, "\r\n") {
		return fmt.Errorf("invalid jobid: contains control characters")
	}
	return c.cmd("DELE %s", jobid)
}

func (c *jesClient) getJobOutput(jobid string) ([]byte, error) {
	if strings.ContainsAny(jobid, "\r\n") {
		return nil, fmt.Errorf("invalid jobid: contains control characters")
	}
	if err := c.cmd("TYPE A"); err != nil {
		return nil, fmt.Errorf("failed to set ASCII mode: %w", err)
	}
	lines, err := c.retrData("RETR", jobid)
	if err != nil {
		return nil, err
	}
	return []byte(strings.Join(lines, "\n")), nil
}
