// Design: docs/architecture/plugin/rib-storage-design.md -- a failed receive publication ends its source session
// Related: peer_session_end.go -- failReceivePublication
// Related: peer_run_test.go -- downOrderObserver
package reactor

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/message"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
)

// publicationPanicReceiver panics on every batch, as a defective receiver
// would. The delivery worker recovers it.
type publicationPanicReceiver struct{ MessageReceiver }

func (publicationPanicReceiver) OnMessageBatchReceived(*plugin.PeerInfo, []bgptypes.RawMessage) []int {
	panic("BUG: test receiver panics on delivery")
}

// TestOwedKeyPublicationFailureTearsDownSource is the Phase 2b teardown half:
// a receive publication that fails, through a panicking receiver or a nil
// one, ends the source's session with Cease / Out of Resources, and the
// session's down still arrives. The delivery worker calls the teardown
// itself, so a teardown that waited on the worker would hang until the
// deadline (A-11). The GR-capable half lands with D1's reason producer.
//
// VALIDATES: AC-17 teardown, A-11 (no wait on deliveryDone).
// PREVENTS: a source left Established while its UPDATEs never reach the plugins.
func TestOwedKeyPublicationFailureTearsDownSource(t *testing.T) {
	// The worker's seam hands it the failing receiver; the read goroutine
	// still reads the real one, so the UPDATE reaches the worker.
	cases := []struct {
		name     string
		receiver func(MessageReceiver) MessageReceiver
	}{
		{"panicking receiver", func(prior MessageReceiver) MessageReceiver {
			return publicationPanicReceiver{MessageReceiver: prior}
		}},
		{"nil receiver", func(MessageReceiver) MessageReceiver { return nil }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			peers := lowLiveRouter(t, lowLiveSettings("10.0.0.1", 65000, 65002))
			v := peers[0]
			p := v.peer
			r := p.reactor
			observer := &downOrderObserver{downAdded: make(chan struct{}, 1)}
			r.addPeerObserver(observer)
			publication := p.receivePublication.Load()
			p.sessionEndHooks.Store(&sessionEndHooks{deliveryReceiver: tc.receiver})

			attrs := firstASAttrs(4, 65002)                                            // RFC 6793 Section 3.
			ownershipLiveSend(t, v, nil, attrs, netip.MustParsePrefix("10.20.0.0/24")) // RFC 4271 Section 4.3.

			// RFC 4486 Section 4: Cease (6) with subcode Out of Resources (8).
			lowEventually(t, func() bool { return publicationCeaseSent(v) }, "Cease / Out of Resources on the wire")
			lowEventually(t, func() bool { return p.State() != PeerStateEstablished }, "source left Established")
			lowEventually(t, func() bool { return len(observer.records()) == 1 }, "the source's down")
			if !publication.failed.Load() {
				t.Fatal("the publication that tore the session down is not marked failed")
			}
		})
	}
}

// publicationCeaseSent reports whether the remote received a NOTIFICATION
// Cease / Out of Resources.
//
// RFC 4271 Section 4.5: the NOTIFICATION body is Error code (1 octet), Error
// subcode (1 octet), Data (variable).
//
//	Byte offset: | 0..15 Marker | 16..17 Length | 18 Type | 19 Error code | 20 Error subcode |
func publicationCeaseSent(v *lowLivePeer) bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	for _, frame := range v.frames {
		if len(frame) < message.HeaderLen+2 {
			continue
		}
		if frame[18] != byte(msgtype.TypeNOTIFICATION) {
			continue
		}
		if frame[19] == byte(message.NotifyCease) && frame[20] == message.NotifyCeaseOutOfResources {
			return true
		}
	}
	return false
}
