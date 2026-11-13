package update

import (
	"errors"
	"net/netip"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/textparse"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/core/opererr"
)

// VALIDATES: a default gateway survives the operator's parser as a late-bound
// policy alongside the positively accepted announcement.
// PREVENTS: the parser silently turning a default candidate into an explicit
// BGP next hop before a destination peer can choose its address.
func TestUpdateTextDefaultNextHopPreservesCandidate(t *testing.T) {
	result, err := ParseSendableUpdateText(strings.Fields("next-hop default 192.0.2.1 nlri add 203.0.113.0/24"))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Groups) != 1 {
		t.Fatalf("groups = %d, want one announcement", len(result.Groups))
	}
	group := result.Groups[0]
	if len(group.Announce) != 1 {
		t.Fatalf("announcements = %d, want one", len(group.Announce))
	}
	if group.NextHop.Policy != bgptypes.NextHopDefault {
		t.Fatalf("policy = %d, want Default", group.NextHop.Policy)
	}
	if group.NextHop.Addr != netip.MustParseAddr("192.0.2.1") {
		t.Fatalf("candidate = %s, want 192.0.2.1", group.NextHop.Addr)
	}
}

// VALIDATES: an incomplete default declaration is refused without replacing
// the preceding cursor value, and its error carries the exact repair command.
// PREVENTS: an absent gateway becoming a guessed address or a partial policy.
func TestUpdateTextDefaultNextHopRequiresGateway(t *testing.T) {
	attrs := parsedAttrs{NextHop: netip.MustParseAddr("192.0.2.9"), NextHopPolicy: bgptypes.NextHopExplicit}
	before := attrs
	_, err := parseNhopFlat([]string{"next-hop", "default"}, &attrs)
	var operatorError *opererr.Error
	if !errors.As(err, &operatorError) {
		t.Fatalf("error = %v, want operator refusal", err)
	}
	if operatorError.Why() == "" {
		t.Fatal("refusal does not explain the missing candidate")
	}
	if operatorError.Fix() != "update text next-hop default 192.0.2.1 nlri add 203.0.113.0/24" {
		t.Fatalf("repair = %q", operatorError.Fix())
	}
	if attrs.NextHop != before.NextHop || attrs.NextHopPolicy != before.NextHopPolicy {
		t.Fatal("a refused default declaration changed the previous next hop")
	}
}

// VALIDATES: invalid default gateway inputs are refused as actionable operator
// errors and leave the preceding candidate unchanged.
// PREVENTS: malformed gateway candidates reaching per-peer origination.
func TestUpdateTextDefaultNextHopRejectsUnusableGateway(t *testing.T) {
	for _, value := range []string{"not-an-address", "fe80::gg"} {
		t.Run(value, func(t *testing.T) {
			attrs := parsedAttrs{NextHop: netip.MustParseAddr("192.0.2.9"), NextHopPolicy: bgptypes.NextHopExplicit}
			_, err := parseNhopFlat([]string{"next-hop", "default", value}, &attrs)
			var operatorError *opererr.Error
			if !errors.As(err, &operatorError) {
				t.Fatalf("error = %v, want operator refusal", err)
			}
			if operatorError.Why() == "" {
				t.Fatal("refusal does not explain the invalid gateway")
			}
			if operatorError.Fix() != "update text next-hop default 192.0.2.1 nlri add 203.0.113.0/24" {
				t.Fatalf("repair = %q", operatorError.Fix())
			}
			if attrs.NextHopPolicy != bgptypes.NextHopExplicit {
				t.Fatal("invalid gateway replaced the preceding policy")
			}
			if attrs.NextHop != netip.MustParseAddr("192.0.2.9") {
				t.Fatal("invalid gateway replaced the preceding address")
			}
		})
	}
}

// VALIDATES: clearing a cursor's default next hop removes both its policy and
// address, and a subsequent explicit address takes effect as explicit.
// PREVENTS: a previous default policy reinterpreting an explicit replacement.
func TestCursorDefaultNextHopClearsAndReplaces(t *testing.T) {
	var attrs parsedAttrs
	if _, err := parseNhopFlat([]string{"next-hop", "default", "192.0.2.1"}, &attrs); err != nil {
		t.Fatal(err)
	}
	clearAttr(&attrs, textparse.KWNextHop)
	_, cleared, _ := attrs.snapshot()
	if cleared.IsValid() {
		t.Fatalf("cleared cursor still has next hop %s", cleared)
	}
	if _, err := parseNhopFlat([]string{"next-hop", "192.0.2.9"}, &attrs); err != nil {
		t.Fatal(err)
	}
	_, replacement, _ := attrs.snapshot()
	if replacement.Policy != bgptypes.NextHopExplicit {
		t.Fatalf("replacement policy = %d, want Explicit", replacement.Policy)
	}
	if replacement.Addr != netip.MustParseAddr("192.0.2.9") {
		t.Fatalf("replacement = %s, want 192.0.2.9", replacement.Addr)
	}
}
