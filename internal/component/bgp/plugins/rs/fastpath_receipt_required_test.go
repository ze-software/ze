// Design: docs/architecture/api/process-protocol.md -- mandatory source-bound UPDATE receipt fields.
package rs

import (
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestRSFastReceiptRequiresExplicitFields drives the real envelope dispatcher.
// Every identity and decision member is mandatory, even when its value is zero
// or false. Omitting or nulling it must not invent a receipt that re-forwards an
// already-served target. The explicit-zero/false control still forwards its
// actual announcement and records it for DOWN; refusing every envelope fails.
func TestRSFastReceiptRequiresExplicitFields(t *testing.T) {
	type receiptInput struct {
		name  string
		field string
		null  bool
	}
	cases := []receiptInput{{name: "explicit-zero-false"}}
	for _, field := range []string{"peer", "message-id", "source-policy-epoch", "reactor-forwarded"} {
		cases = append(cases,
			receiptInput{name: field + "-missing", field: field},
			receiptInput{name: field + "-null", field: field, null: true})
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			rs := newTestRouteServer(t)
			rs.workers.Stop()
			items := make(chan workItem, 1)
			rs.workers = newWorkerPool(func(_ workerKey, item workItem) { items <- item }, poolConfig{chanSize: 1})
			const source = "10.0.0.1"
			const target = "10.0.0.2"
			rs.peers[source] = &PeerState{Address: source, Up: true, StateSeen: true}
			rs.peers[target] = &PeerState{Address: target, Up: true, StateSeen: true}
			message := fastPathServedAnnounce(t)
			message.Type = msgtype.TypeUPDATE
			message.Direction = rpc.DirectionReceived
			message.MessageID = 0
			message.SourcePolicyEpoch = 0
			message.ReactorForwarded = false
			rs.dispatchText(receiptEnvelope(t, message, source, func(metadata map[string]any) {
				if test.field == "" {
					return
				}
				if test.null {
					metadata[test.field] = nil
					return
				}
				delete(metadata, test.field)
			}))
			rs.workers.Stop()
			var item workItem
			select {
			case item = <-items:
				if test.field != "" {
					t.Fatal("incomplete receipt reached the forwarding worker")
				}
			default:
				if test.field == "" {
					t.Fatal("explicit zero identity and false decision lost their valid announcement")
				}
				return
			}
			var forwarded []string
			rs.forwardCachedHook = func(ids []uint64, targets []string) {
				if !slices.Equal(ids, []uint64{0}) {
					t.Errorf("forward IDs = %v, want explicit zero", ids)
				}
				forwarded = append(forwarded, targets...)
			}
			key := workerKey{sourcePeer: source}
			rs.processForward(key, item)
			rs.flushWorkerBatch(key)
			if !slices.Equal(forwarded, []string{target}) {
				t.Fatalf("explicit false targets = %v, want its unserved target", forwarded)
			}
			if len(rs.withdrawals[source]) != 1 {
				t.Fatal("explicit-zero receipt lost the source's DOWN inventory")
			}
		})
	}
}
