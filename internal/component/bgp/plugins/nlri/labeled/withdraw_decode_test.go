// Design: docs/architecture/wire/nlri.md -- labeled withdrawal decoder context.
package labeled

import (
	"encoding/json"
	"testing"
)

// TestDecodeNLRIHexWithdrawalContext proves that the real decoder ignores all
// Compatibility bits, keeps every prefix and Path Identifier, and retains the
// complete label entries only when the caller selects announcement decoding.
func TestDecodeNLRIHexWithdrawalContext(t *testing.T) {
	t.Parallel()
	families := []struct {
		name, length, prefix, want, prefixNext, wantNext string
	}{
		{"ipv4/mpls-label", "20", "0a", "10.0.0.0/8", "0b", "11.0.0.0/8"},
		{"ipv6/mpls-label", "38", "20010db8", "2001:db8::/32", "20010db9", "2001:db9::/32"},
	}
	layouts := []struct {
		name, path, json string
		addPath          bool
	}{
		{"base", "", "", false},
		{"path-zero", "00000000", `"path-id":0,`, true},
		{"path-seventeen", "00000011", `"path-id":17,`, true},
	}
	for _, fam := range families {
		for _, layout := range layouts {
			for _, compatibility := range []string{"800000", "000000", "000641", "123456", "ffffff"} {
				t.Run(fam.name+"/"+layout.name+"/"+compatibility, func(t *testing.T) {
					t.Parallel()
					wire := layout.path + fam.length + compatibility + fam.prefix
					want := `{` + layout.json + `"prefix":"` + fam.want + `"}`
					assertLabeledDecodeJSON(t, fam.name, wire, layout.addPath, true, want)
					wireNext := layout.path + fam.length + compatibility + fam.prefixNext
					wantNext := `{` + layout.json + `"prefix":"` + fam.wantNext + `"}`
					assertLabeledDecodeJSON(t, fam.name, wire+wireNext, layout.addPath, true, "["+want+","+wantNext+"]")
					assertLabeledDecodeJSON(t, fam.name, wire+wire, layout.addPath, true, "["+want+","+want+"]")
				})
			}
			t.Run(fam.name+"/"+layout.name+"/announcement", func(t *testing.T) {
				t.Parallel()
				length := "38"
				if fam.name == "ipv6/mpls-label" {
					length = "50"
				}
				wire := layout.path + length + "00064e000c8b" + fam.prefix
				want := `{"labels":[[100,1614],[200,3211]],` + layout.json + `"prefix":"` + fam.want + `"}`
				assertLabeledDecodeJSON(t, fam.name, wire, layout.addPath, false, want)
				wireNext := layout.path + length + "00064e000c8b" + fam.prefixNext
				wantNext := `{"labels":[[100,1614],[200,3211]],` + layout.json + `"prefix":"` + fam.wantNext + `"}`
				assertLabeledDecodeJSON(t, fam.name, wire+wireNext, layout.addPath, false, "["+want+","+wantNext+"]")
				// Identical octets mean an announcement only when withdraw is false.
				assertLabeledDecodeJSON(t, fam.name, layout.path+fam.length+"000641"+fam.prefix,
					layout.addPath, false, `{"labels":[[100,1601]],`+layout.json+`"prefix":"`+fam.want+`"}`)
			})
		}
	}
}

// TestDecodeNLRIHexWithdrawalMalformed rejects truncated fields, invalid prefix
// widths, and malformed tails rather than returning a partial decoded section.
func TestDecodeNLRIHexWithdrawalMalformed(t *testing.T) {
	t.Parallel()
	for _, wire := range []string{"", "zz", "20", "208000", "20800000", "17800000", "398000000000000000", "208000000a20"} {
		t.Run(wire, func(t *testing.T) {
			t.Parallel()
			if got, err := DecodeNLRIHex("ipv4/mpls-label", wire, false, true); err == nil {
				t.Fatalf("DecodeNLRIHex(%q) = %#v without error", wire, got)
			}
		})
	}
	for _, wire := range []string{"000000", "00000011", "00000011208000000a000000"} {
		if got, err := DecodeNLRIHex("ipv4/mpls-label", wire, true, true); err == nil {
			t.Errorf("ADD-PATH DecodeNLRIHex(%q) = %#v without error", wire, got)
		}
	}
}

func assertLabeledDecodeJSON(t *testing.T, family, wire string, addPath, withdraw bool, want string) {
	t.Helper()
	// RFC 8277 Section 2.4: the caller supplies withdrawal context.
	got, err := DecodeNLRIHex(family, wire, addPath, withdraw)
	if err != nil {
		t.Fatalf("DecodeNLRIHex(%q): %v", wire, err)
	}
	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if string(encoded) != want {
		t.Errorf("DecodeNLRIHex(%q) = %s, want %s", wire, encoded, want)
	}
}
