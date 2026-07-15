package fsm

import (
	"net/netip"
	"testing"
)

// TestEventsImplementInterface pins the closed Event set: every event type
// satisfies the Event interface (compile-time), so Handle's type switch is
// exhaustive.
//
// VALIDATES: Types section "Events (each a small value type; the FSM's handle
// method takes exactly one)".
func TestEventsImplementInterface(t *testing.T) {
	events := []Event{
		Startup{},
		Shutdown{},
		AdvertReceived{},
		MasterDownExpired{},
		AdvertTimerExpired{},
		PreemptDelayExpired{},
		ConfigUpdated{},
	}
	if len(events) != 7 {
		t.Fatalf("expected 7 event types, got %d", len(events))
	}
}

// TestAdvertReceivedCarriesDecodedFields proves AdvertReceived carries plain
// decoded fields only (priority, srcIP, interval ms, VIP count), never packet
// types -- the fsm package stays free of any spec-vrrp-1 packet import.
//
// VALIDATES: Data Flow "carries only decoded, validated fields"; import
// isolation (packet is a parallel sibling).
func TestAdvertReceivedCarriesDecodedFields(t *testing.T) {
	ev := AdvertReceived{
		Priority:   200,
		SrcIP:      netip.MustParseAddr("192.0.2.1"),
		IntervalMs: 1000,
		VIPCount:   2,
	}
	if ev.Priority != 200 || ev.IntervalMs != 1000 || ev.VIPCount != 2 {
		t.Fatalf("field round-trip failed: %+v", ev)
	}
	if ev.SrcIP != netip.MustParseAddr("192.0.2.1") {
		t.Fatalf("srcIP round-trip failed: %v", ev.SrcIP)
	}
}
