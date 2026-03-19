package cmd

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"zm/internal/connection"

	"github.com/spf13/cobra"
)

var (
	jobsOwner   string
	jobsOutput  bool
	jobsCancel  bool
	jobsPurge   bool
	jobsTail    bool
	jobsDD      string
	jobsStatus  string
	jobsLimit   int
	jobsSort    string
	jobsReverse bool
)

var jobsCmd = &cobra.Command{
	Use:   "jobs [jobid]",
	Short: "List jobs or show job status/output",
	Long:  `List jobs for current user, or show status/output of a specific job.`,
	RunE:  runJobs,
}

func init() {
	rootCmd.AddCommand(jobsCmd)
	jobsCmd.Flags().StringVar(&jobsOwner, "owner", "", "filter by owner (default: current user, use '*' for all)")
	jobsCmd.Flags().BoolVarP(&jobsOutput, "output", "o", false, "show job output (requires jobid)")
	jobsCmd.Flags().BoolVar(&jobsCancel, "cancel", false, "cancel an active job (requires jobid)")
	jobsCmd.Flags().BoolVar(&jobsPurge, "purge", false, "purge job from spool (requires jobid)")
	jobsCmd.Flags().BoolVarP(&jobsTail, "tail", "f", false, "follow job output in real-time (requires jobid)")
	jobsCmd.Flags().StringVar(&jobsDD, "dd", "", "filter output by DD name (requires --output)")
	jobsCmd.Flags().StringVar(&jobsStatus, "status", "", "filter job list by status (ACTIVE, OUTPUT, INPUT)")
	jobsCmd.Flags().IntVar(&jobsLimit, "limit", 0, "limit number of results")
	jobsCmd.Flags().StringVar(&jobsSort, "sort", "jobid", "sort jobs by: jobid, jobname, owner, status, rc")
	jobsCmd.Flags().BoolVarP(&jobsReverse, "reverse", "r", false, "reverse sort order")
}

func runJobs(cmd *cobra.Command, args []string) error {
	_, conn, err := openConnection()
	if err != nil {
		return err
	}
	defer conn.Close()

	if len(args) > 0 {
		jobid := args[0]

		// Mutually exclusive actions
		actions := 0
		if jobsCancel {
			actions++
		}
		if jobsPurge {
			actions++
		}
		if jobsTail {
			actions++
		}
		if jobsOutput {
			actions++
		}
		if actions > 1 {
			return fmt.Errorf("--output, --cancel, --purge, and --tail are mutually exclusive")
		}

		if jobsCancel {
			if err := conn.CancelJob(jobid); err != nil {
				return err
			}
			fmt.Printf("Job %s cancel requested\n", jobid)
			return nil
		}

		if jobsPurge {
			if err := conn.PurgeJob(jobid); err != nil {
				return err
			}
			fmt.Printf("Job %s purged\n", jobid)
			return nil
		}

		if jobsTail {
			return tailJob(conn, jobid)
		}

		if jobsOutput {
			output, err := conn.GetJobOutput(jobid)
			if err != nil {
				return err
			}
			if jobsDD != "" {
				fmt.Print(filterByDD(string(output), jobsDD))
			} else {
				fmt.Print(string(output))
			}
			return nil
		}

		job, err := conn.GetJobStatus(jobid)
		if err != nil {
			return err
		}
		printJobDetail(job)
		return nil
	}

	// List mode — validate flags
	if jobsOutput {
		return fmt.Errorf("--output requires a jobid")
	}
	if jobsCancel {
		return fmt.Errorf("--cancel requires a jobid")
	}
	if jobsPurge {
		return fmt.Errorf("--purge requires a jobid")
	}
	if jobsTail {
		return fmt.Errorf("--tail requires a jobid")
	}
	if jobsDD != "" {
		return fmt.Errorf("--dd requires --output and a jobid")
	}

	jobs, err := conn.ListJobs(jobsOwner)
	if err != nil {
		return err
	}

	if jobsStatus != "" {
		filter := strings.ToUpper(jobsStatus)
		filtered := make([]connection.JobStatus, 0, len(jobs))
		for _, j := range jobs {
			if j.Status == filter {
				filtered = append(filtered, j)
			}
		}
		jobs = filtered
	}

	sortJobs(jobs, jobsSort)

	if jobsReverse {
		for i, j := 0, len(jobs)-1; i < j; i, j = i+1, j-1 {
			jobs[i], jobs[j] = jobs[j], jobs[i]
		}
	}

	if jobsLimit > 0 && len(jobs) > jobsLimit {
		jobs = jobs[:jobsLimit]
	}

	if len(jobs) == 0 {
		fmt.Println("No jobs found")
		return nil
	}

	printJobList(jobs)
	return nil
}

func tailJob(conn connection.Connection, jobid string) error {
	var lastLen int
	delay := time.Second

	for {
		status, err := conn.GetJobStatus(jobid)
		if err != nil {
			return err
		}

		output, err := conn.GetJobOutput(jobid)
		if err != nil && status.Status != "ACTIVE" {
			return err
		}

		if len(output) > lastLen {
			fmt.Print(string(output[lastLen:]))
			lastLen = len(output)
			delay = time.Second // reset on new output
		} else if delay < 30*time.Second {
			delay = delay * 3 / 2 // grow 1.5x up to 30s
		}

		if status.Status == "OUTPUT" {
			return nil
		}

		time.Sleep(delay)
	}
}

func filterByDD(output, ddName string) string {
	ddName = strings.ToUpper(ddName)
	var result strings.Builder
	lines := strings.Split(output, "\n")
	capturing := false

	for _, line := range lines {
		if strings.HasPrefix(line, "--- DD: ") {
			// Extract DD name from header "--- DD: NAME (Step: STEP) ---"
			header := strings.TrimPrefix(line, "--- DD: ")
			if spaceIdx := strings.IndexByte(header, ' '); spaceIdx != -1 {
				header = header[:spaceIdx]
			}
			capturing = strings.ToUpper(header) == ddName
			if capturing {
				result.WriteString(line)
				result.WriteByte('\n')
			}
			continue
		}
		if capturing {
			result.WriteString(line)
			result.WriteByte('\n')
		}
	}

	return result.String()
}

func printJobList(jobs []connection.JobStatus) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "JOBNAME\tJOBID\tOWNER\tSTATUS\tRC")
	for _, j := range jobs {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", j.JobName, j.JobID, j.Owner, j.Status, j.RetCode)
	}
	w.Flush()
}

func sortJobs(jobs []connection.JobStatus, field string) {
	switch strings.ToLower(field) {
	case "jobname":
		sort.Slice(jobs, func(i, j int) bool {
			return jobs[i].JobName < jobs[j].JobName
		})
	case "owner":
		sort.Slice(jobs, func(i, j int) bool {
			return jobs[i].Owner < jobs[j].Owner
		})
	case "status":
		sort.Slice(jobs, func(i, j int) bool {
			return jobs[i].Status < jobs[j].Status
		})
	case "rc":
		sort.Slice(jobs, func(i, j int) bool {
			return jobs[i].RetCode < jobs[j].RetCode
		})
	default: // "jobid"
		sort.Slice(jobs, func(i, j int) bool {
			return jobs[i].JobID < jobs[j].JobID
		})
	}
}

func printJobDetail(job *connection.JobStatus) {
	fmt.Printf("Job ID:    %s\n", job.JobID)
	fmt.Printf("Job Name:  %s\n", job.JobName)
	fmt.Printf("Owner:     %s\n", job.Owner)
	fmt.Printf("Status:    %s\n", job.Status)
	if job.RetCode != "" {
		fmt.Printf("Return:    %s\n", job.RetCode)
	}
	if job.Class != "" {
		fmt.Printf("Class:     %s\n", job.Class)
	}
}
