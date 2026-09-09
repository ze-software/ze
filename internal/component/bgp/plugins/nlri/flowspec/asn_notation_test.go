// Design: docs/architecture/wire/nlri.md -- AS-number notation in a flowspec RD
package flowspec

import "testing"

// TestFlowRDStringReadsEveryNotation proves this package's route distinguisher
// reader takes any of the three RFC 5396 spellings, and produces the bytes the
// decimal spelling produces.
//
// It is the third of four RD readers in the tree. They must not disagree: an
// operator whose config file was accepted must not have the same RD refused
// when the flowspec encoder reads it.
//
// VALIDATES: flowRDStringToBytes reads asplain, asdot and asdot+ (AC-1).
// PREVENTS: "invalid rd ASN 1.10" from the encoder for a value the config
// parser took.
func TestFlowRDStringReadsEveryNotation(t *testing.T) {
	want, err := flowRDStringToBytes("65546:5")
	if err != nil {
		t.Fatalf("flowRDStringToBytes(65546:5): %v", err)
	}
	got, err := flowRDStringToBytes("1.10:5")
	if err != nil {
		t.Fatalf("flowRDStringToBytes(1.10:5): %v", err)
	}
	if got != want {
		t.Errorf("flowRDStringToBytes(1.10:5) = %v, want %v", got, want)
	}

	// An asdot+ spelling of a two-byte AS number still yields RFC 4364 type 0.
	small, err := flowRDStringToBytes("0.100:5")
	if err != nil {
		t.Fatalf("flowRDStringToBytes(0.100:5): %v", err)
	}
	plain, err := flowRDStringToBytes("100:5")
	if err != nil {
		t.Fatalf("flowRDStringToBytes(100:5): %v", err)
	}
	if small != plain || small[1] != 0 {
		t.Errorf("flowRDStringToBytes(0.100:5) = %v, want %v with type 0", small, plain)
	}

	// The IPv4 form is decided by netip before the AS branch.
	ipForm, err := flowRDStringToBytes("192.0.2.1:5")
	if err != nil {
		t.Fatalf("flowRDStringToBytes(192.0.2.1:5): %v", err)
	}
	if ipForm[1] != 1 {
		t.Errorf("RD type for 192.0.2.1:5 = %d, want 1", ipForm[1])
	}

	for _, bad := range []string{"1.99999:5", "65536.0:5", "1.2.3:5"} {
		if _, err := flowRDStringToBytes(bad); err == nil {
			t.Errorf("flowRDStringToBytes(%q) was accepted", bad)
		}
	}
}
