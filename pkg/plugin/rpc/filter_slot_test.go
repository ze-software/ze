// Design: docs/architecture/api/process-protocol.md -- the typed filter-update slot.
package rpc

import (
	"errors"
	"testing"
)

// slotView is a FilterView whose Fields counts its encodings.
type slotView struct {
	fields  []byte
	raw     []byte
	encoded int
}

func (v *slotView) Fields() ([]byte, error) { v.encoded++; return v.fields, nil }
func (v *slotView) Raw() []byte             { return v.raw }
func (v *slotView) Engine() any             { return v }

// TestBridgeFilterSlotNoEncoding proves AC-22's fast-path half at the bridge:
// the slot is called on the caller's goroutine with the engine's view, the
// route's fields are never encoded unless the evaluator asks, and nothing is
// queued on callbackCh. Method: an activated bridge with a slot that reads
// only the header and the engine view; the call is made with no reader on
// callbackCh, so a queued callback would block or show in the channel.
func TestBridgeFilterSlotNoEncoding(t *testing.T) {
	b := NewDirectBridge()
	var sawEngine any
	b.SetFilterSlot(func(route *FilterRoute) (*FilterUpdateOutput, error) {
		sawEngine = route.Engine()
		if route.Peer() != "192.0.2.1" || route.PeerAS() != 65001 || route.Direction() != "import" {
			return nil, errors.New("the header did not reach the slot")
		}
		return &FilterUpdateOutput{Action: FilterAccept}, nil
	})
	b.SetReady()
	if !b.HasFilterSlot() {
		t.Fatal("HasFilterSlot false after SetFilterSlot")
	}

	view := &slotView{fields: []byte(`{"med":5}`)}
	header := FilterUpdateInput{Filter: "f", Direction: "import", Peer: "192.0.2.1", PeerAS: 65001}
	route := ViewFilterRoute(&header, view)
	out, err := b.CallFilterSlot(&route)
	if err != nil {
		t.Fatal(err)
	}
	if out.Action != FilterAccept {
		t.Fatalf("action %v, want accept", out.Action)
	}
	if sawEngine != view {
		t.Fatal("the slot did not receive the engine's view")
	}
	if view.encoded != 0 {
		t.Fatalf("fields encoded %d times for a slot that never read them", view.encoded)
	}
	if n := len(b.callbackCh); n != 0 {
		t.Fatalf("%d callbacks queued for a slot call", n)
	}
}

// TestBridgeFilterSlotClosed proves AC-33 at the bridge: before activation and
// after CloseCallbacks or FailCallbacks the slot answers ErrBridgeClosed and
// the evaluator is not called. Method: count calls across the lifecycle.
func TestBridgeFilterSlotClosed(t *testing.T) {
	for _, stop := range []string{"close", "fail"} {
		t.Run(stop, func(t *testing.T) {
			b := NewDirectBridge()
			calls := 0
			b.SetFilterSlot(func(*FilterRoute) (*FilterUpdateOutput, error) {
				calls++
				return &FilterUpdateOutput{Action: FilterAccept}, nil
			})
			header := FilterUpdateInput{Filter: "f"}
			route := ViewFilterRoute(&header, &slotView{})
			if _, err := b.CallFilterSlot(&route); !errors.Is(err, ErrBridgeClosed) {
				t.Fatalf("before ready: %v, want ErrBridgeClosed", err)
			}
			b.SetReady()
			if _, err := b.CallFilterSlot(&route); err != nil {
				t.Fatalf("after ready: %v", err)
			}
			if stop == "close" {
				b.CloseCallbacks()
			} else {
				b.FailCallbacks(errors.New("loop died"))
			}
			if _, err := b.CallFilterSlot(&route); !errors.Is(err, ErrBridgeClosed) {
				t.Fatalf("after %s: %v, want ErrBridgeClosed", stop, err)
			}
			if calls != 1 {
				t.Fatalf("evaluator called %d times, want once (only while ready)", calls)
			}
			if !b.HasFilterSlot() {
				t.Fatal("HasFilterSlot cleared by shutdown: the engine would fall back to the queued callback")
			}
		})
	}
}

// TestBridgeFilterSlotPanicFailsTheRoute proves a panicking evaluator answers
// an error for its route and leaves the engine goroutine running.
func TestBridgeFilterSlotPanicFailsTheRoute(t *testing.T) {
	b := NewDirectBridge()
	b.SetFilterSlot(func(*FilterRoute) (*FilterUpdateOutput, error) { panic("evaluator defect") })
	b.SetReady()
	header := FilterUpdateInput{Filter: "f"}
	route := ViewFilterRoute(&header, &slotView{})
	if _, err := b.CallFilterSlot(&route); err == nil {
		t.Fatal("a panicking evaluator answered no error")
	}
}

// TestFilterRouteInputEncodesTheView proves the batched transport sends a
// fast-path route with its fields and raw body, and an encoded route as is.
func TestFilterRouteInputEncodesTheView(t *testing.T) {
	view := &slotView{fields: []byte(`{"med":5}`), raw: []byte{1, 2}}
	header := FilterUpdateInput{Filter: "f", Peer: "p", Update: "ignored"}
	route := ViewFilterRoute(&header, view)
	in, err := route.Input()
	if err != nil {
		t.Fatal(err)
	}
	if string(in.Fields) != `{"med":5}` || len(in.Raw) != 2 || in.Peer != "p" || in.Update != "" {
		t.Fatalf("input %+v", in)
	}
	encoded := InputFilterRoute(&FilterUpdateInput{Filter: "f", Fields: []byte(`{}`)})
	if f, _ := encoded.Fields(); string(f) != `{}` || encoded.Engine() != nil {
		t.Fatalf("encoded route fields %q engine %v", f, encoded.Engine())
	}
}
