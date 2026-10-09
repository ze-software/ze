// Design: docs/architecture/edge-cases/extended-message.md -- final ordinary UPDATE splitting.
// RFC naming: untagged -- regression for asymmetric negotiated framing after next-hop growth; canonical claims are maintained separately.
package reactor

import (
	"bytes"
	"encoding/binary"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/rib"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
)

// TestOrdinaryIPv6GrowthAsymmetricFraming drives a raw export replacement through
// SendRoutes and the actual Session writer. Each NLRI-bearing field must retain
// its own family's framing when adding the owned link-local triggers splitting.
// The large-attribute case must deliver the plain withdrawal before refusing the
// indivisible IPv6 announcement, without retiring the otherwise healthy session.
func TestOrdinaryIPv6GrowthAsymmetricFraming(t *testing.T) {
	for _, tc := range []struct {
		name         string
		reachAddPath bool
		mpWithdrawal bool
		communities  int
		refuse       bool
	}{
		{name: "plain-legacy-path-reach", reachAddPath: true, communities: 998},
		{name: "path-legacy-plain-reach", communities: 998},
		{name: "plain-mp-unreach-path-reach", reachAddPath: true, mpWithdrawal: true, communities: 997},
		{name: "path-mp-unreach-plain-reach", mpWithdrawal: true, communities: 997},
		{name: "plain-legacy-path-reach-refused", reachAddPath: true, communities: 1001, refuse: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, peer, conn := newOrdinaryJointPeer(t, netip.MustParseAddr("2001:db8:a::2"), ordinaryJointPolicyPair)
			session := peer.session
			t.Cleanup(session.timers.StopAll)
			apFamily := family.IPv4Unicast
			if tc.reachAddPath {
				apFamily = family.IPv6Unicast
			}
			local := append(peerOffering(family.IPv4Unicast, family.IPv6Unicast),
				&capability.ASN4{ASN: 65000},
				&capability.AddPath{Families: []capability.AddPathFamily{{AFI: apFamily.AFI, SAFI: apFamily.SAFI, Mode: capability.AddPathSend}}})
			remote := append(peerOffering(family.IPv4Unicast, family.IPv6Unicast),
				&capability.ASN4{ASN: 65001},
				&capability.AddPath{Families: []capability.AddPathFamily{{AFI: apFamily.AFI, SAFI: apFamily.SAFI, Mode: capability.AddPathReceive}}})
			negotiated := capability.Negotiate(local, remote, capability.PeerIdentity{LocalASN: 65000, PeerASN: 65001})
			sendCtx := bgpctx.FromNegotiatedSend(negotiated)
			ctxID, err := bgpctx.Registry.Register(sendCtx)
			if err != nil {
				t.Fatal(err)
			}
			peer.negotiated.Store(NewNegotiatedCapabilities(negotiated))
			peer.sendCtx.Store(sendCtx)
			peer.sendCtxID = ctxID
			session.negotiated = negotiated
			session.sendCtxID = ctxID
			session.adjOut = &peer.adjOut
			peer.fwdFacts.Store(peer.buildForwardFacts())
			if sendCtx.AddPath(family.IPv6Unicast) != tc.reachAddPath || sendCtx.AddPath(family.IPv4Unicast) == tc.reachAddPath {
				t.Fatal("fixture did not negotiate asymmetric send framing")
			}

			communities := make([]byte, 4+4*tc.communities)
			copy(communities, []byte{0xd0, 8})
			binary.BigEndian.PutUint16(communities[2:4], uint16(4*tc.communities))
			for i := range tc.communities {
				binary.BigEndian.PutUint16(communities[4+4*i:], 65000)
				binary.BigEndian.PutUint16(communities[6+4*i:], uint16(i))
			}
			base := append(mustHex(t, "4001010040020602010000fde8"), communities...)
			_, reachNLRI := ordinaryGrowthUpdate(t, 1)
			withdrawNLRI := mustHex(t, "18c63364")
			if tc.reachAddPath {
				reachNLRI = append(mustHex(t, "12345678"), reachNLRI...)
			} else {
				withdrawNLRI = append(mustHex(t, "deadbeef"), withdrawNLRI...)
			}
			mp := append([]byte{0, 2, 1, 16}, peer.settings.LocalAddress.AsSlice()...)
			mp = append(mp, 0)
			mp = append(mp, reachNLRI...)
			attrs := bytes.Clone(base)
			attrs = append(attrs, 0x80, 14, byte(len(mp)))
			attrs = append(attrs, mp...)
			withdrawn := withdrawNLRI
			var wantWithdrawAttrs []byte
			if tc.mpWithdrawal {
				unreach := append([]byte{0, 1, 1}, withdrawNLRI...)
				unreachAttr := append([]byte{0x80, 15, byte(len(unreach))}, unreach...)
				attrs = append(attrs, unreachAttr...)
				wantWithdrawAttrs = append(bytes.Clone(base), unreachAttr...)
				withdrawn = nil
			}
			replacement := ordinaryFramingFrame(withdrawn, attrs)
			if len(replacement) > 4096 || len(replacement)+16 <= 4096 {
				t.Fatalf("replacement length %d does not cross 4096 after sixteen-octet growth", len(replacement))
			}
			if tc.refuse && len(replacement) != 4093 {
				t.Fatalf("refusal boundary length = %d, want 4093", len(replacement))
			}
			r.api = &pluginserver.Server{}
			policyCalls := 0
			r.policyFilterSeam = func(_, _, _, _ string, _ uint32, _ string) PolicyResponse {
				policyCalls++
				if policyCalls == 1 {
					return PolicyResponse{Action: PolicyModify, Raw: replacement[message.HeaderLen:]}
				}
				return PolicyResponse{Action: PolicyAccept}
			}
			api := &reactorAPIAdapter{r: r}
			routes := []*rib.Route{rib.NewRouteWithASPath(
				nlri.NewINET(family.IPv6Unicast, netip.MustParsePrefix("2001:db8::/128"), 7),
				peer.settings.LocalAddress, nil, nil)}
			result, sendErr := api.SendRoutes(selector.All(), routes, nil, false, plugin.OperatorSender())
			if !tc.refuse && sendErr != nil {
				t.Errorf("valid asymmetric replacement refused: %v", sendErr)
			}
			wantWire := ordinaryFramingFrame(withdrawn, wantWithdrawAttrs)
			wantAnnouncements, wantUpdates := 0, 1
			if !tc.refuse {
				grownMP := append([]byte{0, 2, 1, 32}, peer.settings.LocalAddress.AsSlice()...)
				grownMP = append(grownMP, peer.settings.LinkLocal.AsSlice()...)
				grownMP = append(grownMP, 0)
				grownMP = append(grownMP, reachNLRI...)
				grownAttrs := append(bytes.Clone(base), 0x80, 14, byte(len(grownMP)))
				grownAttrs = append(grownAttrs, grownMP...)
				wantWire = append(wantWire, ordinaryFramingFrame(nil, grownAttrs)...)
				wantAnnouncements, wantUpdates = 1, 2
			}
			if got := conn.written(); !bytes.Equal(got, wantWire) {
				t.Errorf("wire differs: got %d octets, want %d; exact withdrawal, MP path IDs and complete frames must survive", len(got), len(wantWire))
			}
			if result.RoutesQueued != 1 || result.RoutesAnnounced != wantAnnouncements || result.RoutesWithdrawn != 1 || result.UpdatesSent != wantUpdates || result.EORSent != 0 {
				t.Errorf("commit result = %+v, want announced=%d withdrawn=1 updates=%d EOR=0", result, wantAnnouncements, wantUpdates)
			}
			if len(result.Peers) != 1 {
				t.Fatalf("peer result rows = %d, want 1", len(result.Peers))
			}
			row := result.Peers[0]
			if row.RoutesAnnounced != wantAnnouncements || row.RoutesWithdrawn != 1 || row.UpdatesSent != wantUpdates || row.EORSent != 0 {
				t.Errorf("peer result = %+v, want exact final-frame accounting", row)
			}
			refused := false
			for _, reason := range row.Reasons {
				if reason == bgptypes.CommitReasonAnnounceRefused {
					refused = true
				}
			}
			if refused != tc.refuse {
				t.Errorf("announce-refused = %v, want %v; reasons=%v", refused, tc.refuse, row.Reasons)
			}
			if stats := peer.Stats(); stats.UpdatesSent != uint32(wantUpdates) || stats.EORSent != 0 {
				t.Errorf("session counters = %d/%d, want %d/0", stats.UpdatesSent, stats.EORSent, wantUpdates)
			}
			if session.writePending || peer.adjOut.pending {
				t.Error("split left a pending write or ownership receipt")
			}
			if got := len(peer.adjOut.routes); got != wantAnnouncements {
				t.Errorf("owned routes = %d, want %d", got, wantAnnouncements)
			}
			if policyCalls != 1 {
				t.Errorf("replacement policy calls = %d, want 1", policyCalls)
			}

			before := len(conn.written())
			batch := bgptypes.NLRIBatch{
				Family:  family.IPv6Unicast,
				NLRIs:   []nlri.NLRI{nlri.NewINET(family.IPv6Unicast, netip.MustParsePrefix("2001:db8::1/128"), 9)},
				NextHop: bgptypes.NewNextHopExplicit(peer.settings.LocalAddress),
			}
			if err := api.AnnounceNLRIBatch(t.Context(), selector.All(), batch, plugin.OperatorSender()); err != nil {
				t.Fatalf("healthy continuation refused: %v", err)
			}
			_, allNLRI := ordinaryGrowthUpdate(t, 2)
			wantContinuation := allNLRI[17:]
			if tc.reachAddPath {
				wantContinuation = append(mustHex(t, "00000009"), wantContinuation...)
			}
			if frames := ordinaryGrowthWire(t, conn.written()[before:], 4096, wantContinuation, 0); frames != 1 {
				t.Errorf("continuation frames = %d, want 1", frames)
			}
			if policyCalls != 2 {
				t.Errorf("policy calls after continuation = %d, want 2", policyCalls)
			}
			if session.writeFailed != nil || session.tearingDown.Load() || session.fsm.State() != fsm.StateEstablished {
				t.Errorf("split damaged session: error=%v teardown=%v state=%v", session.writeFailed, session.tearingDown.Load(), session.fsm.State())
			}
			if got := len(peer.adjOut.routes); got != wantAnnouncements+1 {
				t.Errorf("ownership after continuation = %d, want %d", got, wantAnnouncements+1)
			}
		})
	}
}

// ordinaryFramingFrame constructs expected complete wire without using the
// production UPDATE writer, splitter or next-hop normalization as its oracle.
func ordinaryFramingFrame(withdrawn, attrs []byte) []byte {
	frame := bytes.Repeat([]byte{0xff}, message.HeaderLen)
	binary.BigEndian.PutUint16(frame[16:18], uint16(message.HeaderLen+4+len(withdrawn)+len(attrs)))
	frame[18] = 2
	frame = binary.BigEndian.AppendUint16(frame, uint16(len(withdrawn)))
	frame = append(frame, withdrawn...)
	frame = binary.BigEndian.AppendUint16(frame, uint16(len(attrs)))
	return append(frame, attrs...)
}
