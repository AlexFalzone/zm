package connection

import (
	"fmt"
	"time"
)

type ConnectionOptions struct {
	Host          string
	Port          int
	User          string
	Password      string
	Protocol      string
	KeyPath       string
	Encoding      string
	TLSVerify     bool
	CACertPath    string
	RetryAttempts int
	RetryDelay    time.Duration
}

func NewConnection(opts ConnectionOptions) (Connection, error) {
	switch opts.Protocol {
	case "zosmf":
		z := NewZOSMFConnection(opts.Host, opts.Port, opts.User, opts.Password, opts.Encoding)
		z.tlsVerify = opts.TLSVerify
		z.caCertPath = opts.CACertPath
		z.retryAttempts = opts.RetryAttempts
		z.retryDelay = opts.RetryDelay
		return z, nil
	case "ftp":
		f := NewFTPConnection(opts.Host, opts.Port, opts.User, opts.Password)
		f.retryAttempts = opts.RetryAttempts
		f.retryDelay = opts.RetryDelay
		return f, nil
	case "ssh":
		s := NewSSHConnection(opts.Host, opts.Port, opts.User, opts.Password, opts.KeyPath, opts.Encoding)
		s.retryAttempts = opts.RetryAttempts
		s.retryDelay = opts.RetryDelay
		return s, nil
	default:
		return nil, fmt.Errorf("unsupported protocol: %s", opts.Protocol)
	}
}
