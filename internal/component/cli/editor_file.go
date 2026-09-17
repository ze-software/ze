// Design: docs/guide/config-editor.md — explicit loose-file editing
package cli

import (
	"fmt"
	"os"

	"github.com/ze-software/ze/internal/core/cliio"
)

// These operations belong to the editor's explicit file mode, not to Storage:
// loose files have no key space, sessions, or persistence backend of their own.
func (e *Editor) readFile(path string) ([]byte, error) {
	if e.looseFile {
		return cliio.ReadFile(path)
	}
	if e.store == nil {
		return nil, fmt.Errorf("config %s has no persistent source", path)
	}
	return e.store.ReadFile(path)
}

func (e *Editor) writeFile(path string, data []byte) error {
	if e.looseFile {
		return cliio.WriteFile(path, data, 0o600)
	}
	if e.store == nil {
		return fmt.Errorf("config %s has no persistent destination", path)
	}
	return e.store.WriteFile(path, data, 0o600)
}

func (e *Editor) fileExists(path string) bool {
	if e.looseFile {
		_, err := os.Stat(path)
		return err == nil
	}
	return e.store != nil && e.store.Exists(path)
}

func (e *Editor) removeFile(path string) error {
	if e.looseFile {
		return os.Remove(path)
	}
	if e.store == nil {
		return nil // Content-only editors never wrote a sidecar.
	}
	return e.store.Remove(path)
}
