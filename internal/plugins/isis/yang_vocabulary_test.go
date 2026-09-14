// Design: docs/architecture/isis/isis-4-component-config.md -- IS-IS config resolution
// Related: config.go -- Level.String, the one place the level words are spelled
// Related: auth_keystore.go -- algoFromString, the key-chain algorithm words
//
// Two vocabularies cross the configuration boundary here: the routing level of an IS or
// a circuit, and the digest algorithm of a key-chain key. ze-isis-conf.yang decides which
// words an operator can type, and the Go side decides what each word selects: a Level the
// engine runs at, or a packet.AuthAlgorithm that signs a PDU. Neither side derives from
// the other, so the two are gated against each other here. A word added to the module
// alone is one the engine drops silently, and an algorithm added to the packet package
// alone is one no operator can reach (ai/rules/principles.md).

package isis

import (
	"slices"
	"testing"

	gyang "github.com/openconfig/goyang/pkg/yang"

	configyang "github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/plugins/isis/packet"

	// The blank import registers ze-isis-conf with the loader. It imports only the
	// modules the loader embeds, so the trees below resolve in a binary that links
	// nothing else.
	_ "github.com/ze-software/ze/internal/plugins/isis/yang"
)

const isisModule = "ze-isis-conf"

// TestLevelVocabularyMatchesModel compares the words Level.String renders with the
// enumeration the module declares, at the instance leaf and at the per-circuit
// override, which are two leaves of one vocabulary.
func TestLevelVocabularyMatchesModel(t *testing.T) {
	loader, err := configyang.DefaultLoader()
	if err != nil {
		t.Fatalf("load the YANG model: %v", err)
	}

	spelled := []string{LevelL1.String(), LevelL2.String(), LevelL1L2.String()}
	slices.Sort(spelled)

	paths := [][]string{
		{"isis", "level"},
		{"isis", "interfaces", "interface", "level"},
	}
	for _, path := range paths {
		model := modelEnum(t, loader, path)
		if slices.Equal(model, spelled) {
			continue
		}
		t.Errorf("the routing levels disagree: the model at %v holds %v and the Level type spells %v. "+
			"A word only the model carries reaches the engine as the level String renders for an unknown value",
			path, model, spelled)
	}
}

// TestKeyChainAlgorithmVocabularyMatchesModel checks both directions of the key-chain
// algorithm binding: every word the module admits selects an algorithm, and every
// algorithm the packet package carries is reachable from one of those words.
func TestKeyChainAlgorithmVocabularyMatchesModel(t *testing.T) {
	loader, err := configyang.DefaultLoader()
	if err != nil {
		t.Fatalf("load the YANG model: %v", err)
	}

	model := modelEnum(t, loader, []string{"isis", "key-chains", "key", "algorithm"})

	selected := map[packet.AuthAlgorithm]string{}
	for _, word := range model {
		algo, known := algoFromString(word)
		if !known {
			t.Errorf("the module admits algorithm %q and algoFromString does not know it, "+
				"so a key configured with it is dropped from the chain and signs nothing", word)
			continue
		}
		if first, seen := selected[algo]; seen {
			t.Errorf("the module words %q and %q both select one algorithm, so one of them signs with a digest the operator did not name",
				first, word)
			continue
		}
		selected[algo] = word
	}

	// The other direction. AuthAlgoNone is the absence of authentication and no word
	// names it; every value above it is a digest a key can be configured with, so each
	// one owes a word. The range is walked rather than listed, because a value added
	// to the const block must show up here rather than be added to a list beside it.
	for algo := packet.AuthAlgoCleartext; algo <= packet.AuthAlgoHMACSHA512; algo++ {
		if _, reachable := selected[algo]; reachable {
			continue
		}
		t.Errorf("the packet package carries authentication algorithm %d and no key-chains/key/algorithm word selects it, "+
			"so no operator can ask a chain to sign with it", algo)
	}
}

// modelEnum answers the values of the enumeration at one leaf of the loaded model.
//
// It FAILS on a path that names no enumeration rather than answering an empty set.
// DefaultLoader discards its own LoadRegistered and Resolve errors, so a model that
// loaded half way comes back looking whole, and an empty set here would let every
// comparison above pass over nothing (ai/rules/evidence.md).
func modelEnum(t *testing.T, loader *configyang.Loader, path []string) []string {
	t.Helper()

	entry := loader.GetEntry(isisModule)
	if entry == nil {
		t.Fatalf("the loaded model holds no module %s: this binary registered nothing to compare against", isisModule)
	}
	walked := isisModule
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
