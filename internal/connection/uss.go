package connection

import (
	"bufio"
	"fmt"
	"net"
	"strconv"
	"strings"
)

type ussClient struct {
	ftpBase
}

func newUSSClient(host string, port int, user, password string) (*ussClient, error) {
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	conn, err := net.DialTimeout("tcp", addr, ftpTimeout)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	c := &ussClient{
		ftpBase: ftpBase{
			conn:   conn,
			reader: bufio.NewReader(conn),
		},
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
