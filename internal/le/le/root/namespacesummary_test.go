// Related: namespacesummary.go -- the namespace registry these tests pin

package leroot

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command/registry"
)

// init registers a summary for every namespace the probes of this package's
// tests register areas under, because the first Dispatch or Tree runs the
// startup check over whatever is registered by then, and a probe namespace
// with no summary would stop the test binary there.
func init() {
	for _, namespace := range []string{
		"action-shape", "help-namespace-probe", "probe-renamed", "treeprobe",
		"probe-ns", "probe-deep", "probe-bare", "probe-members", "probe-lookup",
	} {
		RegisterNamespace(namespace, "a probe namespace")
	}
}

// TestANamespaceWithoutASummaryStopsLeNamingIt proves AC-53 (D-P7-namespace):
// a namespace whose areas registered and that registered no summary stops le
// before its first answer, and the stop names the namespace and the call that
// fixes it. A namespace holding a summary, and a first word that is itself an
// area, pass the same check. Method: run the check over named areas.
// VALIDATES: requireEveryNamespaceHasASummary, run once by Dispatch and Tree.
// PREVENTS: a sub-menu with no description in the menu, on the root page and
// in `le help ai`.
func TestANamespaceWithoutASummaryStopsLeNamingIt(t *testing.T) {
	RegisterNamespace("zz-nscheck-held", "a namespace that says what it is for")
	requireEveryNamespaceHasASummary([]string{"zz-nscheck-held member", "zz-nscheck-area", "zz-nscheck-area member"})

	const missing = "zz-nscheck-missing"
	defer func() {
		stop, _ := recover().(string)
		if !strings.Contains(stop, `"`+missing+`"`) {
			t.Fatalf("the stop does not name the namespace %q: %q", missing, stop)
		}
		if !strings.Contains(stop, `leroot.RegisterNamespace("`+missing+`", `) {
			t.Fatalf("the stop does not name the call that fixes it: %q", stop)
		}
		if !strings.Contains(stop, "internal/le/"+missing+"/register.go") {
			t.Fatalf("the stop does not name the file the call goes in: %q", stop)
		}
	}()
	requireEveryNamespaceHasASummary([]string{"zz-nscheck-held member", missing + " member"})
	t.Fatal("a namespace with no summary passed the check")
}

// TestANamespaceThatIsAlsoAnAreaIsDescribedOnce proves a namespace
// registration for a name that is also an area is refused as a second
// description: the area's own summary is the one le shows. Method: register
// the namespace, then run the check over an area of the same name.
func TestANamespaceThatIsAlsoAnAreaIsDescribedOnce(t *testing.T) {
	const both = "zz-nscheck-both"
	RegisterNamespace(both, "a second description")
	defer func() {
		stop, _ := recover().(string)
		if !strings.Contains(stop, `"`+both+`" is also an area`) {
			t.Fatalf("the stop does not name the double declaration: %q", stop)
		}
	}()
	requireEveryNamespaceHasASummary([]string{both, both + " member"})
	t.Fatal("a namespace that is also an area passed the check")
}

// TestTheTreeAndTheRootPageShowANamespaceSummary proves the menu's node for a
// namespace carries its registered summary, a namespace that is also an area
// carries the area's own, and the root page heads the namespace's block with
// it. Method: build the tree and the root page over pinned areas.
func TestTheTreeAndTheRootPageShowANamespaceSummary(t *testing.T) {
	const namespace, summary = "zz-nsshow", "the namespace's own line. More words"
	RegisterNamespace(namespace, summary)
	meta := registry.Meta{ShortHelp: "the area's own line", Mode: "offline", Section: registry.SectionTest}
	tree := treeFrom([]registry.RootCommand{
		{Name: namespace + " member", Meta: meta},
		{Name: "zz-nsshow-area", Meta: meta},
		{Name: "zz-nsshow-area member", Meta: meta},
	})
	if got := tree.Children[namespace].ShortHelp; got != summary {
		t.Errorf("the namespace node says %q, want %q", got, summary)
	}
	if got := tree.Children["zz-nsshow-area"].ShortHelp; got != meta.ShortHelp {
		t.Errorf("the area's node says %q, want the area's own %q", got, meta.ShortHelp)
	}

	page := Manifest{Program: "le", Areas: []ManifestArea{{Name: namespace + " member", Description: "a member"}}}.page()
	var out strings.Builder
	page.WriteTo(&out, false)
	if want := namespace + ": the namespace's own line."; !strings.Contains(out.String(), want) {
		t.Errorf("the root page does not head the block with %q:\n%s", want, out.String())
	}
}
