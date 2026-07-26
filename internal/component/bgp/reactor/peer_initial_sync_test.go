package reactor

import (
	"bufio"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
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
