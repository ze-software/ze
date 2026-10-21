// Design: docs/architecture/appliance/docker-host.md -- one setup run at a time
// Related: colima.go -- start and stop, which run under this lock
// Related: build.go -- the image build, which runs under this lock

package dockerhost

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// lockFileName is the lock file under the docker-host cache root.
const lockFileName = "setup.lock"

// hostLock is an exclusive hold on the docker-host cache, taken by build,
// start and stop so two sessions never run `colima delete` under each other's
// start, nor write the same image (spec R-10). The kernel releases the hold
// when the process exits, so a killed run never leaves the cache locked.
// Not safe for concurrent use: one goroutine takes it, and the same one MUST
// call release.
type hostLock struct {
	file *os.File
}

// takeLock takes the hold without waiting. When another process holds it,
// takeLock refuses naming that process's pid and the lock file, so the
// operator can see which run to wait for.
func takeLock(cacheDir string) (*hostLock, error) {
	if err := os.MkdirAll(cacheDir, 0o755); err != nil { //nolint:gosec // a cache directory in the user's home
		return nil, fmt.Errorf("create %s: %w", cacheDir, err)
	}
	path := filepath.Join(cacheDir, lockFileName)
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o644) //nolint:gosec // the lock file in the cache root
	if err != nil {
		return nil, fmt.Errorf("open the setup lock %s: %w", path, err)
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		holder := lockHolder(file)
		_ = file.Close() //nolint:errcheck // the refusal below is the answer
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, fmt.Errorf("another `./le setup docker-host` run (pid %s) holds %s, and two runs at once could delete "+
				"the profile under each other. Fix: wait for that run to finish, then run this command again", holder, path)
		}
		return nil, fmt.Errorf("lock %s: %w", path, err)
	}
	if err := writeHolder(file); err != nil {
		_ = file.Close() //nolint:errcheck // closing drops the hold; the write error is the answer
		return nil, fmt.Errorf("record this run's pid in %s: %w", path, err)
	}
	return &hostLock{file: file}, nil
}

// release drops the hold. It MUST be called once, after takeLock succeeded.
func (l *hostLock) release() error {
	return l.file.Close()
}

// writeHolder replaces the lock file's content with this process's pid.
func writeHolder(file *os.File) error {
	if err := file.Truncate(0); err != nil {
		return err
	}
	_, err := file.WriteAt([]byte(strconv.Itoa(os.Getpid())+"\n"), 0)
	return err
}

// lockHolder answers the pid the holder wrote, or says it wrote none yet.
func lockHolder(file *os.File) string {
	buffer := make([]byte, 32)
	count, err := file.ReadAt(buffer, 0)
	pid := strings.TrimSpace(string(buffer[:count]))
	if pid == "" {
		if err != nil {
			return "unknown: " + err.Error()
		}
		return "unknown: not yet written"
	}
	return pid
}
