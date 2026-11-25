// Design: docs/architecture/firewall/fw-6-firewall-vpp.md -- the VPP verifier refuses the destination-local match
// Related: verify.go -- the match switch under test

package firewallvpp

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/firewall"
)

// TestVPPVerifierRefusesLocalDestination proves the VPP verifier refuses the
// destination-local match, which a VPP ACL cannot express, with a message that
// names the match and the backend that can carry it. The control is the same
// term without the match, which the verifier accepts, so the refusal is tied
// to the match and not to the term around it.
func TestVPPVerifierRefusesLocalDestination(t *testing.T) {
	term := func(matches ...firewall.Match) []firewall.Table {
		return simpleTable(baseChain(firewall.Term{
			Name:    "published",
			Matches: matches,
			Actions: []firewall.Action{firewall.Accept{}},
		}))
	}
	port := firewall.MatchDestinationPort{Ranges: []firewall.PortRange{{Lo: 6379, Hi: 6379}}}
	protocol := firewall.MatchProtocol{Protocol: "tcp"}

	if err := Verify(term(protocol, port)); err != nil {
		t.Fatalf("control: the term without destination-local was refused: %v", err)
	}
	err := Verify(term(protocol, port, firewall.MatchDestinationLocal{}))
	if err == nil {
		t.Fatal("a term with destination-local was accepted by the vpp verifier")
	}
	for _, want := range []string{"destination-local", "backend vpp", "use backend kernel"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal %q does not say %q", err.Error(), want)
		}
	}

	// The use that matters: a destination translation in a base NAT chain. The
	// refusal must be the one that names the leaf, never the generic arm that
	// names a Go type.
	const natConfig = `{"firewall":{"table":{"nat":{"family":"ip","chain":{"prerouting":{` +
		`"type":"nat","hook":"prerouting","priority":"-100","policy":"accept",` +
		`"term":{"published":{"from":{"protocol":"tcp","destination-port":"6379","destination-local":""},` +
		`"then":{"dnat":{"to":"192.0.2.10:6379"}}}}}}}}}}`
	nat, err := firewall.ParseFirewallConfig(natConfig)
	if err != nil {
		t.Fatalf("parse the translation: %v", err)
	}
	err = Verify(nat)
	if err == nil {
		t.Fatal("a destination translation with destination-local was accepted by the vpp verifier")
	}
	if !strings.Contains(err.Error(), "destination-local match not supported with dnat by backend vpp") {
		t.Fatalf("the NAT refusal does not name the destination-local match: %v", err)
	}
}
