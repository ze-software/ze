// Design: docs/architecture/api/process-protocol.md -- bounded state requests.
package rpc

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/ze-software/ze/pkg/zefs"
)

// TestStateBoundPerKey proves a key's declared bound replaces the default for
// that key alone, at the last valid and the first invalid size.
//
// Method: the IRR dataset key declares zefs.IRRDatasetValueMax; a value of
// exactly that size validates and one byte more is refused. The pointer key
// beside it declares nothing and keeps StateDataMax: StateDataMax validates,
// StateDataMax+1 is refused even though it is far under the dataset's bound.
func TestStateBoundPerKey(t *testing.T) {
	dataset := zefs.KeyIRRDataset.Key("whois.radb.net", "AS-TEST")
	if got := StateDataMaxFor(dataset); got != zefs.IRRDatasetValueMax {
		t.Fatalf("StateDataMaxFor(%s) = %d, want %d", dataset, got, zefs.IRRDatasetValueMax)
	}
	if err := ValidateStateInput(StateInput{Key: dataset, Data: make([]byte, zefs.IRRDatasetValueMax)}); err != nil {
		t.Fatalf("dataset at its bound refused: %v", err)
	}
	if err := ValidateStateInput(StateInput{Key: dataset, Data: make([]byte, zefs.IRRDatasetValueMax+1)}); err == nil {
		t.Fatal("dataset one byte over its bound accepted")
	}
	pointer := zefs.KeyIRRSet.Key("AS-TEST")
	if got := StateDataMaxFor(pointer); got != StateDataMax {
		t.Fatalf("StateDataMaxFor(%s) = %d, want the default %d", pointer, got, StateDataMax)
	}
	if err := ValidateStateInput(StateInput{Key: pointer, Data: make([]byte, StateDataMax)}); err != nil {
		t.Fatalf("pointer at the default bound refused: %v", err)
	}
	if err := ValidateStateInput(StateInput{Key: pointer, Data: make([]byte, StateDataMax+1)}); err == nil {
		t.Fatal("pointer one byte over the default bound accepted")
	}
	if got := StateDataLargest(); got != zefs.IRRDatasetValueMax {
		t.Fatalf("StateDataLargest() = %d, want %d", got, zefs.IRRDatasetValueMax)
	}
}

// TestDeclaredStateBoundsFitAMessage proves every declared bound still travels
// in one RPC message: the value goes base64 in JSON, and the request line
// carries its id, method and key around it.
//
// Method: for each registered entry that declares a bound, encode a value of
// that size and add a generous envelope (the longest key plus 256 bytes of
// framing), then compare with MaxMessageSize.
func TestDeclaredStateBoundsFitAMessage(t *testing.T) {
	const envelope = StateKeyMax + 256
	for _, entry := range zefs.AllEntries() {
		if entry.ValueMax == 0 {
			continue
		}
		size := base64.StdEncoding.EncodedLen(entry.ValueMax) + envelope
		if size > MaxMessageSize {
			t.Errorf("%s declares %d bytes: %d bytes on the wire, over MaxMessageSize %d", entry.Pattern, entry.ValueMax, size, MaxMessageSize)
		}
	}
}

// Boundary and traversal cases must be rejected before either transport reaches
// storage. A maximum-sized opaque value remains a valid write.
func TestStateInputBounds(t *testing.T) {
	valid := StateInput{Key: "meta/" + strings.Repeat("k", StateKeyMax-5), Data: make([]byte, StateDataMax)}
	if err := ValidateStateInput(valid); err != nil {
		t.Fatal(err)
	}
	cases := []StateInput{
		{Key: "meta/" + strings.Repeat("k", StateKeyMax-4)},
		{Key: "meta/ospf/value", Data: make([]byte, StateDataMax+1)},
		{Key: "meta/ospf/../auth/password"},
		{Key: "/meta/ospf/value"},
		{Key: "meta/ospf/value\nforged"},
		{Key: "file/active/ze.conf"},
	}
	for _, input := range cases {
		if err := ValidateStateInput(input); err == nil {
			t.Fatalf("accepted invalid state request key=%q data-bytes=%d", input.Key, len(input.Data))
		}
	}
}
