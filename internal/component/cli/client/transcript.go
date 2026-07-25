// Design: docs/architecture/core-design.md — CLI transcript file creation
// Related: ../transcript.go — TranscriptWriter

package client

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	unicli "github.com/ze-software/ze/internal/component/cli"
)

// openTranscriptFile creates the transcript directory and file if transcript
// is enabled. Returns nil if disabled or on error (warnings printed to stderr).
func openTranscriptFile() *os.File {
	if !unicli.TranscriptEnabled() {
		return nil
	}

	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: transcript: %v\n", err)
			return nil
		}
		dataHome = filepath.Join(home, ".local", "share")
	}

	dir := filepath.Join(dataHome, "ze", "transcripts")
	if mkErr := os.MkdirAll(dir, 0o700); mkErr != nil {
		fmt.Fprintf(os.Stderr, "warning: transcript directory: %v\n", mkErr)
		return nil
	}

	filename := "transcript-" + time.Now().Format("20060102-150405") + "-" + strconv.Itoa(os.Getpid()) + ".log"
	path := filepath.Join(dir, filename)

	f, openErr := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600) //nolint:gosec // path constructed from UserHomeDir + constant subpath
	if openErr != nil {
		fmt.Fprintf(os.Stderr, "warning: transcript file: %v\n", openErr)
		return nil
	}

	return f
}
