package reactor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/family"
)

// RFC requirement: DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-7 positive -- the container receive-request knob and per-family override determine the limits encoded in OPEN.
// RFC requirement: DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-7 negative -- an omitted limit requests no bound, and disabled/refused families cannot advertise an orphan limit.
func TestConfiguredPathsLimitOpen(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name         string
		direction    string
		limit        string
		v4           map[string]any
		v6           map[string]any
		wantFamilies []family.Family
		wantLimits   map[family.Family]uint16
	}{
		{
			name: "default and per-family override", direction: "send/receive", limit: "10",
			v4:           map[string]any{"limit": "3"},
			wantFamilies: []family.Family{family.IPv4Unicast, family.IPv6Unicast},
			wantLimits:   map[family.Family]uint16{family.IPv4Unicast: 3, family.IPv6Unicast: 10},
		},
		{
			name: "no receive request", direction: "send/receive",
			wantFamilies: []family.Family{family.IPv4Unicast, family.IPv6Unicast},
		},
		{
			name: "disabled override", direction: "send/receive", limit: "10",
			v4:           map[string]any{"mode": "disable", "limit": "3"},
			wantFamilies: []family.Family{family.IPv6Unicast},
			wantLimits:   map[family.Family]uint16{family.IPv6Unicast: 10},
		},
		{
			name: "all disabled", direction: "send/receive", limit: "10",
			v4: map[string]any{"mode": "disable", "limit": "3"},
			v6: map[string]any{"mode": "disable", "limit": "4"},
		},
		{
			name: "refused beside required", direction: "send/receive", limit: "10",
			v4:           map[string]any{"mode": "refuse", "limit": "3"},
			v6:           map[string]any{"mode": "require", "limit": "4"},
			wantFamilies: []family.Family{family.IPv6Unicast},
			wantLimits:   map[family.Family]uint16{family.IPv6Unicast: 4},
		},
		{
			name: "all refused", direction: "send/receive", limit: "10",
			v4: map[string]any{"mode": "refuse", "limit": "3"},
			v6: map[string]any{"mode": "refuse", "limit": "4"},
		},
		{
			name: "limit without ADD-PATH direction", limit: "10",
			v4: map[string]any{"limit": "3"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			addPath := map[string]any{
				"direction": tc.direction,
				"family":    map[string]any{"ipv4/unicast": tc.v4, "ipv6/unicast": tc.v6},
			}
			if tc.limit != "" {
				addPath["limit"] = tc.limit
			}
			tree := map[string]any{
				"connection": map[string]any{"remote": map[string]any{"ip": "192.0.2.1"}, "local": map[string]any{"ip": "auto"}},
				"session": map[string]any{
					"asn": map[string]any{"remote": "65001"},
					"family": map[string]any{
						"ipv4/unicast": map[string]any{"prefix": map[string]any{"maximum": "100000"}},
						"ipv6/unicast": map[string]any{"prefix": map[string]any{"maximum": "100000"}},
					},
					"capability": map[string]any{"add-path": addPath},
				},
			}
			settings, err := parsePeerFromTree("peer1", tree, 65000, 0)
			require.NoError(t, err)
			open, err := NewSession(settings).buildOpen(settings, settings.Capabilities)
			require.NoError(t, err)
			caps, err := capability.ParseFromOptionalParams(open.OptionalParams, open.ExtendedParams)
			require.NoError(t, err)
			var gotFamilies []family.Family
			var gotLimits map[family.Family]uint16
			for _, cap := range caps {
				switch cap := cap.(type) {
				case *capability.AddPath:
					for _, entry := range cap.Families {
						gotFamilies = append(gotFamilies, family.Family{AFI: entry.AFI, SAFI: entry.SAFI})
					}
				case *capability.PathsLimit:
					gotLimits = make(map[family.Family]uint16)
					for _, entry := range cap.Entries {
						gotLimits[family.Family{AFI: entry.AFI, SAFI: entry.SAFI}] = entry.Limit
					}
				}
			}
			assert.ElementsMatch(t, tc.wantFamilies, gotFamilies, "advertised ADD-PATH families")
			assert.Equal(t, tc.wantLimits, gotLimits, "receive requests encoded in OPEN")
		})
	}
}
