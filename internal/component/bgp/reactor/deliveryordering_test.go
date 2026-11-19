// Design: docs/architecture/exabgp-bridge.md -- received messages precede their terminal notification.
// Related: peer_run_test.go -- the live session and held delivery receiver.
// Related: receive_publication.go -- the accepted UPDATE publication frontier.
package reactor

import (
	"context"
	"net/netip"
	"runtime"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestReceivedNotificationFollowsBufferedUpdate preserves the wire order seen
// by observers that stop at the terminal NOTIFICATION, as the consolidated
// ExaBGP API checker does. A real net.Pipe session accepts one UPDATE while its
// delivery worker is held, then reads a following NOTIFICATION. The existing
// publication waiter coordinates release without a scheduler delay: either
// notification already overtook the UPDATE, or the reader registered its wait.
// The assertion judges the observed callback order after the worker is joined.
func TestReceivedNotificationFollowsBufferedUpdate(t *testing.T) {
	sink := notificationDiagnosticLog(t)
	c := newSessionEndCase(t, acceptingLiveSettings(t, "10.0.0.1", 65000, 65002))
	c.install(t, sessionEndHooks{})
	receiver := &deliveryOrderingReceiver{
		MessageReceiver: c.receiver,
		source:          c.p.Settings().Address,
		notified:        make(chan struct{}),
	}
	c.r.setMessageReceiver(receiver)
	t.Cleanup(func() { c.r.setMessageReceiver(c.receiver) })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ownershipLiveSend(t, c.v, nil, firstASAttrs(4, 65002), netip.MustParsePrefix("10.20.0.0/24"))
	deliveryOrderingAwait(t, ctx, c.receiver.entered, "UPDATE delivery worker entered its held receiver")
	publication := c.p.receivePublication.Load()
	if publication == nil {
		t.Fatal("live session has no received publication frontier")
	}
	if publication.failed.Load() {
		t.Fatal("UPDATE publication failed before the notification input")
	}
	if publication.accepted.Load() <= publication.completed.Load() {
		t.Fatal("the held UPDATE has no unfinished publication")
	}
	if publication.waiting.Load() {
		t.Fatal("an unrelated publication wait predates the notification input")
	}

	// RFC 4271 Sections 4.3 and 4.5: the complete UPDATE was written above;
	// this same stream now carries Cease / Administrative Shutdown (6/2).
	c.v.send(t, message.PackTo(&message.Notification{
		ErrorCode: message.NotifyCease, ErrorSubcode: message.NotifyCeaseAdminShutdown,
		Data: []byte{0},
	}, nil))

	// The old synchronous control path reaches notified while UPDATE is held.
	// A path preserving publication order registers the existing frontier wait
	// instead. Neither outcome depends on sleeping long enough for a callback.
ready:
	for !publication.waiting.Load() {
		select {
		case <-receiver.notified:
			break ready
		case <-ctx.Done():
			t.Fatal("notification neither reached its observer nor waited for the earlier UPDATE")
		default:
			runtime.Gosched()
		}
	}
	c.releaseReceiver()
	deliveryOrderingAwait(t, ctx, receiver.notified, "notification observer completed")
	deliveryOrderingAwait(t, ctx, c.drained, "session delivery worker joined")

	require.Empty(t, closeRecordsWith(closeLogRecords(t, sink), "code", codeReceivedNotificationSkipped))
	require.Empty(t, notificationDiagnosticEvents())

	want := []msgtype.MessageType{msgtype.TypeUPDATE, msgtype.TypeNOTIFICATION}
	if got := receiver.records(); !slices.Equal(got, want) {
		t.Fatalf("received observer order %v, want UPDATE then NOTIFICATION (%v)", got, want)
	}
}

// deliveryOrderingReceiver records publication after the delegated callback
// returns. The inherited receiver holds UPDATEs until the test releases it.
// Only the two injected received messages are recorded, bounding the history.
type deliveryOrderingReceiver struct {
	MessageReceiver
	source     netip.Addr
	notified   chan struct{}
	notifyOnce sync.Once
	mu         sync.Mutex
	observed   []msgtype.MessageType
}

func (receiver *deliveryOrderingReceiver) OnMessageBatchReceived(peer *plugin.PeerInfo, messages []bgptypes.RawMessage) []int {
	counts := receiver.MessageReceiver.OnMessageBatchReceived(peer, messages)
	for index := range messages {
		receiver.record(peer, &messages[index])
	}
	return counts
}

func (receiver *deliveryOrderingReceiver) OnMessageReceived(peer *plugin.PeerInfo, msg bgptypes.RawMessage) int {
	count := receiver.MessageReceiver.OnMessageReceived(peer, msg)
	receiver.record(peer, &msg)
	return count
}

func (receiver *deliveryOrderingReceiver) record(peer *plugin.PeerInfo, msg *bgptypes.RawMessage) {
	if peer.Address != receiver.source {
		return
	}
	if msg.Direction != rpc.DirectionReceived {
		return
	}
	if msg.Type != msgtype.TypeUPDATE {
		if msg.Type != msgtype.TypeNOTIFICATION {
			return
		}
	}
	receiver.mu.Lock()
	receiver.observed = append(receiver.observed, msg.Type)
	receiver.mu.Unlock()
	if msg.Type == msgtype.TypeNOTIFICATION {
		receiver.notifyOnce.Do(func() { close(receiver.notified) })
	}
}

func (receiver *deliveryOrderingReceiver) records() []msgtype.MessageType {
	receiver.mu.Lock()
	defer receiver.mu.Unlock()
	return slices.Clone(receiver.observed)
}

func deliveryOrderingAwait(t *testing.T, ctx context.Context, ready <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-ready:
	case <-ctx.Done():
		t.Fatalf("timed out waiting for %s: %v", what, ctx.Err())
	}
}

// TestReceivedNotificationWaitFailureStillCloses drives both wait failures
// through a real net.Pipe session. Teardown and the decoded notification log
// must finish while the earlier UPDATE receiver is still held. Only its
// observer event is skipped, and cleanup always releases and joins the worker.
func TestReceivedNotificationWaitFailureStillCloses(t *testing.T) {
	for _, stopped := range []bool{false, true} {
		name := "publication failure"
		if stopped {
			name = "peer stop cancels wait"
		}
		t.Run(name, func(t *testing.T) {
			sink := notificationDiagnosticLog(t)
			sessionSink := closeLogSink(t)
			c := newSessionEndCase(t, acceptingLiveSettings(t, "10.0.0.1", 65000, 65002))
			c.install(t, sessionEndHooks{})
			receiver := &deliveryOrderingReceiver{
				MessageReceiver: c.receiver,
				source:          c.p.Settings().Address,
				notified:        make(chan struct{}),
			}
			c.r.setMessageReceiver(receiver)
			// This cleanup runs before hook removal and fixture shutdown, even
			// after an early Fatal. Stop first unblocks any pending frontier wait.
			t.Cleanup(func() {
				c.p.Stop()
				c.releaseReceiver()
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				deliveryOrderingAwait(t, ctx, c.drained, "cleanup delivery worker joined")
				c.r.setMessageReceiver(c.receiver)
			})
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			ownershipLiveSend(t, c.v, nil, firstASAttrs(4, 65002), netip.MustParsePrefix("10.20.0.0/24"))
			deliveryOrderingAwait(t, ctx, c.receiver.entered, "UPDATE receiver held")
			publication := c.p.receivePublication.Load()
			require.NotNil(t, publication)
			require.False(t, publication.failed.Load())
			require.Greater(t, publication.accepted.Load(), publication.completed.Load())
			require.False(t, publication.waiting.Load())
			c.v.send(t, message.PackTo(&message.Notification{
				ErrorCode: message.NotifyCease, ErrorSubcode: message.NotifyCeaseAdminShutdown,
				Data: []byte{0},
			}, nil))
			// Existing frontier state proves the callback actually waits. A
			// synchronous notification observer is an immediate failure.
			for !publication.waiting.Load() {
				select {
				case <-receiver.notified:
					t.Fatal("NOTIFICATION observer overtook the held UPDATE")
				case <-ctx.Done():
					t.Fatal("NOTIFICATION did not register its publication wait")
				default:
					runtime.Gosched()
				}
			}
			cause := errReceivePublicationFailed
			if stopped {
				cause = context.Canceled
				stopDone := make(chan struct{})
				go func() {
					defer close(stopDone)
					c.p.Stop()
				}()
				joinOnCleanup(t, stopDone, "Peer.Stop")
				deliveryOrderingAwait(t, ctx, stopDone, "Peer.Stop returned")
			} else {
				// Force the frontier failure itself, without assuming another
				// producer already reported it or closed the session for us.
				require.True(t, publication.fail())
			}
			deliveryOrderingAwait(t, ctx, c.session.Done(), "Session.Run returned before receiver release")
			deliveryOrderingAwait(t, ctx, c.v.stopped, "remote socket closed before receiver release")
			require.Equal(t, fsm.StateIdle, c.session.State())
			require.Nil(t, c.session.Conn())
			require.False(t, c.session.timers.IsHoldTimerRunning())
			require.False(t, c.session.timers.IsKeepaliveTimerRunning())
			require.Empty(t, receiver.records(), "UPDATE remains held and NOTIFICATION observer is skipped")
			notificationDiagnosticRequire(t, sink, c.p.addrString, cause)
			received := closeRecordsWith(closeLogRecords(t, sessionSink), "msg", "notification received")
			require.Len(t, received, 1, "the normal decoded notification handler still runs")
			require.Equal(t, "Cease", received[0]["code"])
			require.Equal(t, "Administrative Shutdown", received[0]["subcode"])
			c.releaseReceiver()
			deliveryOrderingAwait(t, ctx, c.drained, "delivery worker joined after release")
			require.Equal(t, []msgtype.MessageType{msgtype.TypeUPDATE}, receiver.records())
		})
	}
}
