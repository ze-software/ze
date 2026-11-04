// Design: docs/contributing/running-commands.md -- terminal-only issuance boundaries.
// Package humantest supplies real process streams for producer boundary tests.
package humantest

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/creack/pty"
)

// Streams installs a real terminal, pipes, or closed descriptors for one test.
// Callers MUST NOT run in parallel; cleanup restores both process streams.
func Streams(t testing.TB, mode string) {
	t.Helper()
	master, terminal, err := pty.Open()
	if err != nil {
		t.Fatal(err)
	}
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	closed, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	if err := closed.Close(); err != nil {
		t.Fatal(err)
	}
	stdin, stdout := os.Stdin, os.Stdout
	os.Stdin, os.Stdout = terminal, terminal
	switch mode {
	case "terminal":
	case "piped-stdin":
		os.Stdin = reader
	case "piped-stdout":
		os.Stdout = writer
	case "piped-both":
		os.Stdin, os.Stdout = reader, writer
	case "closed-stdin":
		os.Stdin = closed
	case "closed-stdout":
		os.Stdout = closed
	default:
		t.Fatalf("unknown stream fixture %q", mode)
	}
	t.Cleanup(func() {
		os.Stdin, os.Stdout = stdin, stdout
		for _, file := range []*os.File{master, terminal, reader, writer} {
			if err := file.Close(); err != nil {
				t.Error(err)
			}
		}
	})
}

// Snapshot records every directory, file mode and file byte under root, so a
// refused producer cannot conceal a newly created directory or changed artifact.
func Snapshot(t testing.TB, root string) map[string]string {
	t.Helper()
	entries := make(map[string]string)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		value := info.Mode().String()
		if info.Mode().IsRegular() {
			body, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			value += string(body)
		}
		entries[path] = value
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return entries
}
