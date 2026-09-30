// VALIDATES: an operator who writes `unmatched discard` either gets the discard
// catch-all installed or is told the configuration was not applied. A catch-all that
// cannot be installed, because the install failed, because no dataplane is loaded, or
// because the backend holds no entry bound to no interface, fails the apply rather than
// leaving the kernel to pass what the operator asked to stop. The same condition is
// refused at config verify when the loaded backend says ahead of time that it cannot
// hold the entry.
// PREVENTS: the fail-open catch-all of RULINGS R29, where installUnmatched logged a
// failed DISCARD install as a warning and applyConfig reported the configuration applied.

package engine

import (
	"errors"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// catchAllRefusingDP is a backend that declares, ahead of any install, that it cannot
// hold an SPD entry bound to no interface, as the VPP backend does.
type catchAllRefusingDP struct {
	bypassDP
}

func (catchAllRefusingDP) CatchAllSupported() error {
	return dataplane.ErrNotSupported
}

// catchAllDP is a backend that declares it can hold the catch-all.
type catchAllDP struct {
	bypassDP
}

func (catchAllDP) CatchAllSupported() error { return nil }

var failClosedBackendSeq atomic.Uint64

// loadFailClosedBackend makes dp the active dataplane for the test, the way the
// daemon's runEngine does with dataplane.Load, and closes it at cleanup.
func loadFailClosedBackend(t *testing.T, dp dataplane.Dataplane) {
	t.Helper()
	var name strings.Builder
	name.WriteString("unmatched-failclosed-")
	name.WriteString(t.Name())
	name.WriteByte('-')
	name.WriteByte(byte('a' + failClosedBackendSeq.Add(1)%26))
	if err := dataplane.Register(name.String(), func() (dataplane.Dataplane, error) { return dp, nil }); err != nil {
		t.Fatalf("register backend: %v", err)
	}
	if err := dataplane.Load(name.String()); err != nil {
		t.Fatalf("load backend: %v", err)
	}
	t.Cleanup(func() {
		if err := dataplane.CloseBackend(); err != nil {
			t.Errorf("close backend: %v", err)
		}
	})
}

// A failed DISCARD install fails the apply, on both deliveries, and the error names the
// catch-all. A failed BYPASS install does not: with no entry the kernel passes the
// packet, which is what a bypass entry would have done.
func TestUnmatchedDiscardInstallFailureFailsTheApply(t *testing.T) {
	loadFailClosedBackend(t, &bypassDP{installErr: errors.New("operation not permitted")})

	for _, phase := range []applyPhase{applyStartup, applyReload} {
		state := applyTestState(t)
		cfg := testIPsecConfig(applyTestPeer("127.0.0.1"))
		cfg.Unmatched = dataplane.SPActionDiscard
		err := state.applyConfig(cfg, phase)
		if err == nil {
			t.Fatalf("phase %d: a discard catch-all that failed to install was reported applied", phase)
		}
		if !strings.Contains(err.Error(), "catch-all") {
			t.Errorf("phase %d: error %q does not name the catch-all", phase, err)
		}

		bypass := applyTestState(t)
		cfg = testIPsecConfig(applyTestPeer("127.0.0.1"))
		cfg.Unmatched = dataplane.SPActionBypass
		if err := bypass.applyConfig(cfg, phase); err != nil {
			t.Errorf("phase %d: a failed bypass catch-all failed the apply: %v", phase, err)
		}
	}
}

// A DISCARD with no dataplane, or with a backend that cannot hold the entry, is an
// error from installUnmatched. A BYPASS in the same place is not.
func TestUnmatchedDiscardWithNoEnforcingDataplaneIsAnError(t *testing.T) {
	log := slogutil.Logger("test")
	if err := installUnmatched(nil, dataplane.SPActionDiscard, log); err == nil {
		t.Error("a discard catch-all with no dataplane loaded returned no error")
	}
	unsupported := &bypassDP{installErr: dataplane.ErrNotSupported}
	if err := installUnmatched(unsupported, dataplane.SPActionDiscard, log); err == nil {
		t.Error("a discard catch-all on a backend without XFRM returned no error")
	}
	if err := installUnmatched(nil, dataplane.SPActionBypass, log); err != nil {
		t.Errorf("a bypass catch-all with no dataplane returned %v", err)
	}
	if err := installUnmatched(unsupported, dataplane.SPActionBypass, log); err != nil {
		t.Errorf("a bypass catch-all on a backend without XFRM returned %v", err)
	}
	installed := &bypassDP{}
	if err := installUnmatched(installed, dataplane.SPActionDiscard, log); err != nil {
		t.Fatalf("a discard catch-all that installed returned %v", err)
	}
	if len(installed.installed) != 6 {
		t.Fatalf("installed %d policies, want 6", len(installed.installed))
	}
}

// Config verify refuses `unmatched discard` where the loaded dataplane cannot enforce
// it, and accepts it where it can. It never refuses a bypass.
func TestUnmatchedDiscardRefusedAtVerifyWhereItCannotBeEnforced(t *testing.T) {
	cases := []struct {
		name    string
		dp      dataplane.Dataplane
		action  dataplane.SPAction
		refused bool
	}{
		{"no dataplane, discard", nil, dataplane.SPActionDiscard, true},
		{"backend refuses a node-wide entry, discard", &catchAllRefusingDP{}, dataplane.SPActionDiscard, true},
		{"backend declares nothing, discard", &bypassDP{}, dataplane.SPActionDiscard, true},
		{"backend holds the entry, discard", &catchAllDP{}, dataplane.SPActionDiscard, false},
		{"no dataplane, bypass", nil, dataplane.SPActionBypass, false},
		{"backend refuses a node-wide entry, bypass", &catchAllRefusingDP{}, dataplane.SPActionBypass, false},
	}
	for _, tc := range cases {
		err := verifyUnmatchedEnforceable(tc.dp, tc.action)
		if tc.refused && err == nil {
			t.Errorf("%s: accepted", tc.name)
		}
		if !tc.refused && err != nil {
			t.Errorf("%s: refused: %v", tc.name, err)
		}
	}
}
