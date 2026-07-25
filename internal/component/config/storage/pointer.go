// Design: docs/architecture/zefs-format.md -- transactional config pointers
// Related: storage.go -- Storage abstraction and version operations

package storage

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ze-software/ze/pkg/zefs"
)

// ErrCandidateExists is returned when a new candidate would overwrite one already staged.
var ErrCandidateExists = errors.New("candidate config already staged")

// PointerName identifies a named config version pointer.
type PointerName string

const (
	PointerActive    PointerName = "active"
	PointerCandidate PointerName = "candidate"
	PointerRollback  PointerName = "rollback"
	PointerRecovery  PointerName = "recovery"
)

func (p PointerName) valid() bool {
	switch p {
	case PointerActive, PointerCandidate, PointerRollback, PointerRecovery:
		return true
	default:
		return false
	}
}

// ReadPointer returns the timestamp stored in a named pointer.
func ReadPointer(store Storage, configPath string, pointer PointerName) (string, bool, error) {
	path, err := pointerPath(store, configPath, pointer)
	if err != nil {
		return "", false, err
	}
	if !store.Exists(path) {
		return "", false, nil
	}
	data, err := store.ReadFile(path)
	if err != nil {
		return "", false, fmt.Errorf("read %s pointer: %w", pointer, err)
	}
	stamp := strings.TrimSpace(string(data))
	if _, err := ParseVersionStamp(stamp); err != nil {
		return "", false, fmt.Errorf("read %s pointer: %w", pointer, err)
	}
	return stamp, true, nil
}

// WritePointer stores a timestamp in a named pointer.
func WritePointer(store Storage, configPath string, pointer PointerName, stamp string) (err error) {
	guard, err := store.AcquireLock(configPath)
	if err != nil {
		return err
	}
	defer func() { err = releaseGuard(guard, err) }()
	return writePointerLocked(store, guard, configPath, pointer, stamp)
}

func writePointerLocked(store Storage, guard WriteGuard, configPath string, pointer PointerName, stamp string) error {
	path, err := pointerPath(store, configPath, pointer)
	if err != nil {
		return err
	}
	if _, err := ParseVersionStamp(stamp); err != nil {
		return fmt.Errorf("write %s pointer: %w", pointer, err)
	}
	return guard.WriteFile(path, []byte(stamp+"\n"), 0o600)
}

// ClearPointer removes a named pointer if it exists.
func ClearPointer(store Storage, configPath string, pointer PointerName) (err error) {
	guard, err := store.AcquireLock(configPath)
	if err != nil {
		return err
	}
	defer func() { err = releaseGuard(guard, err) }()
	return clearPointerLocked(store, guard, configPath, pointer)
}

// WriteCandidateVersion writes a timestamped candidate version and points candidate at it.
func WriteCandidateVersion(store Storage, configPath string, data []byte, stamp time.Time) (stampStr string, err error) {
	guard, err := store.AcquireLock(configPath)
	if err != nil {
		return "", err
	}
	defer func() { err = releaseGuard(guard, err) }()
	return WriteCandidateVersionWithGuard(store, guard, configPath, data, stamp)
}

// WriteCandidateVersionWithGuard writes a candidate while the caller already holds the config lock.
func WriteCandidateVersionWithGuard(store Storage, guard WriteGuard, configPath string, data []byte, stamp time.Time) (stampStr string, err error) {
	if _, ok, err := readPointerLocked(store, guard, configPath, PointerCandidate); err != nil || ok {
		if err != nil {
			return "", err
		}
		return "", ErrCandidateExists
	}

	stampStr = FormatVersionStamp(stamp)
	if err := guard.WriteVersion(configPath, data, stamp); err != nil {
		return "", fmt.Errorf("write candidate version: %w", err)
	}
	if err := writePointerLocked(store, guard, configPath, PointerCandidate, stampStr); err != nil {
		removeErr := removeVersionLocked(store, guard, configPath, stampStr)
		return "", errors.Join(fmt.Errorf("write candidate pointer: %w", err), removeErr)
	}
	return stampStr, nil
}

// EnsureActiveVersion writes the current config as a version and points active at it
// when a legacy config has no active pointer yet.
func EnsureActiveVersion(store Storage, configPath string, data []byte, stamp time.Time) (stampStr string, wrote bool, err error) {
	guard, err := store.AcquireLock(configPath)
	if err != nil {
		return "", false, err
	}
	defer func() { err = releaseGuard(guard, err) }()

	existing, ok, err := readPointerLocked(store, guard, configPath, PointerActive)
	if err != nil || ok {
		return existing, false, err
	}

	stampStr = FormatVersionStamp(stamp)
	if err := guard.WriteVersion(configPath, data, stamp); err != nil {
		return "", false, fmt.Errorf("write active version: %w", err)
	}
	if err := writePointerLocked(store, guard, configPath, PointerActive, stampStr); err != nil {
		removeErr := removeVersionLocked(store, guard, configPath, stampStr)
		return "", false, errors.Join(fmt.Errorf("write active pointer: %w", err), removeErr)
	}
	return stampStr, true, nil
}

// ReadVersion reads a timestamped config version.
func ReadVersion(store Storage, configPath, stamp string) ([]byte, error) {
	path, err := versionPath(store, configPath, stamp)
	if err != nil {
		return nil, err
	}
	data, err := store.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config version %s: %w", stamp, err)
	}
	return data, nil
}

// RemoveVersion removes a timestamped config version.
func RemoveVersion(store Storage, configPath, stamp string) (err error) {
	guard, err := store.AcquireLock(configPath)
	if err != nil {
		return err
	}
	defer func() { err = releaseGuard(guard, err) }()
	return removeVersionLocked(store, guard, configPath, stamp)
}

// ClearCandidate removes the transient candidate pointer and candidate version.
func ClearCandidate(store Storage, configPath string) (err error) {
	guard, err := store.AcquireLock(configPath)
	if err != nil {
		return err
	}
	defer func() { err = releaseGuard(guard, err) }()

	stamp, ok, err := readPointerLocked(store, guard, configPath, PointerCandidate)
	if err != nil || !ok {
		return err
	}
	if err := clearPointerLocked(store, guard, configPath, PointerCandidate); err != nil {
		return err
	}
	return removeVersionLocked(store, guard, configPath, stamp)
}

// PromoteCandidate promotes candidate to active and stores the previous active in rollback.
func PromoteCandidate(store Storage, configPath string) (err error) {
	guard, err := store.AcquireLock(configPath)
	if err != nil {
		return err
	}
	defer func() { err = releaseGuard(guard, err) }()

	candidate, ok, err := readPointerLocked(store, guard, configPath, PointerCandidate)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("promote candidate: candidate pointer not set")
	}
	candidateData, err := readVersionLocked(store, guard, configPath, candidate)
	if err != nil {
		return err
	}

	active, hasActive, err := readPointerLocked(store, guard, configPath, PointerActive)
	if err != nil {
		return err
	}
	if !hasActive {
		legacyData, readErr := guard.ReadFile(configPath)
		if readErr == nil {
			activeTime := time.Now()
			if FormatVersionStamp(activeTime) == candidate {
				activeTime = activeTime.Add(-time.Millisecond)
			}
			active = FormatVersionStamp(activeTime)
			if writeErr := guard.WriteVersion(configPath, legacyData, activeTime); writeErr != nil {
				return fmt.Errorf("promote candidate: write rollback version: %w", writeErr)
			}
			hasActive = true
		} else if !isNotExist(readErr) {
			return fmt.Errorf("promote candidate: read legacy active: %w", readErr)
		}
	}

	if hasActive {
		if err := writePointerLocked(store, guard, configPath, PointerRollback, active); err != nil {
			return err
		}
	} else if err := clearPointerLocked(store, guard, configPath, PointerRollback); err != nil {
		return err
	}
	if err := writePointerLocked(store, guard, configPath, PointerActive, candidate); err != nil {
		return err
	}
	if err := clearPointerLocked(store, guard, configPath, PointerCandidate); err != nil {
		return err
	}
	if err := guard.WriteFile(configPath, candidateData, 0o600); err != nil {
		slog.Warn("storage: mirror active config failed", "path", configPath, "error", err)
	}
	return nil
}

func readPointerLocked(store Storage, guard WriteGuard, configPath string, pointer PointerName) (string, bool, error) {
	path, err := pointerPath(store, configPath, pointer)
	if err != nil {
		return "", false, err
	}
	data, err := guard.ReadFile(path)
	if err != nil {
		if isNotExist(err) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("read %s pointer: %w", pointer, err)
	}
	stamp := strings.TrimSpace(string(data))
	if _, err := ParseVersionStamp(stamp); err != nil {
		return "", false, fmt.Errorf("read %s pointer: %w", pointer, err)
	}
	return stamp, true, nil
}

func clearPointerLocked(store Storage, guard WriteGuard, configPath string, pointer PointerName) error {
	path, err := pointerPath(store, configPath, pointer)
	if err != nil {
		return err
	}
	if err := guard.Remove(path); err != nil {
		if isNotExist(err) {
			return nil
		}
		return fmt.Errorf("clear %s pointer: %w", pointer, err)
	}
	return nil
}

func readVersionLocked(store Storage, guard WriteGuard, configPath, stamp string) ([]byte, error) {
	path, err := versionPath(store, configPath, stamp)
	if err != nil {
		return nil, err
	}
	data, err := guard.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config version %s: %w", stamp, err)
	}
	return data, nil
}

func removeVersionLocked(store Storage, guard WriteGuard, configPath, stamp string) error {
	path, err := versionPath(store, configPath, stamp)
	if err != nil {
		return err
	}
	if err := guard.Remove(path); err != nil {
		if isNotExist(err) {
			return nil
		}
		return fmt.Errorf("remove config version %s: %w", stamp, err)
	}
	return nil
}

func releaseGuard(guard WriteGuard, err error) error {
	if releaseErr := guard.Release(); releaseErr != nil {
		return errors.Join(err, fmt.Errorf("release storage lock: %w", releaseErr))
	}
	return err
}

// ReadActiveConfig reads the config referenced by the active pointer, falling back to legacy active storage.
func ReadActiveConfig(store Storage, configPath string) ([]byte, error) {
	stamp, ok, err := ReadPointer(store, configPath, PointerActive)
	if err != nil {
		return nil, err
	}
	if ok {
		return ReadVersion(store, configPath, stamp)
	}
	return store.ReadFile(configPath)
}

// ReadCandidateConfig reads the config version referenced by candidate.
func ReadCandidateConfig(store Storage, configPath string) ([]byte, string, bool, error) {
	stamp, ok, err := ReadPointer(store, configPath, PointerCandidate)
	if err != nil || !ok {
		return nil, "", ok, err
	}
	data, err := ReadVersion(store, configPath, stamp)
	if err != nil {
		return nil, "", false, err
	}
	return data, stamp, true, nil
}

func pointerPath(store Storage, configPath string, pointer PointerName) (string, error) {
	if !pointer.valid() {
		return "", fmt.Errorf("unknown config pointer %q", pointer)
	}
	if IsBlobStorage(store) {
		return pointerKey(pointer), nil
	}
	return filepath.Join(filepath.Dir(configPath), "meta", "config", string(pointer)), nil
}

func pointerKey(pointer PointerName) string {
	switch pointer {
	case PointerActive:
		return zefs.KeyConfigActive.Key()
	case PointerCandidate:
		return zefs.KeyConfigCandidate.Key()
	case PointerRollback:
		return zefs.KeyConfigRollback.Key()
	case PointerRecovery:
		return zefs.KeyConfigRecovery.Key()
	default:
		return ""
	}
}

func versionPath(store Storage, configPath, stamp string) (string, error) {
	if _, err := ParseVersionStamp(stamp); err != nil {
		return "", fmt.Errorf("config version path: %w", err)
	}
	if IsBlobStorage(store) {
		return zefs.KeyFileVersion.Key(stamp, resolvePathToKey(configPath, "")), nil
	}
	return versionPathFS(configPath, stamp), nil
}

func versionPathFS(name, stamp string) string {
	dir := filepath.Dir(name)
	base := filepath.Base(name)
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	return filepath.Join(dir, "rollback", stem+"-"+stamp+".conf")
}

func isNotExist(err error) bool {
	return errors.Is(err, fs.ErrNotExist) || errors.Is(err, os.ErrNotExist)
}
