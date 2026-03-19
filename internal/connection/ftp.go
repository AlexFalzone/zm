package connection

import (
	"bytes"
	"fmt"
	"io"
	"net"
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
	debugBuf bytes.Buffer
	uss      *ussClient
	jes      *jesClient
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
	addr := net.JoinHostPort(f.host, strconv.Itoa(f.port))

	conn, err := ftp.Dial(addr, ftp.DialWithTimeout(ftpTimeout), ftp.DialWithDebugOutput(&f.debugBuf))
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
	if f.conn == nil {
		return nil, fmt.Errorf("not connected")
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
	if f.conn == nil {
		return nil, fmt.Errorf("not connected")
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
	return f.parseMemberListFromDebug(f.debugBuf.String())
}

func (f *FTPConnection) parseMemberListFromDebug(debug string) ([]Member, error) {
	// Find list boundaries to avoid scanning the entire debug buffer
	startMarker := "125 List started"
	endMarker := "250 List completed"

	startIdx := strings.Index(debug, startMarker)
	if startIdx == -1 {
		return nil, nil
	}
	startIdx += len(startMarker)

	endIdx := strings.Index(debug[startIdx:], endMarker)
	if endIdx == -1 {
		endIdx = len(debug) - startIdx
	}
	listData := debug[startIdx : startIdx+endIdx]

	lines := strings.Split(listData, "\n")
	members := make([]Member, 0, len(lines))

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || (strings.Contains(line, "Name") && strings.Contains(line, "VV.MM")) {
			continue
		}

		member := parseMemberLine(line)
		if member.Name != "" {
			members = append(members, member)
		}
	}

	return members, nil
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
	if f.conn == nil {
		return fmt.Errorf("not connected")
	}

	if err := f.conn.Type(ftp.TransferTypeASCII); err != nil {
		return fmt.Errorf("failed to set ASCII mode: %w", err)
	}

	dsn := fmt.Sprintf("'%s(%s)'", strings.Trim(dataset, "'"), member)
	if err := f.conn.Stor(dsn, bytes.NewReader(content)); err != nil {
		return fmt.Errorf("failed to write %s: %w", dsn, err)
	}
	return nil
}

func (f *FTPConnection) ListFiles(dirPath string) ([]USSFile, error) {
	if f.conn == nil {
		return nil, fmt.Errorf("not connected")
	}

	if err := f.conn.ChangeDir(dirPath); err != nil {
		return nil, fmt.Errorf("failed to access %s: %w", dirPath, err)
	}

	// Try LIST first (full metadata format)
	f.debugBuf.Reset()
	f.conn.List("")

	files, _ := parseUSSListFromDebug(f.debugBuf.String())
	if len(files) > 0 {
		return files, nil
	}

	// If empty, retry with -a to catch hidden files (name-only format)
	f.debugBuf.Reset()
	f.conn.List("-a")

	return parseUSSListFromDebug(f.debugBuf.String())
}

func parseUSSListFromDebug(debug string) ([]USSFile, error) {
	// Find list data between start/end markers
	startIdx := -1
	for _, marker := range []string{"150 Opening", "125 List started"} {
		if idx := strings.Index(debug, marker); idx != -1 {
			startIdx = idx + len(marker)
			break
		}
	}
	if startIdx == -1 {
		return nil, nil
	}

	endIdx := len(debug)
	for _, marker := range []string{"250 List completed", "226 Transfer"} {
		if idx := strings.Index(debug[startIdx:], marker); idx != -1 {
			if startIdx+idx < endIdx {
				endIdx = startIdx + idx
			}
			break
		}
	}
	listData := debug[startIdx:endIdx]

	lines := strings.Split(listData, "\n")
	files := make([]USSFile, 0, len(lines))

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "total ") {
			continue
		}

		f := parseUSSLine(line)
		if f.Name == "" {
			if line == "." || line == ".." {
				continue
			}
			f = USSFile{Name: line, Type: "file"}
		}
		if f.Name != "." && f.Name != ".." {
			files = append(files, f)
		}
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
