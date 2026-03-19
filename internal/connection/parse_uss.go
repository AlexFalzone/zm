package connection

import (
	"strconv"
	"strings"
)

// parseUSSLine parses a Unix-style listing line:
// drwxr-xr-x   2 FALZONE  SYS1        8192 Mar 12 10:20 analyzer
// -rw-r--r--   1 FALZONE  SYS1        1884 Mar 12 10:16 Makefile
func parseUSSLine(line string) USSFile {
	fields := strings.Fields(line)
	if len(fields) < 9 {
		return USSFile{}
	}

	mode := fields[0]
	size, _ := strconv.ParseInt(fields[4], 10, 64)
	mtime := fields[5] + " " + fields[6] + " " + fields[7]
	name := strings.Join(fields[8:], " ")

	fileType := "file"
	if len(mode) > 0 {
		switch mode[0] {
		case 'd':
			fileType = "directory"
		case 'l':
			fileType = "symlink"
			// Symlinks have "name -> target", keep only name
			if idx := strings.Index(name, " -> "); idx != -1 {
				name = name[:idx]
			}
		}
	}

	return USSFile{
		Name:  name,
		Type:  fileType,
		Size:  size,
		Mode:  mode,
		User:  fields[2],
		Group: fields[3],
		Mtime: mtime,
	}
}
