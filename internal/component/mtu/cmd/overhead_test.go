// Design: docs/architecture/diagnostics/path-mtu.md -- the ESP overhead tests
// Related: overhead.go -- deriveESPOverhead

package cmd

import (
	"errors"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/ike/crypto"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/core/ipsecinventory"
)

// espTransformCase is one hand-computed overhead: the transform pair as the
// inventory carries it, and the octets it adds over an IPv4 endpoint without
// UDP encapsulation. The other three combinations are derived in the test
// from the header sizes and asserted against the same row.
type espTransformCase struct {
	encryptionName string
	integrityName  string // "" for an AEAD cipher
	// wireOctets is 8 ESP + IV + ICV: what the cipher pair puts on the wire.
	wireOctets uint16
	block      uint16
}

// espTransformCases is the hand-computed table. The population is asserted
// against the crypto registry in TestESPOverheadPerTransform, so a transform
// the registry gains reddens the test until a row is written here.
var espTransformCases = []espTransformCase{
	// AES-GCM: 8 ESP + 8 IV + 16 ICV, padded to the 4-octet ESP boundary.
	{encryptionName: "aes128gcm", wireOctets: 32, block: 4},
	{encryptionName: "aes256gcm", wireOctets: 32, block: 4},
	// AES-CBC: 8 ESP + 16 IV + the truncated HMAC, padded to the 16-octet
	// AES block. SHA-256 truncates to 16, SHA-384 to 24, SHA-512 to 32.
	{encryptionName: "aes128", integrityName: "sha256", wireOctets: 40, block: 16},
	{encryptionName: "aes128", integrityName: "sha384", wireOctets: 48, block: 16},
	{encryptionName: "aes128", integrityName: "sha512", wireOctets: 56, block: 16},
	{encryptionName: "aes256", integrityName: "sha256", wireOctets: 40, block: 16},
	{encryptionName: "aes256", integrityName: "sha384", wireOctets: 48, block: 16},
	{encryptionName: "aes256", integrityName: "sha512", wireOctets: 56, block: 16},
	// NULL (integrity-only ESP, RFC 4303 Section 1): 8 ESP + no IV + the
	// truncated HMAC, padded to the 4-octet ESP boundary alone.
	{encryptionName: "null", integrityName: "sha256", wireOctets: 24, block: 4},
	{encryptionName: "null", integrityName: "sha384", wireOctets: 32, block: 4},
	{encryptionName: "null", integrityName: "sha512", wireOctets: 40, block: 4},
	// SHA-1 is ESP-only integrity: 12 ICV octets, never an IKE PRF.
	{encryptionName: "aes128", integrityName: "sha1", wireOctets: 36, block: 16},
	{encryptionName: "aes256", integrityName: "sha1", wireOctets: 36, block: 16},
	{encryptionName: "null", integrityName: "sha1", wireOctets: 20, block: 4},
	// RFC 7634 Section 2: 8 ESP + 8 transmitted IV + 16 tag, aligned to 4.
	// The four-octet per-SA salt is NOT transmitted.
	{encryptionName: "chacha20poly1305", wireOctets: 32, block: 4},
	// RFC 2451 Sections 2.1/2.4: 8 ESP + 8 IV + truncated HMAC, aligned to 8.
	{encryptionName: "3des", integrityName: "sha1", wireOctets: 28, block: 8},
	{encryptionName: "3des", integrityName: "sha256", wireOctets: 32, block: 8},
	{encryptionName: "3des", integrityName: "sha384", wireOctets: 40, block: 8},
	{encryptionName: "3des", integrityName: "sha512", wireOctets: 48, block: 8},
}

// upTunnel builds an installed tunnel-mode Child SA over the given endpoint
// with the named transform pair.
func upTunnel(t *testing.T, c *espTransformCase, remote netip.Addr, udp bool) ipsecinventory.Tunnel {
	t.Helper()
	enc, err := crypto.LookupESPEncryption(c.encryptionName)
	if err != nil {
		t.Fatalf("LookupEncryption(%q): %v", c.encryptionName, err)
	}
	integrity := crypto.AUTH_NONE
	if c.integrityName != "" {
		transform, err := crypto.LookupESPIntegrity(c.integrityName)
		if err != nil {
			t.Fatalf("LookupESPIntegrity(%q): %v", c.integrityName, err)
		}
		integrity = transform.ID
	}
	return ipsecinventory.Tunnel{
		Peer:              "site-a",
		Up:                true,
		InstalledRemote:   remote,
		InstalledLocal:    remote,
		UDPEncap:          udp,
		Mode:              ipsecinventory.ModeTunnel,
		EncryptionName:    c.encryptionName,
		IntegrityName:     c.integrityName,
		Encryption:        ipsecinventory.EncryptionID(enc.ID),
		EncryptionKeyBits: enc.KeyLength,
		Integrity:         ipsecinventory.IntegrityID(integrity),
	}
}

// TestESPOverheadPerTransform validates A-2: the overhead is derivable for
// every ESP transform pair Ze can negotiate. The population comes from the
// registries the negotiator reads (ipsec.SupportedESPEncryptionNames and
// crypto.SupportedESPIntegrityNames), so a transform added there fails the test
// until espTransformCases carries its independently computed framing. Each row is then
// checked over both endpoint families, with and without UDP encapsulation:
// AES-GCM-128 over IPv4 is 52 bare (AC-2) and 60 encapsulated (AC-3).
func TestESPOverheadPerTransform(t *testing.T) {
	byPair := make(map[[2]string]*espTransformCase, len(espTransformCases))
	for i := range espTransformCases {
		c := &espTransformCases[i]
		byPair[[2]string{c.encryptionName, c.integrityName}] = c
	}
	var population [][2]string
	for _, encName := range ipsec.SupportedESPEncryptionNames() {
		enc, err := crypto.LookupESPEncryption(encName)
		if err != nil {
			t.Fatalf("LookupEncryption(%q): %v", encName, err)
		}
		if enc.ID.IsAEAD() {
			population = append(population, [2]string{encName, ""})
			continue
		}
		for _, integrityName := range crypto.SupportedESPIntegrityNames() {
			population = append(population, [2]string{encName, integrityName})
		}
	}
	if len(population) != len(espTransformCases) {
		t.Errorf("the registries can negotiate %d transform pairs, the table hand-computes %d", len(population), len(espTransformCases))
	}
	v4 := netip.MustParseAddr("192.0.2.1")
	v6 := netip.MustParseAddr("2001:db8::1")
	for _, pair := range population {
		c, ok := byPair[pair]
		if !ok {
			t.Errorf("transform pair %v is negotiable and has no hand-computed row", pair)
			continue
		}
		for _, sub := range []struct {
			name   string
			remote netip.Addr
			udp    bool
			want   uint16
		}{
			{"ipv4", v4, false, ipv4HeaderOctets + c.wireOctets},
			{"ipv4-udp", v4, true, ipv4HeaderOctets + udpHeaderOctets + c.wireOctets},
			{"ipv6", v6, false, ipv6HeaderOctets + c.wireOctets},
			{"ipv6-udp", v6, true, ipv6HeaderOctets + udpHeaderOctets + c.wireOctets},
		} {
			tunnel := upTunnel(t, c, sub.remote, sub.udp)
			got, err := deriveESPOverhead(&tunnel)
			if err != nil {
				t.Errorf("%s/%s %s: %v", c.encryptionName, c.integrityName, sub.name, err)
				continue
			}
			if got.octets() != sub.want {
				t.Errorf("%s/%s %s: overhead %d, want %d", c.encryptionName, c.integrityName, sub.name, got.octets(), sub.want)
			}
			if got.block != c.block {
				t.Errorf("%s/%s %s: block %d, want %d", c.encryptionName, c.integrityName, sub.name, got.block, c.block)
			}
			if got.mode != ipsecinventory.ModeTunnel {
				t.Errorf("%s/%s %s: mode %s, want tunnel", c.encryptionName, c.integrityName, sub.name, got.mode)
			}
		}
	}
	// The two figures the spec pins by name.
	gcm := upTunnel(t, &espTransformCases[0], v4, false)
	got, err := deriveESPOverhead(&gcm)
	if err != nil {
		t.Fatalf("aes128gcm ipv4: %v", err)
	}
	if got.octets() != 52 {
		t.Errorf("AC-2: aes128gcm over ipv4 = %d, want 52", got.octets())
	}
	gcm.UDPEncap = true
	got, err = deriveESPOverhead(&gcm)
	if err != nil {
		t.Fatalf("aes128gcm ipv4 udp: %v", err)
	}
	if got.octets() != 60 {
		t.Errorf("AC-3: aes128gcm over ipv4 with udp = %d, want 60", got.octets())
	}
}

// TestESPOverheadUnknownTransformRefuses proves that a transform the
// arithmetic does not know is a refusal, never a default: an unassigned ENCR
// id, the AES CCM id IKE can negotiate but no dataplane installs, a CBC cipher
// with no integrity transform, a child that is not installed, and a mode
// nobody set each answer errOverheadRefused.
func TestESPOverheadUnknownTransformRefuses(t *testing.T) {
	v4 := netip.MustParseAddr("192.0.2.1")
	base := upTunnel(t, &espTransformCases[0], v4, false)
	cases := []struct {
		name   string
		mutate func(tunnel *ipsecinventory.Tunnel)
	}{
		{"unassigned encr id", func(tunnel *ipsecinventory.Tunnel) { tunnel.Encryption = 1023 }},
		{"aes ccm, not installable in esp", func(tunnel *ipsecinventory.Tunnel) {
			tunnel.Encryption = ipsecinventory.EncryptionID(crypto.ENCR_AES_CCM_16)
		}},
		{"cbc without integrity", func(tunnel *ipsecinventory.Tunnel) {
			tunnel.Encryption = ipsecinventory.EncryptionID(crypto.ENCR_AES_CBC)
			tunnel.Integrity = ipsecinventory.IntegrityID(crypto.AUTH_NONE)
		}},
		{"cbc with an unknown integrity id", func(tunnel *ipsecinventory.Tunnel) {
			tunnel.Encryption = ipsecinventory.EncryptionID(crypto.ENCR_AES_CBC)
			tunnel.Integrity = 1023
		}},
		{"child down", func(tunnel *ipsecinventory.Tunnel) { tunnel.Up = false }},
		{"no installed endpoint", func(tunnel *ipsecinventory.Tunnel) { tunnel.InstalledRemote = netip.Addr{} }},
		{"mode unspecified", func(tunnel *ipsecinventory.Tunnel) { tunnel.Mode = ipsecinventory.ModeUnspecified }},
		{"mode unknown", func(tunnel *ipsecinventory.Tunnel) { tunnel.Mode = ipsecinventory.Mode(255) }},
	}
	for _, c := range cases {
		tunnel := base
		c.mutate(&tunnel)
		got, err := deriveESPOverhead(&tunnel)
		if !errors.Is(err, errOverheadRefused) {
			t.Errorf("%s: err = %v, overhead = %+v; want errOverheadRefused", c.name, err, got)
		}
	}
}

// TestESPOverheadTransportMode proves AC-16's derivation half: a transport
// mode SA carries the mode through, with the same fixed octets and the
// packet's own header as outerHeader, so arith.go can tell the two apart.
func TestESPOverheadTransportMode(t *testing.T) {
	tunnel := upTunnel(t, &espTransformCases[0], netip.MustParseAddr("2001:db8::1"), true)
	tunnel.Mode = ipsecinventory.ModeTransport
	got, err := deriveESPOverhead(&tunnel)
	if err != nil {
		t.Fatalf("transport: %v", err)
	}
	if got.mode != ipsecinventory.ModeTransport {
		t.Errorf("mode = %s, want transport", got.mode)
	}
	if got.outerHeader != ipv6HeaderOctets {
		t.Errorf("outerHeader = %d, want %d", got.outerHeader, ipv6HeaderOctets)
	}
	if got.fixed != udpHeaderOctets+32 {
		t.Errorf("fixed = %d, want %d", got.fixed, udpHeaderOctets+32)
	}
}

// TestESPFixedCipherMTUAdvice drives overhead through ceiling, recommendation and
// MSS. A 1499-byte path distinguishes ChaCha's four-byte and 3DES's eight-byte
// alignment, and transport mode must preserve the original IP header.
func TestESPFixedCipherMTUAdvice(t *testing.T) {
	for _, tc := range []struct {
		name      string
		hash      string
		mode      ipsecinventory.Mode
		ceiling   uint16
		recommend uint16
		mssV4     uint16
	}{
		{"chacha20poly1305", "", ipsecinventory.ModeTunnel, 1442, 1410, 1370},
		{"chacha20poly1305", "", ipsecinventory.ModeTransport, 1462, 1430, 1390},
		{"3des", "sha1", ipsecinventory.ModeTunnel, 1446, 1414, 1374},
		{"3des", "sha1", ipsecinventory.ModeTransport, 1466, 1430, 1390},
	} {
		tunnel := upTunnel(t, &espTransformCase{encryptionName: tc.name, integrityName: tc.hash},
			netip.MustParseAddr("192.0.2.1"), false)
		tunnel.Mode = tc.mode
		overhead, err := deriveESPOverhead(&tunnel)
		if err != nil {
			t.Fatalf("%s/%s: %v", tc.name, tc.mode, err)
		}
		gotCeiling := ceiling(1499, &overhead)
		if gotCeiling != tc.ceiling {
			t.Fatalf("%s/%s: ceiling %d, want %d", tc.name, tc.mode, gotCeiling, tc.ceiling)
		}
		for _, family := range []ipFamily{ipFamilyV4, ipFamilyV6} {
			gotRecommended, err := recommended(gotCeiling, &overhead, family)
			if err != nil {
				t.Fatalf("%s/%s/%s recommendation: %v", tc.name, tc.mode, family, err)
			}
			if gotRecommended != tc.recommend {
				t.Fatalf("%s/%s: recommended %d, want %d", tc.name, tc.mode, gotRecommended, tc.recommend)
			}
			wantMSS := tc.mssV4
			if family == ipFamilyV6 {
				wantMSS -= 20
			}
			gotMSS, err := mss(gotRecommended, family)
			if err != nil {
				t.Fatalf("%s/%s/%s MSS: %v", tc.name, tc.mode, family, err)
			}
			if gotMSS != wantMSS {
				t.Errorf("%s/%s/%s: MSS %d, want %d", tc.name, tc.mode, family, gotMSS, wantMSS)
			}
		}
	}
}

// TestNonIPsecTunnelOverheadExact validates AC-1: every modeled non-IPsec
// tunnel kind (gre, gretap, ip6gre, ip6gretap, vxlan, ipip, sit, ip6tnl,
// ipip6) and WireGuard produce exact hand-computed overheads, trailer=0,
// block=1, and exact ceilings and recommendations over a 1500-octet underlay.
func TestNonIPsecTunnelOverheadExact(t *testing.T) {
	v4 := netip.MustParseAddr("198.51.100.1")
	v6 := netip.MustParseAddr("2001:db8::1")
	cases := []struct {
		name         string
		info         iface.TunnelInfo
		wantOverhead uint16
		inner        ipFamily
		wantCeiling  uint16
		wantRec      uint16
	}{
		{"gre base", iface.TunnelInfo{Kind: iface.TunnelKindGRE, Remote: v4}, 24, ipFamilyV4, 1476, 1444},
		{"gre +key", iface.TunnelInfo{Kind: iface.TunnelKindGRE, Remote: v4, GREKey: true}, 28, ipFamilyV4, 1472, 1440},
		{"gre +csum+key+seq", iface.TunnelInfo{Kind: iface.TunnelKindGRE, Remote: v4, GREChecksum: true, GREKey: true, GRESequence: true}, 36, ipFamilyV4, 1464, 1432},
		{"gretap base", iface.TunnelInfo{Kind: iface.TunnelKindGRETap, Remote: v4}, 38, ipFamilyV6, 1462, 1430},
		{"gretap +key", iface.TunnelInfo{Kind: iface.TunnelKindGRETap, Remote: v4, GREKey: true}, 42, ipFamilyV6, 1458, 1426},
		{"ip6gre default encap-limit", iface.TunnelInfo{Kind: iface.TunnelKindIP6GRE, Remote: v6, IPv6EncapLimit: true}, 52, ipFamilyV6, 1448, 1416},
		{"ip6gre no encap-limit", iface.TunnelInfo{Kind: iface.TunnelKindIP6GRE, Remote: v6}, 44, ipFamilyV6, 1456, 1424},
		{"ip6gre +key +encap-limit", iface.TunnelInfo{Kind: iface.TunnelKindIP6GRE, Remote: v6, GREKey: true, IPv6EncapLimit: true}, 56, ipFamilyV6, 1444, 1412},
		{"ip6gretap default encap-limit", iface.TunnelInfo{Kind: iface.TunnelKindIP6GRETap, Remote: v6, IPv6EncapLimit: true}, 66, ipFamilyV6, 1434, 1402},
		{"ip6gretap no encap-limit", iface.TunnelInfo{Kind: iface.TunnelKindIP6GRETap, Remote: v6}, 58, ipFamilyV6, 1442, 1410},
		{"vxlan ipv4", iface.TunnelInfo{Kind: iface.TunnelKindVxlan, Remote: v4}, 50, ipFamilyV6, 1450, 1418},
		{"vxlan ipv6", iface.TunnelInfo{Kind: iface.TunnelKindVxlan, Remote: v6}, 70, ipFamilyV6, 1430, 1398},
		{"ipip", iface.TunnelInfo{Kind: iface.TunnelKindIPIP, Remote: v4}, 20, ipFamilyV4, 1480, 1448},
		{"sit", iface.TunnelInfo{Kind: iface.TunnelKindSIT, Remote: v4}, 20, ipFamilyV6, 1480, 1448},
		{"ip6tnl default encap-limit", iface.TunnelInfo{Kind: iface.TunnelKindIP6Tnl, Remote: v6, IPv6EncapLimit: true}, 48, ipFamilyV6, 1452, 1420},
		{"ip6tnl ign-encap-limit", iface.TunnelInfo{Kind: iface.TunnelKindIP6Tnl, Remote: v6}, 40, ipFamilyV6, 1460, 1428},
		{"ipip6 default encap-limit", iface.TunnelInfo{Kind: iface.TunnelKindIPIP6, Remote: v6, IPv6EncapLimit: true}, 48, ipFamilyV4, 1452, 1420},
		{"ipip6 ign-encap-limit", iface.TunnelInfo{Kind: iface.TunnelKindIPIP6, Remote: v6}, 40, ipFamilyV4, 1460, 1428},
	}
	for _, tc := range cases {
		got, err := deriveTunnelOverhead(&tc.info)
		if err != nil {
			t.Errorf("%s: unexpected error: %v", tc.name, err)
			continue
		}
		if got.octets() != tc.wantOverhead {
			t.Errorf("%s: overhead = %d, want %d", tc.name, got.octets(), tc.wantOverhead)
		}
		if got.trailer != 0 || got.block != 1 || got.mode != ipsecinventory.ModeTunnel {
			t.Errorf("%s: framing = %+v, want trailer=0 block=1 mode=tunnel", tc.name, got)
		}
		if ceil := ceiling(1500, &got); ceil != tc.wantCeiling {
			t.Errorf("%s: ceiling(1500) = %d, want %d", tc.name, ceil, tc.wantCeiling)
		}
		rec, err := recommended(tc.wantCeiling, &got, tc.inner)
		if err != nil {
			t.Errorf("%s: recommended: %v", tc.name, err)
		}
		if rec != tc.wantRec {
			t.Errorf("%s: recommended = %d, want %d", tc.name, rec, tc.wantRec)
		}
	}

	// WireGuard IPv4 (60) and IPv6 (80): matches Linux 1420 default on 1500 Ethernet.
	for _, wg := range []struct {
		name     string
		endpoint netip.Addr
		want     uint16
		ceil1500 uint16
	}{
		{"wireguard ipv4", v4, 60, 1440},
		{"wireguard ipv6", v6, 80, 1420},
	} {
		got, err := deriveWireguardOverhead(wg.endpoint)
		if err != nil {
			t.Fatalf("%s: %v", wg.name, err)
		}
		if got.octets() != wg.want {
			t.Errorf("%s: overhead = %d, want %d", wg.name, got.octets(), wg.want)
		}
		if got.trailer != 0 || got.block != 1 || got.mode != ipsecinventory.ModeTunnel {
			t.Errorf("%s: framing = %+v, want trailer=0 block=1 mode=tunnel", wg.name, got)
		}
		if ceil := ceiling(1500, &got); ceil != wg.ceil1500 {
			t.Errorf("%s: ceiling(1500) = %d, want %d", wg.name, ceil, wg.ceil1500)
		}
	}
}

// TestNonIPsecTunnelOverheadRefusesUnknownOrFOU validates AC-2: exact-or-reject
// refuses nil info, FOU/GUE outer encapsulation (OuterEncapType != 0), unknown
// or unsupported TunnelKind, VXLAN with an invalid remote, and WireGuard with
// an invalid endpoint.
func TestNonIPsecTunnelOverheadRefusesUnknownOrFOU(t *testing.T) {
	v4 := netip.MustParseAddr("198.51.100.1")
	v6 := netip.MustParseAddr("2001:db8::1")
	if _, err := deriveTunnelOverhead(nil); !errors.Is(err, errOverheadRefused) {
		t.Errorf("nil info: err = %v, want errOverheadRefused", err)
	}
	cases := []struct {
		name string
		info iface.TunnelInfo
	}{
		{"FOU outer encap (1)", iface.TunnelInfo{Kind: iface.TunnelKindGRE, Remote: v4, OuterEncapType: 1}},
		{"GUE outer encap (2)", iface.TunnelInfo{Kind: iface.TunnelKindIPIP, Remote: v4, OuterEncapType: 2}},
		{"unknown kind", iface.TunnelInfo{Kind: iface.TunnelKindUnknown, Remote: v4}},
		{"unsupported kind", iface.TunnelInfo{Kind: iface.TunnelKind(255), Remote: v4}},
		{"gre with IPv6 remote", iface.TunnelInfo{Kind: iface.TunnelKindGRE, Remote: v6}},
		{"ipip with IPv6 remote", iface.TunnelInfo{Kind: iface.TunnelKindIPIP, Remote: v6}},
		{"ip6gre with IPv4 remote", iface.TunnelInfo{Kind: iface.TunnelKindIP6GRE, Remote: v4}},
		{"ip6tnl with IPv4 remote", iface.TunnelInfo{Kind: iface.TunnelKindIP6Tnl, Remote: v4}},
		{"vxlan invalid remote", iface.TunnelInfo{Kind: iface.TunnelKindVxlan}},
	}
	for _, tc := range cases {
		got, err := deriveTunnelOverhead(&tc.info)
		if !errors.Is(err, errOverheadRefused) {
			t.Errorf("%s: err = %v, overhead = %+v; want errOverheadRefused", tc.name, err, got)
		}
	}
	if got, err := deriveWireguardOverhead(netip.Addr{}); !errors.Is(err, errOverheadRefused) {
		t.Errorf("wireguard invalid endpoint: err = %v, overhead = %+v; want errOverheadRefused", err, got)
	}
}
