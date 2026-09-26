// Design: docs/architecture/zefs-format.md -- transactional config pointers
// Related: storage.go -- Storage abstraction and version operations

package storage

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/pkg/zefs"
)

// ErrCandidateExists is returned when a new candidate would overwrite one already staged.
var ErrCandidateExists = errors.New("candidate config already staged")

// ErrActiveUnresolved marks a ReadActiveConfig failure where the active
// pointer EXISTS and names a version that does not resolve. It tells a
// repaired store apart from a store that never had an active version: both
// can carry fs.ErrNotExist, and only the first is a rebuild worth logging.
var ErrActiveUnresolved = errors.New("active config version does not resolve")

// pointerName identifies a named config version pointer.
type pointerName string

const (
	pointerActive    pointerName = "active"
	pointerCandidate pointerName = "candidate"
	pointerRollback  pointerName = "rollback"
	pointerRecovery  pointerName = "recovery"
)

func (p pointerName) valid() bool {
	switch p {
	case pointerActive, pointerCandidate, pointerRollback, pointerRecovery:
		return true
	default:
		return false
	}
}

// readPointer returns the timestamp stored in a named pointer.
func readPointer(store Storage, configPath string, pointer pointerName) (string, bool, error) {
	path, err := pointerPath(store, configPath, pointer)
	if err != nil {
		return "", false, err
	}
	data, err := store.ReadFile(path)
	if err != nil {
		if isNotExist(err) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("read %s pointer: %w", pointer, err)
	}
	stamp := strings.TrimSpace(string(data))
	if _, err := parseVersionStamp(stamp); err != nil {
		return "", false, fmt.Errorf("read %s pointer: %w", pointer, err)
	}
	return stamp, true, nil
}

func writePointerLocked(store Storage, guard WriteGuard, configPath string, pointer pointerName, stamp string) error {
	path, err := pointerPath(store, configPath, pointer)
	if err != nil {
		return err
	}
	if _, err := parseVersionStamp(stamp); err != nil {
		return fmt.Errorf("write %s pointer: %w", pointer, err)
	}
	return guard.WriteFile(path, []byte(stamp+"\n"), 0o600)
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
	if _, ok, err := readPointerLocked(store, guard, configPath, pointerCandidate); err != nil || ok {
		if err != nil {
			return "", err
		}
		return "", ErrCandidateExists
	}

	// Millisecond stamps can collide during startup or rapid commits. The
	// exclusive guard keeps the finite occupied set fixed while this advances.
	for {
		stampStr = FormatVersionStamp(stamp)
		path, pathErr := versionPath(store, configPath, stampStr)
		if pathErr != nil {
			return "", pathErr
		}
		_, readErr := guard.ReadFile(path)
		if isNotExist(readErr) {
			break
		}
		if readErr != nil {
			return "", readErr
		}
		stamp = stamp.Add(time.Millisecond)
	}
	if err := guard.WriteVersion(configPath, data, stamp); err != nil {
		return "", fmt.Errorf("write candidate version: %w", err)
	}
	if err := writePointerLocked(store, guard, configPath, pointerCandidate, stampStr); err != nil {
		_, removeErr := removeVersionLocked(store, guard, configPath, stampStr)
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

	existing, ok, err := readPointerLocked(store, guard, configPath, pointerActive)
	if err != nil || ok {
		return existing, false, err
	}

	stampStr = FormatVersionStamp(stamp)
	if err := guard.WriteVersion(configPath, data, stamp); err != nil {
		return "", false, fmt.Errorf("write active version: %w", err)
	}
	if err := writePointerLocked(store, guard, configPath, pointerActive, stampStr); err != nil {
		_, removeErr := removeVersionLocked(store, guard, configPath, stampStr)
		return "", false, errors.Join(fmt.Errorf("write active pointer: %w", err), removeErr)
	}
	return stampStr, true, nil
}

// readVersion reads a timestamped config version through its object.
func readVersion(store Storage, configPath, stamp string) ([]byte, error) {
	data, err := store.ReadVersion(configPath, stamp)
	if err != nil {
		return nil, fmt.Errorf("read config version %s: %w", stamp, err)
	}
	return data, nil
}

// ClearCandidate removes the transient pointer and an otherwise unreferenced
// version, and that version's object when no remaining entry names it. A
// candidate whose entry is already absent (a repaired store) clears cleanly.
func ClearCandidate(store Storage, configPath string) (err error) {
	guard, err := store.AcquireLock(configPath)
	if err != nil {
		return err
	}
	defer func() { err = releaseGuard(guard, err) }()

	stamp, ok, err := readPointerLocked(store, guard, configPath, pointerCandidate)
	if err != nil || !ok {
		return err
	}
	if err := clearPointerLocked(store, guard, configPath, pointerCandidate); err != nil {
		return err
	}
	_, err = removeVersionLocked(store, guard, configPath, stamp)
	return err
}

// PromoteCandidate promotes candidate to active and stores the previous active in rollback.
func PromoteCandidate(store Storage, configPath string) (err error) {
	guard, err := store.AcquireLock(configPath)
	if err != nil {
		return err
	}
	defer func() { err = releaseGuard(guard, err) }()
	return promoteCandidateLocked(store, guard, configPath)
}

// promoteCandidateLocked is PromoteCandidate for a caller that already holds
// the config lock, so a restore can write and promote under one guard.
func promoteCandidateLocked(store Storage, guard WriteGuard, configPath string) error {
	candidate, ok, err := readPointerLocked(store, guard, configPath, pointerCandidate)
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

	active, hasActive, err := readPointerLocked(store, guard, configPath, pointerActive)
	if err != nil {
		return err
	}
	// A persisted active pointer is the commit point. Resuming after it MUST
	// preserve the previous rollback rather than replace it with candidate.
	if hasActive {
		if active == candidate {
			if err := guard.WriteFile(configPath, candidateData, 0o600); err != nil {
				slogutil.Logger("storage").Warn("mirror active config failed", "path", configPath, "error", err)
			}
			return clearPointerLocked(store, guard, configPath, pointerCandidate)
		}
	}
	// An active pointer whose version no longer resolves (repair dropped its
	// entry or object) is not a version anyone can go back to. Recording it
	// as rollback would overwrite the rollback that DOES resolve, so the
	// rollback pointer is left as it is. Any other failure aborts.
	preserveRollback := false
	if hasActive {
		if _, err := readVersionLocked(store, guard, configPath, active); err != nil {
			if !isNotExist(err) {
				return fmt.Errorf("promote candidate: resolve active %s: %w", active, err)
			}
			preserveRollback = true
		}
	}
	if !hasActive {
		legacyData, readErr := guard.ReadFile(configPath)
		if readErr == nil {
			// The adopted config is the one the candidate replaces, so its
			// version is stamped one millisecond BEFORE the candidate. A
			// stamp taken from the clock here is later than the candidate,
			// and history (newest first) would then list the replaced config
			// as newer than the commit that replaced it.
			candidateTime, parseErr := parseVersionStamp(candidate)
			if parseErr != nil {
				return fmt.Errorf("promote candidate: %w", parseErr)
			}
			activeTime := candidateTime.Add(-time.Millisecond)
			active = FormatVersionStamp(activeTime)
			if writeErr := guard.WriteVersion(configPath, legacyData, activeTime); writeErr != nil {
				return fmt.Errorf("promote candidate: write rollback version: %w", writeErr)
			}
			hasActive = true
		} else if !isNotExist(readErr) {
			return fmt.Errorf("promote candidate: read legacy active: %w", readErr)
		}
	}

	switch {
	case preserveRollback:
	case hasActive:
		if err := writePointerLocked(store, guard, configPath, pointerRollback, active); err != nil {
			return err
		}
	default:
		if err := clearPointerLocked(store, guard, configPath, pointerRollback); err != nil {
			return err
		}
	}
	if err := writePointerLocked(store, guard, configPath, pointerActive, candidate); err != nil {
		return err
	}
	if err := guard.WriteFile(configPath, candidateData, 0o600); err != nil {
		slogutil.Logger("storage").Warn("mirror active config failed", "path", configPath, "error", err)
	}
	return clearPointerLocked(store, guard, configPath, pointerCandidate)
}

func readPointerLocked(store Storage, guard WriteGuard, configPath string, pointer pointerName) (string, bool, error) {
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
	if _, err := parseVersionStamp(stamp); err != nil {
		return "", false, fmt.Errorf("read %s pointer: %w", pointer, err)
	}
	return stamp, true, nil
}

func clearPointerLocked(store Storage, guard WriteGuard, configPath string, pointer pointerName) error {
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

// readVersionLocked is readVersion under a held guard. It reads through the
// guard's raw-key reader, because Storage.ReadVersion would wait forever on
// the lock this caller holds.
func readVersionLocked(store Storage, guard WriteGuard, configPath, stamp string) ([]byte, error) {
	path, err := versionPath(store, configPath, stamp)
	if err != nil {
		return nil, err
	}
	data, err := readVersionEntry(guard.ReadKey, path)
	if err != nil {
		return nil, fmt.Errorf("read config version %s: %w", stamp, err)
	}
	return data, nil
}

// removeVersionLocked deletes the history entry of configPath at stamp, then
// its object when no remaining entry of any name references it. It reports
// whether it deleted the entry. It deletes nothing, and reports false, when a
// pointer still names the stamp (the version is retained, entry and object)
// or when the entry is already absent. The object step runs only after a real
// deletion of an entry whose digest was read: a sweep regardless would delete
// the object of a version that is still there.
func removeVersionLocked(store Storage, guard WriteGuard, configPath, stamp string) (bool, error) {
	path, err := versionPath(store, configPath, stamp)
	if err != nil {
		return false, err
	}
	for _, pointer := range []pointerName{pointerActive, pointerRollback, pointerRecovery, pointerCandidate} {
		reference, present, err := readPointerLocked(store, guard, configPath, pointer)
		if err != nil {
			return false, err
		}
		if present {
			if reference == stamp {
				return false, nil
			}
		}
	}
	value, err := guard.ReadKey(path)
	if err != nil {
		if isNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("remove config version %s: %w", stamp, err)
	}
	// A malformed entry names no object, so its removal has nothing to sweep;
	// `ze data check` reports any object it leaves unreferenced.
	digest, digestErr := entryDigest(path, value)
	if err := guard.RemoveKey(path); err != nil {
		if isNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("remove config version %s: %w", stamp, err)
	}
	if digestErr != nil {
		return true, nil
	}
	return true, sweepObject(guard, digest)
}

func releaseGuard(guard WriteGuard, err error) error {
	if releaseErr := guard.Release(); releaseErr != nil {
		return errors.Join(err, fmt.Errorf("release storage lock: %w", releaseErr))
	}
	return err
}

// ReadActiveConfig reads the config referenced by the active pointer, falling back to legacy active storage.
func ReadActiveConfig(store Storage, configPath string) ([]byte, error) {
	stamp, ok, err := readPointer(store, configPath, pointerActive)
	if err != nil {
		return nil, err
	}
	if ok {
		data, err := store.ReadVersion(configPath, stamp)
		if err != nil {
			// An existing active pointer never falls back to the mirror: the
			// error names the pointer and the stamp that did not resolve, and
			// keeps fs.ErrNotExist when the entry or object is absent.
			return nil, fmt.Errorf("%w: %s names %s: %w", ErrActiveUnresolved, activePointerKey(configPath), stamp, err)
		}
		return data, nil
	}
	return store.ReadFile(configPath)
}

// activePointerKey names the active pointer of configPath for an error.
func activePointerKey(configPath string) string {
	return zefs.KeyConfigActive.Key(filepath.Base(configPath))
}

// ReadCandidateConfig reads the config version referenced by candidate.
func ReadCandidateConfig(store Storage, configPath string) ([]byte, string, bool, error) {
	stamp, ok, err := readPointer(store, configPath, pointerCandidate)
	if err != nil || !ok {
		return nil, "", ok, err
	}
	data, err := readVersion(store, configPath, stamp)
	if err != nil {
		return nil, "", false, err
	}
	return data, stamp, true, nil
}

func pointerPath(backing Storage, configPath string, pointer pointerName) (string, error) {
	// The interface method reaches the store through every embedding wrapper.
	if err := backing.CheckName(configPath); err != nil {
		return "", err
	}
	if !pointer.valid() {
		return "", fmt.Errorf("unknown config pointer %q", pointer)
	}
	name := filepath.Base(configPath)
	if err := validKey(name); err != nil {
		return "", err
	}
	if strings.Contains(name, "..") {
		return "", fmt.Errorf("invalid config name %q", name)
	}
	switch pointer {
	case pointerActive:
		return zefs.KeyConfigActive.Key(name), nil
	case pointerCandidate:
		return zefs.KeyConfigCandidate.Key(name), nil
	case pointerRollback:
		return zefs.KeyConfigRollback.Key(name), nil
	case pointerRecovery:
		return zefs.KeyConfigRecovery.Key(name), nil
	}
	return "", fmt.Errorf("unknown config pointer %q", pointer)
}

func versionPath(backing Storage, configPath, stamp string) (string, error) {
	if err := backing.CheckName(configPath); err != nil {
		return "", err
	}
	name := filepath.Base(configPath)
	if err := validKey(name); err != nil {
		return "", err
	}
	if strings.Contains(name, "..") {
		return "", fmt.Errorf("invalid config name %q", name)
	}
	if _, err := parseVersionStamp(stamp); err != nil {
		return "", fmt.Errorf("config version path: %w", err)
	}
	return zefs.KeyFileVersion.Key(stamp, name), nil
}

func isNotExist(err error) bool {
	return errors.Is(err, fs.ErrNotExist) || errors.Is(err, os.ErrNotExist)
}
