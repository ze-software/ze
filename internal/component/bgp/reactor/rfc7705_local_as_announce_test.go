// Design: docs/architecture/wire/attributes.md -- AS_PATH on the announce rail
// RFC: rfc/short/rfc4271.md
// Related: reactor_api_batch.go -- AnnounceNLRIBatch, the entry point under test
// Related: peer_forward_facts.go -- localASPrepend, the fact both rails read
// Overview: rfc7705_local_as_test.go -- the same three configurations on the FORWARD rail
//
// A route ze ORIGINATES leaves by a different rail from a route ze relays.
// AnnounceNLRIBatch builds the UPDATE itself rather than editing one that
// arrived, so RFC 7705 Section 3.3 has to be answered twice, and until
// 2026-09-06 the second answer was missing: buildBatchASPathAttr and
// announceASPathASNs took ONE local AS number and prepended it alone. Toward a
// peer carrying a local-as override with no modifier that produced the
// replace-as bytes, so `local-as`, `local-as no-prepend` and
// `local-as replace-as` were one behavior here exactly as they had been on the
// forward rail before 2026-08-29.
//
// RFC 7705 Section 3.3, the base "Local AS" behavior: the speaker "SHOULD first
// append the globally configured ASN to the AS_PATH immediately followed by the
// 'Local AS' value before advertising the UPDATE to an eBGP neighbor". The
// option that turns the first of those off is "Replace Old AS", under which the
// speaker "MUST NOT append the globally configured ASN from the AS_PATH
// attribute" and "MUST append only the configured 'Local AS' ASN value".
//
// The tests drive AnnounceNLRIBatch and read the bytes each peer's connection
// received. Reading the builder alone would not settle it: the defect was in
// what the ENTRY POINT hands the builder, and until this change the group key
// carried the local AS number without the option beside it, so two peers
// differing only by `replace-as` shared one built UPDATE.
package reactor

import (
	"bufio"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// announceLocalASPeer is one destination of the announce fan-out and the AS_PATH
// it must receive.
type announceLocalASPeer struct {
	name      string
	addr      string
	localAS   uint32 // the effective per-peer local AS: the override where one is set
	peerAS    uint32
	noPrepend bool
	replaceAS bool
	wantPath  []uint32
}

// announceLocalASRun originates one route to every peer given and answers the
// AS_PATH each one received, by address.
//
// groups selects which half of AnnounceNLRIBatch runs. False takes the per-peer
// branch; true takes the build-group branch, where the group key decides whether
// two peers may share one built UPDATE. Both must be exercised, because the key
// is where the defect could come back: a key that carries the local AS number
// without the local-as option beside it puts a replace-as peer and an ordinary
// one in the same group, and whichever built first wins.
func announceLocalASRun(t *testing.T, groups bool, dests []announceLocalASPeer) map[string][]uint32 {
	t.Helper()

	peers := make(map[netip.AddrPort]*Peer, len(dests))
	conns := make(map[string]*recordingConn, len(dests))
	for _, dest := range dests {
		peer, conn := newAnnounceLocalASPeer(t, dest)
		peers[peer.Settings().PeerKey()] = peer
		conns[dest.addr] = conn
	}

	r := &Reactor{
		config:          &Config{LocalAS: localASGlobal},
		peers:           peers,
		attrModHandlers: attrModHandlersWithDefaults(),
	}
	if groups {
		r.updateGroups = newUpdateGroupIndex(true)
	}
	adapter := &reactorAPIAdapter{r: r}

	batch := bgptypes.NLRIBatch{
		Family:  family.IPv4Unicast,
		NLRIs:   []nlri.NLRI{nlri.NewINET(family.IPv4Unicast, netip.MustParsePrefix("10.0.0.0/24"), 0)},
		NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr("1.1.1.1")),
	}
	require.NoError(t, adapter.AnnounceNLRIBatch(selector.All(), batch, plugin.OperatorSender()))

	got := make(map[string][]uint32, len(dests))
	for addr, conn := range conns {
		wire := conn.written()
		require.NotEmpty(t, wire, addr+": the peer must have been sent an UPDATE")
		// RFC 4271 Section 4.1: a 16-octet marker, a 2-octet length and a 1-octet
		// type sit in front of every message body. bodyPathAttr reads a body.
		require.Greater(t, len(wire), 19, addr+": the message must carry a body")
		value, ok := bodyPathAttr(t, wire[19:], 2)
		require.True(t, ok, addr+": every eBGP advertisement carries an AS_PATH")
		got[addr] = aspathASNs(t, value, 4)
	}
	return got
}

// newAnnounceLocalASPeer builds one Established peer backed by a recordingConn,
// the shape newEORGuardPeer uses (reactor_api_forward_test.go), with the
// migration topology's AS numbers on it.
func newAnnounceLocalASPeer(t *testing.T, dest announceLocalASPeer) (*Peer, *recordingConn) {
	t.Helper()
	settings := &PeerSettings{
		Connection: ConnectionBoth,
		Address:    netip.MustParseAddr(dest.addr),
		// GlobalLocalAS is always the router's real AS number, so a peer whose
		// LocalAS differs from it carries a local-as override and a peer whose
		// LocalAS equals it is an ordinary neighbor.
		LocalAS:          dest.localAS,
		GlobalLocalAS:    localASGlobal,
		PeerAS:           dest.peerAS,
		RouterID:         0x01020300 | uint32(netip.MustParseAddr(dest.addr).As4()[3]),
		LocalASNoPrepend: dest.noPrepend,
		LocalASReplaceAS: dest.replaceAS,
	}
	peer := NewPeer(settings)
	peer.state.Store(int32(PeerStateEstablished))
	peer.negotiated.Store(&NegotiatedCapabilities{
		families: map[family.Family]bool{{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast}: true},
	})

	session := NewSession(settings)
	require.NoError(t, session.fsm.Event(fsm.EventManualStart))
	require.NoError(t, session.fsm.Event(fsm.EventTCPConnectionConfirmed))
	require.NoError(t, session.fsm.Event(fsm.EventBGPOpen))
	require.NoError(t, session.fsm.Event(fsm.EventKeepaliveMsg))
	require.Equal(t, fsm.StateEstablished, session.fsm.State())

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

// announceLocalASCases is the migration topology every test below announces
// into: one route, five destinations, and the only thing that differs between
// the first four is `local-options`.
func announceLocalASCases() []announceLocalASPeer {
	return []announceLocalASPeer{
		{
			name: "local-as, no option", addr: "10.0.0.2",
			localAS: localASLegacy, peerAS: 65002,
			wantPath: []uint32{localASLegacy, localASGlobal},
		},
		{
			name: "local-as with no-prepend", addr: "10.0.0.3",
			localAS: localASLegacy, peerAS: 65003, noPrepend: true,
			wantPath: []uint32{localASLegacy, localASGlobal},
		},
		{
			name: "local-as with replace-as", addr: "10.0.0.4",
			localAS: localASLegacy, peerAS: 65004, replaceAS: true,
			wantPath: []uint32{localASLegacy},
		},
		{
			name: "local-as with both options", addr: "10.0.0.5",
			localAS: localASLegacy, peerAS: 65005, noPrepend: true, replaceAS: true,
			wantPath: []uint32{localASLegacy},
		},
		{
			name: "no override: an ordinary neighbor", addr: "10.0.0.6",
			localAS: localASGlobal, peerAS: 65006,
			wantPath: []uint32{localASGlobal},
		},
	}
}

// TestAnnounceLocalASOptionsProduceDifferentASPaths is the announce rail's twin
// of TestLocalASOptionsProduceDifferentASPaths, and it is the test the collapse
// on this rail would have failed.
//
// One originated route, five peers, and the AS_PATH each one received read off
// its own connection. The no-prepend peer and the replace-as peer differ by one
// enum in their configuration and MUST differ on the wire; the ordinary neighbor
// beside them is the control that proves the change reaches only a peer carrying
// an override.
//
// Both polarities are asserted for each option, because the presence assertion
// and the absence assertion fail to different defects: a rail that prepends
// nothing passes "65000 is absent" and a rail that prepends unconditionally
// passes "65000 is present".
//
// VALIDATES: RFC 7705 Section 3.3 on the ORIGINATE rail. The base form appends
// the globally configured ASN and the Local AS; "Replace Old AS" MUST NOT append
// the globally configured ASN and MUST append only the Local AS.
// PREVENTS: the announce rail prepending one AS number for every configuration,
// which made `local-as` behave as `local-as replace-as` for every route ze
// originates.
func TestAnnounceLocalASOptionsProduceDifferentASPaths(t *testing.T) {
	dests := announceLocalASCases()
	paths := announceLocalASRun(t, false, dests)

	for _, dest := range dests {
		assert.Equal(t, dest.wantPath, paths[dest.addr], dest.name)
	}

	noPrepend, replaceAS := paths["10.0.0.3"], paths["10.0.0.4"]
	assert.NotEqual(t, replaceAS, noPrepend,
		"no-prepend is RFC 7705's INBOUND option and replace-as its OUTBOUND one: one announce cannot leave for both peers with the same AS_PATH")

	// Positive polarity: the globally configured AS number IS appended where no
	// outbound option turns it off.
	assert.Contains(t, paths["10.0.0.2"], uint32(localASGlobal),
		"with no option the speaker appends the globally configured ASN and the Local AS")
	assert.Contains(t, noPrepend, uint32(localASGlobal),
		"no-prepend governs the inbound rail, so the globally configured ASN stays on this one")

	// Negative polarity: it is NOT appended where replace-as turns it off, and
	// the Local AS is still there, so the path is shortened rather than emptied.
	assert.NotContains(t, replaceAS, uint32(localASGlobal),
		"replace-as MUST NOT append the globally configured ASN")
	assert.Contains(t, replaceAS, uint32(localASLegacy),
		"replace-as MUST append the configured Local AS: an eBGP advertisement carrying no AS number breaks loop detection")
	assert.NotContains(t, paths["10.0.0.5"], uint32(localASGlobal),
		"both options together are replace-as on the outbound rail")

	// The control: a peer with no override is untouched by either option.
	assert.Equal(t, []uint32{localASGlobal}, paths["10.0.0.6"],
		"a peer with no local-as override receives exactly the globally configured ASN, as it did before RFC 7705 reached this rail")
}

// TestAnnounceLocalASOptionsPartitionUpdateGroups drives the same five peers
// through the BUILD-GROUP branch of AnnounceNLRIBatch.
//
// The group key decides which peers may share one built UPDATE. It carried the
// local AS number alone, so the four peers above carrying local-as 65010
// differed in nothing the key could see and shared one build: whichever
// configuration was built first was sent to all four, and the option an operator
// set had no effect at all. Keying on the whole prepend is what separates them.
//
// VALIDATES: the same RFC 7705 Section 3.3 answers survive update grouping.
// PREVENTS: a group key that reads the AS number and not the option, which is a
// second way to reach the collapse this spec removed.
func TestAnnounceLocalASOptionsPartitionUpdateGroups(t *testing.T) {
	dests := announceLocalASCases()
	paths := announceLocalASRun(t, true, dests)

	for _, dest := range dests {
		assert.Equal(t, dest.wantPath, paths[dest.addr], dest.name+" (grouped)")
	}
	assert.NotEqual(t, paths["10.0.0.4"], paths["10.0.0.3"],
		"two peers whose configuration differs only by replace-as must not share a built UPDATE")
}

// TestAnnounceLocalASOriginASKeepsBothASNs pins the origin-as shape, which is
// the second sequence this rail synthesizes.
//
// A virtual-router announce names the AS the route originates in, and the local
// prepend goes in front of it. Toward a migrating peer that is two AS numbers
// rather than one, so the peer reads [Local AS, globally configured ASN,
// originAS] and sees the legacy AS adjacent to itself, which is the whole point
// of the local-as mechanism.
//
// VALIDATES: announceASPathASNs composes the local prepend with originAS rather
// than replacing it.
// PREVENTS: the origin-as branch keeping the single-AS form after the plain
// export branch was fixed, which would leave the collapse alive on one shape.
func TestAnnounceLocalASOriginASKeepsBothASNs(t *testing.T) {
	const originAS = 64512
	var scratch [3]uint32

	dual := localASPrepend{primary: localASLegacy, secondary: localASGlobal}
	assert.Equal(t, []uint32{localASLegacy, localASGlobal, originAS},
		announceASPathASNs(scratch[:0], false, dual, originAS),
		"an eBGP origin-as announce toward a migrating peer carries both of the router's AS numbers in front of the origin")

	replaceAS := localASOnly(localASLegacy)
	assert.Equal(t, []uint32{localASLegacy, originAS},
		announceASPathASNs(scratch[:0], false, replaceAS, originAS),
		"replace-as removes the globally configured ASN from the origin-as shape too")

	assert.Equal(t, []uint32{originAS},
		announceASPathASNs(scratch[:0], true, dual, originAS),
		"RFC 4271 Section 5.1.2 forbids adding an AS number toward an internal peer, whatever the local-as configuration says")
}

// TestAnnounceLocalASPrependIsOutermostFirst pins the ORDER, which no equality
// on a set would catch.
//
// RFC 7705 Section 3.3 fixes it: the speaker appends the globally configured ASN
// "immediately followed by" the Local AS, so the Local AS ends up closest to the
// peer. A peer that read them the other way round would believe it was adjacent
// to an AS it does not peer with, and the migration's whole purpose is that it
// is not told anything of the sort.
//
// VALIDATES: localASPrepend.asns answers outermost first, and the announce rail
// writes them in that order.
// PREVENTS: the two AS numbers being swapped, which every "both are present"
// assertion in this file would pass.
func TestAnnounceLocalASPrependIsOutermostFirst(t *testing.T) {
	paths := announceLocalASRun(t, false, []announceLocalASPeer{
		{
			name: "local-as, no option", addr: "10.0.0.2",
			localAS: localASLegacy, peerAS: 65002,
			wantPath: []uint32{localASLegacy, localASGlobal},
		},
	})
	path := paths["10.0.0.2"]
	require.Len(t, path, 2, "the base local-as form carries exactly two AS numbers")
	assert.Equal(t, uint32(localASLegacy), path[0],
		"the Local AS is what the peer expects adjacent to itself, so it is outermost")
	assert.Equal(t, uint32(localASGlobal), path[1],
		"the globally configured ASN sits behind it, which is the truth about where the route really came from")

	var scratch [2]uint32
	assert.Equal(t, []uint32{localASLegacy, localASGlobal},
		localASPrepend{primary: localASLegacy, secondary: localASGlobal}.asns(scratch[:0]),
		"asns answers outermost first, which is this rail's order")
}
