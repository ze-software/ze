// Design: docs/architecture/testing/interop.md -- fail-closed wire-history assertions.
package bgp

import (
	"encoding/hex"
	"testing"
)

// TestParsedEmptyMPHistory rejects the historical producer's extra standalone
// EOR and loss of either real route content or the explicit EOR control. These
// are independent recipient bodies, not outputs copied from Ze's splitter.
func TestParsedEmptyMPHistory(t *testing.T) {
	decode := func(text string) []byte {
		t.Helper()
		body, err := hex.DecodeString(text)
		if err != nil {
			t.Fatal(err)
		}
		return body
	}
	eor := decode("00000006800f03000201")
	withdrawal := decode("000418c633640000")
	mixed := decode("000418c633640006800f03000201")
	seed := decode("00000012400101004002040201fdec400304ac1e000a18c63364")
	first := decode("00000012400101004002040201fdec400304ac1e000a18c63365")
	second := decode("00000012400101004002040201fdec400304ac1e000a18c63366")
	third := decode("00000012400101004002040201fdec400304ac1e000a18c63367")
	wrong := decode("000418c633630000")
	asn4 := decode("000000144001010040020602010000fdec400304ac1e000a18c63364")
	cases := []struct {
		name   string
		bodies [][]byte
		phase  int
		valid  bool
	}{
		{"seeded", [][]byte{eor, seed, first}, 1, true},
		{"owned-withdrawal", [][]byte{eor, seed, first, withdrawal, second}, 2, true},
		{"genuine-eor", [][]byte{eor, seed, first, withdrawal, second, eor, third}, 3, true},
		{"old-producer-forged-eor", [][]byte{eor, seed, first, withdrawal, eor, second}, 2, false},
		{"forged-after-desired-frame", [][]byte{eor, seed, first, withdrawal, second, eor}, 2, false},
		{"unseeded-withdrawal", [][]byte{eor, first, withdrawal, second}, 2, false},
		{"missing-withdrawal", [][]byte{eor, seed, first, second}, 2, false},
		{"wrong-withdrawal", [][]byte{eor, seed, first, wrong, second}, 2, false},
		{"duplicate-withdrawal", [][]byte{eor, seed, first, withdrawal, withdrawal, second}, 2, false},
		{"unsplit-mixed-body", [][]byte{eor, seed, first, mixed, second}, 2, false},
		{"missing-genuine-eor", [][]byte{eor, seed, first, withdrawal, second, third}, 3, false},
		{"blanket-eor-suppression", [][]byte{seed, first, withdrawal, second, third}, 3, false},
		{"extra-late-eor", [][]byte{eor, seed, first, withdrawal, second, eor, third, eor}, 3, false},
		{"missing-final-fence", [][]byte{eor, seed, first, withdrawal, second, eor}, 3, false},
		{"not-downgraded", [][]byte{eor, asn4, first}, 1, false},
		{"truncated-tail", [][]byte{eor, seed, first, withdrawal, second, {0}}, 2, false},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			err := parsedEmptyMPHistory(test.bodies, false, test.phase)
			if (err == nil) != test.valid {
				t.Fatalf("history verdict = %v, want valid=%v", err, test.valid)
			}
		})
	}
}
