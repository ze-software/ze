// Design: docs/architecture/isis/isis-11-redistribution.md -- producer wiring test (AC-11).
//
// VALIDATES: spec-isis-11 producer wiring -- RegisterProducer(spf.ProtocolID())
//            ran, so IS-IS is in redistevents.Producers() and the orchestrator
//            subscribes; the single "isis" identity is shared with the Loc-RIB
//            install Source (no second RegisterProtocol).
// PREVENTS:  a regression where source.go registers only the config RouteSource
//            (without RegisterProducer), so IS-IS is absent from Producers() and
//            no IS-IS route ever reaches BGP (the AC-11 trap).

package isisredistevents

import (
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/plugins/isis/spf"
)

// TestISISProducerRegistered asserts the four-part producer wiring (spec AC-11):
// after this package is imported, redistevents.Producers() contains the IS-IS
// ProtocolID, redistevents.ProtocolIDOf("isis") resolves to it, and the local
// ProtocolID equals the single identity spec-isis-9 allocated (spf.ProtocolID()).
// Registering only the config RouteSource (without RegisterProducer) would NOT add
// IS-IS to Producers(); this test proves RegisterProducer ran.
func TestISISProducerRegistered(t *testing.T) {
	// The ProtocolID this package exposes must be the SAME identity SPF allocated
	// (single "isis" source; no second RegisterProtocol).
	if ProtocolID != spf.ProtocolID() {
		t.Fatalf("ProtocolID = %d, want spf.ProtocolID() = %d (single shared identity)", ProtocolID, spf.ProtocolID())
	}

	// ProtocolIDOf("isis") must resolve to the same ID.
	id, ok := redistevents.ProtocolIDOf(Namespace)
	if !ok {
		t.Fatalf("ProtocolIDOf(%q) not found; RegisterProtocol(%q) did not run", Namespace, Namespace)
	}
	if id != ProtocolID {
		t.Fatalf("ProtocolIDOf(%q) = %d, want %d", Namespace, id, ProtocolID)
	}

	// Producers() must contain the IS-IS ID: RegisterProducer ran (the part that
	// makes the orchestrator subscribe -- AC-11).
	if !slices.Contains(redistevents.Producers(), ProtocolID) {
		t.Fatalf("redistevents.Producers() does not contain IS-IS ProtocolID %d; RegisterProducer did not run", ProtocolID)
	}

	// The RouteChange handle must be bound (non-nil) so the source can emit.
	if RouteChange == nil {
		t.Fatal("RouteChange handle is nil; events.Register did not bind the typed handle")
	}

	// Defensive: the registration must have actually run.
	if !producerRegistered {
		t.Fatal("producerRegistered is false; the producer var initializer did not run")
	}
}
