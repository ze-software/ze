// VALIDATES: RFC 4301 Section 5, "If no policy is found in the SPD that matches a
// packet (for either inbound or outbound traffic), the packet MUST be discarded."
// The Linux kernel passes an unmatched packet in the clear, so the discard is an
// entry Ze installs: a wildcard catch-all ranked after every other entry, in every
// direction, for both address families, carrying the disposition the operator chose.
// PREVENTS: a catch-all that outranks an operator entry or a Child SA policy, one that
// misses a direction or a family, and a disposition the operator never wrote.

package engine

import (
	"net"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// operatorOrderMax is the upper bound of the `order` leaf (ze-ipsec-conf.yang,
// range "101..2147483647"). The catch-all must rank after it.
const operatorOrderMax = 2147483647

// RFC requirement: RFC4301-5-1 positive -- the discard catch-all is a wildcard entry in
// SPD-I, SPD-O and the forward half, for IPv4 and IPv6, ranked after every operator
// order and every Child SA policy, with no template.
func TestRFC4301UnmatchedDiscardIsTheLastEntryOfEveryDatabase(t *testing.T) {
	for _, family := range []net.IP{net.IPv4zero, net.IPv6zero} {
		policies, err := unmatchedPolicies(family, dataplane.SPActionDiscard)
		if err != nil {
			t.Fatalf("family %v: unmatchedPolicies: %v", family, err)
		}
		if len(policies) != 3 {
			t.Fatalf("family %v: got %d policies, want in, out and fwd", family, len(policies))
		}
		seen := map[dataplane.SADir]bool{}
		for _, p := range policies {
			seen[p.Dir] = true
			if p.Action != dataplane.SPActionDiscard {
				t.Errorf("family %v dir %d: action %d, want discard", family, p.Dir, p.Action)
			}
			if ones, _ := p.Src.Mask.Size(); ones != 0 {
				t.Errorf("family %v dir %d: source %s is not the wildcard", family, p.Dir, p.Src)
			}
			if ones, _ := p.Dst.Mask.Size(); ones != 0 {
				t.Errorf("family %v dir %d: destination %s is not the wildcard", family, p.Dir, p.Dst)
			}
			if (p.Src.IP.To4() != nil) != (family.To4() != nil) {
				t.Errorf("family %v dir %d: selector %s is the wrong family", family, p.Dir, p.Src)
			}
			if p.Priority <= operatorOrderMax || p.Priority <= dataplane.PriorityChildSA {
				t.Errorf("family %v dir %d: priority %d does not rank after every operator order (max %d) and Child SA (%d)",
					family, p.Dir, p.Priority, operatorOrderMax, dataplane.PriorityChildSA)
			}
			if p.UpperProto != 0 || !p.SrcPort.IsAny() || !p.DstPort.IsAny() {
				t.Errorf("family %v dir %d: selector is not any protocol, any port", family, p.Dir)
			}
			if p.Mode != 0 || p.ReqID != 0 || p.TunnelSrc != nil || p.TunnelDst != nil {
				t.Errorf("family %v dir %d: a discard carries a template", family, p.Dir)
			}
		}
		for _, dir := range []dataplane.SADir{dataplane.SADirIn, dataplane.SADirOut, dataplane.SADirFwd} {
			if !seen[dir] {
				t.Errorf("family %v: no catch-all in direction %d", family, dir)
			}
		}
	}

	dp := &bypassDP{}
	installUnmatched(dp, dataplane.SPActionDiscard, slogutil.Logger("test"))
	if len(dp.installed) != 6 {
		t.Fatalf("installed %d policies, want 3 directions x 2 families", len(dp.installed))
	}
	for _, p := range dp.installed {
		if p.Action != dataplane.SPActionDiscard {
			t.Errorf("installed dir %d action %d, want discard", p.Dir, p.Action)
		}
	}
}

// RFC requirement: RFC4301-5-1 negative -- a bypass disposition never installs a
// discard, a disposition that is neither is refused with no entry built, and no
// catch-all is ever ranked at or above an operator entry.
func TestRFC4301UnmatchedNeverDiscardsWhatTheOperatorDidNotAskToDiscard(t *testing.T) {
	dp := &bypassDP{}
	installUnmatched(dp, dataplane.SPActionBypass, slogutil.Logger("test"))
	if len(dp.installed) != 6 {
		t.Fatalf("installed %d policies, want 6", len(dp.installed))
	}
	for _, p := range dp.installed {
		if p.Action == dataplane.SPActionDiscard {
			t.Errorf("dir %d: a bypass catch-all reached the dataplane as a discard", p.Dir)
		}
		if p.Priority <= operatorOrderMax {
			t.Errorf("dir %d: priority %d is at or above an operator order", p.Dir, p.Priority)
		}
	}

	policies, err := unmatchedPolicies(net.IPv4zero, dataplane.SPActionProtect)
	if err == nil {
		t.Fatal("a protect catch-all was built; it names no transform to hand traffic to")
	}
	if len(policies) != 0 {
		t.Fatalf("a refused disposition still built %d policies", len(policies))
	}

	refused := &bypassDP{}
	installUnmatched(refused, dataplane.SPActionProtect, slogutil.Logger("test"))
	if len(refused.installed) != 0 {
		t.Fatalf("a refused disposition installed %d policies", len(refused.installed))
	}
}
