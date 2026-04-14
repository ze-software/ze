package server

import (
	"bytes"
	"errors"
	"testing"

	txevents "codeberg.org/thomas-mangin/ze/internal/component/config/transaction/events"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

// VALIDATES: ConfigEventGateway.EmitConfigEvent emits in the config namespace
// and the underlying engine subscriber receives the same payload bytes.
// PREVENTS: silent namespace drift (e.g., emitting in "bgp" or "" by mistake).
func TestConfigEventGatewayEmit(t *testing.T) {
	s := &Server{engineSubscribers: newEngineEventSubscribers()}
	g := NewConfigEventGateway(s)

	var got string
	unsub := s.SubscribeEngineEvent(txevents.Namespace, "verify-ok", func(event string) {
		got = event
	})
	defer unsub()

	payload := []byte(`{"plugin":"bgp","code":"ok"}`)
	delivered, err := g.EmitConfigEvent("verify-ok", payload)
	if err != nil {
		t.Fatalf("EmitConfigEvent: %v", err)
	}
	// No plugin processes; delivered count is 0. Engine handler still fires.
	if delivered != 0 {
		t.Errorf("delivered = %d, want 0 (no plugin processes)", delivered)
	}
	if got != string(payload) {
		t.Errorf("handler received %q, want %q", got, string(payload))
	}
}

// VALIDATES: ConfigEventGateway.SubscribeConfigEvent receives events emitted
// directly via Server.EmitEngineEvent in the config namespace, and the
// handler is given the payload as []byte.
// PREVENTS: subscription lookup failing because the gateway forgot to use
// the config namespace, or the handler being passed the wrong type.
func TestConfigEventGatewaySubscribe(t *testing.T) {
	s := &Server{engineSubscribers: newEngineEventSubscribers()}
	g := NewConfigEventGateway(s)

	var got []byte
	unsub := g.SubscribeConfigEvent("apply-ok", func(payload []byte) {
		got = payload
	})
	defer unsub()

	emitted := `{"plugin":"iface","code":"ok"}`
	if _, err := s.EmitEngineEvent(txevents.Namespace, "apply-ok", emitted); err != nil {
		t.Fatalf("EmitEngineEvent: %v", err)
	}
	if string(got) != emitted {
		t.Errorf("handler received %q, want %q", string(got), emitted)
	}
}

// VALIDATES: SubscribeConfigEvent with a nil handler returns a no-op
// unsubscribe function and does not register anything.
// PREVENTS: nil handler reaching dispatch and causing a nil pointer panic
// (the underlying Server.SubscribeEngineEvent has the same defense, but the
// adapter must not bypass it).
func TestConfigEventGatewayNilHandler(t *testing.T) {
	s := &Server{engineSubscribers: newEngineEventSubscribers()}
	g := NewConfigEventGateway(s)

	unsub := g.SubscribeConfigEvent("verify-ok", nil)
	if unsub == nil {
		t.Fatal("SubscribeConfigEvent returned nil unsub function")
	}
	// Calling unsub must not panic.
	unsub()

	// Verify nothing was registered: emit and observe no panic.
	if _, err := s.EmitEngineEvent(txevents.Namespace, "verify-ok", `payload`); err != nil {
		t.Fatalf("EmitEngineEvent: %v", err)
	}
}

// VALIDATES: a JSON payload round-trips through gateway emit and gateway
// subscribe with byte-for-byte equality.
// PREVENTS: subtle data corruption from the []byte -> string -> []byte
// conversion in the adapter (e.g., if a future "optimization" used unsafe
// conversion or trimmed the slice).
func TestConfigEventGatewayPayloadRoundTrip(t *testing.T) {
	s := &Server{engineSubscribers: newEngineEventSubscribers()}
	g := NewConfigEventGateway(s)

	var received []byte
	unsub := g.SubscribeConfigEvent("verify-ok", func(payload []byte) {
		// Copy because the underlying string sharing may be reused.
		received = append(received[:0], payload...)
	})
	defer unsub()

	// Realistic config ack payload with embedded quotes, slashes, and unicode.
	original := []byte(`{"transaction-id":"tx-1700000000","plugin":"bgp","code":"ok","apply-budget-secs":12,"path":"a/b","tag":"\u00e9"}`)
	if _, err := g.EmitConfigEvent("verify-ok", original); err != nil {
		t.Fatalf("EmitConfigEvent: %v", err)
	}
	if !bytes.Equal(received, original) {
		t.Errorf("round trip mismatch:\n  got:  %q\n  want: %q", received, original)
	}
}

// VALIDATES: EmitConfigEvent rejects unknown event types via deliverEvent's
// validation, returning an *rpc.RPCCallError. The orchestrator relies on
// this error to fail fast.
// PREVENTS: typos in event-type strings silently no-op'ing instead of
// failing the transaction.
func TestConfigEventGatewayRejectsUnknownEvent(t *testing.T) {
	s := &Server{engineSubscribers: newEngineEventSubscribers()}
	g := NewConfigEventGateway(s)

	_, err := g.EmitConfigEvent("not-a-real-event", []byte(`{}`))
	if err == nil {
		t.Fatal("EmitConfigEvent with unknown event type returned nil error")
	}
	var rpcErr *rpc.RPCCallError
	if !errors.As(err, &rpcErr) {
		t.Errorf("expected *rpc.RPCCallError, got %T: %v", err, err)
	}
}

// VALIDATES: EmitConfigEvent rejects empty payload (deliverEvent's empty-event
// guard). string(nil) and string([]byte{}) both produce "".
// PREVENTS: orchestrator accidentally publishing zero-length events that
// downstream subscribers cannot decode.
func TestConfigEventGatewayRejectsEmptyPayload(t *testing.T) {
	s := &Server{engineSubscribers: newEngineEventSubscribers()}
	g := NewConfigEventGateway(s)

	for _, payload := range [][]byte{nil, {}} {
		_, err := g.EmitConfigEvent("verify-ok", payload)
		if err == nil {
			t.Errorf("EmitConfigEvent with empty payload (%v) returned nil error", payload)
		}
	}
}

// VALIDATES: gateway unsubscribe stops further delivery to the handler.
// PREVENTS: orchestrator's defer-unsub leaving stale subscriptions that
// receive late acks from prior transactions.
func TestConfigEventGatewayUnsubscribe(t *testing.T) {
	s := &Server{engineSubscribers: newEngineEventSubscribers()}
	g := NewConfigEventGateway(s)

	var calls int
	unsub := g.SubscribeConfigEvent("verify-ok", func(_ []byte) { calls++ })

	if _, err := g.EmitConfigEvent("verify-ok", []byte(`first`)); err != nil {
		t.Fatalf("first emit: %v", err)
	}
	if calls != 1 {
		t.Fatalf("after first emit: calls = %d, want 1", calls)
	}

	unsub()
	if _, err := g.EmitConfigEvent("verify-ok", []byte(`second`)); err != nil {
		t.Fatalf("second emit: %v", err)
	}
	if calls != 1 {
		t.Errorf("after unsub + second emit: calls = %d, want 1", calls)
	}
}
