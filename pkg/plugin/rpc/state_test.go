// Design: docs/architecture/api/process-protocol.md -- bounded state requests.
package rpc

import (
	"strings"
	"testing"
)

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
