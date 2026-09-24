package rsvpte

// Design: docs/architecture/rsvpte/mpls-rsvp-te.md -- RFC 4090 fast reroute obligations
//
// RFC 4090 head-end (Section 5), PLR (Section 6) and downstream (Section
// 7.2) obligations, each driven through the engine's packet entry points and
// asserted on the wire message or table state the engine produces.
// Related: frr_test.go carries the facility-backup helpers these tests reuse.
//
// VALIDATES: the FAST_REROUTE object crosses a transit unchanged and is never
// inserted by one; the PLR's RRO flags say only what the armed bypass provides;
// the head-end stores, refreshes and reads back the protection objects; the
// downstream LSR keeps state across an upstream link failure until soft-state
// expiry.
// PREVENTS: a transit rewriting the head-end's backup parameters, a PLR
// claiming a bandwidth guarantee its bypass never reserved, and a downstream
// LSR tearing a protected LSP before the PLR can refresh it.

import (
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/slogutil"
)

// pathWithProtectionObjects encodes a PATH for psb carrying exactly the given
// SESSION_ATTRIBUTE and FAST_REROUTE objects (nil omits one), in the RFC 3209
// Section 4.7 order buildPath uses. A test controls the objects a head-end
// signaled rather than what buildPath derives from a protectionRequest.
func pathWithProtectionObjects(psb *pathStateBlock, hop netip.Addr, sa *sessionAttribute, fr *fastReroute) []byte {
	encoders := []objEncoder{
		func(b []byte) int { return encodeSessionIPv4(b, psb.Session) },
		func(b []byte) int { return encodeRSVPHop(b, rsvpHop{NextHop: hop}) },
		func(b []byte) int {
			return encodeTimeValues(b, timeValues{RefreshPeriod: refreshMillis(DefaultRefreshPeriod)})
		},
		func(b []byte) int { return encodeERO(b, psb.ERO) },
		func(b []byte) int { return encodeLabelRequest(b, psb.LabelRequest) },
	}
	if sa != nil {
		encoders = append(encoders, func(b []byte) int { return encodeSessionAttr(b, *sa) })
	}
	if fr != nil {
		encoders = append(encoders, func(b []byte) int { return encodeFastReroute(b, *fr) })
	}
	encoders = append(encoders,
		func(b []byte) int { return encodeSenderTemplate(b, psb.SenderTemplate) },
		func(b []byte) int { return encodeFlowSpec(b, ClassSenderTSpec, psb.SenderTSpec) },
	)
	if psb.RecordRoute {
		encoders = append(encoders, func(b []byte) int { return encodeRRO(b, psb.RRO) })
	}
	return encodeMessage(MsgTypePath, 64, encoders)
}

// relayedRROFlags drives the merge point's RESV into the PLR and returns the
// flags of the PLR's own (first) RRO subobject on the RESV it relays upstream.
func relayedRROFlags(t *testing.T, e *engine, ft *fakeTransport, psb *pathStateBlock, mp netip.Addr, rro []rroEntry) uint8 {
	t.Helper()
	rsb := &resvStateBlock{Session: psb.Session, Label: labelObject{Label: 18000}, Style: StyleSharedExplicit, RRO: rro}
	e.handlePacket(Packet{Src: mp, Payload: buildResv(rsb, psb.SenderTemplate, DefaultRefreshPeriod, mp)})
	relayed, _, ok := ft.lastByType(MsgTypeResv)
	require.True(t, ok, "PLR relays a RESV upstream")
	require.Len(t, relayed.FlowDescriptors, 1)
	require.Len(t, relayed.FlowDescriptors[0].Filters, 1)
	filter := relayed.FlowDescriptors[0].Filters[0]
	require.True(t, filter.HasRRO)
	require.NotEmpty(t, filter.RRO)
	require.Equal(t, e.cfg().RouterID, filter.RRO[0].Address, "PLR's own RRO subobject is first")
	return filter.RRO[0].Flags
}

// TestRFC4090TransitRelaysFastRerouteUnchanged: a transit relays the FAST_REROUTE
// object it received byte for byte, including the affinity fields and a flags
// value naming both methods, which no head-end configuration of this node builds.
//
// RFC requirement: RFC4090-4.1-3 positive — the FAST_REROUTE object a transit relays downstream equals the one it received: both method flags (0x03), hop-limit, priorities, bandwidth and the three affinity fields are unchanged.
func TestRFC4090TransitRelaysFastRerouteUnchanged(t *testing.T) {
	e, ft, _ := plrEngine(t)
	ingress := netip.MustParseAddr("10.0.0.1")
	psb := protectedTransitPSB(nil)
	received := fastReroute{
		SetupPrio: 3, HoldPrio: 2, HopLimit: 5,
		Flags:      FRRFlagOneToOneBackup | FRRFlagFacilityBackup,
		Bandwidth:  5e7,
		IncludeAny: 0xAABBCCDD, ExcludeAny: 0x11223344, IncludeAll: 0x55667788,
	}
	sa := sessionAttribute{SetupPrio: 3, HoldPrio: 2, Flags: SessAttrLocalProtection | SessAttrLabelRecording, Name: "t1"}
	e.handlePacket(Packet{Src: ingress, Payload: pathWithProtectionObjects(psb, ingress, &sa, &received)})

	fwd, _, ok := ft.lastByType(MsgTypePath)
	require.True(t, ok, "PLR relays the PATH downstream")
	require.True(t, fwd.HasFastReroute, "relayed PATH carries the FAST_REROUTE object")
	assert.Equal(t, received, fwd.FastReroute, "relayed FAST_REROUTE is the received object, unchanged")
}

// TestRFC4090TransitInsertsNoFastReroute: a head-end that asked for protection
// through the SESSION_ATTRIBUTE flag alone sent no FAST_REROUTE, and the transit
// relays none, while still treating the LSP as protected.
//
// RFC requirement: RFC4090-4.1-3 negative — a transit relaying a PATH whose head-end sent only the SESSION_ATTRIBUTE local-protection flag inserts no FAST_REROUTE object (HasFastReroute is false on the relayed PATH).
// RFC requirement: RFC4090-6-2 positive — a PATH with the "local protection desired" flag set and no FAST_REROUTE object is a local-protection request: the PLR stores the protection request and arms the bypass.
func TestRFC4090TransitInsertsNoFastReroute(t *testing.T) {
	e, ft, _ := plrEngine(t)
	ingress := netip.MustParseAddr("10.0.0.1")
	psb := protectedTransitPSB(nil)
	sa := sessionAttribute{SetupPrio: 7, HoldPrio: 7, Flags: SessAttrLocalProtection, Name: "t1"}
	e.handlePacket(Packet{Src: ingress, Payload: pathWithProtectionObjects(psb, ingress, &sa, nil)})

	fwd, _, ok := ft.lastByType(MsgTypePath)
	require.True(t, ok, "PLR relays the PATH downstream")
	assert.False(t, fwd.HasFastReroute, "transit inserts no FAST_REROUTE the head-end did not send")
	assert.True(t, fwd.HasSessionAttr, "the protection request is still relayed in SESSION_ATTRIBUTE")

	lsp, ok := e.table.Get(protectedKey())
	require.True(t, ok)
	lsp.mu.Lock()
	protection := lsp.PSB.Protection
	bypass := lsp.Bypass
	lsp.mu.Unlock()
	require.NotNil(t, protection, "the flag alone is a protection request")
	assert.NotNil(t, bypass, "PLR armed a bypass for the flag-only request")
}

// TestRFC4090FastRerouteAloneRequestsProtection: a FAST_REROUTE object with no
// SESSION_ATTRIBUTE at all is a protection request, and this transit LSR acts
// as the PLR for it.
//
// RFC requirement: RFC4090-6-2 positive — a PATH carrying a FAST_REROUTE object and no SESSION_ATTRIBUTE is a local-protection request: the PLR arms the bypass.
// RFC requirement: RFC4090-6-1 positive — a transit LSR on a protected LSP follows the PLR behavior: it arms the configured bypass whose merge point is the next hop.
func TestRFC4090FastRerouteAloneRequestsProtection(t *testing.T) {
	e, _, _ := plrEngine(t)
	ingress := netip.MustParseAddr("10.0.0.1")
	psb := protectedTransitPSB(nil)
	fr := fastReroute{SetupPrio: 7, HoldPrio: 7, HopLimit: 16, Flags: FRRFlagFacilityBackup, Bandwidth: 1e8}
	e.handlePacket(Packet{Src: ingress, Payload: pathWithProtectionObjects(psb, ingress, nil, &fr)})

	lsp, ok := e.table.Get(protectedKey())
	require.True(t, ok)
	lsp.mu.Lock()
	bypass := lsp.Bypass
	lsp.mu.Unlock()
	require.NotNil(t, bypass, "FAST_REROUTE alone arms the bypass")
	assert.Equal(t, netip.MustParseAddr("10.0.0.3"), bypass.TunnelEndpoint, "the armed bypass merges at the NHOP")
}

// TestRFC4090NoProtectionRequestArmsNothing: a SESSION_ATTRIBUTE without the
// local-protection flag, and no FAST_REROUTE, asks for nothing.
//
// RFC requirement: RFC4090-6-2 negative — a PATH whose SESSION_ATTRIBUTE has the "local protection desired" flag clear and carries no FAST_REROUTE object is not a protection request: no protection state is stored and no bypass is armed.
func TestRFC4090NoProtectionRequestArmsNothing(t *testing.T) {
	e, _, _ := plrEngine(t)
	ingress := netip.MustParseAddr("10.0.0.1")
	psb := protectedTransitPSB(nil)
	sa := sessionAttribute{SetupPrio: 7, HoldPrio: 7, Flags: SessAttrLabelRecording, Name: "t1"}
	e.handlePacket(Packet{Src: ingress, Payload: pathWithProtectionObjects(psb, ingress, &sa, nil)})

	lsp, ok := e.table.Get(protectedKey())
	require.True(t, ok)
	lsp.mu.Lock()
	protection := lsp.PSB.Protection
	bypass := lsp.Bypass
	lsp.mu.Unlock()
	assert.Nil(t, protection, "no protection request stored")
	assert.Nil(t, bypass, "no bypass armed")
}

// TestRFC4090EgressDoesNotActAsPLR: the egress of a protected LSP is the one LSR
// exempt from PLR behavior: it arms no bypass and reports no protection flags,
// even with a bypass configured on it.
//
// RFC requirement: RFC4090-6-1 negative — the egress LSR of a protected LSP does not follow the PLR behavior: it arms no bypass and its RRO subobject carries no protection flag.
func TestRFC4090EgressDoesNotActAsPLR(t *testing.T) {
	e, ft, _ := testEngine(t, "10.0.0.9", func(c *rsvpteConfig) {
		c.Interfaces = []ifaceConfig{{Name: "eth0", MaxBW: 10e9, MaxReservableBW: 10e9, Prefix: netip.MustParsePrefix("10.0.0.0/24")}}
		c.Bypasses = []bypassConfig{{
			Name: "bp", MergePoint: netip.MustParseAddr("10.0.0.3"),
			ERO: []eroHop{{Address: netip.MustParsePrefix("10.0.0.3/32")}},
		}}
	})
	e.admission.setInterface("eth0", 10e9, 10e9)
	prev := netip.MustParseAddr("10.0.0.3")
	psb := protectedTransitPSB(&protectionRequest{Facility: true, HopLimit: 16, Bandwidth: 1e8, SetupPrio: 7, HoldPrio: 7})
	psb.ERO = []eroHop{{Address: netip.MustParsePrefix("10.0.0.9/32")}}
	e.handlePacket(Packet{Src: prev, Payload: buildPath(psb, prev, 64)})

	lsp, ok := e.table.Get(protectedKey())
	require.True(t, ok)
	lsp.mu.Lock()
	role := lsp.Role
	bypass := lsp.Bypass
	lsp.mu.Unlock()
	require.Equal(t, RoleEgress, role)
	assert.Nil(t, bypass, "egress arms no bypass")
	resv, _, ok := ft.lastByType(MsgTypeResv)
	require.True(t, ok, "egress answers with a RESV")
	require.Len(t, resv.FlowDescriptors, 1)
	require.Len(t, resv.FlowDescriptors[0].Filters, 1)
	filter := resv.FlowDescriptors[0].Filters[0]
	require.True(t, filter.HasRRO)
	require.NotEmpty(t, filter.RRO)
	assert.Zero(t, filter.RRO[0].Flags, "egress RRO subobject carries no protection flag")
}

// TestRFC4090BandwidthBitNeverClaimed: a bypass reserves no bandwidth, so the PLR
// reports the backup as available without claiming the requested bandwidth is
// guaranteed, whatever the head-end asked for.
//
// RFC requirement: RFC4090-4.4-6 negative — with "bandwidth protection desired" set and an armed bypass that reserves no bandwidth, the "bandwidth protection" bit (0x04) is clear in the PLR's RRO subobject.
// RFC requirement: RFC4090-4.4-6 positive — the same subobject reports exactly "local protection available" (0x01): the unguaranteed backup is reported, and no bandwidth guarantee is claimed beside it.
// RFC requirement: RFC4090-6-7 positive — with "bandwidth protection desired" set the "bandwidth protection" bit matches the armed backup path, which guarantees no bandwidth, so it is clear.
func TestRFC4090BandwidthBitNeverClaimed(t *testing.T) {
	e, ft, _ := plrEngine(t)
	ingress := netip.MustParseAddr("10.0.0.1")
	mp := netip.MustParseAddr("10.0.0.3")
	psb := protectedTransitPSB(&protectionRequest{Facility: true, BandwidthProtection: true, HopLimit: 16, Bandwidth: 1e8, SetupPrio: 7, HoldPrio: 7})
	e.handlePacket(Packet{Src: ingress, Payload: buildPath(psb, ingress, 64)})
	fwd, _, ok := ft.lastByType(MsgTypePath)
	require.True(t, ok)
	require.NotZero(t, fwd.SessionAttr.Flags&SessAttrBandwidthProtection, "the head-end asked for bandwidth protection")
	bringBypassUp(t, e, 5000, netip.MustParseAddr("10.0.1.3"))

	flags := relayedRROFlags(t, e, ft, psb, mp, nil)
	assert.Zero(t, flags&RROFlagBandwidthProtection, "bandwidth protection bit stays clear: the bypass guarantees no bandwidth")
	assert.Equal(t, RROFlagProtectionAvailable, flags, "only local protection available is reported")
}

// TestRFC4090BandwidthDesiredWithoutBackup: the desired flag alone never sets
// the bandwidth-protection bit; with no backup path there is nothing to match.
//
// RFC requirement: RFC4090-6-7 negative — with "bandwidth protection desired" set and no backup path armed, the "bandwidth protection" bit is clear rather than copied from the request.
func TestRFC4090BandwidthDesiredWithoutBackup(t *testing.T) {
	e, ft, _ := plrEngine(t)
	ingress := netip.MustParseAddr("10.0.0.1")
	nhop := netip.MustParseAddr("10.0.0.7") // no configured bypass merges here
	psb := protectedTransitPSB(&protectionRequest{Facility: true, BandwidthProtection: true, HopLimit: 16, Bandwidth: 1e8, SetupPrio: 7, HoldPrio: 7})
	psb.ERO = []eroHop{
		{Address: netip.MustParsePrefix("10.0.0.2/32")},
		{Address: netip.MustParsePrefix("10.0.0.7/32")},
		{Address: netip.MustParsePrefix("10.0.0.9/32")},
	}
	e.handlePacket(Packet{Src: ingress, Payload: buildPath(psb, ingress, 64)})

	flags := relayedRROFlags(t, e, ft, psb, nhop, nil)
	assert.Zero(t, flags&RROFlagBandwidthProtection, "no backup path: bandwidth bit not taken from the request")
	assert.Zero(t, flags, "no backup path: no protection flag at all")
}

// rfc4090NodePLR is nodePLR with its transport exposed, so a test can read the
// RESV the PLR relays upstream.
func rfc4090NodePLR(t *testing.T) (*engine, *fakeTransport) {
	t.Helper()
	e, ft, _ := testEngine(t, "10.0.0.2", func(c *rsvpteConfig) {
		c.Interfaces = []ifaceConfig{
			{Name: "eth0", Prefix: netip.MustParsePrefix("10.0.0.0/24"), MaxBW: 10e9, MaxReservableBW: 10e9},
			{Name: "eth1", Prefix: netip.MustParsePrefix("10.0.1.0/24"), MaxBW: 10e9, MaxReservableBW: 10e9},
		}
		c.Bypasses = []bypassConfig{{
			Name: "node-bp", MergePoint: netip.MustParseAddr("10.0.0.4"), NodeProtection: true,
			ERO: []eroHop{{Address: netip.MustParsePrefix("10.0.1.4/32")}},
		}}
	})
	e.admission.setInterface("eth0", 10e9, 10e9)
	e.admission.setInterface("eth1", 10e9, 10e9)
	return e, ft
}

// nodeRecordedRRO is the RESV RRO the NHOP sends when every node records its
// label: the NHOP (18000) and the NNHOP merge point (7777).
func nodeRecordedRRO() []rroEntry {
	return []rroEntry{
		{Type: RROSubIPv4, Address: netip.MustParseAddr("10.0.0.3")}, {Type: RROSubLabel, Label: 18000},
		{Type: RROSubIPv4, Address: netip.MustParseAddr("10.0.0.4")}, {Type: RROSubLabel, Label: 7777},
	}
}

// TestRFC4090NodeBitSetWhenNodeProtected: a node-protection request served by a
// bypass that merges at the next-next hop reports "node protection".
//
// RFC requirement: RFC4090-4.4-7 positive — when node protection is provided (a bypass merging at the NNHOP is armed for a node-protection request), the "node protection" bit (0x08) is set in the PLR's RRO subobject.
// RFC requirement: RFC4090-6-6 positive — with "node protection desired" set, the "node protection" bit is set to match a backup path that protects the next node.
func TestRFC4090NodeBitSetWhenNodeProtected(t *testing.T) {
	e, ft := rfc4090NodePLR(t)
	ingress := netip.MustParseAddr("10.0.0.1")
	psb := nodeProtectionPSB()
	e.handlePacket(Packet{Src: ingress, Payload: buildPath(psb, ingress, 64)})
	bringBypassUp(t, e, 5000, netip.MustParseAddr("10.0.1.4"))

	flags := relayedRROFlags(t, e, ft, psb, netip.MustParseAddr("10.0.0.3"), nodeRecordedRRO())
	assert.NotZero(t, flags&RROFlagNodeProtection, "node protection bit set: the armed bypass merges at the NNHOP")
	assert.NotZero(t, flags&RROFlagProtectionAvailable, "local protection available")
}

// TestRFC4090NodeBitClearWithoutNodeBypass: a node-protection request that only
// a link bypass could serve gets no node protection, and the bit stays clear.
//
// RFC requirement: RFC4090-4.4-7 negative — when node protection is not provided (the only configured bypass merges at the NHOP), the "node protection" bit (0x08) is clear in the PLR's RRO subobject.
// RFC requirement: RFC4090-6-6 negative — with "node protection desired" set and no backup path protecting the next node, the "node protection" bit is cleared to match.
func TestRFC4090NodeBitClearWithoutNodeBypass(t *testing.T) {
	e, ft, _ := plrEngine(t) // bypass "bp" merges at the NHOP only
	ingress := netip.MustParseAddr("10.0.0.1")
	psb := nodeProtectionPSB()
	e.handlePacket(Packet{Src: ingress, Payload: buildPath(psb, ingress, 64)})
	bringBypassUp(t, e, 5000, netip.MustParseAddr("10.0.1.3"))

	flags := relayedRROFlags(t, e, ft, psb, netip.MustParseAddr("10.0.0.3"), nodeRecordedRRO())
	assert.Zero(t, flags&RROFlagNodeProtection, "node protection bit clear: no bypass protects the next node")
}

// TestRFC4090InUseOnlyWhileRedirecting: "local protection in use" tracks the
// data plane: clear while the bypass is merely armed, set once a link failure
// redirected traffic onto it.
//
// RFC requirement: RFC4090-6-5 positive — before any local repair the PLR's relayed RESV carries "local protection in use" (0x02) clear, although a bypass is armed and up.
// RFC requirement: RFC4090-6-5 negative — once local repair redirects traffic onto the bypass, the next relayed RESV carries "local protection in use" (0x02) set: the bit is cleared only while no redirection is active.
func TestRFC4090InUseOnlyWhileRedirecting(t *testing.T) {
	e, ft, fib := plrEngine(t)
	ingress := netip.MustParseAddr("10.0.0.1")
	mp := netip.MustParseAddr("10.0.0.3")
	psb := protectedTransitPSB(&protectionRequest{Facility: true, HopLimit: 16, Bandwidth: 1e8, SetupPrio: 7, HoldPrio: 7})
	e.handlePacket(Packet{Src: ingress, Payload: buildPath(psb, ingress, 64)})
	bringBypassUp(t, e, 5000, netip.MustParseAddr("10.0.1.3"))

	before := relayedRROFlags(t, e, ft, psb, mp, nil)
	assert.Zero(t, before&RROFlagProtectionInUse, "in use clear while nothing is redirected")

	e.handleLinkDown("eth0")
	require.Len(t, fib.backups, 1, "local repair redirected traffic onto the bypass")

	after := relayedRROFlags(t, e, ft, psb, mp, nil)
	assert.NotZero(t, after&RROFlagProtectionInUse, "in use set while traffic is redirected")
}

// TestRFC4090FailureSwitchesToBackup: a failure on the protected LSP's link
// reprograms its in-label onto the bypass next hop; a failure elsewhere leaves
// the normal out-segment in place.
//
// RFC requirement: RFC4090-6.3-1 positive — on the protected link's failure the PLR programs the protected in-label to the bypass next hop with the bypass label stacked, instead of the normal out-segment.
// RFC requirement: RFC4090-6.3-1 negative — a failure on a link the protected LSP does not use programs no backup: packets stay on the normal out-segment.
func TestRFC4090FailureSwitchesToBackup(t *testing.T) {
	e, _, fib := plrEngine(t)
	protectedIn := armAndUpProtected(t, e)
	bringBypassUp(t, e, 5000, netip.MustParseAddr("10.0.1.3"))
	require.Len(t, fib.swapped, 1, "the normal out-segment is programmed")

	e.handleLinkDown("eth9")
	assert.Empty(t, fib.backups, "no failure on the protected LSP: nothing switched")

	e.handleLinkDown("eth0")
	require.Len(t, fib.backups, 1, "the protected link's failure switches the LSP")
	assert.Equal(t, protectedIn, fib.backups[0].in, "keyed by the protected in-label")
	assert.Equal(t, netip.MustParseAddr("10.0.1.3"), fib.backups[0].nextHop, "packets go to the bypass next hop")
	assert.Equal(t, uint32(5000), fib.backups[0].out[0], "under the bypass label")
}

// TestRFC4090BypassDestinationIsMergePoint: the bypass LSP is signaled to the
// configured merge point, and a bypass whose destination is not an LSP's merge
// point does not protect that LSP.
//
// RFC requirement: RFC4090-6.2-1 positive — the PATH that signals a bypass tunnel carries the merge point's address as the SESSION tunnel endpoint.
// RFC requirement: RFC4090-6.2-1 negative — a bypass whose destination is not the protected LSP's next hop is not selected for that LSP.
func TestRFC4090BypassDestinationIsMergePoint(t *testing.T) {
	e, ft, _ := plrEngine(t)
	cfg := e.cfg()
	setupBypass(slogutil.DiscardLogger(), e.table, cfg.Bypasses[0], cfg, e)

	path, dst, ok := ft.lastByType(MsgTypePath)
	require.True(t, ok, "bypass PATH sent")
	assert.Equal(t, cfg.Bypasses[0].MergePoint, path.Session.TunnelEndpoint, "bypass destination is the merge point")
	assert.Equal(t, cfg.Bypasses[0].ERO[0].Address.Addr(), dst, "transport dst is the configured first hop")
	sent := lastPathCarriage(t, ft, MsgTypePath, path.Session.TunnelID)
	assert.Equal(t, cfg.Bypasses[0].MergePoint, sent.route.Destination, "IP destination is the merge point")
	assert.Equal(t, cfg.Bypasses[0].ERO[0].Address.Addr(), sent.route.NextHop, "next hop is independent of the tunnel destination")

	pr := &protectionRequest{Facility: true, Transit: true}
	other := []eroHop{{Address: netip.MustParsePrefix("10.0.0.7/32")}, {Address: netip.MustParsePrefix("10.0.0.9/32")}}
	_, selected := e.selectBypass(other, pr)
	assert.False(t, selected, "a bypass to 10.0.0.3 does not protect an LSP whose merge point is 10.0.0.7")
}

// TestRFC4090HeadEndStoresFastRerouteForRefresh: the head-end's refresh PATH
// carries the same FAST_REROUTE the tunnel was signaled with.
//
// RFC requirement: RFC4090-5-1 positive — a head-end refresh PATH for a protected tunnel carries the stored FAST_REROUTE object with the signaled priorities, hop-limit, method flag and bandwidth.
func TestRFC4090HeadEndStoresFastRerouteForRefresh(t *testing.T) {
	e, ft, key := headEndEngine(t)
	lsp, ok := e.table.Get(key)
	require.True(t, ok)
	lsp.mu.Lock()
	lsp.PSB.Protection = &protectionRequest{Facility: true, HopLimit: 9, Bandwidth: 1e8, SetupPrio: 5, HoldPrio: 4, Name: "t1"}
	lsp.mu.Unlock()

	refreshPaths(slogutil.DiscardLogger(), e.table, e)

	path, _, ok := ft.lastByType(MsgTypePath)
	require.True(t, ok, "refresh PATH sent")
	require.True(t, path.HasFastReroute, "refresh PATH carries FAST_REROUTE")
	want := fastReroute{SetupPrio: 5, HoldPrio: 4, HopLimit: 9, Flags: FRRFlagFacilityBackup, Bandwidth: 1e8}
	assert.Equal(t, want, path.FastReroute, "the stored object is refreshed unchanged")
}

// TestRFC4090HeadEndRefreshWithoutProtection: a tunnel signaled without
// protection refreshes without a FAST_REROUTE object.
//
// RFC requirement: RFC4090-5-1 negative — a head-end refresh PATH for a tunnel that signaled no FAST_REROUTE carries none.
func TestRFC4090HeadEndRefreshWithoutProtection(t *testing.T) {
	e, ft, _ := headEndEngine(t)

	refreshPaths(slogutil.DiscardLogger(), e.table, e)

	path, _, ok := ft.lastByType(MsgTypePath)
	require.True(t, ok, "refresh PATH sent")
	assert.False(t, path.HasFastReroute, "no FAST_REROUTE was signaled, none is refreshed")
}

// TestRFC4090HeadEndSetsLabelRecordingDesired: a protected tunnel's PATH asks
// every hop to record its label.
//
// RFC requirement: RFC4090-5-2 positive — the SESSION_ATTRIBUTE of a protected tunnel's PATH has the "label recording desired" flag (0x04) set.
func TestRFC4090HeadEndSetsLabelRecordingDesired(t *testing.T) {
	msg, err := DecodeMessage(buildPath(protectedPSB(), netip.MustParseAddr("10.0.0.1"), 64))
	require.NoError(t, err)
	require.True(t, msg.HasSessionAttr)
	assert.NotZero(t, msg.SessionAttr.Flags&SessAttrLabelRecording, "label recording desired is set")
}

// TestRFC4090UnprotectedPathRequestsNoLabelRecording: with no protection the
// PATH carries no SESSION_ATTRIBUTE, so the flag is never signaled.
//
// RFC requirement: RFC4090-5-2 negative — a tunnel without protection emits no SESSION_ATTRIBUTE, so "label recording desired" is not set on its PATH.
func TestRFC4090UnprotectedPathRequestsNoLabelRecording(t *testing.T) {
	psb := protectedPSB()
	psb.Protection = nil
	msg, err := DecodeMessage(buildPath(psb, netip.MustParseAddr("10.0.0.1"), 64))
	require.NoError(t, err)
	assert.False(t, msg.HasSessionAttr, "no SESSION_ATTRIBUTE without protection")
}

// headEndResvRRO drives a RESV carrying rro into a protected head-end LSP and
// returns the RRO the head-end stored for it.
func headEndResvRRO(t *testing.T, rro []rroEntry) []rroEntry {
	t.Helper()
	e, ft, _ := testEngine(t, "10.0.0.1", nil)
	tc := tunnelConfig{Destination: netip.MustParseAddr("10.0.0.9"), TunnelID: 1, Bandwidth: 8e8,
		ERO: []eroHop{{Address: netip.MustParsePrefix("10.0.0.2/32")}, {Address: netip.MustParsePrefix("10.0.0.9/32")}}}
	setupTunnel(e.log, e.table, tc, e.cfg(), e)
	key := tunnelKey(tc, e.cfg().RouterID)
	lsp, ok := e.table.Get(key)
	require.True(t, ok)
	require.Equal(t, LSPStatePathSent, lsp.State)
	lsp.mu.Lock()
	lsp.PSB.Protection = &protectionRequest{Facility: true, HopLimit: 16, Bandwidth: 1e8, SetupPrio: 7, HoldPrio: 7}
	psb := lsp.PSB
	lsp.mu.Unlock()
	require.NoError(t, e.sendPath(lsp), "head-end requests route recording before the RESV")
	path, _, sent := ft.lastByType(MsgTypePath)
	require.True(t, sent)
	require.True(t, path.HasRRO, "head-end requested route recording")
	nhop := netip.MustParseAddr("10.0.0.2")
	rsb := &resvStateBlock{Session: psb.Session, Label: labelObject{Label: 17000}, Style: StyleSharedExplicit, RRO: rro}
	e.handlePacket(Packet{Src: nhop, Payload: buildResv(rsb, psb.SenderTemplate, DefaultRefreshPeriod, nhop)})

	lsp.mu.Lock()
	defer lsp.mu.Unlock()
	require.Equal(t, LSPStateUp, lsp.State, "head-end accepted the RESV")
	require.NotNil(t, lsp.RSB)
	return lsp.RSB.RRO
}

// rroEntryFor returns the RRO entry recorded for addr.
func rroEntryFor(t *testing.T, rro []rroEntry, addr netip.Addr) rroEntry {
	t.Helper()
	for _, entry := range rro {
		if entry.Type == RROSubIPv4 && entry.Address == addr {
			return entry
		}
	}
	t.Fatalf("no RRO entry for %s in %v", addr, rro)
	return rroEntry{}
}

// TestRFC4090HeadEndAcceptsRROProtectionFlags: the head-end takes a RESV whose
// RRO subobjects carry the four Section 4.4 flags, set or clear, and keeps the
// flags as received.
//
// RFC requirement: RFC4090-5-3 positive — a RESV whose RRO IPv4 subobject has all four Section 4.4 flags set (0x0F) brings the protected LSP up and the head-end stores the subobject with those flags.
// RFC requirement: RFC4090-5-3 negative — a RESV whose RRO IPv4 subobject has the four flags clear brings the LSP up and the head-end stores the subobject with flags 0, inventing none.
func TestRFC4090HeadEndAcceptsRROProtectionFlags(t *testing.T) {
	plr := netip.MustParseAddr("10.0.0.2")
	all := RROFlagProtectionAvailable | RROFlagProtectionInUse | RROFlagBandwidthProtection | RROFlagNodeProtection

	set := headEndResvRRO(t, []rroEntry{{Type: RROSubIPv4, Address: plr, Flags: all}})
	assert.Equal(t, all, rroEntryFor(t, set, plr).Flags, "all four flags kept as received")

	clear := headEndResvRRO(t, []rroEntry{{Type: RROSubIPv4, Address: plr}})
	assert.Zero(t, rroEntryFor(t, clear, plr).Flags, "clear flags kept clear")
}

// TestRFC4090HeadEndAcceptsRROLabelSubobject: the head-end takes a RESV whose RRO
// records labels and can read the label a hop recorded.
//
// RFC requirement: RFC4090-5-4 positive — a RESV whose RRO carries a Label subobject after a hop's IPv4 subobject brings the protected LSP up and the head-end resolves that hop's recorded label.
// RFC requirement: RFC4090-5-4 negative — a RESV whose RRO records no label for a hop brings the LSP up and resolves no label for that hop.
func TestRFC4090HeadEndAcceptsRROLabelSubobject(t *testing.T) {
	plr := netip.MustParseAddr("10.0.0.2")
	egress := netip.MustParseAddr("10.0.0.9")

	withLabel := headEndResvRRO(t, []rroEntry{
		{Type: RROSubIPv4, Address: plr}, {Type: RROSubLabel, Label: 17000},
		{Type: RROSubIPv4, Address: egress}, {Type: RROSubLabel, Label: 3},
	})
	label, ok := labelForAddr(withLabel, plr)
	require.True(t, ok, "the PLR's recorded label is readable")
	assert.Equal(t, uint32(17000), label)

	withoutLabel := headEndResvRRO(t, []rroEntry{{Type: RROSubIPv4, Address: plr}, {Type: RROSubIPv4, Address: egress}})
	_, ok = labelForAddr(withoutLabel, plr)
	assert.False(t, ok, "no label recorded, none resolved")
}

// downstreamEngine is the LSR just downstream of a protected link: upstream PLR
// 10.0.0.2 on eth0, its own address 10.0.1.3, egress 10.0.2.9 on eth2.
func downstreamEngine(t *testing.T) (*engine, *fakeTransport, *pathStateBlock) {
	t.Helper()
	e, ft, _ := testEngine(t, "10.0.1.3", func(c *rsvpteConfig) {
		c.RefreshMultiplier = 3
		c.Interfaces = []ifaceConfig{
			{Name: "eth0", MaxBW: 10e9, MaxReservableBW: 10e9, Prefix: netip.MustParsePrefix("10.0.0.0/24")},
			{Name: "eth2", MaxBW: 10e9, MaxReservableBW: 10e9, Prefix: netip.MustParsePrefix("10.0.2.0/24")},
		}
	})
	e.admission.setInterface("eth0", 10e9, 10e9)
	e.admission.setInterface("eth2", 10e9, 10e9)
	plr := netip.MustParseAddr("10.0.0.2")
	egress := netip.MustParseAddr("10.0.2.9")
	psb := protectedTransitPSB(&protectionRequest{Facility: true, HopLimit: 16, Bandwidth: 1e8, SetupPrio: 7, HoldPrio: 7})
	psb.Session.TunnelEndpoint = egress
	psb.ERO = []eroHop{{Address: netip.MustParsePrefix("10.0.1.3/32")}, {Address: netip.MustParsePrefix("10.0.2.9/32")}}
	e.handlePacket(Packet{Src: plr, Payload: buildPath(psb, plr, 64)})
	rsb := &resvStateBlock{Session: psb.Session, Label: labelObject{Label: 3}, Style: StyleSharedExplicit}
	e.handlePacket(Packet{Src: egress, Payload: buildResv(rsb, psb.SenderTemplate, DefaultRefreshPeriod, egress)})
	return e, ft, psb
}

// TestRFC4090DownstreamKeepsStateOnUpstreamLinkFailure: the LSR below a failed
// link keeps the protected LSP's Path and Resv state and sends no PathTear or
// ResvErr, so the PLR can refresh it through the bypass.
//
// RFC requirement: RFC4090-7.2-1 positive — when the link toward the upstream PLR fails, the downstream LSR keeps the protected LSP's Path and Resv state and sends no PathTear, PathErr or ResvErr.
// RFC requirement: RFC4090-7.2-1 negative — the same LSR is the PLR for the link toward its own next hop: with no bypass that failure tears the LSP down, so retention is specific to the downstream role.
func TestRFC4090DownstreamKeepsStateOnUpstreamLinkFailure(t *testing.T) {
	e, ft, psb := downstreamEngine(t)
	key := rfc4090Key(psb)
	_, ok := e.table.Get(key)
	require.True(t, ok, "protected LSP is up at the downstream LSR")

	e.handleLinkDown("eth0") // the protected link, upstream of this LSR

	lsp, retained := e.table.Get(key)
	require.True(t, retained, "Path and Resv state kept")
	lsp.mu.Lock()
	hasPSB := lsp.PSB != nil
	hasRSB := lsp.RSB != nil
	lsp.mu.Unlock()
	assert.True(t, hasPSB && hasRSB, "both state blocks kept")
	for _, msgType := range []uint8{MsgTypePathTear, MsgTypePathErr, MsgTypeResvErr} {
		_, _, sent := ft.lastByType(msgType)
		assert.False(t, sent, "no teardown message %d sent immediately", msgType)
	}

	e.handleLinkDown("eth2") // the link toward this LSR's own next hop: PLR role, no bypass

	_, alive := e.table.Get(key)
	assert.False(t, alive, "as PLR without a bypass the LSR tears the LSP down")
}

// rfc4090Key is the lspKey a PSB's session and sender template identify.
func rfc4090Key(psb *pathStateBlock) lspKey {
	return lspKey{
		TunnelEndpoint: psb.Session.TunnelEndpoint, TunnelID: psb.Session.TunnelID, ExtTunnelID: psb.Session.ExtTunnelID,
		SenderAddr: psb.SenderTemplate.SenderAddr, LSPID: psb.SenderTemplate.LSPID,
	}
}

// TestRFC4090StateExpiresWithoutRefresh: state the PLR never refreshes through
// the bypass is removed when the refresh timer expires, and refreshed state is
// kept.
//
// RFC requirement: RFC4090-7.2-2 positive — a protected LSP whose PATH was last refreshed longer ago than the refresh period times the multiplier is removed by the cleanup tick.
// RFC requirement: RFC4090-7.2-2 negative — a protected LSP refreshed within the timer is kept by the cleanup tick.
func TestRFC4090StateExpiresWithoutRefresh(t *testing.T) {
	e, _, psb := downstreamEngine(t)
	key := rfc4090Key(psb)
	lsp, ok := e.table.Get(key)
	require.True(t, ok)
	cfg := e.cfg()
	log := slogutil.DiscardLogger()

	cleanupTick(log, e.table, cfg, e, time.Now(), cfg.RefreshPeriod)
	_, kept := e.table.Get(key)
	require.True(t, kept, "refreshed state kept")

	lsp.mu.Lock()
	lsp.PSB.LastRefresh = time.Now().Add(-cfg.RefreshPeriod*time.Duration(cfg.RefreshMultiplier) - time.Second)
	lsp.mu.Unlock()
	cleanupTick(log, e.table, cfg, e, time.Now(), cfg.RefreshPeriod)
	_, alive := e.table.Get(key)
	assert.False(t, alive, "unrefreshed state removed at expiry")
}
