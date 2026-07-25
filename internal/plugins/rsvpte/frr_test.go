// Design: plan/spec-mpls-4-rsvp-te-fast-reroute.md -- Fast Reroute tests
// RFC: rfc/short/rfc4090.md
//
// VALIDATES: RFC 4090 Fast Reroute end to end -- FAST_REROUTE/SESSION_ATTRIBUTE
// codecs and protection flags (AC-1); a transit PLR arming a configured bypass
// and reporting RRO "local protection available" (AC-2); local repair on a
// link-down programming the 2-label backup swap and retaining the LSP (AC-3); the
// PathErr Notify (code 25/3) toward the head-end (AC-4); and the head-end
// make-before-break re-optimization on a Notify (AC-5).
package rsvpte

import (
	"encoding/json"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/slogutil"
)

// TestEncodeDecodeFastReroute round-trips the FAST_REROUTE object and checks its
// header is class 205 / C-Type 1 with the fixed 24-byte length (RFC 4090 S4.1).
func TestEncodeDecodeFastReroute(t *testing.T) {
	fr := fastReroute{
		SetupPrio: 7, HoldPrio: 7, HopLimit: 16,
		Flags:     FRRFlagFacilityBackup,
		Bandwidth: 1e8,
	}
	buf := make([]byte, 64)
	n := encodeFastReroute(buf, fr)
	require.Equal(t, objHdrLen+frrBodyLen, n, "FAST_REROUTE is 24 bytes")

	// RFC requirement: RFC4090-4.1-1 positive -- FAST_REROUTE encodes/decodes as Class 205, C-Type 1, fixed Length 24.
	hdr, err := decodeObjectHeader(buf)
	require.NoError(t, err)
	assert.Equal(t, ClassFastReroute, hdr.ClassNum)
	assert.Equal(t, CTypeFastReroute, hdr.CType)
	assert.Equal(t, uint16(n), hdr.Length)

	got, err := decodeFastReroute(buf[objHdrLen:n])
	require.NoError(t, err)
	assert.Equal(t, fr, got)
}

// TestFastRerouteShortBody rejects a truncated FAST_REROUTE body.
func TestFastRerouteShortBody(t *testing.T) {
	// RFC requirement: RFC4090-4.1-1 negative -- a FAST_REROUTE body shorter than the fixed 24-byte object is rejected (errShortObject), not decoded.
	_, err := decodeFastReroute(make([]byte, frrBodyLen-1))
	assert.ErrorIs(t, err, errShortObject)
}

// TestSessionAttributeProtectionFlags checks the SESSION_ATTRIBUTE flags carry
// the requested protection bits and round-trip with the name (RFC 4090 S4.3).
func TestSessionAttributeProtectionFlags(t *testing.T) {
	pr := &protectionRequest{
		Facility: true, NodeProtection: true, BandwidthProtection: true,
		SetupPrio: 5, HoldPrio: 6, Name: "tunnel-a",
	}
	sa := pr.sessionAttr()
	assert.NotZero(t, sa.Flags&SessAttrLocalProtection, "local protection always desired")
	// RFC requirement: RFC4090-4.3-2 positive -- SESSION_ATTRIBUTE carries the node-protection-desired bit (0x10) when node protection is requested.
	assert.NotZero(t, sa.Flags&SessAttrNodeProtection, "node protection desired set")
	assert.NotZero(t, sa.Flags&SessAttrBandwidthProtection, "bandwidth protection desired set")

	buf := make([]byte, 64)
	n := encodeSessionAttr(buf, sa)
	hdr, err := decodeObjectHeader(buf)
	require.NoError(t, err)
	assert.Equal(t, ClassSessionAttr, hdr.ClassNum)
	assert.Equal(t, CTypeSessionAttr, hdr.CType)

	got, err := decodeSessionAttr(buf[objHdrLen:n], CTypeSessionAttr)
	require.NoError(t, err)
	assert.Equal(t, sa.Flags, got.Flags)
	assert.Equal(t, "tunnel-a", got.Name)
	assert.Equal(t, uint8(5), got.SetupPrio)
	assert.Equal(t, uint8(6), got.HoldPrio)
}

// TestSessionAttributeEmptyName encodes/decodes a name-less SESSION_ATTRIBUTE
// (object Length is then just the 8-byte minimum, 4-byte aligned).
func TestSessionAttributeEmptyName(t *testing.T) {
	buf := make([]byte, 64)
	n := encodeSessionAttr(buf, sessionAttribute{Flags: SessAttrLocalProtection})
	require.Equal(t, objHdrLen+4, n)
	got, err := decodeSessionAttr(buf[objHdrLen:n], CTypeSessionAttr)
	require.NoError(t, err)
	assert.Equal(t, "", got.Name)
	// RFC requirement: RFC4090-4.3-2 negative -- a SESSION_ATTRIBUTE without node protection has flags equal to local-protection only, so the node-protection bit (0x10) stays clear.
	assert.Equal(t, SessAttrLocalProtection, got.Flags)
}

// TestSessionAttributeCType1 decodes the C-Type 1 (LSP_TUNNEL_RA) layout, whose
// 12-byte resource-affinity prefix precedes the priorities/flags. A decoder that
// ignored the C-Type would read an affinity byte as the protection Flags.
func TestSessionAttributeCType1(t *testing.T) {
	body := make([]byte, 12+4) // 3 affinity masks (zero) + Setup/Hold/Flags/NameLen
	body[12] = 5               // Setup Prio
	body[13] = 6               // Hold Prio
	body[14] = SessAttrLocalProtection | SessAttrNodeProtection
	body[15] = 0 // Name Length
	got, err := decodeSessionAttr(body, CTypeSessionAttrRA)
	require.NoError(t, err)
	assert.Equal(t, uint8(5), got.SetupPrio)
	assert.Equal(t, uint8(6), got.HoldPrio)
	assert.NotZero(t, got.Flags&SessAttrLocalProtection, "flags read past the affinity prefix")
	assert.NotZero(t, got.Flags&SessAttrNodeProtection)

	// Misreading as C-Type 7 would take body[2] (an affinity byte = 0) as Flags.
	wrong, err := decodeSessionAttr(body, CTypeSessionAttr)
	require.NoError(t, err)
	assert.Zero(t, wrong.Flags, "C-Type 7 layout reads the affinity prefix, not the flags")

	// A truncated C-Type 1 (fewer than 16 bytes) is rejected.
	_, err = decodeSessionAttr(body[:15], CTypeSessionAttrRA)
	assert.ErrorIs(t, err, errShortObject)
}

// TestRROProtectionFlags confirms the RRO subobject Flags byte carries the
// RFC 4090 protection flags through encode/decode.
func TestRROProtectionFlags(t *testing.T) {
	entries := []rroEntry{{
		Type:    RROSubIPv4,
		Address: netip.MustParseAddr("10.0.0.2"),
		Flags:   RROFlagProtectionAvailable | RROFlagProtectionInUse | RROFlagNodeProtection,
	}}
	buf := make([]byte, 64)
	n := encodeRRO(buf, entries)
	got, err := decodeRRO(buf[objHdrLen:n])
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, RROFlagProtectionAvailable|RROFlagProtectionInUse|RROFlagNodeProtection, got[0].Flags)
}

// protectedPSB is a head-end PSB requesting facility + node protection.
func protectedPSB() *pathStateBlock {
	return &pathStateBlock{
		Session:        sessionIPv4{TunnelEndpoint: netip.MustParseAddr("10.0.0.9"), TunnelID: 1, ExtTunnelID: 0x0a000001},
		SenderTemplate: senderTemplateIPv4{SenderAddr: netip.MustParseAddr("10.0.0.1"), LSPID: 1},
		SenderTSpec:    FlowSpec{TokenRate: 1e8, TokenBucket: 1e8, PeakRate: 1e8},
		LabelRequest:   labelRequest{L3PID: 0x0800},
		RefreshPeriod:  DefaultRefreshPeriod,
		Protection: &protectionRequest{
			Facility: true, NodeProtection: true, HopLimit: 16,
			Bandwidth: 1e8, SetupPrio: 7, HoldPrio: 7, Name: "t1",
		},
	}
}

// TestBuildPathIncludesFastReroute is the wiring test: a PATH built from a
// protection-desired PSB carries both FAST_REROUTE and SESSION_ATTRIBUTE with the
// protection-desired flags set (AC-1).
func TestBuildPathIncludesFastReroute(t *testing.T) {
	raw := buildPath(protectedPSB(), netip.MustParseAddr("10.0.0.1"), 64)
	msg, err := DecodeMessage(raw)
	require.NoError(t, err)

	require.True(t, msg.HasFastReroute, "PATH carries a FAST_REROUTE object")
	// RFC requirement: RFC4090-4.1-2 positive -- a facility-backup request sets the FAST_REROUTE facility method flag (0x02).
	assert.NotZero(t, msg.FastReroute.Flags&FRRFlagFacilityBackup, "facility backup requested")
	assert.Equal(t, uint8(16), msg.FastReroute.HopLimit)

	require.True(t, msg.HasSessionAttr, "PATH carries a SESSION_ATTRIBUTE object")
	// RFC requirement: RFC4090-4.3-1 positive -- a protection-desired PATH carries SESSION_ATTRIBUTE with the local-protection-desired bit (0x01) set.
	assert.NotZero(t, msg.SessionAttr.Flags&SessAttrLocalProtection, "local protection desired")
	// RFC requirement: RFC4090-4.3-2 positive -- the same PATH sets the node-protection-desired bit (0x10) when node protection is requested.
	assert.NotZero(t, msg.SessionAttr.Flags&SessAttrNodeProtection, "node protection desired")
}

// TestBuildPathNoProtection: a PSB without Protection emits neither object, so a
// non-protected LSP signals exactly as base RSVP-TE does.
func TestBuildPathNoProtection(t *testing.T) {
	psb := protectedPSB()
	psb.Protection = nil
	raw := buildPath(psb, netip.MustParseAddr("10.0.0.1"), 64)
	msg, err := DecodeMessage(raw)
	require.NoError(t, err)
	assert.False(t, msg.HasFastReroute)
	// RFC requirement: RFC4090-4.3-1 negative -- a PSB without a protection request emits no SESSION_ATTRIBUTE, so the local-protection-desired bit is never set.
	assert.False(t, msg.HasSessionAttr)
}

// TestProtectionFromPath reconstructs the head-end's protection request from a
// received PATH (what a transit PLR does to learn what to arm), and returns nil
// when no protection is requested.
func TestProtectionFromPath(t *testing.T) {
	raw := buildPath(protectedPSB(), netip.MustParseAddr("10.0.0.1"), 64)
	msg, err := DecodeMessage(raw)
	require.NoError(t, err)

	pr := protectionFromPath(msg)
	require.NotNil(t, pr)
	assert.True(t, pr.Facility)
	assert.True(t, pr.NodeProtection)
	assert.Equal(t, uint8(16), pr.HopLimit)
	assert.Equal(t, "t1", pr.Name)

	psb := protectedPSB()
	psb.Protection = nil
	raw2 := buildPath(psb, netip.MustParseAddr("10.0.0.1"), 64)
	msg2, err := DecodeMessage(raw2)
	require.NoError(t, err)
	assert.Nil(t, protectionFromPath(msg2), "no protection objects -> nil request")
}

// plrEngine builds a transit PLR (10.0.0.2) with two interfaces -- eth0 toward
// the protected next hop (10.0.0.0/24) and eth1 toward the bypass next hop
// (10.0.1.0/24) -- and a configured link-protection bypass whose merge point is
// the protected LSP's next hop (10.0.0.3). Two interfaces are required for FRR:
// the bypass must leave by a different link than the one being protected.
func plrEngine(t *testing.T) (*engine, *fakeTransport, *fakeFIB) {
	t.Helper()
	e, ft, fib := testEngine(t, "10.0.0.2", func(c *rsvpteConfig) {
		c.Interfaces = []ifaceConfig{
			{Name: "eth0", MaxBW: 10e9, MaxReservableBW: 10e9, Prefix: netip.MustParsePrefix("10.0.0.0/24")},
			{Name: "eth1", MaxBW: 10e9, MaxReservableBW: 10e9, Prefix: netip.MustParsePrefix("10.0.1.0/24")},
		}
		c.Bypasses = []bypassConfig{{
			Name:       "bp",
			MergePoint: netip.MustParseAddr("10.0.0.3"),
			ERO:        []eroHop{{Address: netip.MustParsePrefix("10.0.1.3/32")}},
		}}
	})
	e.admission.setInterface("eth0", 10e9, 10e9)
	e.admission.setInterface("eth1", 10e9, 10e9)
	return e, ft, fib
}

// protectedKey is the lspKey of the protected transit LSP signaled through plrEngine.
func protectedKey() lspKey {
	return lspKey{
		TunnelEndpoint: netip.MustParseAddr("10.0.0.9"), TunnelID: 1,
		ExtTunnelID: 0x0a000001, SenderAddr: netip.MustParseAddr("10.0.0.1"), LSPID: 1,
	}
}

// bringBypassUp populates the configured bypass LSP as an up ingress LSP with the
// given merge-point out-label and next hop, as if the bypass's own RESV arrived.
func bringBypassUp(t *testing.T, e *engine, label uint32, nextHop netip.Addr) { //nolint:unparam // label kept explicit for call-site clarity
	t.Helper()
	bk := bypassKey(e.cfg().Bypasses[0], e.cfg().RouterID)
	bl, _ := e.table.GetOrCreate(bk)
	bl.mu.Lock()
	bl.Role = RoleIngress
	bl.IsBypass = true
	bl.OutLabel = label
	bl.NextHop = nextHop
	bl.setState(LSPStateUp)
	bl.mu.Unlock()
}

// armAndUpProtected drives a protection-desired PATH then the egress RESV so the
// protected transit LSP is up with in/out labels and an armed bypass. Returns the
// protected LSP's in-label.
func armAndUpProtected(t *testing.T, e *engine) uint32 {
	t.Helper()
	ingress := netip.MustParseAddr("10.0.0.1")
	mp := netip.MustParseAddr("10.0.0.3")
	psb := protectedTransitPSB(&protectionRequest{Facility: true, HopLimit: 16, Bandwidth: 1e8, SetupPrio: 7, HoldPrio: 7})
	e.handlePacket(Packet{Src: ingress, Payload: buildPath(psb, ingress, 64)})
	rsb := &resvStateBlock{Session: psb.Session, Label: labelObject{Label: 18000}, Style: StyleSharedExplicit}
	e.handlePacket(Packet{Src: mp, Payload: buildResv(rsb, psb.SenderTemplate, DefaultRefreshPeriod, mp)})
	lsp, ok := e.table.Get(protectedKey())
	require.True(t, ok)
	lsp.mu.Lock()
	in := lsp.InLabel
	lsp.mu.Unlock()
	return in
}

// protectedTransitPSB is the head-end PSB for an A->PLR->C->egress LSP whose ERO
// routes through the PLR (10.0.0.2) and the merge point (10.0.0.3).
func protectedTransitPSB(pr *protectionRequest) *pathStateBlock {
	return &pathStateBlock{
		Session:        sessionIPv4{TunnelEndpoint: netip.MustParseAddr("10.0.0.9"), TunnelID: 1, ExtTunnelID: 0x0a000001},
		SenderTemplate: senderTemplateIPv4{SenderAddr: netip.MustParseAddr("10.0.0.1"), LSPID: 1},
		ERO: []eroHop{
			{Address: netip.MustParsePrefix("10.0.0.2/32")},
			{Address: netip.MustParsePrefix("10.0.0.3/32")},
			{Address: netip.MustParsePrefix("10.0.0.9/32")},
		},
		SenderTSpec:  FlowSpec{TokenRate: 1e8, TokenBucket: 1e8, PeakRate: 1e8},
		LabelRequest: labelRequest{L3PID: 0x0800},
		Protection:   pr,
	}
}

// TestPLRArmsBypass: a transit node receiving a protection-desired PATH arms the
// configured bypass whose merge point is the NHOP, relays the protection request
// downstream, and reports "local protection available" in the RESV RRO (AC-2).
func TestPLRArmsBypass(t *testing.T) {
	e, ft, _ := plrEngine(t)
	ingress := netip.MustParseAddr("10.0.0.1")
	psb := protectedTransitPSB(&protectionRequest{Facility: true, HopLimit: 16, Bandwidth: 1e8, SetupPrio: 7, HoldPrio: 7, Name: "t1"})
	e.handlePacket(Packet{Src: ingress, Payload: buildPath(psb, ingress, 64)})

	fwd, dst, ok := ft.lastByType(MsgTypePath)
	require.True(t, ok, "PLR relays the PATH downstream")
	assert.Equal(t, netip.MustParseAddr("10.0.0.3"), dst, "relayed toward the NHOP")
	assert.True(t, fwd.HasFastReroute, "PLR relays FAST_REROUTE so downstream nodes also arm")

	lsp, ok := e.table.Get(keyFromMessage(fwd))
	require.True(t, ok)
	require.NotNil(t, lsp.Bypass, "PLR armed a bypass")
	// RFC requirement: RFC4090-3.2-2 positive -- for link protection the PLR selects the bypass whose merge point is the NHOP.
	assert.Equal(t, netip.MustParseAddr("10.0.0.3"), lsp.Bypass.TunnelEndpoint, "bypass merges at the NHOP")
	assert.True(t, lsp.Bypass.TunnelID >= bypassTunnelIDBase, "bypass uses the reserved tunnel-id range")

	// The egress RESV comes back; the PLR's relayed RESV records protection
	// available in its own RRO subobject (RFC 4090 Section 4.4).
	mp := netip.MustParseAddr("10.0.0.3")
	rsb := &resvStateBlock{Session: psb.Session, Label: labelObject{Label: 18000}, Style: StyleSharedExplicit}
	e.handlePacket(Packet{Src: mp, Payload: buildResv(rsb, psb.SenderTemplate, DefaultRefreshPeriod, mp)})

	relayed, rdst, ok := ft.lastByType(MsgTypeResv)
	require.True(t, ok, "PLR relays a RESV upstream")
	assert.Equal(t, ingress, rdst, "RESV relayed toward the head-end")
	require.True(t, relayed.HasRRO)
	require.NotEmpty(t, relayed.RRO)
	assert.Equal(t, netip.MustParseAddr("10.0.0.2"), relayed.RRO[0].Address, "PLR's own RRO subobject is first")
	// RFC requirement: RFC4090-4.4-1 positive -- once a bypass is armed the PLR's RRO subobject reports "local protection available" (0x01).
	assert.NotZero(t, relayed.RRO[0].Flags&RROFlagProtectionAvailable, "RRO reports local protection available")
}

// TestPLRNoBypassWithoutProtection: a PATH that requests no protection arms no
// bypass, so a non-protected LSP keeps the base behavior.
func TestPLRNoBypassWithoutProtection(t *testing.T) {
	e, ft, _ := plrEngine(t)
	ingress := netip.MustParseAddr("10.0.0.1")
	psb := protectedTransitPSB(nil) // no protection requested
	e.handlePacket(Packet{Src: ingress, Payload: buildPath(psb, ingress, 64)})

	fwd, _, ok := ft.lastByType(MsgTypePath)
	require.True(t, ok)
	assert.False(t, fwd.HasFastReroute, "no FAST_REROUTE relayed")
	lsp, ok := e.table.Get(keyFromMessage(fwd))
	require.True(t, ok)
	assert.Nil(t, lsp.Bypass, "no bypass armed")
	// RFC requirement: RFC4090-4.4-1 negative -- with no bypass armed rroProtectionFlags is 0, so the RRO reports no "local protection available" bit.
	assert.Zero(t, rroProtectionFlags(lsp), "no protection flags")
}

// TestPLRNoMatchingBypass: protection is requested but no configured bypass
// merges at the NHOP, so no bypass is armed (the LSP keeps base behavior).
func TestPLRNoMatchingBypass(t *testing.T) {
	e, _, _ := testEngine(t, "10.0.0.2", func(c *rsvpteConfig) {
		c.Interfaces = []ifaceConfig{{Name: "eth0", MaxBW: 10e9, MaxReservableBW: 10e9}}
		c.Bypasses = []bypassConfig{{Name: "bp", MergePoint: netip.MustParseAddr("10.9.9.9")}} // wrong MP
	})
	e.admission.setInterface("eth0", 10e9, 10e9)
	ingress := netip.MustParseAddr("10.0.0.1")
	psb := protectedTransitPSB(&protectionRequest{Facility: true, HopLimit: 16})
	e.handlePacket(Packet{Src: ingress, Payload: buildPath(psb, ingress, 64)})

	key := lspKey{TunnelEndpoint: netip.MustParseAddr("10.0.0.9"), TunnelID: 1, ExtTunnelID: 0x0a000001, SenderAddr: ingress, LSPID: 1}
	lsp, ok := e.table.Get(key)
	require.True(t, ok)
	assert.Nil(t, lsp.Bypass, "no bypass merges at the NHOP -> none armed")
}

// TestNodeProtectionDisarmsWithoutNNHOPLabel: if the NNHOP does not record its
// label (a peer that ignores label-recording-desired), node protection cannot
// resolve the correct inner label, so the PLR disarms the bypass rather than push
// the wrong (NHOP) label and blackhole on a node failure.
func TestNodeProtectionDisarmsWithoutNNHOPLabel(t *testing.T) {
	e, fib := nodePLR(t)
	ingress := netip.MustParseAddr("10.0.0.1")
	nhop := netip.MustParseAddr("10.0.0.3")
	psb := nodeProtectionPSB()
	e.handlePacket(Packet{Src: ingress, Payload: buildPath(psb, ingress, 64)})

	armed, ok := e.table.Get(protectedKey())
	require.True(t, ok)
	require.NotNil(t, armed.Bypass, "bypass armed at PATH time")

	// RESV from the NHOP records the NHOP's label but NOT the NNHOP's.
	rsb := &resvStateBlock{
		Session: psb.Session, Label: labelObject{Label: 18000}, Style: StyleSharedExplicit,
		RRO: []rroEntry{{Type: RROSubIPv4, Address: nhop}},
	}
	e.handlePacket(Packet{Src: nhop, Payload: buildResv(rsb, psb.SenderTemplate, DefaultRefreshPeriod, nhop)})

	repaired, _ := e.table.Get(protectedKey())
	repaired.mu.Lock()
	bypass := repaired.Bypass
	backupLabel := repaired.BackupLabel
	repaired.mu.Unlock()
	assert.Nil(t, bypass, "node protection disarmed when the NNHOP label is unavailable")
	assert.Zero(t, backupLabel, "no resolved merge-point label (not the NHOP's)")

	// A PATH refresh re-arms the bypass, but the merge-point label is still
	// unresolved (BackupLabel == 0), so a link failure must NOT push the wrong
	// (NHOP) label -- it falls back to teardown, not a blackholing local repair.
	e.handlePacket(Packet{Src: ingress, Payload: buildPath(psb, ingress, 64)})
	bringBypassUp(t, e, 5000, netip.MustParseAddr("10.0.1.4"))
	e.handleLinkDown("eth0")
	assert.Empty(t, fib.backups, "no backup programmed despite a PATH-refresh re-arm")
	_, alive := e.table.Get(protectedKey())
	assert.False(t, alive, "unprotectable LSP torn down on link failure")
}

// TestLocalRepairSwitchesFIB: when the protected link fails, the PLR programs a
// 2-label backup swap (bypass label over the protected label) via the bypass next
// hop and RETAINS the protected LSP, marking protection in use (AC-3).
func TestLocalRepairSwitchesFIB(t *testing.T) {
	e, _, fib := plrEngine(t)
	protectedIn := armAndUpProtected(t, e)
	bringBypassUp(t, e, 5000, netip.MustParseAddr("10.0.1.3"))

	e.handleLinkDown("eth0") // the protected link fails

	require.Len(t, fib.backups, 1, "local repair programs exactly one backup swap")
	bk := fib.backups[0]
	assert.Equal(t, protectedIn, bk.in, "keyed by the protected in-label")
	// RFC requirement: RFC4090-3.2-1 positive -- facility backup stacks the bypass label on top of the swapped protected label.
	assert.Equal(t, []uint32{5000, 18000}, bk.out, "bypass label outermost, then the swapped protected label")
	assert.Equal(t, netip.MustParseAddr("10.0.1.3"), bk.nextHop, "forwarded via the bypass next hop")

	// RFC requirement: RFC4090-6.5-2 positive -- on a successful local repair the protected LSP is retained (not torn down).
	repaired, ok := e.table.Get(protectedKey())
	require.True(t, ok, "protected LSP is retained, not torn down")
	repaired.mu.Lock()
	inUse := repaired.ProtectionInUse
	repaired.mu.Unlock()
	assert.True(t, inUse, "protection marked in use")

	// The bypass LSP (egress via eth1) is untouched by the eth0 failure.
	_, bypassAlive := e.table.Get(bypassKey(e.cfg().Bypasses[0], e.cfg().RouterID))
	assert.True(t, bypassAlive, "bypass LSP survives the protected-link failure")
}

// TestLocalRepairSendsNotify: on local repair the PLR sends a PathErr Notify
// (Error Code 25, value 3 "Tunnel locally repaired") toward the head-end, and
// does NOT send the base teardown PathErr (AC-4).
func TestLocalRepairSendsNotify(t *testing.T) {
	e, ft, _ := plrEngine(t)
	armAndUpProtected(t, e)
	bringBypassUp(t, e, 5000, netip.MustParseAddr("10.0.1.3"))

	e.handleLinkDown("eth0")

	perr, dst, ok := ft.lastByType(MsgTypePathErr)
	require.True(t, ok, "PLR sends a PathErr on local repair")
	assert.Equal(t, netip.MustParseAddr("10.0.0.1"), dst, "Notify goes toward the head-end (prev hop)")
	// RFC requirement: RFC4090-6.5-1 positive -- a local repair sends a PathErr Notify (Error Code 25, value 3 "Tunnel locally repaired") toward the head-end.
	assert.Equal(t, ErrCodeNotify, perr.ErrorSpec.ErrorCode, "Error Code 25 (Notify)")
	assert.Equal(t, ErrValueTunnelLocallyRepaired, perr.ErrorSpec.ErrorValue, "value 3 (Tunnel locally repaired)")
}

// TestLocalRepairFallsBackToTeardown: a protected LSP whose bypass is not up
// cannot be repaired, so the PLR falls back to the base teardown + no-route
// PathErr (the LSP is removed).
func TestLocalRepairFallsBackToTeardown(t *testing.T) {
	e, ft, fib := plrEngine(t)
	armAndUpProtected(t, e)
	// Bypass is NOT brought up.

	e.handleLinkDown("eth0")

	// RFC requirement: RFC4090-3.2-1 negative -- with no usable bypass no label-stacked backup swap is programmed.
	assert.Empty(t, fib.backups, "no backup programmed when the bypass is not ready")
	_, alive := e.table.Get(protectedKey())
	// RFC requirement: RFC4090-6.5-2 negative -- an unrepairable protected LSP falls back to teardown (the LSP is removed).
	assert.False(t, alive, "unrepairable protected LSP is torn down")
	perr, _, ok := ft.lastByType(MsgTypePathErr)
	require.True(t, ok)
	// RFC requirement: RFC4090-6.5-1 negative -- an unrepairable failure sends the base no-route PathErr (Error Code 24), not a Notify.
	assert.Equal(t, ErrCodeRoutingProblem, perr.ErrorSpec.ErrorCode, "base no-route PathErr, not a Notify")
}

// TestLocalRepairIdempotent: a repeated link-down event for an already-repaired
// LSP keeps the repair (no duplicate FIB program, LSP still retained).
func TestLocalRepairIdempotent(t *testing.T) {
	e, _, fib := plrEngine(t)
	armAndUpProtected(t, e)
	bringBypassUp(t, e, 5000, netip.MustParseAddr("10.0.1.3"))

	e.handleLinkDown("eth0")
	e.handleLinkDown("eth0")

	assert.Len(t, fib.backups, 1, "repair programmed once despite repeated link-down events")
	_, alive := e.table.Get(protectedKey())
	assert.True(t, alive, "LSP stays repaired, not torn down")
}

// TestFacilityBypassProtectsMultipleLSPs: one configured bypass protects several
// LSPs crossing the same link -- each is redirected onto the same bypass with its
// own label-stacked swap on a failure (AC-7, the defining facility-backup property).
func TestFacilityBypassProtectsMultipleLSPs(t *testing.T) {
	e, _, fib := plrEngine(t)
	ingress := netip.MustParseAddr("10.0.0.1")
	mp := netip.MustParseAddr("10.0.0.3")
	bringBypassUp(t, e, 5000, netip.MustParseAddr("10.0.1.3"))

	for _, tid := range []uint16{1, 2} {
		psb := protectedTransitPSB(&protectionRequest{Facility: true, HopLimit: 16, Bandwidth: 1e7})
		psb.Session.TunnelID = tid
		e.handlePacket(Packet{Src: ingress, Payload: buildPath(psb, ingress, 64)})
		rsb := &resvStateBlock{Session: psb.Session, Label: labelObject{Label: uint32(18000 + tid)}, Style: StyleSharedExplicit}
		e.handlePacket(Packet{Src: mp, Payload: buildResv(rsb, psb.SenderTemplate, DefaultRefreshPeriod, mp)})
	}

	k1 := protectedKey()
	k1.TunnelID = 1
	k2 := protectedKey()
	k2.TunnelID = 2
	l1, ok1 := e.table.Get(k1)
	l2, ok2 := e.table.Get(k2)
	require.True(t, ok1)
	require.True(t, ok2)
	require.NotNil(t, l1.Bypass)
	require.NotNil(t, l2.Bypass)
	assert.Equal(t, *l1.Bypass, *l2.Bypass, "both protected LSPs share the one configured bypass")

	e.handleLinkDown("eth0")
	assert.Len(t, fib.backups, 2, "one bypass carries both protected LSPs, each label-stacked")
}

// TestEngineConfigReloadPicksUpBypass: a bypass added by a config reload is seen
// by selectBypass (the engine reads its config via the atomic pointer, not a copy
// frozen at startup). Regression for "FRR breaks after a config reload".
func TestEngineConfigReloadPicksUpBypass(t *testing.T) {
	e, _, _ := testEngine(t, "10.0.0.2", func(c *rsvpteConfig) {
		c.Interfaces = []ifaceConfig{
			{Name: "eth0", Prefix: netip.MustParsePrefix("10.0.0.0/24"), MaxBW: 10e9, MaxReservableBW: 10e9},
		}
	})
	e.admission.setInterface("eth0", 10e9, 10e9)
	ingress := netip.MustParseAddr("10.0.0.1")
	psb := protectedTransitPSB(&protectionRequest{Facility: true, HopLimit: 16, Bandwidth: 1e8})
	e.handlePacket(Packet{Src: ingress, Payload: buildPath(psb, ingress, 64)})
	lsp, ok := e.table.Get(protectedKey())
	require.True(t, ok)
	require.Nil(t, lsp.Bypass, "no bypass armed before the reload")

	// Reload: add a bypass merging at the NHOP.
	cfg := e.cfg()
	cfg.Bypasses = []bypassConfig{{
		Name: "bp", MergePoint: netip.MustParseAddr("10.0.0.3"),
		ERO: []eroHop{{Address: netip.MustParsePrefix("10.0.1.3/32")}},
	}}
	e.setConfig(cfg)

	// A PATH refresh now arms the reloaded bypass.
	e.handlePacket(Packet{Src: ingress, Payload: buildPath(psb, ingress, 64)})
	lsp2, ok := e.table.Get(protectedKey())
	require.True(t, ok)
	require.NotNil(t, lsp2.Bypass, "reloaded bypass armed after the config reload")
	assert.Equal(t, netip.MustParseAddr("10.0.0.3"), lsp2.Bypass.TunnelEndpoint)
}

// TestConfigReloadRouterIDChangeNoSplitBrain: when a reload changes the router-id
// to a different valid IPv4, the engine keeps the running router-id (restart-class)
// AND reconcile signals bypasses under that same router-id, so selectBypass resolves
// them and FRR protection still works (no split-brain between the signaled key and
// the engine's runtime read). Mirrors OnConfigApply's fixed flow.
func TestConfigReloadRouterIDChangeNoSplitBrain(t *testing.T) {
	e, _, _ := testEngine(t, "10.0.0.2", func(c *rsvpteConfig) {
		c.Interfaces = []ifaceConfig{
			{Name: "eth0", Prefix: netip.MustParsePrefix("10.0.0.0/24"), MaxBW: 10e9, MaxReservableBW: 10e9},
		}
		c.Bypasses = []bypassConfig{{
			Name: "bp", MergePoint: netip.MustParseAddr("10.0.0.3"),
			ERO: []eroHop{{Address: netip.MustParsePrefix("10.0.1.3/32")}},
		}}
	})

	// Reload changes the router-id to a different valid IPv4.
	newCfg := e.cfg()
	newCfg.RouterID = netip.MustParseAddr("10.0.0.99")
	e.setConfig(newCfg)
	effective := e.cfg()
	require.Equal(t, "10.0.0.2", effective.RouterID.String(), "router-id change ignored (restart-class)")

	// OnConfigApply reconciles against the engine's effective config (the fix).
	reconcileTunnels(slogutil.DiscardLogger(), e.table, effective, e, nil)

	// The bypass is signaled under the preserved router-id, which is exactly the key
	// selectBypass resolves -- protection actually works.
	want := bypassKey(effective.Bypasses[0], effective.RouterID)
	_, ok := e.table.Get(want)
	assert.True(t, ok, "bypass keyed under the engine's effective router-id")
	got, sel := e.selectBypass([]eroHop{{Address: netip.MustParsePrefix("10.0.0.3/32")}}, &protectionRequest{Facility: true})
	require.True(t, sel, "selectBypass matches the configured merge point")
	assert.Equal(t, want, got, "selectBypass resolves the same key reconcile signaled")
}

// TestSetConfigPreservesRouterID: a reload that removes/changes the router-id must
// not be adopted at runtime (RouterID is the LSR identity, restart-class), or the
// engine's As4-based key/encode reads would panic on the next PATH.
func TestSetConfigPreservesRouterID(t *testing.T) {
	e, _, _ := testEngine(t, "10.0.0.2", func(c *rsvpteConfig) {
		c.Interfaces = []ifaceConfig{
			{Name: "eth0", Prefix: netip.MustParsePrefix("10.0.0.0/24"), MaxBW: 10e9, MaxReservableBW: 10e9},
		}
	})
	require.Equal(t, "10.0.0.2", e.cfg().RouterID.String())

	// Reload to a config that drops the router-id but adds a bypass.
	e.setConfig(rsvpteConfig{
		Bypasses: []bypassConfig{{Name: "bp", MergePoint: netip.MustParseAddr("10.0.0.3")}},
	})
	assert.Equal(t, "10.0.0.2", e.cfg().RouterID.String(), "router-id preserved across reload")
	require.Len(t, e.cfg().Bypasses, 1, "reloaded bypasses still adopted")

	// A protection-desired PATH still relays (buildPath uses the router-id) without
	// panicking on As4 of a zero Addr.
	ingress := netip.MustParseAddr("10.0.0.1")
	psb := protectedTransitPSB(&protectionRequest{Facility: true, HopLimit: 16, Bandwidth: 1e8})
	require.NotPanics(t, func() {
		e.handlePacket(Packet{Src: ingress, Payload: buildPath(psb, ingress, 64)})
	})
}

// TestBypassTeardownClearsProtection: when a bypass LSP is torn down, protected
// LSPs that armed it stop reporting protection available (no stale flag).
func TestBypassTeardownClearsProtection(t *testing.T) {
	e, _, _ := plrEngine(t)
	armAndUpProtected(t, e)
	bringBypassUp(t, e, 5000, netip.MustParseAddr("10.0.1.3"))

	prot, ok := e.table.Get(protectedKey())
	require.True(t, ok)
	prot.mu.Lock()
	armed := prot.Bypass != nil
	prot.mu.Unlock()
	require.True(t, armed, "protected LSP armed before bypass teardown")

	// Tear the bypass down (e.g. its own link failed or it expired).
	e.tearLSPLocal(bypassKey(e.cfg().Bypasses[0], e.cfg().RouterID))

	prot.mu.Lock()
	stillArmed := prot.Bypass != nil
	flags := rroProtectionFlags(prot)
	prot.mu.Unlock()
	assert.False(t, stillArmed, "stale bypass association cleared on bypass teardown")
	assert.Zero(t, flags, "no protection-available flag once the bypass is gone")
}

// headEndEngine builds an ingress head-end (10.0.0.1) with one up tunnel LSP.
func headEndEngine(t *testing.T) (*engine, *fakeTransport, lspKey) {
	t.Helper()
	e, ft, _ := testEngine(t, "10.0.0.1", nil)
	rid := netip.MustParseAddr("10.0.0.1")
	key := lspKey{TunnelEndpoint: netip.MustParseAddr("10.0.0.9"), TunnelID: 1, ExtTunnelID: addrToUint32(rid), SenderAddr: rid, LSPID: 1}
	lsp, _ := e.table.GetOrCreate(key)
	lsp.mu.Lock()
	lsp.Role = RoleIngress
	lsp.NextHop = netip.MustParseAddr("10.0.0.2")
	lsp.PSB = &pathStateBlock{
		Session:        sessionIPv4{TunnelEndpoint: key.TunnelEndpoint, TunnelID: 1, ExtTunnelID: key.ExtTunnelID},
		SenderTemplate: senderTemplateIPv4{SenderAddr: rid, LSPID: 1},
		ERO:            []eroHop{{Address: netip.MustParsePrefix("10.0.0.9/32")}},
		SenderTSpec:    FlowSpec{TokenRate: 1e8},
		LabelRequest:   labelRequest{L3PID: 0x0800},
	}
	lsp.setState(LSPStateUp)
	lsp.mu.Unlock()
	return e, ft, key
}

// notifyFor builds a Notify PathErr (code 25/3) for the given LSP from a PLR.
func notifyFor(key lspKey) []byte {
	es := errorSpec{ErrorNode: netip.MustParseAddr("10.0.0.2"), ErrorCode: ErrCodeNotify, ErrorValue: ErrValueTunnelLocallyRepaired}
	session := sessionIPv4{TunnelEndpoint: key.TunnelEndpoint, TunnelID: key.TunnelID, ExtTunnelID: key.ExtTunnelID}
	sender := senderTemplateIPv4{SenderAddr: key.SenderAddr, LSPID: key.LSPID}
	return buildPathErr(session, sender, FlowSpec{TokenRate: 1e8}, es, netip.MustParseAddr("10.0.0.2"))
}

// TestHeadEndReoptimizesOnNotify: the head-end, on a Notify, starts a
// make-before-break reroute (a new PATH with the next LSP_ID) (AC-5).
func TestHeadEndReoptimizesOnNotify(t *testing.T) {
	e, ft, key := headEndEngine(t)
	e.handlePacket(Packet{Src: netip.MustParseAddr("10.0.0.2"), Payload: notifyFor(key)})

	newKey := key
	newKey.LSPID = 2
	_, ok := e.table.Get(newKey)
	require.True(t, ok, "head-end signaled a make-before-break replacement LSP")
	path, _, ok := ft.lastByType(MsgTypePath)
	require.True(t, ok, "a fresh PATH is sent")
	assert.Equal(t, uint16(2), path.SenderTemplate.LSPID, "replacement uses the next LSP_ID")
}

// TestHeadEndReoptimizeIdempotent: repeated Notifies do not spawn a storm of
// replacement LSPs while one reroute is already in flight.
func TestHeadEndReoptimizeIdempotent(t *testing.T) {
	e, _, key := headEndEngine(t)
	e.handlePacket(Packet{Src: netip.MustParseAddr("10.0.0.2"), Payload: notifyFor(key)})
	e.handlePacket(Packet{Src: netip.MustParseAddr("10.0.0.2"), Payload: notifyFor(key)})

	thirdKey := key
	thirdKey.LSPID = 3
	_, exists := e.table.Get(thirdKey)
	assert.False(t, exists, "only one replacement LSP is created for repeated Notifies")
}

// nodeProtectionPSB is a head-end PSB for an A->PLR->NHOP->NNHOP->egress LSP
// requesting node protection.
func nodeProtectionPSB() *pathStateBlock {
	return &pathStateBlock{
		Session:        sessionIPv4{TunnelEndpoint: netip.MustParseAddr("10.0.0.9"), TunnelID: 1, ExtTunnelID: 0x0a000001},
		SenderTemplate: senderTemplateIPv4{SenderAddr: netip.MustParseAddr("10.0.0.1"), LSPID: 1},
		ERO: []eroHop{
			{Address: netip.MustParsePrefix("10.0.0.2/32")}, // PLR
			{Address: netip.MustParsePrefix("10.0.0.3/32")}, // NHOP
			{Address: netip.MustParsePrefix("10.0.0.4/32")}, // NNHOP (merge point)
			{Address: netip.MustParsePrefix("10.0.0.9/32")}, // egress
		},
		SenderTSpec:  FlowSpec{TokenRate: 1e8, TokenBucket: 1e8, PeakRate: 1e8},
		LabelRequest: labelRequest{L3PID: 0x0800},
		Protection:   &protectionRequest{Facility: true, NodeProtection: true, HopLimit: 16},
	}
}

// nodePLR builds a PLR with a node-protection bypass that merges at the NNHOP.
func nodePLR(t *testing.T) (*engine, *fakeFIB) {
	t.Helper()
	e, _, fib := testEngine(t, "10.0.0.2", func(c *rsvpteConfig) {
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
	return e, fib
}

// TestNodeProtectionLocalRepair: node protection selects a bypass that merges at
// the NNHOP and, on a link failure, pushes the NNHOP's RECORDED label (not the
// NHOP's) under the bypass label -- so traffic survives the next node failing
// without blackholing on the wrong inner label (AC-6).
func TestNodeProtectionLocalRepair(t *testing.T) {
	e, fib := nodePLR(t)
	ingress := netip.MustParseAddr("10.0.0.1")
	nhop := netip.MustParseAddr("10.0.0.3")
	psb := nodeProtectionPSB()
	e.handlePacket(Packet{Src: ingress, Payload: buildPath(psb, ingress, 64)})

	lsp, ok := e.table.Get(protectedKey())
	require.True(t, ok)
	require.NotNil(t, lsp.Bypass, "node-protection bypass armed")
	// RFC requirement: RFC4090-3.2-2 positive -- for node protection the PLR selects the bypass whose merge point is the NNHOP (not the NHOP).
	assert.Equal(t, netip.MustParseAddr("10.0.0.4"), lsp.Bypass.TunnelEndpoint, "bypass merges at the NNHOP, not the NHOP")

	// RESV from the NHOP carrying label recording: NHOP label 18000, NNHOP 7777.
	rsb := &resvStateBlock{
		Session: psb.Session, Label: labelObject{Label: 18000}, Style: StyleSharedExplicit,
		RRO: []rroEntry{
			{Type: RROSubIPv4, Address: nhop}, {Type: RROSubLabel, Label: 18000},
			{Type: RROSubIPv4, Address: netip.MustParseAddr("10.0.0.4")}, {Type: RROSubLabel, Label: 7777},
		},
	}
	e.handlePacket(Packet{Src: nhop, Payload: buildResv(rsb, psb.SenderTemplate, DefaultRefreshPeriod, nhop)})

	repaired, _ := e.table.Get(protectedKey())
	repaired.mu.Lock()
	backupLabel := repaired.BackupLabel
	repaired.mu.Unlock()
	assert.Equal(t, uint32(7777), backupLabel, "node protection resolves the NNHOP's recorded label")

	bringBypassUp(t, e, 5000, netip.MustParseAddr("10.0.1.4"))
	e.handleLinkDown("eth0")

	require.Len(t, fib.backups, 1)
	assert.Equal(t, []uint32{5000, 7777}, fib.backups[0].out, "backup stack pushes the bypass label over the NNHOP label")
	assert.Equal(t, netip.MustParseAddr("10.0.1.4"), fib.backups[0].nextHop, "forwarded via the node bypass next hop")
}

// TestNodeProtectionNeedsNodeBypass: a node-protection request is not satisfied
// by a link-only bypass (one that merges at the NHOP), so nothing is armed.
func TestNodeProtectionNeedsNodeBypass(t *testing.T) {
	e, _, _ := testEngine(t, "10.0.0.2", func(c *rsvpteConfig) {
		c.Interfaces = []ifaceConfig{
			{Name: "eth0", Prefix: netip.MustParsePrefix("10.0.0.0/24"), MaxBW: 10e9, MaxReservableBW: 10e9},
		}
		// Only a link bypass at the NHOP, no node bypass at the NNHOP.
		c.Bypasses = []bypassConfig{{Name: "link-bp", MergePoint: netip.MustParseAddr("10.0.0.3")}}
	})
	e.admission.setInterface("eth0", 10e9, 10e9)
	e.handlePacket(Packet{Src: netip.MustParseAddr("10.0.0.1"), Payload: buildPath(nodeProtectionPSB(), netip.MustParseAddr("10.0.0.1"), 64)})
	lsp, ok := e.table.Get(protectedKey())
	require.True(t, ok)
	// RFC requirement: RFC4090-3.2-2 negative -- a node-protection request is not satisfied by a link-only bypass merging at the NHOP, so no bypass is armed.
	assert.Nil(t, lsp.Bypass, "node protection is not satisfied by a link-only bypass")
}

// TestShowFastReroute reports protected LSPs (with their armed bypass and
// available flag) and the configured bypass LSPs (AC-8).
func TestShowFastReroute(t *testing.T) {
	e, _, _ := plrEngine(t)
	armAndUpProtected(t, e)
	bringBypassUp(t, e, 5000, netip.MustParseAddr("10.0.1.3"))

	js, err := json.Marshal(showFastReroute(e.table))
	require.NoError(t, err)
	s := string(js)
	assert.Contains(t, s, `"kind":"protected"`, "reports the protected transit LSP")
	assert.Contains(t, s, `"kind":"bypass"`, "reports the configured bypass LSP")
	assert.Contains(t, s, `"protection-available":true`, "protected LSP shows protection available")
	assert.Contains(t, s, `"mode":"facility"`, "facility mode reported")
}

// TestHeadEndIgnoresNonNotifyPathErr: a normal (non-Notify) PathErr does not
// trigger re-optimization.
func TestHeadEndIgnoresNonNotifyPathErr(t *testing.T) {
	e, _, key := headEndEngine(t)
	es := errorSpec{ErrorNode: netip.MustParseAddr("10.0.0.2"), ErrorCode: ErrCodeRoutingProblem, ErrorValue: ErrValueNoRouteAvailable}
	session := sessionIPv4{TunnelEndpoint: key.TunnelEndpoint, TunnelID: key.TunnelID, ExtTunnelID: key.ExtTunnelID}
	sender := senderTemplateIPv4{SenderAddr: key.SenderAddr, LSPID: key.LSPID}
	raw := buildPathErr(session, sender, FlowSpec{TokenRate: 1e8}, es, netip.MustParseAddr("10.0.0.2"))
	e.handlePacket(Packet{Src: netip.MustParseAddr("10.0.0.2"), Payload: raw})

	newKey := key
	newKey.LSPID = 2
	_, exists := e.table.Get(newKey)
	assert.False(t, exists, "a non-Notify PathErr must not trigger a reroute")
}

// TestFastRerouteOneToOneMethodFlag: the one-to-one (detour) backup method sets the
// FAST_REROUTE one-to-one Flags bit and leaves the facility bit clear -- the
// complement of the facility case (RFC 4090 Section 4.1 method flags). This exercises
// the non-facility branch of protectionRequest.fastReroute (frr.go).
func TestFastRerouteOneToOneMethodFlag(t *testing.T) {
	fr := (&protectionRequest{Facility: false}).fastReroute()
	// RFC requirement: RFC4090-4.1-2 negative -- a one-to-one backup request sets FRR Flags 0x01 and leaves the facility bit 0x02 clear (the other method's flag).
	assert.NotZero(t, fr.Flags&FRRFlagOneToOneBackup, "one-to-one backup flag set")
	assert.Zero(t, fr.Flags&FRRFlagFacilityBackup, "facility bit clear for a one-to-one request")
}

// TestRROProtectionFlagsReflectState drives rroProtectionFlags (frr.go) through the
// PLR states so each RFC 4090 Section 4.4 RRO subobject flag tracks the LSP:
// protection-in-use only after a local repair, and node protection only when the
// head-end requested it.
func TestRROProtectionFlagsReflectState(t *testing.T) {
	bp := lspKey{TunnelEndpoint: netip.MustParseAddr("10.0.0.3"), TunnelID: bypassTunnelIDBase, LSPID: 1}

	// A link-protected LSP: bypass armed, no local repair yet, node protection not requested.
	linkOnly := &LSP{Bypass: &bp, PSB: &pathStateBlock{Protection: &protectionRequest{Facility: true}}}
	linkFlags := rroProtectionFlags(linkOnly)
	require.NotZero(t, linkFlags&RROFlagProtectionAvailable, "available once a bypass is armed")
	// RFC requirement: RFC4090-4.4-2 negative -- "local protection in use" (0x02) stays clear while a bypass is armed but no local repair has redirected traffic.
	assert.Zero(t, linkFlags&RROFlagProtectionInUse, "in-use clear before local repair")
	// RFC requirement: RFC4090-4.4-3 negative -- "node protection" (0x08) stays clear for a link-only protection request.
	assert.Zero(t, linkFlags&RROFlagNodeProtection, "node bit clear for link-only protection")

	// Once a local repair marks the LSP in use (tryLocalRepair sets ProtectionInUse),
	// the RRO reports "local protection in use".
	linkOnly.ProtectionInUse = true
	// RFC requirement: RFC4090-4.4-2 positive -- "local protection in use" (0x02) is set once traffic is on the backup (ProtectionInUse set).
	assert.NotZero(t, rroProtectionFlags(linkOnly)&RROFlagProtectionInUse, "in-use set once on the backup")

	// A node-protection request sets the node-protection bit.
	nodeProt := &LSP{Bypass: &bp, PSB: &pathStateBlock{Protection: &protectionRequest{Facility: true, NodeProtection: true}}}
	// RFC requirement: RFC4090-4.4-3 positive -- "node protection" (0x08) is set when the head-end requested node protection.
	assert.NotZero(t, rroProtectionFlags(nodeProt)&RROFlagNodeProtection, "node bit set when node protection requested")
}
