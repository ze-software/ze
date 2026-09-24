// Design: docs/architecture/plugin/rib-storage-design.md -- validation receive/selection boundary
// RFC: rfc/short/draft-ietf-sidrops-aspa-verification.md -- Section 5.7 mitigation policy
package rib

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/netip"
	"strconv"
	"testing"
	"time"

	_ "github.com/ze-software/ze/internal/component/bgp/plugins/adj_rib_in"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// startValidationReceiver runs the actual registered Adj-RIB-In plugin and its
// startup handshake. Tests feed its public bridge, never its private route map.
func startValidationReceiver(t *testing.T, bus *testEventBus) (*rpc.DirectBridge, context.Context) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	registration := registry.Lookup("bgp-adj-rib-in")
	if registration == nil {
		t.Fatal("Adj-RIB-In registration missing")
	}
	registration.ConfigureEventBus(bus)
	previousValidator := rpc.GetBatchValidator()
	bridge := rpc.NewDirectBridge()
	pluginEnd, engineEnd := net.Pipe()
	mux := rpc.NewMuxConn(rpc.NewConn(engineEnd, engineEnd))
	done := make(chan int, 1)
	go func() { done <- registration.RunEngine(rpc.NewBridgedConn(pluginEnd, bridge)) }()
	t.Cleanup(func() {
		bridge.CloseCallbacks()
		_ = mux.Close()
		_ = pluginEnd.Close()
		_ = engineEnd.Close()
		select {
		case <-done:
		case <-ctx.Done():
			t.Error("Adj-RIB-In did not stop")
		}
		rpc.RegisterBatchValidator(previousValidator)
		registration.ConfigureEventBus(nil)
	})
	for stage := range 3 {
		select {
		case req := <-mux.Requests():
			if req == nil {
				t.Fatal("startup connection closed")
			}
			if err := mux.SendOK(ctx, req.ID); err != nil {
				t.Fatal(err)
			}
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
		switch stage {
		case 0:
			if _, err := mux.CallRPC(ctx, "ze-plugin-callback:configure", &rpc.ConfigureInput{}); err != nil {
				t.Fatal(err)
			}
		case 1:
			if _, err := mux.CallRPC(ctx, "ze-plugin-callback:share-registry", &rpc.ShareRegistryInput{}); err != nil {
				t.Fatal(err)
			}
		}
	}
	// A queued callback is served only after the SDK has installed all direct
	// handlers, so it is a readiness barrier without sleeping or polling.
	if _, err := bridge.SendCallback(ctx, "ze-plugin-callback:post-startup", json.RawMessage(`{}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := bridge.ExecuteCommand(ctx, "enable", "request bgp adj-rib-in enable-validation", nil, ""); err != nil {
		t.Fatal(err)
	}
	return bridge, ctx
}

func validationReceive(t *testing.T, bridge *rpc.DirectBridge, r *RIBManager, msgID uint64, nextHop byte) *rpc.StructuredEvent {
	t.Helper()
	// RFC 4271 Section 4.3: Withdrawn length(2), Attributes length(2),
	// ORIGIN(4), empty AS_PATH(3), NEXT_HOP(7), NLRI(4).
	body := []byte{0, 0, 0, 14, 0x40, 1, 1, 0, 0x40, 2, 0,
		0x40, 3, 4, 192, 0, 2, nextHop, 24, 203, 0, 113}
	ctxID, err := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))
	if err != nil {
		t.Fatal(err)
	}
	wu := wireu.NewWireUpdate(body, ctxID)
	attrs, err := wu.Attrs()
	if err != nil {
		t.Fatal(err)
	}
	event := &rpc.StructuredEvent{
		EventType: rpc.EventKindUpdate, PeerAddress: "192.0.2.1", PeerAS: 65001, LocalAS: 65000,
		RawMessage: &bgptypes.RawMessage{MessageID: msgID, RawBytes: body, WireUpdate: wu, AttrsWire: attrs},
	}
	// Deliberately let the selecting RIB receive first: it must consult the
	// other plugin's gate rather than assuming plugin delivery order.
	r.handleReceivedStructured(event)
	if err := bridge.DeliverStructured([]any{event}); err != nil {
		t.Fatal(err)
	}
	return event
}

func validationDecision(t *testing.T, bridge *rpc.DirectBridge, ctx context.Context, text bool, action string, msgID uint64) {
	t.Helper()
	if text {
		_, err := bridge.ExecuteCommand(ctx, "decision", "request bgp adj-rib-in batch-validate",
			[]string{action, "192.0.2.1", "ipv4/unicast", "203.0.113.0/24", "0", "1", strconv.FormatUint(msgID, 10)}, "")
		if err != nil {
			t.Fatal(err)
		}
		return
	}
	validator := rpc.GetBatchValidator()
	if validator == nil {
		t.Fatal("typed batch validator missing")
	}
	if _, err := validator([]rpc.ValidationDecision{{
		Accept: action == "a", Ineligible: action == "i", PeerAddr: "192.0.2.1",
		Family: "ipv4/unicast", Prefix: "203.0.113.0/24", ValState: 1, MsgID: msgID,
	}}); err != nil {
		t.Fatal(err)
	}
}

type validationStoredRoute struct {
	AttrHex    string `json:"attr-hex"`
	NLRIHex    string `json:"nlri-hex"`
	NextHopHex string `json:"nhop-hex"`
	Ineligible bool   `json:"ineligible"`
}

func validationStored(t *testing.T, bridge *rpc.DirectBridge, ctx context.Context) []validationStoredRoute {
	t.Helper()
	out, err := bridge.ExecuteCommand(ctx, "show", "show bgp adj-rib-in", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		Routes map[string][]validationStoredRoute `json:"adj-rib-in"`
	}
	if err := json.Unmarshal(out.Data, &result); err != nil {
		t.Fatal(err)
	}
	return result.Routes["192.0.2.1"]
}

func validationSelection(t *testing.T, r *RIBManager, loc *locrib.RIB, eligible bool, nextHop string) {
	t.Helper()
	prefix := netip.MustParsePrefix("203.0.113.0/24")
	_, installed := loc.Lookup(family.IPv4Unicast, prefix)
	if installed != eligible {
		t.Fatalf("Loc-RIB installed=%v, want %v", installed, eligible)
	}
	best := r.collectBestPaths()[family.IPv4Unicast]
	if !eligible {
		if len(best) != 0 {
			t.Fatalf("ineligible route leaked into best-path replay: %+v", best)
		}
		return
	}
	if len(best) != 1 || best[0].Prefix != prefix || best[0].NextHop.String() != nextHop {
		t.Fatalf("wrong selected/replayed path: %+v", best)
	}
}

// TestASPARetentionReceiveRecovery drives public receive and validation inputs
// through the registered Adj-RIB-In and the selecting RIB, including Loc-RIB.
// RFC requirement: DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.6-1 positive -- Invalid paths retain their complete received bytes and become selectable on a later accepting decision without another UPDATE, whether rejection precedes receive, follows pending receive, or invalidates an installed path.
func TestASPARetentionReceiveRecovery(t *testing.T) {
	for _, text := range []bool{false, true} {
		for _, arrival := range []string{"early", "pending", "installed"} {
			t.Run(strconv.FormatBool(text)+"/"+arrival, func(t *testing.T) {
				bus := newTestEventBus()
				r := newTestRIBManagerWithBus(bus)
				loc := locrib.NewRIB()
				r.SetLocRIB(loc)
				t.Cleanup(func() { r.SetLocRIB(nil) })
				unsubscribe := ribevents.ValidationChange.Subscribe(bus, r.validationChanged)
				t.Cleanup(unsubscribe)
				bridge, ctx := startValidationReceiver(t, bus)
				if arrival == "early" {
					validationDecision(t, bridge, ctx, text, "i", 10)
					if got := validationStored(t, bridge, ctx); len(got) != 0 {
						t.Fatal("early decision invented an unreceived route")
					}
				}
				event := validationReceive(t, bridge, r, 10, 9)
				if arrival == "installed" {
					validationDecision(t, bridge, ctx, text, "a", 10)
					validationSelection(t, r, loc, true, "192.0.2.9")
				}
				if arrival != "early" {
					validationDecision(t, bridge, ctx, text, "i", 10)
				}
				stored := validationStored(t, bridge, ctx)
				msg := event.RawMessage.(*bgptypes.RawMessage)
				if len(stored) != 1 || !stored[0].Ineligible || stored[0].AttrHex != hex.EncodeToString(msg.AttrsWire.Packed()) ||
					stored[0].NLRIHex != "18cb0071" || stored[0].NextHopHex != "c0000209" {
					t.Fatalf("Invalid route bytes were not retained: %+v", stored)
				}
				validationSelection(t, r, loc, false, "")
				validationDecision(t, bridge, ctx, text, "a", 10)
				validationSelection(t, r, loc, true, "192.0.2.9")
				if stored = validationStored(t, bridge, ctx); len(stored) != 1 || stored[0].Ineligible {
					t.Fatalf("recovered route missing or still ineligible: %+v", stored)
				}
			})
		}
	}
}

// TestASPARetentionReplacementWithdrawal prevents stale paths and decisions from
// defeating rejection, recovery, an ordinary withdrawal, or a session removal.
// RFC requirement: DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.6-1 negative -- rejected replacements remove the previous selected path, stale accepting decisions cannot recover them, and withdrawal or session removal deletes the retained route so a later decision cannot reinstall it.
func TestASPARetentionReplacementWithdrawal(t *testing.T) {
	for _, removal := range []string{"withdraw", "session", "origin-reject"} {
		t.Run(removal, func(t *testing.T) {
			bus := newTestEventBus()
			r := newTestRIBManagerWithBus(bus)
			loc := locrib.NewRIB()
			r.SetLocRIB(loc)
			t.Cleanup(func() { r.SetLocRIB(nil) })
			t.Cleanup(ribevents.ValidationChange.Subscribe(bus, r.validationChanged))
			bridge, ctx := startValidationReceiver(t, bus)
			validationReceive(t, bridge, r, 10, 9)
			validationDecision(t, bridge, ctx, false, "a", 10)
			validationSelection(t, r, loc, true, "192.0.2.9")
			validationDecision(t, bridge, ctx, false, "i", 11)
			validationSelection(t, r, loc, false, "")
			event := validationReceive(t, bridge, r, 11, 10)
			validationDecision(t, bridge, ctx, false, "a", 10)
			validationSelection(t, r, loc, false, "")
			stored := validationStored(t, bridge, ctx)
			if len(stored) != 1 || !stored[0].Ineligible || stored[0].NextHopHex != "c000020a" {
				t.Fatalf("failed replacement did not retain the new bytes: %+v", stored)
			}
			switch removal {
			case "withdraw":
				body := []byte{0, 4, 24, 203, 0, 113, 0, 0}
				msg := event.RawMessage.(*bgptypes.RawMessage)
				wu := wireu.NewWireUpdate(body, msg.WireUpdate.SourceCtxID())
				event.RawMessage = &bgptypes.RawMessage{MessageID: 12, RawBytes: body, WireUpdate: wu}
				r.handleReceivedStructured(event)
			case "session":
				event = &rpc.StructuredEvent{EventType: rpc.EventKindState, State: rpc.SessionStateDown, PeerAddress: "192.0.2.1"}
				r.handleStructuredState(event)
			case "origin-reject":
				validationDecision(t, bridge, ctx, true, "r", 11)
			}
			if removal != "origin-reject" {
				if err := bridge.DeliverStructured([]any{event}); err != nil {
					t.Fatal(err)
				}
			}
			validationDecision(t, bridge, ctx, false, "a", 11)
			if got := validationStored(t, bridge, ctx); len(got) != 0 {
				t.Fatalf("removed route resurrected without UPDATE: %+v", got)
			}
			validationSelection(t, r, loc, false, "")
		})
	}
}

// MUTATION: dropping installStructuredNLRIs' validation-change notification
// leaves the selecting RIB dark when it consumes an UPDATE before Adj-RIB-In.
func TestDisabledRPKISelectionFirstReceiveBecomesSelectable(t *testing.T) {
	bus := newTestEventBus()
	r := newTestRIBManagerWithBus(bus)
	loc := locrib.NewRIB()
	r.SetLocRIB(loc)
	t.Cleanup(func() { r.SetLocRIB(nil) })
	t.Cleanup(ribevents.ValidationChange.Subscribe(bus, r.validationChanged))
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
		EventType: rpc.EventKindUpdate, PeerAddress: "192.0.2.1", PeerAS: 65001, LocalAS: 65000,
		MessageID:  10,
		RawMessage: &bgptypes.RawMessage{MessageID: 10, RawBytes: body, WireUpdate: wu, AttrsWire: attrs},
	}
	// The receiver's generation fence must initially exclude the path. This
	// assertion prevents an always-eligible gate from satisfying the test.
	r.handleReceivedStructured(event)
	validationSelection(t, r, loc, false, "")

	// Only the real receiver is called next. Its emitted validation change
	// must wake selection; no second UPDATE or manual reselection repairs it.
	if err := bridge.DeliverStructured([]any{event}); err != nil {
		t.Fatal(err)
	}
	validationSelection(t, r, loc, true, "192.0.2.9")
	stored := validationStored(t, bridge, ctx)
	if len(stored) != 1 || stored[0].Ineligible || stored[0].NextHopHex != "c0000209" ||
		stored[0].AttrHex != hex.EncodeToString(attrs.Packed()) || stored[0].NLRIHex != "18cb0071" {
		t.Fatalf("disabled receive did not retain the selected route: %+v", stored)
	}
}
