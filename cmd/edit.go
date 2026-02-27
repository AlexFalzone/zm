package cmd

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"zm/internal/connection"
	"zm/internal/editor"

	"github.com/spf13/cobra"
)

var editCmd = &cobra.Command{
	Use:   "edit <dataset(member)> | <uss-path>",
	Short: "Edit a member or USS file",
	Long:  `Download a PDS member or USS file, open it in your editor, and upload changes.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runEdit,
}

func init() {
	rootCmd.AddCommand(editCmd)
}

func runEdit(cmd *cobra.Command, args []string) error {
	_, conn, err := openConnection()
	if err != nil {
		return err
	}
	defer conn.Close()

	path := args[0]
	if path == "" {
		return fmt.Errorf("path cannot be empty")
	}

	if path[0] == '/' {
		return editUSSFile(conn, path)
	}

	return editMember(conn, path)
}

func editMember(conn connection.Connection, dsn string) error {
	dataset, member, err := parseDSN(dsn)
	if err != nil {
		return err
	}

	content, err := conn.ReadMember(dataset, member)
	if err != nil {
		return err
	}

	tmpFile, err := writeTempFile(member, content)
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile)

	if err := editor.Open(tmpFile); err != nil {
		return err
	}

	modified, err := os.ReadFile(tmpFile)
	if err != nil {
		return fmt.Errorf("failed to read edited file: %w", err)
	}

	if bytes.Equal(content, modified) {
		fmt.Println("No changes, skipping upload")
		return nil
	}

	if err := conn.WriteMember(dataset, member, modified); err != nil {
		return err
	}

	fmt.Printf("Uploaded %s(%s)\n", strings.Trim(dataset, "'"), member)
	return nil
}

func editUSSFile(conn connection.Connection, path string) error {
	content, err := conn.ReadFile(path)
	if err != nil {
		return err
	}

	name := filepath.Base(path)
	tmpFile, err := writeTempFile(name, content)
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile)

	if err := editor.Open(tmpFile); err != nil {
		return err
	}

	modified, err := os.ReadFile(tmpFile)
	if err != nil {
		return fmt.Errorf("failed to read edited file: %w", err)
	}

	if bytes.Equal(content, modified) {
		fmt.Println("No changes, skipping upload")
		return nil
	}

	if err := conn.WriteFile(path, modified); err != nil {
		return err
	}

	fmt.Printf("Uploaded %s\n", path)
	return nil
}

func writeTempFile(name string, content []byte) (string, error) {
	ext := filepath.Ext(name)
	if ext == "" {
		ext = ".txt"
	}
	prefix := strings.TrimSuffix(name, ext) + "-"

	f, err := os.CreateTemp("", prefix+"*"+ext)
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}

	if _, err := f.Write(content); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", fmt.Errorf("failed to write temp file: %w", err)
	}

	f.Close()
	return f.Name(), nil
}
