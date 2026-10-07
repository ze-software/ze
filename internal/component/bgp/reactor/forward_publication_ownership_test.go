// Design: docs/architecture/plugin/rib-storage-design.md -- publication before recovery.
// Related: peer_run.go -- the real session delivery worker supplies the receipt.
package reactor

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/netip"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/message"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
)

// TestRecoveryWaitsForFilteredReceivedPublication runs actual Peer.runOnce
// sessions over net.Pipe, including their delivery workers and registered RIB.
// B's filtered withdrawal must publish and apply before recovery can elect a
// replacement for A. A failed delivery worker must fail closed without lookup.
func TestRecoveryWaitsForFilteredReceivedPublication(t *testing.T) {
	for _, fail := range []bool{false, true} {
		name := "published"
		if fail {
			name = "worker-failure"
		}
		t.Run(name, func(t *testing.T) {
			settings := []*PeerSettings{
				lowLiveSettings("192.0.2.1", 65000, 65002),
				lowLiveSettings("192.0.2.2", 65000, 65003),
				lowLiveSettings("192.0.2.3", 65000, 65004),
			}
			for _, setting := range settings {
				setting.RSClient = true
				setting.RSFastPath = true
			}
			peers := lowLiveRouter(t, settings...)
			a, b, destination := peers[0], peers[1], peers[2]
			r := a.peer.reactor
			prefix := netip.MustParsePrefix("203.0.114.0/24")
			attrsA := firstASAttrs(4, 65002)             // RFC 6793 Section 3.
			attrsB := firstASAttrs(4, 65003)             // RFC 6793 Section 3.
			ownershipLiveSend(t, b, nil, attrsB, prefix) // RFC 4271 Section 4.3.
			ownershipLiveThrough(t, destination, prefix, attrsB)
			ownershipLiveSend(t, a, nil, attrsA, prefix) // RFC 4271 Section 4.3.
			ownershipLiveThrough(t, destination, prefix, attrsA)
			lowEventually(t, func() bool {
				return bytes.Equal(extendedRecoveryStoredAttributes(b.peer.addrString, []byte{203, 0, 114}), attrsB)
			}, "candidate retained in the actual selecting RIB")

			r.mu.RLock()
			receiver := r.messageReceiver
			r.mu.RUnlock()
			gate := &ownershipPublicationReceiver{MessageReceiver: receiver,
				source: b.peer.Settings().Address, entered: make(chan struct{}),
				release: make(chan struct{}), fail: fail}
			r.setMessageReceiver(gate)
			t.Cleanup(func() {
				gate.open()
				r.setMessageReceiver(receiver)
			})
			prior := ribevents.RecoveryProvider()
			if prior == nil {
				t.Fatal("registered selecting RIB has no recovery provider")
			}
			var lookups atomic.Int32
			provider := ribevents.PublishRecovery(func(request ribevents.RecoveryRequest) ([]ribevents.RecoveryRoute, error) {
				lookups.Add(1)
				if len(extendedRecoveryStoredAttributes(b.peer.addrString, []byte{203, 0, 114})) != 0 {
					return nil, fmt.Errorf("recovery lookup preceded actual RIB withdrawal application")
				}
				return prior.Lookup(request)
			})
			defer func() {
				provider.Close()
				restored := ribevents.PublishRecovery(prior.Lookup)
				t.Cleanup(restored.Close)
			}()

			ownershipLiveSend(t, b, []netip.Prefix{prefix}, nil) // RFC 4271 Section 4.3.
			awaitEntered(t, gate.entered)
			// The batch callback can run before a queued destination write.
			// A later marker from B fences that same writer while publication
			// is still blocked; the recovery cut must include the marker too.
			marker := netip.MustParsePrefix("203.0.120.0/24")
			ownershipLiveSend(t, b, nil, attrsB, marker)                 // RFC 4271 Section 4.3.
			view := ownershipLiveThrough(t, destination, marker, attrsB) // RFC 4271 Section 9.
			cut := b.peer.receiveCut()
			if cut.publication == nil || cut.publication.done == nil || cut.messageID == 0 {
				t.Fatal("withdrawal did not enter the real runOnce delivery worker")
			}
			if cut.messageID <= gate.messageID {
				t.Fatal("recovery publication cut omitted the later wire marker")
			}
			if cut.publication.completed.Load() >= cut.messageID {
				t.Fatal("blocked delivery was already marked published")
			}
			if !gate.forwarded.Load() {
				t.Fatal("withdrawal did not traverse native forwarding before delivery")
			}
			ownershipLiveRoute(t, view, prefix, attrsA)
			ownershipLiveWithdrawals(t, view, prefix, 0)

			batch := adjOutBatch(prefix.String(), "10.0.0.1")
			batch.RecoverySource = a.peer.Settings().Address
			batch.RecoveryCut = ^uint64(0)
			session := destination.peer.currentSession()
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			done := make(chan error, 1)
			joined := false
			t.Cleanup(func() {
				gate.open()
				cancel()
				if !joined {
					<-done
				}
			})
			go func() {
				done <- (&reactorAPIAdapter{r: r}).recoverNLRIBatch(ctx, batch,
					[]*Peer{destination.peer}, plugin.OperatorSender())
			}()
			lowEventually(t, func() bool { return cut.publication.waiting.Load() },
				"live recovery waiting on actual batch publication")
			if lookups.Load() != 0 {
				t.Fatal("RIB lookup crossed the blocked received publication")
			}
			select {
			case err := <-done:
				joined = true
				t.Fatalf("recovery completed before publication: %v", err)
			default:
			}
			gate.open()
			err := <-done
			joined = true
			cancel()
			if fail {
				if !errors.Is(err, errReceivePublicationFailed) {
					t.Fatalf("failed real delivery worker returned %v", err)
				}
				if lookups.Load() != 0 {
					t.Fatal("failed delivery authorized a RIB lookup")
				}
				lowEventually(t, func() bool {
					destination.mu.Lock()
					defer destination.mu.Unlock()
					for _, frame := range destination.frames {
						if frame[18] == byte(msgtype.TypeNOTIFICATION) && len(frame) >= message.HeaderLen+2 &&
							frame[19] == byte(message.NotifyCease) && frame[20] == message.NotifyCeaseOutOfResources {
							return true
						}
					}
					return false
				}, "failed publication sends recipient Cease Out of Resources")
				if !session.tearingDown.Load() {
					t.Fatal("failed publication did not retire the affected destination session")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if lookups.Load() == 0 {
				t.Fatal("published recovery never reached the actual selecting RIB")
			}
			lowEventually(t, func() bool {
				view = ownershipLiveRead(t, destination) // RFC 4271 Section 9.
				return view.withdrawals[prefix] == 1
			}, "recovery withdraws owner without resurrecting the removed candidate")
			ownershipLiveAbsent(t, view, prefix)
			ownershipLiveAnnouncements(t, view, prefix, attrsB, attrsA)
			ownershipLiveWithdrawals(t, view, prefix, 1)
			ownershipLiveEstablished(t, peers)
		})
	}
}

// ownershipPublicationReceiver gates only the chosen source's withdrawal batch;
// every successful delivery still enters the real event dispatcher. Its owner
// MUST open the gate before joining sessions. The worker itself is production's.
type ownershipPublicationReceiver struct {
	MessageReceiver
	source    netip.Addr
	entered   chan struct{}
	release   chan struct{}
	once      sync.Once
	fail      bool
	forwarded atomic.Bool
	messageID uint64 // Written before entered closes; read only after that receipt.
}

func (receiver *ownershipPublicationReceiver) open() {
	receiver.once.Do(func() { close(receiver.release) })
}

func (receiver *ownershipPublicationReceiver) OnMessageBatchReceived(peer *plugin.PeerInfo, messages []bgptypes.RawMessage) []int {
	if peer.Address == receiver.source {
		for i := range messages {
			if bytes.Equal(messages[i].RawBytes, []byte{0, 4, 24, 203, 0, 114, 0, 0}) {
				receiver.forwarded.Store(messages[i].ReactorForwarded)
				receiver.messageID = messages[i].MessageID
				close(receiver.entered)
				<-receiver.release
				if receiver.fail {
					panic("BUG: injected received batch publication failure")
				}
				break
			}
		}
	}
	return receiver.MessageReceiver.OnMessageBatchReceived(peer, messages)
}
