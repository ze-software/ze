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
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/internal/core/selector"
)

const (
	testBusMsgUpdate  = "bgp:update"
	testBusMsgRefresh = "bgp:refresh"
	testBusMsgRaw     = "bgp:raw"
	testUnsetSender   = "unset"
)

// newAttachedPeer builds one Established peer, backed by a recordingConn, whose
// config carries the given bus endpoint grants. The conn is what makes the
// assertions about REACHING a peer rather than about an error code: a peer the
// permission dropped has written nothing.
func newAttachedPeer(t *testing.T, addr string, grants ...testPeerEndpointGrant) (*Peer, *recordingConn) {
	t.Helper()
	settings := &PeerSettings{
		Connection: ConnectionBoth,
		Address:    netip.MustParseAddr(addr),
		LocalAS:    65000,
		PeerAS:     65001,
		RouterID:   0x01020301,
		Bus:        testBusBlockForGrants(grants...),
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

// newSendPermissionReactor assembles an adapter over the given peers and
// publishes their compiled DeliveryGraph.
func newSendPermissionReactor(t *testing.T, peers ...*Peer) *reactorAPIAdapter {
	t.Helper()
	m := make(map[netip.AddrPort]*Peer, len(peers))
	r := &Reactor{config: &Config{LocalAS: 65000}, peers: m}
	for _, p := range peers {
		m[p.settings.PeerKey()] = p
		p.reactor = r
	}
	publishTestBusGraph(t, r)
	return &reactorAPIAdapter{r: r}
}

// sendUpdateOnly grants a program permission to originate routes (`bgp:update`)
// toward a peer without receiving anything from it.
func sendUpdateOnly(process string) testPeerEndpointGrant {
	return testPeerEndpointGrant{Name: process, Send: []string{bgpevents.SendUpdate}}
}

// sendRawOnly grants a program permission to inject raw BGP messages (`bgp:raw`)
// toward a peer.
func sendRawOnly(process string) testPeerEndpointGrant {
	return testPeerEndpointGrant{Name: process, Send: []string{bgpevents.SendRaw}}
}

// resetSendPermissionReports clears the shared report bus before and after each
// test so refusal events never leak across tests.
func resetSendPermissionReports(t *testing.T) {
	t.Helper()
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
}

// wireBusRefusalRegistry attaches a fake Prometheus metrics registry to
// pluginserver's single bus refusal counter and clears it when the test ends.
func wireBusRefusalRegistry(t *testing.T) *announceFakeRegistry {
	t.Helper()
	reg := &announceFakeRegistry{}
	pluginserver.SetBusSendRefusedMetrics(reg)
	t.Cleanup(func() { pluginserver.SetBusSendRefusedMetrics(nil) })
	return reg
}

// busRefusalEvents returns all reactor bus-send refusal error events on the
// report bus. When subject is non-empty, only events matching subject are
// returned.
func busRefusalEvents(subject string) []report.Issue {
	var matched []report.Issue
	for _, ev := range report.Errors(0) {
		if ev.Source != reportSourceBGPReactor {
			continue
		}
		if ev.Code != diagnostic.CodeDoctorPluginRequestRefused {
			continue
		}
		if subject != "" {
			if ev.Subject != subject {
				continue
			}
		}
		matched = append(matched, ev)
	}
	return matched
}

func testBatchOnePrefix(t *testing.T) bgptypes.NLRIBatch {
	t.Helper()
	return bgptypes.NLRIBatch{
		Family: family.IPv4Unicast,
		NLRIs:  []nlri.NLRI{nlri.NewINET(family.IPv4Unicast, netip.MustParsePrefix("192.0.2.0/24"), 0, false)},
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
// command that resolves one inherits it, a partial match serves the peers it
// may reach instead of failing the whole command, and every refusal increments
// ze_bus_send_refused_total and raises CodeDoctorPluginRequestRefused on the
// report bus.
// PREVENTS: the hole this phase exists to close -- `peer *` from a process
// reaching a peer that never attached it, which is route injection rather than
// an information leak.
func TestSendPermissionRefusesUnattachedPeer(t *testing.T) {
	resetSendPermissionReports(t)
	reg := wireBusRefusalRegistry(t)
	require.Contains(t, reg.names, "ze_bus_send_refused_total",
		"refusal metric must register under its ze_-prefixed name")
	require.NotNil(t, reg.vec)
	injectorParticipant := plugin.EndpointPath("injector")
	require.Equal(t, "/bus/endpoint/injector", injectorParticipant)
	seriesKey := injectorParticipant + "|" + testBusMsgUpdate
	before := pluginserver.BusSendRefusedCount(injectorParticipant, testBusMsgUpdate)

	attached, attachedConn := newAttachedPeer(t, "10.0.0.2", sendUpdateOnly("injector"))
	unattached, unattachedConn := newAttachedPeer(t, "10.0.0.3")
	adapter := newSendPermissionReactor(t, attached, unattached)

	// AC-9. One command, every peer named, two different answers.
	err := adapter.AnnounceEOR(selector.All(), uint16(family.AFIIPv4), uint8(family.SAFIUnicast), plugin.ProcessSender("injector"))
	require.NoError(t, err, "a selector that reaches one permitted peer is not an error")

	require.Equal(t, eorWire(family.IPv4Unicast), attachedConn.written(),
		"the peer that attaches the process with send [ update ] must be served")
	require.Empty(t, unattachedConn.written(),
		"a peer that never attached the process must receive nothing from it")

	// AC-9's second half: the refusal is REPORTED. A drop that only happens is
	// indistinguishable from a routing bug, so it is counted in
	// ze_bus_send_refused_total and raised on the report bus as well as logged.
	assert.Equal(t, before+1, pluginserver.BusSendRefusedCount(injectorParticipant, testBusMsgUpdate),
		"one refused peer must move the participant/message series in ze_bus_send_refused_total exactly once")
	require.Contains(t, reg.vec.counters, seriesKey)
	assert.Equal(t, 1, reg.vec.counters[seriesKey].n,
		"one refused peer must increment the Prometheus CounterVec series once")
	events := busRefusalEvents(injectorParticipant)
	require.Len(t, events, 1, "one refused peer must raise CodeDoctorPluginRequestRefused on the report bus")
	assert.Equal(t, reportSourceBGPReactor, events[0].Source)
	assert.Equal(t, diagnostic.CodeDoctorPluginRequestRefused, events[0].Code)
	assert.Equal(t, injectorParticipant, events[0].Subject)
	assert.Contains(t, events[0].Message, injectorParticipant, "the refusal event must name the sending participant")
	assert.Contains(t, events[0].Message, "/bgp/ip/10.0.0.3", "the refusal event must name the peer participant")
	assert.Contains(t, events[0].Message, testBusMsgUpdate, "the refusal event must name the bus message")
	assert.Contains(t, events[0].Fix, "bus receive filter", "the fix must point to the peer bus receive filter")
	assert.Contains(t, events[0].Fix, "bus send filter", "the fix must point to the endpoint bus send filter")

	// AC-10. Aimed at the unattached peer alone, the whole command is refused,
	// and the message names both halves so the process can act on it.
	err = adapter.AnnounceEOR(selector.Addr(netip.MustParseAddr("10.0.0.3")),
		uint16(family.AFIIPv4), uint8(family.SAFIUnicast), plugin.ProcessSender("injector"))
	require.ErrorIs(t, err, errSendNotPermitted)
	assert.Contains(t, err.Error(), "injector", "the refusal must name the process")
	assert.Contains(t, err.Error(), "10.0.0.3", "the refusal must name the peer")
	require.Empty(t, unattachedConn.written(),
		"a refused command must put nothing on that peer's wire")
	assert.Equal(t, before+2, pluginserver.BusSendRefusedCount(injectorParticipant, testBusMsgUpdate),
		"refusing the targeted peer must increment ze_bus_send_refused_total")
	assert.Equal(t, 2, reg.vec.counters[seriesKey].n,
		"refusing the targeted peer must increment the Prometheus CounterVec series")
	require.Len(t, busRefusalEvents(injectorParticipant), 2,
		"refusing the targeted peer must raise CodeDoctorPluginRequestRefused")

	// The announce rail refuses before it builds anything, so the same guard
	// covers the command an operator's program actually issues.
	err = adapter.AnnounceNLRIBatch(t.Context(), selector.Addr(netip.MustParseAddr("10.0.0.3")), testBatchOnePrefix(t), plugin.ProcessSender("injector"))
	require.ErrorIs(t, err, errSendNotPermitted)
	assert.Equal(t, before+3, pluginserver.BusSendRefusedCount(injectorParticipant, testBusMsgUpdate),
		"refusing AnnounceNLRIBatch must increment ze_bus_send_refused_total")
	assert.Equal(t, 3, reg.vec.counters[seriesKey].n,
		"refusing AnnounceNLRIBatch must increment the Prometheus CounterVec series")
	require.Len(t, busRefusalEvents(injectorParticipant), 3,
		"refusing AnnounceNLRIBatch must raise CodeDoctorPluginRequestRefused")
}

// TestSendPermissionSeparatesUpdateFromRefresh proves the two send types are two
// permissions and not one.
//
// A ROUTE-REFRESH asks a peer to re-advertise; an UPDATE originates a route.
// Granting one must not grant the other, or `send [ refresh ]` would be a route-
// injection permission spelled with a safe-looking word.
//
// VALIDATES: the six selector-resolving commands are gated on the message type
// each one actually puts on the wire, and each refusal records the exact
// participant and bus message label in ze_bus_send_refused_total and on the
// report bus.
// PREVENTS: gating all six on `update`, which is the easy reading and which
// would make `send [ refresh ]` alone unable to send a refresh.
func TestSendPermissionSeparatesUpdateFromRefresh(t *testing.T) {
	resetSendPermissionReports(t)
	pollerParticipant := plugin.EndpointPath("poller")
	injectorParticipant := plugin.EndpointPath("injector")
	beforePollerUpdate := pluginserver.BusSendRefusedCount(pollerParticipant, testBusMsgUpdate)
	beforeInjectorRefresh := pluginserver.BusSendRefusedCount(injectorParticipant, testBusMsgRefresh)

	refreshOnly, conn := newAttachedPeer(t, "10.0.0.2", testPeerEndpointGrant{
		Name: "poller",
		Send: []string{bgpevents.SendRefresh},
	})
	adapter := newSendPermissionReactor(t, refreshOnly)
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
		adapter.AnnounceNLRIBatch(t.Context(), all, testBatchOnePrefix(t), plugin.ProcessSender("poller")),
		errSendNotPermitted,
		"send [ refresh ] must NOT permit an announce")
	assert.Equal(t, beforePollerUpdate+2, pluginserver.BusSendRefusedCount(pollerParticipant, testBusMsgUpdate),
		"the two refused UPDATE commands must move poller's bgp:update series twice")
	require.Len(t, busRefusalEvents(pollerParticipant), 2,
		"the two refused UPDATE commands must raise CodeDoctorPluginRequestRefused twice")

	// And the converse, so neither type is simply always allowed.
	updateOnly, _ := newAttachedPeer(t, "10.0.0.4", sendUpdateOnly("injector"))
	updateAdapter := newSendPermissionReactor(t, updateOnly)
	require.ErrorIs(t,
		updateAdapter.SendRefresh(all, uint16(family.AFIIPv4), uint8(family.SAFIUnicast), plugin.ProcessSender("injector")),
		errSendNotPermitted,
		"send [ update ] must NOT permit a ROUTE-REFRESH")
	assert.Equal(t, beforeInjectorRefresh+1, pluginserver.BusSendRefusedCount(injectorParticipant, testBusMsgRefresh),
		"the refused ROUTE-REFRESH must move injector's bgp:refresh series once")
	require.Len(t, busRefusalEvents(injectorParticipant), 1,
		"the refused ROUTE-REFRESH must raise CodeDoctorPluginRequestRefused")
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
	resetSendPermissionReports(t)
	bridgeParticipant := plugin.EndpointPath("bridge")
	observerParticipant := plugin.EndpointPath("observer")
	strangerParticipant := plugin.EndpointPath("stranger")
	beforeBridgeUpdate := pluginserver.BusSendRefusedCount(bridgeParticipant, testBusMsgUpdate)
	beforeBridgeRefresh := pluginserver.BusSendRefusedCount(bridgeParticipant, testBusMsgRefresh)
	beforeObserverUpdate := pluginserver.BusSendRefusedCount(observerParticipant, testBusMsgUpdate)
	beforeStrangerUpdate := pluginserver.BusSendRefusedCount(strangerParticipant, testBusMsgUpdate)

	peer, conn := newAttachedPeer(t, "10.0.0.2",
		testPeerEndpointGrant{Name: "bridge", SendAll: true},
		testPeerEndpointGrant{Name: "observer"}, // attached, granted nothing
	)
	adapter := newSendPermissionReactor(t, peer)
	all := selector.All()

	require.NoError(t, adapter.AnnounceEOR(all, uint16(family.AFIIPv4), uint8(family.SAFIUnicast), plugin.ProcessSender("bridge")),
		"send [ * ] must grant update")
	require.NotEmpty(t, conn.written())
	require.NoError(t, adapter.SendRefresh(all, uint16(family.AFIIPv4), uint8(family.SAFIUnicast), plugin.ProcessSender("bridge")),
		"send [ * ] must grant refresh too")
	assert.Equal(t, beforeBridgeUpdate, pluginserver.BusSendRefusedCount(bridgeParticipant, testBusMsgUpdate),
		"permitted wildcard sends must record no update refusal")
	assert.Equal(t, beforeBridgeRefresh, pluginserver.BusSendRefusedCount(bridgeParticipant, testBusMsgRefresh),
		"permitted wildcard sends must record no refresh refusal")
	assert.Empty(t, busRefusalEvents(bridgeParticipant),
		"permitted wildcard sends must raise no refusal event")

	require.ErrorIs(t,
		adapter.AnnounceNLRIBatch(t.Context(), all, testBatchOnePrefix(t), plugin.ProcessSender("observer")),
		errSendNotPermitted,
		"a process attached with no send list must be refused, even beside a wildcard sibling")
	require.ErrorIs(t,
		adapter.AnnounceNLRIBatch(t.Context(), all, testBatchOnePrefix(t), plugin.ProcessSender("stranger")),
		errSendNotPermitted,
		"a process the peer does not name at all must be refused")
	assert.Equal(t, beforeObserverUpdate+1, pluginserver.BusSendRefusedCount(observerParticipant, testBusMsgUpdate),
		"observer's refused announce must increment ze_bus_send_refused_total")
	assert.Equal(t, beforeStrangerUpdate+1, pluginserver.BusSendRefusedCount(strangerParticipant, testBusMsgUpdate),
		"stranger's refused announce must increment ze_bus_send_refused_total")
	require.Len(t, busRefusalEvents(observerParticipant), 1)
	require.Len(t, busRefusalEvents(strangerParticipant), 1)
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
	resetSendPermissionReports(t)
	reg := wireBusRefusalRegistry(t)
	beforeOpUpdate := pluginserver.BusSendRefusedCount("operator", testBusMsgUpdate)
	beforeOpRefresh := pluginserver.BusSendRefusedCount("operator", testBusMsgRefresh)
	beforeUnsetUpdate := pluginserver.BusSendRefusedCount(testUnsetSender, testBusMsgUpdate)
	beforeUnsetRefresh := pluginserver.BusSendRefusedCount(testUnsetSender, testBusMsgRefresh)

	bare, conn := newAttachedPeer(t, "10.0.0.2")
	adapter := newSendPermissionReactor(t, bare)
	all := selector.All()

	require.NoError(t, adapter.AnnounceEOR(all, uint16(family.AFIIPv4), uint8(family.SAFIUnicast), plugin.OperatorSender()))
	require.Equal(t, eorWire(family.IPv4Unicast), conn.written(),
		"an operator command must reach a peer that attaches nothing")
	require.NoError(t, adapter.SendRefresh(all, uint16(family.AFIIPv4), uint8(family.SAFIUnicast), plugin.OperatorSender()))

	require.NotNil(t, reg.vec)
	assert.Empty(t, reg.vec.counters, "an operator command must increment no refusal counter series")
	assert.Equal(t, beforeOpUpdate, pluginserver.BusSendRefusedCount("operator", testBusMsgUpdate),
		"an operator update command must record no refusal")
	assert.Equal(t, beforeOpRefresh, pluginserver.BusSendRefusedCount("operator", testBusMsgRefresh),
		"an operator refresh command must record no refusal")
	assert.Equal(t, beforeUnsetUpdate, pluginserver.BusSendRefusedCount(testUnsetSender, testBusMsgUpdate),
		"an operator update command must not record an unset refusal")
	assert.Equal(t, beforeUnsetRefresh, pluginserver.BusSendRefusedCount(testUnsetSender, testBusMsgRefresh),
		"an operator refresh command must not record an unset refusal")
	assert.Empty(t, busRefusalEvents(""), "an operator command must raise no refusal event on the report bus")
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
// points that resolve a selector, and says so through the log,
// ze_bus_send_refused_total, and CodeDoctorPluginRequestRefused on the report
// bus.
// PREVENTS: a new dispatch path that forgets to set CommandContext.Sender
// inheriting the operator's authority by omission, on the one guard that stops
// a process reaching a peer that never attached it (ai/rules/evidence.md).
func TestSendPermissionRefusesACommandWithNoSender(t *testing.T) {
	resetSendPermissionReports(t)
	reg := wireBusRefusalRegistry(t)
	beforeUnsetUpdate := pluginserver.BusSendRefusedCount(testUnsetSender, testBusMsgUpdate)
	beforeUnsetRefresh := pluginserver.BusSendRefusedCount(testUnsetSender, testBusMsgRefresh)
	beforeEmptyUpdate := pluginserver.BusSendRefusedCount("", testBusMsgUpdate)
	beforePrefixUpdate := pluginserver.BusSendRefusedCount(plugin.BusEndpointPathPrefix, testBusMsgUpdate)

	bare, conn := newAttachedPeer(t, "10.0.0.2")
	attached, attachedConn := newAttachedPeer(t, "10.0.0.5", sendUpdateOnly("injector"))
	adapter := newSendPermissionReactor(t, bare, attached)
	all := selector.All()

	var nobody plugin.Sender // the zero value: nobody said who is sending

	err := adapter.AnnounceEOR(all, uint16(family.AFIIPv4), uint8(family.SAFIUnicast), nobody)
	require.ErrorIs(t, err, errSendNoSender, "an End-of-RIB from nobody must be refused")
	assert.Contains(t, err.Error(), "CommandContext.Sender",
		"the refusal must name the field the dispatch path failed to set")

	require.ErrorIs(t, adapter.AnnounceNLRIBatch(t.Context(), all, testBatchOnePrefix(t), nobody), errSendNoSender,
		"an announce from nobody must be refused")
	require.ErrorIs(t, adapter.SendRefresh(all, uint16(family.AFIIPv4), uint8(family.SAFIUnicast), nobody), errSendNoSender,
		"a ROUTE-REFRESH from nobody must be refused")
	_, err = adapter.SoftClearPeer(all, nobody)
	require.ErrorIs(t, err, errSendNoSender, "a soft clear from nobody must be refused")

	require.Empty(t, conn.written(),
		"a peer that attaches nothing must receive nothing from an unnamed sender")
	require.Empty(t, attachedConn.written(),
		"a peer that attaches a process must receive nothing from an unnamed sender either")

	// Reported, not only refused. The series is keyed on the participant label
	// ("unset" for a zero Sender) and the bus message name.
	require.NotNil(t, reg.vec)
	require.Contains(t, reg.vec.counters, testUnsetSender+"|"+testBusMsgUpdate)
	assert.Equal(t, 2, reg.vec.counters[testUnsetSender+"|"+testBusMsgUpdate].n,
		"the two UPDATE commands must move the Prometheus unset|bgp:update series once each")
	require.Contains(t, reg.vec.counters, testUnsetSender+"|"+testBusMsgRefresh)
	assert.Equal(t, 2, reg.vec.counters[testUnsetSender+"|"+testBusMsgRefresh].n,
		"the two ROUTE-REFRESH commands must move the Prometheus unset|bgp:refresh series once each")
	assert.NotContains(t, reg.vec.counters, "|"+testBusMsgUpdate,
		"an unset sender must never be recorded under an empty participant label")
	assert.Equal(t, beforeUnsetUpdate+2, pluginserver.BusSendRefusedCount(testUnsetSender, testBusMsgUpdate),
		"the two UPDATE commands must move the unset/bgp:update series once each")
	assert.Equal(t, beforeUnsetRefresh+2, pluginserver.BusSendRefusedCount(testUnsetSender, testBusMsgRefresh),
		"the two ROUTE-REFRESH commands must move the unset/bgp:refresh series once each")
	assert.Equal(t, beforeEmptyUpdate, pluginserver.BusSendRefusedCount("", testBusMsgUpdate),
		"an unset sender must never be recorded under an empty participant name")
	assert.Equal(t, beforePrefixUpdate, pluginserver.BusSendRefusedCount(plugin.BusEndpointPathPrefix, testBusMsgUpdate),
		"an unset sender must never be recorded under a bare endpoint path prefix")

	events := busRefusalEvents(testUnsetSender)
	require.Len(t, events, 4, "all four no-sender refusals must raise CodeDoctorPluginRequestRefused on the report bus")
	for _, ev := range events {
		assert.Equal(t, reportSourceBGPReactor, ev.Source)
		assert.Equal(t, diagnostic.CodeDoctorPluginRequestRefused, ev.Code)
		assert.Equal(t, testUnsetSender, ev.Subject)
		assert.Contains(t, ev.Fix, "CommandContext.Sender")
	}
}

// TestRecordSendRefusedIsSilentWithoutARegistry verifies the bus refusal
// recorder and peer denial path are safe before any Prometheus metrics registry
// is wired, which is the default build.
//
// VALIDATES: a daemon with metrics disabled still refuses, logs, counts via the
// shared bus refusal recorder, and raises CodeDoctorPluginRequestRefused on the
// report bus without panicking.
// PREVENTS: a nil-pointer panic on the refusal path in every such build.
func TestRecordSendRefusedIsSilentWithoutARegistry(t *testing.T) {
	resetSendPermissionReports(t)
	pluginserver.SetBusSendRefusedMetrics(nil)
	t.Cleanup(func() { pluginserver.SetBusSendRefusedMetrics(nil) })

	participant := plugin.EndpointPath("injector")
	beforeUpdate := pluginserver.BusSendRefusedCount(participant, testBusMsgUpdate)
	beforeRefresh := pluginserver.BusSendRefusedCount(participant, testBusMsgRefresh)

	assert.NotPanics(t, func() {
		pluginserver.RecordBusSendRefused(participant, testBusMsgUpdate)
	}, "the shared bus refusal recorder must not panic without a metrics registry")
	assert.Equal(t, beforeUpdate+1, pluginserver.BusSendRefusedCount(participant, testBusMsgUpdate))

	// Wire a registry, then clear it via SetBusSendRefusedMetrics(nil) and prove
	// the detached registry receives no increments while recording stays safe.
	reg := &announceFakeRegistry{}
	pluginserver.SetBusSendRefusedMetrics(reg)
	pluginserver.SetBusSendRefusedMetrics(nil)

	peer, _ := newAttachedPeer(t, "10.0.0.9")
	assert.NotPanics(t, func() {
		sendPermissionDenied(peer, refreshOrigin(plugin.ProcessSender("injector")))
	}, "peer send refusal must not panic after clearing the metrics registry")
	require.NotNil(t, reg.vec)
	assert.Empty(t, reg.vec.counters, "cleared Prometheus registry must receive no increments")
	assert.Equal(t, beforeRefresh+1, pluginserver.BusSendRefusedCount(participant, testBusMsgRefresh))
	require.Len(t, busRefusalEvents(participant), 1,
		"peer send refusal must still raise CodeDoctorPluginRequestRefused without a metrics registry")
}
