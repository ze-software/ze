package reactor

import (
	"bufio"
	"bytes"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/family"
)

// newInitialSyncPeer returns a peer primed to run sendInitialRoutes with the
// given negotiated families. When established is true it is backed by an
// Established Session writing into the returned recordingConn, so a test can
// assert on the bytes that actually reached the wire; when false the Session is
// left in Idle, which is the state a session that went away mid-establishment
// presents to SendUpdateHeld (session_write.go: ErrInvalidState, nothing written).
func newInitialSyncPeer(t *testing.T, established bool, families ...family.Family) (*Peer, *recordingConn) {
	t.Helper()
	settings := &PeerSettings{
		Connection: ConnectionBoth,
		Address:    netip.MustParseAddr("10.0.0.2"),
		LocalAS:    65000,
		PeerAS:     65001,
		RouterID:   0x01020301,
	}
	peer := NewPeer(settings)
	peer.state.Store(int32(PeerStateEstablished))

	nc := &NegotiatedCapabilities{families: make(map[family.Family]bool, len(families))}
	for _, f := range families {
		nc.families[f] = true
	}
	peer.negotiated.Store(nc)

	session := NewSession(settings)
	if established {
		require.NoError(t, session.fsm.Event(fsm.EventManualStart))
		require.NoError(t, session.fsm.Event(fsm.EventTCPConnectionConfirmed))
		require.NoError(t, session.fsm.Event(fsm.EventBGPOpen))
		require.NoError(t, session.fsm.Event(fsm.EventKeepaliveMsg))
		require.Equal(t, fsm.StateEstablished, session.fsm.State())
	}

	conn := &recordingConn{}
	session.mu.Lock()
	session.conn = conn
	session.bufWriter = bufio.NewWriterSize(conn, 4096)
	session.mu.Unlock()

	peer.mu.Lock()
	peer.session = session
	peer.mu.Unlock()

	// The FSM callback sets the flag to 1 before sendInitialRoutes upgrades 1->2.
	peer.sendingInitialRoutes.Store(1)
	return peer, conn
}

// TestInitialSyncEORCountedOncePerFamilyOnTheWire pins the success half of the
// eor-sent contract: one End-of-RIB per negotiated family, counted only after the
// frame reached the socket.
//
// VALIDATES: RFC 4724 Section 4 -- an EOR per negotiated family at the end of the
// initial update -- and that eor-sent reports exactly those frames.
// PREVENTS: an over-strict error guard silently zeroing a counter that operators
// (`show bgp peer <sel> detail`) and ~20 functional tests depend on.
func TestInitialSyncEORCountedOncePerFamilyOnTheWire(t *testing.T) {
	peer, conn := newInitialSyncPeer(t, true, family.IPv4Unicast, family.IPv6Unicast)

	peer.sendInitialRoutes()

	want := append(eorWire(family.IPv4Unicast), eorWire(family.IPv6Unicast)...)
	assert.Equal(t, want, conn.written(),
		"both negotiated families' End-of-RIB markers must reach the wire, AFI order")
	assert.Equal(t, uint32(2), peer.Stats().EORSent, "one counted EOR per family sent")
}

// TestInitialSyncEORReachesTheSilentFamilyToo pins the clause that decides WHICH
// families get a marker: the loop walks the NEGOTIATED families, not the families
// that happened to carry a route. One family announces a prefix here and the other
// announces nothing, and both still get their End-of-RIB.
//
// RFC requirement: RFC4724-4-1 positive -- "The End-of-RIB marker MUST be sent by a
// BGP speaker to its peer once it completes the initial routing update (including
// the case when there is no update to send) for an address family after the BGP
// session is established" (RFC 4724 Section 4). ipv6/unicast is the parenthesised
// case: it has no update to send and its marker is owed all the same.
// RFC requirement: RFC4724-4.2-9 positive -- the same frame satisfies the Receiving
// Speaker's obligation to mark the end of its initial update per address family.
//
// VALIDATES: the End-of-RIB loop in sendInitialRoutes iterates nc.Families().
// PREVENTS: the reading three deleted tests in peer_test.go asserted on -- that a
// family is owed a marker only when a route was sent for it. Those tests populated
// their own local map and never called production code, so they were green against
// any implementation, and one session read them as proof of a conformance gap.
func TestInitialSyncEORReachesTheSilentFamilyToo(t *testing.T) {
	peer, conn := newInitialSyncPeer(t, true, family.IPv4Unicast, family.IPv6Unicast)
	peer.settings.StaticRoutes = []StaticRoute{{
		Prefix:  netip.MustParsePrefix("192.0.2.0/24"),
		NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr("10.0.0.1")),
	}}

	peer.sendInitialRoutes()

	markers := append(eorWire(family.IPv4Unicast), eorWire(family.IPv6Unicast)...)
	written := conn.written()
	assert.True(t, bytes.HasSuffix(written, markers),
		"both negotiated families' markers must close the initial update, the silent one included")
	assert.Greater(t, len(written), len(markers),
		"the ipv4 route UPDATE must precede the markers, or the fixture sent no route and the "+
			"silent-family case is not the one under test")
	assert.Equal(t, uint32(2), peer.Stats().EORSent, "one counted EOR per negotiated family")
}

// TestInitialSyncEORAlsoCountsAsAnUpdateSent pins the counter ARITHMETIC that every
// "did a route reach this peer" gate in the functional suite is built on.
//
// RFC 4724 Section 2 makes the End-of-RIB an UPDATE message with no reachable NLRI
// and no withdrawn routes, so it travels the ordinary UPDATE write path and
// notifyMessageReceiver (reactor_notify.go) counts it in updates-sent exactly like a
// route: the sent branch switches on the message TYPE and nothing there separates
// the marker from a route. eor-sent is the second counter that names the subset, so
// updates-sent MINUS eor-sent is the count of frames that were not the marker, and
// that difference is the only reading that witnesses a route.
//
// VALIDATES: updates-sent includes the initial-sync End-of-RIB, and the difference
// against eor-sent is zero for a peer that was sent nothing else.
// PREVENTS: an observer gating on `updates-sent >= 1` and reading establishment as
// delivery. test/plugin/remove-private-as-replace-peer.ci did exactly that and then
// dispatched `request shutdown`, so under load the daemon was torn down before it
// forwarded and the peer got a Cease NOTIFICATION where the UPDATE was due.
func TestInitialSyncEORAlsoCountsAsAnUpdateSent(t *testing.T) {
	r := New(&Config{ListenAddr: "127.0.0.1:0"})
	addr := netip.MustParseAddr("10.0.0.2")
	require.NoError(t, r.AddPeer(NewPeerSettings(addr, 65000, 65001, 0x01020301)))

	r.mu.RLock()
	peer, found := r.findPeerByAddr(addr)
	r.mu.RUnlock()
	require.True(t, found, "the peer just added must resolve, or the counters below belong to nobody")

	peer.state.Store(int32(PeerStateEstablished))
	nc := &NegotiatedCapabilities{families: map[family.Family]bool{family.IPv4Unicast: true}}
	peer.negotiated.Store(nc)

	session := NewSession(peer.Settings())
	require.NoError(t, session.fsm.Event(fsm.EventManualStart))
	require.NoError(t, session.fsm.Event(fsm.EventTCPConnectionConfirmed))
	require.NoError(t, session.fsm.Event(fsm.EventBGPOpen))
	require.NoError(t, session.fsm.Event(fsm.EventKeepaliveMsg))
	require.Equal(t, fsm.StateEstablished, session.fsm.State())

	// The peer's own callback, the one AddPeer installed, so the counters move
	// through the production path rather than through a stub the test wrote.
	session.SetMessageCallback(peer.messageCallback)
	conn := &recordingConn{}
	session.mu.Lock()
	session.conn = conn
	session.bufWriter = bufio.NewWriterSize(conn, 4096)
	session.mu.Unlock()

	peer.mu.Lock()
	peer.session = session
	peer.mu.Unlock()
	peer.sendingInitialRoutes.Store(1)

	peer.sendInitialRoutes()

	require.Equal(t, eorWire(family.IPv4Unicast), conn.written(),
		"the initial sync must have put the End-of-RIB and nothing else on the wire, or the "+
			"arithmetic below is measuring a route as well as the marker")

	stats := peer.Stats()
	assert.Equal(t, uint32(1), stats.EORSent, "the marker is counted as an End-of-RIB")
	assert.Equal(t, uint32(1), stats.UpdatesSent,
		"the marker is ALSO counted as an UPDATE, so `updates-sent >= 1` is already true "+
			"with no route sent")
	assert.Equal(t, uint32(0), stats.UpdatesSent-stats.EORSent,
		"updates-sent minus eor-sent is the count of frames that were not the marker")
}

// TestNoTestBuildsItsOwnFamiliesSentMap refuses the return of the three tests the
// two above replaced: a test that re-implements the End-of-RIB family tracking in a
// local map and then asserts on its own copy.
//
// VALIDATES: the End-of-RIB family set is asserted from production code, never
// simulated inside the test that names it.
// PREVENTS: the reason those tests survived for so long -- they DO assert, so the
// assert-nothing detector (`make ze-test-sensitivity-check`) never saw them, and
// their names read as coverage of RFC 4724.
//
// Two limitations, stated rather than hidden. It greps ONE identifier, so the same
// defect written with a different variable name is invisible to it. And
// filepath.Glob resolves against the package directory, so its reach is THIS
// package and no other -- which is where the three tests lived, and is all the
// claim it makes. No gate in this repository catches the general class
// (`ai/rules/testing.md`, "A test that re-implements the logic it names").
func TestNoTestBuildsItsOwnFamiliesSentMap(t *testing.T) {
	// Split so the guard does not match its own source.
	needle := "families" + "Sent"

	files, err := filepath.Glob("*_test.go")
	require.NoError(t, err)
	require.NotEmpty(t, files,
		"the glob saw no test file, so this guard would pass while checking nothing")

	for _, name := range files {
		src, err := os.ReadFile(name)
		require.NoError(t, err)
		// The NotContains form of this check prints the whole file as its haystack,
		// leaving the reader to hunt for the one line that matched.
		assert.False(t, strings.Contains(string(src), needle),
			"%s re-implements the End-of-RIB family tracking in a local map. Drive "+
				"sendInitialRoutes and assert on the wire instead: see "+
				"TestInitialSyncEORReachesTheSilentFamilyToo", name)
	}
}

// TestInitialSyncEORNotCountedWhenSessionLeftEstablished is the failure half: when
// the session drops between sendInitialRoutes reading p.session and the EOR write,
// SendUpdateHeld returns ErrInvalidState WITHOUT writing anything, and the counter
// must not move.
//
// VALIDATES: the IncrEORSent contract (peer_stats.go) -- "eor-sent >= 1 means the
// marker is on the wire" -- which test/scripts/ze_api.py wait_peer_eor_sent and the
// functional suite use as the initial-sync barrier before asserting the EOR frame.
// PREVENTS: the pre-fix `_ = sendFn(...)` followed by an unconditional
// IncrEORSent, which published an End-of-RIB the peer never received and so turned
// every barrier built on the counter into one that waits for nothing.
func TestInitialSyncEORNotCountedWhenSessionLeftEstablished(t *testing.T) {
	peer, conn := newInitialSyncPeer(t, false, family.IPv4Unicast, family.IPv6Unicast)

	peer.sendInitialRoutes()

	assert.Empty(t, conn.written(), "no EOR can reach a session that is not Established")
	assert.Equal(t, uint32(0), peer.Stats().EORSent,
		"a failed EOR send must not be counted: the peer never received the marker")
}

// TestInitialSyncEORStopsAtFirstSendFailure verifies the loop stops on the first
// error instead of walking the remaining families. One failure here is
// session-level (ErrNotConnected / ErrInvalidState / a write error), so every
// later family would fail identically.
//
// VALIDATES: the connection-error convention already used by this file's queue
// drain (stop, do not spin per item).
// PREVENTS: one log line and one counter decision per negotiated family for a
// single session-down event.
func TestInitialSyncEORStopsAtFirstSendFailure(t *testing.T) {
	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65000, 65001, 0x01020304)
	peer := NewPeer(settings)
	nc := &NegotiatedCapabilities{families: map[family.Family]bool{
		family.IPv4Unicast: true,
		family.IPv6Unicast: true,
	}}
	peer.negotiated.Store(nc)
	peer.sendingInitialRoutes.Store(1)

	// No session at all: sendFn is p.sendUpdateDirect -> ErrNotConnected.
	peer.sendInitialRoutes()

	assert.Equal(t, uint32(0), peer.Stats().EORSent,
		"no EOR is on the wire when the peer has no session")
}

// TestInitialSyncEORWaitsForPeerUpBarrier is the one test that pins the barrier
// to the thing it exists to gate: no End-of-RIB reaches the wire while a barrier
// plugin still owes an acknowledgement for this session's peer-up event.
//
// VALIDATES: sendInitialRoutes calls waitPeerUpBarrier BEFORE it writes the
// marker, so "End-of-RIB sent" implies "every plugin that registers this peer as
// a forward target has processed the peer-up event".
// PREVENTS: the barrier becoming dead code. Every other barrier test drives
// waitPeerUpBarrier directly; delete the single call in peer_initial_sync.go and
// they all stay green while the guarantee silently disappears
// (ai/rules/evidence.md, "drive the guard from the entry point").
//
// The oracle is triggerClock.waiting, which receives when waitPeerUpBarrier
// evaluates its select operands, i.e. exactly when sendInitialRoutes is inside
// the wait and has not passed it. Without that signal the test would be a race
// between the goroutine and an assertion, and it would pass with the wait
// deleted; with it, deleting the wait means the receive below never happens and
// the test fails. Its clock's After() only fires when the test says so, so a
// timeout cannot substitute for the acknowledgement either.
func TestInitialSyncEORWaitsForPeerUpBarrier(t *testing.T) {
	peer, conn := newInitialSyncPeer(t, true, family.IPv4Unicast)
	tc := newTriggerClock()
	peer.SetClock(tc)
	peer.ResetPeerUpBarrier()
	peer.SetPeerUpBarrier(1) // one plugin registers this peer; it has not acknowledged

	done := make(chan struct{})
	go func() {
		peer.sendInitialRoutes()
		close(done)
	}()

	select {
	case <-tc.waiting:
	case <-time.After(5 * time.Second):
		t.Fatal("sendInitialRoutes never entered the peer-up barrier: the end-of-rib does not " +
			"wait for the plugins that register this peer as a forward target")
	}
	assert.Empty(t, conn.written(),
		"the end-of-rib must not reach the wire while a barrier plugin still owes an acknowledgement")

	peer.SignalPeerUpBarrier()

	require.Eventually(t, func() bool {
		select {
		case <-done:
			return true
		default:
			return false
		}
	}, 2*time.Second, time.Millisecond, "sendInitialRoutes must resume once the barrier opens")
	assert.Equal(t, eorWire(family.IPv4Unicast), conn.written(),
		"the end-of-rib must reach the wire once every barrier plugin has acknowledged")
	assert.Equal(t, uint32(1), peer.Stats().EORSent)
}

// TestDefaultOriginateFilterFailsClosedWithoutReactor verifies that the
// default-originate conditional filter fails closed when the peer has no
// reactor attached.
//
// VALIDATES: cmd-2 AC-7 guardrail -- a filter that cannot be evaluated
// must not silently originate the default route.
// PREVENTS: A missing reactor/API causing default routes to leak out
// unfiltered while the operator believes the filter is enforcing policy.
func TestDefaultOriginateFilterFailsClosedWithoutReactor(t *testing.T) {
	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65000, 65001, 0x01020304)
	peer := NewPeer(settings)

	// No reactor attached -- fail-closed branch.
	fam := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast}
	prefix := netip.MustParsePrefix("0.0.0.0/0")
	nh := netip.MustParseAddr("10.0.0.1")

	ok := peer.defaultOriginateFilterAccepts("policy:drop-all", fam, prefix, nh)
	assert.False(t, ok, "missing reactor must fail closed to prevent unfiltered origination")
}

// TestDefaultOriginateFilterFailsClosedOnMalformedRef verifies that a
// malformed filter reference (no "<plugin>:<filter>" colon) fails closed
// instead of being silently ignored.
//
// VALIDATES: cmd-2 AC-7 guardrail -- invalid config must not let a
// default route escape without filtering.
// PREVENTS: Typos in filter names ("drop" instead of "policy:drop")
// silently disabling the filter and originating the default.
func TestDefaultOriginateFilterFailsClosedOnMalformedRef(t *testing.T) {
	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65000, 65001, 0x01020304)
	peer := NewPeer(settings)
	// Attach a reactor so the nil-reactor branch is not taken.
	r := &Reactor{}
	peer.SetReactor(r)

	fam := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast}
	prefix := netip.MustParsePrefix("0.0.0.0/0")
	nh := netip.MustParseAddr("10.0.0.1")

	ok := peer.defaultOriginateFilterAccepts("missing-colon", fam, prefix, nh)
	assert.False(t, ok, "malformed filter ref must fail closed")
}

// fakeFilterRawInfo is a filterRawInfo stub that reports a fixed raw flag and
// records the plugin/filter names it was queried with.
type fakeFilterRawInfo struct {
	raw       bool
	gotPlugin string
	gotFilter string
}

func (f *fakeFilterRawInfo) FilterInfo(pluginName, filterName string) ([]string, bool) {
	f.gotPlugin, f.gotFilter = pluginName, filterName
	return nil, f.raw
}

// TestDefaultOriginateRejectsRawFilter verifies a filter declared raw=true is
// rejected as a default-originate gate: the synthetic default route has no wire
// bytes, so a raw filter would evaluate empty hex and decide on nothing.
//
// VALIDATES: L119 -- raw filters bound to default-originate-filter must not
// silently gate on empty hex; fail-closed instead.
// PREVENTS: a raw filter accepting/rejecting the default route based on an empty
// raw payload, silently emitting or suppressing 0.0.0.0/0.
func TestDefaultOriginateRejectsRawFilter(t *testing.T) {
	info := &fakeFilterRawInfo{raw: true}
	rejected := defaultOriginateRejectsRawFilter(info, "policy:raw-thing", "192.0.2.1")
	assert.True(t, rejected, "a raw=true filter must be rejected for default-originate")
	assert.Equal(t, "policy", info.gotPlugin, "ref must be split on ':' before the raw lookup")
	assert.Equal(t, "raw-thing", info.gotFilter)
}

// TestDefaultOriginateAllowsNonRawFilter verifies a text (raw=false) filter is
// not blocked by the raw guard and proceeds to the normal dry-run.
//
// VALIDATES: L119 -- the raw guard only blocks raw filters, leaving text gates working.
// PREVENTS: the raw guard over-reaching and disabling legitimate text filters.
func TestDefaultOriginateAllowsNonRawFilter(t *testing.T) {
	info := &fakeFilterRawInfo{raw: false}
	rejected := defaultOriginateRejectsRawFilter(info, "policy:text-gate", "192.0.2.1")
	assert.False(t, rejected, "a text filter must not be blocked by the raw guard")
}

// TestDefaultOriginateRawGuardIgnoresMalformedRef verifies the raw guard leaves a
// ref without a ':' alone (the caller's colon check already fails it closed), and
// does not perform a lookup on a bogus split.
//
// VALIDATES: L119 -- the raw guard defers malformed refs to the existing check.
// PREVENTS: double-handling / a lookup with an empty filter name.
func TestDefaultOriginateRawGuardIgnoresMalformedRef(t *testing.T) {
	info := &fakeFilterRawInfo{raw: true}
	rejected := defaultOriginateRejectsRawFilter(info, "no-colon", "192.0.2.1")
	assert.False(t, rejected, "malformed ref must be left to the caller's colon check")
	assert.Empty(t, info.gotPlugin, "no lookup should happen on a malformed ref")
}

// newDefaultOriginatePeer returns a peer with IPv6 default-originate enabled and
// an Established session recording what reaches the wire.
//
// peerAddr decides the peer half of RFC 2545 Section 3's condition, and
// localAddress is both the session's local endpoint and the default route's next
// hop (defaultRouteForAFI, peer_initial_sync.go).
func newDefaultOriginatePeer(t *testing.T, peerAddr, localAddress, linkLocal string) (*Peer, *recordingConn) {
	t.Helper()
	settings := &PeerSettings{
		Connection:       ConnectionBoth,
		Address:          netip.MustParseAddr(peerAddr),
		LocalAddress:     netip.MustParseAddr(localAddress),
		LinkLocal:        netip.MustParseAddr(linkLocal),
		LocalAS:          65000,
		PeerAS:           65001,
		RouterID:         0x01020301,
		DefaultOriginate: map[string]bool{"ipv6/unicast": true},
	}
	peer := NewPeer(settings)
	peer.state.Store(int32(PeerStateEstablished))

	nc := &NegotiatedCapabilities{families: map[family.Family]bool{family.IPv6Unicast: true}}
	peer.negotiated.Store(nc)

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

	// setEncodingContexts (peer.go) refreshes the link scope at establishment.
	peer.refreshLinkScope()
	peer.sendingInitialRoutes.Store(1)
	return peer, conn
}

// mpReachIPv6Attr builds the MP_REACH_NLRI attribute a conforming encoder writes
// for an IPv6 unicast route with the given NLRI bytes and next-hop addresses.
//
// RFC 2545 Section 3 wire form: AFI(2) SAFI(1) Length(1) address(es) Reserved(1)
// NLRI. The Length octet is 16 for one address and 32 for two, and the global
// address is always first.
func mpReachIPv6Attr(t *testing.T, nlriBytes []byte, addrs ...string) []byte {
	t.Helper()
	var nh []byte
	for _, a := range addrs {
		b := netip.MustParseAddr(a).As16()
		nh = append(nh, b[:]...)
	}
	value := []byte{0x00, 0x02, 0x01, byte(len(nh))}
	value = append(value, nh...)
	value = append(value, 0x00) // Reserved
	value = append(value, nlriBytes...)
	return append([]byte{0x80, 0x0E, byte(len(value))}, value...)
}

// defaultRouteNLRI is ::/0 on the wire: a prefix length of zero and no address
// octets.
var defaultRouteNLRI = []byte{0x00}

// TestDefaultOriginateAppendsLinkLocalWhenSection3Holds drives RFC 2545 Section 3
// through the default-originate rail, from sendInitialRoutes to the socket.
//
// RFC requirement: RFC2545-3-1 positive -- the Next Hop field of the originated
// default route carries the global IPv6 address of the next hop followed by the
// link-local IPv6 address of the next hop.
//
// RFC requirement: RFC2545-3-2 positive -- the Length of Next Hop Network Address
// octet is 32 (0x20) because a link-local address is also included.
//
// RFC requirement: RFC2545-3-3 positive -- both halves of the condition hold: the
// speaker shares the loopback subnet with the entity named by the global next hop
// (::1) and with the peer the route is advertised to (fd00::2).
//
// VALIDATES: the originated ::/0 leaves with the 32-octet form, global first.
// PREVENTS: the default-originate rail emitting the 16-octet form in a case
// Section 3 requires the second address, which no other test covers -- the
// exabgp-compat pair drives the STATIC route rail.
//
// The peer is fd00::2 and the next hop is ::1, and they must stay different.
// RFC 4271 Section 5.1.3 forbids advertising a peer its own address as NEXT_HOP,
// and originatedNextHopIsPeerOwn (forward_next_hop.go) refuses it, so a fixture
// that gives both ends ::1 asserts the wire form of a message Ze must never send.
// `make ze-dev-setup` provisions fd00::2 on the loopback.
func TestDefaultOriginateAppendsLinkLocalWhenSection3Holds(t *testing.T) {
	peer, conn := newDefaultOriginatePeer(t, "fd00::2", "::1", "fe80::1")

	peer.sendInitialRoutes()

	assert.Contains(t, string(conn.written()), string(mpReachIPv6Attr(t, defaultRouteNLRI, "::1", "fe80::1")),
		"RFC 2545 Section 3: global address first, link-local second, length octet 0x20")
}

// TestDefaultOriginateOmitsLinkLocalWhenPeerOffLink is the other polarity.
//
// RFC requirement: RFC2545-3-3 negative -- the peer half of the condition fails
// (2001:db8:dead:beef::2 sits on no locally connected subnet), so the link-local
// address is NOT included even though the leaf names one.
//
// RFC requirement: RFC2545-3-4 positive -- "in all other cases" the speaker
// advertises only the global address and sets the length octet to 16.
//
// VALIDATES: the same rail emits the 16-octet form when Section 3 excludes the
// second address. Without this row an encoder that always appended would pass the
// positive test above.
func TestDefaultOriginateOmitsLinkLocalWhenPeerOffLink(t *testing.T) {
	peer, conn := newDefaultOriginatePeer(t, "2001:db8:dead:beef::2", "::1", "fe80::1")

	peer.sendInitialRoutes()

	written := string(conn.written())
	assert.Contains(t, written, string(mpReachIPv6Attr(t, defaultRouteNLRI, "::1")),
		"RFC 2545 Section 3: the global address alone, length octet 0x10")
	assert.NotContains(t, written, string(mpReachIPv6Attr(t, defaultRouteNLRI, "::1", "fe80::1")),
		"no link-local may be appended when the peer shares no subnet with the speaker")
}

// TestInitialSyncEORSentWhenNeitherSideDeclaredAFamily drives the whole chain a
// peer with no `family` block walks: capability.Negotiate over two OPENs that
// carry no Multiprotocol capability, NewNegotiatedCapabilities over the result,
// then sendInitialRoutes. The minimum-length UPDATE that closes the initial
// routing update must reach the socket.
//
// RFC requirement: RFC4724-4-1 positive -- "The End-of-RIB marker MUST be sent by
// a BGP speaker to its peer once it completes the initial routing update
// (including the case when there is no update to send) for an address family
// after the BGP session is established" (RFC 4724 Section 4). A session where
// neither side advertised a Multiprotocol capability still exchanges IPv4 unicast
// under RFC 4271, so IPv4 unicast is "an address family" here and its marker is
// owed.
//
// VALIDATES: AC-1 and AC-5 -- the marker reaches the wire for a peer configured
// with no family block, and the counter operators and the functional suite's
// initial-sync barrier read moves with it.
// PREVENTS: the silent skip this spec exists to remove. Before the fix in
// capability.Negotiate, both sides contributed the empty set, nc.Families() was
// empty, the End-of-RIB loop never ran, and nothing logged or counted the
// absence: the peer waited forever for a barrier that never arrived.
//
// It negotiates rather than populating the family map, which is the point: the
// sibling tests above build NegotiatedCapabilities directly and so cannot see a
// defect in the producer that fills it.
func TestInitialSyncEORSentWhenNeitherSideDeclaredAFamily(t *testing.T) {
	peer, conn := newInitialSyncPeer(t, true)
	peer.negotiated.Store(NewNegotiatedCapabilities(capability.Negotiate(nil, nil, 65000, 65001)))

	peer.sendInitialRoutes()

	assert.Equal(t, eorWire(family.IPv4Unicast), conn.written(),
		"a session that declared no Multiprotocol capability still exchanges IPv4 "+
			"unicast, so its End-of-RIB marker is owed (RFC 4724 Section 4)")
	assert.Equal(t, uint32(1), peer.Stats().EORSent, "the marker is counted once")
}
