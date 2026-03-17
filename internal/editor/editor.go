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
// If editorOverride is non-empty, it is used instead of the detected editor.
func Open(path, editorOverride string) error {
	ed := editorOverride
	if ed == "" {
		ed = DetectEditor()
	}

	parts := strings.Fields(ed)
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
