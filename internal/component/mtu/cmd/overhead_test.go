// Design: docs/architecture/diagnostics/path-mtu.md -- the ESP overhead tests
// Related: overhead.go -- deriveESPOverhead

package cmd

import (
	"errors"
	"net/netip"
	"testing"

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
}

// upTunnel builds an installed tunnel-mode Child SA over the given endpoint
// with the named transform pair.
func upTunnel(t *testing.T, c *espTransformCase, remote netip.Addr, udp bool) ipsecinventory.Tunnel {
	t.Helper()
	enc, err := crypto.LookupEncryption(c.encryptionName)
	if err != nil {
		t.Fatalf("LookupEncryption(%q): %v", c.encryptionName, err)
	}
	integrity := crypto.AUTH_NONE
	if c.integrityName != "" {
		transform, err := crypto.LookupIntegrity(c.integrityName)
		if err != nil {
			t.Fatalf("LookupIntegrity(%q): %v", c.integrityName, err)
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
// crypto.SupportedIntegrityNames), so a transform added there fails the test
// until espTransformCases and espCipherWires both carry it. Each row is then
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
		enc, err := crypto.LookupEncryption(encName)
		if err != nil {
			t.Fatalf("LookupEncryption(%q): %v", encName, err)
		}
		if enc.ID.IsAEAD() {
			population = append(population, [2]string{encName, ""})
			continue
		}
		for _, integrityName := range crypto.SupportedIntegrityNames() {
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
