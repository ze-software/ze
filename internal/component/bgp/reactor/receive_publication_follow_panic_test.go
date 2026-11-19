// Design: docs/architecture/memory/lifetime-contracts.md -- accumulated receive ownership
// Related: reactor_notify.go -- policy withdrawal continuations
package reactor

import (
	"bufio"
	"bytes"
	"errors"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/policychain"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestReceivePublicationFollowPanicPreservesPrimaryOwnership feeds a real
// two-family UPDATE through each selected reader and the actual ingress
// withdrawal producer. The primary publication succeeds while retaining its
// real read slot and an adopted variant; the heap-backed continuation then
// panics before handoff. That later failure must preserve both primary slots,
// their bytes and the already-published primary withdrawal until final release.
func TestReceivePublicationFollowPanicPreservesPrimaryOwnership(t *testing.T) {
	bufMuxGlobalMu.Lock()
	defer bufMuxGlobalMu.Unlock()
	for _, coalesced := range []bool{false, true} {
		name := "standard"
		if coalesced {
			name = "coalesced"
		}
		t.Run(name, func(t *testing.T) {
			_, baseline := combinedBufMuxStats()
			r := New(&Config{LocalAS: 65000})
			t.Cleanup(r.recentUpdates.Stop)
			r.filterTransportSeam = &externalTransport{reads: []string{"local-preference"}, reply: rpc.FilterUpdateOutput{Action: rpc.FilterWithdraw}}
			r.orderedIngressSteps = []orderedIngressStep{{name: policyChainStepName, policyChain: true}}
			settings := NewPeerSettings(netip.MustParseAddr("192.0.2.4"), 65000, 65000, 0x01010101)
			settings.ImportPolicy = mixedPolicy(t, "", policychain.Member{Name: "f", Plugin: "probe"})
			peer := NewPeer(settings)
			peer.state.Store(int32(PeerStateEstablished))
			r.peers[settings.PeerKey()] = peer
			session := NewSession(settings)
			caps := []capability.Capability{
				&capability.ASN4{ASN: 65000},
				&capability.Multiprotocol{AFI: capability.AFIIPv6, SAFI: capability.SAFIUnicast},
				&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIMulticast},
			}
			session.negotiated = capability.Negotiate(caps, caps, capability.PeerIdentity{LocalASN: 65000, PeerASN: 65000, Internal: true})
			collapseEstablish(t, session)
			session.onMessageReceived = r.capturedMessageReceiver
			ctxID, err := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))
			if err != nil {
				t.Fatal(err)
			}
			session.recvCtxID = ctxID
			body, withdrawals := twoFamilyBody()
			var published [][]byte
			r.messageReceiver = &testDeliveryReceiver{onReceived: func(_ plugin.PeerInfo, msg bgptypes.RawMessage) {
				published = append(published, bytes.Clone(msg.WireUpdate.Payload()))
			}}
			var primaryID uint64
			var primary *ReceivedUpdate
			var receipt []byte
			var variant BufHandle
			calls := 0
			panicCause := errors.New("test: follow-up failed after primary publication")
			peer.sessionEndHooks.Store(&sessionEndHooks{beforeHandOff: func() {
				calls++
				id := peer.receiveCut().messageID
				update, found := r.recentUpdates.Get(id)
				if !found {
					panic("test: current publication did not adopt its receipt")
				}
				if calls == 2 {
					if update.poolBuf.ID != noPoolBufID {
						panic("test: follow-up did not use its owned heap body")
					}
					panic(panicCause)
				}
				primaryID, primary = id, update
				if primary.poolBuf.ID == noPoolBufID {
					panic("test: primary did not own a real read slot")
				}
				if !r.recentUpdates.Retain(primaryID) {
					panic("test: primary retain refused before publication")
				}
				receipt = bytes.Clone(primary.poolBuf.Buf)
				variant = getReadBuf(false)
				if variant.Buf == nil {
					panic("test: cannot borrow primary forward variant")
				}
				for i := range variant.Buf {
					variant.Buf[i] = 0x5a
				}
				primary.adoptFwdHandle(variant)
			}})
			t.Cleanup(func() {
				peer.sessionEndHooks.Store(nil)
				if primaryID != 0 {
					r.recentUpdates.Release(primaryID)
				}
			})
			var gotPanic any
			func() {
				defer func() { gotPanic = recover() }()
				reader := bufio.NewReader(bytes.NewReader(buildUpdateMsg(body)))
				if coalesced {
					err = session.readAndProcessCoalesced(nil, reader)
				} else {
					err = session.readAndProcessMessage(nil, reader)
				}
			}()
			if gotPanic != panicCause { //nolint:errorlint // Panic transfer must preserve the identical error value, not a wrapping error.
				t.Fatalf("read returned panic %v and error %v, want the original continuation cause", gotPanic, err)
			}
			if len(published) != 1 {
				t.Fatalf("published %d messages, want the successful primary only", len(published))
			}
			if !bytes.Equal(published[0], withdrawals[0]) {
				t.Errorf("primary publication %x, want withdrawal %x", published[0], withdrawals[0])
			}
			if _, found := r.recentUpdates.Get(primaryID); !found {
				t.Error("later continuation failure revoked the successful primary")
			}
			_, held := combinedBufMuxStats()
			if held != baseline+2*4096 {
				t.Errorf("primary receipt and variant hold %d bytes, want %d", held, baseline+2*4096)
			}
			if !bytes.Equal(primary.poolBuf.Buf, receipt) {
				t.Error("later continuation failure returned the still-retained primary slot")
			}
			if !bytes.Equal(variant.Buf, bytes.Repeat([]byte{0x5a}, 4096)) {
				t.Error("later continuation failure returned the primary's adopted variant")
			}
			r.recentUpdates.Release(primaryID)
			primaryID = 0
			if r.recentUpdates.Len() != 0 {
				t.Errorf("final primary release leaves %d cache entries", r.recentUpdates.Len())
			}
			_, used := combinedBufMuxStats()
			if used != baseline {
				t.Errorf("final primary release holds %d bytes, started with %d", used, baseline)
			}
		})
	}
}
