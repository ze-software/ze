// Design: docs/guide/ipsec.md -- a transport-mode peer carries one traffic selector
// Related: config.go -- validateTransportOnePair, the refusal under test
// Related: ts_narrow.go -- oneTransportPair and errTSTransportPairs, the negotiation-time rule it mirrors

package engine

import (
	"errors"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
)

// TestTransportPeerWithSeveralSelectorsIsRefused checks the config-verify refusal of a
// transport-mode peer configured with more than one traffic-selector entry.
//
// VALIDATES: two single-host entries on a transport-mode peer fail with
// ErrTrafficSelectorPolicy naming the peer, the count and both fixes; one entry on a
// transport-mode peer, and two entries on a tunnel-mode peer, pass.
// PREVENTS: a list that commits and then fails at negotiation, where the responder keeps
// the first pair and the initiator deletes the SA (errTSTransportPairs).
func TestTransportPeerWithSeveralSelectorsIsRefused(t *testing.T) {
	two := []ipsec.TrafficSelectorPolicy{
		comboPolicy(t, "1", "10.0.0.1/32", "10.0.0.2/32"),
		comboPolicy(t, "2", "10.0.0.1/32", "10.0.0.3/32"),
	}
	peers := func(mode uint8, ts []ipsec.TrafficSelectorPolicy) *ipsec.IPsecConfig {
		return &ipsec.IPsecConfig{Peers: map[string]ipsec.SiteToSitePeer{
			"p": {Name: "p", Mode: mode, TrafficSelectors: ts},
		}}
	}

	err := validateTransportOnePair(peers(dataplane.ModeTransport, two))
	if !errors.Is(err, ipsec.ErrTrafficSelectorPolicy) {
		t.Fatalf("transport peer with two selectors: err = %v, want ErrTrafficSelectorPolicy", err)
	}
	for _, want := range []string{`peer "p"`, "2 traffic-selector entries", "exactly one selector pair", "RFC 4301 Section 5.2", "set mode tunnel"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal %q does not name %q", err, want)
		}
	}

	if err := validateTransportOnePair(peers(dataplane.ModeTransport, two[:1])); err != nil {
		t.Errorf("transport peer with one selector was refused: %v", err)
	}
	if err := validateTransportOnePair(peers(dataplane.ModeTunnel, two)); err != nil {
		t.Errorf("tunnel peer with two selectors was refused: %v", err)
	}
}
