// Design: docs/architecture/hub-architecture.md -- explicit configuration authority

package storage

import (
	"errors"
	"os"
	"sync/atomic"
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
	Storage
	path string
	mode ConfigSource
	// boot is the pending boot record (boot_record.go), nil when none is armed.
	boot atomic.Pointer[BootRecord]
}

// AcquireLock waits for the boot record before it hands out the lock of the
// bound configuration name: every config write takes that lock, so none can
// land before the boot configuration is durable, or race the record's own
// candidate. Other keys (runtime state, the SSH host key) never wait.
func (c *configStore) AcquireLock(path string) (WriteGuard, error) {
	if record := c.boot.Load(); record != nil && record.covers(path) {
		if err := record.await(); err != nil {
			return nil, err
		}
	}
	return c.Storage.AcquireLock(path)
}

// BindConfigSource records the launch decision without opening another store.
// Consumers borrow the result; only the original owner MUST close the handle.
func BindConfigSource(store Storage, path string, mode ConfigSource) Storage {
	if store == nil {
		return nil
	}
	return &configStore{Storage: store, path: path, mode: mode}
}

// SourceMode reports an explicit binding, never guessing authority from a path.
func SourceMode(store Storage) ConfigSource {
	if bound, ok := store.(*configStore); ok {
		return bound.mode
	}
	return ConfigSourceUnspecified
}

// SourcePath is the launch input, or empty when the handle is not source-bound.
func SourcePath(store Storage) string {
	if bound, ok := store.(*configStore); ok {
		return bound.path
	}
	return ""
}

// ReadConfigSource reads the chosen authority, not a fallback on read failure.
func ReadConfigSource(store Storage, path string) ([]byte, error) {
	switch SourceMode(store) {
	case ConfigSourceFile:
		if path != SourcePath(store) {
			return nil, errors.New("configuration path does not match the explicit source")
		}
		return os.ReadFile(path) //nolint:gosec // explicit operator-supplied config
	case ConfigSourceStored:
		return ReadActiveConfig(store, path)
	case ConfigSourceStdin:
		return nil, errors.New("stdin configuration has no reload source")
	case ConfigSourceUnspecified:
		return nil, errors.New("configuration source mode is unspecified")
	default:
		panic("BUG: unknown bound configuration source mode")
	}
}

// ReadReloadConfig prefers a staged candidate, preserving read failures rather
// than concealing a corrupt candidate behind the previous active configuration.
func ReadReloadConfig(store Storage, path string) ([]byte, error) {
	if store != nil {
		data, _, present, err := ReadCandidateConfig(store, path)
		if err != nil {
			return nil, err
		}
		if present {
			return data, nil
		}
	}
	return ReadConfigSource(store, path)
}
