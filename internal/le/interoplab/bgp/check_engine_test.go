package bgp

import (
	"net/netip"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// TestFailedCheckShowsThePeerOutputItSearched proves every one-shot assertion that
// read the peer's answer carries that answer when it fails, so a red names what the
// peer printed beside what the checker expected.
//
// VALIDATES: requireContains, requireAbsentWithProof and requireJSONFields wrap
// their verdict with the searched output; a pass stays nil.
// PREVENTS: bgp-addpath-frr's red, where FRR 10.3 printed "Paths: (2 available"
// and the checker said only that "2 paths" was missing, so reading the answer took
// a temporary code edit.
func TestFailedCheckShowsThePeerOutputItSearched(t *testing.T) {
	t.Parallel()
	frr := "BGP routing table entry for 10.0.0.0/24\nPaths: (2 available, best #1, table default)"

	err := requireContains(frr, "BGP routing table", "2 paths")
	if err == nil {
		t.Fatal("a missing needle must fail")
	}
	for _, want := range []string{`peer output is missing "2 paths"`, "searched output:\n", "Paths: (2 available"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("requireContains error lacks %q: %q", want, err.Error())
		}
	}
	if err := requireContains(frr, "Paths:"); err != nil {
		t.Errorf("a present needle must pass, got %v", err)
	}

	err = requireAbsentWithProof(frr, []string{"best #1"}, []string{"Paths:"})
	if err == nil || !strings.Contains(err.Error(), `unexpectedly contains "best #1"`) || !strings.Contains(err.Error(), "Paths: (2 available") {
		t.Errorf("an absence failure must name the needle and the searched output, got %v", err)
	}
	err = requireAbsentWithProof("session Idle", []string{"x"}, []string{"Established"})
	if err == nil || !strings.Contains(err.Error(), "searched output:\nsession Idle") {
		t.Errorf("an unexercised absence must carry the searched output, got %v", err)
	}

	err = requireJSONFields(`{"state":"Idle"}`, map[string]string{"state": "Established"}, nil)
	if err == nil || !strings.Contains(err.Error(), "searched output:\n"+`{"state":"Idle"}`) {
		t.Errorf("a JSON failure must carry the searched output, got %v", err)
	}

	err = requireContains("", "Established")
	if err == nil || !strings.Contains(err.Error(), "(the peer printed nothing)") {
		t.Errorf("an empty answer must be named as empty, got %v", err)
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
