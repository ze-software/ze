// Design: docs/architecture/hub-architecture.md -- explicit configuration authority

package resolve

import (
	"errors"
	"os"

	"github.com/ze-software/ze/internal/component/config/storage"
)

// ConfigSource states which input remains authoritative throughout a daemon run.
type ConfigSource uint8

const (
	ConfigSourceUnspecified ConfigSource = iota
	ConfigSourceStored
	ConfigSourceFile
	ConfigSourceStdin
)

// configStore carries source identity on the same owned handle every runtime
// consumer borrows. Embedding retains raw keys, observer and lifetime ownership.
type configStore struct {
	storage.Storage
	path string
	mode ConfigSource
}

// BindConfigSource records the launch decision without opening another store.
// Consumers borrow the result; only the original owner MUST close the handle.
func BindConfigSource(store storage.Storage, path string, mode ConfigSource) storage.Storage {
	if store == nil {
		return nil
	}
	return &configStore{Storage: store, path: path, mode: mode}
}

// SourceMode reports an explicit binding, never guessing authority from a path.
func SourceMode(store storage.Storage) ConfigSource {
	if bound, ok := store.(*configStore); ok {
		return bound.mode
	}
	return ConfigSourceUnspecified
}

// SourcePath is the launch input, or empty when the handle is not source-bound.
func SourcePath(store storage.Storage) string {
	if bound, ok := store.(*configStore); ok {
		return bound.path
	}
	return ""
}

// ReadConfigSource reads the chosen authority, not a fallback on read failure.
func ReadConfigSource(store storage.Storage, path string) ([]byte, error) {
	switch SourceMode(store) {
	case ConfigSourceFile:
		if path != SourcePath(store) {
			return nil, errors.New("configuration path does not match the explicit source")
		}
		return os.ReadFile(path) //nolint:gosec // explicit operator-supplied config
	case ConfigSourceStored:
		return storage.ReadActiveConfig(store, path)
	case ConfigSourceStdin:
		return nil, errors.New("stdin configuration has no reload source")
	default:
		return nil, errors.New("configuration source mode is unspecified")
	}
}

// ReadReloadConfig prefers a staged candidate, preserving read failures rather
// than concealing a corrupt candidate behind the previous active configuration.
func ReadReloadConfig(store storage.Storage, path string) ([]byte, error) {
	if store != nil {
		data, _, present, err := storage.ReadCandidateConfig(store, path)
		if err != nil {
			return nil, err
		}
		if present {
			return data, nil
		}
	}
	return ReadConfigSource(store, path)
}
