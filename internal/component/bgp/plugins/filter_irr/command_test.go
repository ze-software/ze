package filter_irr

import (
	"strings"
	"testing"
)

// VALIDATES: AC-11 -- `update bgp irr asn <asn>` rejects a value above the 32-bit
// ASN range instead of truncating it into a different ASN.
// PREVENTS: readUint parsing 64-bit and the handler narrowing to uint32, so
// "4294967296" would silently become ASN 0 and report "no IRR-filtered peer with
// ASN 0" -- an error about the wrong ASN, and a lookup the operator never asked for.
func TestUpdateASNRejectsOutOfRange(t *testing.T) {
	tests := []struct {
		name    string
		arg     string
		wantErr string
	}{
		{"zero", "0", "no IRR-filtered peer"},
		{"typical", "65001", "no IRR-filtered peer"},
		{"last valid", "4294967295", "no IRR-filtered peer"},
		{"first invalid above", "4294967296", "invalid ASN"},
		{"far above", "18446744073709551615", "invalid ASN"},
		{"negative", "-1", "invalid ASN"},
		{"not a number", "asn65001", "invalid ASN"},
		{"empty", "", "usage:"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			plug := &irrPlugin{}

			status, _, err := plug.updateASN([]string{tc.arg})

			if err == nil {
				t.Fatalf("updateASN(%q) returned no error, want %q", tc.arg, tc.wantErr)
			}
			if status != statusError {
				t.Errorf("updateASN(%q) status = %q, want %q", tc.arg, status, statusError)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("updateASN(%q) error = %q, want it to contain %q", tc.arg, err, tc.wantErr)
			}
			// An out-of-range value must never reach the ASN lookup: if it did, the
			// error would name a truncated ASN instead of rejecting the input.
			if tc.wantErr == "invalid ASN" && strings.Contains(err.Error(), "no IRR-filtered peer") {
				t.Errorf("updateASN(%q) truncated and looked up an ASN: %q", tc.arg, err)
			}
		})
	}
}
