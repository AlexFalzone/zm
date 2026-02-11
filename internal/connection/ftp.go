package connection

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strconv"
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

func (f *FTPConnection) ListMembers(dataset string) ([]Member, error) {
	if f.conn == nil {
		return nil, fmt.Errorf("not connected")
	}

	dsn := strings.Trim(dataset, "'")

	// Create a new connection with debug output to capture LIST response
	addr := fmt.Sprintf("%s:%d", f.host, f.port)
	var debugBuf bytes.Buffer
	conn, err := ftp.Dial(addr, ftp.DialWithTimeout(ftpTimeout), ftp.DialWithDebugOutput(&debugBuf))
	if err != nil {
		return nil, fmt.Errorf("failed to connect for LIST: %w", err)
	}
	defer conn.Quit()

	if err := conn.Login(f.user, f.password); err != nil {
		return nil, fmt.Errorf("login failed for LIST: %w", err)
	}

	// cd to PDS
	if err := conn.ChangeDir(fmt.Sprintf("'%s'", dsn)); err != nil {
		return nil, fmt.Errorf("failed to access dataset %s: %w", dsn, err)
	}

	// Call List - it will fail to parse but debug output will have the raw data
	conn.List("")

	// Parse the debug output to extract member info
	return f.parseMemberListFromDebug(debugBuf.String())
}

func (f *FTPConnection) parseMemberListFromDebug(debug string) ([]Member, error) {
	var members []Member
	lines := strings.Split(debug, "\n")

	inList := false
	for _, line := range lines {
		// Look for lines after "125 List started" until "250 List completed"
		if strings.Contains(line, "125 List started") {
			inList = true
			continue
		}
		if strings.Contains(line, "250 List completed") {
			break
		}
		if !inList {
			continue
		}

		// Skip header line
		if strings.Contains(line, "Name") && strings.Contains(line, "VV.MM") {
			continue
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		member := parseMemberLine(line)
		if member.Name != "" {
			members = append(members, member)
		}
	}

	return members, nil
}

func (f *FTPConnection) parseMemberList(r io.Reader) ([]Member, error) {
	var members []Member
	scanner := bufio.NewScanner(r)

	for scanner.Scan() {
		line := scanner.Text()

		// Skip header line
		if strings.HasPrefix(line, " Name") || strings.HasPrefix(line, "Name") {
			continue
		}

		// Skip empty lines
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		member := parseMemberLine(line)
		if member.Name != "" {
			members = append(members, member)
		}
	}

	return members, scanner.Err()
}

func parseMemberLine(line string) Member {
	// Format: Name     VV.MM   Created       Changed      Size  Init   Mod   Id
	// Example: HSISAPIE  01.82 2024/04/16 2025/12/10 20:18     5    27     0 FALZONE
	fields := strings.Fields(line)
	if len(fields) < 8 {
		return Member{}
	}

	m := Member{Name: fields[0]}

	// Parse VV.MM
	if vvmm := strings.Split(fields[1], "."); len(vvmm) == 2 {
		m.VV, _ = strconv.Atoi(vvmm[0])
		m.MM, _ = strconv.Atoi(vvmm[1])
	}

	// Created date
	m.Created = fields[2]

	// Changed date and time
	if len(fields) >= 5 {
		m.Changed = fields[3] + " " + fields[4]
	}

	// Size, Init, Mod, User
	if len(fields) >= 6 {
		m.Size, _ = strconv.Atoi(fields[5])
	}
	if len(fields) >= 7 {
		m.Init, _ = strconv.Atoi(fields[6])
	}
	if len(fields) >= 8 {
		m.Mod, _ = strconv.Atoi(fields[7])
	}
	if len(fields) >= 9 {
		m.User = fields[8]
	}

	return m
}

func (f *FTPConnection) ReadMember(dataset, member string) ([]byte, error) {
	if f.conn == nil {
		return nil, fmt.Errorf("not connected")
	}

	// Set ASCII mode for EBCDIC to ASCII conversion
	if err := f.conn.Type(ftp.TransferTypeASCII); err != nil {
		return nil, fmt.Errorf("failed to set ASCII mode: %w", err)
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

	// Set ASCII mode for EBCDIC to ASCII conversion
	if err := f.conn.Type(ftp.TransferTypeASCII); err != nil {
		return nil, fmt.Errorf("failed to set ASCII mode: %w", err)
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
	jobs, err := f.ListJobs("")
	if err != nil {
		return nil, err
	}

	for _, job := range jobs {
		if job.JobID == jobid {
			return &job, nil
		}
	}

	return nil, fmt.Errorf("job %s not found", jobid)
}

func (f *FTPConnection) ListJobs(owner string) ([]JobStatus, error) {
	jes, err := newJESClient(f.host, f.port, f.user, f.password)
	if err != nil {
		return nil, err
	}
	defer jes.close()

	if owner == "" {
		owner = f.user
	}
	if err := jes.setOwner(owner); err != nil {
		return nil, err
	}

	return jes.listJobs()
}

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

func (f *FTPConnection) GetJobOutput(jobid string) ([]byte, error) {
	jes, err := newJESClient(f.host, f.port, f.user, f.password)
	if err != nil {
		return nil, err
	}
	defer jes.close()

	if err := jes.setOwner(f.user); err != nil {
		return nil, err
	}

	return jes.getJobOutput(jobid)
}

var _ Connection = (*FTPConnection)(nil)
