package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var lsCmd = &cobra.Command{
	Use:   "ls [dataset]",
	Short: "List datasets or members",
	Long:  `List datasets matching a pattern, or members of a PDS.`,
	RunE:  runLs,
}

func init() {
	rootCmd.AddCommand(lsCmd)
}

func runLs(cmd *cobra.Command, args []string) error {
	profile, conn, err := openConnection()
	if err != nil {
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

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tVV.MM\tCHANGED\tSIZE\tUSER")
	for _, m := range members {
		fmt.Fprintf(w, "%s\t%02d.%02d\t%s\t%d\t%s\n",
			m.Name, m.VV, m.MM, m.Changed, m.Size, m.User)
	}
	w.Flush()
	return nil
}
