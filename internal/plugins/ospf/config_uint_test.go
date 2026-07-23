package ospf

import (
	"os"
	"strings"
	"testing"
)

// VALIDATES: AC-13 -- a config value above the target type's range is refused by
// configUint8/16/32, so the caller keeps its default rather than storing a
// truncated value.
// PREVENTS: The bare `uintN(v)` this replaced, which turned an out-of-range leaf
// into a different, valid-looking setting (4294967296 -> 0). The config file
// parser already rejects such values against the leaf's YANG type, but relying on
// a guard three layers up fails OPEN for any entry point that skips it
// (ai/rules/fail-closed-guards.md).
func TestConfigUintRejectsAboveMax(t *testing.T) {
	t.Run("uint8", func(t *testing.T) {
		tests := []struct {
			name string
			in   any
			want uint8
			ok   bool
		}{
			{"zero", "0", 0, true},
			{"last valid", "255", 255, true},
			{"first invalid above", "256", 0, false},
			{"far above", "4294967296", 0, false},
			{"max uint64", "18446744073709551615", 0, false},
			{"negative", "-1", 0, false},
			{"not a number", "many", 0, false},
			{"json number", float64(42), 42, true},
			{"json number above", float64(300), 0, false},
		}
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				got, ok := configUint8(tc.in)
				if ok != tc.ok || got != tc.want {
					t.Errorf("configUint8(%v) = (%d, %t), want (%d, %t)", tc.in, got, ok, tc.want, tc.ok)
				}
			})
		}
	})

	t.Run("uint16", func(t *testing.T) {
		tests := []struct {
			name string
			in   any
			want uint16
			ok   bool
		}{
			{"last valid", "65535", 65535, true},
			{"first invalid above", "65536", 0, false},
			{"far above", "4294967296", 0, false},
		}
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				got, ok := configUint16(tc.in)
				if ok != tc.ok || got != tc.want {
					t.Errorf("configUint16(%v) = (%d, %t), want (%d, %t)", tc.in, got, ok, tc.want, tc.ok)
				}
			})
		}
	})

	t.Run("uint32", func(t *testing.T) {
		tests := []struct {
			name string
			in   any
			want uint32
			ok   bool
		}{
			{"last valid", "4294967295", 4294967295, true},
			{"first invalid above", "4294967296", 0, false},
			{"max uint64", "18446744073709551615", 0, false},
		}
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				got, ok := configUint32(tc.in)
				if ok != tc.ok || got != tc.want {
					t.Errorf("configUint32(%v) = (%d, %t), want (%d, %t)", tc.in, got, ok, tc.want, tc.ok)
				}
			})
		}
	})
}

// VALIDATES: AC-13 -- an out-of-range leaf leaves the parsed field at its default
// instead of storing a wrapped value.
// PREVENTS: A truncating narrow silently redefining a setting: "256" for an
// 8-bit leaf must not become 0, which is a different and often dangerous value
// (0 means "ineligible" for priority, "unlimited" for several caps).
func TestOutOfRangeLeafKeepsDefault(t *testing.T) {
	cfg := defaultOSPFConfig()
	want := cfg.MaximumPaths
	if want == 0 {
		t.Fatal("test premise broken: default MaximumPaths is 0, so a wrap would be invisible")
	}

	// One above the uint8 range, exactly the case the old `uint8(v)` wrapped to 0.
	if v, ok := configUint8("256"); ok {
		cfg.MaximumPaths = v
	}

	if cfg.MaximumPaths != want {
		t.Errorf("MaximumPaths = %d after an out-of-range leaf, want the default %d", cfg.MaximumPaths, want)
	}
}

// VALIDATES: every YANG leaf feeding a configUintN call declares a range no
// wider than the Go type it feeds, which is the invariant that makes the
// mechanical narrowing safe.
// PREVENTS: The srms-preference case. YANG `uint8` maps to schema TypeUint16
// (internal/component/config/yang_schema.go), so a uint8 leaf with NO explicit
// range accepts 256..65535 at parse time. configUint8 then returns ok=false and
// the caller keeps its default -- for srms-preference that meant HasSRMS stayed
// false and the SRMS-Preference TLV was silently never advertised, off a value
// the parser had accepted.
func TestNarrowedLeavesDeclareARange(t *testing.T) {
	data, err := os.ReadFile("yang/ze-ospf-conf.yang")
	if err != nil {
		t.Fatalf("read yang: %v", err)
	}
	body := string(data)

	// Every leaf consumed by configUint8 must bound itself at or below 255.
	for _, leaf := range []string{"srms-preference", "maximum-paths", "priority"} {
		idx := strings.Index(body, "leaf "+leaf+" {")
		if idx < 0 {
			t.Errorf("leaf %q not found in the OSPF schema", leaf)
			continue
		}
		decl := body[idx:min(idx+220, len(body))]
		if !strings.Contains(decl, "range") {
			t.Errorf("leaf %q declares no range: a bare uint8 leaf accepts 256..65535 at parse "+
				"time, so configUint8 would reject a parser-accepted value and silently keep the "+
				"default:\n%s", leaf, decl)
		}
	}
}
