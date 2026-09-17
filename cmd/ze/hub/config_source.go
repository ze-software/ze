// Design: docs/architecture/hub-architecture.md -- explicit-file commit authority

package hub

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/ze-software/ze/internal/component/config/storage"
	internalresolve "github.com/ze-software/ze/internal/core/resolve"
	"github.com/ze-software/ze/pkg/zefs"
)

// fileCommit is durable before either authority is changed. A restart can finish
// this exact publication, but cannot overwrite a third value an operator wrote.
type fileCommit struct {
	Path     string `json:"path"`
	Expected []byte `json:"expected"`
	Content  []byte `json:"content"`
	Stamp    string `json:"stamp"`
}

func initializeConfigSource(store storage.Storage, path string, content []byte) error {
	if internalresolve.SourceMode(store) != internalresolve.ConfigSourceFile {
		_, _, err := storage.EnsureActiveVersion(store, path, content, time.Now())
		return err
	}
	active, err := storage.ReadActiveConfig(store, path)
	if err == nil {
		if bytes.Equal(active, content) {
			return nil
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if _, err := storage.WriteCandidateVersion(store, path, content, time.Now()); err != nil {
		return err
	}
	return storage.PromoteCandidate(store, path)
}

func promoteConfigCandidate(store storage.Storage, path string) error {
	if internalresolve.SourceMode(store) != internalresolve.ConfigSourceFile {
		return storage.PromoteCandidate(store, path)
	}
	content, stamp, present, err := storage.ReadCandidateConfig(store, path)
	if err != nil {
		return err
	}
	if !present {
		return errors.New("explicit config commit has no candidate")
	}
	expected, err := storage.ReadActiveConfig(store, path)
	if err != nil {
		return err
	}
	disk, err := os.ReadFile(path) //nolint:gosec // explicit operator config path
	if err != nil {
		return err
	}
	if !bytes.Equal(disk, expected) {
		if !bytes.Equal(disk, content) {
			return fmt.Errorf("explicit config changed externally: %s; reload the file before committing", path)
		}
		// A signal reload adopts the file the operator just edited.
		expected = disk
	}
	intent := fileCommit{Path: path, Expected: expected, Content: content, Stamp: stamp}
	encoded, err := json.Marshal(intent)
	if err != nil {
		return err
	}
	key := zefs.KeyConfigFileCommit.Key(filepath.Base(path))
	if err := store.WriteKey(key, encoded); err != nil {
		return fmt.Errorf("persist explicit config commit intent: %w", err)
	}
	return recoverFileCommit(store, path)
}

// recoverFileCommit MUST run before stale-candidate cleanup and before the
// explicit source is read. An intent surviving pointer publication is harmless:
// the active bytes prove completion, then only the intent is retired.
func recoverFileCommit(store storage.Storage, path string) error {
	if store == nil {
		return nil
	}
	if path == "" || path == "-" {
		return nil
	}
	key := zefs.KeyConfigFileCommit.Key(filepath.Base(path))
	encoded, err := store.ReadKey(key)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var intent fileCommit
	if err := json.Unmarshal(encoded, &intent); err != nil {
		return fmt.Errorf("decode explicit config commit intent: %w", err)
	}
	if !filepath.IsAbs(intent.Path) || filepath.Base(intent.Path) != filepath.Base(path) || intent.Stamp == "" {
		return errors.New("invalid explicit config commit intent")
	}
	if internalresolve.SourceMode(store) == internalresolve.ConfigSourceFile {
		if intent.Path != path {
			return errors.New("explicit config commit intent names another source")
		}
	}
	// Ask the store's path mapper to enforce the selected folder boundary.
	// A missing active entry is valid here; an out-of-folder path is not.
	if _, err := store.ReadFile(intent.Path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("invalid explicit config commit source: %w", err)
	}
	candidate, stamp, present, err := storage.ReadCandidateConfig(store, path)
	if err != nil {
		return err
	}
	if present {
		if stamp != intent.Stamp || !bytes.Equal(candidate, intent.Content) {
			return errors.New("explicit config commit intent does not match candidate")
		}
	} else {
		active, readErr := storage.ReadActiveConfig(store, path)
		if readErr != nil {
			return readErr
		}
		if !bytes.Equal(active, intent.Content) {
			return errors.New("explicit config commit intent has neither candidate nor matching active version")
		}
	}
	disk, err := os.ReadFile(intent.Path) //nolint:gosec // recorded explicit source
	if err != nil {
		return err
	}
	if !bytes.Equal(disk, intent.Content) {
		if !bytes.Equal(disk, intent.Expected) {
			return fmt.Errorf("explicit config changed externally: %s; pending commit retained", intent.Path)
		}
		if err := storage.WriteConfigFile(intent.Path, intent.Content); err != nil {
			return fmt.Errorf("publish explicit config file: %w", err)
		}
	}
	if present {
		if err := storage.PromoteCandidate(store, path); err != nil {
			return fmt.Errorf("publish explicit config history (intent retained): %w", err)
		}
	}
	if err := store.RemoveKey(key); err != nil {
		return fmt.Errorf("retire explicit config commit intent: %w", err)
	}
	return nil
}

func commitRuntimeConfig(store storage.Storage, sourcePath, path string, expected, content []byte, reload func() error) error {
	if store == nil {
		return errors.New("configuration commits require a persistent store")
	}
	if reload == nil {
		return errors.New("configuration commit has no daemon reload handler")
	}
	if err := recoverFileCommit(store, sourcePath); err != nil {
		return err
	}
	if path != sourcePath {
		return errors.New("configuration commit names a different daemon source")
	}
	current, err := internalresolve.ReadConfigSource(store, path)
	if err != nil {
		return err
	}
	if !bytes.Equal(current, expected) {
		return errors.New("configuration changed since this edit began; reload before committing")
	}
	if _, err := storage.WriteCandidateVersion(store, path, content, time.Now()); err != nil {
		return err
	}
	return reload()
}

func publishRecoveredConfig(store storage.Storage, path string, expected, content []byte) error {
	current, err := internalresolve.ReadConfigSource(store, path)
	if err != nil {
		return err
	}
	if !bytes.Equal(current, expected) {
		return errors.New("configuration changed externally during schema recovery")
	}
	if err := initializeConfigSource(store, path, expected); err != nil {
		return err
	}
	if _, err := storage.WriteCandidateVersion(store, path, content, time.Now()); err != nil {
		return err
	}
	return promoteConfigCandidate(store, path)
}

// Once intent exists, its candidate is recovery evidence, not stale editing
// state. Ordinary reload rejection may discard only an uncommitted candidate.
func clearUncommittedCandidate(store storage.Storage, path string) error {
	_, err := store.ReadKey(zefs.KeyConfigFileCommit.Key(filepath.Base(path)))
	if err == nil {
		return nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return storage.ClearCandidate(store, path)
}

func warnStorelessFeatures() {
	for _, feature := range []string{
		"configuration editing, drafts and history", "runtime state persistence",
		"persistent machine identity", "graceful-restart marker persistence",
		"stored SSH credentials and host keys", "managed configuration client",
		"managed configuration server", "web configuration editor",
		"persistent looking-glass certificates",
	} {
		fmt.Fprintf(os.Stderr, "warning: %s unavailable: stdin has no persistent store (run ze init first)\n", feature)
	}
}
