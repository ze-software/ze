// Design: docs/architecture/plugin/rib-storage-design.md -- receive validation gate
// Related: rib_validation.go -- dispatchText, validationChanged
// Related: draft_ietf_sidrops_aspa_verification_rib_aspa_retention_test.go -- startValidationReceiver

package rib

import (
	"encoding/json"
	"net/netip"
	"testing"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestValidationChangeArrivesAsEnvelopedText proves bgp-rib hears the receive
// store's validation-change as the enveloped text event the SDK subscription
// delivers, on the path both transports share.
//
// VALIDATES: an enveloped bgp-rib/validation-change reaching dispatchText
// reruns selection for the named route, so a route first fenced out becomes
// selectable with no second UPDATE.
// PREVENTS: a forked bgp-rib, which never receives the daemon's EventBus,
// never rerunning selection when Adj-RIB-In's eligibility changes.
// Method: run the real Adj-RIB-In, wrap each change it emits in the envelope a
// subscriber with SetEnvelope receives and hand it to dispatchText, then follow
// TestDisabledRPKISelectionFirstReceiveBecomesSelectable.
func TestValidationChangeArrivesAsEnvelopedText(t *testing.T) {
	bus := newTestEventBus()
	r := newTestRIBManagerWithBus(bus)
	loc := locrib.NewRIB()
	r.SetLocRIB(loc)
	t.Cleanup(func() { r.SetLocRIB(nil) })
	// The engine's delivery to a subscriber with SetEnvelope: the batch, as
	// JSON, inside its envelope, handed to the text event handler.
	t.Cleanup(ribevents.ValidationChange.Subscribe(bus, func(changes []ribevents.ValidationRoute) {
		payload, err := json.Marshal(changes)
		if err != nil {
			t.Error(err)
			return
		}
		envelope, err := json.Marshal(rpc.EventEnvelope{Namespace: ribevents.Namespace, Event: ribevents.EventValidationChange, Payload: payload})
		if err != nil {
			t.Error(err)
			return
		}
		r.dispatchText(string(envelope))
	}))
	bridge, ctx := startValidationReceiver(t, bus)
	if _, err := bridge.ExecuteCommand(ctx, "disable", "request bgp adj-rib-in disable-validation", nil, ""); err != nil {
		t.Fatal(err)
	}

	body := []byte{0, 0, 0, 14, 0x40, 1, 1, 0, 0x40, 2, 0,
		0x40, 3, 4, 192, 0, 2, 9, 24, 203, 0, 113}
	wu := wireu.NewWireUpdate(body, bgpctx.APIContextID)
	attrs, err := wu.Attrs()
	if err != nil {
		t.Fatal(err)
	}
	event := &rpc.StructuredEvent{
		EventType: rpc.EventKindUpdate, PeerAddress: "192.0.2.1", PeerAS: rpc.RemoteASOf(65001), LocalAS: 65000,
		MessageID:  10,
		RawMessage: &bgptypes.RawMessage{MessageID: 10, RawBytes: body, WireUpdate: wu, AttrsWire: attrs},
	}
	// The receiver's generation fence excludes the path until the receiver
	// stores it, so selection starts empty.
	r.handleReceivedStructured(event)
	validationSelection(t, r, loc, false, "")

	// The receiver's emitted change, as enveloped text, must wake selection.
	if err := bridge.DeliverStructured([]any{event}); err != nil {
		t.Fatal(err)
	}
	validationSelection(t, r, loc, true, "192.0.2.9")
}

// TestValidationGateArrivesAsEnvelopedText proves bgp-rib applies the receive
// store's validation-gate announcement, the event a forked bgp-rib hears in
// place of the store's lookup.
//
// VALIDATES: an enveloped bgp-rib/validation-gate reaching dispatchText with
// no lookup in this process makes selection's gate refuse a path, and the
// disabled announcement lifts it.
// PREVENTS: a forked bgp-rib selecting every received path as best while
// RPKI or ASPA validation is enabled in the receive store's process.
// Method: hand dispatchText each envelope and ask the rib's own gate query.
func TestValidationGateArrivesAsEnvelopedText(t *testing.T) {
	t.Cleanup(func() { ribevents.RegisterValidationLookup(nil, nil) })
	ribevents.RegisterValidationLookup(nil, nil)
	r := newTestRIBManagerWithBus(newTestEventBus())
	gate := func(enabled bool) string {
		payload, err := json.Marshal(ribevents.ValidationGateState{Enabled: enabled})
		if err != nil {
			t.Fatal(err)
		}
		envelope, err := json.Marshal(rpc.EventEnvelope{Namespace: ribevents.Namespace, Event: ribevents.EventValidationGate, Payload: payload})
		if err != nil {
			t.Fatal(err)
		}
		return string(envelope)
	}
	route := ribevents.ValidationRoute{
		Peer:   netip.MustParseAddr("192.0.2.1"),
		Family: family.IPv4Unicast,
		Prefix: netip.MustParsePrefix("203.0.113.0/24"),
	}

	r.dispatchText(gate(true))
	if !ribevents.ValidationEnabled() {
		t.Fatal("the announced gate must turn validation on in this process")
	}
	if ribevents.RouteEligible(route, 0) {
		t.Fatal("selection must refuse a path the gate cannot judge here")
	}

	r.dispatchText(gate(false))
	if ribevents.ValidationEnabled() {
		t.Fatal("the disabled gate must leave no validation here")
	}
}
