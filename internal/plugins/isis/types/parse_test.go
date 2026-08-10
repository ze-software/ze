package types

import (
	"errors"
	"testing"
)

// VALIDATES: every Parse* constructor rejects wrong group count, odd nibbles,
// and bad separators with an explicit error (spec risk R-4, security
// input-validation row). No partial value leaks.
// PREVENTS: silent corruption of identifiers from malformed config/CLI strings.
func TestParseRejectsMalformed(t *testing.T) {
	type parseFn func(string) error
	wrap := func(f func(string) error) parseFn { return f }

	systemID := wrap(func(s string) error { _, err := ParseSystemID(s); return err })
	sourceID := wrap(func(s string) error { _, err := ParseSourceID(s); return err })
	lspID := wrap(func(s string) error { _, err := ParseLSPID(s); return err })
	net := wrap(func(s string) error { _, err := ParseNET(s); return err })

	cases := []struct {
		name string
		fn   parseFn
		in   string
	}{
		// SystemID: must be exactly three two-octet groups.
		{"systemid empty", systemID, ""},
		{"systemid short", systemID, "0001.0002"},
		{"systemid long", systemID, "0001.0002.0003.0004"},
		{"systemid odd nibble", systemID, "0001.0002.003"},
		{"systemid bad digit", systemID, "00g1.0002.0003"},
		{"systemid no separators", systemID, "000100020003"},
		{"systemid wrong grouping", systemID, "00.0100.0200.03"},

		// SourceID: SystemID + "." + one pseudonode octet.
		{"sourceid empty", sourceID, ""},
		{"sourceid missing pseudonode", sourceID, "0001.0002.0003"},
		{"sourceid odd pseudonode", sourceID, "0001.0002.0003.0"},
		{"sourceid bad pseudonode", sourceID, "0001.0002.0003.zz"},
		{"sourceid two-octet pseudonode", sourceID, "0001.0002.0003.0102"},

		// LSPID: SourceID + "-" + one LSP-number octet.
		{"lspid empty", lspID, ""},
		{"lspid no dash", lspID, "0001.0002.0003.00"},
		{"lspid empty number", lspID, "0001.0002.0003.00-"},
		{"lspid odd number", lspID, "0001.0002.0003.00-1"},
		{"lspid bad number", lspID, "0001.0002.0003.00-zz"},

		// NET: 8..20 octets, whole-octet groups.
		{"net empty", net, ""},
		{"net too short", net, "0000.0000.0001.00"},    // 7 octets
		{"net odd nibble", net, "49.0000.0000.000"},    // odd group
		{"net bad digit", net, "49.0000.0000.00zz.00"}, // bad digit
		{
			"net too long", net,
			"49.00.00.00.00.00.00.00.00.00.00.00.00.00.0000.0000.0001.00", // 21 octets
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := c.fn(c.in); err == nil {
				t.Errorf("expected error for %q, got nil", c.in)
			}
		})
	}
}

// VALIDATES: every numeric *FromBytes rejects wrong wire lengths before indexing
// (security input-validation row: no out-of-range slice on attacker lengths).
// PREVENTS: panics or partial values from malformed wire length fields.
func TestFromBytesRejectsWrongLength(t *testing.T) {
	type fromBytesFn func([]byte) error
	systemID := func(b []byte) error { _, err := SystemIDFromBytes(b); return err }
	sourceID := func(b []byte) error { _, err := SourceIDFromBytes(b); return err }
	lspID := func(b []byte) error { _, err := LSPIDFromBytes(b); return err }
	net := func(b []byte) error { _, err := nETFromBytes(b); return err }
	metric := func(b []byte) error { _, err := MetricFromBytes(b); return err }
	pmetric := func(b []byte) error { _, err := PrefixMetricFromBytes(b); return err }
	seq := func(b []byte) error { _, err := SequenceNumberFromBytes(b); return err }
	rl := func(b []byte) error { _, err := RemainingLifetimeFromBytes(b); return err }
	ht := func(b []byte) error { _, err := HoldingTimeFromBytes(b); return err }
	area := func(b []byte) error { _, err := AreaIDFromBytes(b); return err }

	cases := []struct {
		name string
		fn   fromBytesFn
		bad  []int
		good int
	}{
		{"SystemID", systemID, []int{0, 5, 7}, SystemIDLen},
		{"SourceID", sourceID, []int{0, 6, 8}, SourceIDLen},
		{"LSPID", lspID, []int{0, 7, 9}, LSPIDLen},
		{"NET", net, []int{0, MinNETLen - 1, MaxNETLen + 1}, MinNETLen},
		{"Metric", metric, []int{0, 2, 4}, MetricLen},
		{"PrefixMetric", pmetric, []int{0, 3, 5}, PrefixMetricLen},
		{"SequenceNumber", seq, []int{0, 3, 5}, SequenceNumberLen},
		{"RemainingLifetime", rl, []int{0, 1, 3}, LifetimeLen},
		{"HoldingTime", ht, []int{0, 1, 3}, LifetimeLen},
		{"AreaID", area, []int{0, MaxAreaIDLen + 1}, MinAreaIDLen},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			for _, l := range c.bad {
				if err := c.fn(make([]byte, l)); err == nil {
					t.Errorf("%s(len=%d) should error", c.name, l)
				} else if !errors.Is(err, ErrWrongLength) {
					t.Errorf("%s(len=%d) error = %v, want ErrWrongLength", c.name, l, err)
				}
			}
			if err := c.fn(make([]byte, c.good)); err != nil {
				t.Errorf("%s(len=%d) should succeed, got %v", c.name, c.good, err)
			}
		})
	}
}
