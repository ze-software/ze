package reactor

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
)

// Build-buffer capacity guard for the batch announce rail.
//
// attrBuf comes from getBuildBuf, which returns backing[off:off+4096] out of a
// 128-slot slab (session.go), so the slice's CAP runs into the NEXT peer's buffer
// while its LEN is the slot. A build that walks past len(attrBuf) therefore does
// not panic on most slots -- it silently reads and writes the neighbor's memory,
// and the resulting attrBuf[:attrOff] is handed to sendUpdateWithSplit and
// transmitted. On the slab's last slot cap == len, so the same reslice panics and
// takes the daemon down instead.
//
// These tests live in their own file rather than beside the ordering tests: that
// file's cases carry `RFC requirement:` tags for RFC 4271 Section 5 attribute
// ordering, and this is a memory-safety concern, not an ordering one.

// TestAnnounceNLRIBatch_RejectsBatchTooLargeForBuildBuffer drives the guard from
// the API entry point with enough NLRI to overflow the 4K build slot.
//
// It asserts the FAILURE IS REPORTED, deliberately not that the emitted bytes are
// right. A byte-content assertion could not have caught the defect: on most slots
// the leaked neighbor bytes are indistinguishable from uninitialized buffer, and
// on the last slot the test would panic before asserting anything. An error
// return is the only observation that separates "rejected" from "sent something
// wrong".
//
// VALIDATES: an announce whose attributes cannot fit the build buffer is rejected
// with errAnnounceTooLarge, and no UPDATE is sent for it.
// PREVENTS: the reslice past len(attrBuf) that put another peer's buffer contents
// on the wire, and the panic on the slab's final slot.
func TestAnnounceNLRIBatch_RejectsBatchTooLargeForBuildBuffer(t *testing.T) {
	settings := &PeerSettings{
		Connection: ConnectionBoth,
		Address:    netip.MustParseAddr("10.0.0.2"),
		LocalAS:    65000,
		PeerAS:     65000, // iBGP
		RouterID:   0x01020301,
	}
	peer := NewPeer(settings)
	peer.state.Store(int32(PeerStateEstablished))
	peer.negotiated.Store(&NegotiatedCapabilities{
		families:        map[family.Family]bool{family.IPv6Unicast: true},
		ExtendedMessage: false,
	})

	r := &Reactor{
		config: &Config{LocalAS: 65000},
		peers:  map[netip.AddrPort]*Peer{settings.PeerKey(): peer},
	}
	adapter := &reactorAPIAdapter{r: r}

	// 240 IPv6 /128s at 17 wire bytes each (1 length octet + 16 address) = 4080,
	// which with the MP_REACH header, AFI/SAFI, next-hop and reserved octet
	// overflows the 4096-byte slot.
	batch := bgptypes.NLRIBatch{
		Family:  family.IPv6Unicast,
		NLRIs:   manyIPv6Host(240),
		NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr("2001:db8::1")),
	}

	err := adapter.AnnounceNLRIBatch(selector.All(), batch)
	require.Error(t, err, "an announce that cannot be encoded must not report success")
	assert.ErrorIs(t, err, errAnnounceTooLarge)
}

// TestInsertAttrOrdered_RefusesToExceedBuffer pins the guard itself at its own
// boundary: the last attribute that fits is written, and one octet more is
// refused rather than written past len.
//
// VALIDATES: insertAttrOrdered returns ok=false and leaves attrOff untouched when
// the attribute would not fit, and ok=true at exactly the fitting size.
// PREVENTS: an off-by-one in the capacity check re-opening the out-of-slot write.
func TestInsertAttrOrdered_RefusesToExceedBuffer(t *testing.T) {
	// LOCAL_PREF is 7 wire bytes (3 header + 4 value).
	const localPrefWire = 7
	lp := attribute.LocalPref(100)
	require.Equal(t, localPrefWire, attrWireLen(lp), "fixture assumes a 7-byte LOCAL_PREF")

	t.Run("exact-fit-is-accepted", func(t *testing.T) {
		buf := make([]byte, localPrefWire)
		off, ok := insertAttrOrdered(buf, 0, lp)
		require.True(t, ok, "an attribute that exactly fills the buffer must be written")
		assert.Equal(t, localPrefWire, off)
	})

	t.Run("one-octet-short-is-refused", func(t *testing.T) {
		buf := make([]byte, localPrefWire-1)
		off, ok := insertAttrOrdered(buf, 0, lp)
		assert.False(t, ok, "must refuse rather than write past len(buf)")
		assert.Equal(t, 0, off, "a refused insert must not advance the offset")
	})

	t.Run("refused-when-existing-content-leaves-too-little", func(t *testing.T) {
		buf := make([]byte, localPrefWire+3)
		off, ok := insertAttrOrdered(buf, 4, lp) // 4 used, 6 free, needs 7
		assert.False(t, ok)
		assert.Equal(t, 4, off, "a refused insert must leave the block length unchanged")
	})
}

// manyIPv6Host returns n distinct IPv6 /128 NLRIs.
func manyIPv6Host(n int) []nlri.NLRI {
	out := make([]nlri.NLRI, 0, n)
	for i := range n {
		addr := netip.AddrFrom16([16]byte{
			0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0,
			0, 0, 0, 0, 0, 0, byte(i >> 8), byte(i), //nolint:gosec // G115: bounded by n
		})
		out = append(out, nlri.NewINET(family.IPv6Unicast, netip.PrefixFrom(addr, 128), 0))
	}
	return out
}
