// Design: docs/architecture/rsvpte/mpls-rsvp-te.md -- the PLR reports "local protection
// available" from the bypass LSP's state, not from the configuration match (RFC 4090
// Section 6).
package rsvpte

import (
	"net/netip"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// plrRelayedRROFlags signals the protected PATH through a plrEngine, optionally
// brings the armed bypass up, feeds the merge point's RESV and returns the flags
// the PLR wrote on its own RRO subobject of the relayed RESV.
func plrRelayedRROFlags(t *testing.T, bypassUp bool) uint8 {
	t.Helper()
	e, ft, _ := plrEngine(t)
	ingress := netip.MustParseAddr("10.0.0.1")
	psb := protectedTransitPSB(&protectionRequest{Facility: true, HopLimit: 16, Bandwidth: 1e8, SetupPrio: 7, HoldPrio: 7, Name: "t1"})
	e.handlePacket(Packet{Src: ingress, Payload: buildPath(psb, ingress, 64)})
	lsp, ok := e.table.Get(protectedKey())
	require.True(t, ok)
	require.NotNil(t, lsp.Bypass, "PLR armed a bypass")
	if bypassUp {
		bringBypassUp(t, e, 5000, netip.MustParseAddr("10.0.1.3"))
	}

	mp := netip.MustParseAddr("10.0.0.3")
	rsb := &resvStateBlock{Session: psb.Session, Label: labelObject{Label: 18000}, Style: StyleSharedExplicit}
	e.handlePacket(Packet{Src: mp, Payload: buildResv(rsb, psb.SenderTemplate, DefaultRefreshPeriod, mp)})

	relayed, _, ok := ft.lastByType(MsgTypeResv)
	require.True(t, ok, "PLR relays a RESV upstream")
	require.Len(t, relayed.FlowDescriptors, 1)
	require.Len(t, relayed.FlowDescriptors[0].Filters, 1)
	filter := relayed.FlowDescriptors[0].Filters[0]
	require.True(t, filter.HasRRO)
	require.NotEmpty(t, filter.RRO)
	require.Equal(t, netip.MustParseAddr("10.0.0.2"), filter.RRO[0].Address, "PLR's own RRO subobject is first")
	return filter.RRO[0].Flags
}

// RFC requirement: RFC4090-6-3 negative — while the armed bypass LSP is not yet up the PLR's RRO subobject carries none of the four protection flags.
// RFC requirement: RFC4090-6-4 negative — with the bypass tunnel not established the PLR's RRO subobject does not carry "local protection available".
func TestRFC4090FlagsClearUntilBypassEstablished(t *testing.T) {
	flags := plrRelayedRROFlags(t, false)
	assert.Zero(t, flags&RROFlagProtectionAvailable, "local protection available stays clear")
	assert.Zero(t, flags, "all four flags stay clear")
}

// RFC requirement: RFC4090-6-3 positive — once the armed bypass LSP is up the PLR sets the protection flags in its RRO subobject.
// RFC requirement: RFC4090-6-4 positive — with the bypass tunnel established the PLR's RRO subobject carries "local protection available".
func TestRFC4090FlagsSetOnceBypassEstablished(t *testing.T) {
	flags := plrRelayedRROFlags(t, true)
	assert.NotZero(t, flags&RROFlagProtectionAvailable, "local protection available is set")
}

// TestRFC4090RefreshTracksBypassState changes the bypass without another
// downstream RESV. The PLR must update the flags in its own periodic refresh.
// RFC requirement: RFC4090-6-4 negative -- a RESV refresh clears local protection available after the bypass goes down.
func TestRFC4090RefreshTracksBypassState(t *testing.T) {
	e, ft, _ := plrEngine(t)
	armAndUpProtected(t, e)
	lsp, ok := e.table.Get(protectedKey())
	require.True(t, ok)
	bringBypassUp(t, e, 5000, netip.MustParseAddr("10.0.1.3"))
	require.NoError(t, e.sendResv(lsp))
	up, _, ok := ft.lastByType(MsgTypeResv)
	require.True(t, ok)
	require.Len(t, up.FlowDescriptors, 1)
	require.Len(t, up.FlowDescriptors[0].Filters, 1)
	require.True(t, up.FlowDescriptors[0].Filters[0].HasRRO)
	require.NotEmpty(t, up.FlowDescriptors[0].Filters[0].RRO)
	require.Equal(t, RROFlagProtectionAvailable, up.FlowDescriptors[0].Filters[0].RRO[0].Flags)

	bypass, ok := e.table.Get(*lsp.Bypass)
	require.True(t, ok)
	bypass.mu.Lock()
	bypass.setState(LSPStateDown)
	bypass.mu.Unlock()
	require.NoError(t, e.sendResv(lsp))
	down, _, ok := ft.lastByType(MsgTypeResv)
	require.True(t, ok)
	require.Len(t, down.FlowDescriptors, 1)
	require.Len(t, down.FlowDescriptors[0].Filters, 1)
	require.True(t, down.FlowDescriptors[0].Filters[0].HasRRO)
	require.NotEmpty(t, down.FlowDescriptors[0].Filters[0].RRO)
	assert.Zero(t, down.FlowDescriptors[0].Filters[0].RRO[0].Flags, "cached RESV flags must track the live bypass")
}

// TestRefreshDoesNotAdvertiseUnresolvedNodeRepair recreates a PATH refresh that
// re-arms a node bypass before the merge point has supplied its protected label.
func TestRefreshDoesNotAdvertiseUnresolvedNodeRepair(t *testing.T) {
	e, ft := rfc4090NodePLR(t)
	ingress := netip.MustParseAddr("10.0.0.1")
	psb := nodeProtectionPSB()
	e.handlePacket(Packet{Src: ingress, Payload: buildPath(psb, ingress, 64)})
	mp, resv := resvWithRRO(1)
	e.handlePacket(Packet{Src: mp, Payload: resv})
	e.handlePacket(Packet{Src: ingress, Payload: buildPath(psb, ingress, 64)})
	bringBypassUp(t, e, 5000, netip.MustParseAddr("10.0.1.4"))
	lsp, ok := e.table.Get(protectedKey())
	require.True(t, ok)
	require.NoError(t, e.sendResv(lsp))
	refreshed, _, ok := ft.lastByType(MsgTypeResv)
	require.True(t, ok)
	require.Len(t, refreshed.FlowDescriptors, 1)
	require.Len(t, refreshed.FlowDescriptors[0].Filters, 1)
	filter := refreshed.FlowDescriptors[0].Filters[0]
	require.True(t, filter.HasRRO)
	require.NotEmpty(t, filter.RRO)
	assert.Zero(t, filter.RRO[0].Flags, "an unresolved merge label cannot protect this LSP")
}

// lastResvFor finds the reservation addressed to one branch of an MP merge.
func lastResvFor(t *testing.T, ft *fakeTransport, sender netip.Addr) []byte {
	t.Helper()
	ft.mu.Lock()
	defer ft.mu.Unlock()
	for index := range slices.Backward(ft.sent) {
		sent := &ft.sent[index]
		msg, err := DecodeMessage(sent.payload)
		if err != nil || msg.Header.MsgType != MsgTypeResv {
			continue
		}
		for _, descriptor := range msg.FlowDescriptors {
			for _, filter := range descriptor.Filters {
				if filter.Filter.SenderAddr == sender {
					return sent.payload
				}
			}
		}
	}
	t.Fatalf("no RESV for sender %s", sender)
	return nil
}

// TestRFC4090ProtectedPathSurvivesRepair exercises both engines through failure,
// refreshed labels, independent branch expiry, and final teardown.
// RFC requirement: RFC4090-6.4-1 positive -- the backup PATH carries the PLR's own RSVP_HOP.
// RFC requirement: RFC4090-6.4-2 positive -- the backup PATH carries an ERO from the MP to the egress.
// RFC requirement: RFC4090-6.4-3 positive -- node repair removes the failed node before the MP.
// RFC requirement: RFC4090-7.1-1 positive -- an MP forwards the protected sender, not the PLR's backup sender.
// RFC requirement: RFC4090-7.1-2 positive -- the MP refreshes that protected PATH without another incoming PATH.
func TestRFC4090ProtectedPathSurvivesRepair(t *testing.T) {
	for _, tc := range []struct {
		name   string
		node   bool
		egress bool
	}{{name: "link"}, {name: "node", node: true}, {name: "egress", node: true, egress: true}} {
		t.Run(tc.name, func(t *testing.T) {
			plr, pt, fib := plrEngine(t)
			ingress := netip.MustParseAddr("10.0.0.1")
			plrAddr := plr.cfg().RouterID
			nhop := netip.MustParseAddr("10.0.0.3")
			mpAddr := nhop
			psb := protectedTransitPSB(&protectionRequest{Facility: true})
			if tc.node {
				psb = nodeProtectionPSB()
				mpAddr = netip.MustParseAddr("10.0.0.4")
			}
			if tc.egress {
				mpAddr = psb.Session.TunnelEndpoint
				psb.ERO = psb.ERO[:2]
				psb.ERO = append(psb.ERO, eroHop{Address: netip.PrefixFrom(mpAddr, 32)})
			}
			cfg := plr.cfg()
			cfg.Bypasses[0].MergePoint = mpAddr
			cfg.Bypasses[0].NodeProtection = tc.node
			plr.setConfig(cfg)
			plr.handlePacket(Packet{Src: ingress, Payload: buildPath(psb, ingress, 64)})
			mp, mt, _ := testEngine(t, mpAddr.String(), nil)
			mpPath := *psb
			index := slices.IndexFunc(psb.ERO, func(h eroHop) bool { return h.Address.Addr() == mpAddr })
			require.NotEqual(t, -1, index)
			mpPath.ERO = psb.ERO[index:]
			previous := plrAddr
			if tc.node {
				previous = nhop
			}
			mp.handlePacket(Packet{Src: previous, Payload: buildPath(&mpPath, previous, 64)})
			if !tc.egress {
				tail := psb.Session.TunnelEndpoint
				rsb := &resvStateBlock{Session: psb.Session, Label: labelObject{Label: 19000}, Style: StyleSharedExplicit}
				mp.handlePacket(Packet{Src: tail, Payload: buildResv(rsb, psb.SenderTemplate, DefaultRefreshPeriod, tail)})
			}
			normal := mustDecode(t, lastResvFor(t, mt, ingress))
			require.Len(t, normal.FlowDescriptors, 1)
			require.Len(t, normal.FlowDescriptors[0].Filters, 1)
			normalFilter := normal.FlowDescriptors[0].Filters[0]
			reservation := &resvStateBlock{Session: psb.Session, Label: normalFilter.Label,
				Style: StyleSharedExplicit, RRO: normalFilter.RRO}
			plr.handlePacket(Packet{Src: nhop, Payload: buildResv(reservation, psb.SenderTemplate, DefaultRefreshPeriod, nhop)})
			bringBypassUp(t, plr, 5000, netip.MustParseAddr("10.0.1.3"))
			plr.handleLinkDown("eth0")

			backup, dst, ok := pt.lastByType(MsgTypePath)
			require.True(t, ok)
			assert.Equal(t, mpAddr, dst, "the bypass ingress FEC is the MP host address")
			assert.Equal(t, psb.Session, backup.Session)
			assert.Equal(t, plrAddr, backup.SenderTemplate.SenderAddr)
			assert.Equal(t, plrAddr, backup.Hop.NextHop)
			require.True(t, backup.HasERO)
			assert.Equal(t, mpAddr, backup.ERO[0].Address.Addr())
			assert.Equal(t, psb.ERO[index:], backup.ERO)
			assert.Zero(t, backup.SessionAttr.Flags&(SessAttrLocalProtection|SessAttrNodeProtection|SessAttrBandwidthProtection))
			raw, ok := pt.lastSentPayload(MsgTypePath)
			require.True(t, ok)
			mp.handlePacket(Packet{Src: plrAddr, Payload: raw})
			if !tc.egress {
				forwarded, _, ok := mt.lastByType(MsgTypePath)
				require.True(t, ok)
				assert.Equal(t, psb.SenderTemplate, forwarded.SenderTemplate, "MP preserves the protected downstream identity")
				before := mt.countByType(MsgTypePath)
				refreshPaths(mp.log, mp.table, mp)
				assert.Equal(t, before+1, mt.countByType(MsgTypePath), "MP refreshes the merged PATH periodically")
			}
			mpResv := lastResvFor(t, mt, plrAddr)
			merged := mustDecode(t, mpResv)
			require.Len(t, merged.FlowDescriptors, 1)
			require.Len(t, merged.FlowDescriptors[0].Filters, 1)
			assert.Equal(t, normalFilter.Label, merged.FlowDescriptors[0].Filters[0].Label, "merged branch retains the protected incoming label")
			swaps := len(fib.swapped)
			plr.handlePacket(Packet{Src: mpAddr, Payload: mpResv})
			assert.Len(t, fib.swapped, swaps, "backup RESV must not restore the failed out-segment")
			require.NotEmpty(t, fib.backups)
			assert.Equal(t, []uint32{5000, normalFilter.Label.Label}, fib.backups[len(fib.backups)-1].out)
			updated := &resvStateBlock{Session: psb.Session, Label: labelObject{Label: 21000},
				Style: StyleSharedExplicit, RRO: normalFilter.RRO}
			plr.handlePacket(Packet{Src: mpAddr, Payload: buildResv(updated, backup.SenderTemplate, DefaultRefreshPeriod, mpAddr)})
			assert.Equal(t, []uint32{5000, 21000}, fib.backups[len(fib.backups)-1].out, "MP label changes update the backup stack")
			plr.handlePacket(Packet{Src: nhop, Payload: buildResv(reservation, psb.SenderTemplate, DefaultRefreshPeriod, nhop)})
			assert.Len(t, fib.swapped, swaps, "a late normal RESV cannot undo local repair")
			assert.Equal(t, []uint32{5000, 21000}, fib.backups[len(fib.backups)-1].out)
			problem := errorSpec{ErrorNode: mpAddr, ErrorCode: ErrCodeRoutingProblem, ErrorValue: ErrValueNoRouteAvailable}
			plr.handlePacket(Packet{Src: mpAddr, Payload: buildPathErr(psb.Session, backup.SenderTemplate, psb.SenderTSpec, problem)})
			relayedErr, errDst, ok := pt.lastByType(MsgTypePathErr)
			require.True(t, ok)
			assert.Equal(t, ingress, errDst)
			assert.Equal(t, psb.SenderTemplate, relayedErr.SenderTemplate)
			assert.Equal(t, problem, relayedErr.ErrorSpec)

			before := pt.countByType(MsgTypePath)
			refreshPaths(plr.log, plr.table, plr)
			assert.Equal(t, before+1, pt.countByType(MsgTypePath), "PLR refreshes protected PATH through bypass")
			refresh, dst, ok := pt.lastByType(MsgTypePath)
			require.True(t, ok)
			assert.Equal(t, plrAddr, refresh.SenderTemplate.SenderAddr)
			assert.Equal(t, mpAddr, dst)

			// The failed original branch expires, but the merged branch keeps
			// the label and downstream state alive.
			mergedState, ok := mp.table.Get(protectedKey())
			require.True(t, ok)
			now := time.Now()
			mergedState.mu.Lock()
			original := mergedState.MergedPaths[psb.SenderTemplate]
			original.LastRefresh = now.Add(-10 * DefaultRefreshPeriod)
			mergedState.MergedPaths[psb.SenderTemplate] = original
			mergedState.mu.Unlock()
			assert.Empty(t, mp.table.expiredPSBs(now, 3))

			plr.handlePacket(Packet{Src: ingress, Payload: buildPathTear(psb, ingress)})
			tear, dst, ok := pt.lastByType(MsgTypePathTear)
			require.True(t, ok)
			assert.Equal(t, mpAddr, dst)
			assert.Equal(t, plrAddr, tear.SenderTemplate.SenderAddr)
			raw, ok = pt.lastSentPayload(MsgTypePathTear)
			require.True(t, ok)
			mp.handlePacket(Packet{Src: plrAddr, Payload: raw})
			_, alive := mp.table.Get(protectedKey())
			assert.False(t, alive, "last merged branch teardown releases the protected state")
		})
	}
}

// RFC requirement: RFC4090-6.4-1 negative -- without a usable bypass no backup-sender PATH is emitted.
// RFC requirement: RFC4090-6.4-2 negative -- an unprotected failure emits no fabricated backup ERO.
// RFC requirement: RFC4090-6.4-3 negative -- a PATH before repair retains the normal ERO including the protected hop.
func TestRFC4090NoBackupPathBeforeRepair(t *testing.T) {
	e, ft := rfc4090NodePLR(t)
	ingress := netip.MustParseAddr("10.0.0.1")
	psb := nodeProtectionPSB()
	e.handlePacket(Packet{Src: ingress, Payload: buildPath(psb, ingress, 64)})
	normal, _, ok := ft.lastByType(MsgTypePath)
	require.True(t, ok)
	assert.Equal(t, psb.SenderTemplate, normal.SenderTemplate)
	assert.Equal(t, psb.ERO[1:], normal.ERO)
	before := ft.countByType(MsgTypePath)
	e.handleLinkDown("eth0")
	assert.Equal(t, before, ft.countByType(MsgTypePath), "no established bypass: no backup PATH")
}

// RFC requirement: RFC4090-7.1-1 negative -- different remaining EROs are not merged and retain their own sender identities.
// RFC requirement: RFC4090-7.1-2 negative -- an unmerged transit PATH is not refreshed locally after its sender stops.
func TestRFC4090DifferentPathsDoNotMerge(t *testing.T) {
	e, ft, _ := testEngine(t, "10.0.0.3", nil)
	psb := protectedTransitPSB(&protectionRequest{Facility: true})
	psb.ERO = psb.ERO[1:]
	e.handlePacket(Packet{Src: netip.MustParseAddr("10.0.0.2"), Payload: buildPath(psb, netip.MustParseAddr("10.0.0.2"), 64)})
	backup := *psb
	backup.Protection = nil
	backup.SenderTemplate.SenderAddr = netip.MustParseAddr("10.0.0.2")
	backup.ERO = []eroHop{{Address: netip.MustParsePrefix("10.0.0.3/32")}, {Address: netip.MustParsePrefix("10.0.0.8/32")}, {Address: netip.MustParsePrefix("10.0.0.9/32")}}
	e.handlePacket(Packet{Src: backup.SenderTemplate.SenderAddr, Payload: buildPath(&backup, backup.SenderTemplate.SenderAddr, 64)})
	forwarded, _, ok := ft.lastByType(MsgTypePath)
	require.True(t, ok)
	assert.Equal(t, backup.SenderTemplate, forwarded.SenderTemplate)
	before := ft.countByType(MsgTypePath)
	refreshPaths(e.log, e.table, e)
	assert.Equal(t, before, ft.countByType(MsgTypePath), "unmerged transit has no local PATH refresh")
}

func headEndPLR(t *testing.T, alternate bool) (*engine, *fakeTransport, *fakeFIB, lspKey, *pathStateBlock) {
	t.Helper()
	e, ft, fib := plrEngine(t)
	cfg := e.cfg()
	cfg.Interfaces[0].Prefix = netip.MustParsePrefix("10.0.0.2/24")
	if alternate {
		cfg.Interfaces[1].Prefix = netip.MustParsePrefix("10.0.1.2/24")
		ft.localAddresses = append(ft.localAddresses, netip.MustParseAddr("10.0.1.2"))
	} else {
		cfg.Interfaces[1].Prefix = netip.Prefix{}
	}
	e.setConfig(cfg)
	psb := protectedTransitPSB(&protectionRequest{Facility: true})
	psb.SenderTemplate.SenderAddr = cfg.RouterID
	psb.RefreshPeriod = DefaultRefreshPeriod
	psb.LastRefresh = time.Now()
	key := keyFromMessage(mustDecode(t, buildPath(psb, cfg.RouterID, 64)))
	lsp, _ := e.table.GetOrCreate(key)
	lsp.mu.Lock()
	lsp.Role = RoleIngress
	lsp.PSB = psb
	lsp.mu.Unlock()
	require.NoError(t, e.sendPath(lsp))
	rsb := &resvStateBlock{Session: psb.Session, Label: labelObject{Label: 18000}, Style: StyleSharedExplicit}
	mp := cfg.Bypasses[0].MergePoint
	e.handlePacket(Packet{Src: mp, Payload: buildResv(rsb, psb.SenderTemplate, DefaultRefreshPeriod, mp)})
	bringBypassUp(t, e, 5000, netip.MustParseAddr("10.0.1.3"))
	return e, ft, fib, key, psb
}

// RFC requirement: RFC4090-6.1-1 positive -- head-end local repair uses a distinct assigned local sender while retaining its own RSVP_HOP.
func TestRFC4090HeadEndUsesDistinctSender(t *testing.T) {
	e, ft, fib, key, psb := headEndPLR(t, true)
	e.handleLinkDown("eth0")
	alternate := netip.MustParseAddr("10.0.1.2")
	var backup *ParsedMessage
	ft.mu.Lock()
	for _, sent := range ft.sent {
		msg, err := DecodeMessage(sent.payload)
		if err == nil && msg.Header.MsgType == MsgTypePath && msg.SenderTemplate.SenderAddr == alternate {
			backup = msg
		}
	}
	ft.mu.Unlock()
	require.NotNil(t, backup, "repair sends the backup PATH before starting re-optimization")
	assert.Equal(t, e.cfg().RouterID, backup.Hop.NextHop)
	assert.Equal(t, psb.Session, backup.Session)
	assert.Equal(t, []uint32{5000, 18000}, fib.pushLabels[len(fib.pushLabels)-1])
	mp := e.cfg().Bypasses[0].MergePoint
	rsb := &resvStateBlock{Session: psb.Session, Label: labelObject{Label: 21000}, Style: StyleSharedExplicit}
	e.handlePacket(Packet{Src: mp, Payload: buildResv(rsb, backup.SenderTemplate, DefaultRefreshPeriod, mp)})
	assert.Equal(t, []uint32{5000, 21000}, fib.pushLabels[len(fib.pushLabels)-1], "backup FILTER_SPEC resolves back to the head-end")

	// Completing make-before-break must retire the backup sender, not remove
	// the new ingress FEC's forwarding entry.
	replacement := psb.SenderTemplate
	replacement.LSPID++
	rsb.Label.Label = 22000
	e.handlePacket(Packet{Src: mp, Payload: buildResv(rsb, replacement, DefaultRefreshPeriod, mp)})
	_, alive := e.table.Get(key)
	assert.False(t, alive)
	assert.Equal(t, []uint32{22000}, fib.pushLabels[len(fib.pushLabels)-1])
	assert.Empty(t, fib.removed, "old teardown cannot remove the replacement's FEC")
	tear, dst, ok := ft.lastByType(MsgTypePathTear)
	require.True(t, ok)
	assert.Equal(t, alternate, tear.SenderTemplate.SenderAddr)
	assert.Equal(t, mp, dst)
}

// RFC requirement: RFC4090-6.1-1 negative -- a head-end without a distinct local sender does not signal a colliding backup identity.
func TestRFC4090HeadEndRequiresAlternateSender(t *testing.T) {
	e, ft, fib, _, _ := headEndPLR(t, false)
	before := ft.countByType(MsgTypePath)
	e.handleLinkDown("eth0")
	assert.Equal(t, before, ft.countByType(MsgTypePath))
	for _, stack := range fib.pushLabels {
		assert.Len(t, stack, 1, "no repair stack is installed without a usable backup identity")
	}
}
