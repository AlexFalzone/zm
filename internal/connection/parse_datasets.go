package connection

import "strings"

// parseListDatasetsOutput parses SSH tsocmd "LISTDS" or "LISTCAT" output.
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
