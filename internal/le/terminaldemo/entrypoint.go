package terminaldemo

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func runContainerEntrypoint(args []string, stdout, stderr io.Writer) (err error) {
	environ := demoEnvironment()
	var lock *os.File
	if envValue(environ, "ZE_DEMO_LOCK_HELD") == "" {
		lock, err = acquireContainerLock()
		if err != nil {
			return err
		}
	}
	if lock != nil {
		defer func() {
			if closeErr := lock.Close(); err == nil && closeErr != nil {
				err = fmt.Errorf("close demo lock: %w", closeErr)
			}
		}()
	}
	defer giveDemoOwnershipBack(environ)

	home := envValue(environ, "HOME")
	for _, directory := range []string{home, envValue(environ, "XDG_CONFIG_HOME"), envValue(environ, "XDG_DATA_HOME"), envValue(environ, "XDG_RUNTIME_DIR"), filepath.Join(home, ".ssh")} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			return err
		}
	}
	sshConfig := "Host ze-demo\n  HostName 127.0.0.1\n  Port 2222\n  User admin\n  StrictHostKeyChecking no\n  UserKnownHostsFile /dev/null\n"
	if err := os.WriteFile(filepath.Join(home, ".ssh", "config"), []byte(sshConfig), 0o600); err != nil {
		return err
	}
	if err := os.MkdirAll("/root/.ssh", 0o700); err != nil {
		return err
	}
	if err := os.WriteFile("/root/.ssh/config", []byte(sshConfig), 0o600); err != nil {
		return err
	}

	name := args[0]
	arguments := args[1:]
	if strings.HasSuffix(name, ".tape") {
		arguments = append([]string{"--tape", name}, arguments...)
		name = demoBinary("ze-terminal-pty")
	}
	// The demo tree is the directory a tape's own paths are written against: the
	// shared `Source common.tape` and each `Output artifacts/<id>.<ext>`, which is
	// the mounted artifact directory. `containerCommand` passes it as --workdir for
	// that reason, and running the child anywhere else writes the recording outside
	// the mount. The image's own WORKDIR is /src, so this cannot be left to inherit.
	process := newProcess(name, arguments, commandOptions{stdin: os.Stdin, stdout: stdout, stderr: stderr, env: environ, dir: demoTree()})
	if err := process.Run(); err != nil {
		if exitError, ok := errors.AsType[*os.PathError](err); ok {
			return exitError
		}
		return fmt.Errorf("%s: %w", name, err)
	}
	return nil
}

// acquireContainerLock takes the exclusive demo lock. The caller MUST NOT call
// it when an outer process already holds that lock.
func acquireContainerLock() (*os.File, error) {
	lockDir := os.Getenv("ZE_DEMO_LOCK_DIR")
	if lockDir == "" {
		lockDir = filepath.Join(demoRoot(), "tmp", "terminal-demos")
	}
	if err := os.MkdirAll(lockDir, 0o750); err != nil {
		return nil, fmt.Errorf("cannot create the demo lock directory %s: %w", lockDir, err)
	}
	lockPath := filepath.Join(lockDir, "demo-run.lock")
	// 0o644 is deliberate: the container and the host user both open this lock.
	lock, err := os.OpenFile(lockPath, os.O_CREATE|os.O_WRONLY, 0o644) //nolint:gosec // shared between the container and the host account, see above
	if err != nil {
		return nil, err
	}
	waitSeconds := 1800
	if value := os.Getenv("ZE_DEMO_LOCK_WAIT"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 0 {
			_ = lock.Close()
			return nil, fmt.Errorf("ZE_DEMO_LOCK_WAIT must be seconds >= 0, got %q", value)
		}
		waitSeconds = parsed
	}
	const retryDelay = 100 * time.Millisecond
	for attempt := 0; attempt <= waitSeconds*10; attempt++ {
		err = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			return lock, nil
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) && !errors.Is(err, syscall.EAGAIN) {
			_ = lock.Close()
			return nil, fmt.Errorf("flock %s: %w", lockPath, err)
		}
		if attempt == waitSeconds*10 {
			_ = lock.Close()
			return nil, fmt.Errorf("another demo run held %s for %d seconds", lockPath, waitSeconds)
		}
		time.Sleep(retryDelay)
	}
	return nil, errors.New("unreachable demo lock state")
}

func giveDemoOwnershipBack(environ []string) {
	uid, uidErr := strconv.Atoi(envValue(environ, "HOST_UID"))
	gid, gidErr := strconv.Atoi(envValue(environ, "HOST_GID"))
	if uidErr != nil || gidErr != nil {
		return
	}
	roots := []string{filepath.Join(demoRoot(), "demos", "terminal", "artifacts"), filepath.Join(demoRoot(), "tmp", "terminal-demos")}
	for _, root := range roots {
		_ = filepath.Walk(root, func(path string, _ os.FileInfo, err error) error {
			if err == nil {
				_ = os.Chown(path, uid, gid) //nolint:gosec // G122: giving the host account back what the container created, best effort
			}
			return nil
		})
	}
}
