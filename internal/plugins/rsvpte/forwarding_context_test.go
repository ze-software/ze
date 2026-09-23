// Design: docs/architecture/rsvpte/mpls-rsvp-te-fast-reroute.md -- selected bypass and reply identity
package rsvpte

import (
	"net/netip"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/linkstateevents"
)

func signalBypass(t *testing.T, e *engine, index int, label uint32, hop netip.Addr) lspKey {
	t.Helper()
	cfg := e.cfg()
	setupBypass(e.log, e.table, cfg.Bypasses[index], cfg, e)
	key := bypassKey(cfg.Bypasses[index], cfg.RouterID)
	lsp, ok := e.table.Get(key)
	require.True(t, ok)
	rsb := &resvStateBlock{Session: lsp.PSB.Session, Label: labelObject{Label: label}, Style: StyleSharedExplicit}
	e.handlePacket(Packet{Src: hop, Payload: buildResv(rsb, lsp.PSB.SenderTemplate, DefaultRefreshPeriod, hop)})
	require.Equal(t, LSPStateUp, lsp.State)
	return key
}

func lastPathCarriage(t *testing.T, ft *fakeTransport, kind uint8, tunnel uint16) sentMsg {
	t.Helper()
	ft.mu.Lock()
	defer ft.mu.Unlock()
	for _, sent := range slices.Backward(ft.sent) {
		msg, err := DecodeMessage(sent.payload)
		if err == nil && msg.Header.MsgType == kind && msg.Session.TunnelID == tunnel {
			return sent
		}
	}
	t.Fatalf("no message type %d for tunnel %d", kind, tunnel)
	return sentMsg{}
}

func twoBypassPLR(t *testing.T) (*engine, *fakeTransport, *fakeFIB, lspKey, lspKey) {
	t.Helper()
	e, ft, fib := plrEngine(t)
	cfg := e.cfg()
	second := cfg.Bypasses[0]
	second.Name = "other-bypass"
	second.ERO = []eroHop{{Address: netip.MustParsePrefix("10.0.2.3/32")}}
	cfg.Bypasses = append(cfg.Bypasses, second)
	e.setConfig(cfg)
	first := signalBypass(t, e, 0, 5000, netip.MustParseAddr("10.0.1.3"))
	other := signalBypass(t, e, 1, 6000, netip.MustParseAddr("10.0.2.3"))
	armAndUpProtected(t, e)
	e.handleLinkDown("eth0")
	return e, ft, fib, first, other
}

func TestSelectedBypassCarriesAllProtectedControl(t *testing.T) {
	e, ft, fib, selected, other := twoBypassPLR(t)
	labels := make(map[uint32]uint32)
	for i, table := range fib.pushTables {
		labels[table] = fib.pushLabels[i][0]
	}
	require.Len(t, labels, 2, "same merge point must retain two forwarding contexts")
	require.Equal(t, []uint32{5000, 18000}, fib.backups[len(fib.backups)-1].out)
	check := func(kind uint8) {
		t.Helper()
		sent := lastPathCarriage(t, ft, kind, protectedKey().TunnelID)
		assert.Equal(t, uint32(5000), labels[sent.route.TableID], "control must take the bypass carrying repaired data")
		assert.Equal(t, protectedKey().TunnelEndpoint, sent.route.Destination)
		assert.Equal(t, selected.TunnelEndpoint, sent.route.NextHop)
	}
	check(MsgTypePath)
	psb := protectedTransitPSB(&protectionRequest{Facility: true})
	confirm := &ParsedMessage{Session: psb.Session,
		ResvConfirm: psb.Session.TunnelEndpoint, HasResvConfirm: true, Style: StyleSharedExplicit}
	descriptor := flowDescriptor{Filters: []reservationFilter{{Filter: psb.SenderTemplate}}}
	e.handlePacket(Packet{Src: psb.SenderTemplate.SenderAddr, Payload: buildResvConf(confirm, &descriptor, psb.SenderTemplate.SenderAddr)})
	check(MsgTypeResvConf)
	forwarded := mustDecode(t, lastPathCarriage(t, ft, MsgTypeResvConf, psb.Session.TunnelID).payload)
	require.Len(t, forwarded.FlowDescriptors, 1)
	require.Len(t, forwarded.FlowDescriptors[0].Filters, 1)
	assert.Equal(t, e.cfg().RouterID, forwarded.FlowDescriptors[0].Filters[0].Filter.SenderAddr)
	e.handlePacket(Packet{Src: psb.SenderTemplate.SenderAddr, Payload: buildPathTear(psb, psb.SenderTemplate.SenderAddr)})
	check(MsgTypePathTear)

	e.teardownLSP(other)
	assert.Equal(t, []uint32{bypassTableID(other)}, fib.removeTables)
	assert.True(t, e.bypassEstablished(&selected), "removing the other same-MP bypass must not remove this one")
	e.shutdown()
	assert.Equal(t, []uint32{bypassTableID(other), bypassTableID(selected)}, fib.removeTables)
	assert.Zero(t, e.table.Len(), "shutdown withdraws all signaling state")
}

// TestBypassRefreshContinuesDuringRepair keeps both bypass sessions alive while
// the protected PATH refresh uses only its selected forwarding context.
func TestBypassRefreshContinuesDuringRepair(t *testing.T) {
	e, ft, _, selected, other := twoBypassPLR(t)
	before := sentCount(ft)
	refreshPaths(e.log, e.table, e)

	ft.mu.Lock()
	sent := slices.Clone(ft.sent[before:])
	ft.mu.Unlock()
	paths := make(map[lspKey]sentMsg)
	for _, message := range sent {
		decoded := mustDecode(t, message.payload)
		if decoded.Header.MsgType != MsgTypePath {
			continue
		}
		key := keyFromMessage(decoded)
		_, duplicate := paths[key]
		require.False(t, duplicate, "one PATH refresh per sender")
		paths[key] = message
	}
	require.Len(t, paths, 3, "both bypasses and the repaired sender refresh")
	for index, key := range []lspKey{selected, other} {
		message, present := paths[key]
		require.True(t, present, "bypass %d keeps its own PATH session alive", index)
		path := mustDecode(t, message.payload)
		assert.Equal(t, e.cfg().Bypasses[index].ERO, path.ERO)
		assert.Equal(t, key.TunnelEndpoint, message.route.Destination)
		assert.Equal(t, path.ERO[0].Address.Addr(), message.route.NextHop)
		assert.Zero(t, message.route.TableID, "bypass refresh follows its ordinary ERO")
	}
	backupKey := protectedKey()
	backupKey.SenderAddr = e.cfg().RouterID
	message, present := paths[backupKey]
	require.True(t, present, "protected PATH refresh retains the backup sender")
	assert.Equal(t, bypassTableID(selected), message.route.TableID)
	assert.Equal(t, selected.TunnelEndpoint, message.route.NextHop)
	assert.Equal(t, backupKey.TunnelEndpoint, message.route.Destination)
}

func TestBypassWithdrawalRetiresDependentRepair(t *testing.T) {
	e, ft, fib, selected, other := twoBypassPLR(t)
	protected, ok := e.table.Get(protectedKey())
	require.True(t, ok)
	inLabel := protected.InLabel
	e.teardownLSP(selected)
	assert.NotEqual(t, LSPStateUp, protected.State)
	assert.Nil(t, protected.RSB, "periodic refresh cannot advertise the retired repair")
	assert.Contains(t, fib.removedSwap, inLabel)
	assert.Equal(t, []uint32{bypassTableID(selected)}, fib.removeTables)
	assert.True(t, e.bypassEstablished(&other))
	tear := lastPathCarriage(t, ft, MsgTypePathTear, protected.Key.TunnelID)
	assert.Equal(t, bypassTableID(selected), tear.route.TableID)
	assert.Equal(t, e.cfg().RouterID, mustDecode(t, tear.payload).SenderTemplate.SenderAddr)
	before := ft.countByType(MsgTypeResv)
	refreshPaths(e.log, e.table, e)
	assert.Equal(t, before, ft.countByType(MsgTypeResv))
}

func TestBackupReplyAcceptsOnlyKnownMergePointAddresses(t *testing.T) {
	e, ft, fib, selected, _ := twoBypassPLR(t)
	alias := netip.MustParseAddr("10.2.0.3")
	unrelated := netip.MustParseAddr("203.0.113.8")
	psb := protectedTransitPSB(&protectionRequest{Facility: true})
	filter := psb.SenderTemplate
	filter.SenderAddr = e.cfg().RouterID
	reply := func(source netip.Addr, label uint32) {
		rsb := &resvStateBlock{Session: psb.Session, Label: labelObject{Label: label}, Style: StyleSharedExplicit}
		e.handlePacket(Packet{Src: source, Payload: buildResv(rsb, filter, DefaultRefreshPeriod, selected.TunnelEndpoint)})
	}
	id := selected.TunnelEndpoint.As4()
	node := linkstateevents.NodeID{RouterID: id[:]}
	domain := linkstateevents.Domain{Protocol: linkstateevents.OSPFv2, Instance: 3, Identifier: 3}
	e.updatePeerIdentity("ospf", &linkstateevents.Snapshot{Domain: domain, Generation: 1,
		Nodes:    []linkstateevents.Node{{ID: node}},
		Links:    []linkstateevents.Link{{Local: node, LocalAddresses: []netip.Addr{alias}}},
		Prefixes: []linkstateevents.Prefix{{Node: node, Prefix: netip.PrefixFrom(unrelated, 32)}}})
	reply(unrelated, 20000)
	assert.Equal(t, []uint32{5000, 18000}, fib.backups[len(fib.backups)-1].out, "a node's advertised route is not its own address")
	reply(alias, 21000)
	assert.Equal(t, []uint32{5000, 21000}, fib.backups[len(fib.backups)-1].out)
	errRaw := buildPathErr(psb.Session, filter, psb.SenderTSpec,
		errorSpec{ErrorNode: selected.TunnelEndpoint, ErrorCode: ErrCodeRoutingProblem}, selected.TunnelEndpoint)
	e.handlePacket(Packet{Src: alias, Payload: errRaw})
	pathErr, dst, ok := ft.lastByType(MsgTypePathErr)
	require.True(t, ok)
	assert.Equal(t, psb.SenderTemplate, pathErr.SenderTemplate)
	assert.Equal(t, psb.SenderTemplate.SenderAddr, dst)
	assert.Equal(t, ErrCodeRoutingProblem, pathErr.ErrorSpec.ErrorCode)
	assert.Equal(t, selected.TunnelEndpoint, pathErr.ErrorSpec.ErrorNode)
	e.updatePeerIdentity("ospf", &linkstateevents.Snapshot{Domain: domain, Generation: 2})
	reply(alias, 22000)
	assert.Equal(t, []uint32{5000, 21000}, fib.backups[len(fib.backups)-1].out, "withdrawn address association must not remain authorized")
	reply(selected.TunnelEndpoint, 23000)
	assert.Equal(t, []uint32{5000, 23000}, fib.backups[len(fib.backups)-1].out)
}
