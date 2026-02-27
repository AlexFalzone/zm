package connection

import "fmt"

func NewConnection(host string, port int, user, password, protocol string) (Connection, error) {
	switch protocol {
	case "zosmf":
		return NewZOSMFConnection(host, port, user, password), nil
	case "ftp":
		return NewFTPConnection(host, port, user, password), nil
	default:
		return nil, fmt.Errorf("unsupported protocol: %s", protocol)
	}
}
