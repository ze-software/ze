// Design: docs/architecture/testing/interop.md -- FRR raw origination decode evidence.
// Related: check_raw_originated_next_hop.go -- the live receiver oracle.
package bgp

import "testing"

// TestRawOriginatedAnnouncementDecoded separates foreign receive evidence from
// withdrawal, another peer, another prefix and a different log event.
func TestRawOriginatedAnnouncementDecoded(t *testing.T) {
	const peer = "172.30.0.2"
	const prefix = "10.92.1.0/24"
	for _, tc := range []struct {
		name string
		line string
		want bool
	}{
		{"accepted", "BGP: 172.30.0.2(Unknown) rcvd 10.92.1.0/24 IPv4 unicast", true},
		{"denied", "BGP: 172.30.0.2 rcvd UPDATE about 10.92.1.0/24 IPv4 unicast -- DENIED due to: martian or self next-hop;", true},
		{"withdrawn", "BGP: 172.30.0.2 rcvd UPDATE about 10.92.1.0/24 IPv4 unicast -- withdrawn", false},
		{"different peer", "BGP: 172.30.0.22 rcvd 10.92.1.0/24 IPv4 unicast", false},
		{"different prefix", "BGP: 172.30.0.2 rcvd 110.92.1.0/24 IPv4 unicast", false},
		{"sent", "BGP: 172.30.0.2 send 10.92.1.0/24 IPv4 unicast", false},
		{"separate lines", "BGP: 172.30.0.2 rcvd another prefix\nBGP: 10.92.1.0/24 IPv4 unicast", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := frrDecodedAnnouncement(tc.line, peer, prefix); got != tc.want {
				t.Fatalf("announcement decode=%v, want %v for %q", got, tc.want, tc.line)
			}
		})
	}
}

// TestFRRDecodedAnnouncementForPeerOwnRelay pins the relay subject separately:
// withdrawing it is legal, while an accepted or denied announcement reached FRR.
func TestFRRDecodedAnnouncementForPeerOwnRelay(t *testing.T) {
	const peer = "172.30.0.2"
	const prefix = "10.11.0.0/24"
	for _, tc := range []struct {
		name string
		line string
		want bool
	}{
		{"withdrawn", "BGP: 172.30.0.2 rcvd UPDATE about 10.11.0.0/24 IPv4 unicast -- withdrawn", false},
		{"accepted", "BGP: 172.30.0.2(Unknown) rcvd 10.11.0.0/24 IPv4 unicast", true},
		{"denied", "BGP: 172.30.0.2 rcvd UPDATE about 10.11.0.0/24 IPv4 unicast -- DENIED due to: martian or self next-hop;", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := frrDecodedAnnouncement(tc.line, peer, prefix); got != tc.want {
				t.Fatalf("relay announcement decode=%v, want %v for %q", got, tc.want, tc.line)
			}
		})
	}
}
