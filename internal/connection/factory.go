package connection

import "fmt"

func NewConnection(host string, port int, user, password, protocol, keyPath string) (Connection, error) {
	switch protocol {
	case "zosmf":
		return NewZOSMFConnection(host, port, user, password), nil
	case "ftp":
		return NewFTPConnection(host, port, user, password), nil
	case "ssh":
		return NewSSHConnection(host, port, user, password, keyPath), nil
	default:
		return nil, fmt.Errorf("unsupported protocol: %s", protocol)
	}
}
