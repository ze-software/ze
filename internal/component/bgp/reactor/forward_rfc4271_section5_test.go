package reactor

import (
	"bytes"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
)

// RFC 4271 Section 5: "Once a BGP peer has updated any well-known attributes,
// it MUST pass these attributes to its peers in any updates it transmits."
//
// The forward rail is where Ze updates a received route's well-known
// attributes for a destination: applyFactsNextHop (peer_forward_facts.go)
// records the NEXT_HOP rewrite a next-hop-self peer is configured for, and
// the AS_PATH prepend is recorded for an external destination. The cases here
// run one received UPDATE through the rail toward such a peer and read the
// attribute bytes that reach the forward pool for it, so they assert what the
// peer is about to transmit, not what the facts say.

// s5ForwardAttrs runs one received UPDATE through the general forward rail toward
// the given peers and returns the path-attribute bytes each destination was
// asked to write. A destination absent from the map was written nothing.
func s5ForwardAttrs(t testing.TB, payload []byte, peers ...*Peer) map[netip.Addr][]byte {
	t.Helper()

	srcCtx := bgpctx.EncodingContextForASN4(true)
	srcCtxID, err := bgpctx.Registry.Register(srcCtx)
	require.NoError(t, err)

	cache := newRecentUpdateCache(100)
	update, id := newLeakTestUpdate(t, cache, payload, srcCtxID)

	type delivery struct {
		addr  netip.Addr
		attrs []byte
	}
	delivered := make(chan delivery, 8)
	pool := newFwdPool(func(k fwdKey, items []fwdItem) {
		var attrs []byte
		for i := range items {
			for _, body := range items[i].rawBodies {
				u, unpackErr := message.UnpackUpdate(body)
				require.NoError(t, unpackErr)
				attrs = append(attrs, u.PathAttributes...)
			}
			for _, u := range items[i].updates {
				attrs = append(attrs, u.PathAttributes...)
			}
		}
		delivered <- delivery{addr: k.peerAddr.Addr(), attrs: attrs}
	}, fwdPoolConfig{chanSize: 8, idleTimeout: time.Second})
	t.Cleanup(pool.Stop)

	peerMap := make(map[netip.AddrPort]*Peer, len(peers))
	for _, p := range peers {
		key := fwdKey{peerAddr: p.Settings().PeerKey()}
		pool.registerOutgoingPool(key, 4096)
		peerMap[key.peerAddr] = p
	}

	r := &Reactor{
		attrModHandlers: attrModHandlersWithDefaults(),
		recentUpdates:   cache,
		peers:           peerMap,
		fwdPool:         pool,
	}
	adapter := &reactorAPIAdapter{r: r}
	require.NoError(t, adapter.forwardUpdateCore(update, id, peers, forwardSourceInfo{
		resolved: true,
		isIBGP:   false,
	}))

	got := make(map[netip.Addr][]byte, len(peers))
	deadline := time.After(2 * time.Second)
	for range peers {
		select {
		case d := <-delivered:
			got[d.addr] = d.attrs
		case <-deadline:
			return got
		}
	}
	return got
}

// s5NextHopSelfEBGPPeer builds an established external destination in AS 65002
// that rewrites NEXT_HOP to its own local address, 10.0.0.254.
func s5NextHopSelfEBGPPeer(t testing.TB, addr string, ctx *bgpctx.EncodingContext, ctxID bgpctx.ContextID) *Peer {
	t.Helper()
	peer := wkPeer(t, addr, 65002, ctx, ctxID)
	peer.settings.NextHopMode = NextHopSelf
	peer.refreshForwardFacts()
	require.Equal(t, nhModeSelf4, peer.forwardFacts().nhMode,
		"precondition: the destination must rewrite its next hop")
	return peer
}

// s5Attr returns the value of the first attribute of the given type code in a
// path-attribute block, or nil when the block carries none.
func s5Attr(attrs []byte, code byte) []byte {
	for off := 0; off+3 <= len(attrs); {
		flags, typ := attrs[off], attrs[off+1]
		var length, hdr int
		if flags&0x10 != 0 {
			if off+4 > len(attrs) {
				return nil
			}
			length, hdr = int(attrs[off+2])<<8|int(attrs[off+3]), 4
		} else {
			length, hdr = int(attrs[off+2]), 3
		}
		if off+hdr+length > len(attrs) {
			return nil
		}
		if typ == code {
			return attrs[off+hdr : off+hdr+length]
		}
		off += hdr + length
	}
	return nil
}

// s5SourceNextHop is the NEXT_HOP the received route carried before Ze updated it.
const s5SourceNextHop = "192.0.2.10"

// s5Payload announces 10.0.0.0/24 with ORIGIN igp, an AS_PATH of one 4-octet
// AS_SEQUENCE holding 65001, and the given NEXT_HOP. The source context is
// ASN4, so the AS number is written on 4 octets: nhPayload writes 2, which the
// prepend keeps as it found it and the assertion below would then read as a
// malformed segment of the product's making.
func s5Payload(nextHop string) []byte {
	addr := netip.MustParseAddr(nextHop).As4()
	attrs := []byte{0x40, 0x01, 0x01, 0x00} // ORIGIN igp
	attrs = append(attrs, 0x40, 0x02, 0x06, 0x02, 0x01, 0x00, 0x00, 0xfd, 0xe9)
	attrs = append(attrs, 0x40, 0x03, 0x04)
	attrs = append(attrs, addr[:]...)
	return buildUpdatePayload(attrs, nhAnnouncedPrefix)
}

// RFC requirement: RFC4271-5-8 positive — after the next-hop-self rewrite and the local-AS prepend, the UPDATE written for the external destination carries NEXT_HOP 10.0.0.254 and an AS_PATH that starts with the local AS 65000 ahead of the received 65001
func TestForwardTransmitsUpdatedWellKnownAttributes(t *testing.T) {
	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, err := bgpctx.Registry.Register(ctx)
	require.NoError(t, err)
	peer := s5NextHopSelfEBGPPeer(t, "10.0.0.2", ctx, ctxID)

	got := s5ForwardAttrs(t, s5Payload(s5SourceNextHop), peer)
	attrs, reached := got[netip.MustParseAddr("10.0.0.2")]
	require.True(t, reached, "the route reached no destination")

	self := netip.MustParseAddr("10.0.0.254").As4()
	require.Equal(t, self[:], s5Attr(attrs, 3),
		"NEXT_HOP on the wire is not the rewritten address: % x", attrs)

	// AS_SEQUENCE of two 4-octet ASNs: 65000 then 65001.
	wantPath := []byte{0x02, 0x02, 0x00, 0x00, 0xfd, 0xe8, 0x00, 0x00, 0xfd, 0xe9}
	require.Equal(t, wantPath, s5Attr(attrs, 2),
		"AS_PATH on the wire does not carry the local AS ahead of the received path: % x", attrs)
}

// RFC requirement: RFC4271-5-8 negative — the received NEXT_HOP 192.0.2.10 and the received AS_PATH of 65001 alone never reach the wire for a destination whose attributes Ze updated; the transmitted UPDATE carries no stale copy beside the updated one
func TestForwardNeverTransmitsTheSupersededWellKnownAttributes(t *testing.T) {
	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, err := bgpctx.Registry.Register(ctx)
	require.NoError(t, err)
	peer := s5NextHopSelfEBGPPeer(t, "10.0.0.3", ctx, ctxID)

	got := s5ForwardAttrs(t, s5Payload(s5SourceNextHop), peer)
	attrs, reached := got[netip.MustParseAddr("10.0.0.3")]
	require.True(t, reached, "the route reached no destination")

	stale := netip.MustParseAddr(s5SourceNextHop).As4()
	require.False(t, bytes.Equal(s5Attr(attrs, 3), stale[:]),
		"the superseded NEXT_HOP reached the wire: % x", attrs)
	require.NotContains(t, string(attrs), string(stale[:]),
		"the superseded NEXT_HOP is somewhere in the transmitted attributes: % x", attrs)

	receivedOnly := []byte{0x02, 0x01, 0x00, 0x00, 0xfd, 0xe9} // the received path alone
	require.False(t, bytes.Equal(s5Attr(attrs, 2), receivedOnly),
		"the AS_PATH reached the wire without the local AS: % x", attrs)
}
