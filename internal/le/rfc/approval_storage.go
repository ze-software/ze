// Design: docs/contributing/committing.md -- pending verdicts survive competing writers
// Related: approve.go -- upsert; ../commit/script.go -- post-publication exact-row cleanup
package rfc

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/gofrs/flock"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// withApprovalLedgerLock serializes the entire read/transform/replace mutation.
// Every writer of full MUST use this boundary. The sidecar MUST remain named
// after unlock: removing it could split waiting writers across distinct inodes.
// An existing sidecar MUST be a regular file, never a directory or symlink.
// A failed lock refuses the mutation; unsupported locking has no fallback.
func withApprovalLedgerLock(full string, mutate func() error) (result error) {
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		return err
	}
	lockPath := approvalLedgerLockPath(full)
	info, err := os.Lstat(lockPath)
	if err == nil {
		if !info.Mode().IsRegular() {
			var problem textbuf.Buffer
			return opererr.New(problem.Str("approval lock ").Str(lockPath).Str(" is not a regular file").String(),
				"pending approvals require a stable lock file; inspect this entry before changing it",
				"ls -ld -- '"+strings.ReplaceAll(lockPath, "'", "'\\''")+"'")
		}
	} else {
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	// flock defaults to a read-only open, which can successfully lock a directory
	// on Linux. A read-write open also refuses a directory appearing after Lstat.
	lock := flock.New(lockPath, flock.SetFlag(os.O_CREATE|os.O_RDWR), flock.SetPermissions(0o600))
	if err := lock.Lock(); err != nil {
		return err
	}
	defer func() { result = errors.Join(result, lock.Unlock()) }()
	return mutate()
}

func approvalLedgerLockPath(full string) string {
	return full + ".lock"
}

// writeApprovalLedger writes a unique sibling and atomically replaces full.
// The caller MUST hold full's stable process lock through this call. A failed
// stage leaves the earlier ledger intact; only this call's sibling is removed.
func writeApprovalLedger(full string, body []byte) (result error) {
	mode := os.FileMode(0o600)
	info, err := os.Stat(full)
	if err == nil {
		mode = info.Mode().Perm()
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	staged, err := os.CreateTemp(filepath.Dir(full), ".approval-*.tmp")
	if err != nil {
		return err
	}
	name := staged.Name()
	defer func() {
		if err := os.Remove(name); err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				result = errors.Join(result, err)
			}
		}
	}()
	if err := staged.Chmod(mode); err != nil {
		return errors.Join(err, staged.Close())
	}
	if _, err := staged.Write(body); err != nil {
		return errors.Join(err, staged.Close())
	}
	if err := staged.Close(); err != nil {
		return err
	}
	return os.Rename(name, full)
}

// PruneApprovals removes only the exact whole lines a published commit spent.
// It is safe for concurrent use with ApproveInto and other prune calls. The
// native caller MUST bind file to its prepared source kind and namespace, and
// MUST call only after successful publication. Missing ledgers remain missing;
// a renewed row with different text survives even when its unit is the same.
func PruneApprovals(root string, file ApprovalFile, dropped []string) error {
	removed := make(map[string]struct{}, len(dropped))
	for _, row := range dropped {
		if row == "" {
			return errors.New("approval cleanup: an exact consumed row is required")
		}
		if strings.ContainsAny(row, "\r\n") {
			return errors.New("approval cleanup: a consumed row must be one whole line")
		}
		removed[row] = struct{}{}
	}
	full := filepath.Join(root, filepath.FromSlash(file.Path))
	return withApprovalLedgerLock(full, func() error {
		content, err := os.ReadFile(full) //nolint:gosec // native file kind and captured namespace select this checkout's approval ledger
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		var page textbuf.Buffer
		for line := range strings.SplitAfterSeq(string(content), "\n") {
			if _, spent := removed[strings.TrimSuffix(line, "\n")]; spent {
				continue
			}
			page.Str(line)
		}
		body := []byte(page.String())
		if bytes.Equal(content, body) {
			return nil
		}
		return writeApprovalLedger(full, body)
	})
}
