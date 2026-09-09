// Design: docs/architecture/wire/nlri.md -- AS-number notation in MVPN text
package mvpn

import "testing"

// TestMVPNRDStringReadsEveryNotation proves this package's route distinguisher
// reader takes any of the three RFC 5396 spellings. It is the fourth of four
// RD readers in the tree, and they must not disagree.
//
// VALIDATES: rdStringToBytes reads asplain, asdot and asdot+ (AC-1).
// PREVENTS: an MVPN route refused for an RD the config file accepts.
func TestMVPNRDStringReadsEveryNotation(t *testing.T) {
	want, err := rdStringToBytes("65546:5")
	if err != nil {
		t.Fatalf("rdStringToBytes(65546:5): %v", err)
	}
	got, err := rdStringToBytes("1.10:5")
	if err != nil {
		t.Fatalf("rdStringToBytes(1.10:5): %v", err)
	}
	if got != want {
		t.Errorf("rdStringToBytes(1.10:5) = %v, want %v", got, want)
	}
	ipForm, err := rdStringToBytes("192.0.2.1:5")
	if err != nil {
		t.Fatalf("rdStringToBytes(192.0.2.1:5): %v", err)
	}
	if ipForm[1] != 1 {
		t.Errorf("RD type for 192.0.2.1:5 = %d, want 1", ipForm[1])
	}
	for _, bad := range []string{"1.99999:5", "1.2.3:5"} {
		if _, err := rdStringToBytes(bad); err == nil {
			t.Errorf("rdStringToBytes(%q) was accepted", bad)
		}
	}
}

// TestMVPNSourceASReadsEveryNotation proves the `source-as` word of an MVPN
// route takes the dotted spellings. RFC 6514 Section 4.3 carries a four-byte
// AS number in the Source AS route, so every spelling of one applies.
//
// VALIDATES: parseMVPNFields reads asplain, asdot and asdot+ (AC-1).
// PREVENTS: "mvpn source-as 1.10" being refused at the command.
func TestMVPNSourceASReadsEveryNotation(t *testing.T) {
	for _, spelling := range []string{"65546", "1.10"} {
		fields, err := parseMVPNFields([]string{"shared-join", "rd", "65000:1", "source", "10.0.0.1", "group", "232.0.0.1", "source-as", spelling}, false)
		if err != nil {
			t.Fatalf("parseMVPNFields(source-as %s): %v", spelling, err)
		}
		if fields.sourceAS != 65546 {
			t.Errorf("source-as %s = %d, want 65546", spelling, fields.sourceAS)
		}
	}
	if _, err := parseMVPNFields([]string{"shared-join", "rd", "65000:1", "source", "10.0.0.1", "group", "232.0.0.1", "source-as", "1.99999"}, false); err == nil {
		t.Error("parseMVPNFields accepted an out-of-range asdot AS number")
	}
}
