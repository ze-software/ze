package cli

import (
	"strings"
	"testing"
)

// TestRecordOpenASRefusesWhatItCannotRead covers the server-mode AS read.
//
// VALIDATES: a record's `asn` value that is not a number, and AS 0, each fail the
// run naming the test and the value; a real AS becomes one unkeyed declaration;
// no `asn` at all declares nothing.
// PREVENTS: the silent drop the .ci path already closed being left open here. A
// dropped value left ze-peer opening with ze's own AS, so `--server` on a test
// naming another AS could only ever answer NOTIFICATION 2/2 Bad Peer AS.
func TestRecordOpenASRefusesWhatItCannotRead(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  uint32
		names string
	}{
		{name: "absent", value: ""},
		{name: "an AS", value: "65001", want: 65001},
		{name: "largest AS", value: "4294967295", want: 4294967295},
		{name: "not a number", value: "sixty-five thousand", names: `"sixty-five thousand" is not a number`},
		{name: "above 32 bits", value: "4294967296", names: `"4294967296" is not a number`},
		{name: "zero", value: "0", names: "asn 0 is reserved"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := recordOpenAS("example", tt.value)
			if tt.names != "" {
				if err == nil {
					t.Fatalf("asn %q must fail the run, got %v", tt.value, got)
				}
				if !strings.Contains(err.Error(), tt.names) {
					t.Errorf("error = %q, want it to name %q", err, tt.names)
				}
				if !strings.Contains(err.Error(), "example") {
					t.Errorf("error = %q, want it to name the test", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("asn %q: %v", tt.value, err)
			}
			if tt.want == 0 {
				if len(got) != 0 {
					t.Errorf("asn %q declared %v, want nothing", tt.value, got)
				}
				return
			}
			if len(got) != 1 || got[0].AS != tt.want || got[0].Addr.IsValid() {
				t.Errorf("asn %q = %v, want one unkeyed declaration of %d", tt.value, got, tt.want)
			}
		})
	}
}
