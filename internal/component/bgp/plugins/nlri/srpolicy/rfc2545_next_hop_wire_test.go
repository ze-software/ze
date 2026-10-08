// Design: docs/architecture/update-building.md -- SR Policy producer next-hop bytes.

package srpolicy

import (
	"bytes"
	"errors"
	"net/netip"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// TestSRPolicyConfigNextHopWire feeds the actual config parser output into the
// generic plugin consumer and checks literal MP_REACH bytes under both NLRI AFIs.
//
// RFC requirement: RFC2545-3-1 positive -- the real SR Policy config parser
// and plugin builder retain the literal IPv6 global address under either AFI.
// RFC requirement: RFC2545-3-2 positive -- those IPv6 controls encode exactly
// sixteen next-hop octets; native IPv4 controls are a separate RFC9830 case.
func TestSRPolicyConfigNextHopWire(t *testing.T) {
	t.Parallel()
	for _, endpoint := range []string{"192.0.2.9", "2001:db8::9"} {
		for _, nextHop := range []string{"192.0.2.1", "2001:db8::1"} {
			t.Run(endpoint+"/"+nextHop, func(t *testing.T) {
				isIPv6 := netip.MustParseAddr(endpoint).Is6()
				pr, err := parseConfigRoute(registry.ConfigRouteRequest{
					Content: strings.Fields("distinguisher 7 color 100 endpoint " + endpoint + " preference 100"),
					NextHop: nextHop,
					IsIPv6:  isIPv6,
				})
				if err != nil {
					t.Fatal(err)
				}
				rawAttrs := make([][]byte, 0, len(pr.Attrs))
				for _, attr := range pr.Attrs {
					rawAttrs = append(rawAttrs, srPolicyAttrWire(attr.Flags, attr.Code, attr.Value))
				}
				ub := message.GetUpdateBuilder(65000, false, true, false)
				defer message.PutUpdateBuilder(ub)
				update := ub.BuildPlugin(message.PluginParams{
					SAFI:         73,
					IsIPv6:       pr.IsIPv6,
					NLRI:         pr.NLRI,
					NextHop:      netip.MustParseAddr(pr.NextHop),
					RawAttrs:     rawAttrs,
					MapV4NextHop: pr.MapV4NextHop,
				})
				checkSRPolicyNextHopWire(t, message.PackTo(update, nil), pr.NLRI, isIPv6, nextHop)
			})
		}
	}
}

// TestSRPolicyEncodeNextHopWire exercises the public route encoder without any
// Extended Next Hop capability and checks native IPv4 and global IPv6 controls.
//
// RFC requirement: RFC2545-3-1 positive -- the public SR Policy encoder
// retains the literal IPv6 global address under either policy AFI.
// RFC requirement: RFC2545-3-2 positive -- those IPv6 controls encode exactly
// sixteen next-hop octets independently of the policy AFI.
func TestSRPolicyEncodeNextHopWire(t *testing.T) {
	t.Parallel()
	for _, endpoint := range []string{"192.0.2.9", "2001:db8::9"} {
		for _, nextHop := range []string{"192.0.2.1", "2001:db8::1"} {
			t.Run(endpoint+"/"+nextHop, func(t *testing.T) {
				isIPv6 := netip.MustParseAddr(endpoint).Is6()
				familyName := "ipv4/sr-policy"
				if isIPv6 {
					familyName = "ipv6/sr-policy"
				}
				update, nlri, err := EncodeRoute("distinguisher 7 color 100 endpoint "+endpoint+
					" next-hop "+nextHop+" preference 100", familyName, 65000, false, true, false)
				if err != nil {
					t.Fatal(err)
				}
				checkSRPolicyNextHopWire(t, update, nlri, isIPv6, nextHop)
			})
		}
	}
}

// TestSRPolicyEncodeRejectsInvalidIPv6NextHop feeds invalid global-address roles
// through the public encoder and requires an error instead of malformed bytes.
//
// RFC requirement: RFC2545-3-1 negative -- the public SR Policy encoder
// rejects loopback, unspecified, multicast and mapped global-address roles
// with ErrUnicastNextHopUnusable and no returned UPDATE or NLRI.
func TestSRPolicyEncodeRejectsInvalidIPv6NextHop(t *testing.T) {
	t.Parallel()
	for _, endpoint := range []string{"192.0.2.9", "2001:db8::9"} {
		for _, nextHop := range []string{"::1", "::", "ff02::1", "::ffff:192.0.2.1"} {
			t.Run(endpoint+"/"+nextHop, func(t *testing.T) {
				familyName := "ipv4/sr-policy"
				if netip.MustParseAddr(endpoint).Is6() {
					familyName = "ipv6/sr-policy"
				}
				update, nlri, err := EncodeRoute("distinguisher 7 color 100 endpoint "+endpoint+
					" next-hop "+nextHop+" preference 100", familyName, 65000, false, true, false)
				if !errors.Is(err, message.ErrUnicastNextHopUnusable) {
					t.Fatalf("error = %v, want unusable next hop; UPDATE = %x", err, update)
				}
				if len(update) != 0 {
					t.Errorf("rejected next hop emitted UPDATE = %x", update)
				}
				if len(nlri) != 0 {
					t.Errorf("rejected next hop returned NLRI = %x", nlri)
				}
			})
		}
	}
}

// checkSRPolicyNextHopWire compares the actual attribute bytes with independent
// address, NLRI and Tunnel Encapsulation fixtures, not a second route encoding.
func checkSRPolicyNextHopWire(t *testing.T, update, nlri []byte, isIPv6 bool, nextHop string) {
	t.Helper()
	attrs := rfc9830PathAttributes(t, update)
	_, _, mp, found := attribute.AttrFind(attrs, attribute.AttrMPReachNLRI)
	if !found {
		t.Fatal("missing MP_REACH_NLRI")
	}
	wantAFI := byte(1)
	wantNLRI := []byte{96, 0, 0, 0, 7, 0, 0, 0, 100, 192, 0, 2, 9}
	if isIPv6 {
		wantAFI = 2
		wantNLRI = []byte{192, 0, 0, 0, 7, 0, 0, 0, 100,
			0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 9}
	}
	wantNextHop := []byte{192, 0, 2, 1}
	if nextHop == "2001:db8::1" {
		wantNextHop = []byte{0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}
	}
	if len(mp) < 5 {
		t.Fatalf("short MP_REACH: %x", mp)
	}
	if !bytes.Equal(mp[:3], []byte{0, wantAFI, 73}) {
		t.Errorf("MP family = %x, want 00 %02x 49", mp[:3], wantAFI)
	}
	if int(mp[3]) != len(wantNextHop) {
		t.Fatalf("next-hop length = %d, want %d; MP_REACH = %x", mp[3], len(wantNextHop), mp)
	}
	if len(mp) != 5+len(wantNextHop)+len(wantNLRI) {
		t.Fatalf("MP_REACH length = %d, want %d", len(mp), 5+len(wantNextHop)+len(wantNLRI))
	}
	if !bytes.Equal(mp[4:4+len(wantNextHop)], wantNextHop) {
		t.Errorf("next-hop = %x, want %x", mp[4:4+len(wantNextHop)], wantNextHop)
	}
	if mp[4+len(wantNextHop)] != 0 {
		t.Error("MP_REACH reserved octet is not zero")
	}
	if !bytes.Equal(mp[5+len(wantNextHop):], wantNLRI) {
		t.Errorf("wire NLRI = %x, want %x", mp[5+len(wantNextHop):], wantNLRI)
	}
	if !bytes.Equal(nlri, wantNLRI) {
		t.Errorf("producer NLRI = %x, want %x", nlri, wantNLRI)
	}
	_, _, tunnel, found := attribute.AttrFind(attrs, attribute.AttributeCode(23))
	if !found {
		t.Fatal("missing Tunnel Encapsulation attribute")
	}
	if !bytes.Equal(tunnel, []byte{0, 15, 0, 8, 12, 6, 0, 0, 0, 0, 0, 100}) {
		t.Errorf("Tunnel Encapsulation = %x, want preference 100 in SR Policy CP tunnel", tunnel)
	}
}
