package connection

import (
	"bufio"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

type ftpBase struct {
	conn   net.Conn
	reader *bufio.Reader
}

func (c *ftpBase) close() {
	c.send("QUIT")
	c.conn.Close()
}

func (c *ftpBase) noop() error {
	return c.cmd("NOOP")
}

func (c *ftpBase) cmd(format string, args ...interface{}) error {
	_, err := c.cmdResp(format, args...)
	return err
}

func (c *ftpBase) cmdResp(format string, args ...interface{}) (string, error) {
	if err := c.send(format, args...); err != nil {
		return "", err
	}
	return c.readResponse()
}

func (c *ftpBase) send(format string, args ...interface{}) error {
	cmd := fmt.Sprintf(format, args...)
	c.conn.SetWriteDeadline(time.Now().Add(ftpTimeout))
	_, err := fmt.Fprintf(c.conn, "%s\r\n", cmd)
	return err
}

func (c *ftpBase) readResponse() (string, error) {
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

func (c *ftpBase) retrData(cmd, arg string) ([]string, error) {
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
		if err := c.send("%s", cmd); err != nil {
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

	// Read end response — tolerate errors if we already have data
	endResp, endErr := c.readResponse()
	if endErr != nil && len(lines) == 0 {
		return nil, fmt.Errorf("no output available: %s", endResp)
	}

	return lines, nil
}

func (c *ftpBase) storData(cmd string, data []byte) ([]string, error) {
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

	if err := c.send("%s", cmd); err != nil {
		dataConn.Close()
		return nil, err
	}

	resp, err := c.readResponse()
	if err != nil {
		dataConn.Close()
		return nil, err
	}
	if !strings.HasPrefix(resp, "125") && !strings.HasPrefix(resp, "150") {
		dataConn.Close()
		return nil, fmt.Errorf("STOR failed: %s", resp)
	}

	dataConn.SetWriteDeadline(time.Now().Add(ftpTimeout))
	_, err = dataConn.Write(data)
	dataConn.Close()
	if err != nil {
		return nil, fmt.Errorf("failed to send data: %w", err)
	}

	// Read completion response(s)
	var responses []string
	for {
		endResp, endErr := c.readResponse()
		responses = append(responses, endResp)
		if endErr != nil || strings.HasPrefix(endResp, "250") {
			break
		}
	}

	return responses, nil
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
