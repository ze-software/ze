// Design: docs/architecture/traffic/cos-dynamic.md -- command/departure custody
// Related: shared_access_command_report_test.go -- registered SDK command fixture

//go:build ze_l2tp

package cos

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// Goal: cleanup counts an owner recovered only after both its failed command
// compensation and the already requested final departure complete.
// Method: fail apply plus compensation, fail Down's rollback, then drive the
// registered cleanup once. It must confirm the final baseline, remove the
// departed session, clear the same warning and count that owner once. One final
// restore or compensation followed by final restore are both valid; custody
// and actual final maps decide recovery, not a fixed implementation's writes.
func TestCoSLegacySharedCleanupFinishesFailedCommandDeparture(t *testing.T) {
	f := newCoSCommandReportFixture(t)
	registeredCoSCleanupReviewUp(t, f.bus, 63233)
	token, err := f.command("prepare", 63233, sharedCoSTestDifferent, "")
	if err != nil {
		t.Fatal(err)
	}
	f.failures = []error{errors.New("departing command apply failed"), errors.New("departing command compensation failed")}
	if _, err := f.command("apply", 0, "", token); err == nil {
		t.Fatal("failed apply and compensation were acknowledged")
	}
	subject := requireCoSCommandWarning(t, "", "departing command apply failed", "departing command compensation failed")
	f.failures = []error{errors.New("departing command rollback failed")}
	registeredCoSCleanupReviewDown(t, f.bus, 63233)
	requireCoSCommandWarning(t, subject, "departing command rollback failed")
	key := fixtureCoSKey(sharedCoSTestTunnel, 63233)
	if _, retained := sessionStore.Load(key); !retained {
		t.Fatal("failed Down lost its original session before recovery")
	}
	writes := f.writes
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	output, err := f.bridge.ExecuteCommand(ctx, "cos-departure-cleanup-test", cleanupCommandForReview, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if output.Status != rpc.StatusDone {
		t.Fatalf("departure cleanup failed: %s", output.Data)
	}
	var counts map[string]int
	if err := json.Unmarshal(output.Data, &counts); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"attempted", "recovered"} {
		count, present := counts[name]
		if !present {
			t.Fatalf("departure cleanup omitted %s: %s", name, output.Data)
		}
		if count != 1 {
			t.Fatalf("departure cleanup %s=%d, want one original owner", name, count)
		}
	}
	if f.writes-writes < 1 {
		t.Fatal("departure cleanup made no native restoration")
	}
	if f.writes-writes > 2 {
		t.Fatalf("departure cleanup replayed native restoration %d times", f.writes-writes)
	}
	f.requireMaps(f.original)
	if _, retained := sessionStore.Load(key); retained {
		t.Fatal("cleanup counted recovery while the original departure remained queued")
	}
	if len(report.Warnings()) != 0 {
		t.Fatal("completed original departure kept its warning")
	}
	writes = f.writes
	if err := f.cleanup(); err != nil {
		t.Fatal(err)
	}
	if f.writes != writes {
		t.Fatal("completed departure cleanup replayed native work")
	}
}
