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
