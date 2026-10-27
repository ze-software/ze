// Design: docs/architecture/testing/runner-architecture.md — cross-process exclusive groups.

package runner

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// leaseExclusiveGroup serializes participating same-user runners sharing /tmp.
// The caller MUST close the returned file only after the test and its cleanup
// finish. Lock files MUST NOT be unlinked, even after release: an existing waiter
// still names that inode, so replacing it would admit a second holder.
func leaseExclusiveGroup(ctx context.Context, group string) (*os.File, error) {
	lease, err := leaseExclusiveGroupIn(ctx, portLockDirectory(), group)
	if err != nil {
		return nil, fmt.Errorf("acquire exclusive group %q: %w", group, err)
	}
	return lease, nil
}

func leaseExclusiveGroupIn(ctx context.Context, lockDir, group string) (*os.File, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(lockDir, 0o700); err != nil {
		return nil, fmt.Errorf("create exclusive group lock directory: %w", err)
	}
	if err := checkPortLockDir(lockDir); err != nil {
		return nil, err
	}
	// Hash the complete declared name so separators and long names cannot escape
	// the protected directory or exceed a filesystem component's length limit.
	digest := sha256.Sum256([]byte(group))
	path := filepath.Join(lockDir, "group-"+hex.EncodeToString(digest[:])+".lock")
	lease, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0o600) //nolint:gosec // Hashed group name in the checked effective-UID lock directory; links are refused.
	if err != nil {
		return nil, fmt.Errorf("open exclusive group lock %s: %w", path, err)
	}
	if err := checkExclusiveGroupFile(lease); err != nil {
		return nil, errors.Join(err, lease.Close())
	}
	if err := waitExclusiveGroup(ctx, lease); err != nil {
		return nil, errors.Join(err, lease.Close())
	}
	return lease, nil
}

// checkExclusiveGroupFile refuses objects that cannot safely retain the lock
// inode. O_NONBLOCK above prevents a planted FIFO from blocking before this check.
func checkExclusiveGroupFile(lease *os.File) error {
	info, err := lease.Stat()
	if err != nil {
		return fmt.Errorf("inspect exclusive group lock %s: %w", lease.Name(), err)
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fmt.Errorf("exclusive group lock %s: no owner information on this platform", lease.Name())
	}
	if info.Mode().IsRegular() && int(stat.Uid) == os.Geteuid() && info.Mode().Perm()&0o022 == 0 {
		return nil
	}
	return fmt.Errorf("exclusive group lock %s is owned by uid %d with mode %s; "+
		"it must be a regular file owned by uid %d without group or other write permission: stop participating runners, remove it and rerun",
		lease.Name(), stat.Uid, info.Mode(), os.Geteuid())
}

// waitExclusiveGroup polls nonblocking flock until acquisition or cancellation.
// The runner context bounds the wait, not a new test deadline. No goroutine can
// remain stuck in a blocking flock after the runner has been canceled.
func waitExclusiveGroup(ctx context.Context, lease *os.File) error {
	var retry *time.Timer
	defer func() {
		if retry != nil {
			retry.Stop()
		}
	}()
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		err := syscall.Flock(int(lease.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			return nil
		}
		if !isWouldBlock(err) && !errors.Is(err, syscall.EINTR) {
			return fmt.Errorf("flock exclusive group %s: %w", lease.Name(), err)
		}
		if retry == nil {
			retry = time.NewTimer(20 * time.Millisecond)
		} else {
			retry.Reset(20 * time.Millisecond)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-retry.C:
		}
	}
}
