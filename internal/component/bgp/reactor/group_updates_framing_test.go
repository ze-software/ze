// Design: docs/architecture/update-building.md -- route grouping and the framing this file guards
// Related: reactor_api_batch.go -- nlriUnitLen, announceBatchToPeers, withdrawBatchFromPeers
// Related: peer_initial_sync.go -- the config-driven sync, which read the same leaf first
// Overview: rfc7705_local_as_announce_test.go -- the harness these tests borrow
//
// `behavior { group-updates false }` asks for one UPDATE per prefix toward one
// peer. Until 2026-09-06 only the config-driven initial sync read the leaf, so
// an operator who wrote it received one UPDATE per route for a route declared in
// the configuration and ONE packed UPDATE for the same route announced through
// the API, with nothing saying so.
//
// The tests drive AnnounceNLRIBatch and WithdrawNLRIBatch and count the MESSAGES
// each peer's connection received, because the number of frames is the whole
// behavior the leaf names. Counting a call inside the builder would pass against
// a rail that builds twice and sends once.
//
// config_test.go already pins the other half of the path: `group-updates false`
// in a peer's behavior block reaches PeerSettings.GroupUpdates.
package reactor

import (
	"bufio"
	"encoding/binary"
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

// groupUpdatesPrefixes is the two-prefix batch every test below sends. Two is
// the smallest batch whose framing differs between the two settings of the leaf.
var groupUpdatesPrefixes = []netip.Prefix{
	netip.MustParsePrefix("10.10.0.0/24"),
	netip.MustParsePrefix("10.10.1.0/24"),
}

// newGroupUpdatesPeer builds one Established peer backed by a recordingConn.
//
// The settings come from NewPeerSettings, so the peer whose leaf this test does
// not touch carries the YANG default (`default true` in ze-bgp-conf.yang, which
// NewPeerSettings is required to match). A test that wrote `GroupUpdates: true`
// into a bare struct literal would prove nothing about an operator who writes no
// leaf at all.
func newGroupUpdatesPeer(t *testing.T, addr, leaf string) (*Peer, *recordingConn) {
	t.Helper()

	ip := netip.MustParseAddr(addr)
	settings := NewPeerSettings(ip, 65000, 65001, 0x01020300|uint32(ip.As4()[3]))
	switch leaf {
	case "false":
		settings.GroupUpdates = false
	case "absent":
		// NewPeerSettings already carries the YANG default.
	default:
		t.Fatalf("unknown group-updates leaf %q", leaf)
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

// groupUpdatesReactor wires the peers given into a reactor, with cross-peer
// update groups on or off.
func groupUpdatesReactor(peers []*Peer, groups bool) *reactorAPIAdapter {
	index := make(map[netip.AddrPort]*Peer, len(peers))
	for _, peer := range peers {
		index[peer.Settings().PeerKey()] = peer
	}
	r := &Reactor{
		config:          &Config{LocalAS: 65000},
		peers:           index,
		attrModHandlers: attrModHandlersWithDefaults(),
	}
	if groups {
		r.updateGroups = newUpdateGroupIndex(true)
	}
	return &reactorAPIAdapter{r: r}
}

// groupUpdatesBatch is the two-prefix IPv4 unicast batch under test.
func groupUpdatesBatch() bgptypes.NLRIBatch {
	nlris := make([]nlri.NLRI, 0, len(groupUpdatesPrefixes))
	for _, prefix := range groupUpdatesPrefixes {
		nlris = append(nlris, nlri.NewINET(family.IPv4Unicast, prefix, 0))
	}
	return bgptypes.NLRIBatch{
		Family:  family.IPv4Unicast,
		NLRIs:   nlris,
		NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr("1.1.1.1")),
	}
}

// updateBodies splits what a connection received into the bodies of its UPDATE
// messages. RFC 4271 Section 4.1: a 16-octet marker, a 2-octet length covering
// the whole message, and a 1-octet type sit in front of every body.
//
// It walks the stream rather than assuming one message, because the number of
// messages IS what these tests measure.
func updateBodies(t *testing.T, wire []byte) [][]byte {
	t.Helper()
	var bodies [][]byte
	for off := 0; off < len(wire); {
		require.GreaterOrEqual(t, len(wire)-off, 19, "a BGP message carries a 19-octet header")
		length := int(binary.BigEndian.Uint16(wire[off+16 : off+18]))
		require.GreaterOrEqual(t, length, 19, "RFC 4271 Section 4.1: length covers the header")
		require.LessOrEqual(t, off+length, len(wire), "the message must fit the bytes written")
		if wire[off+18] == 2 { // UPDATE
			bodies = append(bodies, wire[off+19:off+length])
		}
		off += length
	}
	return bodies
}

// updatePrefixes reads the prefixes an IPv4 unicast UPDATE body carries, from
// its NLRI section when withdrawn is false and from its Withdrawn Routes field
// when it is true (RFC 4271 Section 4.3).
func updatePrefixes(t *testing.T, body []byte, withdrawn bool) []netip.Prefix {
	t.Helper()
	require.GreaterOrEqual(t, len(body), 4, "an UPDATE body carries both length fields")
	withdrawnLen := int(binary.BigEndian.Uint16(body[0:2]))
	require.LessOrEqual(t, 2+withdrawnLen, len(body), "withdrawn length must fit the body")
	if withdrawn {
		return decodePrefixes(t, body[2:2+withdrawnLen])
	}
	rest := body[2+withdrawnLen:]
	require.GreaterOrEqual(t, len(rest), 2, "an UPDATE body carries a path-attribute length")
	attrLen := int(binary.BigEndian.Uint16(rest[0:2]))
	require.LessOrEqual(t, 2+attrLen, len(rest), "attribute length must fit the body")
	return decodePrefixes(t, rest[2+attrLen:])
}

// decodePrefixes reads a run of length-prefixed IPv4 NLRI (RFC 4271 Section 4.3:
// one length octet in BITS, then the minimum number of octets holding them).
func decodePrefixes(t *testing.T, section []byte) []netip.Prefix {
	t.Helper()
	var prefixes []netip.Prefix
	for off := 0; off < len(section); {
		bits := int(section[off])
		require.LessOrEqual(t, bits, 32, "an IPv4 prefix is at most 32 bits")
		octets := (bits + 7) / 8
		require.LessOrEqual(t, off+1+octets, len(section), "the prefix must fit its section")
		var addr [4]byte
		copy(addr[:], section[off+1:off+1+octets])
		prefixes = append(prefixes, netip.PrefixFrom(netip.AddrFrom4(addr), bits))
		off += 1 + octets
	}
	return prefixes
}

// framesOf answers the prefixes each UPDATE a peer received carried, one entry
// per message, so a test can assert the framing and the payload at once.
func framesOf(t *testing.T, conn *recordingConn, withdrawn bool) [][]netip.Prefix {
	t.Helper()
	var frames [][]netip.Prefix
	for _, body := range updateBodies(t, conn.written()) {
		frames = append(frames, updatePrefixes(t, body, withdrawn))
	}
	return frames
}

// TestAnnounceGroupUpdatesFalseSendsOneUpdatePerNLRI is the defect's own case.
//
// VALIDATES: `behavior { group-updates false }` on the API announce rail: a
// two-prefix batch leaves as TWO UPDATE messages, each carrying one prefix.
// PREVENTS: the API rail packing a batch the operator asked to be sent one
// prefix per message, which is what it did while PeerSettings.GroupUpdates had a
// single reader in the config-driven initial sync.
func TestAnnounceGroupUpdatesFalseSendsOneUpdatePerNLRI(t *testing.T) {
	peer, conn := newGroupUpdatesPeer(t, "10.0.0.2", "false")
	adapter := groupUpdatesReactor([]*Peer{peer}, false)

	require.NoError(t, adapter.AnnounceNLRIBatch(selector.All(), groupUpdatesBatch(), plugin.OperatorSender()))

	frames := framesOf(t, conn, false)
	require.Len(t, frames, 2, "group-updates false sends one UPDATE per NLRI")
	assert.Equal(t, []netip.Prefix{groupUpdatesPrefixes[0]}, frames[0])
	assert.Equal(t, []netip.Prefix{groupUpdatesPrefixes[1]}, frames[1])
}

// TestAnnounceGroupUpdatesAbsentSendsOneUpdate is the other polarity, and it is
// the reason the fix cannot be "always send one prefix per message".
//
// VALIDATES: a peer with no `group-updates` leaf keeps the YANG default and
// receives ONE UPDATE carrying both prefixes.
// PREVENTS: the framing change reaching a peer that never asked for it, which
// would multiply the message count of every API announce in the default
// configuration.
func TestAnnounceGroupUpdatesAbsentSendsOneUpdate(t *testing.T) {
	peer, conn := newGroupUpdatesPeer(t, "10.0.0.3", "absent")
	require.True(t, peer.Settings().GroupUpdates, "the YANG default for group-updates is true")
	adapter := groupUpdatesReactor([]*Peer{peer}, false)

	require.NoError(t, adapter.AnnounceNLRIBatch(selector.All(), groupUpdatesBatch(), plugin.OperatorSender()))

	frames := framesOf(t, conn, false)
	require.Len(t, frames, 1, "a peer that groups updates receives one UPDATE")
	assert.Equal(t, groupUpdatesPrefixes, frames[0])
}

// TestAnnounceBuildGroupSplitsOnGroupUpdates drives the half of the rail where
// several peers share ONE built UPDATE.
//
// The two peers differ in nothing but the leaf, so a group key that could not
// see it would put them in one group, build once, and send both peers the same
// single packed message -- and nothing would go red, because that message is
// well formed and correct for the peer that built it.
//
// VALIDATES: groupUpdates is a field of announceFacts, which is the build-group
// key, so a peer carrying `group-updates false` cannot share a build with a peer
// that groups.
// PREVENTS: the cross-peer update group silently overriding a per-peer leaf.
func TestAnnounceBuildGroupSplitsOnGroupUpdates(t *testing.T) {
	packed, packedConn := newGroupUpdatesPeer(t, "10.0.0.4", "absent")
	split, splitConn := newGroupUpdatesPeer(t, "10.0.0.5", "false")
	adapter := groupUpdatesReactor([]*Peer{packed, split}, true)

	require.NoError(t, adapter.AnnounceNLRIBatch(selector.All(), groupUpdatesBatch(), plugin.OperatorSender()))

	packedFrames := framesOf(t, packedConn, false)
	require.Len(t, packedFrames, 1, "the grouping peer receives one UPDATE")
	assert.Equal(t, groupUpdatesPrefixes, packedFrames[0])

	splitFrames := framesOf(t, splitConn, false)
	require.Len(t, splitFrames, 2, "the peer carrying group-updates false receives one UPDATE per NLRI")
	assert.Equal(t, []netip.Prefix{groupUpdatesPrefixes[0]}, splitFrames[0])
	assert.Equal(t, []netip.Prefix{groupUpdatesPrefixes[1]}, splitFrames[1])
}

// advertiseFirst announces the batch these tests withdraw, and answers how many
// frames each connection has received by the time it returns.
//
// A withdrawal test has to reach the state a real session is in. RFC 4271
// Section 4.3 identifies a withdrawn route "in the context of the BGP speaker -
// BGP speaker connection to which it has been previously advertised", so the
// API rail writes no withdrawal to a connection that has advertised nothing and
// names the peers instead (withdrawBatchFromPeers). Announcing is how a session
// reaches that state, and driving it through the rail rather than setting the
// flag is what keeps these tests honest about the path.
func advertiseFirst(t *testing.T, adapter *reactorAPIAdapter, conns ...*recordingConn) []int {
	t.Helper()
	require.NoError(t, adapter.AnnounceNLRIBatch(selector.All(), groupUpdatesBatch(), plugin.OperatorSender()))

	sent := make([]int, 0, len(conns))
	for _, conn := range conns {
		sent = append(sent, len(framesOf(t, conn, false)))
	}
	return sent
}

// TestWithdrawGroupUpdatesFalseSendsOneUpdatePerNLRI is the withdraw rail's
// twin of the announce case above.
//
// VALIDATES: `group-updates false` frames a withdrawal the same way it frames an
// announce: two prefixes leave as two UPDATE messages, each with one prefix in
// its Withdrawn Routes field.
// PREVENTS: a peer told one prefix per UPDATE on the way in being told all of
// them in one message on the way out.
func TestWithdrawGroupUpdatesFalseSendsOneUpdatePerNLRI(t *testing.T) {
	peer, conn := newGroupUpdatesPeer(t, "10.0.0.6", "false")
	adapter := groupUpdatesReactor([]*Peer{peer}, false)
	announced := advertiseFirst(t, adapter, conn)

	require.NoError(t, adapter.WithdrawNLRIBatch(selector.All(), groupUpdatesBatch(), plugin.OperatorSender()))

	frames := framesOf(t, conn, true)[announced[0]:]
	require.Len(t, frames, 2, "group-updates false withdraws one prefix per UPDATE")
	assert.Equal(t, []netip.Prefix{groupUpdatesPrefixes[0]}, frames[0])
	assert.Equal(t, []netip.Prefix{groupUpdatesPrefixes[1]}, frames[1])
}

// TestWithdrawBuildGroupSplitsOnGroupUpdates is the withdraw rail's twin of the
// build-group case: withdrawFactsFor reads the leaf into the group key, so the
// two peers cannot share one built withdrawal.
//
// VALIDATES: both settings of the leaf, in one fan-out, on the grouped path.
// PREVENTS: the withdraw key omitting the leaf that the announce key holds,
// which would make the two rails disagree about one peer.
func TestWithdrawBuildGroupSplitsOnGroupUpdates(t *testing.T) {
	packed, packedConn := newGroupUpdatesPeer(t, "10.0.0.7", "absent")
	split, splitConn := newGroupUpdatesPeer(t, "10.0.0.8", "false")
	adapter := groupUpdatesReactor([]*Peer{packed, split}, true)
	announced := advertiseFirst(t, adapter, packedConn, splitConn)

	require.NoError(t, adapter.WithdrawNLRIBatch(selector.All(), groupUpdatesBatch(), plugin.OperatorSender()))

	packedFrames := framesOf(t, packedConn, true)[announced[0]:]
	require.Len(t, packedFrames, 1, "the grouping peer receives one withdrawal")
	assert.Equal(t, groupUpdatesPrefixes, packedFrames[0])

	splitFrames := framesOf(t, splitConn, true)[announced[1]:]
	require.Len(t, splitFrames, 2, "the peer carrying group-updates false receives one withdrawal per NLRI")
	assert.Equal(t, []netip.Prefix{groupUpdatesPrefixes[0]}, splitFrames[0])
	assert.Equal(t, []netip.Prefix{groupUpdatesPrefixes[1]}, splitFrames[1])
}
