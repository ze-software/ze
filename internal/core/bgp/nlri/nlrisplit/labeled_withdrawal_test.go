// Design: docs/architecture/wire/nlri.md -- labeled withdrawal on send
// Overview: prefix_key.go -- WithdrawalWriter, writeWithdrawalField

package nlrisplit

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/ze-software/ze/internal/core/family"
)

// TestLabeledWithdrawalOneCompatibilityField checks that a run of announced
// labeled NLRIs becomes the withdrawal form of RFC 8277 Section 2.4, one
// three-octet Compatibility field per NLRI whatever the label count, with the
// length octet reduced by 24 bits for each label dropped. Method: convert
// announcement-form sections in place with the family's registered writer, as
// the RFC 7606 treat-as-withdraw does, and compare the whole result.
//
// RFC 8277 Section 2.4: "This encoding is used whether or not the Multiple
// Labels Capability has been sent or received on the session." and "Upon
// transmission, the Compatibility field SHOULD be set to 0x800000.".
func TestLabeledWithdrawalOneCompatibilityField(t *testing.T) {
	labeled := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIMPLSLabel}
	vpn := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIVPN}
	tests := []struct {
		name    string
		fam     family.Family
		addPath bool
		in      string
		want    string
	}{
		{
			name: "one label",
			fam:  labeled,
			in:   "20" + "003E81" + "0A",
			want: "20" + "800000" + "0A",
		},
		{
			// Two labels and a /8: 48 + 8 bits announced, 24 + 8 withdrawn.
			name: "two labels",
			fam:  labeled,
			in:   "38" + "000010" + "003E81" + "0A",
			want: "20" + "800000" + "0A",
		},
		{
			// VPN-IPv4: three labels, RD 65000:1, 1.4.0.0/16.
			name: "three labels and an RD",
			fam:  vpn,
			in:   "98" + "000010" + "000020" + "003E81" + "0000FDE800000001" + "0104",
			want: "68" + "800000" + "0000FDE800000001" + "0104",
		},
		{
			// ADD-PATH, two routes, the second with two labels.
			name:    "ADD-PATH, two routes",
			fam:     labeled,
			addPath: true,
			in:      "00000001" + "20" + "000011" + "0A" + "00000002" + "40" + "000010" + "000021" + "C0A8",
			want:    "00000001" + "20" + "800000" + "0A" + "00000002" + "28" + "800000" + "C0A8",
		},
		{
			name: "empty section",
			fam:  labeled,
			in:   "",
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			section, err := hex.DecodeString(tt.in)
			if err != nil {
				t.Fatal(err)
			}
			want, err := hex.DecodeString(tt.want)
			if err != nil {
				t.Fatal(err)
			}
			announced := routeKeys(t, SplitLabeled, section, tt.addPath, false)
			routes := len(announced)
			n, err := GetWithdrawalWriter(tt.fam)(section, section, tt.addPath, false)
			if err != nil {
				t.Fatalf("withdrawal writer: %v", err)
			}
			if !bytes.Equal(section[:n], want) {
				t.Fatalf("withdrawal:\n got %X\nwant %X", section[:n], want)
			}
			// The withdrawal frames under the family's withdrawal splitter.
			count, err := splitVPN(section[:n], tt.addPath, nil)
			if err != nil {
				t.Fatalf("withdrawal does not frame: %v", err)
			}
			if count != routes {
				t.Fatalf("withdrawal framed %d routes, want %d", count, routes)
			}
			// Each withdrawal names the route its announcement named, so an
			// Adj-RIB-Out keyed by route identity finds it.
			withdrawn := routeKeys(t, splitVPN, section[:n], tt.addPath, true)
			for i := range announced {
				if announced[i] != withdrawn[i] {
					t.Fatalf("route %d: announced key %X, withdrawn key %X", i, announced[i], withdrawn[i])
				}
			}
		})
	}
}

// TestLabeledWithdrawalRefusesMalformed checks a section whose label stack or
// prefix runs past its end is refused, and that the writer never modifies its
// source field, so a caller that discards the refused destination holds the
// section byte for byte as it was. Method: one well-formed route followed by a
// truncated one, written into a separate destination.
func TestLabeledWithdrawalRefusesMalformed(t *testing.T) {
	for _, in := range []string{
		"38" + "000010" + "003E81" + "0A" + "38" + "000010",
		"20" + "003E81" + "0A" + "40" + "003E81",
	} {
		section, err := hex.DecodeString(in)
		if err != nil {
			t.Fatal(err)
		}
		before := bytes.Clone(section)
		dst := make([]byte, len(section))
		writer := GetWithdrawalWriter(family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIMPLSLabel})
		if _, err := writer(dst, section, false, false); err == nil {
			t.Fatalf("%s: want an error", in)
		}
		if !bytes.Equal(section, before) {
			t.Fatalf("%s: section changed to %X", in, section)
		}
	}
}

// routeKeys frames section with split and returns each route's identity
// (keyLabeled), the ADD-PATH identifier kept in front of it.
func routeKeys(t *testing.T, split Splitter, section []byte, addPath, withdraw bool) []string {
	t.Helper()
	head := 0
	if addPath {
		head = 4
	}
	var keys []string
	var keyErr error
	_, err := split(section, addPath, func(route []byte) bool {
		scratch := make([]byte, PrefixKeyScratchSize)
		key, err := keyLabeled(route[head:], scratch, withdraw)
		if err != nil {
			keyErr = err
			return false
		}
		keys = append(keys, string(route[:head])+string(key))
		return true
	})
	if err != nil {
		t.Fatal(err)
	}
	if keyErr != nil {
		t.Fatal(keyErr)
	}
	return keys
}
