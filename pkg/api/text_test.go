package api

import (
	"net/netip"
	"testing"
)

// TestFormatReceivedUpdate tests ExaBGP text format for received UPDATE messages.
//
// VALIDATES: Received UPDATE messages are formatted correctly for text encoder.
// Format: "neighbor <ip> receive update announced <prefix> next-hop <nh> <attrs>"
//
// PREVENTS: Process scripts not receiving expected UPDATE format, causing check test timeout.
func TestFormatReceivedUpdate(t *testing.T) {
	tests := []struct {
		name       string
		peerAddr   string
		prefix     string
		nextHop    string
		origin     string
		localPref  uint32
		wantOutput string
	}{
		{
			name:      "basic ipv4 announce",
			peerAddr:  "127.0.0.1",
			prefix:    "0.0.0.0/32",
			nextHop:   "127.0.0.1",
			origin:    "igp",
			localPref: 100,
			wantOutput: "neighbor 127.0.0.1 receive update start\n" +
				"neighbor 127.0.0.1 receive update announced 0.0.0.0/32 next-hop 127.0.0.1 origin igp local-preference 100\n" +
				"neighbor 127.0.0.1 receive update end\n",
		},
		{
			name:      "different prefix and nexthop",
			peerAddr:  "192.168.1.1",
			prefix:    "10.0.0.0/8",
			nextHop:   "192.168.1.1",
			origin:    "egp",
			localPref: 200,
			wantOutput: "neighbor 192.168.1.1 receive update start\n" +
				"neighbor 192.168.1.1 receive update announced 10.0.0.0/8 next-hop 192.168.1.1 origin egp local-preference 200\n" +
				"neighbor 192.168.1.1 receive update end\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			peerAddr := netip.MustParseAddr(tt.peerAddr)
			prefix := netip.MustParsePrefix(tt.prefix)
			nextHop := netip.MustParseAddr(tt.nextHop)

			route := ReceivedRoute{
				Prefix:          prefix,
				NextHop:         nextHop,
				Origin:          tt.origin,
				LocalPreference: tt.localPref,
			}

			got := FormatReceivedUpdate(peerAddr, []ReceivedRoute{route})
			if got != tt.wantOutput {
				t.Errorf("FormatReceivedUpdate() =\n%q\nwant:\n%q", got, tt.wantOutput)
			}
		})
	}
}

// TestFormatReceivedUpdateMultipleRoutes tests formatting multiple routes in one UPDATE.
//
// VALIDATES: Multiple routes in one UPDATE are formatted correctly.
//
// PREVENTS: Missing routes when UPDATE contains multiple NLRIs.
func TestFormatReceivedUpdateMultipleRoutes(t *testing.T) {
	peerAddr := netip.MustParseAddr("127.0.0.1")
	routes := []ReceivedRoute{
		{
			Prefix:          netip.MustParsePrefix("10.0.0.0/24"),
			NextHop:         netip.MustParseAddr("192.168.1.1"),
			Origin:          "igp",
			LocalPreference: 100,
		},
		{
			Prefix:          netip.MustParsePrefix("10.0.1.0/24"),
			NextHop:         netip.MustParseAddr("192.168.1.1"),
			Origin:          "igp",
			LocalPreference: 100,
		},
	}

	got := FormatReceivedUpdate(peerAddr, routes)
	want := "neighbor 127.0.0.1 receive update start\n" +
		"neighbor 127.0.0.1 receive update announced 10.0.0.0/24 next-hop 192.168.1.1 origin igp local-preference 100\n" +
		"neighbor 127.0.0.1 receive update announced 10.0.1.0/24 next-hop 192.168.1.1 origin igp local-preference 100\n" +
		"neighbor 127.0.0.1 receive update end\n"

	if got != want {
		t.Errorf("FormatReceivedUpdate() =\n%q\nwant:\n%q", got, want)
	}
}
