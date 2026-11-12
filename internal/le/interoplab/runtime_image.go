// Design: docs/contributing/preview-artifacts.md -- images package inspected binaries.
// Related: docker_build_context.go -- only this staged directory reaches Docker.
package interoplab

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// StageRuntimeImage creates a new, caller-owned context containing only the
// shared runtime recipe and binary. The caller MUST supply a completed binary
// built for the Docker server and MUST remove directory after Build returns.
// No path is reused, and symlink inputs are refused. Independent directories
// may be staged concurrently; a directory has exactly one owner.
func StageRuntimeImage(root, directory, binary, target string) (build ImageBuild, err error) {
	switch target {
	case "deployment", "lab":
	default:
		return build, fmt.Errorf("unknown runtime image target %q; choose deployment or lab", target)
	}
	directory, err = filepath.Abs(directory)
	if err != nil {
		return build, err
	}
	if err := os.Mkdir(directory, 0o700); err != nil {
		return build, err
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, os.RemoveAll(directory))
		}
	}()
	if err := copyRuntimeInput(filepath.Join(root, "docker", "Dockerfile.runtime"), filepath.Join(directory, "Dockerfile"), 0o644); err != nil {
		return build, err
	}
	if err := copyRuntimeInput(binary, filepath.Join(directory, "ze"), 0o755); err != nil {
		return build, err
	}
	return ImageBuild{Context: directory, Dockerfile: filepath.Join(directory, "Dockerfile"), Target: target}, nil
}

func openRuntimeInput(path string) (*os.File, error) {
	before, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !before.Mode().IsRegular() {
		return nil, fmt.Errorf("runtime image input must be a regular file without a symlink: %s", path)
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	after, err := file.Stat()
	if err != nil {
		return nil, errors.Join(err, file.Close())
	}
	if !os.SameFile(before, after) {
		return nil, errors.Join(fmt.Errorf("runtime image input changed while opening: %s", path), file.Close())
	}
	return file, nil
}

func copyRuntimeInput(source, path string, mode os.FileMode) (err error) {
	input, err := openRuntimeInput(source)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, input.Close()) }()
	if mode&0o111 != 0 {
		info, err := input.Stat()
		if err != nil {
			return err
		}
		if info.Mode().Perm()&0o111 == 0 {
			return fmt.Errorf("runtime image binary is not executable: %s", source)
		}
	}
	output, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, output.Close()) }()
	if _, err := io.Copy(output, input); err != nil {
		return err
	}
	return output.Chmod(mode)
}
