// Design: docs/architecture/ike/ipsec-3-data-model.md -- IPsec data model
// Related: config.go -- parseSiteToSitePeer, which resolves an absent policy-priority
// RFC: rfc/short/rfc4301.md -- Security Policy Database ordering (Section 4.4.1)
package ipsec

import (
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	ipsecyang "github.com/ze-software/ze/internal/component/ike/ipsec/yang"
)

// VALIDATES: the policy-priority YANG default and dataplane.PriorityChildSA are the same
// number, and the leaf's range starts above the IKE control-plane bypass.
// PREVENTS: two declarations of one fact drifting apart. YANG cannot name a Go constant,
// so the default is written twice: once where the config system reads it and once where
// parseSiteToSitePeer resolves an absent leaf. A disagreement would put a peer with no
// stated order at one rank when the leaf is delivered and at another when it is not, and
// nothing would report it (ai/rules/principles.md, declare a fact once).
func TestPolicyPriorityYANGDefaultMatchesTheGoConstant(t *testing.T) {
	leaf := yangLeaf(t, "policy-priority")

	def := yangStatement(t, leaf, "default")
	n, err := strconv.Atoi(def)
	if err != nil {
		t.Fatalf("the policy-priority default %q is not a number: %v", def, err)
	}
	if n != dataplane.PriorityChildSA {
		t.Errorf("the YANG default is %d and dataplane.PriorityChildSA is %d",
			n, dataplane.PriorityChildSA)
	}

	// The range floor is the other half of the same agreement: ValidatePolicyOrder
	// refuses at or below PriorityIKEBypass, so the leaf must not offer those values.
	rng := yangStatement(t, leaf, "range")
	floor, _, found := strings.Cut(strings.Trim(rng, `"`), "..")
	if !found {
		t.Fatalf("the policy-priority range %q has no lower bound", rng)
	}
	low, err := strconv.Atoi(floor)
	if err != nil {
		t.Fatalf("the policy-priority range floor %q is not a number: %v", floor, err)
	}
	if low != dataplane.PriorityIKEBypass+1 {
		t.Errorf("the YANG range starts at %d, and the first rank ValidatePolicyOrder "+
			"accepts is %d", low, dataplane.PriorityIKEBypass+1)
	}
}

// yangLeaf returns the text of one leaf definition from the embedded IPsec module.
func yangLeaf(t *testing.T, name string) string {
	t.Helper()
	head := "leaf " + name + " {"
	start := strings.Index(ipsecyang.ZeIPsecConfYANG, head)
	if start < 0 {
		t.Fatalf("the embedded module carries no %q leaf", name)
	}
	rest := ipsecyang.ZeIPsecConfYANG[start:]
	// One leaf's body is short, so the next "leaf " at the same nesting bounds it well
	// enough for a default and a range, and the whole tail bounds the last leaf.
	if end := strings.Index(rest[len(head):], "\n                    leaf "); end >= 0 {
		return rest[:len(head)+end]
	}
	return rest
}

// yangStatement returns the argument of the named YANG statement inside a leaf body.
func yangStatement(t *testing.T, body, keyword string) string {
	t.Helper()
	for line := range strings.SplitSeq(body, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || fields[0] != keyword {
			continue
		}
		return strings.TrimSuffix(strings.TrimSuffix(fields[1], ";"), "{")
	}
	t.Fatalf("the leaf body carries no %q statement:\n%s", keyword, body)
	return ""
}
