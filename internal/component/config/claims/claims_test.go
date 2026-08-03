package claims

import (
	"strings"
	"testing"
)

// tree builds a Node tree from "a/b/c" leaf paths. Every path ends in a leaf,
// every intermediate segment is a container.
func tree(module string, leafPaths ...string) *Node {
	root := &Node{Children: map[string]*Node{}}
	for _, p := range leafPaths {
		cur := root
		segs := strings.Split(p, "/")
		for i, seg := range segs {
			child := cur.Children[seg]
			if child == nil {
				path := seg
				if cur.Path != "" {
					path = cur.Path + "/" + seg
				}
				child = &Node{Path: path, Children: map[string]*Node{}}
				cur.Children[seg] = child
			}
			child.Modules = []string{module}
			if i == len(segs)-1 {
				child.IsLeaf = true
			}
			cur = child
		}
	}
	return root
}

func findings(r Report, k Kind) []Finding {
	var out []Finding
	for _, f := range r.Findings {
		if f.Kind == k {
			out = append(out, f)
		}
	}
	return out
}

// TestAuditReportsUnclaimedSubtree checks the gate's core judgement.
//
// VALIDATES: AC-1 -- a config-schema root no claim covers is reported, naming
// the module, the root, and the nearest existing claim.
// PREVENTS: a YANG config module reaching operators with no delivery surface.
func TestAuditReportsUnclaimedSubtree(t *testing.T) {
	root := tree("ze-widget-conf", "widget/size", "bgp/local-as")
	cs := []Claim{{Path: "bgp", Source: "plugin:bgp"}}

	r := Audit(root, cs, nil)

	got := findings(r, KindUnclaimed)
	if len(got) != 1 {
		t.Fatalf("want 1 unclaimed finding, got %d: %v", len(got), r.Findings)
	}
	if got[0].Path != "widget" {
		t.Errorf("want the unclaimed root reported as %q, got %q", "widget", got[0].Path)
	}
	for _, want := range []string{"ze-widget-conf", "1 config leaves", "Nearest existing claim:"} {
		if !strings.Contains(got[0].String(), want) {
			t.Errorf("finding does not name %q: %s", want, got[0].String())
		}
	}
}

// TestAuditNamesTheNearestClaim checks the hint AC-1 asks for.
//
// The usual cause of an unclaimed subtree is a claim one segment away, so the
// finding names the claim sharing the longest path prefix. With no shared
// segment it says so rather than pointing at an unrelated claim.
//
// VALIDATES: AC-1 -- the finding carries the nearest existing claim.
// PREVENTS: a failure that says a subtree is unclaimed without saying what the
// nearby claims are.
func TestAuditNamesTheNearestClaim(t *testing.T) {
	root := tree("ze-anomaly-conf", "anomaly/detect/threshold", "anomaly/shape/rate")
	cs := []Claim{
		{Path: "anomaly/detect", Source: "plugin:anomaly-detect"},
		{Path: "bgp", Source: "plugin:bgp"},
	}

	r := Audit(root, cs, nil)

	got := findings(r, KindUnclaimed)
	if len(got) != 1 {
		t.Fatalf("want 1 unclaimed finding, got %d: %v", len(got), r.Findings)
	}
	if !strings.Contains(got[0].String(), "Nearest existing claim: anomaly/detect") {
		t.Errorf("want the sibling claim named, got: %s", got[0].String())
	}

	unrelated := Audit(tree("m", "widget/size"), []Claim{{Path: "bgp", Source: "plugin:bgp"}}, nil)
	if got := findings(unrelated, KindUnclaimed); len(got) != 1 ||
		!strings.Contains(got[0].String(), "Nearest existing claim: none") {
		t.Errorf("with no shared segment the finding must say none, got: %v", unrelated.Findings)
	}
}

// TestAuditAcceptsCoveredSubtrees checks that the audit models the production
// matcher rather than a stricter one.
//
// rootHasChanges (internal/component/plugin/server/reload.go) matches a claim
// against its own path and everything below it, and treats "*" as every root.
// A hub handler path claims the same way through SchemaRegistry.FindHandler.
//
// VALIDATES: AC-2 -- a claimed root, a root claimed by a deeper path's parent,
// a hub handler path, and a wildcard claim are all silent.
// PREVENTS: false positives that would push real claims onto the allowlist.
func TestAuditAcceptsCoveredSubtrees(t *testing.T) {
	cases := []struct {
		name  string
		tree  *Node
		claim Claim
	}{
		{"exact root", tree("m", "bgp/local-as"), Claim{Path: "bgp", Source: "plugin:bgp"}},
		{"subtree below the claim", tree("m", "bgp/peer/timers/hold"), Claim{Path: "bgp", Source: "plugin:bgp"}},
		{"hub handler path", tree("m", "bgp/peer/hold"), Claim{Path: "bgp", Source: "hub-handler"}},
		{"wildcard", tree("m", "anything/at/all"), Claim{Path: "*", Source: "plugin:catch-all"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := Audit(tc.tree, []Claim{tc.claim}, nil)
			if got := findings(r, KindUnclaimed); len(got) != 0 {
				t.Errorf("claim %q should cover the tree, got %v", tc.claim.Path, got)
			}
		})
	}
}

// TestAuditDescendsPastStructuralContainer checks the one case where a root is
// itself unclaimed and that is correct.
//
// Plugins claim deeper paths than the top-level root: anomaly-detect claims
// "anomaly/detect". The container "anomaly" is covered by nobody, and reporting
// it would be wrong. A leaf beside the claimed children is still a finding.
//
// VALIDATES: AC-1, AC-2 boundary -- claim granularity is the declared path, not
// the top-level root.
// PREVENTS: reporting every augment target as unclaimed.
func TestAuditDescendsPastStructuralContainer(t *testing.T) {
	root := tree("ze-anomaly-conf", "anomaly/detect/threshold", "anomaly/orphan-leaf")
	cs := []Claim{{Path: "anomaly/detect", Source: "plugin:anomaly-detect"}}

	r := Audit(root, cs, nil)

	got := findings(r, KindUnclaimed)
	if len(got) != 1 {
		t.Fatalf("want 1 unclaimed finding, got %d: %v", len(got), r.Findings)
	}
	if got[0].Path != "anomaly/orphan-leaf" {
		t.Errorf("want %q reported, got %q", "anomaly/orphan-leaf", got[0].Path)
	}
}

// TestAuditReportsLeaflessPresenceContainer checks that an empty container is
// not treated as an empty finding.
//
// A presence container carries data by existing: `connected { }` enables the
// connected plugin and holds no leaf (ze-connected-conf.yang). A gate that
// skipped a zero leaf count would leave the emptiest config surface the one
// nothing checks (ai/rules/fail-closed-guards.md).
//
// VALIDATES: AC-1 for a container with no leaves.
// PREVENTS: a presence-only root reaching operators with no delivery surface.
func TestAuditReportsLeaflessPresenceContainer(t *testing.T) {
	root := &Node{Children: map[string]*Node{
		"connected": {Path: "connected", Modules: []string{"ze-connected-conf"}, Children: map[string]*Node{}},
	}}
	cs := []Claim{{Path: "bgp", Source: "plugin:bgp"}}

	r := Audit(root, cs, nil)

	got := findings(r, KindUnclaimed)
	if len(got) != 1 || got[0].Path != "connected" {
		t.Fatalf("want the presence container reported, got %v", r.Findings)
	}
	if !strings.Contains(got[0].String(), "0 config leaves") {
		t.Errorf("the finding should say the subtree has no leaves: %s", got[0].String())
	}
}

// TestAuditReportsPhantomClaim checks the inverse comparison.
//
// VALIDATES: AC-3 -- a claim naming no schema node is reported with the
// claiming plugin.
// PREVENTS: a typo'd ConfigRoots entry leaving a plugin never selected by
// reloadConfig and never configured.
func TestAuditReportsPhantomClaim(t *testing.T) {
	root := tree("ze-bgp-conf", "bgp/local-as")
	cs := []Claim{
		{Path: "bgp", Source: "plugin:bgp"},
		{Path: "bpg", Source: "plugin:typo"},
	}

	r := Audit(root, cs, nil)

	got := findings(r, KindPhantomClaim)
	if len(got) != 1 {
		t.Fatalf("want 1 phantom finding, got %d: %v", len(got), r.Findings)
	}
	if got[0].Path != "bpg" || !strings.Contains(got[0].String(), "plugin:typo") {
		t.Errorf("phantom finding must name the path and the plugin: %s", got[0].String())
	}
}

// TestAuditSkipsAllowlistedPath checks that a recorded exception suppresses the
// finding and stays visible.
//
// VALIDATES: AC-4.
// PREVENTS: an allowlist that silences a subtree without saying it did.
func TestAuditSkipsAllowlistedPath(t *testing.T) {
	root := tree("ze-widget-conf", "widget/size")
	cs := []Claim{{Path: "bgp", Source: "plugin:bgp"}}
	allow := []Allow{{Path: "widget", Reason: "read by the hub", Owner: "cmd/ze/hub/main.go extractWidget"}}

	r := Audit(root, cs, allow)

	if got := findings(r, KindUnclaimed); len(got) != 0 {
		t.Errorf("allowlisted path should not be reported: %v", got)
	}
	if len(r.Allowlisted) != 1 || r.Allowlisted[0] != "widget" {
		t.Errorf("want the skipped path listed, got %v", r.Allowlisted)
	}
}

// TestAuditRejectsAllowlistEntryWithoutJustification checks the guard on the
// guard: an exception with no reason or no owner is not an exception.
//
// VALIDATES: AC-5.
// PREVENTS: the allowlist rotting into a dumping ground (spec improve-7 R-1).
func TestAuditRejectsAllowlistEntryWithoutJustification(t *testing.T) {
	root := tree("ze-widget-conf", "widget/size")
	cs := []Claim{{Path: "bgp", Source: "plugin:bgp"}}

	cases := []struct {
		name  string
		allow Allow
	}{
		{"no reason", Allow{Path: "widget", Owner: "somebody"}},
		{"blank reason", Allow{Path: "widget", Reason: "   ", Owner: "somebody"}},
		{"no owner", Allow{Path: "widget", Reason: "read by the hub"}},
		{"no path", Allow{Reason: "read by the hub", Owner: "somebody"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := Audit(root, cs, []Allow{tc.allow})
			if got := findings(r, KindAllowlistNoReason); len(got) != 1 {
				t.Fatalf("want the entry rejected, got %v", r.Findings)
			}
			// The rejected entry must not also have suppressed the finding it
			// was written to cover, or a reasonless entry would still work.
			if tc.allow.Path != "" {
				if got := findings(r, KindUnclaimed); len(got) != 1 {
					t.Errorf("a rejected allowlist entry must not suppress its subtree: %v", r.Findings)
				}
			}
		})
	}
}

// TestAuditReportsStaleAllowlistEntry checks that an exception is removed when
// its reason expires.
//
// VALIDATES: AC-4 hygiene -- an allowlist entry that is now claimed, or names
// no schema node, is a finding.
// PREVENTS: an allowlist that keeps growing because nothing ever leaves it.
func TestAuditReportsStaleAllowlistEntry(t *testing.T) {
	root := tree("ze-widget-conf", "widget/size")

	t.Run("now claimed", func(t *testing.T) {
		cs := []Claim{{Path: "widget", Source: "plugin:widget"}}
		r := Audit(root, cs, []Allow{{Path: "widget", Reason: "was unclaimed", Owner: "nobody"}})
		if got := findings(r, KindAllowlistStale); len(got) != 1 {
			t.Fatalf("want the claimed entry reported stale, got %v", r.Findings)
		}
	})

	t.Run("names no node", func(t *testing.T) {
		cs := []Claim{{Path: "widget", Source: "plugin:widget"}}
		r := Audit(root, cs, []Allow{{Path: "gone", Reason: "deleted module", Owner: "nobody"}})
		if got := findings(r, KindAllowlistStale); len(got) != 1 {
			t.Fatalf("want the dangling entry reported stale, got %v", r.Findings)
		}
	})
}

// TestAuditFailsClosed checks the audit reports what it cannot judge.
//
// A guard that returns a clean report when its inputs are missing is worse than
// no guard: it reads as a pass (ai/rules/fail-closed-guards.md).
//
// VALIDATES: fail-closed behavior for an empty schema, an empty claim set, and
// a claim with no path.
// PREVENTS: the gate going green because enumeration broke.
func TestAuditFailsClosed(t *testing.T) {
	cases := []struct {
		name  string
		root  *Node
		cs    []Claim
		wantK Kind
	}{
		{"nil tree", nil, []Claim{{Path: "bgp", Source: "plugin:bgp"}}, KindUnclassifiable},
		{"empty tree", &Node{Children: map[string]*Node{}}, []Claim{{Path: "bgp"}}, KindUnclassifiable},
		{"no claims", tree("m", "bgp/local-as"), nil, KindUnclassifiable},
		{"claim with no path", tree("m", "bgp/local-as"), []Claim{{Source: "plugin:broken"}}, KindUnclassifiable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := Audit(tc.root, tc.cs, nil)
			if !r.Failed() {
				t.Fatal("audit returned a clean report for an input it cannot judge")
			}
			if got := findings(r, tc.wantK); len(got) == 0 {
				t.Errorf("want a %s finding, got %v", tc.wantK, r.Findings)
			}
		})
	}
}

// TestAllowlistParses checks the shipped allowlist is readable and complete.
//
// VALIDATES: AC-5 over the real file rather than a fixture.
// PREVENTS: a malformed or unjustified entry shipping.
func TestAllowlistParses(t *testing.T) {
	allow, err := Allowlist()
	if err != nil {
		t.Fatalf("Allowlist: %v", err)
	}
	for _, a := range allow {
		if a.Path == "" || strings.TrimSpace(a.Reason) == "" || strings.TrimSpace(a.Owner) == "" {
			t.Errorf("allowlist entry %+v is missing a path, a reason, or an owner", a)
		}
	}
}
