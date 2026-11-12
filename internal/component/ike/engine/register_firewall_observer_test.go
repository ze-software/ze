package engine

import (
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/internal/component/ike/dataplane"
)

// espFormHolderDP is a backend whose dual-form receiver can be held off, as the
// XFRM backend is: it counts holds and releases.
type espFormHolderDP struct {
	bypassDP
	holds, releases int
}

func (d *espFormHolderDP) HoldESPFormFastPath() func() {
	d.holds++
	return func() { d.releases++ }
}

var _ dataplane.ESPFormFirewallHolder = (*espFormHolderDP)(nil)

// TestHoldESPFormFastPathForCommit calls the observer directly over each backend
// the engine's process can hold.
//
// VALIDATES: with a holder loaded, the observer takes one hold and its after
// releases it, once, whatever error the commit answered; with no dataplane loaded,
// or a backend that is not a holder, the observer takes nothing and answers a nil
// after.
// PREVENTS: a commit that lands with the fast path still rewriting bare ESP past
// the rule it adds, and a hold never released, which keeps bare ESP on the
// complete path for the life of the process.
func TestHoldESPFormFastPathForCommit(t *testing.T) {
	t.Run("a holder", func(t *testing.T) {
		dp := &espFormHolderDP{}
		loadESPFormBackend(t, dp)
		after := holdESPFormFastPathForCommit()
		if after == nil {
			t.Fatal("the observer answered no after over a holder")
		}
		if dp.holds != 1 || dp.releases != 0 {
			t.Fatalf("before the commit: holds %d, releases %d; want 1 and 0", dp.holds, dp.releases)
		}
		after(nil)
		if dp.holds != 1 || dp.releases != 1 {
			t.Fatalf("after the commit: holds %d, releases %d; want 1 and 1", dp.holds, dp.releases)
		}
	})
	t.Run("no dataplane", func(t *testing.T) {
		loadESPFormBackend(t, nil)
		if after := holdESPFormFastPathForCommit(); after != nil {
			t.Fatal("the observer answered an after with no dataplane loaded")
		}
	})
	t.Run("not a holder", func(t *testing.T) {
		loadESPFormBackend(t, &bypassDP{})
		if after := holdESPFormFastPathForCommit(); after != nil {
			t.Fatal("the observer answered an after over a backend that is not a holder")
		}
	})
}

// observerFirewallBackend is a firewall backend that reads the holder's counts
// while Apply runs.
type observerFirewallBackend struct {
	dp                      *espFormHolderDP
	holdsSeen, releasesSeen int
	applied                 int
}

func (b *observerFirewallBackend) Apply([]firewall.Table) error {
	b.applied++
	b.holdsSeen, b.releasesSeen = b.dp.holds, b.dp.releases
	return nil
}
func (b *observerFirewallBackend) ListTables() ([]firewall.Table, error) { return nil, nil }
func (b *observerFirewallBackend) GetCounters(string) ([]firewall.ChainCounters, error) {
	return nil, nil
}
func (b *observerFirewallBackend) Close() error { return nil }

// observerBackendSeq numbers the firewall backends this file registers: the
// firewall registry keeps every name for the life of the process, so a name that
// repeats under -count fails RegisterBackend.
var observerBackendSeq atomic.Uint64

// TestFirewallCommitHoldsESPFormFastPath commits a firewall reconcile through
// firewall.ApplyAll with a holder loaded as the IPsec dataplane.
//
// VALIDATES: the observer this package's init() registers is reached by every
// commit: the hold is taken before Backend.Apply runs, Apply sees it held and not
// released, and it is released once Apply returns.
// PREVENTS: an observer written and never registered, which leaves Ze's own
// commits racing the fast path.
func TestFirewallCommitHoldsESPFormFastPath(t *testing.T) {
	dp := &espFormHolderDP{}
	loadESPFormBackend(t, dp)

	fw := &observerFirewallBackend{dp: dp}
	name := "ike-observer-test-" + strconv.FormatUint(observerBackendSeq.Add(1), 10)
	if err := firewall.RegisterBackend(name, func() (firewall.Backend, error) { return fw, nil }); err != nil {
		t.Fatalf("firewall.RegisterBackend: %v", err)
	}
	if err := firewall.LoadBackend(name); err != nil {
		t.Fatalf("firewall.LoadBackend: %v", err)
	}
	const owner = "ike-observer-test"
	t.Cleanup(func() {
		if err := firewall.RegisterTables(owner, nil); err != nil {
			t.Errorf("withdraw the test table: %v", err)
		}
		if err := firewall.CloseBackend(); err != nil {
			t.Errorf("close the firewall backend: %v", err)
		}
	})
	if err := firewall.RegisterTables(owner, []firewall.Table{{Name: "ze_ike_observer_test", Family: firewall.FamilyInet}}); err != nil {
		t.Fatalf("firewall.RegisterTables: %v", err)
	}

	if err := firewall.ApplyAll(); err != nil {
		t.Fatalf("firewall.ApplyAll: %v", err)
	}
	if fw.applied != 1 {
		t.Fatalf("Backend.Apply ran %d times, want 1", fw.applied)
	}
	if fw.holdsSeen != 1 || fw.releasesSeen != 0 {
		t.Fatalf("during Apply: holds %d, releases %d; want 1 and 0", fw.holdsSeen, fw.releasesSeen)
	}
	if dp.holds != 1 || dp.releases != 1 {
		t.Fatalf("after ApplyAll: holds %d, releases %d; want 1 and 1", dp.holds, dp.releases)
	}
}
