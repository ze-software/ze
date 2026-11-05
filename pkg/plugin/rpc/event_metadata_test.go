// Design: docs/architecture/api/process-protocol.md -- namespace-owned event envelope metadata.
package rpc

import (
	"encoding/json"
	"testing"
)

// TestEventEnvelopePreservesOpaqueMetadata proves the generic parser and writer
// preserve a namespace's metadata without interpreting its fields or numbers.
// Other events legitimately have none and retain the existing bare envelope.
func TestEventEnvelopePreservesOpaqueMetadata(t *testing.T) {
	const input = `{"namespace":"example","event":"changed","payload":"unchanged text\n","metadata":{"receipt":18446744073709551610,"owned":["one","two"]}}`
	envelope, err := ParseEventEnvelope(input)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	var output map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &output); err != nil {
		t.Fatal(err)
	}
	if string(output["metadata"]) != `{"receipt":18446744073709551610,"owned":["one","two"]}` {
		t.Fatalf("opaque metadata lost or changed: %s", encoded)
	}
	if string(output["payload"]) != `"unchanged text\n"` {
		t.Fatalf("envelope changed its text payload: %s", encoded)
	}
	plain, err := ParseEventEnvelope(`{"namespace":"example","event":"ready","payload":{}}`)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err = json.Marshal(plain)
	if err != nil {
		t.Fatal(err)
	}
	if string(encoded) != `{"namespace":"example","event":"ready","payload":{}}` {
		t.Fatalf("metadata-free event changed: %s", encoded)
	}
}
