package connection

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/jlaffaye/ftp"
)

const ftpTimeout = 30 * time.Second

type FTPConnection struct {
	host     string
	port     int
	user     string
	password string
	conn     *ftp.ServerConn
}

func NewFTPConnection(host string, port int, user, password string) *FTPConnection {
	return &FTPConnection{
		host:     host,
		port:     port,
		user:     user,
		password: password,
	}
}

func (f *FTPConnection) Connect() error {
	addr := fmt.Sprintf("%s:%d", f.host, f.port)

	conn, err := ftp.Dial(addr, ftp.DialWithTimeout(ftpTimeout))
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", addr, err)
	}

	if err := conn.Login(f.user, f.password); err != nil {
		conn.Quit()
		return fmt.Errorf("login failed: %w", err)
	}

	f.conn = conn
	return nil
}

func (f *FTPConnection) Close() error {
	if f.conn != nil {
		if err := f.conn.Quit(); err != nil {
			return fmt.Errorf("failed to close connection: %w", err)
		}
		f.conn = nil
	}
	return nil
}

func (f *FTPConnection) ListDatasets(pattern string) ([]string, error) {
	if f.conn == nil {
		return nil, fmt.Errorf("not connected")
	}

	// z/OS FTP: list datasets matching pattern (e.g., 'USERNAME.*')
	query := fmt.Sprintf("'%s.*'", pattern)
	entries, err := f.conn.NameList(query)
	if err != nil {
		return nil, fmt.Errorf("failed to list datasets: %w", err)
	}

	var datasets []string
	for _, e := range entries {
		name := strings.TrimSpace(e)
		if name != "" {
			datasets = append(datasets, name)
		}
	}
	return datasets, nil
}

func (f *FTPConnection) ListMembers(dataset string) ([]string, error) {
	if f.conn == nil {
		return nil, fmt.Errorf("not connected")
	}

	// z/OS FTP: cd to PDS and list members
	dsn := strings.Trim(dataset, "'")
	if err := f.conn.ChangeDir(fmt.Sprintf("'%s'", dsn)); err != nil {
		return nil, fmt.Errorf("failed to access dataset %s: %w", dsn, err)
	}

	// Use LIST to get member details, then parse names
	reader, err := f.conn.Retr("*")
	if err != nil {
		// Fallback to NameList
		entries, err := f.conn.NameList("")
		if err != nil {
			return nil, fmt.Errorf("failed to list members: %w", err)
		}
		var members []string
		for _, e := range entries {
			name := strings.TrimSpace(e)
			if name != "" {
				members = append(members, name)
			}
		}
		return members, nil
	}
	defer reader.Close()

	var members []string
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		// z/OS LIST output: first field is member name
		fields := strings.Fields(line)
		if len(fields) > 0 {
			name := fields[0]
			// Skip header line
			if name != "Name" && !strings.HasPrefix(name, "-") {
				members = append(members, name)
			}
		}
	}
	return members, nil
}

func (f *FTPConnection) ReadMember(dataset, member string) ([]byte, error) {
	if f.conn == nil {
		return nil, fmt.Errorf("not connected")
	}

	// z/OS FTP: retrieve 'DATASET(MEMBER)'
	dsn := fmt.Sprintf("'%s(%s)'", strings.Trim(dataset, "'"), member)
	reader, err := f.conn.Retr(dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", dsn, err)
	}
	defer reader.Close()

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, reader); err != nil {
		return nil, fmt.Errorf("failed to read content: %w", err)
	}
	return buf.Bytes(), nil
}

func (f *FTPConnection) WriteMember(dataset, member string, content []byte) error {
	return fmt.Errorf("not implemented")
}

func (f *FTPConnection) ReadFile(path string) ([]byte, error) {
	if f.conn == nil {
		return nil, fmt.Errorf("not connected")
	}

	reader, err := f.conn.Retr(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}
	defer reader.Close()

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, reader); err != nil {
		return nil, fmt.Errorf("failed to read content: %w", err)
	}
	return buf.Bytes(), nil
}

func (f *FTPConnection) WriteFile(path string, content []byte) error {
	return fmt.Errorf("not implemented")
}

func (f *FTPConnection) SubmitJCL(jcl []byte) (string, error) {
	return "", fmt.Errorf("not implemented")
}

func (f *FTPConnection) GetJobStatus(jobid string) (*JobStatus, error) {
	return nil, fmt.Errorf("not implemented")
}

func (f *FTPConnection) GetJobOutput(jobid string) ([]byte, error) {
	return nil, fmt.Errorf("not implemented")
}

var _ Connection = (*FTPConnection)(nil)
