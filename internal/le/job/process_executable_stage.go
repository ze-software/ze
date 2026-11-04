// Design: docs/contributing/running-commands.md -- portable held executable launch.
package job

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// stageProcessExecutable borrows executable and reads from offset zero. The
// caller MUST remove any returned exclusive scratch artifact after child reap or
// launch refusal. A nil cleanup means no artifact was needed. It never reopens executable.Name().
func stageProcessExecutable(cmd *exec.Cmd, executable *os.File) (_ func() error, retErr error) {
	if executable == nil {
		return nil, nil //nolint:nilnil // runProcess accepts successful preparation with no cleanup obligation.
	}
	info, err := executable.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("held executable is not regular; rebuild this session's binary")
	}
	root, err := lepath.Root()
	if err != nil {
		return nil, err
	}
	paths, err := lepath.ResolveSession(root, true)
	if err != nil {
		return nil, err
	}
	file, err := os.CreateTemp(filepath.Join(root, paths.Scratch), "seed-exec-*")
	if err != nil {
		return nil, err
	}
	name := file.Name()
	cleanup := func() error { return os.Remove(name) }
	defer func() {
		if retErr != nil {
			retErr = errors.Join(retErr, cleanup())
		}
	}()
	_, copyErr := io.Copy(file, io.NewSectionReader(executable, 0, info.Size()))
	if copyErr != nil {
		return nil, errors.Join(copyErr, file.Close())
	}
	if err := file.Chmod(0o700); err != nil {
		return nil, errors.Join(err, file.Close())
	}
	if err := file.Close(); err != nil {
		return nil, err
	}
	cmd.Path = name
	// Keep fd 3 occupied so the inherited seed directory remains fd 4.
	cmd.ExtraFiles = []*os.File{executable}
	return cleanup, nil
}
