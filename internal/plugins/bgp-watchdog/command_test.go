package bgp_watchdog

import (
	"testing"
)

// VALIDATES: Route definition → update text announce command string
// PREVENTS: Malformed commands rejected by engine text parser

func TestBuildAnnounceCmd(t *testing.T) {
	tests := []struct {
		name string
		b    cmdBuilder
		want string
	}{
		{
			name: "basic ipv4 unicast",
			b: cmdBuilder{
				origin:  "igp",
				nextHop: "10.0.0.1",
				family:  "ipv4/unicast",
				prefix:  "10.0.0.0/24",
			},
			want: "update text origin igp nhop 10.0.0.1 nlri ipv4/unicast add 10.0.0.0/24",
		},
		{
			name: "nhop self",
			b: cmdBuilder{
				origin:  "igp",
				nextHop: "self",
				family:  "ipv4/unicast",
				prefix:  "10.0.0.0/24",
			},
			want: "update text origin igp nhop self nlri ipv4/unicast add 10.0.0.0/24",
		},
		{
			name: "with local-preference and med",
			b: cmdBuilder{
				origin:    "igp",
				nextHop:   "10.0.0.1",
				localPref: 200,
				med:       100,
				family:    "ipv4/unicast",
				prefix:    "10.0.0.0/24",
			},
			want: "update text origin igp nhop 10.0.0.1 pref 200 med 100 nlri ipv4/unicast add 10.0.0.0/24",
		},
		{
			name: "with as-path",
			b: cmdBuilder{
				origin:  "igp",
				nextHop: "10.0.0.1",
				asPath:  []uint32{65001, 65002, 65003},
				family:  "ipv4/unicast",
				prefix:  "10.0.0.0/24",
			},
			want: "update text origin igp nhop 10.0.0.1 path 65001,65002,65003 nlri ipv4/unicast add 10.0.0.0/24",
		},
		{
			name: "with communities",
			b: cmdBuilder{
				origin:      "igp",
				nextHop:     "10.0.0.1",
				communities: []string{"65000:100", "65000:200"},
				family:      "ipv4/unicast",
				prefix:      "10.0.0.0/24",
			},
			want: "update text origin igp nhop 10.0.0.1 s-com 65000:100,65000:200 nlri ipv4/unicast add 10.0.0.0/24",
		},
		{
			name: "with large communities",
			b: cmdBuilder{
				origin:           "igp",
				nextHop:          "10.0.0.1",
				largeCommunities: []string{"65000:1:2", "65001:3:4"},
				family:           "ipv4/unicast",
				prefix:           "10.0.0.0/24",
			},
			want: "update text origin igp nhop 10.0.0.1 l-com 65000:1:2,65001:3:4 nlri ipv4/unicast add 10.0.0.0/24",
		},
		{
			name: "with extended communities",
			b: cmdBuilder{
				origin:         "igp",
				nextHop:        "10.0.0.1",
				extCommunities: []string{"target:65000:100"},
				family:         "ipv4/unicast",
				prefix:         "10.0.0.0/24",
			},
			want: "update text origin igp nhop 10.0.0.1 e-com target:65000:100 nlri ipv4/unicast add 10.0.0.0/24",
		},
		{
			name: "ipv6 unicast",
			b: cmdBuilder{
				origin:  "igp",
				nextHop: "2001:db8::1",
				family:  "ipv6/unicast",
				prefix:  "2001:db8:1::/48",
			},
			want: "update text origin igp nhop 2001:db8::1 nlri ipv6/unicast add 2001:db8:1::/48",
		},
		{
			name: "with path-id (ADD-PATH)",
			b: cmdBuilder{
				origin:  "igp",
				nextHop: "10.0.0.1",
				pathID:  42,
				family:  "ipv4/unicast",
				prefix:  "10.0.0.0/24",
			},
			want: "update text origin igp nhop 10.0.0.1 nlri ipv4/unicast info 42 add 10.0.0.0/24",
		},
		{
			name: "vpn with rd and label",
			b: cmdBuilder{
				origin:  "igp",
				nextHop: "10.0.0.1",
				rd:      "65000:100",
				labels:  []uint32{1000},
				family:  "ipv4/vpn",
				prefix:  "10.0.0.0/24",
			},
			want: "update text origin igp nhop 10.0.0.1 nlri ipv4/vpn rd 65000:100 label 1000 add 10.0.0.0/24",
		},
		{
			name: "origin egp",
			b: cmdBuilder{
				origin:  "egp",
				nextHop: "10.0.0.1",
				family:  "ipv4/unicast",
				prefix:  "10.0.0.0/24",
			},
			want: "update text origin egp nhop 10.0.0.1 nlri ipv4/unicast add 10.0.0.0/24",
		},
		{
			name: "origin incomplete",
			b: cmdBuilder{
				origin:  "incomplete",
				nextHop: "10.0.0.1",
				family:  "ipv4/unicast",
				prefix:  "10.0.0.0/24",
			},
			want: "update text origin incomplete nhop 10.0.0.1 nlri ipv4/unicast add 10.0.0.0/24",
		},
		{
			name: "all attributes",
			b: cmdBuilder{
				origin:           "igp",
				nextHop:          "10.0.0.1",
				localPref:        200,
				med:              50,
				asPath:           []uint32{65001},
				communities:      []string{"65000:100"},
				largeCommunities: []string{"65000:1:2"},
				extCommunities:   []string{"target:65000:100"},
				pathID:           1,
				family:           "ipv4/unicast",
				prefix:           "10.0.0.0/24",
			},
			want: "update text origin igp nhop 10.0.0.1 pref 200 med 50 path 65001 s-com 65000:100 l-com 65000:1:2 e-com target:65000:100 nlri ipv4/unicast info 1 add 10.0.0.0/24",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.b.announce()
			if got != tt.want {
				t.Errorf("announce():\n  got  %q\n  want %q", got, tt.want)
			}
		})
	}
}

// VALIDATES: Route definition → update text withdraw command string
// PREVENTS: Withdrawal missing NLRI family or prefix

func TestBuildWithdrawCmd(t *testing.T) {
	tests := []struct {
		name string
		b    cmdBuilder
		want string
	}{
		{
			name: "basic ipv4 withdrawal",
			b: cmdBuilder{
				family: "ipv4/unicast",
				prefix: "10.0.0.0/24",
			},
			want: "update text nlri ipv4/unicast del 10.0.0.0/24",
		},
		{
			name: "ipv6 withdrawal",
			b: cmdBuilder{
				family: "ipv6/unicast",
				prefix: "2001:db8:1::/48",
			},
			want: "update text nlri ipv6/unicast del 2001:db8:1::/48",
		},
		{
			name: "withdrawal with path-id",
			b: cmdBuilder{
				pathID: 42,
				family: "ipv4/unicast",
				prefix: "10.0.0.0/24",
			},
			want: "update text nlri ipv4/unicast info 42 del 10.0.0.0/24",
		},
		{
			name: "vpn withdrawal with rd and label",
			b: cmdBuilder{
				rd:     "65000:100",
				labels: []uint32{1000},
				family: "ipv4/vpn",
				prefix: "10.0.0.0/24",
			},
			want: "update text nlri ipv4/vpn rd 65000:100 label 1000 del 10.0.0.0/24",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.b.withdraw()
			if got != tt.want {
				t.Errorf("withdraw():\n  got  %q\n  want %q", got, tt.want)
			}
		})
	}
}

// VALIDATES: Route key generation for pool entries
// PREVENTS: Key mismatches between announce and withdraw commands

func TestCmdBuilderRouteKey(t *testing.T) {
	tests := []struct {
		name string
		b    cmdBuilder
		want string
	}{
		{
			name: "simple prefix",
			b:    cmdBuilder{prefix: "10.0.0.0/24"},
			want: "10.0.0.0/24#0",
		},
		{
			name: "with path-id",
			b:    cmdBuilder{prefix: "10.0.0.0/24", pathID: 42},
			want: "10.0.0.0/24#42",
		},
		{
			name: "with rd",
			b:    cmdBuilder{prefix: "10.0.0.0/24", rd: "65000:100"},
			want: "65000:100:10.0.0.0/24#0",
		},
		{
			name: "with rd and path-id",
			b:    cmdBuilder{prefix: "10.0.0.0/24", rd: "65000:100", pathID: 5},
			want: "65000:100:10.0.0.0/24#5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.b.routeKey()
			if got != tt.want {
				t.Errorf("routeKey() = %q, want %q", got, tt.want)
			}
		})
	}
}
