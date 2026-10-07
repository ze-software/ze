package reactor

import (
	"encoding/binary"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/source"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The RFC 7911 keying rule bounds identifier state according to source framing.
// The companion rfc7911_forward_path_id_churn_test.go drives one path per UPDATE
// across many cycles. Unknown-withdrawal nonallocation is covered on the actual
// writer by TestRSWithdrawalOwnershipUnknownPathIDsDoNotAllocate.

const (
	// probeSourceFramed and probeSourceUnframed are this file's two ingress
	// peers. The table is a package global shared with every other test in this
	// binary, so each count below is read for one source id no other test uses.
	probeSourceFramed    source.SourceID = 4301
	probeSourceUnframed  source.SourceID = 4302
	probeSourceReencoded source.SourceID = 4303
)

// fwdPathIDTableSize counts every identifier the table holds, under both keys.
// A count that read bySource alone would report zero for exactly the population
// that can grow, which is the source that frames Path Identifiers.
func fwdPathIDTableSize() (entries, used int) {
	fwdPathIDs.mu.RLock()
	defer fwdPathIDs.mu.RUnlock()
	for _, perSource := range fwdPathIDs.bySource {
		entries += len(perSource)
	}
	for _, perSource := range fwdPathIDs.byPath {
		entries += len(perSource)
	}
	return entries, len(fwdPathIDs.used)
}

// TestPathIDKeyFollowsWhatTheSourceFramed is the keying rule the bound rests on.
//
// VALIDATES: AC-2, AC-4 -- how many prefixes one received identifier covers is
// decided by what the SOURCE framed. A source that frames identifiers names a
// path by (identifier, prefix), so two prefixes under one received value leave
// under two identifiers and each is freed by its own withdraw. A source that
// frames none names a path by its prefix alone and sends every one under
// identifier 0, so one identifier covers all of them and no withdraw may free
// it.
// PREVENTS: both halves of getting the key wrong. Key the framing source on the
// received identifier alone and one entry covers every prefix sent under it, so
// no withdraw can free that entry without renumbering the prefixes still
// advertised -- the reason "release on the relayed withdraw" was wrong before
// the key changed. Key the non-framing source per prefix instead and a
// full-table client costs one entry per route to bound a population that cannot
// grow.
// RFC requirement: RFC7911-2-2 positive -- "the Path Identifier MUST be assigned
// in such a way that the BGP speaker is able to use the (Prefix, Path
// Identifier) to uniquely identify a path advertised to a neighbor". Both
// keyings satisfy it, because uniqueness is asked of the PAIR: one identifier
// shared across prefixes still names one path per prefix.
func TestPathIDKeyFollowsWhatTheSourceFramed(t *testing.T) {
	destCtx, destCtxID := registerForwardBodyTestContext(t, true, true)
	peer := forwardBodyTestPeer(destCtx, destCtxID)

	// One received identifier for every prefix, which is what a source that
	// frames none sends and what a framing source is free to send too.
	const received = 0

	// emit forwards one prefix and returns the identifier the destination reads.
	emit := func(t *testing.T, srcCtxID bgpctx.ContextID, src source.SourceID, framed bool, prefix string) uint32 {
		t.Helper()
		wire := nlri.NewINET(family.IPv4Unicast, netip.MustParsePrefix(prefix), 0).Bytes()
		if framed {
			var idBytes [4]byte
			binary.BigEndian.PutUint32(idBytes[:], received)
			wire = append(idBytes[:], wire...)
		}
		body := buildRawUpdateBody(nil, forwardBodyBaseAttrs(t, 65001), [][]byte{wire})
		update := wireu.NewWireUpdate(body, srcCtxID)
		update.SetSourceID(src)
		result, ok := buildFwdBody(update, message.MaxMsgLen, destCtxID, peer, netip.MustParseAddr("192.0.2.10"), &fwdParseCache{})
		require.True(t, ok, "the UPDATE must forward")
		defer returnReadBuffer(result.transcodeBuf)
		return forwardedPathID(t, result)
	}

	t.Run("source frames identifiers", func(t *testing.T) {
		t.Cleanup(func() { fwdPathIDs.releaseSource(probeSourceFramed) })
		before, _ := fwdPathIDTableSize()

		first := emit(t, destCtxID, probeSourceFramed, true, "10.1.0.0/24")
		second := emit(t, destCtxID, probeSourceFramed, true, "10.2.0.0/24")
		after, _ := fwdPathIDTableSize()
		t.Logf("two prefixes under received identifier %d left as %d and %d", received, first, second)

		assert.NotEqual(t, first, second,
			"one identifier covers both prefixes, so withdrawing either one renumbers the other and strands it at the destination")
		assert.Equal(t, 2, after-before,
			"the two paths must hold one entry each, which is what lets a withdraw free exactly the path it withdraws")
	})

	t.Run("source frames identifiers, re-encode rail", func(t *testing.T) {
		// Same ADD-PATH answer for IPv4, a different context. buildFwdBody takes
		// its raw same-context branch only when the two context ids are equal, so
		// a destination that also reads IPv6 identifiers puts this forward on the
		// re-encode rail (fwdReencodeNLRIs) with both sides framing IPv4. That
		// branch has its own call into the generator and no other test reaches
		// it: keying it on the source alone survives every case above.
		srcCtx := bgpctx.EncodingContextWithAddPath(true, map[family.Family]bool{family.IPv4Unicast: true})
		srcCtxID, err := bgpctx.Registry.Register(srcCtx)
		require.NoError(t, err)
		wideCtx := bgpctx.EncodingContextWithAddPath(true, map[family.Family]bool{family.IPv4Unicast: true, family.IPv6Unicast: true})
		wideCtxID, err := bgpctx.Registry.Register(wideCtx)
		require.NoError(t, err)
		require.NotEqual(t, srcCtxID, wideCtxID, "guard: the fixture must put the forward on the re-encode rail")
		require.True(t, wideCtx.AddPath(family.IPv4Unicast), "guard: the destination must still read IPv4 identifiers")

		widePeer := forwardBodyTestPeer(wideCtx, wideCtxID)
		reencode := func(prefix string) uint32 {
			t.Helper()
			var idBytes [4]byte
			binary.BigEndian.PutUint32(idBytes[:], received)
			wire := append(idBytes[:], nlri.NewINET(family.IPv4Unicast, netip.MustParsePrefix(prefix), 0).Bytes()...)
			body := buildRawUpdateBody(nil, forwardBodyBaseAttrs(t, 65001), [][]byte{wire})
			update := wireu.NewWireUpdate(body, srcCtxID)
			update.SetSourceID(probeSourceReencoded)
			result, ok := buildFwdBody(update, message.MaxMsgLen, wideCtxID, widePeer, netip.MustParseAddr("192.0.2.11"), &fwdParseCache{})
			require.True(t, ok, "the UPDATE must forward")
			defer returnReadBuffer(result.transcodeBuf)
			require.Empty(t, result.rawBodies, "guard: a raw body means the forward took the same-context rail, not the re-encode one")
			return forwardedPathID(t, result)
		}

		t.Cleanup(func() { fwdPathIDs.releaseSource(probeSourceReencoded) })
		before, _ := fwdPathIDTableSize()

		first := reencode("10.5.0.0/24")
		second := reencode("10.6.0.0/24")
		after, _ := fwdPathIDTableSize()
		t.Logf("two prefixes re-encoded under received identifier %d left as %d and %d", received, first, second)

		assert.NotEqual(t, first, second,
			"the re-encode rail keyed both prefixes on the source alone, so withdrawing either one renumbers the other")
		assert.Equal(t, 2, after-before,
			"the two paths must hold one entry each on this rail too")
	})

	t.Run("source frames none", func(t *testing.T) {
		_, srcCtxID := registerForwardBodyTestContext(t, true, false)
		t.Cleanup(func() { fwdPathIDs.releaseSource(probeSourceUnframed) })
		before, _ := fwdPathIDTableSize()

		first := emit(t, srcCtxID, probeSourceUnframed, false, "10.3.0.0/24")
		second := emit(t, srcCtxID, probeSourceUnframed, false, "10.4.0.0/24")
		after, _ := fwdPathIDTableSize()
		t.Logf("two prefixes from a source that framed nothing left as %d and %d", first, second)

		assert.Equal(t, first, second,
			"such a source paid an identifier per prefix, so a full table costs one entry per route to bound a set that cannot grow")
		assert.Equal(t, 1, after-before,
			"one entry serves this source for its whole session, and nothing but peer removal ends it")
	})
}
