// Design: docs/architecture/core-design.md -- disjoint reactor and route-server forwarding receipts.
package rs

import (
	"bytes"
	"encoding/json"
	"net/netip"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/format"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// receiptEnvelope models the protocol-owned envelope, never route metadata.
// Its fields bind the receipt to the message the formatter actually rendered.
func receiptEnvelope(t *testing.T, message *bgptypes.RawMessage, peer string, change func(map[string]any)) string {
	t.Helper()
	payload := string(format.AppendMessage(nil, &plugin.PeerInfo{
		Address: netip.MustParseAddr(peer), PeerAS: rpc.RemoteASOf(65001),
	}, *message, bgptypes.ContentConfig{Encoding: plugin.EncodingText, Format: plugin.FormatParsed}))
	metadata := map[string]any{"peer": peer, "message-id": message.MessageID,
		"source-policy-epoch": message.SourcePolicyEpoch, "reactor-forwarded": message.ReactorForwarded,
		"fast-path-skipped": message.FastPathSkipped}
	if change != nil {
		change(metadata)
	}
	encoded, err := json.Marshal(map[string]any{"namespace": "bgp", "event": "update", "payload": payload, "metadata": metadata})
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

// TestRSFastReceiptSkippedAndReplayTargets fixes the interleaving at the worker
// boundary on both rails: A was already served, B was handed back, C established
// after this UPDATE, D's replay cut owns it, and U is still unready. Only B and C
// are forwarded; the inventory still holds the source route for a later DOWN.
// Mutating the publisher's skipped slice after callback return must not alter
// the queued receipt. Exact bodies, source context and policy epoch stay intact.
func TestRSFastReceiptSkippedAndReplayTargets(t *testing.T) {
	for _, transport := range []string{"structured", "enveloped-text"} {
		t.Run(transport, func(t *testing.T) {
			rs := newTestRouteServer(t)
			rs.workers.Stop()
			items := make(chan workItem, 1)
			rs.workers = newWorkerPool(func(_ workerKey, item workItem) { items <- item }, poolConfig{chanSize: 1})
			const source = "10.0.0.1"
			for address, state := range map[string]PeerState{
				source: {Up: true, StateSeen: true},
				"10.0.0.2": {Up: true, StateSeen: true, InitialReplay: 20},
				"10.0.0.3": {Up: true, StateSeen: true, InitialReplay: 20},
				"10.0.0.4": {Up: true, StateSeen: true, InitialReplay: 50},
				"10.0.0.5": {Up: true, StateSeen: true, ForwardFrom: 45, InitialReplay: 60},
				"10.0.0.6": {StateSeen: true},
			} {
				state.Address = address
				rs.peers[address] = &state
			}
			message := fastPathServedAnnounce(t)
			message.Type = msgtype.TypeUPDATE
			message.Direction = rpc.DirectionReceived
			message.MessageID = 40
			message.SourcePolicyEpoch = 7
			message.FastPathSkipped = []netip.AddrPort{netip.MustParseAddrPort("10.0.0.3:179"), netip.MustParseAddrPort("10.0.0.6:179")}
			body := slices.Clone(message.RawBytes)
			context := message.WireUpdate.SourceCtxID()
			if transport == "structured" {
				rs.dispatchStructured(source, message)
			} else {
				rs.dispatchText(receiptEnvelope(t, message, source, nil))
			}
			rs.workers.Stop()
			var item workItem
			select {
			case item = <-items:
			default:
				t.Fatal("received UPDATE envelope never reached the actual route-server worker")
			}
			message.FastPathSkipped[0] = netip.MustParseAddrPort("10.0.0.2:179")
			var forwarded []string
			rs.forwardCachedHook = func(ids []uint64, targets []string) {
				if !slices.Equal(ids, []uint64{40}) {
					t.Errorf("forwarded IDs = %v, want source message 40", ids)
				}
				forwarded = append(forwarded, targets...)
			}
			key := workerKey{sourcePeer: source}
			rs.processForward(key, item)
			rs.flushWorkerBatch(key)
			if !slices.Equal(forwarded, []string{"10.0.0.3", "10.0.0.4"}) {
				t.Fatalf("forwarded targets = %v, want skipped B and initial-replay C only", forwarded)
			}
			if !bytes.Equal(message.RawBytes, body) || message.WireUpdate.SourceCtxID() != context {
				t.Fatal("receipt transport changed UPDATE bytes or its source encoding context")
			}
			rs.withdrawalMu.Lock()
			defer rs.withdrawalMu.Unlock()
			inventory := rs.withdrawals[source]
			if len(inventory) != 1 {
				t.Fatalf("DOWN inventory contains %d routes, want the source announcement", len(inventory))
			}
			for _, entry := range inventory {
				if entry.sourcePolicyEpoch != 7 {
					t.Fatalf("inventory policy epoch = %d, want source epoch 7", entry.sourcePolicyEpoch)
				}
			}
		})
	}
}

// TestRSFastReceiptRejectsMismatchedIdentity proves a receipt cannot be detached
// from its actual payload and suppress another source, message or policy epoch.
// Each invalid envelope must enqueue no forwarding item, even though the body
// is a valid announcement and its route metadata claims it was reactor-served.
func TestRSFastReceiptRejectsMismatchedIdentity(t *testing.T) {
	for _, field := range []string{"peer", "message-id", "source-policy-epoch"} {
		t.Run(field, func(t *testing.T) {
			rs := newTestRouteServer(t)
			rs.workers.Stop()
			items := make(chan workItem, 1)
			rs.workers = newWorkerPool(func(_ workerKey, item workItem) { items <- item }, poolConfig{chanSize: 1})
			message := fastPathServedAnnounce(t)
			message.Type, message.Direction, message.MessageID, message.SourcePolicyEpoch = msgtype.TypeUPDATE, rpc.DirectionReceived, 40, 7
			message.Meta = map[string]any{"reactor-forwarded": true}
			rs.dispatchText(receiptEnvelope(t, message, "10.0.0.1", func(metadata map[string]any) {
				if field == "peer" {
					metadata[field] = "10.0.0.99"
					return
				}
				metadata[field] = uint64(99)
			}))
			rs.workers.Stop()
			select {
			case <-items:
				t.Fatal("mismatched receipt reached the forwarding worker")
			default:
			}
		})
	}
}
