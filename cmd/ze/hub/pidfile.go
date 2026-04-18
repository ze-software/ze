// Design: docs/architecture/hub-architecture.md -- PID file lifecycle
// Related: main.go -- calls writePIDFile before dropPrivileges / removePIDFile at shutdown
//
// Writes `ze.pid.file` when set (operator-supplied path or YANG
// `environment { daemon { pid "..."; } }` plumbed by config.ApplyEnvConfig).
// Runs BEFORE the privilege drop so root-owned directories like /var/run
// accept the create; when ze.user is configured the file is chowned to the
// drop-to user so the shutdown path (running as that user) can remove it.
// Refuses to overwrite an existing file (symlink attack defense).

package hub

import (
	"fmt"
	"os"
	"os/user"
	"strconv"

	"codeberg.org/thomas-mangin/ze/internal/core/env"
)

// writePIDFile creates the PID file named by ze.pid.file with the current PID.
// Returns (path, nil) when the file was written, ("", nil) when the env var is
// unset, or ("", err) on failure. Fails closed on permission / existence
// errors so the operator sees the problem at startup.
//
// If ze.user is also set, the file is chowned to that user (+ primary group)
// so removePIDFile succeeds post-drop. Chown failures are logged but do not
// abort startup: a PID file that the daemon cannot remove at shutdown is a
// lesser problem than refusing to start.
func writePIDFile() (string, error) {
	path := env.Get("ze.pid.file")
	if path == "" {
		return "", nil
	}
	// O_CREATE|O_EXCL|O_WRONLY refuses to overwrite an existing file;
	// this blocks a symlink-pointed-at-arbitrary-target attack and also
	// surfaces accidental concurrent starts.
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644) //nolint:gosec // operator-supplied path
	if err != nil {
		return "", fmt.Errorf("create pid file %q: %w", path, err)
	}
	pid := os.Getpid()
	if _, werr := f.WriteString(strconv.Itoa(pid) + "\n"); werr != nil {
		if closeErr := f.Close(); closeErr != nil {
			fmt.Fprintf(os.Stderr, "warning: close pid file %q after write failure: %v\n", path, closeErr)
		}
		if rmErr := os.Remove(path); rmErr != nil {
			fmt.Fprintf(os.Stderr, "warning: remove pid file %q after write failure: %v\n", path, rmErr)
		}
		return "", fmt.Errorf("write pid file %q: %w", path, werr)
	}
	if cerr := f.Close(); cerr != nil {
		if rmErr := os.Remove(path); rmErr != nil {
			fmt.Fprintf(os.Stderr, "warning: remove pid file %q after close failure: %v\n", path, rmErr)
		}
		return "", fmt.Errorf("close pid file %q: %w", path, cerr)
	}
	chownPIDFileForDrop(path)
	return path, nil
}

// chownPIDFileForDrop transfers ownership of the PID file to the ze.user so
// the shutdown path (running post-drop) can remove it. No-op when ze.user
// is unset or the chown fails (e.g., test env running as non-root).
func chownPIDFileForDrop(path string) {
	username := env.Get("ze.user")
	if username == "" {
		return
	}
	u, err := user.Lookup(username)
	if err != nil {
		// Fall through: drop will also fail to resolve the user and surface
		// the error there. Avoid doubling the diagnostic.
		return
	}
	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		return
	}
	gid, err := strconv.Atoi(u.Gid)
	if err != nil {
		return
	}
	if err := os.Chown(path, uid, gid); err != nil {
		fmt.Fprintf(os.Stderr, "warning: chown pid file %q to %s (%d:%d): %v\n",
			path, username, uid, gid, err)
	}
}

// removePIDFile removes the PID file previously written by writePIDFile.
// Quietly ignores ENOENT (the file is already gone). Other errors are logged
// to stderr because the caller is on a shutdown path where recovery is not
// possible.
func removePIDFile(path string) {
	if path == "" {
		return
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "warning: remove pid file %q: %v\n", path, err)
	}
}
