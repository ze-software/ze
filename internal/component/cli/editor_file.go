// Design: docs/guide/config-editor.md — explicit loose-file editing
package cli

import (
	"fmt"
	"os"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/cliio"
	"github.com/ze-software/ze/internal/core/opererr"
)

// readFile keeps the selected source's authority: explicit loose files use the
// filesystem, committed stored configs use their active pointer, and editor
// records use raw files.
func (e *Editor) readFile(path string) ([]byte, error) {
	if e.looseFile {
		return cliio.ReadFile(path)
	}
	if e.store == nil {
		return nil, fmt.Errorf("config %s has no persistent source", path)
	}
	if path == e.originalPath {
		return storage.ReadActiveConfig(e.store, path)
	}
	return e.store.ReadFile(path)
}

// checkSaveAuthority refuses mirror-only publication of a versioned source.
// Its owner MUST install SetCommitWriter to validate and publish that source;
// the editor cannot infer live reload or offline promotion from a store handle.
func (e *Editor) checkSaveAuthority() error {
	if e.commitWriter != nil {
		return nil
	}
	if e.looseFile {
		return nil
	}
	if e.store == nil {
		return nil // Content-only editors retain their missing-destination error.
	}
	pointed, err := storage.HasActivePointer(e.store, e.originalPath)
	if err != nil {
		return opererr.Wrap(err,
			fmt.Sprintf("Configuration %s cannot be saved because its stored active version could not be checked.", e.originalPath),
			"Reopen the configuration through its owning daemon or with --backup before committing changes.",
			"ze config edit --help")
	}
	if pointed {
		return opererr.New(
			fmt.Sprintf("Configuration %s cannot be saved from this editor.", e.originalPath),
			"Its stored active version needs the owning daemon or backup editor to publish configuration and history together.",
			"ze config edit --help")
	}
	return nil
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
