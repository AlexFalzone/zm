package editor

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func DetectEditor() string {
	if e := os.Getenv("VISUAL"); e != "" {
		return e
	}
	if e := os.Getenv("EDITOR"); e != "" {
		return e
	}
	return "vi"
}

// Open opens the file in the user's editor and blocks until the editor exits.
func Open(path string) error {
	editor := DetectEditor()

	parts := strings.Fields(editor)
	bin := parts[0]
	args := append(parts[1:], path)

	cmd := exec.Command(bin, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("editor exited with error: %w", err)
	}
	return nil
}
