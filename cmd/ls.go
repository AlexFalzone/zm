package cmd

import (
	"fmt"
	"os"
	"path"
	"sort"
	"strings"
	"text/tabwriter"

	"zm/internal/connection"

	"github.com/spf13/cobra"
)

var (
	lsSort    string
	lsFilter  string
	lsReverse bool
)

var lsCmd = &cobra.Command{
	Use:   "ls [dataset]",
	Short: "List datasets or members",
	Long:  `List datasets matching a pattern, or members of a PDS.`,
	RunE:  runLs,
}

func init() {
	rootCmd.AddCommand(lsCmd)
	lsCmd.Flags().StringVar(&lsSort, "sort", "name", "sort members by: name, changed, size, user")
	lsCmd.Flags().StringVar(&lsFilter, "filter", "", "filter members by wildcard pattern (e.g. PROG*)")
	lsCmd.Flags().BoolVarP(&lsReverse, "reverse", "r", false, "reverse sort order")
}

func runLs(cmd *cobra.Command, args []string) error {
	profile, conn, err := openConnection()
	if err != nil {
		return err
	}
	defer conn.Close()

	// No args → list HLQ.*
	if len(args) == 0 {
		return listDatasets(conn, profile.HLQ+".*")
	}

	arg := args[0]

	// USS path
	if strings.HasPrefix(arg, "/") {
		return listUSSFiles(conn, arg)
	}

	// If contains wildcard → dataset pattern search
	if strings.Contains(arg, "*") {
		return listDatasets(conn, arg)
	}

	// Otherwise → list members
	members, err := conn.ListMembers(arg)
	if err != nil {
		return err
	}

	if lsFilter != "" {
		members = filterMembers(members, lsFilter)
	}

	sortMembers(members, lsSort)

	if lsReverse {
		for i, j := 0, len(members)-1; i < j; i, j = i+1, j-1 {
			members[i], members[j] = members[j], members[i]
		}
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

func listUSSFiles(conn connection.Connection, dirPath string) error {
	files, err := conn.ListFiles(dirPath)
	if err != nil {
		return err
	}

	if len(files) == 0 {
		fmt.Println("No files found")
		return nil
	}

	if lsFilter != "" {
		filter := strings.ToUpper(lsFilter)
		filtered := make([]connection.USSFile, 0, len(files))
		for _, f := range files {
			if matchWildcard(strings.ToUpper(f.Name), filter) {
				filtered = append(filtered, f)
			}
		}
		files = filtered
	}

	sortUSSFiles(files, lsSort)

	if lsReverse {
		for i, j := 0, len(files)-1; i < j; i, j = i+1, j-1 {
			files[i], files[j] = files[j], files[i]
		}
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "TYPE\tNAME\tSIZE\tMODIFIED")
	for _, f := range files {
		typeTag := " "
		if f.Type == "directory" {
			typeTag = "d"
		} else if f.Type == "symlink" {
			typeTag = "l"
		}
		fmt.Fprintf(w, "%s\t%s\t%d\t%s\n", typeTag, f.Name, f.Size, f.Mtime)
	}
	w.Flush()
	return nil
}

func listDatasets(conn connection.Connection, pattern string) error {
	datasets, err := conn.ListDatasets(pattern)
	if err != nil {
		return err
	}

	if lsFilter != "" {
		filter := strings.ToUpper(lsFilter)
		filtered := make([]string, 0, len(datasets))
		for _, ds := range datasets {
			if matchWildcard(ds, filter) {
				filtered = append(filtered, ds)
			}
		}
		datasets = filtered
	}

	for _, ds := range datasets {
		fmt.Println(ds)
	}
	return nil
}

func filterMembers(members []connection.Member, pattern string) []connection.Member {
	pattern = strings.ToUpper(pattern)
	filtered := make([]connection.Member, 0, len(members))
	for _, m := range members {
		if matchWildcard(strings.ToUpper(m.Name), pattern) {
			filtered = append(filtered, m)
		}
	}
	return filtered
}

func matchWildcard(name, pattern string) bool {
	matched, _ := path.Match(pattern, name)
	return matched
}

func sortMembers(members []connection.Member, field string) {
	switch strings.ToLower(field) {
	case "changed":
		sort.Slice(members, func(i, j int) bool {
			return members[i].Changed < members[j].Changed
		})
	case "size":
		sort.Slice(members, func(i, j int) bool {
			return members[i].Size < members[j].Size
		})
	case "user":
		sort.Slice(members, func(i, j int) bool {
			return members[i].User < members[j].User
		})
	default: // "name"
		sort.Slice(members, func(i, j int) bool {
			return members[i].Name < members[j].Name
		})
	}
}

func sortUSSFiles(files []connection.USSFile, field string) {
	switch strings.ToLower(field) {
	case "size":
		sort.Slice(files, func(i, j int) bool {
			return files[i].Size < files[j].Size
		})
	case "changed":
		sort.Slice(files, func(i, j int) bool {
			return files[i].Mtime < files[j].Mtime
		})
	default: // "name"
		sort.Slice(files, func(i, j int) bool {
			return files[i].Name < files[j].Name
		})
	}
}
