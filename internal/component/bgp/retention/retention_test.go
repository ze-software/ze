// Design: docs/guide/graceful-restart.md -- the bgp-gr/retention event.
package retention

import (
	"testing"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// envelopeFor wraps an encoded verdict the way the engine delivers it to an
// asker that opted into enveloped delivery.
func envelopeFor(t *testing.T, payload string) string {
	t.Helper()
	return `{"namespace":"` + Namespace + `","event":"` + EventType + `","payload":` + payload + `}`
}

// TestLedgerRecordTake proves a recorded verdict keeps source and family
// discrimination, is consumed by the DOWN that takes it, and that a later
// verdict replaces an earlier one rather than adding to it.
func TestLedgerRecordTake(t *testing.T) {
	var ledger Ledger
	payload, err := Encode("192.0.2.1", []family.Family{family.IPv4Unicast})
	if err != nil {
		t.Fatal(err)
	}
	envelope, ok := Accepts(envelopeFor(t, payload))
	if !ok {
		t.Fatal("enveloped bgp-gr/retention event not accepted")
	}
	if err := ledger.Record(envelope); err != nil {
		t.Fatal(err)
	}
	if retained := ledger.Take("192.0.2.2"); retained[family.IPv4Unicast] {
		t.Fatal("another source inherited retention")
	}
	retained := ledger.Take("192.0.2.1")
	if !retained[family.IPv4Unicast] {
		t.Fatal("recorded family not retained")
	}
	if retained[family.IPv6Unicast] {
		t.Fatal("unretained family inherited peer-wide retention")
	}
	if again := ledger.Take("192.0.2.1"); again[family.IPv4Unicast] {
		t.Fatal("a taken verdict answered a second DOWN")
	}

	first, _ := Encode("192.0.2.1", []family.Family{family.IPv4Unicast}) //nolint:errcheck // same input encoded above
	empty, _ := Encode("192.0.2.1", nil)                                 //nolint:errcheck // an empty verdict encodes
	for _, step := range []string{first, empty} {
		env, ok := Accepts(envelopeFor(t, step))
		if !ok {
			t.Fatal("verdict not accepted")
		}
		if err := ledger.Record(env); err != nil {
			t.Fatal(err)
		}
	}
	if retained := ledger.Take("192.0.2.1"); retained[family.IPv4Unicast] {
		t.Fatal("an empty verdict did not replace the earlier one")
	}
}

// TestLedgerRefusesBadVerdict proves a verdict naming an unknown family or no
// peer is refused whole and leaves the recorded verdict untouched.
func TestLedgerRefusesBadVerdict(t *testing.T) {
	var ledger Ledger
	good, _ := Encode("192.0.2.1", []family.Family{family.IPv4Unicast}) //nolint:errcheck // a fixed input encodes
	env, _ := Accepts(envelopeFor(t, good))
	if err := ledger.Record(env); err != nil {
		t.Fatal(err)
	}
	for _, payload := range []string{
		`{"peer-address":"192.0.2.1","families":["ipv4/unicast","nonsense/none"]}`,
		`{"peer-address":"","families":[]}`,
		`[1,2]`,
	} {
		if err := ledger.Record(rpc.EventEnvelope{Namespace: Namespace, Event: EventType, Payload: []byte(payload)}); err == nil {
			t.Fatalf("payload %s accepted", payload)
		}
	}
	if retained := ledger.Take("192.0.2.1"); !retained[family.IPv4Unicast] {
		t.Fatal("a refused verdict changed the recorded one")
	}
}

// TestAcceptsOnlyRetention proves the asker's event handler tells the
// retention event from its BGP text events and from another bus event.
func TestAcceptsOnlyRetention(t *testing.T) {
	for _, delivered := range []string{
		"peer 192.0.2.1 remote as 65001 state down",
		`{"namespace":"bgp-rib","event":"validation-change","payload":{}}`,
		`{"namespace":"bgp-gr","event":"other","payload":{}}`,
		"",
	} {
		if _, ok := Accepts(delivered); ok {
			t.Fatalf("%q accepted as a retention event", delivered)
		}
	}
}
