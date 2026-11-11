// Design: docs/architecture/config/transaction-protocol.md -- failed undo recovery

package sdk

import (
	"errors"
	"testing"
)

// A failed undo can damage the candidate that still occupies tx.running. If
// restart is refused and this object survives, an equal candidate must repair
// that damage; a retried rollback must still target the preceding config.
func TestConfigTxRollbackLostRunningConfig(t *testing.T) {
	for _, next := range []string{"reapply-candidate", "retry-rollback", "ordinary-failure"} {
		t.Run(next, func(t *testing.T) {
			installed := "unconfigured"
			calls := 0
			var failure error
			tx := NewConfigTx("unconfigured", testTxParse, func(cfg string) error {
				calls++
				if failure != nil {
					if errors.Is(failure, ErrRunningConfigLost) {
						installed = "partially-running"
					}
					return failure
				}
				installed = cfg
				return nil
			})
			mustNoErr(t, "boot", tx.Configure(ownSection("a")))
			mustNoErr(t, "verify candidate", tx.Verify(ownSection("b")))
			mustNoErr(t, "apply candidate", tx.Apply())
			failure = errTestLost
			if next == "ordinary-failure" {
				failure = errors.New("undo refused without changing current state")
			}
			if err := tx.Rollback(); !errors.Is(err, failure) {
				t.Fatalf("rollback returned %v, want %v", err, failure)
			}
			failure = nil
			want, wantCalls := "b", 4
			if next == "retry-rollback" {
				mustNoErr(t, "retry rollback", tx.Rollback())
				want = "a"
			} else {
				mustNoErr(t, "verify same candidate", tx.Verify(ownSection("b")))
				mustNoErr(t, "reapply same candidate", tx.Apply())
			}
			if next == "ordinary-failure" {
				wantCalls = 3
			}
			if installed != want {
				t.Fatalf("installed %q, want %q: failed undo must not leave a no-op repair", installed, want)
			}
			if calls != wantCalls {
				t.Fatalf("install calls %d, want %d", calls, wantCalls)
			}
			mustNoErr(t, "verify repaired config", tx.Verify(ownSection(want)))
			mustNoErr(t, "apply repaired config", tx.Apply())
			if calls != wantCalls {
				t.Fatal("successful repair did not restore equal-config no-op behavior")
			}
		})
	}
}
