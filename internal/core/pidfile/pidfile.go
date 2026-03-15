// Design: docs/architecture/system-architecture.md — PID file management
//
// Package pidfile manages PID files for Ze daemon instances.
//
// It provides mutual exclusion via flock(2) to prevent duplicate instances,
// stale PID detection, and XDG-compliant file location resolution.
package pidfile

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/config/storage"
)

// PIDFile represents an acquired PID file with an active flock.
// In filesystem mode, fd holds the flock for the daemon's lifetime.
// In blob mode, fd is nil and store holds the storage reference for cleanup.
type PIDFile struct {
	path  string
	fd    *os.File
	store storage.Storage // non-nil in blob mode; used by Release to remove PID entry
}

// Info contains parsed information from a PID file.
type Info struct {
	PID        int
	ConfigPath string
	StartTime  string
	Locked     bool
}

// Location returns the PID file path for a given config file path.
// Uses the same resolution cascade as the API socket (see config.DefaultSocketPath):
//
//  1. $XDG_RUNTIME_DIR/ze/<config-hash>.pid (per-user runtime dir)
//  2. /var/run/ze/<config-hash>.pid (system runtime dir, when running as root)
//  3. /tmp/ze/<config-hash>.pid (fallback, always writable)
func Location(configPath string) (string, error) {
	absConfig, err := filepath.Abs(configPath)
	if err != nil {
		return "", fmt.Errorf("resolve config path: %w", err)
	}

	hash := ConfigHash(absConfig)
	filename := hash + ".pid"

	// Priority 1: XDG_RUNTIME_DIR
	if xdg := os.Getenv("XDG_RUNTIME_DIR"); xdg != "" {
		zeDir := filepath.Join(xdg, "ze")
		if mkErr := os.MkdirAll(zeDir, 0o700); mkErr == nil {
			return filepath.Join(zeDir, filename), nil
		}
	}

	// Priority 2: /var/run/ze (root only)
	if os.Getuid() == 0 {
		zeDir := "/var/run/ze"
		if mkErr := os.MkdirAll(zeDir, 0o750); mkErr == nil { //nolint:gosec // System runtime dir needs group read for service management
			return filepath.Join(zeDir, filename), nil
		}
	}

	// Priority 3: /tmp/ze (fallback)
	zeDir := filepath.Join(os.TempDir(), "ze")
	if mkErr := os.MkdirAll(zeDir, 0o700); mkErr == nil {
		return filepath.Join(zeDir, filename), nil
	}

	return "", fmt.Errorf("no writable location for PID file")
}

// ConfigHash returns the first 8 characters of the SHA256 hash of a config path.
func ConfigHash(configPath string) string {
	h := sha256.Sum256([]byte(configPath))
	return fmt.Sprintf("%x", h[:4])
}

// Noop returns a PIDFile whose Release is a no-op.
// Used when PID file acquisition is skipped (e.g., stdin config).
func Noop() *PIDFile {
	return &PIDFile{}
}

// Acquire creates a PID file and acquires an exclusive flock.
// Returns an error if another instance already holds the lock.
func Acquire(pidPath, configPath string) (*PIDFile, error) {
	// Create parent directory if needed
	if err := os.MkdirAll(filepath.Dir(pidPath), 0o700); err != nil {
		return nil, fmt.Errorf("create PID file directory: %w", err)
	}

	// Open or create the file
	fd, err := os.OpenFile(pidPath, os.O_CREATE|os.O_RDWR, 0o644) //nolint:gosec // PID file needs to be readable
	if err != nil {
		return nil, fmt.Errorf("open PID file: %w", err)
	}

	// Try non-blocking exclusive lock
	if err := syscall.Flock(int(fd.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		fd.Close() //nolint:errcheck,gosec // Best effort cleanup on lock failure
		return nil, fmt.Errorf("already running (PID file %s is locked)", pidPath)
	}

	// Write PID file content
	content := fmt.Sprintf("%d\n%s\n%s\n", os.Getpid(), configPath, time.Now().UTC().Format(time.RFC3339))
	if err := fd.Truncate(0); err != nil {
		fd.Close() //nolint:errcheck,gosec // Best effort cleanup on write failure
		return nil, fmt.Errorf("truncate PID file: %w", err)
	}
	if _, err := fd.Seek(0, 0); err != nil {
		fd.Close() //nolint:errcheck,gosec // Best effort cleanup on write failure
		return nil, fmt.Errorf("seek PID file: %w", err)
	}
	if _, err := fd.WriteString(content); err != nil {
		fd.Close() //nolint:errcheck,gosec // Best effort cleanup on write failure
		return nil, fmt.Errorf("write PID file: %w", err)
	}
	if err := fd.Sync(); err != nil {
		fd.Close() //nolint:errcheck,gosec // Best effort cleanup on write failure
		return nil, fmt.Errorf("sync PID file: %w", err)
	}

	return &PIDFile{path: pidPath, fd: fd}, nil
}

// AcquireWithStorage writes a PID entry into blob storage with kill(0) mutual exclusion.
// Unlike filesystem Acquire (flock held for daemon lifetime), the storage WriteLock
// is held only for the check-and-write. Mutual exclusion relies on PID + kill check.
func AcquireWithStorage(store storage.Storage, pidKey, configPath string) (*PIDFile, error) {
	guard, err := store.AcquireLock(configPath)
	if err != nil {
		return nil, fmt.Errorf("acquire storage lock for PID: %w", err)
	}
	defer guard.Release() //nolint:errcheck // best effort release

	// Check for existing PID entry.
	if data, readErr := guard.ReadFile(pidKey); readErr == nil {
		lines := strings.Split(strings.TrimSpace(string(data)), "\n")
		if len(lines) >= 1 {
			if pid, parseErr := strconv.Atoi(lines[0]); parseErr == nil && pid > 0 {
				// Check if process is alive via kill(0).
				// EPERM means the process exists but belongs to a different user.
				killErr := syscall.Kill(pid, 0)
				if killErr == nil || errors.Is(killErr, syscall.EPERM) {
					return nil, fmt.Errorf("already running (PID %d in storage key %s)", pid, pidKey)
				}
			}
		}
	}

	// Write own PID entry.
	content := fmt.Sprintf("%d\n%s\n%s\n", os.Getpid(), configPath, time.Now().UTC().Format(time.RFC3339))
	if err := guard.WriteFile(pidKey, []byte(content), 0o644); err != nil {
		return nil, fmt.Errorf("write PID to storage: %w", err)
	}

	return &PIDFile{path: pidKey, store: store}, nil
}

// Release releases the flock and removes the PID file.
// In blob mode, removes the PID entry from storage.
func (f *PIDFile) Release() {
	if f.store != nil {
		// Blob mode: remove PID entry from storage.
		f.store.Remove(f.path) //nolint:errcheck // best effort cleanup
		f.store = nil
		return
	}
	if f.fd == nil {
		return
	}
	// Filesystem mode: unlock, close, remove.
	syscall.Flock(int(f.fd.Fd()), syscall.LOCK_UN) //nolint:errcheck,gosec // Best effort unlock
	f.fd.Close()                                   //nolint:errcheck,gosec // Best effort close
	os.Remove(f.path)                              //nolint:errcheck,gosec // Best effort remove
	f.fd = nil
}

// ReadInfo reads and parses a PID file, checking if it's locked.
func ReadInfo(pidPath string) (*Info, error) {
	data, err := os.ReadFile(pidPath) //nolint:gosec // PID file path from user config
	if err != nil {
		return nil, fmt.Errorf("read PID file: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) < 3 {
		return nil, fmt.Errorf("malformed PID file: expected 3 lines, got %d", len(lines))
	}

	pid, err := strconv.Atoi(lines[0])
	if err != nil {
		return nil, fmt.Errorf("invalid PID %q: %w", lines[0], err)
	}
	if pid < 1 {
		return nil, fmt.Errorf("invalid PID %d: must be >= 1", pid)
	}

	locked := isLocked(pidPath)

	return &Info{
		PID:        pid,
		ConfigPath: lines[1],
		StartTime:  lines[2],
		Locked:     locked,
	}, nil
}

// isLocked checks if a PID file has an active flock.
func isLocked(pidPath string) bool {
	fd, err := os.Open(pidPath) //nolint:gosec // PID file path from user config
	if err != nil {
		return false
	}
	defer fd.Close() //nolint:errcheck,gosec // Best effort close in probe

	lockErr := syscall.Flock(int(fd.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if lockErr != nil {
		// Lock failed → someone holds it → process is running
		return true
	}
	// Lock succeeded → no one holds it → stale; release our probe lock
	syscall.Flock(int(fd.Fd()), syscall.LOCK_UN) //nolint:errcheck,gosec // Best effort unlock
	return false
}
