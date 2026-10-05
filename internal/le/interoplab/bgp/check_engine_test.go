package bgp

import (
	"errors"
	"net/netip"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// TestWithLastOutputCarriesThePeersAnswer proves a timed-out wait names the state the
// peer was in, and that a wait that succeeded or a peer that said nothing is passed
// through unchanged.
//
// VALIDATES: withLastOutput appends the trimmed last output to a non-nil error.
// PREVENTS: a red interop scenario that says only "timed out" while the peer's answer
// (ExStart, in the run that found this) was in hand and dropped.
func TestWithLastOutputCarriesThePeersAnswer(t *testing.T) {
	timedOut := errors.New("wait for frr output timed out")

	got := withLastOutput(timedOut, "  10.0.0.1  1  ExStart/PointToPoint  eth0\n")
	if !errors.Is(got, timedOut) {
		t.Fatalf("the original error is no longer reachable through %v", got)
	}
	if !strings.Contains(got.Error(), "last output:\n10.0.0.1  1  ExStart/PointToPoint  eth0") {
		t.Errorf("the peer's last answer is missing from %q", got.Error())
	}

	if err := withLastOutput(nil, "anything"); err != nil {
		t.Errorf("a wait that succeeded must stay nil, got %v", err)
	}
	if got := withLastOutput(timedOut, "  \n"); got.Error() != timedOut.Error() {
		t.Errorf("a peer that said nothing must leave the error untouched, got %v", got)
	}
}

// TestRewriteOperationJSONFieldsForNetwork checks that a rendered query and its
// JSON expectations name the same selected network without mutating the registry.
func TestRewriteOperationJSONFieldsForNetwork(t *testing.T) {
	t.Parallel()
	registered := operation{
		kind:    opWaitJSONFields,
		command: []string{"show", "172.30.0.9"},
		fields: map[string]string{
			"peer_ip":     "172.30.0.9",
			"bgp_nexthop": "172.30.0.5",
			"bgp_id":      "198.51.100.1",
			"ip_prefix":   "10.99.77.0/24",
		},
	}
	first := registered
	rewriteOperation(interoplab.Network{IPv4: netip.MustParsePrefix("172.31.71.0/24")}, &first)
	if first.command[1] != "172.31.71.9" {
		t.Fatalf("query names %q, want selected-network peer", first.command[1])
	}
	if first.fields["peer_ip"] != "172.31.71.9" {
		t.Errorf("peer expectation=%q, want 172.31.71.9", first.fields["peer_ip"])
	}
	if first.fields["bgp_nexthop"] != "172.31.71.5" {
		t.Errorf("next-hop expectation=%q, want 172.31.71.5", first.fields["bgp_nexthop"])
	}
	if first.fields["bgp_id"] != "198.51.100.1" {
		t.Errorf("fixed Router ID was rewritten: %v", first.fields)
	}
	if first.fields["ip_prefix"] != "10.99.77.0/24" {
		t.Errorf("announced prefix was rewritten: %v", first.fields)
	}
	if registered.fields["peer_ip"] != "172.30.0.9" {
		t.Fatalf("rendering mutated registry expectations: %v", registered.fields)
	}
	second := registered
	rewriteOperation(interoplab.Network{IPv4: netip.MustParsePrefix("172.31.72.0/24")}, &second)
	if second.fields["peer_ip"] != "172.31.72.9" {
		t.Errorf("second run reused another network's expectation: %v", second.fields)
	}
	if first.fields["peer_ip"] != "172.31.71.9" {
		t.Errorf("second run mutated the first run: %v", first.fields)
	}
}
