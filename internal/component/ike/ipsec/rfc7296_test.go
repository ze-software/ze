// VALIDATES: the RFC 7296 Section 2.15 obligations on the management interface that supplies
// a pre-shared secret: it accepts ASCII strings of at least 64 octets, and it does not add a
// null terminator before the secret is used. Each test carries an `RFC requirement:` tag
// binding it to its checklist id.
// PREVENTS: a config-parse change that caps, pads, or terminates an operator's pre-shared
// secret, which would silently break authentication against a conforming peer.
package ipsec

import (
	"strings"
	"testing"
)

// pskPeerTree builds a site-to-site peer whose authentication mode is pre-shared-secret.
func pskPeerTree(t *testing.T, psk string) *IPsecConfig {
	t.Helper()
	tree := makePeerTree("psk-length", peerOpts{
		ikeGroupName: "IKE-RW",
		espGroupName: "ESP-RW",
		ikeGroup:     "IKE-RW",
		espGroup:     "ESP-RW",
		authMode:     "pre-shared-secret",
		psk:          psk,
	})
	cfg, err := ParseIPsecConfig(tree)
	if err != nil {
		t.Fatalf("ParseIPsecConfig: %v", err)
	}
	return cfg
}

// RFC requirement: RFC7296-2.15-1 positive -- the management interface accepts an ASCII shared
// secret of at least 64 octets. The `pre-shared-secret` YANG leaf is an unconstrained
// `type string` (ipsec/yang/ze-ipsec-conf.yang:344-348). parseAuthConfig assigns that
// value through unchanged (config.go:544-553). A 64-octet secret and a 200-octet secret
// both survive the parse byte for byte.
// RFC requirement: RFC7296-2.15-1 negative -- acceptance follows the operator's input and Ze
// never invents a secret. A peer that supplies no `pre-shared-secret` leaf leaves the
// secret empty.
func TestPSKAcceptsAtLeast64ASCIIOctets(t *testing.T) {
	for _, n := range []int{64, 65, 128, 200} {
		want := strings.Repeat("a", n-1) + "Z"
		cfg := pskPeerTree(t, want)
		peer, ok := cfg.Peers["psk-length"]
		if !ok {
			t.Fatal("peer psk-length is absent from the parsed config")
		}
		if peer.Auth.PSK != want {
			t.Errorf("a %d-octet ASCII secret parsed to %d octets; want it accepted unchanged",
				n, len(peer.Auth.PSK))
		}
		if len(peer.Auth.PSK) != n {
			t.Errorf("secret length = %d octets, want %d", len(peer.Auth.PSK), n)
		}
	}

	// The full printable-ASCII range is accepted, not just letters.
	var ascii strings.Builder
	for c := byte(0x20); c < 0x7f; c++ {
		ascii.WriteByte(c)
	}
	full := ascii.String()
	if len(full) < 64 {
		t.Fatalf("printable ASCII sample is %d octets, want at least 64", len(full))
	}
	cfg := pskPeerTree(t, full)
	if got := cfg.Peers["psk-length"].Auth.PSK; got != full {
		t.Errorf("a %d-octet printable-ASCII secret did not survive the parse", len(full))
	}

	// Negative: no leaf, no secret.
	none := pskPeerTree(t, "")
	if got := none.Peers["psk-length"].Auth.PSK; got != "" {
		t.Errorf("a peer with no pre-shared-secret leaf parsed to secret %q; the management "+
			"interface must not invent one", got)
	}
}

// RFC requirement: RFC7296-2.15-2 positive -- Ze adds no null terminator. parseAuthConfig assigns
// the leaf value to AuthConfig.PSK as a Go string with no padding step. The parsed secret
// is therefore exactly as long as the operator's value, and it ends with the operator's
// last octet, never 0x00. computePSKAuth then feeds []byte(psk) straight into the PRF
// (engine/auth.go:245), so the AUTH key is derived over those octets alone.
// RFC requirement: RFC7296-2.15-2 negative -- the no-terminator rule is not a stripping rule.
// A secret the operator deliberately ends with a NUL keeps that octet, so the parser
// neither adds nor removes trailing bytes.
func TestPSKHasNoNullTerminatorAdded(t *testing.T) {
	const plain = "correct horse battery staple correct horse battery staple 1234567"
	cfg := pskPeerTree(t, plain)
	got := cfg.Peers["psk-length"].Auth.PSK
	if len(got) != len(plain) {
		t.Fatalf("secret length = %d octets, want %d; a terminator was added or removed",
			len(got), len(plain))
	}
	if strings.ContainsRune(got, 0) {
		t.Error("the parsed secret contains a NUL octet; no terminator may be added")
	}
	if got[len(got)-1] != plain[len(plain)-1] {
		t.Errorf("last octet = %q, want %q", got[len(got)-1], plain[len(plain)-1])
	}

	// Negative: a NUL the operator supplied is preserved, so the parser is not trimming.
	withNUL := plain + "\x00"
	cfgNUL := pskPeerTree(t, withNUL)
	gotNUL := cfgNUL.Peers["psk-length"].Auth.PSK
	if len(gotNUL) != len(withNUL) {
		t.Errorf("an operator-supplied trailing NUL changed the length: %d octets, want %d",
			len(gotNUL), len(withNUL))
	}
	if gotNUL[len(gotNUL)-1] != 0 {
		t.Error("an operator-supplied trailing NUL was stripped; the parser must neither add " +
			"nor remove trailing octets")
	}
}
