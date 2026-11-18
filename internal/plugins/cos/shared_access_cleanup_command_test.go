// Design: docs/architecture/traffic/cos-dynamic.md -- the reported repair names a real cleanup command
// Related: diagnostic_shared_test.go -- actual registration and native failure fixture

//go:build ze_l2tp

package cos

import (
	"testing"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// Goal: the diagnostic's exact repair command reaches the retained native owner.
// Method: fail its final restore, verify the visible registration, then execute
// that command through the actual plugin command handler. One confirmed native
// restore must clear its warning. This is not a daemon or RPC-surface test.
func TestCoSLegacySharedReportedCleanupCommandRepairsOriginalOwner(t *testing.T) {
	f := newRegisteredCoSReportFixture(t)
	f.up(60218, sharedCoSTestFirst)
	f.requireMaps(f.shared)
	f.failWrites = 1
	f.down(60218)
	entry := f.requireProblem(report.Warnings(), "doctor-cos-cleanup-unconfirmed", 1)
	if entry.Fix != "ze request class-of-service cleanup" {
		t.Fatalf("retained cleanup repair=%q, want the registered cleanup command", entry.Fix)
	}
	registration := registry.Lookup(Name)
	if registration == nil {
		t.Fatal("the class-of-service command owner is not registered")
	}
	const command = "request class-of-service cleanup"
	declarations := 0
	for _, declaration := range registration.Commands {
		if declaration.Name != command {
			continue
		}
		declarations++
		if declaration.Hidden {
			t.Fatal("the diagnostic repair command is hidden from the operator")
		}
	}
	if declarations != 1 {
		t.Fatalf("visible cleanup declarations=%d, want one", declarations)
	}
	writes := f.writes
	status, _, err := executeCommand("diagnostic-recovery", command, nil, "127.0.0.1")
	if err != nil {
		t.Fatalf("the reported cleanup command did not recover its original owner: %v", err)
	}
	if status != rpc.StatusDone {
		t.Fatalf("the reported cleanup command returned %q, want %q", status, rpc.StatusDone)
	}
	if f.writes != writes+1 {
		t.Fatalf("the reported repair attempted %d native writes, want one", f.writes-writes)
	}
	f.requireMaps(f.original)
	if entries := report.Warnings(); len(entries) != 0 {
		t.Fatalf("confirmed command recovery kept the original warning: %+v", entries)
	}
}
