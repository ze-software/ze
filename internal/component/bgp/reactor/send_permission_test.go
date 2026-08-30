package reactor

import (
	"bufio"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
)

// newAttachedPeer builds one Established peer, backed by a recordingConn, whose
// config carries the given attach blocks. The conn is what makes the assertions
// about REACHING a peer rather than about an error code: a peer the permission
// dropped has written nothing.
func newAttachedPeer(t *testing.T, addr string, bindings ...ProcessBinding) (*Peer, *recordingConn) {
	t.Helper()
	settings := &PeerSettings{
		Connection:      ConnectionBoth,
		Address:         netip.MustParseAddr(addr),
		LocalAS:         65000,
		PeerAS:          65001,
		RouterID:        0x01020301,
		ProcessBindings: bindings,
	}
	peer := NewPeer(settings)
	peer.state.Store(int32(PeerStateEstablished))
	peer.negotiated.Store(&NegotiatedCapabilities{
		families:     map[family.Family]bool{{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast}: true},
		RouteRefresh: true,
	})

	session := NewSession(settings)
	require.NoError(t, session.fsm.Event(fsm.EventManualStart))
	require.NoError(t, session.fsm.Event(fsm.EventTCPConnectionConfirmed))
	require.NoError(t, session.fsm.Event(fsm.EventBGPOpen))
	require.NoError(t, session.fsm.Event(fsm.EventKeepaliveMsg))

	conn := &recordingConn{}
	session.mu.Lock()
	session.conn = conn
	session.bufWriter = bufio.NewWriterSize(conn, 4096)
	session.mu.Unlock()

	peer.mu.Lock()
	peer.session = session
	peer.mu.Unlock()

	return peer, conn
}

// newSendPermissionReactor assembles an adapter over the given peers.
func newSendPermissionReactor(peers ...*Peer) *reactorAPIAdapter {
	m := make(map[netip.AddrPort]*Peer, len(peers))
	for _, p := range peers {
		m[p.settings.PeerKey()] = p
	}
	return &reactorAPIAdapter{r: &Reactor{config: &Config{LocalAS: 65000}, peers: m}}
}

// sendUpdateOnly is the binding an operator writes for a program that
// originates routes toward a peer and is told nothing by it: `attach process
// <name> { send [ update ] }`, with no receive line at all. AC-11's config half.
func sendUpdateOnly(process string) ProcessBinding {
	return ProcessBinding{PluginName: process, Send: map[string]bool{bgpevents.SendUpdate: true}}
}

// sendRawOnly is the binding an operator writes for a program that builds whole
// BGP messages itself: `attach process <name> { send [ raw ] }`. It is a
// separate word from `update` because the bytes can be any message, an OPEN or a
// NOTIFICATION included (owner ruling, 2026-08-30).
func sendRawOnly(process string) ProcessBinding {
	return ProcessBinding{PluginName: process, Send: map[string]bool{bgpevents.SendRaw: true}}
}

// resetSendPermissionMetrics puts the package back to its unwired state so tests
// do not leak a registry into each other.
func resetSendPermissionMetrics(t *testing.T) {
	t.Helper()
	t.Cleanup(func() { sendPermissionMetricsPtr.Store(nil) })
	sendPermissionMetricsPtr.Store(nil)
}

func testBatchOnePrefix(t *testing.T) bgptypes.NLRIBatch {
	t.Helper()
	return bgptypes.NLRIBatch{
		Family: family.IPv4Unicast,
		NLRIs:  []nlri.NLRI{nlri.NewINET(family.IPv4Unicast, netip.MustParsePrefix("192.0.2.0/24"), 0)},
	}
}

// TestSendPermissionRefusesUnattachedPeer is AC-9 and AC-10 at the entry point.
//
// AC-9: a process attached to peer A with `send [ update ]` issues a command
// whose selector names every peer. It reaches A and NOT peer B, which never
// attached it, and the refusal for B is REPORTED rather than silent.
//
// AC-10: the same process aimed at B alone is REFUSED, and the refusal names
// the peer and the process.
//
// AC-11's independence is in the same fixture: A's binding carries a send list
// and NO receive line, and it may still announce. The two directions are
// separate map fields with separate readers, and a binding that grants one
// grants nothing about the other.
//
// VALIDATES: the permission is applied where the selector is RESOLVED, so every
// command that resolves one inherits it, and a partial match serves the peers it
// may reach instead of failing the whole command.
// PREVENTS: the hole this phase exists to close -- `peer *` from a process
// reaching a peer that never attached it, which is route injection rather than
// an information leak.
func TestSendPermissionRefusesUnattachedPeer(t *testing.T) {
	resetSendPermissionMetrics(t)
	reg := &announceFakeRegistry{}
	setSendPermissionMetricsRegistry(reg)
	require.Contains(t, reg.names, "ze_bgp_send_refused_total",
		"the refusal counter must register under its ze_-prefixed name")

	attached, attachedConn := newAttachedPeer(t, "10.0.0.2", sendUpdateOnly("injector"))
	unattached, unattachedConn := newAttachedPeer(t, "10.0.0.3")
	adapter := newSendPermissionReactor(attached, unattached)

	// AC-9. One command, every peer named, two different answers.
	err := adapter.AnnounceEOR(selector.All(), uint16(family.AFIIPv4), uint8(family.SAFIUnicast), plugin.ProcessSender("injector"))
	require.NoError(t, err, "a selector that reaches one permitted peer is not an error")

	require.Equal(t, eorWire(family.IPv4Unicast), attachedConn.written(),
		"the peer that attaches the process with send [ update ] must be served")
	require.Empty(t, unattachedConn.written(),
		"a peer that never attached the process must receive nothing from it")

	// AC-9's second half: the refusal is REPORTED. A drop that only happens is
	// indistinguishable from a routing bug, so it is counted as well as logged.
	require.NotNil(t, reg.vec, "the refusal must reach the counter")
	assert.Equal(t, 1, reg.vec.counters["injector|update"].n,
		"one refused peer must move the process/type series exactly once")

	// AC-10. Aimed at the unattached peer alone, the whole command is refused,
	// and the message names both halves so the process can act on it.
	err = adapter.AnnounceEOR(selector.Addr(netip.MustParseAddr("10.0.0.3")),
		uint16(family.AFIIPv4), uint8(family.SAFIUnicast), plugin.ProcessSender("injector"))
	require.ErrorIs(t, err, errSendNotPermitted)
	assert.Contains(t, err.Error(), "injector", "the refusal must name the process")
	assert.Contains(t, err.Error(), "10.0.0.3", "the refusal must name the peer")
	require.Empty(t, unattachedConn.written(),
		"a refused command must put nothing on that peer's wire")

	// The announce rail refuses before it builds anything, so the same guard
	// covers the command an operator's program actually issues.
	err = adapter.AnnounceNLRIBatch(selector.Addr(netip.MustParseAddr("10.0.0.3")), testBatchOnePrefix(t), plugin.ProcessSender("injector"))
	require.ErrorIs(t, err, errSendNotPermitted)
}

// TestSendPermissionSeparatesUpdateFromRefresh proves the two send types are two
// permissions and not one.
//
// A ROUTE-REFRESH asks a peer to re-advertise; an UPDATE originates a route.
// Granting one must not grant the other, or `send [ refresh ]` would be a route-
// injection permission spelled with a safe-looking word.
//
// VALIDATES: the six selector-resolving commands are gated on the message type
// each one actually puts on the wire.
// PREVENTS: gating all six on `update`, which is the easy reading and which
// would make `send [ refresh ]` alone unable to send a refresh.
func TestSendPermissionSeparatesUpdateFromRefresh(t *testing.T) {
	resetSendPermissionMetrics(t)

	refreshOnly, conn := newAttachedPeer(t, "10.0.0.2", ProcessBinding{
		PluginName: "poller",
		Send:       map[string]bool{bgpevents.SendRefresh: true},
	})
	adapter := newSendPermissionReactor(refreshOnly)
	all := selector.All()

	require.NoError(t, adapter.SendRefresh(all, uint16(family.AFIIPv4), uint8(family.SAFIUnicast), plugin.ProcessSender("poller")),
		"send [ refresh ] must permit a ROUTE-REFRESH")
	require.NotEmpty(t, conn.written(), "the ROUTE-REFRESH must reach the wire")

	families, err := adapter.SoftClearPeer(all, plugin.ProcessSender("poller"))
	require.NoError(t, err, "a soft clear is ROUTE-REFRESH, so send [ refresh ] permits it")
	require.NotEmpty(t, families)

	require.ErrorIs(t,
		adapter.AnnounceEOR(all, uint16(family.AFIIPv4), uint8(family.SAFIUnicast), plugin.ProcessSender("poller")),
		errSendNotPermitted,
		"send [ refresh ] must NOT permit an UPDATE")
	require.ErrorIs(t,
		adapter.AnnounceNLRIBatch(all, testBatchOnePrefix(t), plugin.ProcessSender("poller")),
		errSendNotPermitted,
		"send [ refresh ] must NOT permit an announce")

	// And the converse, so neither type is simply always allowed.
	updateOnly, _ := newAttachedPeer(t, "10.0.0.4", sendUpdateOnly("injector"))
	updateAdapter := newSendPermissionReactor(updateOnly)
	require.ErrorIs(t,
		updateAdapter.SendRefresh(all, uint16(family.AFIIPv4), uint8(family.SAFIUnicast), plugin.ProcessSender("injector")),
		errSendNotPermitted,
		"send [ update ] must NOT permit a ROUTE-REFRESH")
}

// TestSendPermissionGrantsTheWildcardAndNamesTheRightProcess pins the two ways a
// binding can say yes, and the one way it says no by naming somebody else.
//
// VALIDATES: `send [ * ]` grants every type through MaySend, and a binding is
// matched by process NAME, so one peer's two attach blocks do not lend each
// other permissions.
// PREVENTS: a peer that attaches ANY process being read as attaching this one,
// which is the shape a `len(bindings) > 0` check would have.
func TestSendPermissionGrantsTheWildcardAndNamesTheRightProcess(t *testing.T) {
	resetSendPermissionMetrics(t)

	peer, conn := newAttachedPeer(t, "10.0.0.2",
		ProcessBinding{PluginName: "bridge", SendAll: true},
		ProcessBinding{PluginName: "observer"}, // attached, granted nothing
	)
	adapter := newSendPermissionReactor(peer)
	all := selector.All()

	require.NoError(t, adapter.AnnounceEOR(all, uint16(family.AFIIPv4), uint8(family.SAFIUnicast), plugin.ProcessSender("bridge")),
		"send [ * ] must grant update")
	require.NotEmpty(t, conn.written())
	require.NoError(t, adapter.SendRefresh(all, uint16(family.AFIIPv4), uint8(family.SAFIUnicast), plugin.ProcessSender("bridge")),
		"send [ * ] must grant refresh too")

	require.ErrorIs(t,
		adapter.AnnounceNLRIBatch(all, testBatchOnePrefix(t), plugin.ProcessSender("observer")),
		errSendNotPermitted,
		"a process attached with no send list must be refused, even beside a wildcard sibling")
	require.ErrorIs(t,
		adapter.AnnounceNLRIBatch(all, testBatchOnePrefix(t), plugin.ProcessSender("stranger")),
		errSendNotPermitted,
		"a process the peer does not name at all must be refused")
}

// TestSendPermissionDoesNotGateAnOperator is the other half of the guard, and
// the reason it is not simply "refuse unless a binding grants it".
//
// An operator at the CLI, SSH or REST surface says so with plugin.OperatorSender,
// and `send` grants authority to a program rather than to a person: their
// authority was already checked by AAA. Every operator surface sets
// CommandContext.Sender to that value (cmd/ze/hub/main_servers.go).
//
// VALIDATES: `ze bgp peer <addr> refresh` keeps working on a peer that attaches
// no process, which is most peers.
// PREVENTS: a guard that fails closed on the wrong population, taking every
// operator command with it.
func TestSendPermissionDoesNotGateAnOperator(t *testing.T) {
	resetSendPermissionMetrics(t)
	reg := &announceFakeRegistry{}
	setSendPermissionMetricsRegistry(reg)

	bare, conn := newAttachedPeer(t, "10.0.0.2")
	adapter := newSendPermissionReactor(bare)
	all := selector.All()

	require.NoError(t, adapter.AnnounceEOR(all, uint16(family.AFIIPv4), uint8(family.SAFIUnicast), plugin.OperatorSender()))
	require.Equal(t, eorWire(family.IPv4Unicast), conn.written(),
		"an operator command must reach a peer that attaches nothing")
	require.NoError(t, adapter.SendRefresh(all, uint16(family.AFIIPv4), uint8(family.SAFIUnicast), plugin.OperatorSender()))

	// The vec exists from registration; what must stay empty is its series. A
	// counter that moved here would mean an operator command had been refused.
	require.NotNil(t, reg.vec)
	assert.Empty(t, reg.vec.counters, "an operator command must record no refusal")
}

// TestSendPermissionRefusesACommandWithNoSender is the third state, and the one
// the operator exemption above must not swallow.
//
// A command that reaches the resolver with the zero plugin.Sender was built by a
// dispatch path that never said who issued it. There is no attach block to
// consult, and the operator's exemption belongs to the operator, so the command
// is refused whole and reported. The peer here attaches NOTHING, which is the
// fixture the operator test uses to prove a command gets through: same peers,
// same commands, opposite answer, so the difference measured is the sender and
// nothing else.
//
// VALIDATES: the guard fails closed on an unset identity at all four entry
// points that resolve a selector, and says so through the log and the counter.
// PREVENTS: a new dispatch path that forgets to set CommandContext.Sender
// inheriting the operator's authority by omission, on the one guard that stops
// a process reaching a peer that never attached it (ai/rules/evidence.md).
func TestSendPermissionRefusesACommandWithNoSender(t *testing.T) {
	resetSendPermissionMetrics(t)
	reg := &announceFakeRegistry{}
	setSendPermissionMetricsRegistry(reg)

	bare, conn := newAttachedPeer(t, "10.0.0.2")
	attached, attachedConn := newAttachedPeer(t, "10.0.0.5", sendUpdateOnly("injector"))
	adapter := newSendPermissionReactor(bare, attached)
	all := selector.All()

	var nobody plugin.Sender // the zero value: nobody said who is sending

	err := adapter.AnnounceEOR(all, uint16(family.AFIIPv4), uint8(family.SAFIUnicast), nobody)
	require.ErrorIs(t, err, errSendNoSender, "an End-of-RIB from nobody must be refused")
	assert.Contains(t, err.Error(), "CommandContext.Sender",
		"the refusal must name the field the dispatch path failed to set")

	require.ErrorIs(t, adapter.AnnounceNLRIBatch(all, testBatchOnePrefix(t), nobody), errSendNoSender,
		"an announce from nobody must be refused")
	require.ErrorIs(t, adapter.SendRefresh(all, uint16(family.AFIIPv4), uint8(family.SAFIUnicast), nobody), errSendNoSender,
		"a ROUTE-REFRESH from nobody must be refused")
	_, err = adapter.SoftClearPeer(all, nobody)
	require.ErrorIs(t, err, errSendNoSender, "a soft clear from nobody must be refused")

	require.Empty(t, conn.written(),
		"a peer that attaches nothing must receive nothing from an unnamed sender")
	require.Empty(t, attachedConn.written(),
		"a peer that attaches a process must receive nothing from an unnamed sender either")

	// Reported, not only refused. The series is keyed on the sender's name, and
	// Sender.String gives an unset sender the bounded value "unset".
	require.NotNil(t, reg.vec, "the refusal must reach the counter")
	assert.Equal(t, 2, reg.vec.counters["unset|update"].n,
		"the two UPDATE commands must move the unset series once each")
	assert.Equal(t, 2, reg.vec.counters["unset|refresh"].n,
		"the two ROUTE-REFRESH commands must move the unset series once each")
	assert.NotContains(t, reg.vec.counters, "|update",
		"an unset sender must never be recorded under an empty process name")
}

// TestRecordSendRefusedIsSilentWithoutARegistry verifies the recorder is safe
// before any registry is wired, which is the default build.
//
// VALIDATES: a daemon with metrics disabled still refuses and still logs.
// PREVENTS: a nil-pointer panic on the refusal path in every such build.
func TestRecordSendRefusedIsSilentWithoutARegistry(t *testing.T) {
	resetSendPermissionMetrics(t)

	assert.NotPanics(t, func() {
		recordSendRefused("injector", bgpevents.SendUpdate)
	}, "the recorder must be a no-op until a registry is wired")

	setSendPermissionMetricsRegistry(nil)
	assert.NotPanics(t, func() {
		recordSendRefused("injector", bgpevents.SendRefresh)
	}, "a nil registry must leave the recorder disabled, not half-wired")
}
