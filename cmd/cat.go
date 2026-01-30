package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"zm/internal/connection"
)

var catCmd = &cobra.Command{
	Use:   "cat <dataset(member)>",
	Short: "Display content of a member or USS file",
	Long: `Display the content of a PDS member or USS file.

Examples:
  zm cat 'USERNAME.SOURCE(MYPROG)'   # display PDS member
  zm cat /u/username/file.txt        # display USS file`,
	Args: cobra.ExactArgs(1),
	RunE: runCat,
}

func init() {
	rootCmd.AddCommand(catCmd)
}

func runCat(cmd *cobra.Command, args []string) error {
	profile, err := GetCurrentProfile()
	if err != nil {
		return err
	}

	conn := connection.NewFTPConnection(profile.Host, profile.Port, profile.User, profile.Password)
	if err := conn.Connect(); err != nil {
		return err
	}
	defer conn.Close()

	path := args[0]

	// USS path starts with /
	if path[0] == '/' {
		content, err := conn.ReadFile(path)
		if err != nil {
			return err
		}
		fmt.Print(string(content))
		return nil
	}

	// Dataset member: DATASET(MEMBER)
	dataset, member, err := parseDSN(path)
	if err != nil {
		return err
	}

	content, err := conn.ReadMember(dataset, member)
	if err != nil {
		return err
	}
	fmt.Print(string(content))
	return nil
}

func parseDSN(dsn string) (dataset, member string, err error) {
	dsn = trimQuotes(dsn)

	start := -1
	end := -1
	for i, c := range dsn {
		if c == '(' {
			start = i
		} else if c == ')' {
			end = i
		}
	}

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
