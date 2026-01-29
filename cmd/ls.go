package cmd

import (
	"fmt"

	"zm/internal/connection"

	"github.com/spf13/cobra"
)

var lsCmd = &cobra.Command{
	Use:   "ls [dataset]",
	Short: "List datasets or members",
	Long: `List datasets matching a pattern, or members of a PDS.

Examples:
  zm ls                    # list datasets matching HLQ.*
  zm ls 'USERNAME.SOURCE'   # list members in PDS`,
	RunE: runLs,
}

func init() {
	rootCmd.AddCommand(lsCmd)
}

func runLs(cmd *cobra.Command, args []string) error {
	profile, err := GetCurrentProfile()
	if err != nil {
		return err
	}

	conn := connection.NewFTPConnection(profile.Host, profile.Port, profile.User, profile.Password)
	if err := conn.Connect(); err != nil {
		return err
	}
	defer conn.Close()

	if len(args) == 0 {
		datasets, err := conn.ListDatasets(profile.HLQ)
		if err != nil {
			return err
		}
		for _, ds := range datasets {
			fmt.Println(ds)
		}
		return nil
	}

	dataset := args[0]
	members, err := conn.ListMembers(dataset)
	if err != nil {
		return err
	}
	for _, m := range members {
		fmt.Println(m)
	}
	return nil
}
