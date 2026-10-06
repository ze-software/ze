// Design: docs/architecture/isis/isis-5-adjacency.md -- invalid three-way input.
package adjacency

import (
	"reflect"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/isis/packet"
)

// TestRFC5303InvalidStateLeavesAdjacencyUntouched feeds every unnamed wire
// state to fresh, Initializing and Up adjacencies and compares every field.
// RFC requirement: RFC5303-3.2-7 negative -- invalid TLV 240 states are rejected without adjacency, neighbor, hold-timer or session-event mutation.
// RFC 5303 Section 3.2: "If the option is present and contains invalid Adjacency
// Three-Way State, the PDU SHALL be discarded and no further action is taken.".
func TestRFC5303InvalidStateLeavesAdjacencyUntouched(t *testing.T) {
	for _, state := range []State{StateDown, StateInitializing, StateUp} {
		for invalid := 3; invalid <= 255; invalid++ {
			adj := adjacencyInState(t, state)
			before := *adj
			in := threeWayHello(t, packet.AdjThreeWayState(invalid))
			in.HoldTime = 600
			in.SNPA = SNPA{1, 2, 3, 4, 5, 6}
			tr := ReceiveHello(adj, p2pLocal(t), in, t0.Add(time.Second))
			if !tr.Rejected || tr.SessionUp || tr.SessionDown || tr.ForwardingChanged {
				t.Errorf("%v / state %d: transition = %+v, want discard", state, invalid, tr)
			}
			if !reflect.DeepEqual(*adj, before) {
				t.Fatalf("%v / state %d mutated adjacency: before=%+v after=%+v", state, invalid, before, *adj)
			}
		}
	}
}
