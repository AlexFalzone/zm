package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var (
	catLines bool
	catHead  int
	catTail  int
)

var catCmd = &cobra.Command{
	Use:   "cat <dataset(member)>",
	Short: "Display content of a member or USS file",
	Long:  `Display the content of a PDS member or USS file.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runCat,
}

func init() {
	rootCmd.AddCommand(catCmd)
	catCmd.Flags().BoolVarP(&catLines, "lines", "n", false, "show line numbers")
	catCmd.Flags().IntVar(&catHead, "head", 0, "show first N lines")
	catCmd.Flags().IntVar(&catTail, "tail", 0, "show last N lines")
}

func runCat(cmd *cobra.Command, args []string) error {
	_, conn, err := openConnection()
	if err != nil {
		return err
	}
	defer conn.Close()

	if catHead > 0 && catTail > 0 {
		return fmt.Errorf("--head and --tail are mutually exclusive")
	}

	path := args[0]
	if path == "" {
		return fmt.Errorf("path cannot be empty")
	}

	var content []byte

	if path[0] == '/' {
		content, err = conn.ReadFile(path)
	} else {
		dataset, member, parseErr := parseDSN(path)
		if parseErr != nil {
			return parseErr
		}
		content, err = conn.ReadMember(dataset, member)
	}

	if err != nil {
		return err
	}

	printContent(string(content), catLines, catHead, catTail)
	return nil
}

func printContent(content string, showLines bool, head, tail int) {
	lines := strings.Split(content, "\n")

	// Remove trailing empty line from final newline
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	if head > 0 && head < len(lines) {
		lines = lines[:head]
	} else if tail > 0 && tail < len(lines) {
		lines = lines[len(lines)-tail:]
	}

	for i, line := range lines {
		if showLines {
			fmt.Printf("%6d  %s\n", i+1, line)
		} else {
			fmt.Println(line)
		}
	}
}

func parseDSN(dsn string) (dataset, member string, err error) {
	dsn = trimQuotes(dsn)

	start := strings.IndexByte(dsn, '(')
	end := strings.IndexByte(dsn, ')')

	if start == -1 || end == -1 || end <= start+1 {
		return "", "", fmt.Errorf("invalid dataset format: %s (expected DATASET(MEMBER))", dsn)
	}

	dataset = dsn[:start]
	member = dsn[start+1 : end]
	return dataset, member, nil
}

func trimQuotes(s string) string {
	if len(s) >= 2 && s[0] == '\'' && s[len(s)-1] == '\'' {
		return s[1 : len(s)-1]
	}
	return s
}
