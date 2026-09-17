// Design: docs/architecture/storage-backends.md -- shared configuration key space.
// Package storage owns the live tree and explicit blob artifacts.
package storage

import (
	"fmt"
	"io/fs"
	"strconv"
	"strings"
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
	SetWriteObserver(func(string))
	ReadKey(string) ([]byte, error)
	WriteKey(string, []byte) error
	RemoveKey(string) error
	ListKeys(string) ([]string, error)
}

// FileMeta is process-local modification metadata, shared by both encodings.
type FileMeta struct {
	ModTime    time.Time
	ModifiedBy string
}

// VersionInfo describes a historical configuration.
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
}

// FormatVersionStamp formats a time as YYYYMMDD-HHMMSS.mmm.
func FormatVersionStamp(t time.Time) string {
	return fmt.Sprintf("%s.%03d", t.Format("20060102-150405"), t.Nanosecond()/1e6)
}

// ParseVersionStamp parses the canonical version timestamp.
func ParseVersionStamp(s string) (time.Time, error) {
	parts := strings.SplitN(s, ".", 2)
	if len(parts) != 2 {
		return time.Time{}, fmt.Errorf("invalid version stamp: %s", s)
	}
	if len(parts[1]) != 3 {
		return time.Time{}, fmt.Errorf("invalid milliseconds: %s", s)
	}
	t, err := time.ParseInLocation("20060102-150405", parts[0], time.Local)
	if err != nil {
		return time.Time{}, err
	}
	ms, err := strconv.Atoi(parts[1])
	if err != nil {
		return time.Time{}, err
	}
	if ms < 0 {
		return time.Time{}, fmt.Errorf("milliseconds out of range: %d", ms)
	}
	if ms > 999 {
		return time.Time{}, fmt.Errorf("milliseconds out of range: %d", ms)
	}
	return t.Add(time.Duration(ms) * time.Millisecond), nil
}
