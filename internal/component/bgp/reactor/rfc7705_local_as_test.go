// Design: docs/architecture/wire/attributes.md -- AS_PATH on the egress rail
// RFC: rfc/short/rfc4271.md
// Related: peer_forward_facts.go -- secondaryPrependAS, the producer under test
// Related: reactor_api_forward.go -- forwardUpdateCore, the rail these bytes leave by
//
// RFC 7705 Section 3.3 gives the "Local AS" mechanism two options, and they are
// two rules on two rails rather than two strengths of one knob:
//
//   - "No Prepend Inbound" is INBOUND. On a route received from the peer that
//     carries it, the local-as value "MUST NOT" be appended "when installing the
//     route or advertising that UPDATE to iBGP neighbors", and the globally
//     configured ASN "MUST still" be appended "as normal when advertising the
//     UPDATE to other local eBGP neighbors".
//   - "Replace Old AS" is OUTBOUND. Toward the peer that carries it the speaker
//     "MUST NOT append the globally configured ASN" and "MUST append only the
//     configured 'Local AS' ASN value".
//
// The tests below read the AS_PATH each destination actually receives off the
// forward rail, because that is the only place the difference exists. The
// requirement ids are RFC7705-3.3-2 and -3.3-3 (No Prepend Inbound) and
// RFC7705-3.3-4 and -3.3-5 (Replace Old AS), in the parked summary
// rfc/pending/rfc7705.md. No `RFC requirement:` tag is carried yet: the summary
// is not in rfc/short/, so a tag would be an unknown id and would fail
// `./le rfc check` (internal/le/rfc/check_core.go). Enrolment belongs to
// plan/spec-bgp-as-migration.md, and the tags land with it.
package reactor

import (
	"encoding/binary"
	"maps"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The migration topology every test in this file forwards through. The router
// has moved into localASGlobal and still answers to localASLegacy on the
// sessions that have not been renumbered yet.
const (
	localASGlobal = 65000 // the router's globally configured ASN
	localASLegacy = 65010 // the per-peer "Local AS" the old customers still peer with
	localASSource = 65002 // the AS of the customer the route is learned from
)

// The same migration with the router's own two AS numbers ABOVE the two-octet
// space. The customer's AS stays inside it, so a two-octet destination reads a
// path that is part mappable and part not, which is the only shape in which
// AS_TRANS and AS4_PATH can both be wrong independently.
const (
	localASGlobal4 = 4200000000 // the router's globally configured ASN, non-mappable
	localASLegacy4 = 4200000010 // the "Local AS" override, non-mappable
	asTransASN     = 23456      // RFC 6793 Section 4.2.2, the AS number an OLD speaker sees instead
)

// localASTopology names the three AS numbers one forward run is built from. It
// is a parameter rather than a constant set because RFC 6793 Section 4.2.2 only
// bites when an AS number does not fit two octets, and the local-as prepend is
// where the router's own AS numbers enter the path.
type localASTopology struct {
	globalAS uint32
	legacyAS uint32
	sourceAS uint32
}

// mappableTopology keeps every AS number inside the two-octet space, so AS_PATH
// carries the real values whatever width the destination negotiated.
var mappableTopology = localASTopology{
	globalAS: localASGlobal,
	legacyAS: localASLegacy,
	sourceAS: localASSource,
}

// fourOctetTopology puts the router's own two AS numbers above 65535.
var fourOctetTopology = localASTopology{
	globalAS: localASGlobal4,
	legacyAS: localASLegacy4,
	sourceAS: localASSource,
}

// localASDest is one destination peer and the AS_PATH it must receive.
type localASDest struct {
	name      string
	addr      string
	localAS   uint32 // the effective per-peer local AS (the local-as override when set)
	peerAS    uint32
	noPrepend bool
	replaceAS bool
	// dstASN2 makes this destination an OLD speaker: it negotiated no four-octet
	// AS number capability, so RFC 6793 Section 4.2.2 governs what it receives.
	dstASN2  bool
	wantPath []uint32
	// wantAS4Path is the AS4_PATH the destination must receive. Nil asserts the
	// attribute is ABSENT, which RFC 6793 Section 4.2.2 requires whenever every
	// AS number in the path is mappable.
	wantAS4Path []uint32
}

// localASFrame is what one destination received from the forward rail: the AS
// numbers of its AS_PATH, and those of its AS4_PATH when it got one.
//
// The two are held together rather than asserted separately because RFC 6793
// Section 4.2.2 makes them one statement: the same path, written twice at two
// widths, and a test that reads only AS_PATH cannot tell AS_TRANS standing in
// for a real AS number from AS_TRANS standing for nothing.
type localASFrame struct {
	asPath  []uint32
	as4Path []uint32 // nil when the destination received no AS4_PATH attribute
}

// localASSourcePayload builds the UPDATE the migrated customer sends: ORIGIN,
// an AS_PATH of exactly its own AS, a NEXT_HOP, and one prefix.
//
// The prefix is load-bearing. A body advertising nothing is relayed as a
// withdrawal, and the egress AS_PATH edit these tests read would never run
// (advertiseGate, forward_build.go).
func localASSourcePayload(sourceAS uint32) []byte {
	origin := []byte{0x40, 0x01, 0x01, 0x00}
	asPath := []byte{0x40, 0x02, 0x06, 0x02, 0x01, 0x00, 0x00, 0x00, 0x00}
	binary.BigEndian.PutUint32(asPath[5:], sourceAS)
	nextHop := []byte{0x40, 0x03, 0x04, 1, 1, 1, 1}

	attrs := make([]byte, 0, len(origin)+len(asPath)+len(nextHop))
	attrs = append(attrs, origin...)
	attrs = append(attrs, asPath...)
	attrs = append(attrs, nextHop...)
	return buildUpdatePayload(attrs, fwdTestNLRI)
}

// aspathASNs reads an AS_PATH or AS4_PATH attribute value holding one
// AS_SEQUENCE and answers its AS numbers, outermost first. octets is the width
// one AS number occupies: 2 for the AS_PATH a destination that negotiated no
// four-octet AS number capability receives, and 4 for every other case,
// AS4_PATH included (RFC 6793 Section 4.2.2).
//
// It refuses any other shape rather than returning a short answer: a segment
// count or a segment type this fixture did not produce means the rail wrote
// something these tests are not reading, and a silent partial decode would let
// that pass as an AS_PATH assertion.
func aspathASNs(t *testing.T, value []byte, octets int) []uint32 {
	t.Helper()
	require.GreaterOrEqual(t, len(value), 2, "AS_PATH must carry a segment header")
	require.Equal(t, byte(2), value[0], "AS_PATH must be one AS_SEQUENCE segment")
	count := int(value[1])
	require.Len(t, value, 2+count*octets, "AS_PATH must hold exactly one segment at the negotiated width")

	asns := make([]uint32, count)
	for i := range asns {
		off := 2 + i*octets
		if octets == 2 {
			asns[i] = uint32(binary.BigEndian.Uint16(value[off:]))
			continue
		}
		asns[i] = binary.BigEndian.Uint32(value[off:])
	}
	return asns
}

// aspathOctets answers the width one AS number occupies in the AS_PATH a
// destination receives.
func aspathOctets(dstASN2 bool) int {
	if dstASN2 {
		return 2
	}
	return 4
}

// forwardLocalAS forwards one UPDATE learned from the migrated customer to
// every destination given, and answers the AS-path family each destination
// received.
//
// The source peer carries the local-as override AND "No Prepend Inbound", which
// is the configuration RFC7705-3.3-2 and -3.3-3 speak about: what the router
// does with a route received from it. It is never a destination -- ForwardUpdate
// excludes the source peer itself.
//
// The source always speaks four-octet AS numbers, so the payload entering the
// rail carries the real AS numbers whatever the destinations negotiated. A
// destination marked dstASN2 is an OLD speaker and reads the two-octet form.
func forwardLocalAS(t *testing.T, topo localASTopology, dests []localASDest) map[string]localASFrame {
	t.Helper()

	ctx4 := bgpctx.EncodingContextForASN4(true)
	ctxID4, _ := bgpctx.Registry.Register(ctx4)
	ctx2 := bgpctx.EncodingContextForASN4(false)
	ctxID2, _ := bgpctx.Registry.Register(ctx2)

	peers := make(map[netip.AddrPort]*Peer, len(dests)+1)
	source := makeLocalASPeer(t, topo, ctx4, ctxID4, localASDest{
		addr:      forwardSourceAddr,
		localAS:   topo.legacyAS,
		peerAS:    topo.sourceAS,
		noPrepend: true,
	})
	source.remoteRouterID.Store(srcOriginatorID)
	peers[source.Settings().PeerKey()] = source

	// The width each destination reads its AS_PATH at, by address. The forward
	// pool hands back the peer key and the bytes, not the settings, so the width
	// is resolved here rather than guessed from the byte count: a two-octet
	// three-AS path and a four-octet one differ, but a decoder free to pick would
	// silently read either.
	octetsByAddr := make(map[string]int, len(dests))
	for _, dest := range dests {
		peerCtx, peerCtxID := ctx4, ctxID4
		if dest.dstASN2 {
			peerCtx, peerCtxID = ctx2, ctxID2
		}
		peer := makeLocalASPeer(t, topo, peerCtx, peerCtxID, dest)
		peers[peer.Settings().PeerKey()] = peer
		octetsByAddr[dest.addr] = aspathOctets(dest.dstASN2)
	}

	var mu sync.Mutex
	got := make(map[string]localASFrame, len(dests))
	seen := make(chan struct{}, len(dests))
	pool := newFwdPool(func(key fwdKey, items []fwdItem) {
		mu.Lock()
		for i := range items {
			for _, body := range items[i].rawBodies {
				addr := key.peerAddr.Addr().String()
				value, ok := bodyPathAttr(t, body, 2)
				if !ok {
					continue
				}
				frame := localASFrame{asPath: aspathASNs(t, value, octetsByAddr[addr])}
				// AS4_PATH is four-octet by definition (RFC 6793 Section 4.2.2), so
				// its width never depends on what this destination negotiated.
				if as4, hasAS4 := bodyPathAttr(t, body, 17); hasAS4 {
					frame.as4Path = aspathASNs(t, as4, 4)
				}
				got[addr] = frame
			}
		}
		mu.Unlock()
		for i := range items {
			if items[i].done != nil {
				items[i].done()
			}
			seen <- struct{}{}
		}
	}, fwdPoolConfig{chanSize: 16, idleTimeout: time.Second})
	t.Cleanup(pool.Stop)

	cache := newRecentUpdateCache(100)
	t.Cleanup(cache.Stop)
	cache.RegisterConsumer("test-plugin")

	wu := wireu.NewWireUpdate(localASSourcePayload(topo.sourceAS), ctxID4)
	const updateID = 7705
	wu.SetMessageID(updateID)
	cache.Add(&ReceivedUpdate{
		WireUpdate:   wu,
		SourcePeerIP: netip.MustParseAddr(forwardSourceAddr),
		ReceivedAt:   time.Now(),
	})
	cache.Activate(updateID, 1)

	r := &Reactor{
		config:          &Config{LocalAS: topo.globalAS},
		recentUpdates:   cache,
		peers:           peers,
		fwdPool:         pool,
		attrModHandlers: attrModHandlersWithDefaults(),
	}
	api := &reactorAPIAdapter{r: r}

	sel, err := selector.Parse("*")
	require.NoError(t, err)
	require.NoError(t, api.ForwardUpdate(sel, updateID, "test-plugin", plugin.OperatorSender()))

	for range dests {
		select {
		case <-seen:
		case <-time.After(5 * time.Second):
			t.Fatal("timeout waiting for every destination to receive the forwarded UPDATE")
		}
	}

	mu.Lock()
	defer mu.Unlock()
	out := make(map[string]localASFrame, len(got))
	maps.Copy(out, got)
	return out
}

// makeLocalASPeer builds one established peer of the migration topology. Its
// GlobalLocalAS is always the router's real ASN, so a peer whose localAS differs
// from it is a local-as override and a peer whose localAS equals it is an
// ordinary neighbor.
func makeLocalASPeer(t *testing.T, topo localASTopology, ctx *bgpctx.EncodingContext, ctxID bgpctx.ContextID, dest localASDest) *Peer {
	t.Helper()
	addr := netip.MustParseAddr(dest.addr)
	peer := NewPeer(&PeerSettings{
		Connection:       ConnectionBoth,
		Address:          addr,
		LocalAS:          dest.localAS,
		GlobalLocalAS:    topo.globalAS,
		PeerAS:           dest.peerAS,
		RouterID:         0x01020300 | uint32(addr.As4()[3]),
		LocalASNoPrepend: dest.noPrepend,
		LocalASReplaceAS: dest.replaceAS,
	})
	peer.state.Store(int32(PeerStateEstablished))
	peer.negotiated.Store(&NegotiatedCapabilities{
		families:        map[family.Family]bool{{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast}: true},
		ExtendedMessage: false,
	})
	peer.sendCtx.Store(ctx)
	peer.sendCtxID = ctxID
	peer.refreshForwardFacts()
	return peer
}

// TestLocalASReplaceASSendsOnlyTheLocalAS pins the OUTBOUND rail of RFC 7705
// Section 3.3 "Replace Old AS": toward a peer carrying it the globally
// configured ASN is absent and the local-as value is the only ASN prepended.
// The peer beside it carries the same local-as with no option and receives both
// ASNs, so the assertion cannot pass on a rail that prepends nothing.
//
// VALIDATES: RFC7705-3.3-4 and RFC7705-3.3-5 on the wire, against the base
// two-ASN form of the same section.
// PREVENTS: replace-as losing its meaning, and the base local-as prepend losing
// the globally configured ASN with it.
func TestLocalASReplaceASSendsOnlyTheLocalAS(t *testing.T) {
	dests := []localASDest{
		{
			name: "replace-as", addr: "10.0.0.4", localAS: localASLegacy, peerAS: 65004,
			replaceAS: true,
			wantPath:  []uint32{localASLegacy, localASSource},
		},
		{
			name: "replace-as with no-prepend beside it", addr: "10.0.0.5", localAS: localASLegacy, peerAS: 65005,
			noPrepend: true, replaceAS: true,
			wantPath: []uint32{localASLegacy, localASSource},
		},
		{
			name: "no option: the base two-ASN form", addr: "10.0.0.6", localAS: localASLegacy, peerAS: 65006,
			wantPath: []uint32{localASLegacy, localASGlobal, localASSource},
		},
	}
	paths := forwardLocalAS(t, mappableTopology, dests)
	for _, dest := range dests {
		assert.Equal(t, dest.wantPath, paths[dest.addr].asPath, dest.name)
	}
}

// TestLocalASNoPrependLeavesEveryOutboundPathAlone pins the INBOUND rail of
// RFC 7705 Section 3.3 "No Prepend Inbound". The source peer carries the option,
// and the three destinations are the three things the requirement names:
//
//   - the iBGP neighbor, which MUST NOT see the local-as value appended;
//   - another eBGP neighbor peering with the globally configured ASN, which MUST
//     still see that ASN appended as normal;
//   - the peer's own outbound path, which the option does not govern and which
//     therefore keeps the base two-ASN form.
//
// VALIDATES: RFC7705-3.3-2 and RFC7705-3.3-3.
// PREVENTS: no-prepend acting on the outbound rail, which is what made it a
// second spelling of replace-as, and an inbound rewrite appearing later.
func TestLocalASNoPrependLeavesEveryOutboundPathAlone(t *testing.T) {
	dests := []localASDest{
		{
			name: "iBGP neighbor keeps the received path", addr: "10.0.0.2",
			localAS: localASGlobal, peerAS: localASGlobal,
			wantPath: []uint32{localASSource},
		},
		{
			name: "native eBGP neighbor gets the global ASN", addr: "10.0.0.3",
			localAS: localASGlobal, peerAS: 65003,
			wantPath: []uint32{localASGlobal, localASSource},
		},
		{
			name: "no-prepend on the destination keeps the base form", addr: "10.0.0.7",
			localAS: localASLegacy, peerAS: 65007, noPrepend: true,
			wantPath: []uint32{localASLegacy, localASGlobal, localASSource},
		},
	}
	paths := forwardLocalAS(t, mappableTopology, dests)
	for _, dest := range dests {
		assert.Equal(t, dest.wantPath, paths[dest.addr].asPath, dest.name)
	}
}

// TestLocalASOptionsProduceDifferentASPaths is the test the collapse would have
// failed. One UPDATE, two peers whose configuration differs by one enum, and the
// AS_PATH they receive read off the same forward run.
//
// Until 2026-08-29 both enums cleared the same field (buildForwardFacts), so
// both peers received the replace-as path and three documented configurations
// were one behavior. Comparing the two paths against each other, rather than
// each against a constant, is what makes a shared regression impossible to miss.
//
// VALIDATES: the two options select different wire output.
// PREVENTS: any future edit that reads both flags with one polarity.
func TestLocalASOptionsProduceDifferentASPaths(t *testing.T) {
	const noPrependAddr = "10.0.0.8"
	const replaceASAddr = "10.0.0.9"
	paths := forwardLocalAS(t, mappableTopology, []localASDest{
		{
			name: "no-prepend", addr: noPrependAddr, localAS: localASLegacy, peerAS: 65008,
			noPrepend: true,
			wantPath:  []uint32{localASLegacy, localASGlobal, localASSource},
		},
		{
			name: "replace-as", addr: replaceASAddr, localAS: localASLegacy, peerAS: 65009,
			replaceAS: true,
			wantPath:  []uint32{localASLegacy, localASSource},
		},
	})

	require.NotEmpty(t, paths[noPrependAddr].asPath, "the no-prepend peer must have received the route")
	require.NotEmpty(t, paths[replaceASAddr].asPath, "the replace-as peer must have received the route")
	assert.NotEqual(t, paths[replaceASAddr].asPath, paths[noPrependAddr].asPath,
		"RFC 7705 Section 3.3 makes no-prepend inbound and replace-as outbound: one UPDATE cannot leave for both peers with the same AS_PATH")
	assert.Contains(t, paths[noPrependAddr].asPath, uint32(localASGlobal),
		"no-prepend does not govern the outbound rail, so the globally configured ASN stays")
	assert.NotContains(t, paths[replaceASAddr].asPath, uint32(localASGlobal),
		"replace-as MUST NOT append the globally configured ASN")
}

// TestLocalASFourOctetTowardTwoOctetPeer pins the local-as prepend against
// RFC 6793 Section 4.2.2, for each of the three configurations.
//
// The router's own two AS numbers are above 65535 and the customer's is not, so
// a destination that negotiated no four-octet AS number capability receives an
// AS_PATH in which the router's AS numbers are AS_TRANS and the customer's is
// itself. That destination "MUST also" receive the AS4_PATH holding the real
// values, because the path is not "composed of mappable four-octet AS numbers
// only".
//
// The pair matters more than either half. AS_TRANS in AS_PATH with no AS4_PATH
// beside it loses the router's identity for every speaker behind this one, and
// it is the shape a prepend that forgot to feed the AS4_PATH derivation
// produces. The four-octet destination beside them is the control: it reads the
// real AS numbers in AS_PATH and MUST NOT be sent an AS4_PATH at all.
//
// VALIDATES: AC-6. RFC7705-3.3-4 and -3.3-5 under RFC 6793 Section 4.2.2:
// replace-as removes the globally configured AS number from BOTH encodings, not
// from the two-octet one alone.
// PREVENTS: a two-octet peer being told the migration is one AS hop where it is
// two, and an AS4_PATH that disagrees with the AS_PATH it explains.
func TestLocalASFourOctetTowardTwoOctetPeer(t *testing.T) {
	dests := []localASDest{
		{
			name: "old speaker, no option", addr: "10.0.0.12",
			localAS: localASLegacy4, peerAS: 65012, dstASN2: true,
			wantPath:    []uint32{asTransASN, asTransASN, localASSource},
			wantAS4Path: []uint32{localASLegacy4, localASGlobal4, localASSource},
		},
		{
			name: "old speaker, no-prepend", addr: "10.0.0.13",
			localAS: localASLegacy4, peerAS: 65013, dstASN2: true, noPrepend: true,
			wantPath:    []uint32{asTransASN, asTransASN, localASSource},
			wantAS4Path: []uint32{localASLegacy4, localASGlobal4, localASSource},
		},
		{
			name: "old speaker, replace-as", addr: "10.0.0.14",
			localAS: localASLegacy4, peerAS: 65014, dstASN2: true, replaceAS: true,
			wantPath:    []uint32{asTransASN, localASSource},
			wantAS4Path: []uint32{localASLegacy4, localASSource},
		},
		{
			name: "new speaker, no option", addr: "10.0.0.15",
			localAS: localASLegacy4, peerAS: 65015,
			wantPath: []uint32{localASLegacy4, localASGlobal4, localASSource},
		},
	}
	frames := forwardLocalAS(t, fourOctetTopology, dests)
	for _, dest := range dests {
		frame := frames[dest.addr]
		require.NotEmpty(t, frame.asPath, dest.name+": must have received the route")
		assert.Equal(t, dest.wantPath, frame.asPath, dest.name+": AS_PATH")
		if dest.wantAS4Path == nil {
			assert.Nil(t, frame.as4Path, dest.name+
				": every AS number reaches this destination at its real value, so RFC 6793 Section 4.2.2 forbids AS4_PATH")
			continue
		}
		assert.Equal(t, dest.wantAS4Path, frame.as4Path, dest.name+": AS4_PATH")
		assert.Len(t, frame.as4Path, len(frame.asPath),
			dest.name+": AS_TRANS preserves the path length, so the two encodings hold the same number of AS numbers")
	}
}
