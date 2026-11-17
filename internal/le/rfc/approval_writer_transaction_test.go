//go:build linux || darwin

// Design: docs/contributing/committing.md -- one process lock spans an approval edit
// Related: approve.go -- upsert; internal/le/commit/script.go -- exact consumed-row prune
package rfc

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/gofrs/flock"
)

// Goal: the natural shared mutation boundary owns its process lock while
// editing rows. Method: a distinct OS-lock handle must be denied inside the
// edit callback; no sleep, scheduler race or process-local mutex can satisfy it.
func TestApprovalLedgerMutationOwnsProcessLock(t *testing.T) {
	full := filepath.Join(t.TempDir(), "pending.md")
	err := withApprovalLedgerLock(full, func() error {
		other := flock.New(approvalLedgerLockPath(full), flock.SetPermissions(0o600))
		acquired, err := other.TryLock()
		if err != nil {
			return err
		}
		if acquired {
			if err := other.Unlock(); err != nil {
				return err
			}
			return errors.New("approval mutation callback ran without its shared process lock")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
