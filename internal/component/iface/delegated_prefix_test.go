package iface

import (
	"testing"
)

// TestDelegatedPrefixVerifyDeterministic checks that the same config always
// yields the same verify error. The parser builds the ethernet list from a Go
// map, so its order changes between parses. Method: parse a two-unit duplicate
// afresh 100 times and require the identical message, naming the lexically
// first unit as the earlier claim, every time; do the same for two units that
// each name an unknown source, where the first refused must be lan0.
func TestDelegatedPrefixVerifyDeterministic(t *testing.T) {
	cases := []struct {
		name   string
		unit0  string
		unit1  string
		source bool
		want   string
	}{
		{
			name:   "duplicate subnet-id",
			unit0:  `{"pppoe0": {"subnet-id": "1"}}`,
			unit1:  `{"pppoe0": {"subnet-id": "1"}}`,
			source: true,
			want:   "interface lan0 unit 0 and interface lan1 unit 0 both take subnet-id 1 from delegated-prefix pppoe0, so they would share one /64; give each unit its own subnet-id",
		},
		{
			name:  "unknown source",
			unit0: `{"pppoe0": {"subnet-id": "1"}}`,
			unit1: `{"pppoe0": {"subnet-id": "2"}}`,
			want:  "interface lan0 unit 0: ipv6 delegated-prefix pppoe0: no pppoe-client of that name requests a delegated prefix, so the unit would never get a /64; set interface pppoe-client pppoe0 ipv6 dhcpv6 enabled true and ipv6 dhcpv6 pd length <48-64>, or name a pppoe-client that does",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for run := range 100 {
				cfg := mustParseIfaceJSON(t, delegatedUnitJSON(tc.unit0, tc.unit1))
				if tc.source {
					withPPPoESource(cfg, 56)
				}
				err := verifyDelegatedPrefixes(cfg)
				if err == nil {
					t.Fatalf("run %d: accepted, want %q", run, tc.want)
				}
				if err.Error() != tc.want {
					t.Fatalf("run %d: got %q, want %q", run, err, tc.want)
				}
			}
		})
	}
}
