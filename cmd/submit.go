package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"zm/internal/connection"

	"github.com/spf13/cobra"
)

var (
	submitWait    bool
	submitOutput  bool
	submitTimeout time.Duration
)

var submitCmd = &cobra.Command{
	Use:   "submit <dataset(member)> | <local-file>",
	Short: "Submit JCL for execution",
	Long:  `Submit JCL from a PDS member or local file.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runSubmit,
}

func init() {
	rootCmd.AddCommand(submitCmd)
	submitCmd.Flags().BoolVarP(&submitWait, "wait", "w", false, "wait for job to complete")
	submitCmd.Flags().BoolVarP(&submitOutput, "output", "o", false, "show output on completion (implies --wait)")
	submitCmd.Flags().DurationVar(&submitTimeout, "timeout", 15*time.Minute, "maximum time to wait for job completion")
}

func runSubmit(cmd *cobra.Command, args []string) error {
	_, conn, err := openConnection()
	if err != nil {
		return err
	}
	defer conn.Close()

	source := args[0]
	var jcl []byte

	if strings.HasPrefix(source, "/") {
		// Could be local or USS remote
		if _, statErr := os.Stat(source); statErr == nil {
			jcl, err = os.ReadFile(source)
			if err != nil {
				return fmt.Errorf("failed to read %s: %w", source, err)
			}
		} else {
			// USS remote file
			jcl, err = conn.ReadFile(source)
			if err != nil {
				return fmt.Errorf("failed to read USS file %s: %w", source, err)
			}
		}
	} else if _, statErr := os.Stat(source); statErr == nil {
		// Local file (relative path)
		jcl, err = os.ReadFile(source)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", source, err)
		}
	} else {
		// PDS member
		dataset, member, err := parseDSN(source)
		if err != nil {
			return err
		}
		jcl, err = conn.ReadMember(dataset, member)
		if err != nil {
			return err
		}
	}

	jobid, err := conn.SubmitJCL(jcl)
	if err != nil {
		return err
	}

	fmt.Printf("Job %s submitted\n", jobid)

	if submitOutput {
		submitWait = true
	}

	if !submitWait {
		return nil
	}

	if err := waitForJob(conn, jobid, submitTimeout); err != nil {
		return err
	}

	if submitOutput {
		output, err := conn.GetJobOutput(jobid)
		if err != nil {
			return fmt.Errorf("failed to get job output: %w", err)
		}
		fmt.Print(string(output))
	}

	return nil
}

func waitForJob(conn connection.Connection, jobid string, timeout time.Duration) error {
	fmt.Printf("Waiting for %s...", jobid)
	delay := time.Second
	deadline := time.Now().Add(timeout)

	for {
		if time.Now().After(deadline) {
			fmt.Println()
			return fmt.Errorf("timed out waiting for job %s after %s", jobid, timeout)
		}

		status, err := conn.GetJobStatus(jobid)
		if err != nil {
			fmt.Println()
			return err
		}

		if status.Status == "OUTPUT" {
			fmt.Println()
			rc := status.RetCode
			if rc == "" {
				rc = "N/A"
			}
			fmt.Printf("Job %s completed — %s\n", jobid, rc)

			if strings.Contains(rc, "ABEND") {
				return fmt.Errorf("job ended with %s", rc)
			}
			return nil
		}

		fmt.Print(".")
		time.Sleep(delay)
		if delay < 30*time.Second {
			delay = delay * 3 / 2
		}
	}
}
