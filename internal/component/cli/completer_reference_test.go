// Design: docs/guide/config-editor.md -- a reference leaf completes the
// entries the candidate tree configures.

package cli

import (
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/yang"
)

// referenceProbeModule declares a leafref inside a multi-key list entry, one
// that climbs out of the entry, and a ze:reference leaf, so the test reads
// every shape of reference completion without depending on the shipped model.
const referenceProbeModule = `module ze-test-reference-conf {
    namespace "urn:ze:test-reference:conf";
    prefix trf;
    import ze-extensions { prefix ze; }

    revision 2026-01-01 { description "Probe module for reference completion."; }

    container test-reference {
        list pool { key name; leaf name { type string; } }
        list site {
            key "region name";
            leaf region { type string; }
            leaf name { type string; }
            list slot { key id; leaf id { type string; } }
            leaf primary { type leafref { path "../slot/id"; } }
            leaf pool { type leafref { path "../../pool/name"; } }
            leaf tint { type string; ze:reference "cli-probe-tint"; }
        }
    }
}`

// tintAsked records the data paths the cli-probe-tint source was asked at.
var (
	tintMu    sync.Mutex
	tintAsked [][]string
)

func init() {
	yang.RegisterReferenceSource("cli-probe-tint", func(_ map[string]any, at []string) []string {
		tintMu.Lock()
		tintAsked = append(tintAsked, slices.Clone(at))
		tintMu.Unlock()
		return []string{"teal", "amber"}
	})
}

// referenceCompleter builds a Completer over the shipped model plus the probe
// module above.
func referenceCompleter(t *testing.T) *Completer {
	t.Helper()
	loader := yang.NewLoader()
	require.NoError(t, loader.LoadEmbedded())
	require.NoError(t, loader.LoadRegistered())
	require.NoError(t, loader.AddModuleFromText("ze-test-reference-conf", referenceProbeModule))
	resolved, err := loader.Resolve()
	require.NoError(t, err)
	return &Completer{schema: resolved}
}

// referenceTree configures pools p1 and p2, site eu/a with slots s1 and s2,
// and site eu/b with slot s9.
func referenceTree() *config.Tree {
	root := config.NewTree()
	probe := config.NewTree()
	for _, name := range []string{"p1", "p2"} {
		probe.AddListEntry("pool", name, config.NewTree())
	}
	for address, slots := range map[string][]string{"eu name a": {"s1", "s2"}, "eu name b": {"s9"}} {
		site := config.NewTree()
		for _, slot := range slots {
			site.AddListEntry("slot", slot, config.NewTree())
		}
		probe.AddListEntry("site", address, site)
	}
	root.SetContainer("test-reference", probe)
	return root
}

// TestCompleterReferenceValues checks that a reference leaf completes the
// entries this call's tree configures. Method: the probe module over
// referenceTree, completing each reference leaf from the root and from a
// context inside the multi-key entry.
//
// VALIDATES: completion.valueCompletions offers ReferenceInstances' answer,
// filtered by prefix, for a leafref within and out of a list entry and for a
// ze:reference source, which receives the leaf's data path.
// PREVENTS: a reference leaf completing only its type hint, and a sibling
// entry's instances leaking into another entry's completion.
func TestCompleterReferenceValues(t *testing.T) {
	c := referenceCompleter(t)
	tree := referenceTree()

	assert.Equal(t, []string{"s1", "s2"}, completionTexts(c.Complete(tree, "set test-reference site eu name a primary ", nil)))
	assert.Equal(t, []string{"s2"}, completionTexts(c.Complete(tree, "set test-reference site eu name a primary s2", nil)))
	assert.Equal(t, []string{"s9"}, completionTexts(c.Complete(tree, "set test-reference site eu name b primary ", nil)))
	assert.Equal(t, []string{"s1", "s2"}, completionTexts(c.Complete(tree, "set primary ", []string{"test-reference", "site", "eu", "name", "a"})))
	assert.Equal(t, []string{"p1", "p2"}, completionTexts(c.Complete(tree, "set test-reference site eu name a pool ", nil)))
	assert.Equal(t, []string{"amber", "teal"}, completionTexts(c.Complete(tree, "set test-reference site eu name a tint ", nil)))

	tintMu.Lock()
	asked := slices.Clone(tintAsked)
	tintMu.Unlock()
	assert.Contains(t, asked, []string{"test-reference", "site", "eu name a", "tint"})
}

// TestCompleterReferenceNoInstances checks that a leafref with nothing to
// name, in a call with no tree, falls back to its type hint.
//
// VALIDATES: an empty reference answer reaches the type hint.
// PREVENTS: an empty completion list, which tells the operator nothing.
func TestCompleterReferenceNoInstances(t *testing.T) {
	c := referenceCompleter(t)
	got := completionTexts(c.Complete(nil, "set test-reference site eu name a primary ", nil))
	require.Len(t, got, 1)
	assert.True(t, strings.HasPrefix(got[0], "<"), "want the type hint, got %v", got)
}
