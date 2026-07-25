// VALIDATES: spec-ospfv3-2-wire AC-10 -- the Inter-Area-Prefix-LSA round-trips
// the 24-bit metric and the inlined prefix (length, options, address).
// PREVENTS: mis-sizing the inlined AddressPrefix against the prefix length.

package packet

import (
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/v3/types"
)

func TestOSPFv3InterAreaPrefixRoundTrip(t *testing.T) {
	want := LSA{
		Header:       sampleLSAHeader(t, types.LSTypeInterAreaPrefix, "0.0.0.1"),
		InterAreaPfx: &InterAreaPrefixLSA{Metric: 0x0a0b0c, Prefix: makePrefix(t, 64, types.OptPrefixNU, 0)},
	}
	wire := encodeLSA(t, want)

	decoded, err := DecodeLSA(wire)
	if err != nil {
		t.Fatalf("DecodeLSA inter-area-prefix: %v", err)
	}
	body, err := decoded.DecodeInterAreaPrefix()
	if err != nil {
		t.Fatalf("DecodeInterAreaPrefix: %v", err)
	}
	if body.Metric != want.InterAreaPfx.Metric {
		t.Fatalf("metric = %#x, want %#x", body.Metric, want.InterAreaPfx.Metric)
	}
	assertPrefixEqual(t, body.Prefix, want.InterAreaPfx.Prefix)
}

// assertPrefixEqual compares two decoded prefixes by length, options, and bytes.
func assertPrefixEqual(t *testing.T, got, want Prefix) {
	t.Helper()
	if got.Length != want.Length || got.Options != want.Options {
		t.Fatalf("prefix length/options = %d/%#x, want %d/%#x", got.Length, got.Options, want.Length, want.Options)
	}
	if len(got.Address) != want.Length.ByteLen() {
		t.Fatalf("prefix address length = %d, want %d", len(got.Address), want.Length.ByteLen())
	}
	for i := range want.Address {
		if got.Address[i] != want.Address[i] {
			t.Fatalf("prefix address[%d] = %#x, want %#x", i, got.Address[i], want.Address[i])
		}
	}
}
