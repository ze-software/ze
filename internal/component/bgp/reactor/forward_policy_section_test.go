// Design: docs/architecture/bgp/structural-forwarding.md -- withheld policy output sections
// Related: filter_ordered.go -- runEgressPolicyChainASN4
package reactor

import (
	"bytes"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/family"
)

// TestRawExportMixedSectionsPreservePolicy forwards a single-field source through
// a configured raw export filter that returns different legacy prefixes beside a
// link-local MP announcement. It reads every dispatched UPDATE after pool drain.
// The callback rejects its second invocation, bounding the pre-fix recursion.
// VALIDATES: only the MP route is withdrawn; the rewritten legacy announcement
// and earlier in-process LOCAL_PREF edit survive, with no source-prefix leakage.
// PREVENTS: regenerating mixed policy output on every single-field retry.
func TestRawExportMixedSectionsPreservePolicy(t *testing.T) {
	for _, tc := range []struct {
		name string
		aigp bool
	}{{name: "plain"}, {name: "received-aigp", aigp: true}} {
		t.Run(tc.name, func(t *testing.T) {
			testRawExportMixedSectionsPreservePolicy(t, tc.aigp)
		})
	}
}

func testRawExportMixedSectionsPreservePolicy(t *testing.T, aigp bool) {
	t.Helper()
	h := newFanoutHarnessWith(t, 1, 1, fanoutOpts{groups: true})
	peer := h.dests[0]
	peer.settings.NextHopMode = NextHopUnchanged
	peer.settings.ExportFilters = []filterapi.FilterRef{{Name: "policy:mixed-raw"}}
	peer.negotiated.Load().families[family.IPv6Unicast] = true
	peer.settings.AIGPSession = &aigp
	peer.refreshForwardFacts()

	mpBody, wantMPWithdrawal := llnhLinkLocalOnlyPayload(t)
	mpUpdate, err := message.UnpackUpdate(mpBody)
	if err != nil {
		t.Fatal(err)
	}
	attrs := append([]byte(nil), mpUpdate.PathAttributes...)
	attrs = append(attrs, makeAttr(0x40, 3, []byte{192, 0, 2, 1})...)
	wantAIGP := []byte{1, 0, 11, 0, 0, 0, 0, 0, 0, 0, 100}
	if aigp {
		source, err := message.UnpackUpdate(h.update.WireUpdate.Payload())
		if err != nil {
			t.Fatal(err)
		}
		sourceAttrs := append([]byte(nil), source.PathAttributes...)
		sourceAttrs = append(sourceAttrs, makeAttr(0x80, 26, wantAIGP)...)
		body := buildModTestPayload(sourceAttrs, source.NLRI)
		h.update.WireUpdate = wireu.NewWireUpdate(body, h.update.WireUpdate.SourceCtxID())
		h.update.WireUpdate.SetMessageID(1)
		// The raw policy's replacement metric is not a new received baseline.
		attrs = append(attrs, makeAttr(0x80, 26, []byte{1, 0, 11, 0, 0, 0, 0, 0, 0, 3, 231})...)
	}
	wantLegacy := []byte{24, 203, 0, 113}
	override := buildModTestPayload(attrs, wantLegacy)
	r := h.adapter.r
	r.api = &pluginserver.Server{}
	calls := 0
	r.policyFilterSeam = func(_, _, _, _ string, _ uint32, _ string) PolicyResponse {
		calls++
		if calls > 1 {
			t.Error("raw policy was rerun while partitioning its own output")
			return PolicyResponse{Action: PolicyReject}
		}
		return PolicyResponse{Action: PolicyModify, Raw: override}
	}
	wantLocalPref := []byte{0, 0, 0, 177}
	r.orderedEgressSteps = orderedEgressStepsFromFuncs(func(_, _ filterapi.PeerFilterInfo, _ []byte, _ map[string]any, mods *filterapi.ModAccumulator) bool {
		mods.Op(5, filterapi.AttrModSet, wantLocalPref)
		return true
	})
	r.orderedEgressSteps = append(r.orderedEgressSteps, orderedEgressStep{name: policyChainStepName, policyChain: true})
	if err := h.forward(); err != nil {
		t.Errorf("forward: %v", err)
	}
	r.fwdPool.Stop()
	if calls != 1 {
		t.Errorf("raw policy calls = %d, want 1", calls)
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.sent) != 2 {
		t.Fatalf("dispatched UPDATEs = %d, want legacy announcement and MP withdrawal", len(h.sent))
	}
	var announcements, withdrawals int
	for _, sent := range h.sent {
		u, err := message.UnpackUpdate(sent.body)
		if err != nil {
			t.Fatal(err)
		}
		if len(u.WithdrawnRoutes) != 0 {
			t.Errorf("unexpected legacy withdrawal: %x", u.WithdrawnRoutes)
		}
		if _, _, _, found := attribute.AttrFind(u.PathAttributes, attribute.AttrMPReachNLRI); found {
			t.Error("withheld MP announcement escaped")
		}
		if len(u.NLRI) != 0 {
			announcements++
			if !bytes.Equal(u.NLRI, wantLegacy) {
				t.Errorf("legacy announcement = %x, want policy output %x", u.NLRI, wantLegacy)
			}
			_, _, localPref, found := attribute.AttrFind(u.PathAttributes, attribute.AttrLocalPref)
			if !found || !bytes.Equal(localPref, wantLocalPref) {
				t.Errorf("LOCAL_PREF = %x, want preceding in-process edit %x", localPref, wantLocalPref)
			}
			if aigp {
				_, _, value, found := attribute.AttrFind(u.PathAttributes, attribute.AttrAIGP)
				if !found || !bytes.Equal(value, wantAIGP) {
					t.Errorf("AIGP = %x, want received baseline %x", value, wantAIGP)
				}
			}
		}
		if _, _, value, found := attribute.AttrFind(u.PathAttributes, attribute.AttrMPUnreachNLRI); found {
			withdrawals++
			if !bytes.Equal(value, wantMPWithdrawal) {
				t.Errorf("MP withdrawal = %x, want %x", value, wantMPWithdrawal)
			}
		}
	}
	if announcements != 1 || withdrawals != 1 {
		t.Errorf("announcements/withdrawals = %d/%d, want 1/1", announcements, withdrawals)
	}
}

// TestRawExportMixedBadASPathStillWithdraws reads the consumer output when a
// malformed policy AS_PATH prevents the allowed legacy sibling announcement.
// VALIDATES: neither the link-local withdrawal nor an existing withdrawal is
// lost to announcement-only AS-path materialization.
// PREVENTS: moving AS-path resolution ahead of per-section withholding.
func TestRawExportMixedBadASPathStillWithdraws(t *testing.T) {
	h := newFanoutHarnessWith(t, 1, 1, fanoutOpts{groups: true})
	peer := h.dests[0]
	peer.settings.PeerAS = 65002
	peer.settings.NextHopMode = NextHopUnchanged
	peer.settings.ExportFilters = []filterapi.FilterRef{{Name: "policy:bad-path"}}
	peer.negotiated.Load().families[family.IPv6Unicast] = true
	peer.refreshForwardFacts()
	mpBody, wantMPWithdrawal := llnhLinkLocalOnlyPayload(t)
	mpUpdate, err := message.UnpackUpdate(mpBody)
	if err != nil {
		t.Fatal(err)
	}
	_, _, reach, found := attribute.AttrFind(mpUpdate.PathAttributes, attribute.AttrMPReachNLRI)
	if !found {
		t.Fatal("fixture has no MP_REACH_NLRI")
	}
	attrs := makeAttr(0x80, 14, reach)
	attrs = append(attrs, makeAttr(0x40, 1, []byte{0})...)
	// AS_SET bypasses the sequence prepend shortcut; five ASNs declared,
	// only one present, so the announcement resolver must reject it.
	attrs = append(attrs, makeAttr(0x40, 2, []byte{1, 5, 0, 0, 0xfd, 0xe9})...)
	attrs = append(attrs, makeAttr(0x40, 3, []byte{192, 0, 2, 1})...)
	wantLegacyWithdrawal := []byte{24, 198, 51, 100}
	override := makeUpdateBody(wantLegacyWithdrawal, attrs, []byte{24, 203, 0, 113})
	r := h.adapter.r
	r.api = &pluginserver.Server{}
	calls := 0
	r.policyFilterSeam = func(_, _, _, _ string, _ uint32, _ string) PolicyResponse {
		calls++
		if calls > 1 {
			t.Error("raw policy was rerun")
			return PolicyResponse{Action: PolicyReject}
		}
		return PolicyResponse{Action: PolicyModify, Raw: override}
	}
	r.orderedEgressSteps = []orderedEgressStep{{name: policyChainStepName, policyChain: true}}
	if err := h.forward(); err != nil {
		t.Errorf("forward: %v", err)
	}
	r.fwdPool.Stop()
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.sent) != 2 {
		t.Fatalf("dispatched UPDATEs = %d, want original and withheld-route withdrawals", len(h.sent))
	}
	var legacyWithdrawals, mpWithdrawals int
	for _, sent := range h.sent {
		u, err := message.UnpackUpdate(sent.body)
		if err != nil {
			t.Fatal(err)
		}
		if len(u.NLRI) != 0 {
			t.Errorf("bad AS_PATH announcement escaped: %x", u.NLRI)
		}
		if _, _, _, found := attribute.AttrFind(u.PathAttributes, attribute.AttrMPReachNLRI); found {
			t.Error("link-local announcement escaped")
		}
		if len(u.WithdrawnRoutes) != 0 {
			legacyWithdrawals++
			if !bytes.Equal(u.WithdrawnRoutes, wantLegacyWithdrawal) {
				t.Errorf("legacy withdrawal = %x, want %x", u.WithdrawnRoutes, wantLegacyWithdrawal)
			}
		}
		if _, _, value, found := attribute.AttrFind(u.PathAttributes, attribute.AttrMPUnreachNLRI); found {
			mpWithdrawals++
			if !bytes.Equal(value, wantMPWithdrawal) {
				t.Errorf("MP withdrawal = %x, want %x", value, wantMPWithdrawal)
			}
		}
	}
	if calls != 1 || legacyWithdrawals != 1 || mpWithdrawals != 1 {
		t.Errorf("policy calls/legacy withdrawals/MP withdrawals = %d/%d/%d, want 1/1/1", calls, legacyWithdrawals, mpWithdrawals)
	}
}
