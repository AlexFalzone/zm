package connection

import (
	"strconv"
	"strings"
)

func parseMemberLine(line string) Member {
	// Format: Name     VV.MM   Created       Changed      Size  Init   Mod   Id
	// Example: HSISAPIE  01.82 2024/04/16 2025/12/10 20:18     5    27     0 FALZONE
	fields := strings.Fields(line)
	if len(fields) < 8 {
		return Member{}
	}

	m := Member{Name: fields[0]}

	// Parse VV.MM
	if vvmm := strings.Split(fields[1], "."); len(vvmm) == 2 {
		m.VV, _ = strconv.Atoi(vvmm[0])
		m.MM, _ = strconv.Atoi(vvmm[1])
	}

	// Created date
	m.Created = fields[2]

	// Changed date and time
	if len(fields) >= 5 {
		m.Changed = fields[3] + " " + fields[4]
	}

	// Size, Init, Mod, User
	if len(fields) >= 6 {
		m.Size, _ = strconv.Atoi(fields[5])
	}
	if len(fields) >= 7 {
		m.Init, _ = strconv.Atoi(fields[6])
	}
	if len(fields) >= 8 {
		m.Mod, _ = strconv.Atoi(fields[7])
	}
	if len(fields) >= 9 {
		m.User = fields[8]
	}

	return m
}

// parseListMembersOutput parses SSH tsocmd "LISTDS MEMBERS" output.
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
