package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"zm/internal/connection"

	"github.com/spf13/cobra"
)

var (
	jobsOwner  string
	jobsOutput bool
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
}

func runJobs(cmd *cobra.Command, args []string) error {
	_, conn, err := openConnection()
	if err != nil {
		return err
	}
	defer conn.Close()

	if len(args) > 0 {
		jobid := args[0]

		if jobsOutput {
			output, err := conn.GetJobOutput(jobid)
			if err != nil {
				return err
			}
			fmt.Print(string(output))
			return nil
		}

		job, err := conn.GetJobStatus(jobid)
		if err != nil {
			return err
		}
		printJobDetail(job)
		return nil
	}

	if jobsOutput {
		return fmt.Errorf("--output requires a jobid")
	}

	jobs, err := conn.ListJobs(jobsOwner)
	if err != nil {
		return err
	}

	if len(jobs) == 0 {
		fmt.Println("No jobs found")
		return nil
	}

	printJobList(jobs)
	return nil
}

func printJobList(jobs []connection.JobStatus) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "JOBNAME\tJOBID\tOWNER\tSTATUS\tRC")
	for _, j := range jobs {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", j.JobName, j.JobID, j.Owner, j.Status, j.RetCode)
	}
	w.Flush()
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
