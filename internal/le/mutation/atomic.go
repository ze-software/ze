package mutation

import (
	"fmt"
	"os"
	"path/filepath"
)

// writeAtomic publishes complete bytes with one rename. A reader therefore
// sees the old report or the new report, never the half-written state that made
// the history consumer reject its committed input.
func writeAtomic(path string, content []byte) (err error) {
	mode := os.FileMode(0o644)
	if existing, statErr := os.Stat(path); statErr == nil {
		mode = existing.Mode().Perm()
	} else if !os.IsNotExist(statErr) {
		return fmt.Errorf("inspect existing file %s: %w", path, statErr)
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".mutation-*")
	if err != nil {
		return fmt.Errorf("create temporary file for %s: %w", path, err)
	}
	temporaryPath := temporary.Name()
	defer func() {
		if temporary != nil {
			_ = temporary.Close()
		}
		if err != nil {
			_ = os.Remove(temporaryPath)
		}
	}()

	if err = temporary.Chmod(mode); err != nil {
		return fmt.Errorf("set mode on temporary file for %s: %w", path, err)
	}
	if _, err = temporary.Write(content); err != nil {
		return fmt.Errorf("write temporary file for %s: %w", path, err)
	}
	if err = temporary.Sync(); err != nil {
		return fmt.Errorf("sync temporary file for %s: %w", path, err)
	}
	if err = temporary.Close(); err != nil {
		temporary = nil
		return fmt.Errorf("close temporary file for %s: %w", path, err)
	}
	temporary = nil
	if err = os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("publish %s: %w", path, err)
	}
	return nil
}
