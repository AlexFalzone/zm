package diff

import (
	"fmt"
	"strings"
)

type OpType int

const (
	OpEqual  OpType = iota
	OpDelete        // line in A only
	OpInsert        // line in B only
)

type DiffLine struct {
	Op   OpType
	Text string
}

type Hunk struct {
	StartA, CountA int
	StartB, CountB int
	Lines          []DiffLine
}

// Diff computes a unified diff between linesA and linesB with the given context lines.
func Diff(linesA, linesB []string, context int) []Hunk {
	// Trim equal prefix
	prefix := 0
	for prefix < len(linesA) && prefix < len(linesB) && linesA[prefix] == linesB[prefix] {
		prefix++
	}
	// Trim equal suffix
	suffix := 0
	for suffix < len(linesA)-prefix && suffix < len(linesB)-prefix &&
		linesA[len(linesA)-1-suffix] == linesB[len(linesB)-1-suffix] {
		suffix++
	}

	midA := linesA[prefix : len(linesA)-suffix]
	midB := linesB[prefix : len(linesB)-suffix]

	midOps := lcs(midA, midB)

	// Reconstruct full ops: prefix(Equal) + midOps + suffix(Equal)
	ops := make([]DiffLine, 0, prefix+len(midOps)+suffix)
	for i := 0; i < prefix; i++ {
		ops = append(ops, DiffLine{Op: OpEqual, Text: linesA[i]})
	}
	ops = append(ops, midOps...)
	for i := len(linesA) - suffix; i < len(linesA); i++ {
		ops = append(ops, DiffLine{Op: OpEqual, Text: linesA[i]})
	}

	return buildHunks(ops, context)
}

// lcs computes the edit script using the LCS dynamic programming approach.
func lcs(a, b []string) []DiffLine {
	m, n := len(a), len(b)

	// Single flat allocation for the DP table — better cache locality and fewer GC objects
	flat := make([]int, (m+1)*(n+1))
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = flat[i*(n+1) : (i+1)*(n+1)]
	}

	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if a[i-1] == b[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else if dp[i-1][j] >= dp[i][j-1] {
				dp[i][j] = dp[i-1][j]
			} else {
				dp[i][j] = dp[i][j-1]
			}
		}
	}

	// Backtrack — pre-allocate ops at LCS upper bound
	ops := make([]DiffLine, 0, m+n)
	i, j := m, n
	for i > 0 || j > 0 {
		if i > 0 && j > 0 && a[i-1] == b[j-1] {
			ops = append(ops, DiffLine{Op: OpEqual, Text: a[i-1]})
			i--
			j--
		} else if j > 0 && (i == 0 || dp[i][j-1] >= dp[i-1][j]) {
			ops = append(ops, DiffLine{Op: OpInsert, Text: b[j-1]})
			j--
		} else {
			ops = append(ops, DiffLine{Op: OpDelete, Text: a[i-1]})
			i--
		}
	}

	// Reverse (we built it backwards)
	for l, r := 0, len(ops)-1; l < r; l, r = l+1, r-1 {
		ops[l], ops[r] = ops[r], ops[l]
	}
	return ops
}

func buildHunks(ops []DiffLine, context int) []Hunk {
	if len(ops) == 0 {
		return nil
	}

	// Find ranges of changes with context
	type changeRange struct{ start, end int }
	var changes []changeRange
	for i, op := range ops {
		if op.Op != OpEqual {
			start := i - context
			if start < 0 {
				start = 0
			}
			end := i + context + 1
			if end > len(ops) {
				end = len(ops)
			}
			changes = append(changes, changeRange{start, end})
		}
	}

	if len(changes) == 0 {
		return nil
	}

	// Merge overlapping ranges
	merged := []changeRange{changes[0]}
	for _, c := range changes[1:] {
		last := &merged[len(merged)-1]
		if c.start <= last.end {
			if c.end > last.end {
				last.end = c.end
			}
		} else {
			merged = append(merged, c)
		}
	}

	// Precompute prefix line positions to avoid re-scanning for each hunk
	posA := make([]int, len(ops)+1)
	posB := make([]int, len(ops)+1)
	for i, op := range ops {
		posA[i+1] = posA[i]
		posB[i+1] = posB[i]
		switch op.Op {
		case OpEqual:
			posA[i+1]++
			posB[i+1]++
		case OpDelete:
			posA[i+1]++
		case OpInsert:
			posB[i+1]++
		}
	}

	// Build hunks from merged ranges
	var hunks []Hunk
	for _, r := range merged {
		h := Hunk{
			StartA: posA[r.start] + 1,
			StartB: posB[r.start] + 1,
			CountA: posA[r.end] - posA[r.start],
			CountB: posB[r.end] - posB[r.start],
			Lines:  make([]DiffLine, r.end-r.start),
		}
		copy(h.Lines, ops[r.start:r.end])
		hunks = append(hunks, h)
	}

	return hunks
}

// FormatUnified formats hunks as unified diff output.
func FormatUnified(nameA, nameB string, hunks []Hunk) string {
	if len(hunks) == 0 {
		return ""
	}

	var b strings.Builder
	fmt.Fprintf(&b, "--- %s\n+++ %s\n", nameA, nameB)

	for _, h := range hunks {
		fmt.Fprintf(&b, "@@ -%d,%d +%d,%d @@\n", h.StartA, h.CountA, h.StartB, h.CountB)
		for _, line := range h.Lines {
			switch line.Op {
			case OpEqual:
				b.WriteByte(' ')
			case OpDelete:
				b.WriteByte('-')
			case OpInsert:
				b.WriteByte('+')
			}
			b.WriteString(line.Text)
			b.WriteByte('\n')
		}
	}

	return b.String()
}
