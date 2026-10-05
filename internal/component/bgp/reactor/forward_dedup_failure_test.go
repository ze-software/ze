// Design: docs/architecture/bgp/fanout-dedup.md -- materialization ownership.
package reactor

import (
	"bytes"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/message"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
)

// TestFailedBodyDoesNotPublishReleasedMaterialization makes the first body's
// encoding-context lookup fail, then reuses its free outgoing-pool slots before
// forwarding to the next destination with identical edits. That destination
// must receive its own UPDATE, never the unrelated bytes now occupying a slot
// returned by the failed body. The egress hook schedules reuse deterministically;
// no goroutine or sleep is needed. The dedup-disabled control sends the same wire.
func TestFailedBodyDoesNotPublishReleasedMaterialization(t *testing.T) {
	for _, fast := range []bool{false, true} {
		for _, dedupOff := range []bool{false, true} {
			name := "general"
			if fast {
				name = "fast"
			}
			if dedupOff {
				name += "/dedup-off"
			} else {
				name += "/dedup-on"
			}
			t.Run(name, func(t *testing.T) {
				h := newFanoutHarnessWith(t, 2, 1, fanoutOpts{modify: true, groups: true, dedupOff: dedupOff})
				r := h.adapter.r
				failed, target := h.dests[0], h.dests[1]
				const missingContext bgpctx.ContextID = 65535
				if bgpctx.Registry.Get(missingContext) != nil {
					t.Fatal("fixture requires an unregistered destination encoding context")
				}
				failed.sendCtxID = missingContext
				failed.refreshForwardFacts()
				pool := r.fwdPool.outgoingPool(fwdKey{peerAddr: failed.Settings().PeerKey()})
				if pool == nil {
					t.Fatal("fixture missing the failed peer's outgoing pool")
				}
				unrelated := fanoutPayload()
				copy(unrelated[len(unrelated)-8:], []byte{24, 198, 51, 100, 24, 198, 51, 101})
				var borrowed [peerPoolSize]int
				t.Cleanup(func() {
					for _, idx := range borrowed {
						if idx > 0 {
							pool.Return(idx)
						}
					}
				})
				reuse := func(_, dest filterapi.PeerFilterInfo, _ []byte, _ map[string]any, _ *filterapi.ModAccumulator) bool {
					if dest.Address != target.Settings().Address {
						return true
					}
					// A concurrent forward to the failed peer can legally fill any
					// free slot. Borrow all available slots without assuming their order.
					for i := range borrowed {
						buf, idx := pool.Get()
						if idx == 0 {
							break
						}
						borrowed[i] = idx
						copy(buf, unrelated)
					}
					return true
				}
				r.orderedEgressSteps = orderedEgressStepsFromFuncs(reuse)
				r.egressFilters = []filterapi.EgressFilterFunc{reuse}
				if fast {
					// The existing section entry point accepts an ordered destination
					// subset; this exercises its real failure and dispatch paths.
					skipped, sent, bySection := reactorForwardRSSection(r, h.update, h.update.WireUpdate, h.dests, 1, h.update.SourcePeerIP, nil)
					if len(skipped) != 0 || len(bySection) != 0 || sent != 1 {
						t.Fatalf("sent=%d skipped=%v section-retries=%d", sent, skipped, len(bySection))
					}
				} else if err := h.forward(); err != nil {
					t.Fatal(err)
				}
				sent := h.delivered(t, 1)
				if len(sent) != 1 || sent[0].peer != target.Settings().Address {
					t.Fatalf("delivery=%v, want only the healthy destination", sent)
				}
				update, err := message.UnpackUpdate(sent[0].body)
				if err != nil {
					t.Fatal(err)
				}
				want := []byte{24, 10, 0, 0, 24, 10, 0, 1}
				if !bytes.Equal(update.NLRI, want) {
					t.Fatalf("healthy destination received NLRI %x, want %x; released materialization reused unrelated wire", update.NLRI, want)
				}
			})
		}
	}
}
