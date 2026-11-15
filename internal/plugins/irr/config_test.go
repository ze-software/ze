package irr

import (
	"testing"
	"time"

	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestParseOwnerConfig proves the owner reads system irr and system peeringdb
// url into exactly the config it then runs: the defaults when a leaf or the
// whole section is absent, each explicit value, and the refresh interval in
// both forms a delivery can carry (a JSON string and a JSON number).
// Method: parseOwnerConfig over hand-built sections, comparing the whole
// ownerConfig value and the present flag.
// VALIDATES: the settings the consumers no longer parse reach the owner with
// their values (spec "What the operator sees": set once under system irr).
// PREVENTS: a numeric refresh-interval silently read as 0 (manual) or as the
// default, and an explicit server or PeeringDB URL dropped for the default.
func TestParseOwnerConfig(t *testing.T) {
	system := func(data string) []sdk.ConfigSection {
		return []sdk.ConfigSection{{Root: configRootSystem, Data: data}}
	}
	cases := []struct {
		name     string
		sections []sdk.ConfigSection
		want     ownerConfig
		present  bool
	}{
		{name: "no section", sections: nil, want: defaultOwnerConfig(), present: false},
		{name: "system without irr or peeringdb", sections: system(`{"system":{"host-name":"r1"}}`), want: defaultOwnerConfig(), present: true},
		{
			name:     "explicit server and source address",
			sections: system(`{"system":{"irr":{"server":"whois.example.net","source-address":"192.0.2.1"}}}`),
			want:     ownerConfig{server: "whois.example.net", sourceAddress: "192.0.2.1", refreshInterval: defaultRefreshInterval, peeringDBURL: defaultPeeringDBURL},
			present:  true,
		},
		{
			name:     "refresh interval as a string",
			sections: system(`{"system":{"irr":{"refresh-interval":"1800"}}}`),
			want:     ownerConfig{server: defaultServer, refreshInterval: 1800 * time.Second, peeringDBURL: defaultPeeringDBURL},
			present:  true,
		},
		{
			name:     "refresh interval as a JSON number",
			sections: system(`{"system":{"irr":{"refresh-interval":7200}}}`),
			want:     ownerConfig{server: defaultServer, refreshInterval: 7200 * time.Second, peeringDBURL: defaultPeeringDBURL},
			present:  true,
		},
		{
			name:     "refresh interval 0 is manual",
			sections: system(`{"system":{"irr":{"refresh-interval":"0"}}}`),
			want:     ownerConfig{server: defaultServer, refreshInterval: 0, peeringDBURL: defaultPeeringDBURL},
			present:  true,
		},
		{
			name:     "explicit peeringdb url",
			sections: system(`{"system":{"peeringdb":{"url":"https://pdb.example.net"}}}`),
			want:     ownerConfig{server: defaultServer, refreshInterval: defaultRefreshInterval, peeringDBURL: "https://pdb.example.net"},
			present:  true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, present, err := parseOwnerConfig(tc.sections)
			if err != nil {
				t.Fatalf("parseOwnerConfig: %v", err)
			}
			if present != tc.present {
				t.Errorf("present = %v, want %v", present, tc.present)
			}
			if got != tc.want {
				t.Errorf("config = %+v, want %+v", got, tc.want)
			}
		})
	}
	// The defaults by value, not by name: a changed constant is a changed
	// default every box without a system irr or peeringdb leaf runs.
	literal := ownerConfig{server: "whois.radb.net", refreshInterval: 3600 * time.Second, peeringDBURL: "https://www.peeringdb.com"}
	if got := defaultOwnerConfig(); got != literal {
		t.Errorf("defaults = %+v, want %+v", got, literal)
	}
	if !(ownerConfig{refreshInterval: 0}).manual() {
		t.Error("refresh-interval 0 is not reported manual")
	}
}
