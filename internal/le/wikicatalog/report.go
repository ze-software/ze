package wikicatalog

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// StaleExit distinguishes catalog drift from a generation or I/O failure.
const StaleExit = 3

// Report is the structured result of checking or updating one destination.
type Report struct {
	File     string  `json:"file"`
	Commands []Entry `json:"commands"`
	Bytes    int     `json:"bytes"`
	Stale    bool    `json:"stale"`
	Written  bool    `json:"written"`
}

// Text is the default human rendering. Pipe operators render the structured
// fields instead.
func (report Report) Text() string {
	var text textbuf.Buffer
	switch {
	case report.Written:
		text.Str("wrote ").Str(report.File).Str(" (").Int(int64(len(report.Commands))).Str(" commands)")
	case report.Stale:
		text.Str(report.File).Str(" is stale")
	default:
		text.Str("checked ").Int(int64(len(report.Commands))).Str(" commands, ").Str(report.File).Str(" up to date")
	}
	return text.Byte('\n').String()
}

// Check compares destination byte-for-byte with a catalog collected from the
// live product registries. A missing destination is stale, not an I/O failure.
func Check(destination string) (Report, error) {
	entries := Collect()
	markdown, err := Render(entries)
	if err != nil {
		return Report{}, err
	}
	return checkCatalog(destination, entries, markdown)
}

// Update renders the live product catalog and atomically replaces destination.
// The parent directory must already exist; the caller chooses the destination,
// and this package never assumes a checkout or wiki layout.
func Update(destination string) (Report, error) {
	entries := Collect()
	markdown, err := Render(entries)
	if err != nil {
		return Report{}, err
	}
	return updateCatalog(destination, entries, markdown, atomicWrite)
}

func checkCatalog(destination string, entries []Entry, markdown []byte) (Report, error) {
	report := Report{File: destination, Commands: entries, Bytes: len(markdown)}
	current, err := os.ReadFile(destination) //nolint:gosec // the caller explicitly selected this destination
	if errors.Is(err, os.ErrNotExist) {
		report.Stale = true
		return report, nil
	}
	if err != nil {
		return Report{}, fmt.Errorf("read wiki catalog %q: %w", destination, err)
	}
	report.Stale = !bytes.Equal(current, markdown)
	return report, nil
}

type catalogWriter func(string, []byte) error

func updateCatalog(
	destination string,
	entries []Entry,
	markdown []byte,
	write catalogWriter,
) (Report, error) {
	report := Report{File: destination, Commands: entries, Bytes: len(markdown)}
	if err := write(destination, markdown); err != nil {
		return report, fmt.Errorf("write wiki catalog %q: %w", destination, err)
	}
	report.Written = true
	return report, nil
}

// atomicWrite publishes complete bytes or leaves the previous destination in
// place. The temporary file lives beside the destination, so Rename is one
// filesystem operation. Existing permissions survive replacement.
func atomicWrite(destination string, content []byte) error {
	directory := filepath.Dir(destination)
	base := filepath.Base(destination)
	temporary, err := os.CreateTemp(directory, "."+base+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temporary file: %w", err)
	}
	temporaryName := temporary.Name()
	published := false
	defer func() {
		if !published {
			_ = os.Remove(temporaryName)
		}
	}()

	mode := os.FileMode(0o644)
	info, statErr := os.Stat(destination)
	switch {
	case statErr == nil:
		mode = info.Mode().Perm()
	case !errors.Is(statErr, os.ErrNotExist):
		_ = temporary.Close()
		return fmt.Errorf("inspect destination: %w", statErr)
	}
	if err := temporary.Chmod(mode); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("set temporary file mode: %w", err)
	}
	written, err := temporary.Write(content)
	if err == nil && written != len(content) {
		err = io.ErrShortWrite
	}
	if err != nil {
		_ = temporary.Close()
		return fmt.Errorf("write temporary file: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("sync temporary file: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close temporary file: %w", err)
	}
	if err := os.Rename(temporaryName, destination); err != nil {
		return fmt.Errorf("replace destination: %w", err)
	}
	published = true
	return nil
}

var errDestinationRequired = errors.New("argument keyword \"file\" is required and must name a destination")

func validateDestination(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", errDestinationRequired
	}
	return path, nil
}
