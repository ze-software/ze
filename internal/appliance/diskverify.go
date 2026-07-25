// Design: plan/learned/907-appliance-install-robust.md -- build-side inject verify

package appliance

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// verifyInject runs all build-side verification tiers on the perm image:
// content read-back (mandatory), e2fsck structural check, loopback mount
// (Linux root only). Logs and returns the first error found.
func verifyInject(permImg, dbPath string) error {
	if err := verifyInjectedDB(permImg, dbPath); err != nil {
		slog.Error("inject verification failed", "error", err)
		return err
	}
	if err := verifyE2fsck(permImg); err != nil {
		slog.Error("ext4 structural check failed", "error", err)
		return err
	}
	if err := tryLoopbackVerify(permImg); err != nil {
		slog.Error("mount verification failed", "error", err)
		return err
	}
	return nil
}

// verifyInjectedDB reads the perm image and checks that the source
// database bytes are present. Catches silent debugfs write failures
// (debugfs -R exits 0 on internal errors) without shelling out.
func verifyInjectedDB(permImg, dbPath string) error {
	perm, err := os.ReadFile(permImg) //nolint:gosec // build-controlled path
	if err != nil {
		return fmt.Errorf("read perm image: %w", err)
	}

	source, err := os.ReadFile(dbPath) //nolint:gosec // build-controlled path
	if err != nil {
		return fmt.Errorf("read source: %w", err)
	}

	if len(source) == 0 {
		return fmt.Errorf("source database is empty")
	}

	if !bytes.Contains(perm, source) {
		return fmt.Errorf("database not found in perm image (debugfs write silently failed)")
	}

	return nil
}

// verifyE2fsck runs e2fsck -fn on the perm image to check ext4 structural
// integrity. Skips with a warning if e2fsck is not found. Fails the build
// if e2fsck runs and reports errors.
func verifyE2fsck(permImg string) error {
	e2fsck := filepath.Join(e2fsDir, "e2fsck")
	if _, statErr := os.Stat(e2fsck); statErr != nil {
		slog.Warn("e2fsck not found, skipping structural check")
		return nil //nolint:nilerr // intentional: skip when e2fsck not installed
	}
	if _, err := runExternalFn(e2fsck, "-f", "-n", permImg); err != nil {
		return fmt.Errorf("ext4 structural check failed: %w", err)
	}
	return nil
}

// tryLoopbackVerify mounts the perm image read-only and confirms
// ze/database.zefs is present. Only runs on Linux with root; silently
// skips on other platforms or non-root builds.
func tryLoopbackVerify(permImg string) error {
	if runtime.GOOS != "linux" || os.Getuid() != 0 {
		return nil
	}

	var tb textbuf.Buffer
	mountDir := tb.Str(permImg).Str(".mount").String()
	if err := os.MkdirAll(mountDir, 0o700); err != nil {
		return nil
	}
	defer os.Remove(mountDir) //nolint:errcheck // cleanup

	if _, err := runExternalFn("mount", "-o", "loop,ro", permImg, mountDir); err != nil {
		slog.Warn("loopback mount failed, skipping mount verify")
		return nil
	}
	defer func() { _, _ = runExternalFn("umount", mountDir) }()

	dbFile := filepath.Join(mountDir, "ze", "database.zefs")
	info, err := os.Stat(dbFile)
	if err != nil {
		return fmt.Errorf("loopback verify: ze/database.zefs not found: %w", err)
	}
	if info.Size() == 0 {
		return fmt.Errorf("loopback verify: ze/database.zefs is empty")
	}
	return nil
}

// extractPartition reads size bytes at offset from imgPath.
func extractPartition(imgPath string, offset, size int64) ([]byte, error) {
	f, err := os.Open(imgPath) //nolint:gosec // build-controlled path
	if err != nil {
		return nil, fmt.Errorf("open image: %w", err)
	}
	defer f.Close() //nolint:errcheck // read-only

	buf := make([]byte, size)
	n, err := f.ReadAt(buf, offset)
	if n == int(size) {
		return buf, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %d bytes at offset %d (got %d): %w", size, offset, n, err)
	}
	return buf[:n], nil
}

// writePartition writes data at offset into imgPath without truncating.
func writePartition(imgPath string, data []byte, offset int64) error {
	f, err := os.OpenFile(imgPath, os.O_WRONLY, 0) //nolint:gosec // build-controlled path
	if err != nil {
		return fmt.Errorf("open image for write: %w", err)
	}
	closed := false
	defer func() {
		if !closed {
			f.Close() //nolint:errcheck // cleanup on error path
		}
	}()

	if _, err := f.WriteAt(data, offset); err != nil {
		return fmt.Errorf("write %d bytes at offset %d: %w", len(data), offset, err)
	}
	closed = true
	return f.Close()
}
