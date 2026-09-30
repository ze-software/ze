package message

import (
	"bytes"
	"strings"
	"testing"
	"unicode/utf8"
)

// TestRFC8203ShutdownCommunicationOnlyUnderSubcode2Or4 proves the sender side of
// the subcode rule: the Cease data Ze builds carries the Shutdown Communication
// under subcodes 2 and 4, and nothing under any other Cease subcode.
//
// VALIDATES: BuildCeaseData carries the message only under subcodes 2 and 4.
// PREVENTS: a teardown with another Cease subcode sending a Shutdown Communication.
//
// Method: BuildCeaseData is the one builder the session teardown uses for the
// Cease NOTIFICATION data. Each subcode 0..10 is built with the same message; 2
// and 4 must carry the length-prefixed message, every other one no data.
//
// RFC requirement: RFC8203-2-1 positive -- under subcode 2 (Administrative Shutdown) and 4
// (Administrative Reset) the Cease data is the length octet followed by the message.
// RFC requirement: RFC8203-2-1 negative -- under subcodes 0, 1, 3 and 5..10 the Cease data is
// empty, so no Shutdown Communication is sent under a subcode other than 2 or 4.
// RFC requirement: RFC9003-2-3 positive -- under subcode 2 and 4 the Cease data is the length
// octet followed by the message.
// RFC requirement: RFC9003-2-3 negative -- under subcodes 0, 1, 3 and 5..10 the Cease data is
// empty, so no Shutdown Communication is sent under a subcode other than 2 or 4.
func TestRFC8203ShutdownCommunicationOnlyUnderSubcode2Or4(t *testing.T) {
	const msg = "maintenance"
	for subcode := range uint8(11) {
		data := BuildCeaseData(subcode, msg)
		allowed := subcode == NotifyCeaseAdminShutdown || subcode == NotifyCeaseAdminReset
		if !allowed {
			if len(data) != 0 {
				t.Errorf("subcode %d: Cease data %x carries a Shutdown Communication", subcode, data)
			}
			continue
		}
		want := append([]byte{byte(len(msg))}, msg...)
		if !bytes.Equal(data, want) {
			t.Errorf("subcode %d: Cease data %x, want %x", subcode, data, want)
		}
	}
}

// TestRFC8203ShutdownCommunicationSentAsShortestFormUTF8 proves the sender side
// of the encoding rules: the Shutdown Communication Ze puts on the wire is
// UTF-8 in shortest form, whatever string the operator supplied.
//
// VALIDATES: BuildShutdownData sends valid shortest-form UTF-8 only.
// PREVENTS: invalid, overlong or cut multibyte octets reaching the peer.
//
// Method: a valid multibyte message must be sent octet for octet. Invalid input
// (stray bytes, an overlong 0xC0 0xAF, a surrogate half) and a multibyte message
// whose 128-octet cut lands inside a character must still produce a field that
// utf8.Valid accepts (Go's decoder refuses overlong and surrogate forms, so
// valid means shortest form), with the offending octets gone.
//
// RFC requirement: RFC8203-2-3 positive -- a valid UTF-8 message with 2-, 3- and 4-octet
// characters is sent unchanged, length octet then the exact UTF-8 octets.
// RFC requirement: RFC8203-2-3 negative -- invalid UTF-8 input, and a 128-octet cut inside a
// multibyte character, still produce a field that is valid UTF-8.
// RFC requirement: RFC8203-6-1 positive -- the shortest-form encoding of U+00E9 (0xC3 0xA9) is
// sent unchanged.
// RFC requirement: RFC8203-6-1 negative -- an overlong 0xC0 0xAF and a surrogate encoding
// 0xED 0xA0 0x80 in the input never reach the field; the field is shortest-form UTF-8.
func TestRFC8203ShutdownCommunicationSentAsShortestFormUTF8(t *testing.T) {
	const valid = "café € \U0001F600"
	data := BuildShutdownData(valid)
	want := append([]byte{byte(len(valid))}, valid...)
	if !bytes.Equal(data, want) {
		t.Fatalf("valid UTF-8: field %x, want %x", data, want)
	}

	cases := map[string]string{
		"stray octets": "a\xff\xfeb",
		"overlong":     "x\xc0\xafy",
		"surrogate":    "x\xed\xa0\x80y",
		"cut in char":  strings.Repeat("a", 127) + "éé",
	}
	for name, input := range cases {
		field := BuildShutdownData(input)
		if int(field[0]) != len(field)-1 {
			t.Errorf("%s: length octet %d, field holds %d octets", name, field[0], len(field)-1)
			continue
		}
		text := field[1:]
		if !utf8.Valid(text) {
			t.Errorf("%s: sent %x, not shortest-form UTF-8", name, text)
		}
		if len(text) > MaxShutdownMessageLen {
			t.Errorf("%s: sent %d octets, above %d", name, len(text), MaxShutdownMessageLen)
		}
	}
	if got := string(BuildShutdownData(cases["overlong"])[1:]); got != "xy" {
		t.Errorf("overlong: sent %q, want %q", got, "xy")
	}
}
