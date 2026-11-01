// Design: docs/architecture/api/process-protocol.md -- the typed filter-update slot.
package rpc

import (
	"bytes"
	"strings"
	"sync"
	"testing"
)

// TestBridgeFilterSlotRawRewriteIsOwned proves a whole-body rewrite answered
// through the typed slot is the engine's own copy: the engine keeps it past
// the call (it replaces the received body in the cache and on the wire), so a
// rewrite that aliases a buffer the plugin reuses, or the engine's borrowed
// received body, must not change after the call returns. Method: two
// evaluators, one answering a slice of a scratch buffer it rewrites on its
// next call, one answering a slice of route.Raw(); after each call the test
// poisons the source buffer and checks the answered rewrite still holds the
// bytes the evaluator returned. Concurrent callers reusing their own bodies
// run under -race. A plugin that writes a slice after it answered breaks the
// FilterSlotHandler contract, and no engine copy can repair that, so the
// scratch evaluator reuses its buffer only on its next call.
func TestBridgeFilterSlotRawRewriteIsOwned(t *testing.T) {
	t.Run("plugin reuses its scratch", func(t *testing.T) {
		b := NewDirectBridge()
		var mu sync.Mutex
		scratch := make([]byte, 8)
		b.SetFilterSlot(func(route *FilterRoute) (*FilterUpdateOutput, error) {
			mu.Lock()
			defer mu.Unlock()
			copy(scratch, []byte{0, 0, 0, 4, 0x40, 0x01, 0x01, 0x00})
			return &FilterUpdateOutput{Action: FilterModify, Raw: scratch[:8]}, nil
		})
		b.SetReady()

		header := FilterUpdateInput{Filter: "f"}
		route := ViewFilterRoute(&header, &slotView{})
		out, err := b.CallFilterSlot(&route)
		if err != nil {
			t.Fatal(err)
		}
		want := []byte{0, 0, 0, 4, 0x40, 0x01, 0x01, 0x00}
		mu.Lock()
		for i := range scratch {
			scratch[i] = 0xEE
		}
		mu.Unlock()
		if !bytes.Equal(out.Raw, want) {
			t.Fatalf("the rewrite changed after the plugin reused its scratch: % x, want % x", out.Raw, want)
		}
	})

	t.Run("plugin answers the borrowed body", func(t *testing.T) {
		b := NewDirectBridge()
		b.SetFilterSlot(func(route *FilterRoute) (*FilterUpdateOutput, error) {
			return &FilterUpdateOutput{Action: FilterModify, Raw: route.Raw()}, nil
		})
		b.SetReady()

		received := []byte{0, 0, 0, 0}
		header := FilterUpdateInput{Filter: "f"}
		route := ViewFilterRoute(&header, &slotView{raw: received})
		out, err := b.CallFilterSlot(&route)
		if err != nil {
			t.Fatal(err)
		}
		for i := range received {
			received[i] = 0xEE
		}
		if !bytes.Equal(out.Raw, []byte{0, 0, 0, 0}) {
			t.Fatalf("the rewrite aliases the engine's borrowed body: % x", out.Raw)
		}
	})

	t.Run("concurrent calls each own their rewrite", func(t *testing.T) {
		b := NewDirectBridge()
		b.SetFilterSlot(func(route *FilterRoute) (*FilterUpdateOutput, error) {
			return &FilterUpdateOutput{Action: FilterModify, Raw: route.Raw()}, nil
		})
		b.SetReady()

		var wg sync.WaitGroup
		for i := range 8 {
			wg.Go(func() {
				header := FilterUpdateInput{Filter: "f"}
				for range 200 {
					body := []byte{0, 0, 0, byte(i)}
					route := ViewFilterRoute(&header, &slotView{raw: body})
					out, err := b.CallFilterSlot(&route)
					if err != nil {
						t.Error(err)
						return
					}
					body[3] = 0xEE
					if !bytes.Equal(out.Raw, []byte{0, 0, 0, byte(i)}) {
						t.Errorf("call %d answered % x after its body was reused", i, out.Raw)
						return
					}
				}
			})
		}
		wg.Wait()
	})

	t.Run("a rewrite wider than the forked transport is refused", func(t *testing.T) {
		b := NewDirectBridge()
		wide := make([]byte, MaxMessageSize+1)
		b.SetFilterSlot(func(*FilterRoute) (*FilterUpdateOutput, error) {
			return &FilterUpdateOutput{Action: FilterModify, Raw: wide}, nil
		})
		b.SetReady()
		header := FilterUpdateInput{Filter: "f"}
		route := ViewFilterRoute(&header, &slotView{})
		_, err := b.CallFilterSlot(&route)
		if err == nil || !strings.Contains(err.Error(), "rewrite") {
			t.Fatalf("a rewrite of %d octets answered %v, want a refusal", len(wide), err)
		}
	})
}
