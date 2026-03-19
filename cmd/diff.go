package cmd

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"zm/internal/connection"
	"zm/internal/diff"

	"github.com/spf13/cobra"
)

var (
	diffNoColor bool
	diffContext int
)

var diffCmd = &cobra.Command{
	Use:   "diff SOURCE1 SOURCE2",
	Short: "Compare two sources (PDS members, USS files, or local files)",
	Long: `Compare two sources and show differences in unified diff format.
Each source can be a PDS member (DATASET(MEMBER)), USS file (/path), or local file (./path).`,
	Args: cobra.ExactArgs(2),
	RunE: runDiff,
}

func init() {
	rootCmd.AddCommand(diffCmd)
	diffCmd.Flags().BoolVar(&diffNoColor, "no-color", false, "disable color output")
	diffCmd.Flags().IntVarP(&diffContext, "context", "C", 3, "lines of context")
}

func runDiff(cmd *cobra.Command, args []string) error {
	source1 := args[0]
	source2 := args[1]

	needsConn := !isLocalFile(source1) || !isLocalFile(source2)

	var conn connection.Connection
	if needsConn {
		var err error
		_, conn, err = openConnection()
		if err != nil {
			return err
		}
		defer conn.Close()
	}

	var contentA, contentB string
	var errA, errB error

	localA := isLocalFile(source1)
	localB := isLocalFile(source2)
	_, isConcurrent := conn.(connection.ConcurrentConnection)
	canParallel := (localA || localB) || (isConcurrent && !localA && !localB)

	if canParallel {
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			contentA, errA = resolveSource(conn, source1)
		}()
		go func() {
			defer wg.Done()
			contentB, errB = resolveSource(conn, source2)
		}()
		wg.Wait()
	} else {
		contentA, errA = resolveSource(conn, source1)
		if errA == nil {
			contentB, errB = resolveSource(conn, source2)
		}
	}

	if errA != nil {
		return fmt.Errorf("source1: %w", errA)
	}
	if errB != nil {
		return fmt.Errorf("source2: %w", errB)
	}

	linesA := splitLines(contentA)
	linesB := splitLines(contentB)

	hunks := diff.Diff(linesA, linesB, diffContext)
	if len(hunks) == 0 {
		fmt.Println("Files are identical")
		return nil
	}

	output := diff.FormatUnified(source1, source2, hunks)

	if diffNoColor {
		fmt.Print(output)
	} else {
		printColorDiff(output)
	}
	return nil
}

func isLocalFile(source string) bool {
	if strings.HasPrefix(source, "/") {
		// Could be USS — check if it exists locally
		_, err := os.Stat(source)
		return err == nil
	}
	if strings.HasPrefix(source, "./") || strings.HasPrefix(source, "../") {
		return true
	}
	// If it contains parentheses, it's a DSN
	if strings.Contains(source, "(") {
		return false
	}
	// Check if it exists as a local file
	_, err := os.Stat(source)
	return err == nil
}

func resolveSource(conn connection.Connection, source string) (string, error) {
	// Local file: starts with ./ or ../ or exists on disk
	if strings.HasPrefix(source, "./") || strings.HasPrefix(source, "../") {
		data, err := os.ReadFile(source)
		if err != nil {
			return "", err
		}
		return string(data), nil
	}

	// Check local file existence first (except for paths starting with / which could be USS)
	if !strings.HasPrefix(source, "/") && !strings.Contains(source, "(") {
		if _, err := os.Stat(source); err == nil {
			data, err := os.ReadFile(source)
			if err != nil {
				return "", err
			}
			return string(data), nil
		}
	}

	// Absolute path starting with / — try local first, then USS
	if strings.HasPrefix(source, "/") {
		if _, err := os.Stat(source); err == nil {
			data, err := os.ReadFile(source)
			if err != nil {
				return "", err
			}
			return string(data), nil
		}
		if conn == nil {
			return "", fmt.Errorf("no connection available for USS path %s", source)
		}
		data, err := conn.ReadFile(source)
		if err != nil {
			return "", err
		}
		return string(data), nil
	}

	// PDS member: DATASET(MEMBER)
	if strings.Contains(source, "(") {
		if conn == nil {
			return "", fmt.Errorf("no connection available for dataset %s", source)
		}
		dataset, member, err := parseDSN(source)
		if err != nil {
			return "", err
		}
		data, err := conn.ReadMember(dataset, member)
		if err != nil {
			return "", err
		}
		return string(data), nil
	}

	return "", fmt.Errorf("cannot resolve source: %s", source)
}

func splitLines(content string) []string {
	if content == "" {
		return nil
	}
	lines := strings.Split(content, "\n")
	// Remove trailing empty line from final newline
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

const (
	colorRed   = "\033[31m"
	colorGreen = "\033[32m"
	colorCyan  = "\033[36m"
	colorReset = "\033[0m"
)

func printColorDiff(output string) {
	var buf strings.Builder
	for _, line := range strings.Split(output, "\n") {
		switch {
		case strings.HasPrefix(line, "---") || strings.HasPrefix(line, "+++"):
			buf.WriteString(colorCyan + line + colorReset + "\n")
		case strings.HasPrefix(line, "@@"):
			buf.WriteString(colorCyan + line + colorReset + "\n")
		case strings.HasPrefix(line, "-"):
			buf.WriteString(colorRed + line + colorReset + "\n")
		case strings.HasPrefix(line, "+"):
			buf.WriteString(colorGreen + line + colorReset + "\n")
		default:
			buf.WriteString(line + "\n")
		}
	}
	fmt.Print(buf.String())
}
