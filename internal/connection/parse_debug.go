package connection

import (
	"fmt"
	"strings"
)

// isDebugControlLine returns true for FTP debug output noise lines
// (server responses "< ...", client commands "> ...", or their remnants).
func isDebugControlLine(line string) bool {
	return line == "<" || line == ">" ||
		strings.HasPrefix(line, "< ") || strings.HasPrefix(line, "> ")
}

// parseMemberListFromDebug extracts member info from FTP debug output.
// The debug buffer contains the raw LIST response between start/end markers.
func parseMemberListFromDebug(debug string) ([]Member, error) {
	startIdx := -1
	for _, marker := range []string{"125 List started", "150 Opening"} {
		if idx := strings.Index(debug, marker); idx != -1 {
			startIdx = idx + len(marker)
			break
		}
	}
	if startIdx == -1 {
		return nil, fmt.Errorf("no list start marker found in debug output")
	}

	endIdx := len(debug) - startIdx
	for _, marker := range []string{"250 List completed", "226 Transfer"} {
		if idx := strings.Index(debug[startIdx:], marker); idx != -1 {
			endIdx = idx
			break
		}
	}

	if startIdx > startIdx+endIdx {
		return nil, fmt.Errorf("invalid debug output: start marker after end marker")
	}
	listData := debug[startIdx : startIdx+endIdx]

	lines := strings.Split(listData, "\n")
	members := make([]Member, 0, len(lines))

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || isDebugControlLine(line) {
			continue
		}
		if strings.Contains(line, "Name") && strings.Contains(line, "VV.MM") {
			continue
		}

		member := parseMemberLine(line)
		if member.Name != "" {
			members = append(members, member)
		}
	}

	return members, nil
}

// parseUSSListFromDebug extracts USS file listing from FTP debug output.
func parseUSSListFromDebug(debug string) ([]USSFile, error) {
	startIdx := -1
	for _, marker := range []string{"150 Opening", "125 List started"} {
		if idx := strings.Index(debug, marker); idx != -1 {
			startIdx = idx + len(marker)
			break
		}
	}
	if startIdx == -1 {
		return nil, fmt.Errorf("no list start marker found in debug output")
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
		if line == "" || strings.HasPrefix(line, "total ") || isDebugControlLine(line) {
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
