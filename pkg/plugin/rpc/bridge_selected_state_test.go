// Design: docs/architecture/api/process-protocol.md -- selected state projection.
package rpc

import "testing"

// TestSelectedStatePoolReset proves returned state metadata cannot authorize
// the next event. Method: return an eligible state and inspect the same object.
func TestSelectedStatePoolReset(t *testing.T) {
	event := GetStructuredEvent()
	event.SelectedDestination, event.DestinationSession = true, 91
	PutStructuredEvent(event)
	if event.SelectedDestination {
		t.Fatal("pooled state retained selected destination eligibility")
	}
	if event.DestinationSession != 0 {
		t.Fatal("pooled state retained a previous connection identity")
	}
}
