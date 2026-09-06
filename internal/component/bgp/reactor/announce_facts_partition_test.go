// Design: reactor_api_batch.go -- announceFacts, the struct this file guards.
// Related: rfc7705_local_as_announce_test.go -- the same harness on one field.
// Related: plan/journal/key-omits-a-fact-the-builder-uses.md -- the defect class.
//
// AnnounceNLRIBatch builds ONE UPDATE for each group of peers and sends it to
// every member of the group, so a per-peer fact the group key cannot see makes
// one peer receive another peer's bytes. Which peer wins is Go map iteration
// order, and nothing goes red: the UPDATE is well formed and the peer that
// built first is correct.
//
// announceFacts closes that by construction. It is the builder's argument set
// and the map key at once, so a fact the builder reads is a fact the key holds,
// and the compiler refuses any other arrangement. A compile-time property has
// no runtime assertion, so this file asserts the BEHAVIOR that property exists
// to produce, one case per field: two peers differing in that field alone each
// receive exactly what they receive when they are announced to alone.
//
// Each case was observed RED with its own field deleted from announceFacts,
// which is what makes the set a guard rather than a description.
package reactor

import (
	"bufio"
	"encoding/hex"
	"net/netip"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// announceFactGlobalAS is the router's own AS number, and announceFactLegacyAS
// the per-peer override a migrating peer carries (RFC 7705 Section 3.3).
const (
	announceFactGlobalAS uint32 = 65000
	announceFactLegacyAS uint32 = 65010
	announceFactPeerAS   uint32 = 65001
)

// announceFactPeer is one destination of an announce. Every field a case does
// not name is equal on both of its peers, so the case isolates one fact.
type announceFactPeer struct {
	addr string
	// peerAS is the neighbor's AS number. It reaches no built byte: the builder
	// is given isIBGP, never the AS itself. Only the iBGP case sets it.
	peerAS uint32
	// localAS is the per-peer local-as override, or zero for a peer that has
	// none and speaks under the router's own AS number.
	localAS   uint32
	replaceAS bool
	rsClient  bool
	// srDomain is propagate-srv6-prefix-sid: the operator declaring this peer to
	// be inside ze's SR domain (RFC 8669 Section 8).
	srDomain bool
	// localAddr is the address a next-hop-self announce resolves to.
	localAddr string
	// oldSpeaker is a peer that advertised no four-octet AS capability, so
	// RFC 6793 Section 4.2.2 puts two-octet AS numbers on this session.
	oldSpeaker bool
	addPath    bool
	extended   bool
}

// announceFactCase is one field of announceFacts, and the two peers that differ
// in it alone.
type announceFactCase struct {
	field string
	// why states what the field changes on the wire. It is the message the
	// vacuity guard prints, because a case whose two peers receive identical
	// bytes when announced to alone proves nothing about grouping.
	why   string
	batch func() bgptypes.NLRIBatch
	left  announceFactPeer
	right announceFactPeer
}

// announceFactCases carries one case per announceFacts field. A field added to
// that struct with no case here is a per-peer decision nothing proves the
// grouping respects.
func announceFactCases() []announceFactCase {
	return []announceFactCase{
		{
			field: "nextHop",
			why:   "a next-hop-self announce writes each peer's own local address into NEXT_HOP",
			batch: announceFactSelfBatch,
			left:  announceFactPeer{addr: "10.0.0.2", peerAS: announceFactPeerAS, localAddr: "192.0.2.1"},
			right: announceFactPeer{addr: "10.0.0.3", peerAS: announceFactPeerAS, localAddr: "192.0.2.2"},
		},
		{
			field: "isIBGP",
			why:   "RFC 4271 Section 5.1.5 owes an internal peer a LOCAL_PREF, and Section 5.1.2 forbids it the AS_PATH prepend an external peer gets",
			batch: announceFactBatch,
			left:  announceFactPeer{addr: "10.0.0.2", peerAS: announceFactGlobalAS},
			right: announceFactPeer{addr: "10.0.0.3", peerAS: announceFactPeerAS},
		},
		{
			field: "rsClient",
			why:   "RFC 7947 Section 2.2.2.1 leaves an RS-client's AS_PATH alone where an ordinary external peer is prepended",
			batch: announceFactPathBatch,
			left:  announceFactPeer{addr: "10.0.0.2", peerAS: announceFactPeerAS, rsClient: true},
			right: announceFactPeer{addr: "10.0.0.3", peerAS: announceFactPeerAS},
		},
		{
			field: "propagatePrefixSID",
			why:   "RFC 8669 Section 8 removes attribute 40 toward an external peer the operator has not placed inside the SR domain",
			batch: announceFactPrefixSIDBatch,
			left:  announceFactPeer{addr: "10.0.0.2", peerAS: announceFactPeerAS, srDomain: true},
			right: announceFactPeer{addr: "10.0.0.3", peerAS: announceFactPeerAS},
		},
		{
			field: "prepend",
			why:   "RFC 7705 Section 3.3 appends the globally configured ASN behind the Local AS unless replace-as turns it off",
			batch: announceFactBatch,
			left: announceFactPeer{
				addr: "10.0.0.2", peerAS: announceFactPeerAS,
				localAS: announceFactLegacyAS, replaceAS: true,
			},
			right: announceFactPeer{
				addr: "10.0.0.3", peerAS: announceFactPeerAS,
				localAS: announceFactLegacyAS,
			},
		},
		{
			field: "addPath",
			why:   "RFC 7911 Section 3 puts a four-octet path identifier in front of every NLRI toward a peer that negotiated ADD-PATH",
			batch: announceFactBatch,
			left:  announceFactPeer{addr: "10.0.0.2", peerAS: announceFactPeerAS, addPath: true},
			right: announceFactPeer{addr: "10.0.0.3", peerAS: announceFactPeerAS},
		},
		{
			field: "asn4",
			why:   "RFC 6793 Section 4.2.2 encodes the AS_PATH in two-octet AS numbers toward an OLD speaker and four-octet ones toward a NEW one",
			batch: announceFactBatch,
			left:  announceFactPeer{addr: "10.0.0.2", peerAS: announceFactPeerAS, oldSpeaker: true},
			right: announceFactPeer{addr: "10.0.0.3", peerAS: announceFactPeerAS},
		},
		{
			field: "extended",
			why:   "RFC 8654 raises the maximum message size to 65535 octets, so an announce that one peer receives whole is split for the other",
			batch: announceFactLargeBatch,
			left:  announceFactPeer{addr: "10.0.0.2", peerAS: announceFactPeerAS, extended: true},
			right: announceFactPeer{addr: "10.0.0.3", peerAS: announceFactPeerAS},
		},
	}
}

// TestAnnounceFactsPartitionUpdateGroups drives one originated route to two
// peers that differ in exactly one announceFacts field, with update groups
// enabled, and reads what each peer's connection received.
//
// Each case asserts the same two things. First, the two peers receive different
// bytes when each is announced to ALONE, which is the guard against a vacuous
// case: a field that reaches no byte would let every arrangement of the key
// pass. Second, announcing to both at once leaves each peer's bytes exactly as
// they were alone. The second is the whole property, because a key that cannot
// see the field puts both peers in one group and sends one of them the other's
// UPDATE.
//
// VALIDATES: announceFacts partitions the build groups on every fact the
// builder reads, and on the message size the send reads.
// PREVENTS: the four defects this shape has already produced -- an unlisted
// nextHop, addPath, propagatePrefixSID or local-as prepend collapsing two peers
// onto one build, so the peer that did not build first received bytes computed
// for a different neighbor.
func TestAnnounceFactsPartitionUpdateGroups(t *testing.T) {
	for _, c := range announceFactCases() {
		t.Run(c.field, func(t *testing.T) {
			alone := announceFactRun(t, c.batch(), c.left)[c.left.addr]
			aloneRight := announceFactRun(t, c.batch(), c.right)[c.right.addr]

			require.NotEmpty(t, alone, "the left peer must be sent an UPDATE")
			require.NotEmpty(t, aloneRight, "the right peer must be sent an UPDATE")
			require.NotEqual(t, hex.EncodeToString(alone), hex.EncodeToString(aloneRight),
				"this case proves nothing unless the field reaches the wire: "+c.why)

			together := announceFactRun(t, c.batch(), c.left, c.right)
			assert.Equal(t, hex.EncodeToString(alone), hex.EncodeToString(together[c.left.addr]),
				"grouped with a peer differing only in "+c.field+", the left peer must still receive its own bytes")
			assert.Equal(t, hex.EncodeToString(aloneRight), hex.EncodeToString(together[c.right.addr]),
				"grouped with a peer differing only in "+c.field+", the right peer must still receive its own bytes")
		})
	}
}

// announceFactRun originates one batch to the peers given, with update groups
// ENABLED, and answers the bytes each peer's connection received, by address.
func announceFactRun(t *testing.T, batch bgptypes.NLRIBatch, dests ...announceFactPeer) map[string][]byte {
	t.Helper()

	peers := make(map[netip.AddrPort]*Peer, len(dests))
	conns := make(map[string]*recordingConn, len(dests))
	for _, dest := range dests {
		peer, conn := newAnnounceFactPeer(t, dest)
		peers[peer.Settings().PeerKey()] = peer
		conns[dest.addr] = conn
	}

	adapter := &reactorAPIAdapter{r: &Reactor{
		config:          &Config{LocalAS: announceFactGlobalAS},
		peers:           peers,
		attrModHandlers: attrModHandlersWithDefaults(),
		updateGroups:    newUpdateGroupIndex(true),
	}}
	require.NoError(t, adapter.AnnounceNLRIBatch(selector.All(), batch, plugin.OperatorSender()))

	got := make(map[string][]byte, len(dests))
	for addr, conn := range conns {
		got[addr] = conn.written()
	}
	return got
}

// newAnnounceFactPeer builds one Established peer backed by a recordingConn,
// the shape newAnnounceLocalASPeer uses, with the capabilities each case needs
// negotiated on it.
func newAnnounceFactPeer(t *testing.T, dest announceFactPeer) (*Peer, *recordingConn) {
	t.Helper()

	localAS := announceFactGlobalAS
	if dest.localAS != 0 {
		localAS = dest.localAS
	}
	addr := netip.MustParseAddr(dest.addr)
	// Built through NewPeerSettings rather than as a literal, so every leaf a
	// real peer carries by default is the value a real peer carries. A literal
	// reads `group-updates` as false, which frames one UPDATE per prefix and
	// hides what the message size decides.
	settings := NewPeerSettings(addr, localAS, dest.peerAS, 0x01020300|uint32(addr.As4()[3]))
	// GlobalLocalAS is always the router's real AS number, so a peer whose
	// LocalAS differs from it carries a local-as override.
	settings.GlobalLocalAS = announceFactGlobalAS
	settings.LocalASReplaceAS = dest.replaceAS
	settings.RSClient = dest.rsClient
	settings.PropagateSRv6PrefixSID = dest.srDomain
	if dest.localAddr != "" {
		settings.LocalAddress = netip.MustParseAddr(dest.localAddr)
	}

	peer := NewPeer(settings)
	peer.state.Store(int32(PeerStateEstablished))

	neg := announceFactNegotiate(dest, localAS)
	peer.setEncodingContexts(neg)
	peer.negotiated.Store(NewNegotiatedCapabilities(neg))

	session := NewSession(settings)
	require.NoError(t, session.fsm.Event(fsm.EventManualStart))
	require.NoError(t, session.fsm.Event(fsm.EventTCPConnectionConfirmed))
	require.NoError(t, session.fsm.Event(fsm.EventBGPOpen))
	require.NoError(t, session.fsm.Event(fsm.EventKeepaliveMsg))
	require.Equal(t, fsm.StateEstablished, session.fsm.State())

	conn := &recordingConn{}
	session.mu.Lock()
	session.conn = conn
	session.bufWriter = bufio.NewWriterSize(conn, message.MaxMsgLen)
	// RFC 8654: the session write buffer starts at 4096 and grows to 65535 when
	// the capability is negotiated, which session_negotiate.go does on a live
	// session. Without it an extended peer cannot receive the larger message it
	// negotiated, and the case below would compare two truncations.
	session.writeBuf.Resize(neg.ExtendedMessage)
	session.mu.Unlock()

	peer.mu.Lock()
	peer.session = session
	peer.mu.Unlock()

	return peer, conn
}

// announceFactNegotiate runs a real capability negotiation, so asn4, addPath
// and extended reach the peer by the route a session gives them rather than by
// a field a test wrote.
func announceFactNegotiate(dest announceFactPeer, localAS uint32) *capability.Negotiated {
	ipv4 := &capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast}
	local := []capability.Capability{ipv4}
	remote := []capability.Capability{ipv4}

	// RFC 6793: an OLD speaker advertises no four-octet AS capability at all.
	if !dest.oldSpeaker {
		local = append(local, &capability.ASN4{ASN: localAS})
		remote = append(remote, &capability.ASN4{ASN: dest.peerAS})
	}
	if dest.addPath {
		local = append(local, &capability.AddPath{Families: []capability.AddPathFamily{
			{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast, Mode: capability.AddPathSend},
		}})
		remote = append(remote, &capability.AddPath{Families: []capability.AddPathFamily{
			{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast, Mode: capability.AddPathReceive},
		}})
	}
	if dest.extended {
		local = append(local, &capability.ExtendedMessage{})
		remote = append(remote, &capability.ExtendedMessage{})
	}
	return capability.Negotiate(local, remote, localAS, dest.peerAS)
}

// announceFactBatch is one originated route with no caller attributes, which is
// the shape every case takes unless it needs something the base must carry.
func announceFactBatch() bgptypes.NLRIBatch {
	return bgptypes.NLRIBatch{
		Family:  family.IPv4Unicast,
		NLRIs:   []nlri.NLRI{nlri.NewINET(family.IPv4Unicast, netip.MustParsePrefix("10.0.0.0/24"), 0)},
		NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr("1.1.1.1")),
	}
}

// announceFactSelfBatch resolves its next hop per peer, which is what makes two
// peers differ in the nextHop field at all.
func announceFactSelfBatch() bgptypes.NLRIBatch {
	batch := announceFactBatch()
	batch.NextHop = bgptypes.NewNextHopSelf()
	return batch
}

// announceFactPathBatch supplies an AS_PATH, because the prepend an RS-client
// is spared is only reachable when the caller brought a path of its own: a
// synthesized path asks prefixSIDAllowedTo's sibling question and never reads
// rsClient.
func announceFactPathBatch() bgptypes.NLRIBatch {
	b := attribute.NewBuilder()
	b.SetOrigin(0)
	b.SetASPath([]uint32{65100})

	batch := announceFactBatch()
	batch.Attrs = b
	return batch
}

// announceFactPrefixSIDBatch carries a Prefix-SID in the caller's block, which
// is the only way attribute 40 reaches this rail (nothing here contributes one).
// The wire form is prefixSIDAnnounceWire, declared beside the rail's own
// RFC 8669 test.
func announceFactPrefixSIDBatch() bgptypes.NLRIBatch {
	// ORIGIN igp, AS_PATH [65001], then the Prefix-SID.
	packed, err := hex.DecodeString(strings.ReplaceAll(
		"400101 00 4002 06 02010000fde9 "+prefixSIDAnnounceWire, " ", ""))
	if err != nil {
		panic("BUG: the Prefix-SID announce fixture does not decode: " + err.Error())
	}

	batch := announceFactBatch()
	batch.Wire = attribute.NewAttributesWire(packed, bgpctx.APIContextID)
	return batch
}

// announceFactLargeBatch is an announce that does not fit one 4096-octet
// message, so RFC 8654 decides how many messages the peer receives.
//
// 1020 prefixes of four octets each is 4080 octets of NLRI, which leaves room
// in the 4096-octet build buffer and still carries the message past the
// standard 4096-octet limit once the header and the attributes are counted.
func announceFactLargeBatch() bgptypes.NLRIBatch {
	const prefixCount = 1020

	nlris := make([]nlri.NLRI, 0, prefixCount)
	for i := range prefixCount {
		addr := netip.AddrFrom4([4]byte{10, byte(i>>8) + 1, byte(i), 0})
		nlris = append(nlris, nlri.NewINET(family.IPv4Unicast, netip.PrefixFrom(addr, 24), 0))
	}

	batch := announceFactBatch()
	batch.NLRIs = nlris
	return batch
}
