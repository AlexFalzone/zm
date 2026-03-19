package connection

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"

	"zm/internal/retry"

	"github.com/jlaffaye/ftp"
)

const ftpTimeout = 30 * time.Second

type FTPConnection struct {
	host          string
	port          int
	user          string
	password      string
	retryAttempts int
	retryDelay    time.Duration
	conn          *ftp.ServerConn
	debugBuf      bytes.Buffer
	uss           *ussClient
	jes           *jesClient
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
	return nil // lazy — connection opened on first dataset operation
}

func (f *FTPConnection) ensureMainConn() error {
	if f.conn != nil {
		return nil
	}
	return retry.Do(retry.Config{
		Attempts: f.retryAttempts,
		Delay:    f.retryDelay,
	}, func() error {
		return f.dialMainConn()
	})
}

func (f *FTPConnection) dialMainConn() error {
	addr := net.JoinHostPort(f.host, strconv.Itoa(f.port))
	conn, err := ftp.Dial(addr, ftp.DialWithTimeout(ftpTimeout), ftp.DialWithDebugOutput(&f.debugBuf))
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", addr, err)
	}
	if err := conn.Login(f.user, f.password); err != nil {
		conn.Quit()
		return fmt.Errorf("login failed: %w", err)
	}
	if err := conn.Type(ftp.TransferTypeASCII); err != nil {
		conn.Quit()
		return fmt.Errorf("failed to set ASCII mode: %w", err)
	}
	f.conn = conn
	return nil
}

func (f *FTPConnection) Close() error {
	if f.jes != nil {
		f.jes.close()
		f.jes = nil
	}
	if f.uss != nil {
		f.uss.close()
		f.uss = nil
	}
	if f.conn != nil {
		if err := f.conn.Quit(); err != nil {
			return fmt.Errorf("failed to close connection: %w", err)
		}
		f.conn = nil
	}
	return nil
}

func (f *FTPConnection) getJES() (*jesClient, error) {
	if f.jes != nil {
		return f.jes, nil
	}
	jes, err := newJESClient(f.host, f.port, f.user, f.password)
	if err != nil {
		return nil, err
	}
	if err := jes.setOwner(f.user); err != nil {
		jes.close()
		return nil, err
	}
	f.jes = jes
	return jes, nil
}

func (f *FTPConnection) getUSS() (*ussClient, error) {
	if f.uss != nil {
		return f.uss, nil
	}
	uss, err := newUSSClient(f.host, f.port, f.user, f.password)
	if err != nil {
		return nil, err
	}
	f.uss = uss
	return uss, nil
}

func (f *FTPConnection) ListDatasets(pattern string) ([]string, error) {
	if err := f.ensureMainConn(); err != nil {
		return nil, err
	}

	// z/OS FTP: list datasets matching pattern (e.g., 'USERNAME.*')
	query := fmt.Sprintf("'%s'", strings.Trim(pattern, "'"))
	entries, err := f.conn.NameList(query)
	if err != nil {
		return nil, fmt.Errorf("failed to list datasets: %w", err)
	}

	datasets := make([]string, 0, len(entries))
	for _, e := range entries {
		name := strings.TrimSpace(e)
		if name != "" {
			datasets = append(datasets, name)
		}
	}
	return datasets, nil
}

func (f *FTPConnection) ListMembers(dataset string) ([]Member, error) {
	if err := f.ensureMainConn(); err != nil {
		return nil, err
	}

	dsn := strings.Trim(dataset, "'")

	// Reset debug buffer and capture only this LIST operation
	f.debugBuf.Reset()

	// cd to PDS
	if err := f.conn.ChangeDir(fmt.Sprintf("'%s'", dsn)); err != nil {
		return nil, fmt.Errorf("failed to access dataset %s: %w", dsn, err)
	}

	// Call List - it will fail to parse but debug output will have the raw data
	f.conn.List("")

	// Parse the debug output to extract member info
	return parseMemberListFromDebug(f.debugBuf.String())
}

func (f *FTPConnection) ReadMember(dataset, member string) ([]byte, error) {
	if err := f.ensureMainConn(); err != nil {
		return nil, err
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
	if err := f.ensureMainConn(); err != nil {
		return err
	}

	dsn := fmt.Sprintf("'%s(%s)'", strings.Trim(dataset, "'"), member)
	if err := f.conn.Stor(dsn, bytes.NewReader(content)); err != nil {
		return fmt.Errorf("failed to write %s: %w", dsn, err)
	}
	return nil
}

func (f *FTPConnection) ListFiles(dirPath string) ([]USSFile, error) {
	if strings.HasPrefix(dirPath, "/") {
		uss, err := f.getUSS()
		if err != nil {
			return nil, err
		}
		return uss.listFiles(dirPath)
	}

	// Dataset listing via main conn
	if err := f.ensureMainConn(); err != nil {
		return nil, err
	}

	if err := f.conn.ChangeDir(dirPath); err != nil {
		return nil, fmt.Errorf("failed to access %s: %w", dirPath, err)
	}

	f.debugBuf.Reset()
	f.conn.List("")

	files, _ := parseUSSListFromDebug(f.debugBuf.String())
	if len(files) > 0 {
		return files, nil
	}

	f.debugBuf.Reset()
	f.conn.List("-a")

	return parseUSSListFromDebug(f.debugBuf.String())
}

func (f *FTPConnection) ReadFile(path string) ([]byte, error) {
	uss, err := f.getUSS()
	if err != nil {
		return nil, err
	}
	return uss.readFile(path)
}

func (f *FTPConnection) WriteFile(path string, content []byte) error {
	uss, err := f.getUSS()
	if err != nil {
		return err
	}
	return uss.writeFile(path, content)
}

func (f *FTPConnection) SubmitJCL(jcl []byte) (string, error) {
	jes, err := f.getJES()
	if err != nil {
		return "", err
	}
	return jes.submitJCL(jcl)
}

func (f *FTPConnection) GetJobStatus(jobid string) (*JobStatus, error) {
	jes, err := f.getJES()
	if err != nil {
		return nil, err
	}
	return jes.getJobStatus(jobid)
}

func (f *FTPConnection) ListJobs(owner string) ([]JobStatus, error) {
	jes, err := f.getJES()
	if err != nil {
		return nil, err
	}
	return jes.listJobs()
}

func (f *FTPConnection) GetJobOutput(jobid string) ([]byte, error) {
	jes, err := f.getJES()
	if err != nil {
		return nil, err
	}
	return jes.getJobOutput(jobid)
}

func (f *FTPConnection) CancelJob(jobid string) error {
	return fmt.Errorf("cancel job is not supported via FTP")
}

func (f *FTPConnection) PurgeJob(jobid string) error {
	jes, err := f.getJES()
	if err != nil {
		return err
	}
	return jes.purgeJob(jobid)
}

var _ Connection = (*FTPConnection)(nil)
