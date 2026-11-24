// Design: docs/architecture/update-building.md -- plugin-owned MUP IPv4-mapped IPv6 next-hop preservation
// Related: config.go -- parseConfigRoute sets MapV4NextHop for MUP config routes
// Related: encode.go -- EncodeRoute sets MapV4NextHop for MUP route commands

package mup

import (
	"bytes"
	"encoding/binary"
	"net/netip"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/family"
)

// TestMUPMappedNextHopPreserved checks that IPv6 MUP (AFI 2, SAFI 85) preserves
// 16-octet IPv4-mapped IPv6 (::ffff:<ipv4>) next-hop encoding and decoding
// alongside native IPv6 across the shared MPNextHopProfile contract, MUP config
// route parsing, EncodeRoute, and BuildPlugin without altering native-IPv4
// behavior for unmapped families such as SR Policy and BGP-LS.
//
// VALIDATES: AC-12 step 3a (spec-bgp-next-hop-auto-rewrites-for-ebgp) --
// attribute.MPNextHopProfile(2, 85) preserves explicit IPv4-to-mapped-IPv6
// selection (::ffff:<ipv4>), MUP config.go and encode.go pass MapV4NextHop=true
// into BuildPlugin to emit 16-octet mapped IPv4 and native IPv6 next hops in
// MP_REACH_NLRI without NEXT_HOP (code 3), ParseMPReachNLRI and ParseMUP
// round-trip both forms cleanly, and unmapped families retain native 4-octet
// IPv4 next hops.
// PREVENTS: unifying MP next-hop framing under MPNextHopProfile from dropping
// MUP's plugin-owned MapV4NextHop conversion or leaking mapped-IPv4 conversion
// into BGP-LS (16388/71, 16388/72) or SR Policy (2/73).
func TestMUPMappedNextHopPreserved(t *testing.T) {
	t.Parallel()

	ipv4 := netip.MustParseAddr("192.0.2.1")
	mapped := netip.MustParseAddr("::ffff:192.0.2.1")
	ipv6 := netip.MustParseAddr("2001:db8::1")
	wantMappedBytes := []byte{
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0xff, 0xff, 192, 0, 2, 1,
	}
	wantIPv6Bytes := []byte{
		0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1,
	}
	wantIPv4Bytes := []byte{192, 0, 2, 1}

	t.Run("profile_select_and_roundtrip", func(t *testing.T) {
		t.Parallel()

		profile := attribute.MPNextHopProfile(attribute.AFIIPv6, attribute.SAFI(SAFIMUP))
		if len(profile.Lengths) != 0 {
			t.Fatalf("MUP 2/85 profile Lengths = %v, want nil (plugin-owned intent, not core table)", profile.Lengths)
		}
		if profile.EncodeIPv4Mapped {
			t.Fatal("MUP 2/85 profile unexpectedly sets EncodeIPv4Mapped in core")
		}
		if !profile.AdmitsNextHopOctets(16) {
			t.Fatal("MUP 2/85 profile must admit 16-octet next hops")
		}

		cases := []struct {
			name       string
			input      netip.Addr
			mapV4      bool
			wantAddr   netip.Addr
			wantWire   []byte
			wantIs4In6 bool
		}{
			{
				name:       "ipv4_with_mapV4_true",
				input:      ipv4,
				mapV4:      true,
				wantAddr:   mapped,
				wantWire:   wantMappedBytes,
				wantIs4In6: true,
			},
			{
				name:       "supplied_mapped_ipv4",
				input:      mapped,
				mapV4:      false,
				wantAddr:   mapped,
				wantWire:   wantMappedBytes,
				wantIs4In6: true,
			},
			{
				name:       "native_ipv6_with_mapV4_true",
				input:      ipv6,
				mapV4:      true,
				wantAddr:   ipv6,
				wantWire:   wantIPv6Bytes,
				wantIs4In6: false,
			},
			{
				name:       "ipv4_with_mapV4_false_stays_native",
				input:      ipv4,
				mapV4:      false,
				wantAddr:   ipv4,
				wantWire:   wantIPv4Bytes,
				wantIs4In6: false,
			},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				selected := profile.SelectAddress(tc.input, tc.mapV4)
				if selected != tc.wantAddr {
					t.Fatalf("SelectAddress(%v, %v) = %v, want %v", tc.input, tc.mapV4, selected, tc.wantAddr)
				}
				if selected.Is4In6() != tc.wantIs4In6 {
					t.Fatalf("SelectAddress(%v, %v).Is4In6() = %v, want %v", tc.input, tc.mapV4, selected.Is4In6(), tc.wantIs4In6)
				}
				if gotLen := profile.NextHopLen(selected, netip.Addr{}); gotLen != len(tc.wantWire) {
					t.Fatalf("NextHopLen(%v) = %d, want %d", selected, gotLen, len(tc.wantWire))
				}

				buf := bytes.Repeat([]byte{0xa5}, 2+len(tc.wantWire)+2)
				written := profile.WriteNextHops(buf, 2, selected, netip.Addr{})
				if written != len(tc.wantWire) {
					t.Fatalf("WriteNextHops wrote %d bytes, want %d", written, len(tc.wantWire))
				}
				if !bytes.Equal(buf[:2], []byte{0xa5, 0xa5}) {
					t.Fatalf("leading canary overwritten: %x", buf[:2])
				}
				if !bytes.Equal(buf[2+written:], []byte{0xa5, 0xa5}) {
					t.Fatalf("trailing canary overwritten: %x", buf[2+written:])
				}
				wire := buf[2 : 2+written]
				if !bytes.Equal(wire, tc.wantWire) {
					t.Fatalf("wire next hop = %x, want %x", wire, tc.wantWire)
				}

				decoded, err := profile.DecodeNextHops(attribute.AFIIPv6, wire)
				if err != nil {
					t.Fatalf("DecodeNextHops(%x): %v", wire, err)
				}
				if !slices.Equal(decoded.Slice(), []netip.Addr{tc.wantAddr}) {
					t.Fatalf("DecodeNextHops = %v, want [%v]", decoded.Slice(), tc.wantAddr)
				}
				if decoded.Slice()[0].Is4In6() != tc.wantIs4In6 {
					t.Fatalf("DecodeNextHops Is4In6() = %v, want %v", decoded.Slice()[0].Is4In6(), tc.wantIs4In6)
				}

				fieldDecoded, err := profile.DecodeNextHopField(attribute.AFIIPv6, wire)
				if err != nil {
					t.Fatalf("DecodeNextHopField(%x): %v", wire, err)
				}
				if !slices.Equal(fieldDecoded.Slice(), []netip.Addr{tc.wantAddr}) {
					t.Fatalf("DecodeNextHopField = %v, want [%v]", fieldDecoded.Slice(), tc.wantAddr)
				}
			})
		}
	})

	t.Run("config_route_to_build_plugin", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name     string
			content  string
			nextHop  string
			isIPv6   bool
			wantAFI  uint16
			wantWire []byte
			wantAddr netip.Addr
		}{
			{
				name:     "ipv6_mup_maps_ipv4_next_hop",
				content:  "mup-isd 2001:db8::/64 rd 100:100",
				nextHop:  "192.0.2.1",
				isIPv6:   true,
				wantAFI:  uint16(AFIIPv6),
				wantWire: wantMappedBytes,
				wantAddr: mapped,
			},
			{
				name:     "ipv6_mup_preserves_native_ipv6_next_hop",
				content:  "mup-isd 2001:db8::/64 rd 100:100",
				nextHop:  "2001:db8::1",
				isIPv6:   true,
				wantAFI:  uint16(AFIIPv6),
				wantWire: wantIPv6Bytes,
				wantAddr: ipv6,
			},
			{
				name:     "ipv4_mup_keeps_native_ipv4_next_hop",
				content:  "mup-isd 10.0.1.0/24 rd 100:100",
				nextHop:  "192.0.2.1",
				isIPv6:   false,
				wantAFI:  uint16(AFIIPv4),
				wantWire: wantIPv4Bytes,
				wantAddr: ipv4,
			},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				pr, err := parseConfigRoute(registry.ConfigRouteRequest{
					Content: strings.Fields(tc.content),
					NextHop: tc.nextHop,
					IsIPv6:  tc.isIPv6,
				})
				if err != nil {
					t.Fatalf("parseConfigRoute: %v", err)
				}
				if !pr.MapV4NextHop {
					t.Fatal("parseConfigRoute returned MapV4NextHop=false, want true")
				}

				nhAddr, err := netip.ParseAddr(pr.NextHop)
				if err != nil {
					t.Fatalf("ParseAddr(%q): %v", pr.NextHop, err)
				}

				// Mirror peer_static_routes.go:toPluginParamsForPeer into BuildPlugin.
				ub := message.GetUpdateBuilder(65001, false, true, false)
				defer message.PutUpdateBuilder(ub)
				upd := ub.BuildPlugin(message.PluginParams{
					AFI:          tc.wantAFI,
					SAFI:         byte(SAFIMUP),
					IsIPv6:       pr.IsIPv6,
					NLRI:         pr.NLRI,
					NextHop:      nhAddr,
					MapV4NextHop: pr.MapV4NextHop,
				})
				if upd == nil {
					t.Fatal("BuildPlugin returned nil UPDATE")
				}
				checkMUPMPReachRoundTrip(t, upd.PathAttributes, tc.wantAFI, tc.wantWire, tc.wantAddr, pr.NLRI)
			})
		}
	})

	t.Run("encode_route_roundtrip", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name     string
			family   string
			routeCmd string
			wantAFI  uint16
			wantWire []byte
			wantAddr netip.Addr
		}{
			{
				name:     "ipv6_mup_ipv4_next_hop_maps_to_16_octets",
				family:   "ipv6/mup",
				routeCmd: "mup-isd 2001:db8::/64 rd 100:100 next-hop 192.0.2.1",
				wantAFI:  uint16(AFIIPv6),
				wantWire: wantMappedBytes,
				wantAddr: mapped,
			},
			{
				name:     "ipv6_mup_explicit_mapped_next_hop_preserved",
				family:   "ipv6/mup",
				routeCmd: "mup-isd 2001:db8::/64 rd 100:100 next-hop ::ffff:192.0.2.1",
				wantAFI:  uint16(AFIIPv6),
				wantWire: wantMappedBytes,
				wantAddr: mapped,
			},
			{
				name:     "ipv6_mup_native_ipv6_next_hop",
				family:   "ipv6/mup",
				routeCmd: "mup-isd 2001:db8::/64 rd 100:100 next-hop 2001:db8::1",
				wantAFI:  uint16(AFIIPv6),
				wantWire: wantIPv6Bytes,
				wantAddr: ipv6,
			},
			{
				name:     "ipv4_mup_native_ipv4_next_hop",
				family:   "ipv4/mup",
				routeCmd: "mup-isd 10.0.1.0/24 rd 100:100 next-hop 192.0.2.1",
				wantAFI:  uint16(AFIIPv4),
				wantWire: wantIPv4Bytes,
				wantAddr: ipv4,
			},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				frame, nlriBytes, err := EncodeRoute(tc.routeCmd, tc.family, 65001, false, true, false)
				if err != nil {
					t.Fatalf("EncodeRoute(%q, %q): %v", tc.routeCmd, tc.family, err)
				}
				const attrsOffset = 19 + 2 + 2
				if len(frame) < attrsOffset {
					t.Fatalf("UPDATE frame too short: %d bytes", len(frame))
				}
				withdrawnLen := int(binary.BigEndian.Uint16(frame[19:21]))
				if withdrawnLen != 0 {
					t.Fatalf("withdrawn length = %d, want 0", withdrawnLen)
				}
				attrLen := int(binary.BigEndian.Uint16(frame[21:23]))
				if attrsOffset+attrLen > len(frame) {
					t.Fatalf("attrLen %d exceeds frame length %d", attrLen, len(frame))
				}
				attrs := frame[attrsOffset : attrsOffset+attrLen]
				checkMUPMPReachRoundTrip(t, attrs, tc.wantAFI, tc.wantWire, tc.wantAddr, nlriBytes)
			})
		}
	})

	t.Run("unmapped_families_retain_native_ipv4", func(t *testing.T) {
		t.Parallel()

		// SR Policy IPv6 (2/73) leaves MapV4NextHop false and emits native 4-octet IPv4.
		ub := message.GetUpdateBuilder(65001, false, true, false)
		defer message.PutUpdateBuilder(ub)
		srNLRI := []byte{0x00, 0x00, 0x00, 0x01}
		srUpdate := ub.BuildPlugin(message.PluginParams{
			AFI:          uint16(AFIIPv6),
			SAFI:         byte(attribute.SAFISRPolicy),
			IsIPv6:       true,
			NLRI:         srNLRI,
			NextHop:      ipv4,
			MapV4NextHop: false,
		})
		if srUpdate == nil {
			t.Fatal("BuildPlugin for SR Policy 2/73 with native IPv4 returned nil")
		}
		_, _, srVal, found := attribute.AttrFind(srUpdate.PathAttributes, attribute.AttrMPReachNLRI)
		if !found {
			t.Fatalf("SR Policy UPDATE missing MP_REACH_NLRI: %x", srUpdate.PathAttributes)
		}
		if len(srVal) < 9 {
			t.Fatalf("SR Policy MP_REACH_NLRI too short: %x", srVal)
		}
		if srVal[3] != 4 {
			t.Fatalf("SR Policy 2/73 next-hop length = %d, want 4 (native IPv4)", srVal[3])
		}
		if !bytes.Equal(srVal[4:8], wantIPv4Bytes) {
			t.Fatalf("SR Policy 2/73 next hop = %x, want %x", srVal[4:8], wantIPv4Bytes)
		}

		// BGP-LS (16388/71) and BGP-LS VPN (16388/72) never map native IPv4 to IPv4-mapped IPv6.
		for _, safi := range []family.SAFI{family.SAFIBGPLinkState, family.SAFIBGPLinkStateVPN} {
			profile := attribute.MPNextHopProfile(attribute.AFI(family.AFIBGPLS), attribute.SAFI(safi))
			if profile.EncodeIPv4Mapped {
				t.Fatalf("BGP-LS 16388/%d profile unexpectedly sets EncodeIPv4Mapped", safi)
			}
			if profile.MappedIPv4 {
				t.Fatalf("BGP-LS 16388/%d profile unexpectedly sets MappedIPv4", safi)
			}
			selected := profile.SelectAddress(ipv4, false)
			if selected != ipv4 {
				t.Fatalf("BGP-LS 16388/%d SelectAddress(%v, false) = %v, want native IPv4", safi, ipv4, selected)
			}
			if !selected.Is4() {
				t.Fatalf("BGP-LS 16388/%d selected address %v is not native IPv4", safi, selected)
			}
		}
	})
}

// checkMUPMPReachRoundTrip verifies that attrs carry MP_REACH_NLRI with exact
// literal AFI/SAFI/next-hop/reserved/NLRI bytes (and no legacy NEXT_HOP), then
// round-trips the attribute through ParseMPReachNLRI and ParseMUP.
func checkMUPMPReachRoundTrip(
	t *testing.T,
	attrs []byte,
	wantAFI uint16,
	wantWire []byte,
	wantAddr netip.Addr,
	wantNLRI []byte,
) {
	t.Helper()

	if _, _, _, hasLegacy := attribute.AttrFind(attrs, attribute.AttrNextHop); hasLegacy {
		t.Fatalf("MUP UPDATE carries legacy NEXT_HOP (code 3) beside MP_REACH_NLRI: %x", attrs)
	}
	_, flags, value, found := attribute.AttrFind(attrs, attribute.AttrMPReachNLRI)
	if !found {
		t.Fatalf("MUP UPDATE missing MP_REACH_NLRI: %x", attrs)
	}
	if flags != attribute.FlagOptional {
		t.Fatalf("MP_REACH_NLRI flags = %#x, want %#x", flags, attribute.FlagOptional)
	}
	wantTotal := 5 + len(wantWire) + len(wantNLRI)
	if len(value) != wantTotal {
		t.Fatalf("MP_REACH_NLRI value len = %d, want %d (value=%x)", len(value), wantTotal, value)
	}
	gotAFI := binary.BigEndian.Uint16(value[0:2])
	if gotAFI != wantAFI {
		t.Fatalf("MP_REACH_NLRI AFI = %d, want %d", gotAFI, wantAFI)
	}
	if value[2] != byte(SAFIMUP) {
		t.Fatalf("MP_REACH_NLRI SAFI = %d, want %d", value[2], SAFIMUP)
	}
	if int(value[3]) != len(wantWire) {
		t.Fatalf("MP_REACH_NLRI next-hop length = %d, want %d (value=%x)", value[3], len(wantWire), value)
	}
	if !bytes.Equal(value[4:4+len(wantWire)], wantWire) {
		t.Fatalf("MP_REACH_NLRI next-hop bytes = %x, want %x", value[4:4+len(wantWire)], wantWire)
	}
	if value[4+len(wantWire)] != 0 {
		t.Fatalf("MP_REACH_NLRI reserved octet = %#x, want 0", value[4+len(wantWire)])
	}
	if !bytes.Equal(value[5+len(wantWire):], wantNLRI) {
		t.Fatalf("MP_REACH_NLRI NLRI = %x, want %x", value[5+len(wantWire):], wantNLRI)
	}

	parsed, err := attribute.ParseMPReachNLRI(value)
	if err != nil {
		t.Fatalf("ParseMPReachNLRI(%x): %v", value, err)
	}
	if parsed.AFI != attribute.AFI(wantAFI) {
		t.Fatalf("parsed AFI = %d, want %d", parsed.AFI, wantAFI)
	}
	if parsed.SAFI != attribute.SAFI(SAFIMUP) {
		t.Fatalf("parsed SAFI = %d, want %d", parsed.SAFI, SAFIMUP)
	}
	if !slices.Equal(parsed.NextHops.Slice(), []netip.Addr{wantAddr}) {
		t.Fatalf("parsed NextHops = %v, want [%v]", parsed.NextHops.Slice(), wantAddr)
	}
	if parsed.NextHops.Slice()[0].Is4In6() != wantAddr.Is4In6() {
		t.Fatalf("parsed NextHop Is4In6() = %v, want %v", parsed.NextHops.Slice()[0].Is4In6(), wantAddr.Is4In6())
	}
	if !bytes.Equal(parsed.NLRI, wantNLRI) {
		t.Fatalf("parsed NLRI = %x, want %x", parsed.NLRI, wantNLRI)
	}

	nlriDecoded, rest, err := ParseMUP(AFI(wantAFI), parsed.NLRI)
	if err != nil {
		t.Fatalf("ParseMUP(AFI=%d, %x): %v", wantAFI, parsed.NLRI, err)
	}
	if len(rest) != 0 {
		t.Fatalf("ParseMUP left %d trailing bytes", len(rest))
	}
	if nlriDecoded.RouteType() != MUPISD {
		t.Fatalf("ParseMUP route type = %v, want MUPISD", nlriDecoded.RouteType())
	}
}
