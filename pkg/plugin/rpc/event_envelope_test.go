// VALIDATES: spec-fixit-plugin-event-subscription AC-7 (wire compatibility) and
// the Gap B envelope contract.
//   - The additive Namespace/Envelope fields are omitempty: a pre-existing
//     namespace-less, non-enveloped SubscribeEventsInput marshals byte-identical
//     to before the change, so an out-of-tree plugin built against the old SDK
//     keeps producing and receiving identical wire bytes.
//   - EventEnvelope round-trips and preserves the raw payload verbatim.
package rpc

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSubscribeEventsInputNamespaceOmittedFromJSON(t *testing.T) {
	in := SubscribeEventsInput{
		Events: []string{"update"},
		Format: "full",
	}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(b)

	// Byte-identity with the pre-change shape (no namespace/envelope keys).
	const want = `{"events":["update"],"format":"full"}`
	if got != want {
		t.Fatalf("marshaled = %s, want %s (additive fields must be omitempty)", got, want)
	}
	if strings.Contains(got, "namespace") || strings.Contains(got, "envelope") {
		t.Fatalf("legacy input leaked additive keys: %s", got)
	}
}

func TestSubscribeEventsInputNamespaceAndEnvelopePresent(t *testing.T) {
	in := SubscribeEventsInput{
		Events:    []string{"sa-up"},
		Namespace: "vpn-ipsec",
		Envelope:  true,
	}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(b)
	if !strings.Contains(got, `"namespace":"vpn-ipsec"`) {
		t.Fatalf("marshaled = %s, want a namespace field", got)
	}
	if !strings.Contains(got, `"envelope":true`) {
		t.Fatalf("marshaled = %s, want an envelope field", got)
	}
}

func TestEventEnvelopeRoundTrip(t *testing.T) {
	raw := json.RawMessage(`{"peer-name":"peer-1"}`)
	b, err := json.Marshal(EventEnvelope{
		Namespace: "vpn-ipsec",
		Event:     "sa-up",
		Payload:   raw,
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	env, err := ParseEventEnvelope(string(b))
	if err != nil {
		t.Fatalf("ParseEventEnvelope: %v", err)
	}
	if env.Namespace != "vpn-ipsec" || env.Event != "sa-up" {
		t.Fatalf("round-trip = %+v, want namespace=vpn-ipsec event=sa-up", env)
	}
	if string(env.Payload) != string(raw) {
		t.Fatalf("payload = %q, want verbatim %q", env.Payload, raw)
	}
}
