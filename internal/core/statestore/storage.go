// Design: docs/architecture/storage-backends.md -- the store contract every
// runtime consumer borrows. The config storage component implements it and
// aliases these names, so the contract is declared once, in the leaf tier.
package statestore

import (
	"io/fs"
	"time"
)

// Storage is safe for concurrent use. Callers MUST Close every opened store.
// Unlocked reads return caller-owned bytes; guarded reads expire at Release.
type Storage interface {
	ReadFile(string) ([]byte, error)
	WriteFile(string, []byte, fs.FileMode) error
	Remove(string) error
	Exists(string) bool
	List(string) ([]string, error)
	AcquireLock(string) (WriteGuard, error)
	Stat(string) (FileMeta, error)
	Rename(string, string) error
	Close() error
	WriteVersion(string, []byte, time.Time) error
	ListVersions(string) ([]VersionInfo, error)
	// ReadVersion reads the version of a config name at a stamp: the
	// history entry, then the object it names, with the SHA-256 verified.
	// A missing entry or object keeps fs.ErrNotExist in the error chain.
	ReadVersion(name, stamp string) ([]byte, error)
	SetWriteObserver(func(string))
	ReadKey(string) ([]byte, error)
	WriteKey(string, []byte) error
	RemoveKey(string) error
	ListKeys(string) ([]string, error)
	// CheckName refuses a config name outside the store; every config-name
	// path, including the pointer and version helpers, MUST reach it.
	CheckName(string) error
}

// FileMeta is process-local modification metadata, shared by both encodings.
type FileMeta struct {
	ModTime    time.Time
	ModifiedBy string
}

// VersionInfo describes a historical configuration. Path is the history
// entry key file/<stamp>/<name>, whose stored value is `sha256:<hex>`, never
// the config bytes: read the bytes with ReadVersion.
type VersionInfo struct {
	Stamp string
	Date  time.Time
	Path  string
}

// WriteGuard serializes a group of operations. Callers MUST Release it and MUST
// use the guard, not Storage, until Release. Guarded bytes MUST NOT outlive it.
type WriteGuard interface {
	ReadFile(string) ([]byte, error)
	WriteFile(string, []byte, fs.FileMode) error
	Remove(string) error
	Has(string) bool
	List(string) ([]string, error)
	Release() error
	SetModifier(string)
	WriteVersion(string, []byte, time.Time) error
	// ReadKey and ListKeys are the raw-key pair Storage carries, answered
	// under the held guard. Storage.ReadKey and Storage.ListKeys take the
	// store's lock again, so a caller holding a guard MUST use these instead.
	// ListKeys is literal-prefix, recursive and sorted; List answers only the
	// immediate file children of a resolved name.
	ReadKey(string) ([]byte, error)
	ListKeys(string) ([]string, error)
	// WriteKey and RemoveKey are the raw-key write pair, under the held
	// guard, for the same reason: Storage.WriteKey and Storage.RemoveKey
	// acquire the store again and a caller holding a guard would hang.
	WriteKey(string, []byte) error
	RemoveKey(string) error
}
