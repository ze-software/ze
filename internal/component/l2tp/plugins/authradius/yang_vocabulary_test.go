// Design: docs/research/l2tpv2-ze-integration.md -- RADIUS attribute selection
// Related: exclude.go -- excludableAttributes and excludablePacketKinds, the two tables read here
//
// exclude.go binds each `attributes exclude` container to the RADIUS attribute it holds
// back, and each `packet-type` word to the record kind it names. The module decides which
// containers and which words an operator can write. Neither side derives from the other,
// so the two are gated against each other here: a container or a word added to the module
// alone fails the configuration load at run time, and this test says so at build time
// instead (ai/rules/principles.md).

package l2tpauthradius

import (
	"slices"
	"testing"

	gyang "github.com/openconfig/goyang/pkg/yang"

	configyang "github.com/ze-software/ze/internal/component/config/yang"

	// The blank import registers ze-l2tp-auth-radius-conf with the loader. It imports
	// only the modules the loader embeds, so the tree below resolves in a binary that
	// links nothing else.
	_ "github.com/ze-software/ze/internal/component/l2tp/plugins/authradius/yang"
)

const radiusModule = "ze-l2tp-auth-radius-conf"

// excludePath is the container holding one child for each attribute an operator can
// hold back.
var excludePath = []string{"l2tp", "auth", "radius", "attributes", "exclude"}

// TestExcludeVocabularyMatchesModel compares the two tables in exclude.go with the
// `attributes exclude` container the module declares.
func TestExcludeVocabularyMatchesModel(t *testing.T) {
	loader, err := configyang.DefaultLoader()
	if err != nil {
		t.Fatalf("load the YANG model: %v", err)
	}

	exclude := entryAt(t, loader, excludePath)
	if len(exclude.Dir) == 0 {
		t.Fatalf("%s/%v holds no child: the model this binary loaded carries no attribute to compare",
			radiusModule, excludePath)
	}

	attributes := make([]string, 0, len(exclude.Dir))
	kinds := map[string]bool{}
	for name, attribute := range exclude.Dir {
		attributes = append(attributes, name)
		for _, word := range packetTypeWords(t, attribute, name) {
			kinds[word] = true
		}
	}
	slices.Sort(attributes)

	if held := sortedKeys(excludableAttributes); !slices.Equal(held, attributes) {
		t.Errorf("the attributes this plugin can hold back disagree with the module: exclude.go names %v and the module declares %v. "+
			"A container the module admits and this map lacks fails the configuration load",
			held, attributes)
	}

	if held := sortedKeys(excludablePacketKinds); !slices.Equal(held, sortedKeys(kinds)) {
		t.Errorf("the record kinds disagree with the module: exclude.go names %v and every packet-type leaf-list together declares %v. "+
			"A word only the module carries reaches this map as no kind at all, which holds the attribute back from nothing",
			held, sortedKeys(kinds))
	}
}

// packetTypeWords answers the enumeration one attribute's packet-type leaf-list
// declares. An attribute container without one is a failure rather than an empty
// set: every attribute in this container carries the leaf-list, so an absent one
// says the walk read a tree this test does not understand (ai/rules/evidence.md).
func packetTypeWords(t *testing.T, attribute *gyang.Entry, name string) []string {
	t.Helper()

	leaf := childEntry(attribute, "packet-type")
	if leaf == nil {
		t.Fatalf("%s holds no packet-type leaf-list: the model this binary loaded is not the one this test reads", name)
	}
	if leaf.Type == nil || leaf.Type.Enum == nil {
		t.Fatalf("the packet-type of %s is not an enumeration, so it declares no vocabulary to compare", name)
	}
	return leaf.Type.Enum.Names()
}

// entryAt walks one path of the loaded model, and FAILS on a name the tree does not
// carry. DefaultLoader discards its own LoadRegistered and Resolve errors, so a model
// that loaded half way comes back looking whole (ai/rules/evidence.md).
func entryAt(t *testing.T, loader *configyang.Loader, path []string) *gyang.Entry {
	t.Helper()

	entry := loader.GetEntry(radiusModule)
	if entry == nil {
		t.Fatalf("the loaded model holds no module %s: this binary registered nothing to compare against", radiusModule)
	}
	walked := radiusModule
	for _, name := range path {
		child := childEntry(entry, name)
		if child == nil {
			t.Fatalf("%s holds no child %q: the model this binary loaded is not the one this test reads", walked, name)
		}
		entry = child
		walked += "/" + name
	}
	return entry
}

func childEntry(entry *gyang.Entry, name string) *gyang.Entry {
	if entry.Dir == nil {
		return nil
	}
	return entry.Dir[name]
}

func sortedKeys[V any](table map[string]V) []string {
	keys := make([]string, 0, len(table))
	for key := range table {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}
