// VALIDATES: spec-ospf-ext-14 AC-21 -- the offline `ze ospf decode --v3` path renders an
// OSPFv3 LSA's scope-aware LS Type (U/S2/S1 + function code) plus its typed body, with no
// running engine.
// PREVENTS: an offline v3 decode that reports a flat type number or hides the scope.
package cli

import (
	"testing"

	ospfv3packet "github.com/ze-software/ze/internal/plugins/ospf/v3/packet"
	ospfv3types "github.com/ze-software/ze/internal/plugins/ospf/v3/types"
)

func TestV3OfflineDecode(t *testing.T) {
	rb := ospfv3packet.RouterLSA{}
	body := make([]byte, rb.EncodedLen())
	rb.WriteTo(body, 0)
	l := ospfv3packet.LSA{
		Header: ospfv3packet.LSAHeader{
			Type:   ospfv3types.LSTypeRouter,
			Length: uint16(20 + len(body)),
		},
		Body:   body,
		Router: &rb,
	}
	out := renderV3LSA(l)
	if out.LSTypeHex != "0x2001" {
		t.Fatalf("ls-type-hex = %q, want 0x2001", out.LSTypeHex)
	}
	if out.Scope != "area" {
		t.Fatalf("scope = %q, want area", out.Scope)
	}
	if out.FunctionCode != 1 || out.Decoded == nil {
		t.Fatalf("function-code/decoded = %d/%v", out.FunctionCode, out.Decoded)
	}
}
