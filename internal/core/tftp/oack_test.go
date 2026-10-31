// VALIDATES: ParseOACK's whole refusal set, each as an *OptionError the client
// answers with ERROR 8, and what a valid OACK grants. Method: one hand-built
// OACK per refusal, judged against the request the client sends.
// PREVENTS: a malformed or out-of-range OACK being used, or a refusal that
// the client cannot recognize as an option refusal and so answers without
// ERROR 8.
package tftp

import (
	"encoding/binary"
	"errors"
	"strings"
	"testing"
)

// testRequest is the request Ze's client sends: blksize 1468, tsize and
// windowsize 4.
var testRequest = Request{Blksize: BlksizeEthernet, Tsize: true, Windowsize: 4}

// oackPacket builds an OACK from raw fields, each followed by its NUL. A
// field ending in "!" is written without its NUL, to build a truncated packet.
func oackPacket(fields ...string) []byte {
	pkt := binary.BigEndian.AppendUint16(nil, OpOACK)
	for _, field := range fields {
		if unterminated, ok := strings.CutSuffix(field, "!"); ok {
			pkt = append(pkt, unterminated...)
			continue
		}
		pkt = append(pkt, field...)
		pkt = append(pkt, 0)
	}
	return pkt
}

func TestParseOACKGrants(t *testing.T) {
	granted, err := ParseOACK(oackPacket("BlkSize", "8", "tsize", "1073741824", "windowsize", "1"), testRequest)
	if err != nil {
		t.Fatalf("ParseOACK: %v", err)
	}
	want := Negotiated{Blksize: 8, Windowsize: 1, Tsize: 1073741824, HasTsize: true}
	if granted != want {
		t.Fatalf("granted %+v, want %+v", granted, want)
	}

	granted, err = ParseOACK(oackPacket("blksize", "1468", "windowsize", "4"), testRequest)
	if err != nil {
		t.Fatalf("ParseOACK at the upper bounds: %v", err)
	}
	want = Negotiated{Blksize: 1468, Windowsize: 4}
	if granted != want {
		t.Fatalf("granted %+v, want %+v", granted, want)
	}

	// An option the OACK leaves out keeps its RFC 1350 value.
	granted, err = ParseOACK(oackPacket(), testRequest)
	if err != nil {
		t.Fatalf("ParseOACK of an OACK with no option: %v", err)
	}
	if granted != Unnegotiated() {
		t.Fatalf("granted %+v, want %+v", granted, Unnegotiated())
	}
}

func TestParseOACKRefusals(t *testing.T) {
	refused := map[string]struct {
		pkt  []byte
		want string
	}{
		"truncated opcode":       {[]byte{0}, "shorter than its opcode"},
		"name not terminated":    {oackPacket("blksize!"), "name is not NUL-terminated"},
		"value not terminated":   {oackPacket("blksize", "512!"), "is not NUL-terminated"},
		"empty name":             {oackPacket("", "512"), "name is empty"},
		"empty value":            {oackPacket("blksize", ""), "empty value"},
		"blksize not decimal":    {oackPacket("blksize", "0x10"), "not a decimal number"},
		"blksize signed":         {oackPacket("blksize", "+512"), "not a decimal number"},
		"blksize below 8":        {oackPacket("blksize", "7"), "below the minimum of 8"},
		"blksize above request":  {oackPacket("blksize", "1469"), "above the 1468 requested"},
		"windowsize 0":           {oackPacket("windowsize", "0"), "below the minimum of 1"},
		"windowsize above":       {oackPacket("windowsize", "5"), "above the 4 requested"},
		"windowsize not decimal": {oackPacket("windowsize", "four"), "not a decimal number"},
		"tsize negative":         {oackPacket("tsize", "-1"), "not a decimal number"},
		"tsize overflows":        {oackPacket("tsize", "99999999999999999999"), "out of range"},
		"unrequested option":     {oackPacket("timeout", "5"), `"timeout" was not requested`},
		"option twice":           {oackPacket("tsize", "1", "TSIZE", "1"), "appears twice"},
	}
	for name, tc := range refused {
		t.Run(name, func(t *testing.T) {
			_, err := ParseOACK(tc.pkt, testRequest)
			if _, ok := errors.AsType[*OptionError](err); !ok {
				t.Fatalf("ParseOACK(%q) = %v, want an *OptionError", tc.pkt, err)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("ParseOACK(%q) = %v, want it to name %q", tc.pkt, err, tc.want)
			}
		})
	}
}

// TestParseOACKRefusesWhatWasNotRequested: an option the request left out is
// refused even when its value would be valid, because only the client may
// start a negotiation.
func TestParseOACKRefusesWhatWasNotRequested(t *testing.T) {
	cases := map[string][]byte{
		"blksize":    oackPacket("blksize", "512"),
		"tsize":      oackPacket("tsize", "1"),
		"windowsize": oackPacket("windowsize", "1"),
	}
	for name, pkt := range cases {
		_, err := ParseOACK(pkt, Request{})
		if _, ok := errors.AsType[*OptionError](err); !ok {
			t.Fatalf("%s with nothing requested: %v, want an *OptionError", name, err)
		}
		if !strings.Contains(err.Error(), "was not requested") {
			t.Fatalf("%s with nothing requested: %v", name, err)
		}
	}
}

// TestParseOACKNotAnOACK: a packet of another opcode is a caller defect, not
// an option refusal, so the client sends no ERROR 8 for it.
func TestParseOACKNotAnOACK(t *testing.T) {
	_, err := ParseOACK(binary.BigEndian.AppendUint16(nil, OpDATA), testRequest)
	if err == nil {
		t.Fatal("ParseOACK accepted a DATA packet")
	}
	if _, ok := errors.AsType[*OptionError](err); ok {
		t.Fatalf("a DATA packet was reported as an option refusal: %v", err)
	}
}

// TestParseDecimal reads option values the one way the client and the server
// both read them: plain ASCII digits only.
// VALIDATES: ParseDecimal accepts digits and refuses a sign, a space, an empty
// value and a number too large for int64, each as an *OptionError.
// PREVENTS: the server reading "+3" or " 4" as a number the client would refuse.
func TestParseDecimal(t *testing.T) {
	t.Parallel()

	for value, want := range map[string]int64{"0": 0, "1": 1, "255": 255, "065535": 65535} {
		got, err := ParseDecimal(OptionTimeout, value)
		if err != nil {
			t.Fatalf("ParseDecimal(%q): %v", value, err)
		}
		if got != want {
			t.Fatalf("ParseDecimal(%q) = %d, want %d", value, got, want)
		}
	}
	for _, value := range []string{"", "+3", "-1", " 4", "4 ", "0x10", "99999999999999999999"} {
		_, err := ParseDecimal(OptionTimeout, value)
		if _, ok := errors.AsType[*OptionError](err); !ok {
			t.Fatalf("ParseDecimal(%q) = %v, want an *OptionError", value, err)
		}
	}
}
