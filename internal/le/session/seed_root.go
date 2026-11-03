// Design: docs/contributing/running-commands.md -- retained session seed directories.
package session

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// seedResources holds the acquired native entry. Its zero value is invalid;
// only openSeedResources constructs it. The owner MUST close both handles after
// the last child returns. It is not shared between seeding calls.
type seedResources struct {
	config     *os.Root
	executable *os.File
}

// openSeedResources retains the config directory and executable under an owned
// session. The caller MUST close both returned handles after their last use.
func openSeedResources(root, binary string) (_ *seedResources, retErr error) {
	dir, err := validateSessionBinary(binary)
	if err != nil {
		return nil, err
	}
	// This prefix deliberately permits the managed scratch mount's symlink.
	parent, err := os.OpenRoot(filepath.Join(root, "tmp", "session"))
	if err != nil {
		return nil, err
	}
	defer parent.Close() //nolint:errcheck // Read-only parent handle.
	session, err := openSeedDirectory(parent, filepath.Base(dir), false)
	if err != nil {
		return nil, err
	}
	defer session.Close() //nolint:errcheck // Retained config has its own handle.
	marker, err := session.OpenFile(".session-id", os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	defer marker.Close() //nolint:errcheck // Read-only owner marker.
	info, err := marker.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("session owner marker is not a regular file; use the caller's session binary")
	}
	owner, err := io.ReadAll(marker)
	if err != nil {
		return nil, err
	}
	id, err := lepath.ResolveSessionID(root)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(string(owner)) != id {
		return nil, fmt.Errorf("session owner changed; use the caller's session binary")
	}
	bin, err := openSeedDirectory(session, "bin", false)
	if err != nil {
		return nil, err
	}
	defer bin.Close() //nolint:errcheck // The executable retains its own descriptor.
	executable, err := bin.OpenFile(filepath.Base(binary), os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, fmt.Errorf("cannot open session executable: %w; rebuild this session's binary", err)
	}
	defer func() {
		if retErr != nil {
			retErr = errors.Join(retErr, executable.Close())
		}
	}()
	info, err = executable.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("session executable is not a regular file; rebuild this session's binary")
	}
	if info.Mode()&0o111 == 0 {
		return nil, fmt.Errorf("session binary is not executable; rebuild this session's binary")
	}
	etc, err := openSeedDirectory(session, "etc", true)
	if err != nil {
		return nil, err
	}
	defer etc.Close() //nolint:errcheck // Retained config has its own handle.
	config, err := openSeedDirectory(etc, "ze", true)
	if err != nil {
		return nil, err
	}
	return &seedResources{config: config, executable: executable}, nil
}

// openSeedDirectory validates the opened handle against the nonsymlink entry,
// then retains it. A replacement after this comparison cannot redirect its I/O.
func openSeedDirectory(parent *os.Root, name string, create bool) (*os.Root, error) {
	if create {
		if err := parent.Mkdir(name, 0o700); err != nil {
			if !errors.Is(err, os.ErrExist) {
				return nil, err
			}
		}
	}
	entry, err := parent.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !entry.IsDir() {
		return nil, fmt.Errorf("session directory %s is not a directory or is a symlink; use this session's own directory", name)
	}
	opened, err := parent.OpenRoot(name)
	if err != nil {
		return nil, err
	}
	actual, err := opened.Stat(".")
	if err != nil {
		return nil, errors.Join(err, opened.Close())
	}
	if !os.SameFile(entry, actual) {
		return nil, errors.Join(fmt.Errorf("session directory %s changed while opening; retry with this session's own directory", name), opened.Close())
	}
	return opened, nil
}
