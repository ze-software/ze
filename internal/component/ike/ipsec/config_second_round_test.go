// Design: docs/architecture/ike/ipsec-3-data-model.md -- the second-round field of AuthConfig
// Related: second_round.go -- parseSecondRound and ValidateSecondRounds, the producers under test
// RFC: rfc/short/rfc4739.md -- multiple authentication exchanges

package ipsec

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
)

// secondRoundTree builds a peer's authentication container with a first round of
// the given mode and the second-round local and remote containers given, each
// omitted when nil.
func secondRoundTree(mode string, local, remote map[string]string) *config.Tree {
	auth := config.NewTree()
	auth.Set("mode", mode)
	switch mode {
	case "pre-shared-secret":
		auth.Set("pre-shared-secret", "round-one-secret")
	default:
		auth.Set("certificate", "round-one-cert")
		auth.Set("ca-certificate", "round-one-ca")
	}
	auth.Set("certificate-count", "3")
	second := config.NewTree()
	for name, leaves := range map[string]map[string]string{"local": local, "remote": remote} {
		if leaves == nil {
			continue
		}
		direction := config.NewTree()
		for leaf, value := range leaves {
			direction.Set(leaf, value)
		}
		second.SetContainer(name, direction)
	}
	auth.SetContainer("second-round", second)
	return auth
}

// TestParseAuthConfigSecondRound proves every second-round leaf reaches
// AuthConfig.SecondRound, the hex encoding of a round's secret included, and that
// a peer with no second-round container keeps SecondRound nil.
//
// VALIDATES: mode, pre-shared-secret, pre-shared-secret-encoding, id and
// certificate of local, and mode, pre-shared-secret, id, id-type and
// ca-certificate of remote, each in the round it was written under, and the
// round-1 values left where they were.
// PREVENTS: a round-2 leaf silently dropped, or written into round 1, so the
// engine would sign or verify the second round with the first round's credential.
func TestParseAuthConfigSecondRound(t *testing.T) {
	t.Run("absent", func(t *testing.T) {
		tree := config.NewTree()
		tree.Set("mode", "pre-shared-secret")
		tree.Set("pre-shared-secret", "round-one-secret")
		auth, err := parseAuthConfig("branch", tree)
		if err != nil {
			t.Fatalf("parseAuthConfig: %v", err)
		}
		if auth.SecondRound() != nil {
			t.Fatalf("a peer with no second-round container parsed one: %+v", auth.SecondRound())
		}
		if auth.Additions != nil {
			t.Fatalf("Additions = %+v, want nil so a bare peer still equals a bare peer", auth.Additions)
		}
		if auth.HasSecondRound() {
			t.Fatal("HasSecondRound answers true for a peer with no second round")
		}
	})

	t.Run("empty container", func(t *testing.T) {
		auth, err := parseAuthConfig("branch", secondRoundTree("pre-shared-secret", nil, nil))
		if err != nil {
			t.Fatalf("parseAuthConfig: %v", err)
		}
		if auth.SecondRound() != nil {
			t.Fatalf("an empty second-round container parsed a round: %+v", auth.SecondRound())
		}
		if auth.Additions != nil {
			t.Fatalf("Additions = %+v, want nil so a bare peer still equals a bare peer", auth.Additions)
		}
	})

	t.Run("x509 local and psk remote", func(t *testing.T) {
		tree := secondRoundTree("x509",
			map[string]string{"mode": "x509", "id": "user.example.com", "certificate": "user-cert"},
			map[string]string{
				"mode": "pre-shared-secret", "pre-shared-secret": "00ff41",
				"pre-shared-secret-encoding": "hex", "id": "@example.com", "id-type": "rfc822-address",
			})
		auth, err := parseAuthConfig("branch", tree)
		if err != nil {
			t.Fatalf("parseAuthConfig: %v", err)
		}
		if !auth.HasSecondRound() {
			t.Fatal("HasSecondRound answers false for a peer with both rounds")
		}
		local, remote := auth.SecondRound().Local, auth.SecondRound().Remote
		if local == nil {
			t.Fatal("second-round local was not parsed")
		}
		if remote == nil {
			t.Fatal("second-round remote was not parsed")
		}
		if local.Mode != AuthX509 {
			t.Errorf("local mode = %s, want x509", local.Mode)
		}
		if local.LocalID != "user.example.com" {
			t.Errorf("local id = %q, want user.example.com", local.LocalID)
		}
		if local.Certificate != "user-cert" {
			t.Errorf("local certificate = %q, want user-cert", local.Certificate)
		}
		if local.CertificateCount != 3 {
			t.Errorf("local certificate count = %d, want the peer's 3", local.CertificateCount)
		}
		if remote.Mode != AuthPreSharedSecret {
			t.Errorf("remote mode = %s, want pre-shared-secret", remote.Mode)
		}
		if want := string([]byte{0x00, 0xff, 0x41}); remote.PSK != want {
			t.Errorf("remote hex secret decoded to %q, want %q", remote.PSK, want)
		}
		if remote.RemoteID != "@example.com" {
			t.Errorf("remote id = %q, want @example.com", remote.RemoteID)
		}
		if remote.RemoteIDType != idTypeRFC822ForTest(t) {
			t.Errorf("remote id-type = %d, want the rfc822-address type", remote.RemoteIDType)
		}
		if auth.Certificate != "round-one-cert" {
			t.Errorf("round-1 certificate = %q, a round-2 leaf overwrote it", auth.Certificate)
		}
		if auth.LocalID != "" {
			t.Errorf("round-1 local-id = %q, a round-2 id leaked into it", auth.LocalID)
		}
	})

	t.Run("psk local and x509 remote", func(t *testing.T) {
		tree := secondRoundTree("pre-shared-secret",
			map[string]string{"mode": "pre-shared-secret", "pre-shared-secret": "round-two-secret"},
			map[string]string{"mode": "x509", "ca-certificate": "user-ca", "id": "peer.example.com"})
		auth, err := parseAuthConfig("branch", tree)
		if err != nil {
			t.Fatalf("parseAuthConfig: %v", err)
		}
		local, remote := auth.SecondRound().Local, auth.SecondRound().Remote
		if local.PSK != "round-two-secret" {
			t.Errorf("local ascii secret = %q, want round-two-secret", local.PSK)
		}
		if auth.PSK != "round-one-secret" {
			t.Errorf("round-1 secret = %q, a round-2 secret overwrote it", auth.PSK)
		}
		if remote.CACertificate != "user-ca" {
			t.Errorf("remote ca-certificate = %q, want user-ca", remote.CACertificate)
		}
		if remote.RemoteID != "peer.example.com" {
			t.Errorf("remote id = %q, want peer.example.com", remote.RemoteID)
		}
	})

	t.Run("a second-round edit is a peer change", func(t *testing.T) {
		mk := func(secret string) AuthConfig {
			auth, err := parseAuthConfig("branch", secondRoundTree("pre-shared-secret",
				map[string]string{"mode": "pre-shared-secret", "pre-shared-secret": secret}, nil))
			if err != nil {
				t.Fatalf("parseAuthConfig: %v", err)
			}
			return auth
		}
		if mk("one").Equal(mk("two")) {
			t.Fatal("AuthConfig.Equal ignores a second-round secret change, so reload would not restart the peer")
		}
	})

	t.Run("refusals", func(t *testing.T) {
		for _, tc := range []struct {
			name   string
			local  map[string]string
			remote map[string]string
			want   []string
		}{
			{"eap mode in a round", map[string]string{"mode": "eap-tls"}, nil,
				[]string{"second-round local mode", "pre-shared-secret or x509"}},
			{"bad hex", map[string]string{"mode": "pre-shared-secret", "pre-shared-secret": "abc",
				"pre-shared-secret-encoding": "hex"}, nil,
				[]string{"second-round local", "not valid hex"}},
			{"missing mode", nil, map[string]string{"id": "x"},
				[]string{"second-round remote", "mode is missing"}},
			{"bad id-type", nil, map[string]string{"mode": "x509", "id-type": "bogus"},
				[]string{"second-round remote id-type", "bogus"}},
		} {
			t.Run(tc.name, func(t *testing.T) {
				_, err := parseAuthConfig("branch", secondRoundTree("pre-shared-secret", tc.local, tc.remote))
				if err == nil {
					t.Fatal("parseAuthConfig accepted the round")
				}
				for _, want := range tc.want {
					if !strings.Contains(err.Error(), want) {
						t.Errorf("error %q does not contain %q", err, want)
					}
				}
			})
		}
	})
}

// idTypeRFC822ForTest answers the type number the rfc822-address enum maps to.
func idTypeRFC822ForTest(t *testing.T) uint8 {
	t.Helper()
	idType, known := parseRemoteIDType("rfc822-address")
	if !known {
		t.Fatal("rfc822-address is not a known remote-id-type")
	}
	return idType
}

// TestValidateSecondRound proves each AC-11 refusal reaches the operator with the
// peer, the leaf path and the fix, and that a well-formed config of both rounds
// commits.
//
// VALIDATES: a second round on an EAP peer, a round of mode pre-shared-secret with
// no secret, a local x509 round with no certificate, a remote x509 round with no
// ca-certificate, a PKI name the store lacks, a round id the local-id and
// remote-id rules refuse, and a round certificate chain longer than
// certificate-count.
// PREVENTS: a second round that commits and then can never authenticate, with no
// message saying why.
func TestValidateSecondRound(t *testing.T) {
	certCN := func(name string) string {
		if name == "user-cert" {
			return "user.example.com"
		}
		return ""
	}

	validate := func(t *testing.T, round1 AuthMode, local, remote *AuthConfig) error {
		t.Helper()
		auth := AuthConfig{Mode: round1, PSK: "round-one-secret", Certificate: "round-one-cert",
			CACertificate: "round-one-ca", Additions: &AuthAdditions{SecondRound: &SecondRoundAuth{Local: local, Remote: remote}}}
		cfg := &IPsecConfig{Peers: map[string]SiteToSitePeer{"branch": {Name: "branch", Auth: auth}}}
		if err := cfg.ValidateSecondRounds(certCN); err != nil {
			return err
		}
		return cfg.ValidateCertificateChains(func(name string) int {
			if name == "user-cert" {
				return 3
			}
			return 1
		})
	}

	t.Run("accepted", func(t *testing.T) {
		local := &AuthConfig{Mode: AuthX509, Certificate: "user-cert", LocalID: "user.example.com", CertificateCount: 4}
		remote := &AuthConfig{Mode: AuthPreSharedSecret, PSK: "s", RemoteID: ".example.com"}
		if err := validate(t, AuthX509, local, remote); err != nil {
			t.Fatalf("a well-formed second round was refused: %v", err)
		}
	})

	for _, tc := range []struct {
		name   string
		round1 AuthMode
		local  *AuthConfig
		remote *AuthConfig
		want   []string
	}{
		{"eap first round", AuthEAPMSCHAPv2, nil, &AuthConfig{Mode: AuthPreSharedSecret, PSK: "s"},
			[]string{`"branch"`, "authentication second-round", "eap-mschapv2", "Delete authentication second-round"}},
		{"local psk without secret", AuthPreSharedSecret, &AuthConfig{Mode: AuthPreSharedSecret}, nil,
			[]string{`"branch"`, "second-round local mode is pre-shared-secret", "set authentication second-round local pre-shared-secret"}},
		{"remote psk without secret", AuthPreSharedSecret, nil, &AuthConfig{Mode: AuthPreSharedSecret},
			[]string{`"branch"`, "second-round remote mode is pre-shared-secret", "set authentication second-round remote pre-shared-secret"}},
		{"local id not set", AuthPreSharedSecret, &AuthConfig{Mode: AuthPreSharedSecret, PSK: "s"}, nil,
			[]string{`"branch"`, "second-round local id is not set", "set authentication second-round local id"}},
		{"local x509 without certificate", AuthPreSharedSecret, &AuthConfig{Mode: AuthX509, LocalID: "user.example.com"}, nil,
			[]string{`"branch"`, "no certificate", "set authentication second-round local certificate"}},
		{"remote x509 without ca-certificate", AuthPreSharedSecret, nil, &AuthConfig{Mode: AuthX509},
			[]string{`"branch"`, "no ca-certificate", "set authentication second-round remote ca-certificate"}},
		{"local id does not match certificate", AuthPreSharedSecret,
			&AuthConfig{Mode: AuthX509, Certificate: "user-cert", LocalID: "other.example.com"}, nil,
			[]string{"second-round local id", "does not match certificate CN", `"user.example.com"`}},
		{"local id is a set", AuthPreSharedSecret,
			&AuthConfig{Mode: AuthPreSharedSecret, PSK: "s", LocalID: ".example.com"}, nil,
			[]string{"second-round local id", "names a set of identities", "belong in authentication second-round remote id"}},
		{"local id is a distinguished name", AuthPreSharedSecret,
			&AuthConfig{Mode: AuthPreSharedSecret, PSK: "s", LocalID: "CN=user,O=Example"}, nil,
			[]string{"second-round local id", "distinguished name"}},
		{"remote id carries a terminator", AuthPreSharedSecret, nil,
			&AuthConfig{Mode: AuthPreSharedSecret, PSK: "s", RemoteID: "peer\x00.example.com"},
			[]string{"second-round remote id", "terminator octet"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := validate(t, tc.round1, tc.local, tc.remote)
			if err == nil {
				t.Fatal("the config committed")
			}
			for _, want := range tc.want {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not contain %q", err, want)
				}
			}
		})
	}

	// The chain bound is the peer's certificate-count, so a second-round
	// certificate whose chain is longer is refused as a first-round one is.
	t.Run("round chain longer than certificate-count", func(t *testing.T) {
		local := &AuthConfig{Mode: AuthX509, Certificate: "user-cert"}
		auth := AuthConfig{Mode: AuthPreSharedSecret, PSK: "s", CertificateCount: 2,
			Additions: &AuthAdditions{SecondRound: &SecondRoundAuth{Local: local}}}
		cfg := &IPsecConfig{Peers: map[string]SiteToSitePeer{"branch": {Name: "branch", Auth: auth}}}
		err := cfg.ValidateCertificateChains(func(name string) int {
			if name == "user-cert" {
				return 3
			}
			return 1
		})
		if err == nil {
			t.Fatal("a second-round chain of 3 committed under certificate-count 2")
		}
		for _, want := range []string{"second-round local certificate", "holds a chain of 3", "allows 2"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("error %q does not contain %q", err, want)
			}
		}
	})
}
