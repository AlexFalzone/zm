package connection

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"

	"zm/internal/retry"
	"zm/internal/validate"

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
		if err := f.conn.NoOp(); err == nil {
			return nil
		}
		f.conn.Quit()
		f.conn = nil
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
	f.debugBuf.Reset()
	if err := conn.Type(ftp.TransferTypeASCII); err != nil {
		conn.Quit()
		return fmt.Errorf("failed to set ASCII mode: %w", err)
	}
	f.conn = conn
	return nil
}

func (f *FTPConnection) Close() error {
	var errs []error
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
			errs = append(errs, fmt.Errorf("failed to close main connection: %w", err))
		}
		f.conn = nil
	}
	return errors.Join(errs...)
}

func (f *FTPConnection) getJES() (*jesClient, error) {
	if f.jes != nil {
		if err := f.jes.noop(); err == nil {
			return f.jes, nil
		}
		f.jes.close()
		f.jes = nil
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
		if err := f.uss.noop(); err == nil {
			return f.uss, nil
		}
		f.uss.close()
		f.uss = nil
	}
	uss, err := newUSSClient(f.host, f.port, f.user, f.password)
	if err != nil {
		return nil, err
	}
	f.uss = uss
	return uss, nil
}

func (f *FTPConnection) ListDatasets(pattern string) ([]string, error) {
	if err := validate.DSN(pattern); err != nil {
		return nil, err
	}
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
	dsn := strings.Trim(dataset, "'")
	if err := validate.DSN(dsn); err != nil {
		return nil, err
	}
	if err := f.ensureMainConn(); err != nil {
		return nil, err
	}

	// Reset debug buffer and capture only this LIST operation
	f.debugBuf.Reset()

	// cd to PDS
	if err := f.conn.ChangeDir(fmt.Sprintf("'%s'", dsn)); err != nil {
		return nil, fmt.Errorf("failed to access dataset %s: %w", dsn, err)
	}

	// Call List — may fail to parse structured response, but debug output has raw data
	if _, err := f.conn.List(""); err != nil {
		// List often returns a parse error on z/OS; only fail if debug buffer is also empty
		if f.debugBuf.Len() == 0 {
			return nil, fmt.Errorf("failed to list members of %s: %w", dsn, err)
		}
	}

	return parseMemberListFromDebug(f.debugBuf.String())
}

func (f *FTPConnection) ReadMember(dataset, member string) ([]byte, error) {
	if err := validate.DSN(strings.Trim(dataset, "'")); err != nil {
		return nil, err
	}
	if err := validate.DSN(member); err != nil {
		return nil, err
	}
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
	if err := validate.DSN(strings.Trim(dataset, "'")); err != nil {
		return err
	}
	if err := validate.DSN(member); err != nil {
		return err
	}
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
		if err := validate.USSPath(dirPath); err != nil {
			return nil, err
		}
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

	files, err := parseUSSListFromDebug(f.debugBuf.String())
	if err == nil && len(files) > 0 {
		return files, nil
	}

	f.debugBuf.Reset()
	f.conn.List("-a")

	files, err = parseUSSListFromDebug(f.debugBuf.String())
	if err != nil {
		return nil, fmt.Errorf("failed to list %s: %w", dirPath, err)
	}
	return files, nil
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
