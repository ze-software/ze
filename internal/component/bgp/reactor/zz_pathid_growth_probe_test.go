package reactor

import (
	"encoding/binary"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"

	"github.com/stretchr/testify/require"
)

// MEASUREMENT PROBE for the Review Gate BLOCKER of
// plan/spec-rfc7911-generate-own-path-id.md. It is untracked and is not a
// deliverable: the growth test the spec owes replaces it once the release point
// is decided. Deleting it needs owner approval (the test-deletion hook).
//
// VALIDATES: nothing the spec claims. It measures two facts the blocker's fix
// shape turns on: a withdraw-only UPDATE reaches the destination through the
// same-context raw relay (so the Path Identifier walk runs over withdrawn
// sections), and one ingress key (source, received identifier) carries several
// advertised prefixes.
// PREVENTS: choosing a release point on the strength of a story. If one key
// carries several prefixes, releasing that key when one prefix is withdrawn
// renumbers the prefixes still advertised under it, and those keep a Path
// Identifier the destination will never see withdrawn.

func fwdPathIDTableSize() (entries, used int) {
	fwdPathIDs.mu.RLock()
	defer fwdPathIDs.mu.RUnlock()
	for _, perSource := range fwdPathIDs.bySource {
		entries += len(perSource)
	}
	return entries, len(fwdPathIDs.used)
}

// probeWithdrawOnlyBody builds an UPDATE body whose only content is withdrawn routes,
// ADD-PATH framed, one per identifier in ids, all for the SAME prefix.
func probeWithdrawOnlyBody(ids []uint32, prefix netip.Prefix) []byte {
	var withdrawn []byte
	for _, id := range ids {
		var idBytes [4]byte
		binary.BigEndian.PutUint32(idBytes[:], id)
		withdrawn = append(withdrawn, idBytes[:]...)
		withdrawn = append(withdrawn, nlri.NewINET(family.IPv4Unicast, prefix, 0).Bytes()...)
	}
	body := make([]byte, 0, 4+len(withdrawn))
	var hdr [2]byte
	binary.BigEndian.PutUint16(hdr[:], uint16(len(withdrawn)))
	body = append(body, hdr[:]...)
	body = append(body, withdrawn...)
	body = append(body, 0, 0) // total path attribute length
	return body
}

// TestProbeWithdrawOnlyUpdateGrowsPathIDTable drives one withdraw-only UPDATE
// through reactorForwardRS, which is the function notifyMessageReceiver calls on
// the receive goroutine, and reports how much table the frame bought.
func TestProbeWithdrawOnlyUpdateGrowsPathIDTable(t *testing.T) {
	ctx, ctxID := registerForwardBodyTestContext(t, true, true)
	require.True(t, ctx.AddPath(family.IPv4Unicast))

	src := makeRSPeer(t, "10.0.0.1", 65001, ctx, ctxID)
	dst := makeRSPeer(t, "10.0.0.2", 65002, ctx, ctxID)

	const withdrawn = 200
	ids := make([]uint32, 0, withdrawn)
	for i := range withdrawn {
		ids = append(ids, uint32(1_000_000+i))
	}
	body := probeWithdrawOnlyBody(ids, netip.MustParsePrefix("10.9.0.0/24"))
	t.Logf("withdraw-only UPDATE: %d octets on the wire for %d withdrawn NLRIs", message.HeaderLen+len(body), withdrawn)

	wu := wireu.NewWireUpdate(body, ctxID)
	wu.SetSourceID(src.SourceID())
	wu.SetMessageID(4242)

	update := &ReceivedUpdate{
		WireUpdate:   wu,
		SourcePeerIP: netip.MustParseAddr("10.0.0.1"),
		ReceivedAt:   time.Now(),
	}
	cache := newRecentUpdateCache(100)
	cache.Add(update)
	cache.Activate(4242, 1)

	var mu sync.Mutex
	var dispatched int
	done := make(chan struct{}, 4)
	testPool := newFwdPool(func(_ fwdKey, items []fwdItem) {
		mu.Lock()
		dispatched += len(items)
		mu.Unlock()
		for range items {
			done <- struct{}{}
		}
	}, fwdPoolConfig{chanSize: 8, idleTimeout: time.Second})
	defer testPool.Stop()

	r := &Reactor{
		attrModHandlers: attrModHandlersWithDefaults(),
		recentUpdates:   cache,
		peers: map[netip.AddrPort]*Peer{
			src.Settings().PeerKey(): src,
			dst.Settings().PeerKey(): dst,
		},
		fwdPool: testPool,
	}

	beforeEntries, beforeUsed := fwdPathIDTableSize()
	_, delivered := reactorForwardRS(r, update, 4242, netip.MustParseAddr("10.0.0.1"), src)
	require.Equal(t, 1, delivered, "the withdraw must reach the destination client")
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for dispatch")
	}
	afterEntries, afterUsed := fwdPathIDTableSize()

	t.Logf("fwdPathIDs entries %d -> %d (+%d), used %d -> %d (+%d)",
		beforeEntries, afterEntries, afterEntries-beforeEntries,
		beforeUsed, afterUsed, afterUsed-beforeUsed)
	t.Logf("octets of wire per entry: %.1f", float64(message.HeaderLen+len(body))/float64(max(afterEntries-beforeEntries, 1)))
}

// TestProbeOneReceivedIDCoversManyPrefixes checks whether the ingress key
// (source, received identifier) can cover more than one advertised prefix. If it
// can, releasing that key when one prefix is withdrawn renumbers the others.
func TestProbeOneReceivedIDCoversManyPrefixes(t *testing.T) {
	ctx, ctxID := registerForwardBodyTestContext(t, true, true)
	peer := forwardBodyTestPeer(ctx, ctxID)

	emit := func(prefix string) uint32 {
		var idBytes [4]byte
		binary.BigEndian.PutUint32(idBytes[:], 0) // one received identifier for every prefix
		nlriBytes := append(idBytes[:], nlri.NewINET(family.IPv4Unicast, netip.MustParsePrefix(prefix), 0).Bytes()...)
		body := buildRawUpdateBody(nil, forwardBodyBaseAttrs(t, 65001), [][]byte{nlriBytes})
		wire := wireu.NewWireUpdate(body, ctxID)
		wire.SetSourceID(77)
		result, ok := buildFwdBody(wire, message.MaxMsgLen, ctxID, peer, netip.MustParseAddr("192.0.2.10"), &fwdParseCache{})
		require.True(t, ok)
		defer ReturnReadBuffer(result.transcodeBuf)
		return forwardedPathID(t, result)
	}

	first := emit("10.1.0.0/24")
	second := emit("10.2.0.0/24")
	t.Logf("two prefixes under received identifier 0 left as %d and %d", first, second)
	require.Equal(t, first, second, "one key, one emitted identifier, several prefixes")
}
