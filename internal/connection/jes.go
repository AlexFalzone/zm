package connection

import (
	"bufio"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

type jesClient struct {
	conn   net.Conn
	reader *bufio.Reader
}

func newJESClient(host string, port int, user, password string) (*jesClient, error) {
	addr := fmt.Sprintf("%s:%d", host, port)
	conn, err := net.DialTimeout("tcp", addr, ftpTimeout)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	c := &jesClient{
		conn:   conn,
		reader: bufio.NewReader(conn),
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

func (c *jesClient) close() {
	c.send("QUIT")
	c.conn.Close()
}

func (c *jesClient) setOwner(owner string) error {
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

func (c *jesClient) getJobOutput(jobid string) ([]byte, error) {
	if err := c.cmd("TYPE A"); err != nil {
		return nil, fmt.Errorf("failed to set ASCII mode: %w", err)
	}
	lines, err := c.retrData("RETR", jobid)
	if err != nil {
		return nil, err
	}
	return []byte(strings.Join(lines, "\n")), nil
}

func (c *jesClient) retrData(cmd, arg string) ([]string, error) {
	pasvResp, err := c.cmdResp("PASV")
	if err != nil {
		return nil, err
	}

	dataAddr, err := parsePASV(pasvResp)
	if err != nil {
		return nil, err
	}

	dataConn, err := net.DialTimeout("tcp", dataAddr, ftpTimeout)
	if err != nil {
		return nil, fmt.Errorf("failed to connect data channel: %w", err)
	}
	defer dataConn.Close()

	if arg != "" {
		if err := c.send("%s %s", cmd, arg); err != nil {
			return nil, fmt.Errorf("failed to send %s: %w", cmd, err)
		}
	} else {
		if err := c.send(cmd); err != nil {
			return nil, fmt.Errorf("failed to send %s: %w", cmd, err)
		}
	}

	resp, err := c.readResponse()
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(resp, "125") && !strings.HasPrefix(resp, "150") {
		return nil, fmt.Errorf("%s failed: %s", cmd, resp)
	}

	dataConn.SetReadDeadline(time.Now().Add(ftpTimeout * 2))
	lines := make([]string, 0, 256)
	scanner := bufio.NewScanner(dataConn)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read data: %w", err)
	}

	endResp, endErr := c.readResponse()
	if endErr != nil && len(lines) == 0 {
		return nil, fmt.Errorf("no output available: %s", endResp)
	}

	return lines, nil
}

func (c *jesClient) cmd(format string, args ...interface{}) error {
	_, err := c.cmdResp(format, args...)
	return err
}

func (c *jesClient) cmdResp(format string, args ...interface{}) (string, error) {
	if err := c.send(format, args...); err != nil {
		return "", err
	}
	return c.readResponse()
}

func (c *jesClient) send(format string, args ...interface{}) error {
	cmd := fmt.Sprintf(format, args...)
	c.conn.SetWriteDeadline(time.Now().Add(ftpTimeout))
	_, err := fmt.Fprintf(c.conn, "%s\r\n", cmd)
	return err
}

func (c *jesClient) readResponse() (string, error) {
	c.conn.SetReadDeadline(time.Now().Add(ftpTimeout))
	var resp strings.Builder
	for {
		line, err := c.reader.ReadString('\n')
		if err != nil {
			return "", err
		}
		resp.WriteString(line)
		// Single line response or last line of multi-line
		if len(line) >= 4 && line[3] == ' ' {
			break
		}
	}
	result := strings.TrimSpace(resp.String())
	// Check for error response (4xx, 5xx)
	if len(result) > 0 && (result[0] == '4' || result[0] == '5') {
		return result, fmt.Errorf("ftp error: %s", result)
	}
	return result, nil
}

func parsePASV(resp string) (string, error) {
	// Parse: 227 Entering Passive Mode (h1,h2,h3,h4,p1,p2)
	start := strings.Index(resp, "(")
	end := strings.Index(resp, ")")
	if start == -1 || end == -1 {
		return "", fmt.Errorf("invalid PASV response: %s", resp)
	}

	parts := strings.Split(resp[start+1:end], ",")
	if len(parts) != 6 {
		return "", fmt.Errorf("invalid PASV response: %s", resp)
	}

	host := strings.Join(parts[:4], ".")
	p1, err := strconv.Atoi(strings.TrimSpace(parts[4]))
	if err != nil {
		return "", fmt.Errorf("invalid PASV port: %s", resp)
	}
	p2, err := strconv.Atoi(strings.TrimSpace(parts[5]))
	if err != nil {
		return "", fmt.Errorf("invalid PASV port: %s", resp)
	}
	port := p1*256 + p2

	return fmt.Sprintf("%s:%d", host, port), nil
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
