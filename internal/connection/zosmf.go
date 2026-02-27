package connection

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type ZOSMFConnection struct {
	host      string
	port      int
	user      string
	password  string
	client    *http.Client
	transport *http.Transport
	baseURL   string
}

func NewZOSMFConnection(host string, port int, user, password string) *ZOSMFConnection {
	return &ZOSMFConnection{
		host:     host,
		port:     port,
		user:     user,
		password: password,
	}
}

func (z *ZOSMFConnection) Connect() error {
	z.baseURL = fmt.Sprintf("https://%s:%d", z.host, z.port)
	z.transport = &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
		MaxIdleConns:        10,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
		DialContext: (&net.Dialer{
			Timeout: 10 * time.Second,
		}).DialContext,
	}
	z.client = &http.Client{
		Timeout:   30 * time.Second,
		Transport: z.transport,
	}
	return nil
}

func (z *ZOSMFConnection) Close() error {
	if z.transport != nil {
		z.transport.CloseIdleConnections()
	}
	z.client = nil
	z.transport = nil
	return nil
}

func (z *ZOSMFConnection) doRequest(method, path string, body io.Reader, extraHeaders ...string) (*http.Response, error) {
	req, err := http.NewRequest(method, z.baseURL+path, body)
	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(z.user, z.password)
	req.Header.Set("X-CSRF-ZOSMF-HEADER", "*")

	for i := 0; i+1 < len(extraHeaders); i += 2 {
		req.Header.Set(extraHeaders[i], extraHeaders[i+1])
	}

	return z.client.Do(req)
}

func zosmfError(action string, resp *http.Response) error {
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))

	var errResp struct {
		Message string `json:"message"`
		Details []struct {
			Message string `json:"messageText"`
		} `json:"details"`
	}
	if json.Unmarshal(body, &errResp) == nil && errResp.Message != "" {
		msg := errResp.Message
		if len(errResp.Details) > 0 && errResp.Details[0].Message != "" {
			msg += ": " + errResp.Details[0].Message
		}
		return fmt.Errorf("%s: %s (status %d)", action, msg, resp.StatusCode)
	}

	if len(body) > 0 {
		return fmt.Errorf("%s: %s (status %d)", action, string(body), resp.StatusCode)
	}
	return fmt.Errorf("%s (status %d)", action, resp.StatusCode)
}

// --- Dataset operations ---

type dsListResponse struct {
	Items []struct {
		Dsname string `json:"dsname"`
	} `json:"items"`
}

func (z *ZOSMFConnection) ListDatasets(pattern string) ([]string, error) {
	path := "/zosmf/restfiles/ds?dslevel=" + url.QueryEscape(pattern)
	resp, err := z.doRequest("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list datasets: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, zosmfError("failed to list datasets", resp)
	}
	defer resp.Body.Close()

	var result dsListResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse dataset list: %w", err)
	}

	datasets := make([]string, 0, len(result.Items))
	for _, item := range result.Items {
		datasets = append(datasets, item.Dsname)
	}
	return datasets, nil
}

type memberListResponse struct {
	Items []struct {
		Member string `json:"member"`
		Vers   int    `json:"vers"`
		Mod    int    `json:"mod"`
		C4date string `json:"c4date"`
		M4date string `json:"m4date"`
		Mtime  string `json:"mtime"`
		Cnorc  int    `json:"cnorc"`
		Inorc  int    `json:"inorc"`
		Mnorc  int    `json:"mnorc"`
		User   string `json:"user"`
	} `json:"items"`
}

func (z *ZOSMFConnection) ListMembers(dataset string) ([]Member, error) {
	dsn := strings.Trim(dataset, "'")
	path := fmt.Sprintf("/zosmf/restfiles/ds/%s/member", dsn)
	resp, err := z.doRequest("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list members: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, zosmfError(fmt.Sprintf("failed to list members of %s", dsn), resp)
	}
	defer resp.Body.Close()

	var result memberListResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse member list: %w", err)
	}

	members := make([]Member, 0, len(result.Items))
	for _, item := range result.Items {
		m := Member{
			Name:    item.Member,
			VV:      item.Vers,
			MM:      item.Mod,
			Created: item.C4date,
			Size:    item.Cnorc,
			Init:    item.Inorc,
			Mod:     item.Mnorc,
			User:    item.User,
		}
		if item.Mtime != "" {
			m.Changed = item.M4date + " " + item.Mtime
		} else {
			m.Changed = item.M4date
		}
		members = append(members, m)
	}
	return members, nil
}

func (z *ZOSMFConnection) ReadMember(dataset, member string) ([]byte, error) {
	dsn := strings.Trim(dataset, "'")
	path := fmt.Sprintf("/zosmf/restfiles/ds/%s(%s)", dsn, member)
	resp, err := z.doRequest("GET", path, nil, "X-IBM-Data-Type", "text")
	if err != nil {
		return nil, fmt.Errorf("failed to read %s(%s): %w", dsn, member, err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, zosmfError(fmt.Sprintf("failed to read %s(%s)", dsn, member), resp)
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}

func (z *ZOSMFConnection) WriteMember(dataset, member string, content []byte) error {
	dsn := strings.Trim(dataset, "'")
	path := fmt.Sprintf("/zosmf/restfiles/ds/%s(%s)", dsn, member)
	resp, err := z.doRequest("PUT", path, bytes.NewReader(content),
		"X-IBM-Data-Type", "text", "Content-Type", "text/plain")
	if err != nil {
		return fmt.Errorf("failed to write %s(%s): %w", dsn, member, err)
	}
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusCreated {
		return zosmfError(fmt.Sprintf("failed to write %s(%s)", dsn, member), resp)
	}
	resp.Body.Close()

	return nil
}

// --- USS operations ---

func (z *ZOSMFConnection) ReadFile(path string) ([]byte, error) {
	ussPath := "/zosmf/restfiles/fs" + path
	resp, err := z.doRequest("GET", ussPath, nil, "X-IBM-Data-Type", "text")
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, zosmfError(fmt.Sprintf("failed to read %s", path), resp)
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}

func (z *ZOSMFConnection) WriteFile(path string, content []byte) error {
	ussPath := "/zosmf/restfiles/fs" + path
	resp, err := z.doRequest("PUT", ussPath, bytes.NewReader(content),
		"X-IBM-Data-Type", "text", "Content-Type", "text/plain")
	if err != nil {
		return fmt.Errorf("failed to write %s: %w", path, err)
	}
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusCreated {
		return zosmfError(fmt.Sprintf("failed to write %s", path), resp)
	}
	resp.Body.Close()

	return nil
}

// --- Job operations ---

func (z *ZOSMFConnection) SubmitJCL(jcl []byte) (string, error) {
	resp, err := z.doRequest("PUT", "/zosmf/restjobs/jobs", bytes.NewReader(jcl),
		"Content-Type", "text/plain")
	if err != nil {
		return "", fmt.Errorf("failed to submit JCL: %w", err)
	}
	if resp.StatusCode != http.StatusCreated {
		return "", zosmfError("failed to submit JCL", resp)
	}
	defer resp.Body.Close()

	var result struct {
		JobID   string `json:"jobid"`
		JobName string `json:"jobname"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to parse submit response: %w", err)
	}

	return result.JobID, nil
}

type jobsListResponse struct {
	JobID   string `json:"jobid"`
	JobName string `json:"jobname"`
	Owner   string `json:"owner"`
	Status  string `json:"status"`
	RetCode string `json:"retcode"`
	Class   string `json:"class"`
}

func (z *ZOSMFConnection) ListJobs(owner string) ([]JobStatus, error) {
	if owner == "" {
		owner = z.user
	}
	path := "/zosmf/restjobs/jobs?owner=" + url.QueryEscape(owner) + "&prefix=*"
	resp, err := z.doRequest("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list jobs: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, zosmfError("failed to list jobs", resp)
	}
	defer resp.Body.Close()

	var items []jobsListResponse
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		return nil, fmt.Errorf("failed to parse jobs list: %w", err)
	}

	return parseZOSMFJobs(items), nil
}

func parseZOSMFJobs(items []jobsListResponse) []JobStatus {
	jobs := make([]JobStatus, 0, len(items))
	for _, item := range items {
		jobs = append(jobs, JobStatus{
			JobID:   item.JobID,
			JobName: item.JobName,
			Owner:   item.Owner,
			Status:  item.Status,
			RetCode: item.RetCode,
			Class:   item.Class,
		})
	}
	return jobs
}

func (z *ZOSMFConnection) GetJobStatus(jobid string) (*JobStatus, error) {
	path := "/zosmf/restjobs/jobs?owner=*&jobid=" + url.QueryEscape(jobid)
	resp, err := z.doRequest("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get job status: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, zosmfError("failed to get job status", resp)
	}
	defer resp.Body.Close()

	var items []jobsListResponse
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		return nil, fmt.Errorf("failed to parse job status: %w", err)
	}

	if len(items) == 0 {
		return nil, fmt.Errorf("job %s not found", jobid)
	}

	job := &JobStatus{
		JobID:   items[0].JobID,
		JobName: items[0].JobName,
		Owner:   items[0].Owner,
		Status:  items[0].Status,
		RetCode: items[0].RetCode,
		Class:   items[0].Class,
	}
	return job, nil
}

type jobFileResponse struct {
	ID       int    `json:"id"`
	DDName   string `json:"ddname"`
	StepName string `json:"stepname"`
}

func (z *ZOSMFConnection) GetJobOutput(jobid string) ([]byte, error) {
	status, err := z.GetJobStatus(jobid)
	if err != nil {
		return nil, err
	}

	filesPath := fmt.Sprintf("/zosmf/restjobs/jobs/%s/%s/files", status.JobName, jobid)
	resp, err := z.doRequest("GET", filesPath, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list job files: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, zosmfError("failed to list job files", resp)
	}
	defer resp.Body.Close()

	var files []jobFileResponse
	if err := json.NewDecoder(resp.Body).Decode(&files); err != nil {
		return nil, fmt.Errorf("failed to parse job files: %w", err)
	}

	if len(files) == 0 {
		return nil, nil
	}

	// Fetch all spool files concurrently
	type spoolResult struct {
		index int
		dd    string
		step  string
		data  []byte
		err   error
	}

	results := make([]spoolResult, len(files))
	var wg sync.WaitGroup

	for i, file := range files {
		wg.Add(1)
		go func(idx int, f jobFileResponse) {
			defer wg.Done()
			recordsPath := fmt.Sprintf("/zosmf/restjobs/jobs/%s/%s/files/%d/records",
				status.JobName, jobid, f.ID)
			recResp, err := z.doRequest("GET", recordsPath, nil, "X-IBM-Data-Type", "text")
			if err != nil {
				results[idx] = spoolResult{index: idx, err: fmt.Errorf("failed to read DD %s: %w", f.DDName, err)}
				return
			}
			body, err := io.ReadAll(recResp.Body)
			recResp.Body.Close()
			if err != nil {
				results[idx] = spoolResult{index: idx, err: fmt.Errorf("failed to read DD %s: %w", f.DDName, err)}
				return
			}
			results[idx] = spoolResult{index: idx, dd: f.DDName, step: f.StepName, data: body}
		}(i, file)
	}
	wg.Wait()

	var output strings.Builder
	for _, r := range results {
		if r.err != nil {
			return nil, r.err
		}
		if output.Len() > 0 {
			output.WriteString("\n")
		}
		fmt.Fprintf(&output, "--- DD: %s (Step: %s) ---\n", r.dd, r.step)
		output.Write(r.data)
	}

	return []byte(output.String()), nil
}

var _ Connection = (*ZOSMFConnection)(nil)
