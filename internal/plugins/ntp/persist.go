// Design: docs/features/interfaces.md -- NTP time persistence

package ntp

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// saveTime writes the current time to the given path as RFC3339 text.
// Written atomically (tmp + rename) to avoid corrupt reads on crash.
func saveTime(path string, t time.Time) error {
	buf, err := t.MarshalText()
	if err != nil {
		return fmt.Errorf("ntp persist: marshal: %w", err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("ntp persist: mkdir %s: %w", dir, err)
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, buf, 0o644); err != nil { //nolint:gosec // time file is not sensitive
		return fmt.Errorf("ntp persist: write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("ntp persist: rename: %w", err)
	}
	return nil
}

// loadTime reads a previously saved time from the given path.
// Returns an error if the file does not exist or is corrupt.
func loadTime(path string) (time.Time, error) {
	buf, err := os.ReadFile(path) //nolint:gosec // path is from config, not user input
	if err != nil {
		return time.Time{}, fmt.Errorf("ntp persist: read %s: %w", path, err)
	}
	var t time.Time
	if err := t.UnmarshalText(buf); err != nil {
		return time.Time{}, fmt.Errorf("ntp persist: parse %s: %w", path, err)
	}
	// Reject absurd saved times.
	if t.Year() < 2020 || t.Year() > 2100 {
		return time.Time{}, fmt.Errorf("ntp persist: saved time out of range: %v", t)
	}
	return t, nil
}
