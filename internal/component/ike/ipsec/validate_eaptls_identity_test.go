// Design: docs/architecture/ike/ipsec-3-data-model.md -- EAP-TLS identity policy data model and validation
// RFC: rfc/short/rfc5216.md -- Section 5.3, peer and server identity authorization
//
// VALIDATES: ValidateEAPTLSIdentities enforces the full fail-closed matrix
// across connection-type and authentication mode using opererr errors with exact
// fix commands, parseEAPTLSIdentityPolicy populates AuthAdditions while keeping
// Additions nil when unset, SiteToSitePeer.Equal detects identity policy edits,
// and EffectiveEAPTLSServerIdentity resolves explicit server-identity or
// unambiguous FQDN remote-address fallback.
// PREVENTS: unconstrained EAP-TLS peers committing, malformed identity rules
// passing offline validation, or identity edits failing to restart peers and
// invalidate resumption ticket stores.

package ipsec

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/opererr"
)

// dnsNameOfLen builds a valid multi-label DNS name of exactly total bytes using
// 63-byte labels so boundary tests can exercise the exact 255/256-byte edge.
func dnsNameOfLen(total int) string {
	const maxLabel = 63
	var b strings.Builder
	b.Grow(total)
	rem := total
	for rem > 0 {
		if b.Len() > 0 {
			b.WriteByte('.')
			rem--
		}
		seg := min(rem, maxLabel)
		if rem > maxLabel && rem-seg == 1 {
			// Leave room for the dot plus at least a 1-byte final label.
			seg--
		}
		for range seg {
			b.WriteByte('a')
		}
		rem -= seg
	}
	return b.String()
}

func requireOperRefusal(t *testing.T, err error, wantProblemSub, wantFix string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected refusal naming %q with fix %q, got nil", wantProblemSub, wantFix)
	}
	oe, ok := errors.AsType[*opererr.Error](err)
	if !ok {
		t.Fatalf("refusal %q is %T, want *opererr.Error", err, err)
	}
	if !strings.Contains(oe.Problem(), wantProblemSub) {
		t.Errorf("problem %q does not contain %q", oe.Problem(), wantProblemSub)
	}
	if oe.Why() == "" {
		t.Error("operator error has empty Why")
	}
	if oe.Fix() != wantFix {
		t.Errorf("fix = %q, want %q", oe.Fix(), wantFix)
	}
	lines := strings.Split(oe.Error(), "\n")
	if last := lines[len(lines)-1]; last != "fix: "+wantFix {
		t.Errorf("last rendered line = %q, want %q", last, "fix: "+wantFix)
	}
}

func TestValidateEAPTLSIdentitiesMatrix(t *testing.T) {
	valid255 := dnsNameOfLen(255)
	overlong256 := dnsNameOfLen(256)

	t.Run("accepted configurations", func(t *testing.T) {
		cases := []struct {
			name string
			peer SiteToSitePeer
		}{
			{
				name: "respond with exact DNS, wildcard, NAI, realm, IPv4, IPv6, URI, URN",
				peer: SiteToSitePeer{
					ConnectionType: ConnectionRespond,
					Auth: AuthConfig{
						Mode: AuthEAPTLS,
						Additions: &AuthAdditions{
							AllowedIdentities: []string{
								"swan-eap-client",
								"vpn.example.com",
								"*.branch.example.com",
								"alice@example.com",
								"@example.com",
								"192.0.2.10",
								"2001:db8::10",
								"spiffe://corp.example/node/1",
								"urn:ze:client:1",
								valid255,
							},
						},
					},
				},
			},
			{
				name: "respond with explicit lone wildcard opt-in",
				peer: SiteToSitePeer{
					ConnectionType: ConnectionRespond,
					Auth: AuthConfig{
						Mode:      AuthEAPTLS,
						Additions: &AuthAdditions{AllowedIdentities: []string{"*"}},
					},
				},
			},
			{
				name: "initiate with explicit DNS server-identity over IP remote-address",
				peer: SiteToSitePeer{
					ConnectionType: ConnectionInitiate,
					RemoteAddress:  "172.28.0.3",
					Auth: AuthConfig{
						Mode:      AuthEAPTLS,
						Additions: &AuthAdditions{ServerIdentity: "vpn.example.com"},
					},
				},
			},
			{
				name: "initiate with explicit IPv4 server-identity",
				peer: SiteToSitePeer{
					ConnectionType: ConnectionInitiate,
					RemoteAddress:  "172.28.0.3",
					Auth: AuthConfig{
						Mode:      AuthEAPTLS,
						Additions: &AuthAdditions{ServerIdentity: "172.28.0.3"},
					},
				},
			},
			{
				name: "initiate with explicit IPv6 server-identity",
				peer: SiteToSitePeer{
					ConnectionType: ConnectionInitiate,
					RemoteAddress:  "2001:db8::3",
					Auth: AuthConfig{
						Mode:      AuthEAPTLS,
						Additions: &AuthAdditions{ServerIdentity: "2001:db8::3"},
					},
				},
			},
			{
				name: "initiate with explicit NAI server-identity",
				peer: SiteToSitePeer{
					ConnectionType: ConnectionInitiate,
					RemoteAddress:  "172.28.0.3",
					Auth: AuthConfig{
						Mode:      AuthEAPTLS,
						Additions: &AuthAdditions{ServerIdentity: "gw@example.com"},
					},
				},
			},
			{
				name: "initiate with 1-byte and 255-byte server-identity boundaries",
				peer: SiteToSitePeer{
					ConnectionType: ConnectionInitiate,
					RemoteAddress:  "172.28.0.3",
					Auth: AuthConfig{
						Mode:      AuthEAPTLS,
						Additions: &AuthAdditions{ServerIdentity: valid255},
					},
				},
			},
			{
				name: "initiate with 1-byte single-char hostname server-identity",
				peer: SiteToSitePeer{
					ConnectionType: ConnectionInitiate,
					RemoteAddress:  "172.28.0.3",
					Auth: AuthConfig{
						Mode:      AuthEAPTLS,
						Additions: &AuthAdditions{ServerIdentity: "a"},
					},
				},
			},
			{
				name: "initiate with unambiguous 2-label FQDN remote-address fallback",
				peer: SiteToSitePeer{
					ConnectionType: ConnectionInitiate,
					RemoteAddress:  "vpn.example",
					Auth:           AuthConfig{Mode: AuthEAPTLS},
				},
			},
			{
				name: "non-eap-tls modes without either leaf remain accepted",
				peer: SiteToSitePeer{
					ConnectionType: ConnectionInitiate,
					RemoteAddress:  "172.28.0.3",
					Auth:           AuthConfig{Mode: AuthPreSharedSecret, PSK: "secret"},
				},
			},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				cfg := &IPsecConfig{Peers: map[string]SiteToSitePeer{"peer1": tc.peer}}
				if err := cfg.ValidateEAPTLSIdentities(); err != nil {
					t.Fatalf("unexpected refusal: %v", err)
				}
			})
		}
	})

	t.Run("refused respond configurations", func(t *testing.T) {
		const fixSet = "set vpn ipsec site-to-site peer branch authentication allowed-identities <identity>"
		const fixDelAllowed = "delete vpn ipsec site-to-site peer branch authentication allowed-identities"
		const fixDelServer = "delete vpn ipsec site-to-site peer branch authentication server-identity"

		cases := []struct {
			name    string
			peer    SiteToSitePeer
			problem string
			fix     string
		}{
			{
				name: "empty allowed-identities",
				peer: SiteToSitePeer{
					ConnectionType: ConnectionRespond,
					Auth:           AuthConfig{Mode: AuthEAPTLS},
				},
				problem: "has no allowed-identities",
				fix:     fixSet,
			},
			{
				name: "server-identity set on respond peer",
				peer: SiteToSitePeer{
					ConnectionType: ConnectionRespond,
					Auth: AuthConfig{
						Mode: AuthEAPTLS,
						Additions: &AuthAdditions{
							AllowedIdentities: []string{"client.example.com"},
							ServerIdentity:    "vpn.example.com",
						},
					},
				},
				problem: "server-identity with connection-type respond",
				fix:     fixDelServer,
			},
			{
				name: "server-identity set on respond peer even when allowed-identities is empty",
				peer: SiteToSitePeer{
					ConnectionType: ConnectionRespond,
					Auth: AuthConfig{
						Mode:      AuthEAPTLS,
						Additions: &AuthAdditions{ServerIdentity: "vpn.example.com"},
					},
				},
				problem: "server-identity with connection-type respond",
				fix:     fixDelServer,
			},
			{
				name: "wildcard mixed with specific entry",
				peer: SiteToSitePeer{
					ConnectionType: ConnectionRespond,
					Auth: AuthConfig{
						Mode: AuthEAPTLS,
						Additions: &AuthAdditions{
							AllowedIdentities: []string{"client.example.com", "*"},
						},
					},
				},
				problem: "mixes wildcard",
				fix:     fixDelAllowed,
			},
			{
				name: "TLD wildcard *.com",
				peer: SiteToSitePeer{
					ConnectionType: ConnectionRespond,
					Auth: AuthConfig{
						Mode:      AuthEAPTLS,
						Additions: &AuthAdditions{AllowedIdentities: []string{"*.com"}},
					},
				},
				problem: "*.com",
				fix:     fixSet,
			},
			{
				name: "zero-label wildcard *.",
				peer: SiteToSitePeer{
					ConnectionType: ConnectionRespond,
					Auth: AuthConfig{
						Mode:      AuthEAPTLS,
						Additions: &AuthAdditions{AllowedIdentities: []string{"*."}},
					},
				},
				problem: "*.",
				fix:     fixSet,
			},
			{
				name: "multi-wildcard *.*.example.com",
				peer: SiteToSitePeer{
					ConnectionType: ConnectionRespond,
					Auth: AuthConfig{
						Mode:      AuthEAPTLS,
						Additions: &AuthAdditions{AllowedIdentities: []string{"*.*.example.com"}},
					},
				},
				problem: "*.*.example.com",
				fix:     fixSet,
			},
			{
				name: "partial-label wildcard foo*.example.com",
				peer: SiteToSitePeer{
					ConnectionType: ConnectionRespond,
					Auth: AuthConfig{
						Mode:      AuthEAPTLS,
						Additions: &AuthAdditions{AllowedIdentities: []string{"foo*.example.com"}},
					},
				},
				problem: "foo*.example.com",
				fix:     fixSet,
			},
			{
				name: "leading dot subtree .example.com",
				peer: SiteToSitePeer{
					ConnectionType: ConnectionRespond,
					Auth: AuthConfig{
						Mode:      AuthEAPTLS,
						Additions: &AuthAdditions{AllowedIdentities: []string{".example.com"}},
					},
				},
				problem: ".example.com",
				fix:     fixSet,
			},
			{
				name: "single-label NAI realm @localhost",
				peer: SiteToSitePeer{
					ConnectionType: ConnectionRespond,
					Auth: AuthConfig{
						Mode:      AuthEAPTLS,
						Additions: &AuthAdditions{AllowedIdentities: []string{"@localhost"}},
					},
				},
				problem: "@localhost",
				fix:     fixSet,
			},
			{
				name: "empty NAI realm @",
				peer: SiteToSitePeer{
					ConnectionType: ConnectionRespond,
					Auth: AuthConfig{
						Mode:      AuthEAPTLS,
						Additions: &AuthAdditions{AllowedIdentities: []string{"@"}},
					},
				},
				problem: "@",
				fix:     fixSet,
			},
			{
				name: "CIDR prefix in allowed-identities",
				peer: SiteToSitePeer{
					ConnectionType: ConnectionRespond,
					Auth: AuthConfig{
						Mode:      AuthEAPTLS,
						Additions: &AuthAdditions{AllowedIdentities: []string{"192.0.2.0/24"}},
					},
				},
				problem: "192.0.2.0/24",
				fix:     fixSet,
			},
			{
				name: "empty string entry (0 bytes)",
				peer: SiteToSitePeer{
					ConnectionType: ConnectionRespond,
					Auth: AuthConfig{
						Mode:      AuthEAPTLS,
						Additions: &AuthAdditions{AllowedIdentities: []string{""}},
					},
				},
				problem: "invalid authentication allowed-identities",
				fix:     fixSet,
			},
			{
				name: "256-byte entry exceeds maximum length",
				peer: SiteToSitePeer{
					ConnectionType: ConnectionRespond,
					Auth: AuthConfig{
						Mode:      AuthEAPTLS,
						Additions: &AuthAdditions{AllowedIdentities: []string{overlong256}},
					},
				},
				problem: "invalid authentication allowed-identities",
				fix:     fixSet,
			},
			{
				name: "control byte in entry",
				peer: SiteToSitePeer{
					ConnectionType: ConnectionRespond,
					Auth: AuthConfig{
						Mode:      AuthEAPTLS,
						Additions: &AuthAdditions{AllowedIdentities: []string{"client\x00.example.com"}},
					},
				},
				problem: "invalid authentication allowed-identities",
				fix:     fixSet,
			},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				cfg := &IPsecConfig{Peers: map[string]SiteToSitePeer{"branch": tc.peer}}
				requireOperRefusal(t, cfg.ValidateEAPTLSIdentities(), tc.problem, tc.fix)
			})
		}
	})

	t.Run("refused initiate configurations", func(t *testing.T) {
		const fixSet = "set vpn ipsec site-to-site peer hq authentication server-identity <hostname-or-ip>"
		const fixDelAllowed = "delete vpn ipsec site-to-site peer hq authentication allowed-identities"

		cases := []struct {
			name    string
			peer    SiteToSitePeer
			problem string
			fix     string
		}{
			{
				name: "IP literal remote-address without server-identity",
				peer: SiteToSitePeer{
					ConnectionType: ConnectionInitiate,
					RemoteAddress:  "172.28.0.3",
					Auth:           AuthConfig{Mode: AuthEAPTLS},
				},
				problem: "has no server-identity",
				fix:     fixSet,
			},
			{
				name: "single-label remote-address without server-identity",
				peer: SiteToSitePeer{
					ConnectionType: ConnectionInitiate,
					RemoteAddress:  "vpn",
					Auth:           AuthConfig{Mode: AuthEAPTLS},
				},
				problem: "has no server-identity",
				fix:     fixSet,
			},
			{
				name: "empty remote-address without server-identity",
				peer: SiteToSitePeer{
					ConnectionType: ConnectionInitiate,
					Auth:           AuthConfig{Mode: AuthEAPTLS},
				},
				problem: "has no server-identity",
				fix:     fixSet,
			},
			{
				name: "allowed-identities set on initiate peer",
				peer: SiteToSitePeer{
					ConnectionType: ConnectionInitiate,
					RemoteAddress:  "vpn.example.com",
					Auth: AuthConfig{
						Mode:      AuthEAPTLS,
						Additions: &AuthAdditions{AllowedIdentities: []string{"client.example.com"}},
					},
				},
				problem: "allowed-identities with connection-type initiate",
				fix:     fixDelAllowed,
			},
			{
				name: "wildcard in server-identity",
				peer: SiteToSitePeer{
					ConnectionType: ConnectionInitiate,
					RemoteAddress:  "172.28.0.3",
					Auth: AuthConfig{
						Mode:      AuthEAPTLS,
						Additions: &AuthAdditions{ServerIdentity: "*.example.com"},
					},
				},
				problem: "*.example.com",
				fix:     fixSet,
			},
			{
				name: "bare realm in server-identity",
				peer: SiteToSitePeer{
					ConnectionType: ConnectionInitiate,
					RemoteAddress:  "172.28.0.3",
					Auth: AuthConfig{
						Mode:      AuthEAPTLS,
						Additions: &AuthAdditions{ServerIdentity: "@example.com"},
					},
				},
				problem: "@example.com",
				fix:     fixSet,
			},
			{
				name: "256-byte server-identity exceeds maximum length",
				peer: SiteToSitePeer{
					ConnectionType: ConnectionInitiate,
					RemoteAddress:  "172.28.0.3",
					Auth: AuthConfig{
						Mode:      AuthEAPTLS,
						Additions: &AuthAdditions{ServerIdentity: overlong256},
					},
				},
				problem: "invalid authentication server-identity",
				fix:     fixSet,
			},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				cfg := &IPsecConfig{Peers: map[string]SiteToSitePeer{"hq": tc.peer}}
				requireOperRefusal(t, cfg.ValidateEAPTLSIdentities(), tc.problem, tc.fix)
			})
		}
	})

	t.Run("non-eap-tls modes refuse both leaves", func(t *testing.T) {
		for _, mode := range []AuthMode{AuthPreSharedSecret, AuthX509, AuthEAPMSCHAPv2, AuthEAPMD5} {
			cfgAllowed := &IPsecConfig{
				Peers: map[string]SiteToSitePeer{
					"branch": {
						Auth: AuthConfig{
							Mode:      mode,
							Additions: &AuthAdditions{AllowedIdentities: []string{"client.example.com"}},
						},
					},
				},
			}
			requireOperRefusal(t, cfgAllowed.ValidateEAPTLSIdentities(),
				"allowed-identities in "+mode.String()+" mode",
				"delete vpn ipsec site-to-site peer branch authentication allowed-identities")

			cfgServer := &IPsecConfig{
				Peers: map[string]SiteToSitePeer{
					"branch": {
						Auth: AuthConfig{
							Mode:      mode,
							Additions: &AuthAdditions{ServerIdentity: "vpn.example.com"},
						},
					},
				},
			}
			requireOperRefusal(t, cfgServer.ValidateEAPTLSIdentities(),
				"server-identity in "+mode.String()+" mode",
				"delete vpn ipsec site-to-site peer branch authentication server-identity")
		}
	})
}

func buildPeerAuthTree(connType, remoteAddr string, allowed []string, serverID string) *config.Tree {
	auth := config.NewTree()
	auth.Set("mode", "eap-tls")
	auth.Set("certificate", "node-cert")
	auth.Set("ca-certificate", "corp-ca")
	if len(allowed) > 0 {
		auth.SetSlice("allowed-identities", allowed)
	}
	if serverID != "" {
		auth.Set("server-identity", serverID)
	}

	peer := config.NewTree()
	peer.Set("connection-type", connType)
	if remoteAddr != "" {
		peer.Set("remote-address", remoteAddr)
	}
	peer.SetContainer("authentication", auth)

	sts := config.NewTree()
	sts.AddListEntry("peer", "branch", peer)

	ipsecRoot := config.NewTree()
	ipsecRoot.SetContainer("site-to-site", sts)

	vpnRoot := config.NewTree()
	vpnRoot.SetContainer("ipsec", ipsecRoot)

	root := config.NewTree()
	root.SetContainer("vpn", vpnRoot)
	return root
}

func TestParseEAPTLSIdentityPolicyAndEquality(t *testing.T) {
	t.Run("bare peer keeps Additions nil", func(t *testing.T) {
		cfg, err := ParseIPsecConfig(buildPeerAuthTree("initiate", "vpn.example.com", nil, ""))
		if err != nil {
			t.Fatalf("ParseIPsecConfig: %v", err)
		}
		peer := cfg.Peers["branch"]
		if peer.Auth.Additions != nil {
			t.Fatalf("Additions = %+v, want nil when no optional auth additions are set", peer.Auth.Additions)
		}
		if got := peer.EffectiveEAPTLSServerIdentity(); got != "vpn.example.com" {
			t.Fatalf("EffectiveEAPTLSServerIdentity() = %q, want %q", got, "vpn.example.com")
		}
	})

	t.Run("parse allowed-identities and equality across edits", func(t *testing.T) {
		t1 := buildPeerAuthTree("respond", "192.0.2.2", []string{"client.example.com", "@corp.example"}, "")
		t2 := buildPeerAuthTree("respond", "192.0.2.2", []string{"client.example.com", "@corp.example"}, "")
		t3 := buildPeerAuthTree("respond", "192.0.2.2", []string{"other.example.com"}, "")

		c1, err := ParseIPsecConfig(t1)
		if err != nil {
			t.Fatalf("parse c1: %v", err)
		}
		c2, err := ParseIPsecConfig(t2)
		if err != nil {
			t.Fatalf("parse c2: %v", err)
		}
		c3, err := ParseIPsecConfig(t3)
		if err != nil {
			t.Fatalf("parse c3: %v", err)
		}

		want := []string{"client.example.com", "@corp.example"}
		if got := c1.Peers["branch"].Auth.AllowedIdentities(); !slices.Equal(got, want) {
			t.Fatalf("AllowedIdentities() = %v, want %v", got, want)
		}
		if !c1.Peers["branch"].Equal(c2.Peers["branch"]) {
			t.Fatal("two independent parses of the same allowed-identities config did not compare equal")
		}
		if c1.Peers["branch"].Equal(c3.Peers["branch"]) {
			t.Fatal("editing allowed-identities still compared equal, so reload would not restart the peer")
		}
	})

	t.Run("explicit server-identity overrides FQDN remote-address and triggers Equal diff", func(t *testing.T) {
		c1, err := ParseIPsecConfig(buildPeerAuthTree("initiate", "gw.example.com", nil, "pinned.example.com"))
		if err != nil {
			t.Fatalf("parse c1: %v", err)
		}
		c2, err := ParseIPsecConfig(buildPeerAuthTree("initiate", "gw.example.com", nil, "other.example.com"))
		if err != nil {
			t.Fatalf("parse c2: %v", err)
		}

		p1 := c1.Peers["branch"]
		if got := p1.Auth.ServerIdentity(); got != "pinned.example.com" {
			t.Fatalf("ServerIdentity() = %q, want %q", got, "pinned.example.com")
		}
		if got := p1.EffectiveEAPTLSServerIdentity(); got != "pinned.example.com" {
			t.Fatalf("EffectiveEAPTLSServerIdentity() = %q, want explicit override %q", got, "pinned.example.com")
		}
		if p1.Equal(c2.Peers["branch"]) {
			t.Fatal("editing server-identity still compared equal")
		}
	})

	t.Run("EffectiveEAPTLSServerIdentity rejects IP literal and single-label remote-address", func(t *testing.T) {
		for _, remote := range []string{"", "172.28.0.3", "2001:db8::3", "vpn"} {
			p := SiteToSitePeer{RemoteAddress: remote}
			if got := p.EffectiveEAPTLSServerIdentity(); got != "" {
				t.Errorf("RemoteAddress %q: EffectiveEAPTLSServerIdentity() = %q, want empty", remote, got)
			}
		}
	})
}
