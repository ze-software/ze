// Design: docs/architecture/config/apply-ordering.md -- settlement on (interface, addr-added)
// Related: monitor.go -- emitAddress
// Related: ifacevpp.go -- AddAddress, RemoveAddress

package ifacevpp

import (
	"encoding/json"
	"testing"
)

// TestAddressChangeEmitsEvent proves that the VPP backend announces an address
// it has programmed, in the payload shape ifacenetlink uses.
//
// VALIDATES: AddAddress emits (interface, addr-added) and RemoveAddress emits
// (interface, addr-removed) once VPP's reply reports success, naming the
// interface, its index and the bare address the transaction settles on.
// PREVENTS: a config reload that adds an address under the VPP backend waiting
// for an event VPP never sends, timing out after 5s, and rolling the whole
// reload back (test/plugin/vpp-loopback-reapply.ci).
func TestAddressChangeEmitsEvent(t *testing.T) {
	ch := &monitorChannel{}
	b := &vppBackendImpl{ch: ch, names: newNameMap()}
	bus := &recordingBus{}
	if err := b.StartMonitor(bus); err != nil {
		t.Fatalf("StartMonitor: %v", err)
	}
	defer b.StopMonitor()
	b.names.Add("lo0", 12, "lo0")

	if err := b.AddAddress("lo0", "10.42.0.1/32"); err != nil {
		t.Fatalf("AddAddress: %v", err)
	}
	if err := b.RemoveAddress("lo0", "10.42.0.1/32"); err != nil {
		t.Fatalf("RemoveAddress: %v", err)
	}

	if bus.len() != 2 {
		t.Fatalf("events: got %d, want 2 (addr-added, addr-removed)", bus.len())
	}
	for i, want := range []string{"addr-added", "addr-removed"} {
		ev := bus.at(i)
		if ev.Namespace != "interface" || ev.Type != want {
			t.Fatalf("event %d: got %s/%s, want interface/%s", i, ev.Namespace, ev.Type, want)
		}
		var got addrEventPayload
		if err := json.Unmarshal([]byte(ev.Payload), &got); err != nil {
			t.Fatalf("event %d payload %q: %v", i, ev.Payload, err)
		}
		wantPayload := addrEventPayload{Name: "lo0", Index: 12, Address: "10.42.0.1", PrefixLength: 32, Family: "ipv4", Origin: "static"}
		if got != wantPayload {
			t.Errorf("event %d payload: got %+v, want %+v", i, got, wantPayload)
		}
	}
}
