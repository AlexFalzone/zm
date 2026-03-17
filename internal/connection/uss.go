package connection

import (
	"bufio"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

type ussClient struct {
	conn   net.Conn
	reader *bufio.Reader
}

func newUSSClient(host string, port int, user, password string) (*ussClient, error) {
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	conn, err := net.DialTimeout("tcp", addr, ftpTimeout)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	c := &ussClient{
		conn:   conn,
		reader: bufio.NewReader(conn),
	}

	if _, err := c.readResponse(); err != nil {
		conn.Close()
		return nil, err
	}

	if err := c.cmd("USER %s", user); err != nil {
		conn.Close()
		return nil, err
	}
	if err := c.cmd("PASS %s", password); err != nil {
		conn.Close()
		return nil, err
	}

	// Set EBCDIC→ASCII conversion for USS files
	if err := c.cmd("SITE SBDATACONN=(IBM-1047,ISO8859-1)"); err != nil {
		conn.Close()
		return nil, err
	}

	if err := c.cmd("TYPE A"); err != nil {
		conn.Close()
		return nil, err
	}

	return c, nil
}

func (c *ussClient) close() {
	c.send("QUIT")
	c.conn.Close()
}

func (c *ussClient) listFiles(dirPath string) ([]USSFile, error) {
	if strings.ContainsAny(dirPath, "\r\n") {
		return nil, fmt.Errorf("invalid path: contains control characters")
	}

	if err := c.cmd("CWD %s", dirPath); err != nil {
		return nil, fmt.Errorf("failed to access %s: %w", dirPath, err)
	}

	lines, err := c.retrData("LIST", "-a")
	if err != nil {
		return nil, fmt.Errorf("failed to list %s: %w", dirPath, err)
	}

	files := make([]USSFile, 0, len(lines))
	for _, line := range lines {
		f := parseUSSLine(line)
		if f.Name != "" && f.Name != "." && f.Name != ".." {
			files = append(files, f)
		}
	}
	return files, nil
}

// parseUSSLine parses a Unix-style listing line:
// drwxr-xr-x   2 FALZONE  SYS1        8192 Mar 12 10:20 analyzer
// -rw-r--r--   1 FALZONE  SYS1        1884 Mar 12 10:16 Makefile
func parseUSSLine(line string) USSFile {
	fields := strings.Fields(line)
	if len(fields) < 9 {
		return USSFile{}
	}

	mode := fields[0]
	size, _ := strconv.ParseInt(fields[4], 10, 64)
	mtime := fields[5] + " " + fields[6] + " " + fields[7]
	name := strings.Join(fields[8:], " ")

	fileType := "file"
	if len(mode) > 0 {
		switch mode[0] {
		case 'd':
			fileType = "directory"
		case 'l':
			fileType = "symlink"
			// Symlinks have "name -> target", keep only name
			if idx := strings.Index(name, " -> "); idx != -1 {
				name = name[:idx]
			}
		}
	}

	return USSFile{
		Name:  name,
		Type:  fileType,
		Size:  size,
		Mode:  mode,
		User:  fields[2],
		Group: fields[3],
		Mtime: mtime,
	}
}

func (c *ussClient) readFile(path string) ([]byte, error) {
	if strings.ContainsAny(path, "\r\n") {
		return nil, fmt.Errorf("invalid path: contains control characters")
	}

	lines, err := c.retrData("RETR", path)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}
	return []byte(strings.Join(lines, "\n") + "\n"), nil
}

func (c *ussClient) writeFile(path string, content []byte) error {
	if strings.ContainsAny(path, "\r\n") {
		return fmt.Errorf("invalid path: contains control characters")
	}

	_, err := c.storData(fmt.Sprintf("STOR %s", path), content)
	if err != nil {
		return fmt.Errorf("failed to write %s: %w", path, err)
	}
	return nil
}

func (c *ussClient) retrData(cmd, arg string) ([]string, error) {
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
			return nil, err
		}
	} else {
		if err := c.send("%s", cmd); err != nil {
			return nil, err
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

	c.readResponse()
	return lines, nil
}

func (c *ussClient) storData(cmd string, data []byte) ([]string, error) {
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

func (c *ussClient) cmd(format string, args ...interface{}) error {
	_, err := c.cmdResp(format, args...)
	return err
}

func (c *ussClient) cmdResp(format string, args ...interface{}) (string, error) {
	if err := c.send(format, args...); err != nil {
		return "", err
	}
	return c.readResponse()
}

func (c *ussClient) send(format string, args ...interface{}) error {
	cmd := fmt.Sprintf(format, args...)
	c.conn.SetWriteDeadline(time.Now().Add(ftpTimeout))
	_, err := fmt.Fprintf(c.conn, "%s\r\n", cmd)
	return err
}

func (c *ussClient) readResponse() (string, error) {
	c.conn.SetReadDeadline(time.Now().Add(ftpTimeout))
	var resp strings.Builder
	for {
		line, err := c.reader.ReadString('\n')
		if err != nil {
			return "", err
		}
		resp.WriteString(line)
		if len(line) >= 4 && line[3] == ' ' {
			break
		}
	}
	result := strings.TrimSpace(resp.String())
	if len(result) > 0 && (result[0] == '4' || result[0] == '5') {
		return result, fmt.Errorf("ftp error: %s", result)
	}
	return result, nil
}
