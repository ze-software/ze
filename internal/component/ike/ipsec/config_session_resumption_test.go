// Design: docs/architecture/ike/ipsec-14-responder.md -- the session-resumption leaf
// Related: config_auth_policy.go -- parseSessionResumption, the producer under test
// RFC: rfc/short/rfc9190.md -- Sections 2.1.2 and 2.1.3
//
// VALIDATES: the session-resumption leaf an operator writes reaches AuthConfig,
// and a peer that never names it arrives with the YANG default of true.
// PREVENTS: the Go zero value standing in for that default, which would turn
// EAP-TLS resumption off for every peer whose config does not mention it while
// `show configuration` reported it on.

package ipsec

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
)

// resumptionAuthTree builds the authentication container of an EAP-TLS peer,
// with the session-resumption leaf set to the text given, or absent when it is
// empty.
func resumptionAuthTree(leaf string) *config.Tree {
	auth := config.NewTree()
	auth.Set("mode", "eap-tls")
	auth.Set("certificate", "node-cert")
	auth.Set("ca-certificate", "node-ca")
	if leaf != "" {
		auth.Set("session-resumption", leaf)
	}
	return auth
}

func TestSessionResumptionDefaultsToTheYANGDefault(t *testing.T) {
	auth, err := parseAuthConfig("branch", resumptionAuthTree(""))
	if err != nil {
		t.Fatalf("parseAuthConfig: %v", err)
	}
	if !auth.SessionResumption {
		t.Fatal("a peer that never named session-resumption arrived with it off, " +
			"so the YANG default of true is not what ze runs")
	}
}

func TestSessionResumptionReadsBothPolarities(t *testing.T) {
	for _, tc := range []struct {
		leaf string
		want bool
	}{
		{"true", true},
		{"false", false},
	} {
		t.Run(tc.leaf, func(t *testing.T) {
			auth, err := parseAuthConfig("branch", resumptionAuthTree(tc.leaf))
			if err != nil {
				t.Fatalf("parseAuthConfig: %v", err)
			}
			if auth.SessionResumption != tc.want {
				t.Fatalf("session-resumption %q parsed to %v, want %v", tc.leaf, auth.SessionResumption, tc.want)
			}
		})
	}
}

// TestSessionResumptionRefusesANonBoolean keeps a mistyped value from being
// read as the default. An operator who wrote `yes` asked for something, and
// defaulting would leave the daemon doing the opposite of what they can see in
// the config with nothing saying so.
func TestSessionResumptionRefusesANonBoolean(t *testing.T) {
	_, err := parseAuthConfig("branch", resumptionAuthTree("yes"))
	if err == nil {
		t.Fatal("parseAuthConfig accepted session-resumption yes")
	}
	if !strings.Contains(err.Error(), "session-resumption") {
		t.Fatalf("the error does not name the leaf: %v", err)
	}
}

// TestSessionResumptionIsPartOfPeerEquality keeps a reload noticing the edit.
// AuthConfig.Equal decides whether a running peer session is restarted, and a
// member it did not compare would leave the operator's edit committed and
// unapplied.
func TestSessionResumptionIsPartOfPeerEquality(t *testing.T) {
	on, err := parseAuthConfig("branch", resumptionAuthTree("true"))
	if err != nil {
		t.Fatalf("parseAuthConfig: %v", err)
	}
	off, err := parseAuthConfig("branch", resumptionAuthTree("false"))
	if err != nil {
		t.Fatalf("parseAuthConfig: %v", err)
	}
	if on.Equal(off) {
		t.Fatal("two auth configs differing only in session-resumption compare equal, " +
			"so a reload that turned it off would never restart the peer")
	}
}
