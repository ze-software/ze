// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- SPD dispositions
// Related: unmatched.go -- the catch-all this file proves first in and last out
// RFC: rfc/short/rfc4301.md -- Section 4.4.1, the nominal final entry

package engine

import (
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
)

// spdOp is one write a backend received, in the order it received it.
type spdOp struct {
	install bool
	policy  dataplane.SPParams
}

// lifecycleDP records every SPD write in order and holds the catch-all, so a test can
// replay the database the kernel would have held after each write.
type lifecycleDP struct {
	bypassDP
	ops []spdOp
}

func (d *lifecycleDP) CatchAllSupported() error { return nil }

func (d *lifecycleDP) InstallPolicy(p dataplane.SPParams) error {
	d.ops = append(d.ops, spdOp{install: true, policy: p})
	return d.bypassDP.InstallPolicy(p)
}

func (d *lifecycleDP) RemovePolicyParams(p dataplane.SPParams) error {
	d.ops = append(d.ops, spdOp{policy: p})
	return d.bypassDP.RemovePolicyParams(p)
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
// bypass entry on a backend that records every write, then run the release every exit
// of runEngine runs, and replay the writes, checking the invariant after each one.
// PREVENTS: the IKE bypass installed at engine start with no catch-all beneath it, and
// a release that removed the catch-all while the bypass or an operator entry was still
// in force: in both windows a packet met Ze's entries but not the final one.
//
// RFC requirement: RFC4301-4.4.1-3 positive -- with the `unmatched` leaf absent, the discard catch-all is installed before any other Ze SPD entry and removed after all of them, so every database state that holds a Ze entry also holds the final entry that discards what it does not match.
func TestRFC4301FinalEntryIsUnderEveryZeSPDEntry(t *testing.T) {
	dp := &lifecycleDP{}
	loadFailClosedBackend(t, dp)

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

	state := applyTestState(t)
	if err := state.applyConfig(cfg, applyStartup); err != nil {
		t.Fatalf("apply: %v", err)
	}
	state.stopAllPeers()
	state.releaseSPD(dataplane.Get(), state.log)

	finalEntry := catchAllKeys(t)
	live := map[spKey]dataplane.SPParams{}
	others := 0
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
	// The bypass (8 entries) and the operator entry (in and out) must both have been
	// written and released, or the replay above checked nothing but the catch-all.
	if others < 2*(8+2) {
		t.Fatalf("replayed %d writes of entries other than the catch-all, want at least %d", others, 2*(8+2))
	}
	if len(live) != 0 {
		t.Fatalf("after the release Ze still holds %d SPD entries: %+v", len(live), live)
	}
}
