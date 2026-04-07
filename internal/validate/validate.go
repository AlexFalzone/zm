package validate

import (
	"fmt"
	"strings"
)

// DSN validates a z/OS dataset name or pattern.
// Allowed: A-Z, a-z, 0-9, . @ # $ ( ) *
func DSN(name string) error {
	if name == "" {
		return fmt.Errorf("dataset name cannot be empty")
	}
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

// USSPath validates a USS file path against shell injection patterns.
func USSPath(path string) error {
	if path == "" {
		return fmt.Errorf("USS path cannot be empty")
	}
	for _, bad := range []string{";", "$(", "`", "|", "&", ">", "<", "\r", "\n"} {
		if strings.Contains(path, bad) {
			return fmt.Errorf("invalid character sequence %q in USS path", bad)
		}
	}
	return nil
}

// JobID validates a z/OS job identifier (JOBnnnnn).
func JobID(id string) error {
	if len(id) < 4 {
		return fmt.Errorf("invalid job ID %q: too short", id)
	}
	if !strings.HasPrefix(strings.ToUpper(id), "JOB") {
		return fmt.Errorf("invalid job ID %q: must start with JOB", id)
	}
	for _, c := range id[3:] {
		if c < '0' || c > '9' {
			return fmt.Errorf("invalid job ID %q: non-digit after JOB prefix", id)
		}
	}
	return nil
}

// Owner validates a z/OS RACF user ID / job owner.
// Allowed: A-Z, a-z, 0-9, @ # $ *
func Owner(owner string) error {
	if owner == "" {
		return fmt.Errorf("owner cannot be empty")
	}
	for _, c := range owner {
		switch {
		case c >= 'A' && c <= 'Z':
		case c >= 'a' && c <= 'z':
		case c >= '0' && c <= '9':
		case c == '@' || c == '#' || c == '$' || c == '*':
		default:
			return fmt.Errorf("invalid character %q in owner", c)
		}
	}
	return nil
}
