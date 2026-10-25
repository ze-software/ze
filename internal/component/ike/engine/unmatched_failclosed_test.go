// VALIDATES: an operator gets the catch-all `unmatched` names installed, discard or
// bypass, or is told the configuration was not applied. A catch-all that
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
	"net"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// catchAllRefusingDP is a backend that declares, ahead of any install, that it cannot
// hold an SPD entry bound to no interface, as the VPP backend does.
type catchAllRefusingDP struct {
	bypassDP
}

func (catchAllRefusingDP) CatchAllSupported() error {
	return dataplane.ErrNoNodeWideSPD
}

// noXFRMDP is a backend on a platform without XFRM: it declares the catch-all
// unsupported, as xfrm_other.go does, and every install fails.
type noXFRMDP struct {
	bypassDP
}

func (noXFRMDP) CatchAllSupported() error { return dataplane.ErrNotSupported }

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
	// The whole sequence number, never a letter taken modulo 26: the registry keeps
	// every name for the life of the process, so a name that repeats under -count fails
	// Register on the 27th call.
	name.WriteString(strconv.FormatUint(failClosedBackendSeq.Add(1), 10))
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

// A failed catch-all install fails the apply, on both deliveries and for both
// dispositions, and the error names the catch-all. A failed BYPASS used to be logged and
// tolerated, which left an inbound packet meeting no SPD-I entry at all (RFC 4301 Section
// 5.2 step 3b; owner decision 2026-10-20, Q7 A).
func TestUnmatchedInstallFailureFailsTheApply(t *testing.T) {
	loadFailClosedBackend(t, &bypassDP{installErr: errors.New("operation not permitted")})

	// Each engine runs in its own subtest, so one is live at a time (applyTestState).
	for _, phase := range []applyPhase{applyStartup, applyReload} {
		for _, action := range []dataplane.SPAction{dataplane.SPActionDiscard, dataplane.SPActionBypass} {
			t.Run(strconv.Itoa(int(phase))+"-"+strconv.Itoa(int(action)), func(t *testing.T) {
				state := applyTestState(t)
				cfg := testIPsecConfig(applyTestPeer("127.0.0.1"))
				cfg.Unmatched = action
				err := state.applyConfig(cfg, phase)
				if err == nil {
					t.Fatalf("phase %d action %d: a catch-all that failed to install was reported applied", phase, action)
				}
				if !strings.Contains(err.Error(), "catch-all") {
					t.Errorf("phase %d action %d: error %q does not name the catch-all", phase, action, err)
				}
			})
		}
	}
}

// A reload refused because its DISCARD catch-all failed to install changes nothing the
// running configuration put in place. applyConfig installs the catch-all before the
// operator's SPD entries and the cookie threshold, so the refused configuration's
// threshold never takes effect, its SPD entry never reaches the backend or the engine's
// record, and the running configuration's entry is not removed. Method: apply a running
// discard configuration with one bypass entry and threshold 7 on a backend that holds
// the catch-all, make every later install fail, then apply a configuration with another
// entry and threshold 99, and compare the threshold, installedSPD and the backend's live
// entries with what the running apply left.
func TestUnmatchedDiscardRefusalLeavesThresholdAndSPDEntriesUnchanged(t *testing.T) {
	dp := &catchAllDP{}
	loadFailClosedBackend(t, dp)
	withCookieThreshold(t, CookieThreshold())
	state := applyTestState(t)

	running := testIPsecConfig(applyTestPeer("127.0.0.1"))
	running.Unmatched = dataplane.SPActionDiscard
	running.CookieThreshold = 7
	running.Policies = map[string]ipsec.SPDPolicy{"pass-mgmt": failClosedBypassEntry(t, "pass-mgmt", "192.0.2.0/24")}
	if err := state.applyConfig(running, applyReload); err != nil {
		t.Fatalf("running apply: %v", err)
	}
	runningEntries := spdPolicyParams(running.Policies["pass-mgmt"])
	for _, sp := range runningEntries {
		if _, live := dp.live[keyOf(sp)]; !live {
			t.Fatalf("the running apply did not install its bypass entry %+v", sp)
		}
	}
	removed := len(dp.removed)

	dp.installErr = errors.New("operation not permitted")
	refused := testIPsecConfig(applyTestPeer("127.0.0.1"))
	refused.Unmatched = dataplane.SPActionDiscard
	refused.CookieThreshold = 99
	refused.Policies = map[string]ipsec.SPDPolicy{"pass-other": failClosedBypassEntry(t, "pass-other", "203.0.113.0/24")}
	if err := state.applyConfig(refused, applyReload); err == nil {
		t.Fatal("a reload whose discard catch-all failed to install was reported applied")
	}

	if got := CookieThreshold(); got != 7 {
		t.Errorf("the refused configuration's cookie threshold is in force: %d, want 7", got)
	}
	if _, kept := state.installedSPD["pass-mgmt"]; !kept {
		t.Errorf("the engine no longer records the running entry: %v", state.installedSPD)
	}
	if _, taken := state.installedSPD["pass-other"]; taken {
		t.Errorf("the engine records the refused configuration's entry: %v", state.installedSPD)
	}
	if len(dp.removed) != removed {
		t.Errorf("the refused apply removed %d SPD entries: %+v", len(dp.removed)-removed, dp.removed[removed:])
	}
	for _, sp := range runningEntries {
		if _, live := dp.live[keyOf(sp)]; !live {
			t.Errorf("the running bypass entry %+v is no longer installed", sp)
		}
	}
	for _, sp := range spdPolicyParams(refused.Policies["pass-other"]) {
		if _, live := dp.live[keyOf(sp)]; live {
			t.Errorf("the refused configuration's entry %+v is installed", sp)
		}
	}
}

// failClosedBypassEntry is an operator bypass entry for local prefix local, in both
// directions, any protocol and port.
func failClosedBypassEntry(t *testing.T, name, local string) ipsec.SPDPolicy {
	t.Helper()
	_, localPrefix, err := net.ParseCIDR(local)
	if err != nil {
		t.Fatalf("parse local prefix: %v", err)
	}
	_, remotePrefix, err := net.ParseCIDR("198.51.100.0/24")
	if err != nil {
		t.Fatalf("parse remote prefix: %v", err)
	}
	return ipsec.SPDPolicy{
		Name:         name,
		Action:       dataplane.SPActionBypass,
		Order:        1000,
		Direction:    ipsec.SPDDirBoth,
		Protocol:     protoUDP,
		LocalPrefix:  localPrefix,
		LocalPort:    ipsec.AnyPort(),
		RemotePrefix: remotePrefix,
		RemotePort:   ipsec.AnyPort(),
	}
}

// A catch-all with no dataplane, or on a platform without XFRM, is an error from
// installUnmatched for both dispositions. The one bypass that is neither installed nor
// refused is on a backend that declares it holds no entry bound to no interface (VPP),
// the gap rfc/short/rfc4301.md keeps open on RFC4301-5.2-1 and RFC4301-5.2-9.
func TestUnmatchedWithNoEnforcingDataplaneIsAnError(t *testing.T) {
	log := slogutil.Logger("test")
	unsupported := &bypassDP{installErr: dataplane.ErrNotSupported}
	for _, action := range []dataplane.SPAction{dataplane.SPActionDiscard, dataplane.SPActionBypass} {
		if err := installUnmatched(nil, action, log); err == nil {
			t.Errorf("action %d: a catch-all with no dataplane loaded returned no error", action)
		}
		if err := installUnmatched(unsupported, action, log); err == nil {
			t.Errorf("action %d: a catch-all on a backend without XFRM returned no error", action)
		}
	}
	noXFRM := &noXFRMDP{bypassDP{installErr: dataplane.ErrNotSupported}}
	if err := installUnmatched(noXFRM, dataplane.SPActionBypass, log); err == nil {
		t.Error("a bypass catch-all on a platform that declares no XFRM returned no error")
	}
	vpp := &catchAllRefusingDP{}
	if err := installUnmatched(vpp, dataplane.SPActionBypass, log); err != nil {
		t.Errorf("a bypass on a backend with no node-wide SPD returned %v", err)
	}
	if len(vpp.installed) != 0 {
		t.Errorf("a backend with no node-wide SPD was asked to install %d catch-all entries", len(vpp.installed))
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
