// Design: docs/architecture/cli/plugin-modes.md -- trusted service executables.

package systemd

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// trustedExecutable checks every existing path component. A missing suffix can
// be created by the root installer only beneath an already trusted directory.
func (service nativeService) trustedExecutable(path string) error {
	current := string(filepath.Separator)
	root, err := service.rt.ops.lstat(current)
	if err != nil {
		return err
	}
	if err := service.trustedNode(current, root); err != nil {
		return err
	}
	parts := strings.Split(strings.TrimPrefix(path, current), current)
	for i, part := range parts {
		if part == ".." {
			return errors.New("service executable path contains a parent component; choose --prefix /opt/ze")
		}
		if part != "" {
			current = filepath.Join(current, part)
		}
		info, err := service.rt.ops.lstat(current)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		} // The root installer creates this absent suffix.
		if err != nil {
			return err
		}
		if err := service.trustedNode(current, info); err != nil {
			return err
		}
		if i < len(parts)-1 {
			if !info.IsDir() {
				return fmt.Errorf("service executable ancestor %s is not a directory; select a protected --prefix", current)
			}
		} else if !info.Mode().IsRegular() {
			return fmt.Errorf("service executable %s is not regular; select a protected --prefix", current)
		}
	}
	return nil
}

func (service nativeService) trustedNode(path string, info fs.FileInfo) error {
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("service path %s is a symbolic link; select a protected --prefix or --no-service", path)
	}
	uid, err := service.rt.ops.owner(info)
	if err != nil {
		return err
	}
	if uid != 0 {
		return fmt.Errorf("service path %s is not owned by root; select a root-owned --prefix such as /opt/ze, or --no-service", path)
	}
	if info.Mode().Perm()&0o022 != 0 {
		return fmt.Errorf("service path %s is writable by another account; select a protected --prefix such as /opt/ze, or --no-service", path)
	}
	return nil
}
