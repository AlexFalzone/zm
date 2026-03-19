package cmd

import (
	"fmt"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"zm/internal/connection"

	"github.com/spf13/cobra"
)

// ServerGrepper is optionally implemented by connections that support server-side grep.
type ServerGrepper interface {
	GrepMember(dataset, member, pattern string, caseInsensitive bool) ([]byte, error)
}

var (
	grepFilter          string
	grepMax             int
	grepCaseInsensitive bool
	grepLineNumbers     bool
	grepRegex           bool
)

var grepCmd = &cobra.Command{
	Use:   "grep PATTERN TARGET",
	Short: "Search for a pattern in PDS members or USS files",
	Long:  `Search for a text pattern across all members of a PDS dataset or files in a USS directory.`,
	Args:  cobra.ExactArgs(2),
	RunE:  runGrep,
}

func init() {
	rootCmd.AddCommand(grepCmd)
	grepCmd.Flags().StringVar(&grepFilter, "filter", "", "filter members by wildcard pattern (e.g. PROG*)")
	grepCmd.Flags().IntVar(&grepMax, "max", 0, "stop after N total matches")
	grepCmd.Flags().BoolVarP(&grepCaseInsensitive, "ignore-case", "i", false, "case insensitive search")
	grepCmd.Flags().BoolVarP(&grepLineNumbers, "line-number", "n", true, "show line numbers")
	grepCmd.Flags().BoolVar(&grepRegex, "regex", false, "treat pattern as regex")
}

type grepMatch struct {
	Member string
	Line   int
	Text   string
}

func buildMatcher(pattern string, isRegex, caseInsensitive bool) (func(string) bool, error) {
	if isRegex {
		flags := ""
		if caseInsensitive {
			flags = "(?i)"
		}
		re, err := regexp.Compile(flags + pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid regex: %w", err)
		}
		return re.MatchString, nil
	}

	if caseInsensitive {
		// Use compiled regex for case-insensitive literal search —
		// avoids strings.ToUpper() allocation per line
		re, err := regexp.Compile("(?i)" + regexp.QuoteMeta(pattern))
		if err != nil {
			return nil, fmt.Errorf("invalid pattern: %w", err)
		}
		return re.MatchString, nil
	}
	return func(line string) bool {
		return strings.Contains(line, pattern)
	}, nil
}

func searchLines(content string, matcher func(string) bool) []grepMatch {
	lines := strings.Split(content, "\n")
	var matches []grepMatch
	for i, line := range lines {
		if matcher(line) {
			matches = append(matches, grepMatch{Line: i + 1, Text: line})
		}
	}
	return matches
}

func runGrep(cmd *cobra.Command, args []string) error {
	pattern := args[0]
	target := trimQuotes(args[1])

	_, conn, err := openConnection()
	if err != nil {
		return err
	}
	defer conn.Close()

	matcher, err := buildMatcher(pattern, grepRegex, grepCaseInsensitive)
	if err != nil {
		return err
	}

	totalMatches := 0

	if strings.HasPrefix(target, "/") {
		return grepUSS(cmd, conn, target, pattern, matcher)
	}

	// Wildcard dataset pattern (e.g. FALZONE.*) → resolve datasets first
	if strings.Contains(target, "*") {
		return grepDatasetPattern(cmd, conn, target, pattern, matcher, &totalMatches)
	}

	return grepPDS(cmd, conn, target, pattern, matcher, &totalMatches)
}

func grepDatasetPattern(cmd *cobra.Command, conn connection.Connection, dsPattern, pattern string, matcher func(string) bool, totalMatches *int) error {
	datasets, err := conn.ListDatasets(dsPattern)
	if err != nil {
		return err
	}

	for _, ds := range datasets {
		if grepMax > 0 && *totalMatches >= grepMax {
			break
		}
		if err := grepPDS(cmd, conn, ds, pattern, matcher, totalMatches); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "skipping %s (not a PDS)\n", ds)
		}
	}
	return nil
}

func grepPDS(cmd *cobra.Command, conn connection.Connection, dataset, pattern string, matcher func(string) bool, totalMatches *int) error {
	members, err := conn.ListMembers(dataset)
	if err != nil {
		return err
	}

	if grepFilter != "" {
		members = filterMembers(members, grepFilter)
	}

	// Use parallel reads when connection supports it (z/OSMF)
	if cc, ok := conn.(connection.ConcurrentConnection); ok {
		return grepPDSParallel(cmd, conn, dataset, matcher, members, totalMatches, cc.MaxConcurrency())
	}

	return grepPDSSequential(cmd, conn, dataset, pattern, matcher, members, totalMatches)
}

func grepPDSSequential(cmd *cobra.Command, conn connection.Connection, dataset, pattern string, matcher func(string) bool, members []connection.Member, totalMatches *int) error {
	grepper, hasServerGrep := conn.(ServerGrepper)

	for _, m := range members {
		if grepMax > 0 && *totalMatches >= grepMax {
			break
		}

		var matches []grepMatch

		if hasServerGrep && !grepRegex {
			out, err := grepper.GrepMember(dataset, m.Name, pattern, grepCaseInsensitive)
			if err != nil {
				return fmt.Errorf("grep %s(%s): %w", dataset, m.Name, err)
			}
			if len(out) > 0 {
				matches = parseGrepOutput(string(out), m.Name)
			}
		} else {
			content, err := conn.ReadMember(dataset, m.Name)
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: cannot read %s(%s): %v\n", dataset, m.Name, err)
				continue
			}
			matches = searchLines(string(content), matcher)
		}

		displayName := fmt.Sprintf("%s(%s)", dataset, m.Name)
		var printErr error
		*totalMatches, printErr = printMatches(matches, displayName, *totalMatches)
		if printErr != nil {
			return printErr
		}
	}

	return nil
}

type memberContent struct {
	content []byte
	err     error
}

func grepPDSParallel(cmd *cobra.Command, conn connection.Connection, dataset string, matcher func(string) bool, members []connection.Member, totalMatches *int, concurrency int) error {
	// Pre-fetch all member contents in parallel
	results := make([]memberContent, len(members))
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	for i, m := range members {
		wg.Add(1)
		go func(idx int, name string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			content, err := conn.ReadMember(dataset, name)
			results[idx] = memberContent{content: content, err: err}
		}(i, m.Name)
	}
	wg.Wait()

	// Search and print in order
	for i, m := range members {
		if grepMax > 0 && *totalMatches >= grepMax {
			break
		}

		if results[i].err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: cannot read %s(%s): %v\n", dataset, m.Name, results[i].err)
			continue
		}

		matches := searchLines(string(results[i].content), matcher)
		displayName := fmt.Sprintf("%s(%s)", dataset, m.Name)
		var printErr error
		*totalMatches, printErr = printMatches(matches, displayName, *totalMatches)
		if printErr != nil {
			return printErr
		}
	}

	return nil
}

func grepUSS(cmd *cobra.Command, conn connection.Connection, dirPath, pattern string, matcher func(string) bool) error {
	totalMatches := 0
	return grepUSSRecursive(cmd, conn, dirPath, dirPath, matcher, &totalMatches)
}

func grepUSSRecursive(cmd *cobra.Command, conn connection.Connection, basePath, dirPath string, matcher func(string) bool, totalMatches *int) error {
	if grepMax > 0 && *totalMatches >= grepMax {
		return nil
	}

	files, err := conn.ListFiles(dirPath)
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: cannot list %s: %v\n", dirPath, err)
		return nil
	}

	for _, f := range files {
		if grepMax > 0 && *totalMatches >= grepMax {
			break
		}

		filePath := path.Join(dirPath, f.Name)

		if f.Type == "directory" {
			if err := grepUSSRecursive(cmd, conn, basePath, filePath, matcher, totalMatches); err != nil {
				return err
			}
			continue
		}

		if f.Type != "file" {
			continue
		}

		if grepFilter != "" && !matchWildcard(strings.ToUpper(f.Name), strings.ToUpper(grepFilter)) {
			continue
		}

		content, err := conn.ReadFile(filePath)
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: cannot read %s: %v\n", filePath, err)
			continue
		}

		// Show path relative to the base search path
		displayPath := filePath
		if rel := strings.TrimPrefix(filePath, basePath); rel != filePath {
			displayPath = strings.TrimPrefix(rel, "/")
		}

		matches := searchLines(string(content), matcher)
		var printErr error
		*totalMatches, printErr = printMatches(matches, displayPath, *totalMatches)
		if printErr != nil {
			return printErr
		}
	}

	return nil
}

func printMatches(matches []grepMatch, name string, totalMatches int) (int, error) {
	for _, match := range matches {
		if grepMax > 0 && totalMatches >= grepMax {
			break
		}
		if grepLineNumbers {
			fmt.Printf("%s:%d:%s\n", name, match.Line, match.Text)
		} else {
			fmt.Printf("%s:%s\n", name, match.Text)
		}
		totalMatches++
	}
	return totalMatches, nil
}

func parseGrepOutput(output, member string) []grepMatch {
	lines := strings.Split(output, "\n")
	var matches []grepMatch
	for _, line := range lines {
		if line == "" {
			continue
		}
		// grep -n output format: "LINENUM:content"
		colonIdx := strings.IndexByte(line, ':')
		if colonIdx == -1 {
			continue
		}
		lineNum, _ := strconv.Atoi(line[:colonIdx])
		matches = append(matches, grepMatch{
			Member: member,
			Line:   lineNum,
			Text:   line[colonIdx+1:],
		})
	}
	return matches
}
