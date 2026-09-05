// Design: forward_prefix_sid.go -- prefixSIDAllowedTo, the one site every egress
// rail asks about RFC 8669 Section 8.
// Related: reactor_api_batch.go -- sendStaleReadvertise, the SECOND entry into
// buildBatchAnnounceUpdate and the one this file covers.
// Related: forward_prefix_sid_announce_rail_test.go -- the same boundary on the
// FIRST entry, the API and grouped announce.
//
// It is a file of its own rather than a third test beside the announce rail's
// two, because that file is an RFC-tagged carrier and the commit gate reads any
// edit to one as a weakening only the owner may approve.
package reactor

import (
	"bufio"
	"encoding/hex"
	"net"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/bgp/wire"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// readvertisePeerWithPipe returns an ESTABLISHED external peer whose session
// writes into a net.Pipe, plus a channel carrying every frame the peer flushes.
//
// Session.SendUpdate flushes, and net.Pipe is unbuffered, so the flush blocks
// until something reads: the drain goroutine below is what makes the send
// complete rather than deadlock. It ends when the cleanup closes the pipe.
func readvertisePeerWithPipe(t *testing.T, addr string, propagatePrefixSID bool) (*Peer, <-chan []byte) {
	t.Helper()

	settings := NewPeerSettings(netip.MustParseAddr(addr), 65000, 65001, 0x01020304)
	settings.PropagateSRv6PrefixSID = propagatePrefixSID
	peer := NewPeer(settings)
	peer.state.Store(int32(PeerStateEstablished))
	peer.negotiated.Store(&NegotiatedCapabilities{
		families: map[family.Family]bool{family.IPv4Unicast: true},
	})

	session := NewSession(settings)
	require.NoError(t, session.fsm.Event(fsm.EventManualStart))
	require.NoError(t, session.fsm.Event(fsm.EventTCPConnectionConfirmed))
	require.NoError(t, session.fsm.Event(fsm.EventBGPOpen))
	require.NoError(t, session.fsm.Event(fsm.EventKeepaliveMsg))
	require.Equal(t, fsm.StateEstablished, session.fsm.State())

	server, client := net.Pipe()
	t.Cleanup(func() {
		server.Close() //nolint:errcheck // test cleanup
		client.Close() //nolint:errcheck // test cleanup
	})

	session.mu.Lock()
	session.conn = server
	session.bufWriter = bufio.NewWriterSize(server, 4096)
	session.mu.Unlock()

	peer.mu.Lock()
	peer.session = session
	peer.mu.Unlock()

	frames := make(chan []byte, 4)
	go func() {
		buf := make([]byte, 4096)
		for {
			n, readErr := client.Read(buf)
			if n > 0 {
				frames <- append([]byte(nil), buf[:n]...)
			}
			if readErr != nil {
				return
			}
		}
	}()

	return peer, frames
}

// readvertiseAttrs waits for one flushed frame and returns its UPDATE attribute
// block. The frame is one BGP message: a 16-octet marker, a 2-octet length and a
// 1-octet type, then the UPDATE body of RFC 4271 Section 4.3.
func readvertiseAttrs(t *testing.T, frames <-chan []byte) []byte {
	t.Helper()

	var frame []byte
	select {
	case frame = <-frames:
	case <-time.After(5 * time.Second):
		t.Fatal("the readvertise rail flushed no frame")
	}

	require.Greater(t, len(frame), message.HeaderLen, "the frame carries a body")
	body := frame[message.HeaderLen:]
	sections, err := wire.ParseUpdateSections(body)
	require.NoError(t, err)
	return sections.Attrs(body)
}

// keepStaleReadvertise accepts every destination and asks for no modification,
// which is the LLGR-capable outcome (staleKeep, reactor_api_batch.go): the stale
// route is sent unchanged. That is the only readvertise outcome that puts the
// stored attribute block on the wire, so it is the one this boundary has to hold
// on. A non-LLGR eBGP peer is sent a WITHDRAWAL instead, which carries no
// attributes at all.
func keepStaleReadvertise(_, _ filterapi.PeerFilterInfo, _ []byte, _ map[string]any, _ *filterapi.ModAccumulator) bool {
	return true
}

// VALIDATES: RFC 8669 Section 8 on the RFC 9494 STALE READVERTISE entry into the
// announce rail. sendStaleReadvertise (reactor_api_batch.go) reads the
// destination's propagate-srv6-prefix-sid leaf and hands it to
// buildBatchAnnounceUpdate, so a Prefix-SID that a route's stored block carries
// crosses an AS boundary only toward a neighbor the operator declared to be
// inside ze's SR domain.
// PREVENTS: the leaf reaching the builder on the API announce and not on the
// readvertise. Those are two distinct entries into one builder, and the
// readvertise is the one whose attribute block ze did not author: it replays what
// a peer sent before its session went stale. A constant written at the
// sendStaleReadvertise call site would leak the attribute on every LLGR
// re-advertisement toward an LLGR-capable eBGP peer, with the whole announce-rail
// suite green.
//
// RFC requirement: RFC8669-8-1 negative -- "The propagation to other ASes MUST be
// explicitly configured." An LLGR stale readvertise toward an EXTERNAL peer that
// sets no propagate-srv6-prefix-sid emits no attribute 40.
// RFC requirement: RFC8669-8-1 positive -- the same readvertise toward an external
// peer the operator DID configure carries the attribute byte for byte, so the
// removal is a decision about that one session.
func TestStaleReadvertiseKeepsPrefixSIDInsideTheSRDomain(t *testing.T) {
	// ORIGIN igp, AS_PATH [65001], then the Prefix-SID the announce rail's own
	// test declares: the block a peer sent before its session went stale, which
	// the RIB stored and this rail replays.
	packed, err := hex.DecodeString(strings.ReplaceAll(
		"400101 00 4002 06 02010000fde9 "+prefixSIDAnnounceWire, " ", ""))
	require.NoError(t, err)

	route := nlri.NewINET(family.IPv4Unicast, netip.MustParsePrefix("192.0.2.0/24"), 0)

	// readvertise drives one stale batch through the entry an LLGR
	// re-advertisement actually enters, AnnounceNLRIBatch with Stale set, and
	// returns the attribute block the destination was flushed.
	readvertise := func(propagatePrefixSID bool) []byte {
		peer, frames := readvertisePeerWithPipe(t, "10.0.0.2", propagatePrefixSID)
		adapter := &reactorAPIAdapter{r: &Reactor{
			attrModHandlers:          attrModHandlersWithDefaults(),
			config:                   &Config{LocalAS: 65000},
			peers:                    map[netip.AddrPort]*Peer{peer.Settings().PeerKey(): peer},
			readvertiseEgressFilters: []filterapi.EgressFilterFunc{keepStaleReadvertise},
		}}

		require.NoError(t, adapter.AnnounceNLRIBatch(selector.All(), bgptypes.NLRIBatch{
			Family:  family.IPv4Unicast,
			NLRIs:   []nlri.NLRI{route},
			NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr("10.0.0.1")),
			Wire:    attribute.NewAttributesWire(packed, bgpctx.APIContextID),
			Stale:   1,
		}, plugin.OperatorSender()))

		return readvertiseAttrs(t, frames)
	}

	t.Run("external-peer-with-no-leaf-is-readvertised-no-prefix-sid", func(t *testing.T) {
		attrs := readvertise(false /*leaf unset*/)

		_, _, _, found := attribute.AttrFind(attrs, attribute.AttrPrefixSID)
		assert.False(t, found,
			"RFC 8669 Section 8: \"The propagation to other ASes MUST be explicitly configured.\" "+
				"No leaf was set, so the stale readvertise must not carry attribute 40 in %x", attrs)

		// The absence is specific: the rest of the stored block is re-advertised,
		// so a withheld route fails this line rather than passing it.
		assert.Equal(t, []int{1, 2, 3}, attrCodes(t, attrs))
	})

	t.Run("external-peer-the-operator-configured-keeps-it", func(t *testing.T) {
		configured := readvertise(true /*propagate-srv6-prefix-sid*/)

		_, _, value, found := attribute.AttrFind(configured, attribute.AttrPrefixSID)
		require.True(t, found, "an explicitly configured neighbor must still receive attribute 40")
		assert.Equal(t, "01000700000000000064", hex.EncodeToString(value),
			"the Label-Index TLV is replayed unchanged, not re-encoded")
	})
}
