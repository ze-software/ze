// Design: docs/architecture/core-design.md -- disjoint reactor and route-server forwarding receipts.
// Related: recovery_delivery_integration_test.go -- real router, plugins and recipient TCP harness.
package process_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/netip"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/textparse"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// rsFastReceiptIPCConn hides only the optional DirectBridge interface. The real
// route-server runner still owns every callback, parser and forwarding command.
type rsFastReceiptIPCConn struct{ net.Conn }

// TestRSFastReceiptRecipientExactlyOnce drives the real reactor fast path and
// route server on both transports, then counts actual recipient TCP NLRIs.
// The real initial-replay reports precede input. A source-policy prepare settles
// the source's FIFO worker, and the existing forward-pool barrier settles its
// writes before another source sends the terminal TCP fence. No timed absence or
// writer deduplication can hide an extra subject. The source receipt is observed
// on the real RIB delivery, not invented by the test.
func TestRSFastReceiptRecipientExactlyOnce(t *testing.T) {
	for _, mode := range []string{"direct", "ipc"} {
		t.Run(mode, func(t *testing.T) {
			if mode == "ipc" {
				reg := registry.Lookup("bgp-rs")
				if reg == nil {
					t.Fatal("native fixture has no registered route-server runner")
				}
				original := reg.RunEngine
				reg.RunEngine = func(conn net.Conn) int { return original(&rsFastReceiptIPCConn{Conn: conn}) }
				t.Cleanup(func() { reg.RunEngine = original })
			}
			ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
			defer cancel()
			router, server, peers := recoveryDeliveryRouter(t, ctx)
			forwarding, ok := router.ReactorLifecycleAdapter().(interface {
				FlushForwardPool(context.Context) error
			})
			if !ok {
				t.Fatal("native reactor has no forwarding completion barrier")
			}
			rs := server.ProcessManager().GetProcess("bgp-rs")
			rib := server.ProcessManager().GetProcess("bgp-rib")
			if rs == nil || rib == nil {
				t.Fatal("native fixture did not start both receiving plugins")
			}
			if rs.HasStructuredHandler() != (mode == "direct") {
				t.Fatal("route-server fixture did not select its named transport")
			}
			receipts := make(chan bgptypes.RawMessage, 1)
			gate := process.GateRecoveryDelivery(t, ctx, rib, func(delivery process.EventDelivery) bool {
				event, ok := delivery.Event.(*rpc.StructuredEvent)
				if !ok || event.PeerAddress != "127.0.0.2" || event.EventType != rpc.EventKindUpdate {
					return false
				}
				message, ok := event.RawMessage.(*bgptypes.RawMessage)
				if !ok {
					return false
				}
				receipt := *message
				receipt.FastPathSkipped = slices.Clone(message.FastPathSkipped)
				receipts <- receipt
				return false
			})
			gate.Release()
			queued := make(chan rsReceiptQueued, 1)
			rsGate := process.GateRecoveryDelivery(t, ctx, rs, func(delivery process.EventDelivery) bool {
				id, matched, err := rsReceiptSubjectID(delivery)
				if !matched && err == nil {
					return false
				}
				queued <- rsReceiptQueued{messageID: id, err: err}
				return true
			})
			rsGate.Release()
			t.Cleanup(func() {
				gate.Release()
				rsGate.Release()
				router.Stop()
				server.Stop()
				join, stop := context.WithTimeout(context.Background(), 5*time.Second)
				defer stop()
				if err := gate.Wait(join); err != nil {
					t.Error(err)
				}
				if err := rsGate.Wait(join); err != nil {
					t.Error(err)
				}
			})

			// Readiness is proved by native traffic, not EOR timing or a
			// synthetic credit. One unrelated control prefix is reused with
			// distinct attributes, so an old queued copy cannot prove the next
			// probe reached all clients. The loop has a fixed attempt bound.
			const control = byte(118)
			ready := false
			for attempt := byte(1); attempt <= 4; attempt++ {
				frame := recoveryAnnouncement(control, 2, attempt)
				recoverySend(t, peers[1].conn, frame)
				receipt := rsReceiptNext(t, ctx, receipts)
				for _, index := range []int{0, 2, 3} {
					rsReceiptControlOnWire(t, ctx, peers[index], frame)
				}
				if err := forwarding.FlushForwardPool(ctx); err != nil {
					t.Fatal("control forwarding did not settle", err)
				}
				if receipt.ReactorForwarded && len(receipt.FastPathSkipped) == 0 {
					ready = true
					break
				}
			}
			if !ready {
				t.Fatal("setup control never reached every client through the native ready fast path")
			}
			const subject = byte(119)
			recoverySend(t, peers[1].conn, recoveryAnnouncement(subject, 2, 1))
			received := rsReceiptNext(t, ctx, receipts)
			if !received.ReactorForwarded {
				t.Fatal("subject did not exercise the reactor fast path")
			}
			if len(received.FastPathSkipped) != 0 {
				t.Fatalf("all ready native clients should be served: skipped %v", received.FastPathSkipped)
			}
			// Subscription enqueue order across processes is not promised.
			// Observe the subject on RS's actual FIFO before draining it; a
			// RIB receipt alone cannot prove that admission happened.
			select {
			case observed := <-queued:
				if observed.err != nil {
					t.Fatal("route-server subject observer", observed.err)
				}
				if observed.messageID != received.MessageID {
					t.Fatalf("RS queued message %d, want native source message %d", observed.messageID, received.MessageID)
				}
			case <-ctx.Done():
				t.Fatal("subject never reached the actual route-server queue", ctx.Err())
			}
			if err := rs.DrainEventsApplied(ctx); err != nil {
				t.Fatal(err)
			}
			// Only prepare and target-zero abort run. These opaque identity
			// components label the settle transaction; its epoch and frontier
			// are the source's actual received receipt, and no policy changes.
			input := rpc.SourcePolicyInput{
				TransactionID: "rs-fast-receipt-settle", Operation: rpc.SourcePolicyPrepare,
				Source: rpc.SourcePolicyIdentity{Peer: "127.0.0.2", PeerOwner: 1,
					SessionGeneration: 1, PolicyEpoch: received.SourcePolicyEpoch,
					ReceiveFrontier: received.MessageID, TemplateGeneration: 1},
				DeadlineMS: time.Now().Add(10 * time.Second).UnixMilli(),
			}
			if err := rs.Conn().SendSourcePolicy(ctx, input); err != nil {
				t.Fatal("source forwarding did not settle", err)
			}
			input.Operation = rpc.SourcePolicyAbort
			if err := rs.Conn().SendSourcePolicy(ctx, input); err != nil {
				t.Fatal("settle transaction did not release", err)
			}
			if err := forwarding.FlushForwardPool(ctx); err != nil {
				t.Fatal("source forwarding did not reach the session writers", err)
			}
			const fence = byte(120)
			recoverySend(t, peers[2].conn, recoveryAnnouncement(fence, 3, 1))
			count := 0
			for {
				update := recoveryNextUpdate(t, ctx, peers[0])
				if bytes.Equal(update.NLRI, []byte{24, 203, 0, fence}) {
					break
				}
				if bytes.Equal(update.NLRI, []byte{24, 203, 0, subject}) {
					count++
				}
			}
			if count != 1 {
				t.Fatalf("recipient received subject %d times before the causal TCP fence, want exactly one", count)
			}
		})
	}
}

// rsReceiptNext observes the actual reactor receipt at the real RIB delivery.
func rsReceiptNext(t *testing.T, ctx context.Context, receipts <-chan bgptypes.RawMessage) bgptypes.RawMessage {
	t.Helper()
	select {
	case received := <-receipts:
		return received
	case <-ctx.Done():
		t.Fatal("source UPDATE never reached the native delivery boundary", ctx.Err())
		return bgptypes.RawMessage{}
	}
}

// rsReceiptControlOnWire accepts only the current probe's complete body. Older
// legitimate control reannouncements may still be queued and prove no readiness.
func rsReceiptControlOnWire(t *testing.T, ctx context.Context, peer *recoveryTCPPeer, frame []byte) {
	t.Helper()
	want, err := message.UnpackUpdate(frame[message.HeaderLen:])
	if err != nil {
		t.Fatal(err)
	}
	for {
		update := recoveryNextUpdate(t, ctx, peer)
		if bytes.Equal(update.PathAttributes, want.PathAttributes) && bytes.Equal(update.NLRI, want.NLRI) {
			return
		}
	}
}

type rsReceiptQueued struct {
	messageID uint64
	err       error
}

// rsReceiptSubjectID observes source identity and the subject's parsed NLRI at
// the actual RS queue. It accepts only the fixture's IPv4 announcement; nothing
// is rewritten, applied, acknowledged or forwarded by this passive observer.
func rsReceiptSubjectID(delivery process.EventDelivery) (uint64, bool, error) {
	if event, ok := delivery.Event.(*rpc.StructuredEvent); ok {
		if event.PeerAddress != "127.0.0.2" {
			return 0, false, nil
		}
		if event.EventType != rpc.EventKindUpdate {
			return 0, false, nil
		}
		if raw, ok := event.RawMessage.(*bgptypes.RawMessage); ok && raw.WireUpdate != nil {
			nlri, err := raw.WireUpdate.NLRI()
			return raw.MessageID, bytes.Equal(nlri, []byte{24, 203, 0, 119}), err
		}
		return 0, false, nil
	}
	text := delivery.Output
	if len(text) > 0 && text[0] == '{' {
		envelope, err := rpc.ParseEventEnvelope(text)
		if err != nil {
			return 0, false, err
		}
		if envelope.Namespace != "bgp" {
			return 0, false, nil
		}
		if envelope.Event != "update" {
			return 0, false, nil
		}
		if err := json.Unmarshal(envelope.Payload, &text); err != nil {
			return 0, false, err
		}
	}
	scanner := textparse.NewScanner(text)
	var header [8]string
	for i := range header {
		token, ok := scanner.Next()
		if !ok {
			return 0, false, nil
		}
		header[i] = token
	}
	if header[0] != "peer" || header[1] != "127.0.0.2" || header[6] != "update" {
		return 0, false, nil
	}
	id, err := strconv.ParseUint(header[7], 10, 64)
	if err != nil {
		return 0, false, err
	}
	for token, ok := scanner.Next(); ok; token, ok = scanner.Next() {
		if token != textparse.KWNLRI {
			continue
		}
		family, _ := scanner.Next()
		action, _ := scanner.Next()
		if family != "ipv4/unicast" || action != "add" {
			continue
		}
		prefix, _ := scanner.Next()
		if prefix == "prefix" {
			prefix, _ = scanner.Next()
		}
		parsed, err := netip.ParsePrefix(prefix)
		if err != nil {
			return 0, false, err
		}
		return id, parsed == netip.MustParsePrefix("203.0.119.0/24"), nil
	}
	return 0, false, nil
}
