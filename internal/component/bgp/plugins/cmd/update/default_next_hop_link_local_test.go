package update

import (
	"net/netip"
	"strings"
	"testing"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
)

// TestUpdateTextDefaultNextHopPreservesLinkLocalCandidate keeps an IGP gateway
// available until the destination chooses a usable next hop for its session.
func TestUpdateTextDefaultNextHopPreservesLinkLocalCandidate(t *testing.T) {
	result, err := ParseSendableUpdateText(strings.Fields("next-hop default fe80::1 nlri add 203.0.113.0/24"))
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
	if group.NextHop.Addr != netip.MustParseAddr("fe80::1") {
		t.Fatalf("candidate = %s, want fe80::1", group.NextHop.Addr)
	}
}
