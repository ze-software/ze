// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- SPD dispositions
// Related: unmatched.go -- the catch-all this file proves first in and last out
// RFC: rfc/short/rfc4301.md -- Section 4.4.1, the nominal final entry

package engine

import (
	"bytes"
	"errors"
	"log/slog"
	"net"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// spdOp is one write a backend received, in the order it received it.
type spdOp struct {
	install bool
	policy  dataplane.SPParams
}

// lifecycleDP records every SPD write in order and holds the catch-all, so a test can
// replay the database the kernel would have held after each write. A selector in
// removeErr refuses its removal with that error and stays installed.
type lifecycleDP struct {
	bypassDP
	ops       []spdOp
	removeErr map[spKey]error
	onRemove  func()
}

// PolicyDeleteCommand names a removal in the kernel's `ip xfrm` form, the database
// lifecycleDP stands in for, as the XFRM backend does.
func (d *lifecycleDP) PolicyDeleteCommand(p dataplane.SPParams) (string, error) {
	return dataplane.XFRMPolicyDeleteCommand(&p), nil
}

func (d *lifecycleDP) CatchAllSupported() error { return nil }

func (d *lifecycleDP) InstallPolicy(p dataplane.SPParams) error {
	d.ops = append(d.ops, spdOp{install: true, policy: p})
	return d.bypassDP.InstallPolicy(p)
}

func (d *lifecycleDP) RemovePolicyParams(p dataplane.SPParams) error {
	if d.onRemove != nil {
		d.onRemove()
	}
	if err := d.removeErr[keyOf(p)]; err != nil {
		return err
	}
	d.ops = append(d.ops, spdOp{policy: p})
	return d.bypassDP.RemovePolicyParams(p)
}

// resetSPDLeft empties the process-wide SPD ledger before and after a test, so a
// removal another test refused does not decide whether this test's catch-all goes.
func resetSPDLeft(t *testing.T) {
	t.Helper()
	empty := func() {
		spdLeft.mu.Lock()
		clear(spdLeft.entries)
		spdLeft.mu.Unlock()
	}
	empty()
	t.Cleanup(empty)
}

// lifecyclePeer installs one Child SA's PROTECT entries on dp, as createFirstChildSA
// would, and registers its session in state, so the exit path has a Child SA to remove.
// The selector is narrower than the catch-all's wildcard, so the two never share a key.
// The session's owner goroutine is already done, as one that reached its stopCh is.
func lifecyclePeer(t *testing.T, state *ikeEngineState, dp dataplane.Dataplane) *ChildSA {
	t.Helper()
	child := testChildOnSelector(0x1001, 0x1002)
	_, local, err := net.ParseCIDR("10.1.0.0/24")
	if err != nil {
		t.Fatal(err)
	}
	_, remote, err := net.ParseCIDR("10.2.0.0/24")
	if err != nil {
		t.Fatal(err)
	}
	child.TSLocal = local
	child.TSRemote = remote
	for _, dir := range []dataplane.SADir{dataplane.SADirIn, dataplane.SADirOut} {
		for _, p := range childPolicies(child, dir) {
			if err := dp.InstallPolicy(p); err != nil {
				t.Fatalf("install the Child SA's %s entry: %v", dirName(dir), err)
			}
		}
	}
	done := make(chan struct{})
	close(done)
	ps := &PeerSession{peerName: testPolicyOwner, stopCh: make(chan struct{}), done: done}
	ps.setChildSA(child)
	state.peers[testPolicyOwner] = ps
	return child
}

// childKeys is the selector set of a Child SA's PROTECT entries, both directions.
func childKeys(child *ChildSA) map[spKey]bool {
	keys := map[spKey]bool{}
	for _, dir := range []dataplane.SADir{dataplane.SADirIn, dataplane.SADirOut} {
		for _, p := range childPolicies(child, dir) {
			keys[keyOf(p)] = true
		}
	}
	return keys
}

// lifecycleConfig is a `vpn ipsec` block with the `unmatched` leaf absent and one
// operator bypass entry.
func lifecycleConfig(t *testing.T) *ipsec.IPsecConfig {
	t.Helper()
	tree := config.NewTree()
	tree.GetOrCreateContainer("vpn").GetOrCreateContainer("ipsec")
	cfg, err := ipsec.ParseIPsecConfig(tree)
	if err != nil {
		t.Fatalf("parse a vpn ipsec block with no unmatched leaf: %v", err)
	}
	if cfg.Unmatched != dataplane.SPActionDiscard {
		t.Fatalf("the absent leaf parsed to action %d, want discard", cfg.Unmatched)
	}
	cfg.Policies = map[string]ipsec.SPDPolicy{
		"keep": failClosedBypassEntry(t, "keep", "192.0.2.0/24"),
	}
	return cfg
}

// catchAllKeys is the selector set of the catch-all, both families, every direction.
func catchAllKeys(t *testing.T) map[spKey]bool {
	t.Helper()
	keys := map[spKey]bool{}
	for _, family := range ikeBypassFamilies {
		policies, err := unmatchedPolicies(family, dataplane.SPActionDiscard)
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range policies {
			keys[keyOf(p)] = true
		}
	}
	return keys
}

// VALIDATES: Ze's SPD appears and disappears as one unit with the final entry under it.
// From the first apply to the exit, every database state in which Ze holds any entry
// other than the catch-all also holds the whole catch-all, both families, in, out and
// fwd; the catch-all is the first entry in and the last out, and after the exit Ze
// holds no entry.
// METHOD: apply a configuration with the `unmatched` leaf absent and one operator
// bypass entry on a backend that records every write, establish one peer's Child SA
// (its PROTECT entries, in and out), then run the shutdown every exit of runEngine
// runs, and replay the writes, checking the invariant after each one.
// PREVENTS: the IKE bypass installed at engine start with no catch-all beneath it, and
// a release that removed the catch-all while the bypass, an operator entry or a Child
// SA's PROTECT entry was still in force: in each window a packet met Ze's entries but
// not the final one.
//
// RFC requirement: RFC4301-4.4.1-3 positive -- with the `unmatched` leaf absent, the discard catch-all is installed before any other Ze SPD entry (the IKE bypass, an operator entry, a Child SA's PROTECT entries) and removed after all of them, so every database state that holds a Ze entry also holds the final entry that discards what it does not match.
func TestRFC4301FinalEntryIsUnderEveryZeSPDEntry(t *testing.T) {
	resetSPDLeft(t)
	dp := &lifecycleDP{}
	loadFailClosedBackend(t, dp)

	state := applyTestState(t)
	if err := state.applyConfig(lifecycleConfig(t), applyStartup); err != nil {
		t.Fatalf("apply: %v", err)
	}
	child := lifecyclePeer(t, state, dp)
	state.shutdown(state.log)

	finalEntry := catchAllKeys(t)
	protect := childKeys(child)
	live := map[spKey]dataplane.SPParams{}
	others, protectWrites := 0, 0
	for i, op := range dp.ops {
		key := keyOf(op.policy)
		if op.install {
			live[key] = op.policy
		} else {
			delete(live, key)
		}
		if !finalEntry[key] {
			others++
		}
		if protect[key] {
			protectWrites++
		}
		for k := range live {
			if finalEntry[k] {
				continue
			}
			for want := range finalEntry {
				entry, present := live[want]
				if !present {
					t.Fatalf("write %d: Ze holds entry %+v with the catch-all entry %+v absent", i, k, want)
				}
				if entry.Action != dataplane.SPActionDiscard {
					t.Fatalf("write %d: the catch-all entry %+v is action %d, want discard", i, want, entry.Action)
				}
			}
			break
		}
	}
	// The bypass (8 entries), the operator entry (in and out) and the Child SA's
	// PROTECT entries must all have been written and released, or the replay above
	// checked nothing but the catch-all.
	if protectWrites != 2*len(protect) {
		t.Fatalf("replayed %d writes of the Child SA's %d PROTECT entries, want each installed and removed", protectWrites, len(protect))
	}
	if others < 2*(8+2+len(protect)) {
		t.Fatalf("replayed %d writes of entries other than the catch-all, want at least %d", others, 2*(8+2+len(protect)))
	}
	if len(live) != 0 {
		t.Fatalf("after the release Ze still holds %d SPD entries: %+v", len(live), live)
	}
}

// VALIDATES: a Ze SPD entry the exit could not remove keeps the catch-all beneath it,
// and the daemon names that entry at Warn, whichever kind of entry it is.
// METHOD: apply the lifecycle configuration with one Child SA, make the backend refuse
// one removal (an operator entry, a Child SA PROTECT entry, an IKE bypass entry), run
// the shutdown, then read the database the backend holds and the log.
// PREVENTS: the release removing the catch-all after a failed removal it logged at
// Debug, which left a Ze entry in force with no final entry beneath it after exit.
func TestReleaseSPDKeepsTheCatchAllWhenARemovalFails(t *testing.T) {
	refused := errors.New("netlink: operation not permitted")
	for _, tc := range []struct {
		name string
		pick func(cfg *ipsec.IPsecConfig, child *ChildSA) dataplane.SPParams
	}{
		{"operator entry", func(cfg *ipsec.IPsecConfig, _ *ChildSA) dataplane.SPParams {
			return spdPolicyParams(cfg.Policies["keep"])[0]
		}},
		{"child sa protect entry", func(_ *ipsec.IPsecConfig, child *ChildSA) dataplane.SPParams {
			return childPolicies(child, dataplane.SADirOut)[0]
		}},
		{"ike bypass entry", func(_ *ipsec.IPsecConfig, _ *ChildSA) dataplane.SPParams {
			return ikeBypassPolicies(ikeBypassFamilies[0])[0]
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resetSPDLeft(t)
			dp := &lifecycleDP{}
			loadFailClosedBackend(t, dp)

			var logged bytes.Buffer
			state := applyTestState(t)
			state.log = slog.New(slog.NewTextHandler(&logged, nil))
			cfg := lifecycleConfig(t)
			if err := state.applyConfig(cfg, applyStartup); err != nil {
				t.Fatalf("apply: %v", err)
			}
			child := lifecyclePeer(t, state, dp)
			stuck := tc.pick(cfg, child)
			dp.removeErr = map[spKey]error{keyOf(stuck): refused}

			state.shutdown(state.log)

			if _, present := dp.live[keyOf(stuck)]; !present {
				t.Fatalf("the entry whose removal was refused is not installed; the test refused nothing")
			}
			for want := range catchAllKeys(t) {
				if _, present := dp.live[want]; !present {
					t.Fatalf("the catch-all entry %+v was removed while %+v is still installed", want, keyOf(stuck))
				}
			}
			out := logged.String()
			if !strings.Contains(out, "level=WARN") || !strings.Contains(out, "catch-all stays installed") {
				t.Fatalf("no warning that the catch-all stays installed:\n%s", out)
			}
			want := "ip xfrm policy delete " + dirLabel(stuck.Dir) + " src " + stuck.Src.String() + " dst " + stuck.Dst.String()
			if !strings.Contains(out, want) {
				t.Fatalf("the warning does not name the command that removes the entry left (%s):\n%s", want, out)
			}
		})
	}
}

// VALIDATES: by the time the SPD release starts, operator `clear` can no longer start
// a peer and the IKE sockets are closed, so nothing can install a policy behind it.
// METHOD: give the engine a published re-establish hook and an open IKE socket, run
// the shutdown, and read both from inside the backend's first removal.
// PREVENTS: an SDK error return reaching the release with the sockets open and
// `clear` still armed, so a session it started could install a Child SA policy
// between the release and the backend closing.
func TestShutdownDisarmsClearAndClosesSocketsBeforeTheRelease(t *testing.T) {
	resetSPDLeft(t)
	dp := &lifecycleDP{}
	loadFailClosedBackend(t, dp)

	state := applyTestState(t)
	if err := state.applyConfig(lifecycleConfig(t), applyStartup); err != nil {
		t.Fatalf("apply: %v", err)
	}
	tr, err := transport.NewUDPTransport("127.0.0.1:0", slogutil.DiscardLogger())
	if err != nil {
		t.Fatalf("open an IKE socket: %v", err)
	}
	state.listeners.ike = tr
	armed := func() {}
	reEstablishFn.Store(&armed)
	t.Cleanup(func() { reEstablishFn.Store(nil) })

	checked := false
	dp.onRemove = func() {
		if checked {
			return
		}
		checked = true
		if reEstablishFn.Load() != nil {
			t.Errorf("the SPD release started with operator `clear` still able to start peers")
		}
		if state.listeners.ike != nil {
			t.Errorf("the SPD release started with the IKE socket still open")
		}
	}

	state.shutdown(state.log)
	if !checked {
		t.Fatal("the shutdown removed no SPD entry, so the order was never observed")
	}
}
