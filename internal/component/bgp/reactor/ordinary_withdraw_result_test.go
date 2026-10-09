// Design: docs/architecture/api/architecture.md -- final named-commit results.
// Related: ordinary_growth_test.go -- independent growth wire oracle.
package reactor

import (
	"bytes"
	"encoding/binary"
	"errors"
	"net/netip"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/rib"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
)

// TestOrdinaryWithdrawalFinalResult drives actual withdrawal commits through an
// active export replacement and the Session ownership writer. Offered withdrawal
// counts cannot describe either a successful announcement split or partial output.
func TestOrdinaryWithdrawalFinalResult(t *testing.T) {
	for _, partial := range []bool{false, true} {
		name := "growth-announcements"
		if partial {
			name = "partial-withdrawal"
		}
		t.Run(name, func(t *testing.T) {
			r, peer, conn := newOrdinaryJointPeer(t, netip.MustParseAddr("2001:db8:a::2"), ordinaryJointPolicyPair)
			session := peer.session
			t.Cleanup(session.timers.StopAll)
			ctxID, err := bgpctx.Registry.Register(peer.sendCtx.Load())
			if err != nil {
				t.Fatal(err)
			}
			session.sendCtxID = ctxID
			session.adjOut = &peer.adjOut
			peer.sendCtxID = ctxID
			peer.fwdFacts.Store(peer.buildForwardFacts())
			r.api = &pluginserver.Server{}
			replacement, wantNLRI := ordinaryGrowthUpdate(t, 237)
			if partial {
				replacement = ordinaryWithdrawalPartialBody(t)
			}
			encoded := make([]byte, replacement.Len(nil))
			if n := replacement.WriteTo(encoded, 0, nil); n != len(encoded) {
				t.Fatalf("replacement length = %d, want %d", n, len(encoded))
			}
			policyCalls := 0
			r.policyFilterSeam = func(_, _, _, _ string, _ uint32, _ string) PolicyResponse {
				policyCalls++
				if policyCalls == 1 {
					return PolicyResponse{Action: PolicyModify, Raw: encoded[19:]}
				}
				return PolicyResponse{Action: PolicyAccept}
			}
			api := &reactorAPIAdapter{r: r}
			withdrawals := []nlri.NLRI{nlri.NewINET(family.IPv6Unicast, netip.MustParsePrefix("2001:db8:ffff::/48"), 0)}
			result, err := api.SendRoutes(selector.All(), nil, withdrawals, false, plugin.OperatorSender())
			if err != nil {
				t.Fatalf("named commit aggregate error: %v", err)
			}
			announced, withdrawn, updates := 237, 0, 2
			var reasons []string
			if partial {
				announced, withdrawn, updates = 0, 1, 1
				reasons = []string{bgptypes.CommitReasonSendFailed}
				want := mustHex(t, "ffffffffffffffffffffffffffffffff001b02000418c633640000")
				if got := conn.written(); !bytes.Equal(got, want) {
					t.Errorf("partial wire = %x, want %x", got, want)
				}
			} else if frames := ordinaryGrowthWire(t, conn.written(), 4096, wantNLRI, 0); frames != updates {
				t.Errorf("growth frames = %d, want %d", frames, updates)
			}
			if result.RoutesQueued != 0 || result.WithdrawalsQueued != 1 || result.RoutesAnnounced != announced || result.RoutesWithdrawn != withdrawn || result.UpdatesSent != updates || result.EORSent != 0 {
				t.Errorf("commit result = %+v, want announced=%d withdrawn=%d updates=%d", result, announced, withdrawn, updates)
			}
			if len(result.Peers) != 1 {
				t.Fatalf("peer rows = %d, want 1", len(result.Peers))
			}
			row := result.Peers[0]
			if row.RoutesAnnounced != announced || row.RoutesWithdrawn != withdrawn || row.UpdatesSent != updates || row.EORSent != 0 || !slices.Equal(row.Reasons, reasons) {
				t.Errorf("peer result = %+v, want announced=%d withdrawn=%d updates=%d reasons=%v", row, announced, withdrawn, updates, reasons)
			}
			if policyCalls != 1 || session.writePending || peer.adjOut.pending || len(peer.adjOut.routes) != announced {
				t.Errorf("final writer state: calls=%d pending=%v/%v owned=%d", policyCalls, session.writePending, peer.adjOut.pending, len(peer.adjOut.routes))
			}
			before := len(conn.written())
			healthy, err := api.SendRoutes(selector.All(), nil, withdrawals, false, plugin.OperatorSender())
			if err != nil {
				t.Fatal(err)
			}
			want := mustHex(t, "ffffffffffffffffffffffffffffffff0024020000000d800f0a0002013020010db8ffff")
			if got := conn.written()[before:]; !bytes.Equal(got, want) {
				t.Errorf("healthy withdrawal wire = %x, want %x", got, want)
			}
			if healthy.RoutesAnnounced != 0 || healthy.RoutesWithdrawn != 1 || healthy.UpdatesSent != 1 || len(healthy.Peers) != 1 {
				t.Errorf("healthy result = %+v", healthy)
			} else if len(healthy.Peers[0].Reasons) != 0 {
				t.Errorf("healthy reasons = %v", healthy.Peers[0].Reasons)
			}
			if policyCalls != 2 || session.writeFailed != nil || session.tearingDown.Load() || session.fsm.State() != fsm.StateEstablished {
				t.Errorf("subsequent session: calls=%d error=%v teardown=%v state=%v", policyCalls, session.writeFailed, session.tearingDown.Load(), session.fsm.State())
			}
		})
	}
}

// TestOrdinaryBatchRetainsFinalSplitRefusal distinguishes a negotiated-family
// encoding failure from the API's intentionally soft no-family warning.
func TestOrdinaryBatchRetainsFinalSplitRefusal(t *testing.T) {
	r, peer, conn := newOrdinaryJointPeer(t, netip.MustParseAddr("2001:db8:a::2"), ordinaryJointPolicyPair)
	t.Cleanup(peer.session.timers.StopAll)
	r.api = &pluginserver.Server{}
	replacement := ordinaryWithdrawalPartialBody(t)
	encoded := make([]byte, replacement.Len(nil))
	if n := replacement.WriteTo(encoded, 0, nil); n != len(encoded) {
		t.Fatalf("replacement length = %d, want %d", n, len(encoded))
	}
	calls := 0
	r.policyFilterSeam = func(_, _, _, _ string, _ uint32, _ string) PolicyResponse {
		calls++
		if calls == 1 {
			return PolicyResponse{Action: PolicyModify, Raw: encoded[19:]}
		}
		return PolicyResponse{Action: PolicyAccept}
	}
	api := &reactorAPIAdapter{r: r}
	batch := bgptypes.NLRIBatch{Family: family.IPv6Unicast, NLRIs: manyIPv6Host(1), NextHop: bgptypes.NewNextHopExplicit(peer.settings.LocalAddress)}
	err := api.AnnounceNLRIBatch(t.Context(), selector.All(), batch, plugin.OperatorSender())
	if !errors.Is(err, message.ErrNLRITooLarge) {
		t.Errorf("batch error = %v, want final NLRI size refusal", err)
	}
	want := mustHex(t, "ffffffffffffffffffffffffffffffff001b02000418c633640000")
	if got := conn.written(); !bytes.Equal(got, want) {
		t.Errorf("partial wire = %x, want %x", got, want)
	}
	before := len(conn.written())
	if err := api.AnnounceNLRIBatch(t.Context(), selector.All(), batch, plugin.OperatorSender()); err != nil {
		t.Fatalf("healthy subsequent batch: %v", err)
	}
	_, wantNLRI := ordinaryGrowthUpdate(t, 1)
	ordinaryGrowthWire(t, conn.written()[before:], 4096, wantNLRI, 0)
	if calls != 2 || peer.session.writePending || peer.session.writeFailed != nil || peer.session.fsm.State() != fsm.StateEstablished {
		t.Errorf("subsequent session: calls=%d pending=%v error=%v state=%v", calls, peer.session.writePending, peer.session.writeFailed, peer.session.fsm.State())
	}
}

// TestOrdinaryWithdrawalOutputCannotMaskAnnouncementDrop sends a mixed commit
// whose withdrawal becomes an announcement but whose announcement is suppressed.
// A combined row count cannot decide whether the announcement half fell short.
func TestOrdinaryWithdrawalOutputCannotMaskAnnouncementDrop(t *testing.T) {
	r, peer, conn := newOrdinaryJointPeer(t, netip.MustParseAddr("2001:db8:a::2"), ordinaryJointPolicyPair)
	t.Cleanup(peer.session.timers.StopAll)
	r.api = &pluginserver.Server{}
	replacement, wantNLRI := ordinaryGrowthUpdate(t, 1)
	encoded := make([]byte, replacement.Len(nil))
	if n := replacement.WriteTo(encoded, 0, nil); n != len(encoded) {
		t.Fatalf("replacement length = %d, want %d", n, len(encoded))
	}
	calls := 0
	r.policyFilterSeam = func(_, _, _, _ string, _ uint32, _ string) PolicyResponse {
		calls++
		if calls == 1 {
			return PolicyResponse{Action: PolicyModify, Raw: encoded[19:]}
		}
		return PolicyResponse{Action: PolicyReject}
	}
	api := &reactorAPIAdapter{r: r}
	routes := []*rib.Route{rib.NewRouteWithASPath(manyIPv6Host(1)[0], peer.settings.LocalAddress, nil, nil)}
	result, err := api.SendRoutes(selector.All(), routes, manyIPv6Host(1), false, plugin.OperatorSender())
	if err != nil {
		t.Fatal(err)
	}
	if frames := ordinaryGrowthWire(t, conn.written(), 4096, wantNLRI, 0); frames != 1 {
		t.Errorf("wire frames = %d, want 1", frames)
	}
	if result.RoutesAnnounced != 1 || result.RoutesWithdrawn != 0 || result.UpdatesSent != 1 || len(result.Peers) != 1 {
		t.Fatalf("mixed commit result = %+v, want only withdrawal-produced announcement", result)
	}
	if got := result.Peers[0].Reasons; !slices.Equal(got, []string{bgptypes.CommitReasonRoutesDropped}) {
		t.Errorf("mixed commit reasons = %v, want routes-dropped", got)
	}
	if calls != 2 {
		t.Errorf("policy calls = %d, want both input halves", calls)
	}
}

// ordinaryWithdrawalPartialBody builds a valid 4089-octet mixed UPDATE whose
// next-hop growth leaves insufficient room for its MP route after the legacy
// withdrawal has been accepted. Communities precede MP_REACH in wire order.
func ordinaryWithdrawalPartialBody(t *testing.T) *message.Update {
	t.Helper()
	communities := make([]byte, 4008)
	copy(communities, []byte{0xd0, 8, 0x0f, 0xa4})
	for i := range 1001 {
		binary.BigEndian.PutUint16(communities[4+i*4:], 65000)
		binary.BigEndian.PutUint16(communities[6+i*4:], uint16(i))
	}
	small, _ := ordinaryGrowthUpdate(t, 1)
	attrs := append(bytes.Clone(small.PathAttributes[:13]), communities...)
	mp := small.PathAttributes[17:]
	attrs = append(attrs, 0x80, 14, byte(len(mp)))
	attrs = append(attrs, mp...)
	update := &message.Update{WithdrawnRoutes: mustHex(t, "18c63364"), PathAttributes: attrs}
	if update.Len(nil) != 4089 {
		t.Fatalf("partial replacement length = %d, want 4089", update.Len(nil))
	}
	return update
}
