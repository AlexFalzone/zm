package connection

type JobStatus struct {
	JobID   string
	JobName string
	Owner   string
	Status  string // ACTIVE, OUTPUT, INPUT
	RetCode string // CC 0000, ABEND S806, etc.
	Class   string
}

// Connection is implemented by all transport protocols (FTP, SFTP, future z/OSMF)
type Connection interface {
	Connect() error
	Close() error

	// Dataset
	ListDatasets(pattern string) ([]string, error)
	ListMembers(dataset string) ([]string, error)
	ReadMember(dataset, member string) ([]byte, error)
	WriteMember(dataset, member string, content []byte) error

	// USS
	ReadFile(path string) ([]byte, error)
	WriteFile(path string, content []byte) error

	// Jobs
	SubmitJCL(jcl []byte) (string, error) // returns job ID
	GetJobStatus(jobid string) (*JobStatus, error)
	GetJobOutput(jobid string) ([]byte, error)
}
