package connection

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

const sshTimeout = 30 * time.Second

type SSHConnection struct {
	host     string
	port     int
	user     string
	password string
	keyPath  string
	client   *ssh.Client
	sftp     *sftp.Client
}

func NewSSHConnection(host string, port int, user, password, keyPath string) *SSHConnection {
	return &SSHConnection{
		host:     host,
		port:     port,
		user:     user,
		password: password,
		keyPath:  keyPath,
	}
}

func (s *SSHConnection) Connect() error {
	hostKeyCallback, err := hostKeyCallback()
	if err != nil {
		return fmt.Errorf("failed to setup host key verification: %w", err)
	}

	config := &ssh.ClientConfig{
		User:            s.user,
		HostKeyCallback: hostKeyCallback,
		Timeout:         sshTimeout,
		Config: ssh.Config{
			// Prefer AES-GCM (hardware-accelerated via AES-NI) over CTR modes
			Ciphers: []string{
				"aes128-gcm@openssh.com",
				"aes256-gcm@openssh.com",
				"aes128-ctr",
				"aes192-ctr",
				"aes256-ctr",
			},
			// Prefer fast key exchange
			KeyExchanges: []string{
				"curve25519-sha256",
				"curve25519-sha256@libssh.org",
				"ecdh-sha2-nistp256",
				"ecdh-sha2-nistp384",
			},
			// Prefer ETM MACs (encrypt-then-MAC, faster and more secure)
			MACs: []string{
				"hmac-sha2-256-etm@openssh.com",
				"hmac-sha2-256",
			},
		},
	}

	var authMethods []ssh.AuthMethod
	if s.keyPath != "" {
		keyPath := s.keyPath
		if strings.HasPrefix(keyPath, "~/") {
			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("failed to resolve home directory: %w", err)
			}
			keyPath = home + keyPath[1:]
		}
		keyData, err := os.ReadFile(keyPath)
		if err != nil {
			return fmt.Errorf("failed to read SSH key %s: %w", s.keyPath, err)
		}
		signer, err := ssh.ParsePrivateKey(keyData)
		if err != nil {
			return fmt.Errorf("failed to parse SSH key %s: %w", s.keyPath, err)
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	}
	if s.password != "" {
		authMethods = append(authMethods, ssh.Password(s.password))
	}
	config.Auth = authMethods

	addr := net.JoinHostPort(s.host, strconv.Itoa(s.port))
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", addr, err)
	}
	s.client = client

	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		client.Close()
		return fmt.Errorf("failed to open SFTP session: %w", err)
	}
	s.sftp = sftpClient

	return nil
}

func (s *SSHConnection) Close() error {
	var firstErr error
	if s.sftp != nil {
		if err := s.sftp.Close(); err != nil {
			firstErr = err
		}
		s.sftp = nil
	}
	if s.client != nil {
		if err := s.client.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		s.client = nil
	}
	return firstErr
}

func (s *SSHConnection) exec(cmd string) (string, error) {
	session, err := s.client.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer session.Close()

	out, err := session.CombinedOutput(cmd)
	if err != nil {
		return string(out), fmt.Errorf("command failed: %w: %s", err, string(out))
	}
	return string(out), nil
}

func (s *SSHConnection) execWithStdin(cmd string, stdin []byte) (string, error) {
	session, err := s.client.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer session.Close()

	session.Stdin = bytes.NewReader(stdin)
	out, err := session.CombinedOutput(cmd)
	if err != nil {
		return string(out), fmt.Errorf("command failed: %w: %s", err, string(out))
	}
	return string(out), nil
}

// hostKeyCallback returns an ssh.HostKeyCallback that uses ~/.ssh/known_hosts.
// Trust On First Use: if the host is unknown, its key is appended to known_hosts.
// If the host is known but the key changed, the connection is rejected.
func hostKeyCallback() (ssh.HostKeyCallback, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("cannot find home directory: %w", err)
	}

	khPath := filepath.Join(home, ".ssh", "known_hosts")

	// Ensure ~/.ssh directory exists
	if err := os.MkdirAll(filepath.Dir(khPath), 0700); err != nil {
		return nil, fmt.Errorf("cannot create ~/.ssh: %w", err)
	}

	// Create known_hosts if it doesn't exist
	if _, err := os.Stat(khPath); os.IsNotExist(err) {
		f, err := os.OpenFile(khPath, os.O_CREATE, 0600)
		if err != nil {
			return nil, fmt.Errorf("cannot create known_hosts: %w", err)
		}
		f.Close()
	}

	kh, err := knownhosts.New(khPath)
	if err != nil {
		return nil, fmt.Errorf("cannot read known_hosts: %w", err)
	}

	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		err := kh(hostname, remote, key)
		if err == nil {
			return nil // host known, key matches
		}

		// If the key doesn't match a known host, reject (possible MITM)
		var keyErr *knownhosts.KeyError
		if errors.As(err, &keyErr) && len(keyErr.Want) > 0 {
			return fmt.Errorf("host key mismatch for %s (possible MITM attack): %w", hostname, err)
		}

		// Host unknown — TOFU: append to known_hosts
		f, appendErr := os.OpenFile(khPath, os.O_APPEND|os.O_WRONLY, 0600)
		if appendErr != nil {
			return fmt.Errorf("host unknown and cannot update known_hosts: %w", appendErr)
		}
		defer f.Close()

		line := knownhosts.Line([]string{knownhosts.Normalize(hostname)}, key)
		if _, appendErr := fmt.Fprintln(f, line); appendErr != nil {
			return fmt.Errorf("host unknown and cannot write to known_hosts: %w", appendErr)
		}

		fmt.Fprintf(os.Stderr, "Warning: permanently added '%s' to known hosts\n", hostname)
		return nil
	}, nil
}

// --- Input validation ---

func validateDSN(name string) error {
	for _, c := range name {
		switch {
		case c >= 'A' && c <= 'Z':
		case c >= 'a' && c <= 'z':
		case c >= '0' && c <= '9':
		case c == '.' || c == '@' || c == '#' || c == '$' || c == '(' || c == ')' || c == '*':
		default:
			return fmt.Errorf("invalid character %q in dataset name", c)
		}
	}
	return nil
}

func validateUSSPath(path string) error {
	for _, bad := range []string{";", "$(", "`", "|", "&", ">", "<"} {
		if strings.Contains(path, bad) {
			return fmt.Errorf("invalid character sequence %q in USS path", bad)
		}
	}
	return nil
}

// --- USS operations (via SFTP) ---

func (s *SSHConnection) ListFiles(path string) ([]USSFile, error) {
	if err := validateUSSPath(path); err != nil {
		return nil, err
	}

	entries, err := s.sftp.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("failed to list %s: %w", path, err)
	}

	files := make([]USSFile, 0, len(entries))
	for _, e := range entries {
		if e.Name() == "." || e.Name() == ".." {
			continue
		}

		fileType := "file"
		if e.IsDir() {
			fileType = "directory"
		} else if e.Mode()&os.ModeSymlink != 0 {
			fileType = "symlink"
		}

		files = append(files, USSFile{
			Name:  e.Name(),
			Type:  fileType,
			Size:  e.Size(),
			Mode:  e.Mode().String(),
			Mtime: e.ModTime().Format("Jan 02 15:04"),
		})
	}
	return files, nil
}

func (s *SSHConnection) ReadFile(path string) ([]byte, error) {
	if err := validateUSSPath(path); err != nil {
		return nil, err
	}

	f, err := s.sftp.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open %s: %w", path, err)
	}
	defer f.Close()

	return io.ReadAll(f)
}

func (s *SSHConnection) WriteFile(path string, content []byte) error {
	if err := validateUSSPath(path); err != nil {
		return err
	}

	f, err := s.sftp.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create %s: %w", path, err)
	}
	defer f.Close()

	_, err = f.Write(content)
	if err != nil {
		return fmt.Errorf("failed to write %s: %w", path, err)
	}
	return nil
}

// --- Dataset operations (via SSH exec) ---

func (s *SSHConnection) ListDatasets(pattern string) ([]string, error) {
	if err := validateDSN(pattern); err != nil {
		return nil, err
	}

	out, err := s.exec(fmt.Sprintf(`tsocmd "LISTDS '%s'" 2>/dev/null`, pattern))
	if err != nil {
		// Fallback to LISTCAT
		out, err = s.exec(fmt.Sprintf(`tsocmd "LISTCAT ENTRIES('%s') NAME" 2>/dev/null`, pattern))
		if err != nil {
			return nil, fmt.Errorf("failed to list datasets: %w", err)
		}
	}

	return parseListDatasetsOutput(out), nil
}

func parseListDatasetsOutput(output string) []string {
	lines := strings.Split(output, "\n")
	var datasets []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Skip tsocmd header/status lines
		if strings.HasPrefix(line, "READY") || strings.HasPrefix(line, "END") ||
			strings.Contains(line, "LISTDS") || strings.Contains(line, "LISTCAT") ||
			strings.Contains(line, "---") || strings.Contains(line, "NONVSAM") ||
			strings.Contains(line, "IN-CAT") || strings.Contains(line, "THE FOLLOWING") {
			continue
		}
		// Dataset names are uppercase alphanumeric with dots
		if isDatasetName(line) {
			datasets = append(datasets, line)
		}
	}
	return datasets
}

func isDatasetName(s string) bool {
	if len(s) == 0 || len(s) > 44 {
		return false
	}
	for _, c := range s {
		switch {
		case c >= 'A' && c <= 'Z':
		case c >= '0' && c <= '9':
		case c == '.' || c == '@' || c == '#' || c == '$':
		default:
			return false
		}
	}
	return true
}

func (s *SSHConnection) ListMembers(dataset string) ([]Member, error) {
	dsn := strings.Trim(dataset, "'")
	if err := validateDSN(dsn); err != nil {
		return nil, err
	}

	out, err := s.exec(fmt.Sprintf(`tsocmd "LISTDS '%s' MEMBERS" 2>/dev/null`, dsn))
	if err != nil {
		return nil, fmt.Errorf("failed to list members of %s: %w", dsn, err)
	}

	return parseListMembersOutput(out), nil
}

func parseListMembersOutput(output string) []Member {
	lines := strings.Split(output, "\n")
	var members []Member
	pastMembers := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.Contains(line, "--MEMBERS--") {
			pastMembers = true
			continue
		}
		if !pastMembers {
			continue
		}
		if strings.HasPrefix(line, "READY") || strings.HasPrefix(line, "END") {
			continue
		}
		if line != "" {
			members = append(members, Member{Name: line})
		}
	}
	return members
}

func (s *SSHConnection) ReadMember(dataset, member string) ([]byte, error) {
	dsn := strings.Trim(dataset, "'")
	if err := validateDSN(dsn); err != nil {
		return nil, err
	}
	if err := validateDSN(member); err != nil {
		return nil, err
	}

	out, err := s.exec(fmt.Sprintf(`cat "//'%s(%s)'"`, dsn, member))
	if err != nil {
		return nil, fmt.Errorf("failed to read %s(%s): %w", dsn, member, err)
	}
	return []byte(out), nil
}

func (s *SSHConnection) WriteMember(dataset, member string, content []byte) error {
	dsn := strings.Trim(dataset, "'")
	if err := validateDSN(dsn); err != nil {
		return err
	}
	if err := validateDSN(member); err != nil {
		return err
	}

	tmpPath := fmt.Sprintf("/tmp/zm_write_%d", time.Now().UnixNano())
	if err := s.WriteFile(tmpPath, content); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	_, err := s.exec(fmt.Sprintf(`cp %s "//'%s(%s)'" && rm %s`, tmpPath, dsn, member, tmpPath))
	if err != nil {
		s.exec(fmt.Sprintf("rm -f %s", tmpPath))
		return fmt.Errorf("failed to write %s(%s): %w", dsn, member, err)
	}
	return nil
}

// --- JES operations (via SSH exec) ---

func (s *SSHConnection) SubmitJCL(jcl []byte) (string, error) {
	tmpPath := fmt.Sprintf("/tmp/zm_submit_%d.jcl", time.Now().UnixNano())
	if err := s.WriteFile(tmpPath, jcl); err != nil {
		return "", fmt.Errorf("failed to write JCL temp file: %w", err)
	}

	out, err := s.exec(fmt.Sprintf("submit %s", tmpPath))
	s.exec(fmt.Sprintf("rm -f %s", tmpPath))
	if err != nil {
		return "", fmt.Errorf("failed to submit JCL: %w", err)
	}

	jobID, err := parseSubmitOutput(out)
	if err != nil {
		return "", err
	}
	return jobID, nil
}

func parseSubmitOutput(output string) (string, error) {
	// Expected format: "JOB JOB12345 submitted" or similar
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if idx := strings.Index(strings.ToUpper(line), "JOB"); idx != -1 {
			rest := line[idx:]
			fields := strings.Fields(rest)
			for _, f := range fields {
				f = strings.ToUpper(f)
				if strings.HasPrefix(f, "JOB") && len(f) > 3 {
					// Check if it looks like a job ID (JOBnnnnn)
					if isJobID(f) {
						return f, nil
					}
				}
			}
		}
	}
	return "", fmt.Errorf("could not parse job ID from submit output: %s", output)
}

func isJobID(s string) bool {
	if !strings.HasPrefix(s, "JOB") || len(s) < 4 {
		return false
	}
	for _, c := range s[3:] {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func (s *SSHConnection) ListJobs(owner string) ([]JobStatus, error) {
	if owner == "" {
		owner = s.user
	}

	out, err := s.exec(`tsocmd "STATUS" 2>/dev/null`)
	if err != nil {
		return nil, fmt.Errorf("failed to list jobs: %w", err)
	}

	return parseStatusOutput(out, strings.ToUpper(owner)), nil
}

func parseStatusOutput(output, owner string) []JobStatus {
	// Expected format: "JOB MYJOB(JOB12345) ON OUTPUT QUEUE"
	// or: "JOB MYJOB(JOB12345) EXECUTING"
	lines := strings.Split(output, "\n")
	var jobs []JobStatus
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "JOB ") {
			continue
		}

		// Parse "JOB JOBNAME(JOBID) STATUS..."
		rest := strings.TrimPrefix(line, "JOB ")
		parenOpen := strings.Index(rest, "(")
		parenClose := strings.Index(rest, ")")
		if parenOpen == -1 || parenClose == -1 || parenClose <= parenOpen {
			continue
		}

		jobName := strings.TrimSpace(rest[:parenOpen])
		jobID := rest[parenOpen+1 : parenClose]
		statusPart := strings.TrimSpace(rest[parenClose+1:])

		status := "UNKNOWN"
		if strings.Contains(strings.ToUpper(statusPart), "OUTPUT") {
			status = "OUTPUT"
		} else if strings.Contains(strings.ToUpper(statusPart), "EXECUTING") || strings.Contains(strings.ToUpper(statusPart), "ACTIVE") {
			status = "ACTIVE"
		} else if strings.Contains(strings.ToUpper(statusPart), "INPUT") {
			status = "INPUT"
		}

		jobs = append(jobs, JobStatus{
			JobID:   jobID,
			JobName: jobName,
			Owner:   owner,
			Status:  status,
		})
	}
	return jobs
}

func (s *SSHConnection) GetJobStatus(jobid string) (*JobStatus, error) {
	jobs, err := s.ListJobs("")
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

func (s *SSHConnection) GetJobOutput(jobid string) ([]byte, error) {
	if err := validateDSN(jobid); err != nil {
		return nil, err
	}

	out, err := s.exec(fmt.Sprintf(`tsocmd "OUTPUT %s PRINT" 2>/dev/null`, jobid))
	if err != nil {
		return nil, fmt.Errorf("failed to get job output for %s: %w", jobid, err)
	}
	return []byte(out), nil
}

func (s *SSHConnection) CancelJob(jobid string) error {
	if err := validateDSN(jobid); err != nil {
		return err
	}

	_, err := s.exec(fmt.Sprintf(`tsocmd "CANCEL %s" 2>/dev/null`, jobid))
	if err != nil {
		return fmt.Errorf("failed to cancel job %s: %w", jobid, err)
	}
	return nil
}

func (s *SSHConnection) PurgeJob(jobid string) error {
	if err := validateDSN(jobid); err != nil {
		return err
	}

	_, err := s.exec(fmt.Sprintf(`tsocmd "CANCEL %s PURGE" 2>/dev/null`, jobid))
	if err != nil {
		return fmt.Errorf("failed to purge job %s: %w", jobid, err)
	}
	return nil
}

var _ Connection = (*SSHConnection)(nil)
