// VALIDATES: spec-ospfv3-1-types -- the shared dotted-quad / uint32 parse helpers reject
// malformed and out-of-range input and append canonical decimal text without allocating
// per call.
// PREVENTS: a parser that accepts leading zeros, overlong octets, or out-of-range values.
package types

import "testing"

func TestOSPFv3ParseDottedQuad(t *testing.T) {
	v, err := parseDottedQuad("10.0.0.255")
	if err != nil {
		t.Fatalf("parseDottedQuad: %v", err)
	}
	if v != [4]byte{10, 0, 0, 255} {
		t.Errorf("parseDottedQuad = %v", v)
	}
	for _, bad := range []string{"", "1.2.3", "1.2.3.4.5", "1.2.3.256", "01.2.3.4", "1.2.3.", ".1.2.3", "a.b.c.d"} {
		if _, err := parseDottedQuad(bad); err == nil {
			t.Errorf("parseDottedQuad(%q) accepted, want error", bad)
		}
	}
}

func TestOSPFv3ParseUint32Decimal(t *testing.T) {
	v, err := parseUint32Decimal("4294967295")
	if err != nil || v != 0xffffffff {
		t.Fatalf("parseUint32Decimal max = %d, %v", v, err)
	}
	for _, bad := range []string{"", "01", "4294967296", "12a", "-1"} {
		if _, err := parseUint32Decimal(bad); err == nil {
			t.Errorf("parseUint32Decimal(%q) accepted, want error", bad)
		}
	}
}

func TestOSPFv3AppendDecimalByte(t *testing.T) {
	cases := map[byte]string{0: "0", 9: "9", 10: "10", 99: "99", 100: "100", 255: "255"}
	for v, want := range cases {
		if got := string(appendDecimalByte(nil, v)); got != want {
			t.Errorf("appendDecimalByte(%d) = %q, want %q", v, got, want)
		}
	}
}

func TestOSPFv3Compare4(t *testing.T) {
	if compare4([4]byte{1, 0, 0, 0}, [4]byte{1, 0, 0, 1}) != -1 {
		t.Error("compare4 less")
	}
	if compare4([4]byte{2}, [4]byte{1}) != 1 {
		t.Error("compare4 greater")
	}
	if compare4([4]byte{1, 2, 3, 4}, [4]byte{1, 2, 3, 4}) != 0 {
		t.Error("compare4 equal")
	}
}
