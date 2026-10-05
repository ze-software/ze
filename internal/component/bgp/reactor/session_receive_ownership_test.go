package reactor

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/pkg/plugin/rpc"

	"github.com/stretchr/testify/require"
)

// Ownership may leave the read goroutine during its callback. A cache consumer
// can finish and recycle the received storage before that callback returns;
// processMessage must not validate or otherwise read the UPDATE afterward.
func TestReceiveUpdateDoesNotReadTransferredStorage(t *testing.T) {
	for _, transfer := range []bool{false, true} {
		name := "borrowed"
		if transfer {
			name = "transferred"
		}
		t.Run(name, func(t *testing.T) {
			session := newValidateSession()
			t.Cleanup(session.timers.StopAll)
			caps := []capability.Capability{
				&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
			}
			session.negotiated = capability.Negotiate(caps, caps,
				capability.PeerIdentity{LocalASN: session.settings.LocalAS, PeerASN: session.settings.PeerAS})
			for _, event := range []fsm.Event{
				fsm.EventManualStart, fsm.EventTCPConnectionConfirmed, fsm.EventBGPOpen, fsm.EventKeepaliveMsg,
			} {
				require.NoError(t, session.fsm.Event(event))
			}
			require.Equal(t, fsm.StateEstablished, session.State())

			// An IPv4 MP_UNREACH is valid for this session. Reusing its storage
			// for an IPv6 MP_UNREACH must not retroactively change that verdict.
			body := []byte{0, 0, 0, 10, 0x80, 15, 7, 0, 1, 1, 24, 192, 0, 2}
			called := false
			session.onMessageReceived = func(_ netip.Addr, kind msgtype.MessageType, _ []byte,
				wu *wireu.WireUpdate, _ bgpctx.ContextID, _ rpc.MessageDirection,
				handle BufHandle, _ map[string]any, _ string, _ uint64) bool {
				called = true
				require.Equal(t, msgtype.TypeUPDATE, kind)
				require.Equal(t, byte(1), wu.Payload()[8], "the negotiated family must reach the consumer")
				if transfer {
					// The callback takes ownership. This deterministic reuse stands
					// in for cache eviction followed by another owner's pool Get.
					handle.Buf[8] = 2
				}
				return transfer
			}
			header := message.Header{Type: msgtype.TypeUPDATE, Length: uint16(message.HeaderLen + len(body))}
			err, kept := session.processMessage(&header, body, BufHandle{ID: noPoolBufID, Buf: body})
			require.True(t, called, "the valid withdrawal must reach its consumer")
			require.Equal(t, transfer, kept, "only the ownership-taking callback keeps the storage")
			require.NoError(t, err, "storage reused after publication must not change the accepted UPDATE verdict")
			require.Equal(t, fsm.StateEstablished, session.State(), "reused storage must not end the accepted session")
		})
	}
}
