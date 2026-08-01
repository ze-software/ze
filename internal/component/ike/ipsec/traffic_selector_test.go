package ipsec

import (
	"errors"
	"net"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
)

func cidr(t *testing.T, s string) *net.IPNet {
	t.Helper()
	_, n, err := net.ParseCIDR(s)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return n
}

func cfgWith(peer SiteToSitePeer) *IPsecConfig {
	return &IPsecConfig{Peers: map[string]SiteToSitePeer{peer.Name: peer}}
}

// TestValidateTrafficSelectorsRejectsUnprogrammable drives exact-or-reject at commit time.
//
// VALIDATES: a selector the dataplane cannot program byte for byte is refused by
// ze config verify, with the offending value and the accepted alternatives in the message.
// PREVENTS: an operator's selector being silently approximated at install time, which is
// what makes the wire and the dataplane disagree.
func TestValidateTrafficSelectorsRejectsUnprogrammable(t *testing.T) {
	tests := []struct {
		name string
		peer SiteToSitePeer
		want string // substring the refusal must name
	}{
		{
			name: "opaque port has no dataplane encoding",
			peer: SiteToSitePeer{
				Name: "p", Mode: dataplane.ModeTunnel,
				TrafficSelectors: []TrafficSelectorPolicy{{
					Number: "1", Protocol: 6,
					LocalPrefix: cidr(t, "10.1.0.0/16"), LocalPort: PortSelector{Form: PortOpaque},
					RemotePrefix: cidr(t, "10.2.0.0/16"), RemotePort: AnyPort(),
				}},
			},
			want: "opaque",
		},
		{
			name: "a port needs a protocol that defines ports",
			peer: SiteToSitePeer{
				Name: "p", Mode: dataplane.ModeTunnel,
				TrafficSelectors: []TrafficSelectorPolicy{{
					Number: "1", Protocol: 0,
					LocalPrefix: cidr(t, "10.1.0.0/16"), LocalPort: PortSelector{Form: PortSingle, Port: 443},
					RemotePrefix: cidr(t, "10.2.0.0/16"), RemotePort: AnyPort(),
				}},
			},
			want: "needs a protocol",
		},
		{
			name: "transport mode needs a single host prefix",
			peer: SiteToSitePeer{
				Name: "p", Mode: dataplane.ModeTransport,
				TrafficSelectors: []TrafficSelectorPolicy{{
					Number:      "1",
					LocalPrefix: cidr(t, "10.1.0.0/16"), LocalPort: AnyPort(),
					RemotePrefix: cidr(t, "10.0.0.2/32"), RemotePort: AnyPort(),
				}},
			},
			want: "exactly one IP address",
		},
		{
			name: "transport mode and a vti binding cannot both apply",
			peer: SiteToSitePeer{
				Name: "p", Mode: dataplane.ModeTransport, VTIBind: "vti0",
			},
			want: "vti",
		},
		{
			name: "transport-required needs transport mode",
			peer: SiteToSitePeer{
				Name: "p", Mode: dataplane.ModeTunnel, TransportRequired: true,
			},
			want: "transport-required",
		},
		{
			name: "a selector needs both prefixes",
			peer: SiteToSitePeer{
				Name: "p", Mode: dataplane.ModeTunnel,
				TrafficSelectors: []TrafficSelectorPolicy{{
					Number: "1", LocalPort: AnyPort(), RemotePort: AnyPort(),
					RemotePrefix: cidr(t, "10.2.0.0/16"),
				}},
			},
			want: "local prefix is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := cfgWith(tt.peer).ValidateTrafficSelectors()
			if err == nil {
				t.Fatal("the config was accepted; it would be approximated at install time")
			}
			if !errors.Is(err, ErrTrafficSelectorPolicy) {
				t.Errorf("error %v does not wrap ErrTrafficSelectorPolicy", err)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("refusal %q does not name %q, so an operator cannot act on it", err, tt.want)
			}
		})
	}
}

// TestValidateTrafficSelectorsAcceptsProgrammable is the discriminator for the table
// above: the validator must accept what the dataplane CAN program, or it would reject
// every configuration and the rejections above would prove nothing.
//
// VALIDATES: an unconfigured peer, a prefix pair, a single port under TCP, and a
// transport-mode host pair all pass.
// PREVENTS: a blanket refusal masquerading as exact-or-reject.
func TestValidateTrafficSelectorsAcceptsProgrammable(t *testing.T) {
	tests := []struct {
		name string
		peer SiteToSitePeer
	}{
		{
			name: "an unconfigured peer keeps working",
			peer: SiteToSitePeer{Name: "p", Mode: dataplane.ModeTunnel},
		},
		{
			name: "a prefix pair with any ports",
			peer: SiteToSitePeer{
				Name: "p", Mode: dataplane.ModeTunnel,
				TrafficSelectors: []TrafficSelectorPolicy{{
					Number:      "1",
					LocalPrefix: cidr(t, "10.1.0.0/16"), LocalPort: AnyPort(),
					RemotePrefix: cidr(t, "10.2.0.0/16"), RemotePort: AnyPort(),
				}},
			},
		},
		{
			name: "one TCP port",
			peer: SiteToSitePeer{
				Name: "p", Mode: dataplane.ModeTunnel,
				TrafficSelectors: []TrafficSelectorPolicy{{
					Number: "1", Protocol: 6,
					LocalPrefix: cidr(t, "10.1.0.0/16"), LocalPort: PortSelector{Form: PortSingle, Port: 179},
					RemotePrefix: cidr(t, "10.2.0.0/16"), RemotePort: AnyPort(),
				}},
			},
		},
		{
			name: "transport mode between two hosts",
			peer: SiteToSitePeer{
				Name: "p", Mode: dataplane.ModeTransport, TransportRequired: true,
				TrafficSelectors: []TrafficSelectorPolicy{{
					Number:      "1",
					LocalPrefix: cidr(t, "10.0.0.1/32"), LocalPort: AnyPort(),
					RemotePrefix: cidr(t, "10.0.0.2/32"), RemotePort: AnyPort(),
				}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := cfgWith(tt.peer).ValidateTrafficSelectors(); err != nil {
				t.Errorf("a programmable configuration was refused: %v", err)
			}
		})
	}
}

// TestPortSelectorWireRoundTrip pins the three RFC 7296 Section 3.13.1 encodings and the
// refusal of every other range.
//
// VALIDATES: ANY, a single port and OPAQUE each map to their RFC octet pair and back; an
// arbitrary inclusive range has no form and is reported as such.
// PREVENTS: an unrepresentable range being read as ANY, which would widen a peer's
// proposal.
func TestPortSelectorWireRoundTrip(t *testing.T) {
	tests := []struct {
		name        string
		sel         PortSelector
		start, end  uint16
		roundTripOK bool
	}{
		{"any", PortSelector{Form: PortAny}, 0, 65535, true},
		{"single", PortSelector{Form: PortSingle, Port: 443}, 443, 443, true},
		{"opaque", PortSelector{Form: PortOpaque}, 65535, 0, true},
		{"lowest single port", PortSelector{Form: PortSingle, Port: 1}, 1, 1, true},
		{"highest single port", PortSelector{Form: PortSingle, Port: 65534}, 65534, 65534, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end := tt.sel.Wire()
			if start != tt.start || end != tt.end {
				t.Fatalf("Wire() = %d/%d, want %d/%d", start, end, tt.start, tt.end)
			}
			back, ok := PortSelectorFromWire(start, end)
			if ok != tt.roundTripOK {
				t.Fatalf("PortSelectorFromWire(%d,%d) ok = %v, want %v", start, end, ok, tt.roundTripOK)
			}
			if back != tt.sel {
				t.Errorf("round trip = %+v, want %+v", back, tt.sel)
			}
		})
	}

	// Boundary: an arbitrary inclusive range has no form, and is NOT read as ANY.
	for _, r := range [][2]uint16{{1024, 2048}, {0, 1}, {1, 65535}, {0, 65534}} {
		if got, ok := PortSelectorFromWire(r[0], r[1]); ok {
			t.Errorf("PortSelectorFromWire(%d,%d) = %+v accepted; an arbitrary range has no exact form and must be narrowed by the caller",
				r[0], r[1], got)
		}
	}
}
