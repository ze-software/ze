// Design: docs/research/l2tpv2-ze-integration.md -- PPP auth-phase event boundary
// Related: auth_events.go -- AuthMethod.String, the one place this vocabulary is spelled
//
// AuthMethod.String names each PPP authentication protocol, ParseAuthMethod reads the
// name back, and two YANG modules decide which names an operator can type: the LNS
// reads `l2tp auth-method` and the PPPoE access concentrator reads `pppoe auth-method`.
// Neither side derives from the other, so the three are gated against each other here.
// A method added to the type without its leaf, or to a leaf without its arm, turns this
// test red instead of accepting a word the daemon cannot negotiate
// (ai/rules/principles.md).

package ppp

import (
	"slices"
	"testing"

	gyang "github.com/openconfig/goyang/pkg/yang"

	configyang "github.com/ze-software/ze/internal/component/config/yang"

	// The blank imports register the two modules with the loader. Both import only
	// the modules the loader embeds, so the trees below resolve in a binary that
	// links nothing else. Neither import reaches the l2tp or pppoe runtime package,
	// so the separation doc.go states is untouched.
	_ "github.com/ze-software/ze/internal/component/l2tp/pppoe/yang"
	_ "github.com/ze-software/ze/internal/component/l2tp/yang"
)

// TestAuthMethodVocabularyMatchesModel compares the spellings the type carries with
// the enumeration each transport's module declares.
func TestAuthMethodVocabularyMatchesModel(t *testing.T) {
	loader, err := configyang.DefaultLoader()
	if err != nil {
		t.Fatalf("load the YANG model: %v", err)
	}

	spelled := make([]string, 0, len(authMethods))
	for _, method := range authMethods {
		spelled = append(spelled, method.String())
	}
	slices.Sort(spelled)

	cases := []struct {
		what   string
		module string
		path   []string
	}{
		{what: "the LNS", module: "ze-l2tp-conf", path: []string{"l2tp", "auth-method"}},
		{what: "the PPPoE access concentrator", module: "ze-pppoe-conf", path: []string{"pppoe", "auth-method"}},
	}

	for _, tc := range cases {
		model := modelEnum(t, loader, tc.module, tc.path)
		if slices.Equal(model, spelled) {
			continue
		}
		t.Errorf("%s and the AuthMethod type disagree: %s holds %v and the type spells %v. "+
			"A word only the model carries is one an operator can commit and the session cannot negotiate",
			tc.what, tc.module, model, spelled)
	}
}

// modelEnum answers the values of the enumeration at one leaf of the loaded model.
//
// It FAILS on a path that names no enumeration rather than answering an empty set.
// DefaultLoader discards its own LoadRegistered and Resolve errors, so a model that
// loaded half way comes back looking whole, and an empty set here would let the
// comparison above pass over nothing (ai/rules/evidence.md).
func modelEnum(t *testing.T, loader *configyang.Loader, module string, path []string) []string {
	t.Helper()

	entry := loader.GetEntry(module)
	if entry == nil {
		t.Fatalf("the loaded model holds no module %s: this binary registered nothing to compare against", module)
	}
	walked := module
	for _, name := range path {
		child := childEntry(entry, name)
		if child == nil {
			t.Fatalf("%s holds no child %q: the model this binary loaded is not the one this test reads", walked, name)
		}
		entry = child
		walked += "/" + name
	}
	if entry.Type == nil || entry.Type.Enum == nil {
		t.Fatalf("%s is not an enumeration, so it declares no vocabulary to compare", walked)
	}
	values := slices.Clone(entry.Type.Enum.Names())
	slices.Sort(values)
	return values
}

func childEntry(entry *gyang.Entry, name string) *gyang.Entry {
	if entry.Dir == nil {
		return nil
	}
	return entry.Dir[name]
}
