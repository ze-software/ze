// Design: docs/architecture/isis/isis-4-component-config.md -- IS-IS NET / system-id validators
//
// VALIDATES: TestISISNETValidator -- the isis-net custom validator accepts valid
// NETs and rejects bad hex / NSEL / length, including the 8..20-octet length
// boundary (ISO/IEC 10589 section 6.2). TestISISSystemIDValidator -- the
// isis-system-id validator accepts xxxx.xxxx.xxxx and rejects malformed forms.
package config

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestISISNETValidator(t *testing.T) {
	v := ISISNETValidator()
	require.NotNil(t, v.ValidateFn)

	valid := []string{
		"49.0001.0000.0000.0001.00",      // 1-octet area: 8 octets total (min)
		"49.0001.0002.0000.0000.0001.00", // 2-octet area
		"00.0000.0000.0000.0000.0000.00", // all-zero area
	}
	for _, s := range valid {
		if err := v.ValidateFn("isis/net", s); err != nil {
			t.Errorf("ISISNETValidator(%q) = %v, want nil", s, err)
		}
	}

	// test-relax: the prior "too short (7)" case used "49.0001.0000.0000.0001",
	// which is actually 9 octets (a VALID NET), so it tested the wrong thing. The
	// genuine below-min case (7 octets) is now covered exactly in the length
	// boundary table below via netOf(7).
	invalid := map[string]string{
		"non-hex":     "zz.0001.0000.0000.0001.00",
		"odd nibble":  "490.001.0000.0000.0001.00",
		"empty group": "49..0000.0000.0001.00",
		"empty":       "",
	}
	for name, s := range invalid {
		t.Run(name, func(t *testing.T) {
			if err := v.ValidateFn("isis/net", s); err == nil {
				t.Errorf("ISISNETValidator(%q) = nil, want error", s)
			}
		})
	}

	// Length boundary (ISO/IEC 10589 section 6.2: a NET is 8..20 octets). The
	// decoder counts whole octets across dot-separated groups, so an N-octet NET
	// can be written as a single 2N-hex-digit group. min 8 / max 20 are valid;
	// 7 (below) and 21 (above) are rejected.
	netOf := func(octets int) string { return strings.Repeat("00", octets) }
	for _, c := range []struct {
		octets int
		valid  bool
	}{
		{7, false},  // below min
		{8, true},   // min valid
		{20, true},  // max valid
		{21, false}, // above max
	} {
		s := netOf(c.octets)
		if n, err := isisDecodeNETLen(s); err != nil || n != c.octets {
			t.Fatalf("%d-octet NET decoded to %d octets, err=%v", c.octets, n, err)
		}
		err := v.ValidateFn("isis/net", s)
		if c.valid && err != nil {
			t.Errorf("ISISNETValidator(%d octets) = %v, want nil", c.octets, err)
		}
		if !c.valid && err == nil {
			t.Errorf("ISISNETValidator(%d octets) = nil, want error", c.octets)
		}
	}

	// Non-string input is rejected.
	if err := v.ValidateFn("isis/net", 49); err == nil {
		t.Error("ISISNETValidator(int) = nil, want type error")
	}
}

func TestISISSystemIDValidator(t *testing.T) {
	v := ISISSystemIDValidator()
	require.NotNil(t, v.ValidateFn)

	assert.NoError(t, v.ValidateFn("isis/system-id", "0000.0000.0001"))
	assert.NoError(t, v.ValidateFn("isis/system-id", "abcd.ef01.2345"))

	for name, s := range map[string]string{
		"too few groups":  "0000.0001",
		"too many groups": "0000.0000.0000.0001",
		"short group":     "000.0000.0001",
		"non-hex":         "zzzz.0000.0001",
		"colons":          "0000:0000:0001",
		"empty":           "",
	} {
		t.Run(name, func(t *testing.T) {
			if err := v.ValidateFn("isis/system-id", s); err == nil {
				t.Errorf("ISISSystemIDValidator(%q) = nil, want error", s)
			}
		})
	}
}

// ISISHostnameAccepted is the shared table of hostnames the config boundary
// accepts. One table drives the charset test, the label test and the emit-side
// test in internal/plugins/isis/lsdb, so the config rule and the wire cannot
// drift apart (spec-fixit-isis-hostname-ascii R-1).
var ISISHostnameAccepted = []string{
	"r1",                    // the shortest real fixture
	"r1-isis",               // an existing functional-test fixture
	"ze-p2p",                // an existing interop fixture
	"router-1.example.net",  // a multi-label FQDN
	"router-1.example.net.", // the absolute form, one trailing dot
	"core_1",                // an underscore: RFC 2181 sec 11 forbids restricting label characters
	"a router with spaces",  // RFC 5301 sec 3: "any string operators want to use"
	" ",                     // 0x20, the low boundary octet
	"~",                     // 0x7e, the high boundary octet
	// rfc-test-change-approved: Thomas, 2026-08-10. Removed a row asserting that a
	// 255-octet SINGLE label is accepted. RFC 2181 section 11 gives a label 1 to 63
	// octets, so the validator refuses it correctly, and this table already listed a
	// 64-octet label as refused: the two rows contradicted each other and no code
	// could satisfy both. The total-length boundary this row claimed to cover is
	// covered by the next row, which reaches 255 octets with conforming labels.
	strings.Repeat("a", 63),         // a 63-octet label, the last valid label length
	strings.Repeat("c.", 127) + "d", // 128 short labels: 255 octets, the last valid total
}

// ISISHostnameRefused is the shared table of hostnames the config boundary
// refuses, each with the reason the error must name.
var ISISHostnameRefused = map[string]string{
	"":                              "empty",
	"café.example":                  "utf-8 lead octet 0xc3",
	"r1\x00":                        "NUL, below 0x20",
	"r1\x1f":                        "0x1f, one below the low boundary",
	"r1\x7f":                        "0x7f, one above the high boundary",
	"r1\t":                          "tab, below 0x20",
	"r1\n":                          "newline, below 0x20",
	"ÿ":                             "a single non-ASCII rune",
	"a..b":                          "an empty interior label",
	".a":                            "an empty leading label",
	"a...":                          "more than one trailing dot",
	".":                             "the bare root: no label at all",
	strings.Repeat("a", 64):         "a 64-octet label, one above the limit",
	strings.Repeat("a", 64) + ".b":  "a 64-octet first label",
	"b." + strings.Repeat("a", 64):  "a 64-octet last label",
	strings.Repeat("b", 256):        "a 256-octet name, one above the limit",
	strings.Repeat("c.", 128) + "d": "257 octets with separators",
}

// VALIDATES: AC-1 and AC-4 -- the isis-hostname validator refuses every octet
// outside 0x20..0x7e and accepts every printable 7-bit ASCII name.
//
// RFC requirement: RFC5301-3-7 positive -- "The Value field is encoded in 7-bit
// ASCII" (RFC 5301 Section 3). A value carrying any octet outside printable
// 7-bit ASCII is refused at the config boundary, so it can never be emitted.
//
// RFC requirement: RFC5301-3-7 negative -- the rule refuses only what the RFC
// restricts. Every printable 7-bit ASCII name is accepted, the space and the
// tilde boundary octets included, so the leaf is not blanket-refused.
func TestISISHostnameValidatorCharset(t *testing.T) {
	v := ISISHostnameValidator()
	require.NotNil(t, v.ValidateFn)

	// Every octet in 0x20..0x7e is accepted as a one-character hostname.
	for c := 0x20; c <= 0x7e; c++ {
		if c == '.' {
			continue // a bare dot is the root; the label rule owns it
		}
		s := string(rune(c))
		if err := v.ValidateFn("isis/hostname", s); err != nil {
			t.Errorf("octet 0x%02x rejected: %v", c, err)
		}
	}

	// Every octet outside 0x20..0x7e is refused.
	// rfc-test-change-approved: Thomas, 2026-08-10. Loop form only, to clear the
	// `intrange` lint finding. Same bounds, same body, same assertions.
	for c := range 0x100 {
		if c >= 0x20 && c <= 0x7e {
			continue
		}
		s := "r" + string([]byte{byte(c)})
		if err := v.ValidateFn("isis/hostname", s); err == nil {
			t.Errorf("octet 0x%02x accepted, want refused", c)
		}
	}

	for _, s := range ISISHostnameAccepted {
		if err := v.ValidateFn("isis/hostname", s); err != nil {
			t.Errorf("ISISHostnameValidator(%q) = %v, want nil", s, err)
		}
	}

	// A non-string value fails closed rather than reading as acceptance.
	if err := v.ValidateFn("isis/hostname", 42); err == nil {
		t.Error("ISISHostnameValidator(int) = nil, want type error")
	}
}

// VALIDATES: AC-3 and AC-4 -- the label-structure rule from RFC 2181 section 11.
//
// RFC requirement: RFC5301-3-9 positive -- "The content of this value is a
// domain name, see [RFC2181]" (RFC 5301 Section 3). RFC 2181 Section 11 gives
// each label 1 to 63 octets and a full name at most 255 octets including the
// separators. A value that breaks either bound is refused.
//
// RFC requirement: RFC5301-3-9 negative -- RFC 2181 Section 11 also forbids
// restricting WHICH characters a label may carry, so the rule bounds lengths
// only. An underscore, a space and a single trailing dot stay accepted.
func TestISISHostnameValidatorLabels(t *testing.T) {
	v := ISISHostnameValidator()

	for s, why := range ISISHostnameRefused {
		if err := v.ValidateFn("isis/hostname", s); err == nil {
			t.Errorf("ISISHostnameValidator(%q) = nil, want refused (%s)", s, why)
		}
	}

	// The boundary rows, stated explicitly: 63 valid, 64 refused; 255 valid,
	// 256 refused.
	if err := v.ValidateFn("isis/hostname", strings.Repeat("a", 63)); err != nil {
		t.Errorf("63-octet label rejected: %v", err)
	}
	if err := v.ValidateFn("isis/hostname", strings.Repeat("a", 64)); err == nil {
		t.Error("64-octet label accepted, want refused")
	}
	// rfc-test-change-approved: Thomas, 2026-08-10. The accepted 255-octet case was
	// a single label of 255 octets, which RFC 2181 section 11 forbids. It now reaches
	// 255 octets with conforming labels, so the assertion tests the TOTAL-length
	// boundary it names instead of contradicting the 64-octet label row above.
	if err := v.ValidateFn("isis/hostname", strings.Repeat("c.", 127)+"d"); err != nil {
		t.Errorf("255-octet name rejected: %v", err)
	}
	if err := v.ValidateFn("isis/hostname", strings.Repeat("b", 256)); err == nil {
		t.Error("256-octet name accepted, want refused")
	}
}

// VALIDATES: AC-2 -- the error names the offending value, the offending octet or
// label with its position, and the accepted shape in words. No regex is offered
// as the remediation.
func TestISISHostnameValidatorMessage(t *testing.T) {
	v := ISISHostnameValidator()

	err := v.ValidateFn("isis/hostname", "café.example")
	require.Error(t, err)
	msg := err.Error()
	assert.Contains(t, msg, `"caf`, "the offending value must be quoted")
	assert.Contains(t, msg, "octet 4", "the offending octet position must be named")
	assert.Contains(t, msg, "0xc3", "the offending octet value must be named")
	assert.Contains(t, msg, "0x20", "the accepted shape must be stated")
	assert.Contains(t, msg, "0x7e", "the accepted shape must be stated")
	assert.Contains(t, msg, "isis/hostname", "the leaf path must be named")

	labelErr := v.ValidateFn("isis/hostname", "a.."+strings.Repeat("b", 3))
	require.Error(t, labelErr)
	assert.Contains(t, labelErr.Error(), "label 2", "the offending label must be named")
	assert.Contains(t, labelErr.Error(), "1 to 63 octets", "the label limit must be stated in words")

	longErr := v.ValidateFn("isis/hostname", strings.Repeat("b", 256))
	require.Error(t, longErr)
	assert.Contains(t, longErr.Error(), "256 octets", "the actual length must be named")
	assert.Contains(t, longErr.Error(), "255 octets", "the limit must be named")
}

// VALIDATES: AC-5 -- a Unicode hostname is REFUSED, not converted. Ze's
// configuring user-interface does not permit Unicode characters, so the
// antecedent of RFC 5301 section 3's IDNA sentence is false (Reading A).
//
// RFC requirement: RFC5301-3-10 positive -- "If a user-interface for configuring
// or displaying this field permits Unicode characters, that user-interface is
// responsible for applying the ToASCII and/or ToUnicode algorithm as described
// in [RFC3490]" (RFC 5301 Section 3). Ze's configuring user-interface refuses
// Unicode, so it never enters the conditional. The refusal itself is the proof,
// and no ToASCII output is ever produced.
func TestISISHostnameUnicodeRefusedNotConverted(t *testing.T) {
	v := ISISHostnameValidator()

	unicode := []string{
		"café.example", // Latin-1 supplement
		"münchen",      // umlaut
		"中文",           // CJK
		"ру",           // Cyrillic
		// rfc-test-change-approved: Thomas, 2026-08-10. Escape only, to clear the
		// staticcheck ST1018 finding. The same three octets reach the validator; the
		// literal is now readable instead of carrying an invisible character.
		"a\u200bb", // zero-width space
	}
	for _, s := range unicode {
		if err := v.ValidateFn("isis/hostname", s); err == nil {
			t.Errorf("ISISHostnameValidator(%q) = nil, want refused", s)
		}
	}

	// The punycode form of a refused name is NOT quietly substituted: it is
	// simply an ordinary ASCII name that an operator may configure by hand.
	// Nothing in Ze produces it, so it never appears in the accepted set as a
	// stand-in for the Unicode value.
	if err := v.ValidateFn("isis/hostname", "xn--caf-dma.example"); err != nil {
		t.Errorf("an operator-typed punycode name must validate on its own merits: %v", err)
	}
	for _, s := range ISISHostnameAccepted {
		if strings.HasPrefix(s, "xn--") {
			t.Errorf("accepted set carries a ToASCII form %q: Reading A does no conversion", s)
		}
	}
}
