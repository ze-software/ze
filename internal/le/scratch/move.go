// Design: plan/spec-le-is-a-ze-binary.md -- cross-device scratch migration
// Overview: scratch.go -- migration policy and status reporting
package scratch

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
)

func (m *Manager) moveEntry(source, target string) error {
	if pathExists(target) {
		return errors.New("target already exists")
	}
	err := m.fs.rename(source, target)
	if err == nil {
		return nil
	}
	if !errors.Is(err, syscall.EXDEV) {
		return fmt.Errorf("rename: %w", err)
	}

	parent := filepath.Dir(target)
	stage, err := os.MkdirTemp(parent, ".scratch-migrate-")
	if err != nil {
		return fmt.Errorf("create staging directory: %w", err)
	}
	defer func() {
		_ = os.RemoveAll(stage) //nolint:errcheck // The authoritative source survives every copy failure.
	}()

	payload := filepath.Join(stage, "payload")
	if err := copyEntry(source, payload); err != nil {
		return fmt.Errorf("copy across devices: %w", err)
	}
	if err := renameNoReplace(payload, target); err != nil {
		return fmt.Errorf("publish copied entry: %w", err)
	}
	if err := removeSource(source); err != nil {
		return fmt.Errorf("remove copied source: %w", err)
	}
	return nil
}

func copyEntry(source, target string) error {
	info, err := os.Lstat(source)
	if err != nil {
		return err
	}
	mode := info.Mode()
	if mode&os.ModeSymlink != 0 {
		return copySymlink(source, target, info)
	}
	if mode.IsRegular() {
		return copyRegular(source, target, info)
	}
	if mode.IsDir() {
		return copyDirectory(source, target, info)
	}
	return fmt.Errorf("unsupported file mode %s", mode)
}

func copyRegular(source, target string, info os.FileInfo) (result error) {
	input, err := os.Open(source) //nolint:gosec // source is an owner-selected fixture or checkout entry
	if err != nil {
		return err
	}
	defer func() {
		result = errors.Join(result, input.Close())
	}()

	output, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode().Perm()) //nolint:gosec // the source mode is the contract
	if err != nil {
		return err
	}
	if _, err := io.Copy(output, input); err != nil {
		return errors.Join(err, output.Close())
	}
	if err := output.Close(); err != nil {
		return err
	}
	if err := preserveOwnership(target, info, false); err != nil {
		return err
	}
	return os.Chmod(target, info.Mode())
}

func copyDirectory(source, target string, info os.FileInfo) error {
	if err := os.Mkdir(target, 0o700); err != nil {
		return err
	}
	entries, err := os.ReadDir(source)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if err := copyEntry(filepath.Join(source, entry.Name()), filepath.Join(target, entry.Name())); err != nil {
			return err
		}
	}
	if err := preserveOwnership(target, info, false); err != nil {
		return err
	}
	return os.Chmod(target, info.Mode())
}

func copySymlink(source, target string, info os.FileInfo) error {
	linkTarget, err := os.Readlink(source)
	if err != nil {
		return err
	}
	if err := os.Symlink(linkTarget, target); err != nil {
		return err
	}
	return preserveOwnership(target, info, true)
}

func preserveOwnership(target string, source os.FileInfo, symlink bool) error {
	wanted, ok := source.Sys().(*syscall.Stat_t)
	if !ok {
		return errors.New("source has no ownership metadata")
	}
	current, err := os.Lstat(target)
	if err != nil {
		return err
	}
	got, ok := current.Sys().(*syscall.Stat_t)
	if !ok {
		return errors.New("target has no ownership metadata")
	}
	if wanted.Uid != got.Uid {
		return preserveOwnershipChange(target, wanted, symlink)
	}
	if wanted.Gid != got.Gid {
		return preserveOwnershipChange(target, wanted, symlink)
	}
	return nil
}

func preserveOwnershipChange(target string, wanted *syscall.Stat_t, symlink bool) error {
	if symlink {
		return os.Lchown(target, int(wanted.Uid), int(wanted.Gid))
	}
	return os.Chown(target, int(wanted.Uid), int(wanted.Gid))
}

func removeSource(source string) error {
	info, err := os.Lstat(source)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return os.RemoveAll(source)
	}
	return os.Remove(source)
}
