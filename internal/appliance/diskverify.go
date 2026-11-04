// Design: docs/architecture/appliance/on-device-installer.md -- build-side inject verify

package appliance

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
)

// verifyInject runs all build-side verification tiers on the perm image:
// content read-back (mandatory), e2fsck structural check, loopback mount
// (Linux root only). Logs and returns the first error found.
func verifyInject(ctx context.Context, permImg, dbPath string, execute func(context.Context, string, ...string) ([]byte, error)) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := verifyInjectedDB(permImg, dbPath); err != nil {
		slog.Error("inject verification failed", "error", err)
		return err
	}
	if err := verifyE2fsck(ctx, permImg, execute); err != nil {
		slog.Error("ext4 structural check failed", "error", err)
		return err
	}
	if err := tryLoopbackVerify(ctx, permImg, execute); err != nil {
		slog.Error("mount verification failed", "error", err)
		return err
	}
	return ctx.Err()
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
func verifyE2fsck(ctx context.Context, permImg string, execute func(context.Context, string, ...string) ([]byte, error)) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	e2fsck := e2fsE2fsck
	if e2fsck == "" {
		slog.Warn("e2fsck not found, skipping structural check")
		return nil
	}
	// Still stat it: the path was resolved at process start, and a test (or a
	// package removal) can leave it pointing at something that is no longer there.
	if _, statErr := os.Stat(e2fsck); statErr != nil {
		slog.Warn("e2fsck not found, skipping structural check")
		return nil //nolint:nilerr // intentional: skip when e2fsck not installed
	}
	if _, err := runInjectionCommand(ctx, execute, e2fsck, "-f", "-n", permImg); err != nil {
		return fmt.Errorf("ext4 structural check failed: %w", err)
	}
	return nil
}

// tryLoopbackVerify mounts the perm image read-only and confirms
// ze/database.zefs is present. Only runs on Linux with root; silently
// skips on other platforms or non-root builds.
func tryLoopbackVerify(
	ctx context.Context,
	permImg string,
	execute func(context.Context, string, ...string) ([]byte, error),
) (resultErr error) {
	if err := ctx.Err(); err != nil {
		return err
	}
	if runtime.GOOS != "linux" {
		return nil
	}
	if os.Getuid() != 0 {
		return nil
	}

	mountDir, err := os.MkdirTemp(filepath.Dir(permImg), "ze-inject-mount-")
	if err != nil {
		if cause := ctx.Err(); cause != nil {
			return cause
		}
		slog.Warn("cannot create private mount verification directory, skipping mount verify", "error", err)
		return nil
	}
	defer func() {
		if err := os.Remove(mountDir); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("remove private mount directory %s: %w", mountDir, err))
		}
	}()

	var mountErr error
	// An interrupted mount can already have taken effect. Cleanup MUST be
	// installed before attempting it and MUST finish before releasing mountDir.
	defer func() {
		_, err := runInjectionCommand(context.WithoutCancel(ctx), execute, "umount", mountDir)
		if err == nil {
			return
		}
		cleanupErr := fmt.Errorf("unmount private injection verification directory %s: %w", mountDir, err)
		if mountErr == nil {
			resultErr = errors.Join(resultErr, cleanupErr)
			return
		}
		if errors.Is(mountErr, context.Canceled) {
			resultErr = errors.Join(resultErr, cleanupErr)
			return
		}
		if errors.Is(mountErr, context.DeadlineExceeded) {
			resultErr = errors.Join(resultErr, cleanupErr)
			return
		}
		// A non-interrupted refusal remains the existing optional check.
		// The private directory was never a successful mount target.
		slog.Warn("cleanup after refused optional mount", "error", cleanupErr)
	}()

	_, mountErr = runInjectionCommand(ctx, execute, "mount", "-o", "loop,ro", permImg, mountDir)
	if mountErr != nil {
		if errors.Is(mountErr, context.Canceled) {
			return mountErr
		}
		if errors.Is(mountErr, context.DeadlineExceeded) {
			return mountErr
		}
		slog.Warn("loopback mount failed, skipping mount verify", "error", mountErr)
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	dbFile := filepath.Join(mountDir, "ze", "database.zefs")
	info, err := os.Stat(dbFile)
	if err != nil {
		return fmt.Errorf("loopback verify: ze/database.zefs not found: %w", err)
	}
	if info.Size() == 0 {
		return fmt.Errorf("loopback verify: ze/database.zefs is empty")
	}
	return ctx.Err()
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
