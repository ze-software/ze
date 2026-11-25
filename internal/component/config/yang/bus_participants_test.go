package yang

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testPluginConfForBusParticipants = `module ze-plugin-conf {
    namespace "urn:ze:plugin:conf";
    prefix plug;
    import ze-extensions { prefix ze; }

    grouping participant-bus {
        container receive {
            presence "Configure inbound bus delivery rules.";
            leaf-list allow {
                type string;
                ze:replace-whole;
            }
            leaf-list from {
                type string;
                ze:replace-whole;
            }
        }
        container send {
            presence "Configure outbound bus delivery rules.";
            leaf-list allow {
                type string;
                ze:replace-whole;
            }
            leaf-list to {
                type string;
                ze:replace-whole;
            }
        }
    }

    container bus {
        ze:first-pass;
        leaf note {
            type string;
        }
    }
}`

const testAlphaConfForBusParticipants = `module ze-alpha-conf {
    namespace "urn:ze:alpha:conf";
    prefix alpha;

    container alpha {
        list item {
            key "name";
            leaf name {
                type string;
            }
        }
        list bucket {
            key "name";
            leaf name {
                type string;
            }
            list item {
                key "name";
                leaf name {
                    type string;
                }
            }
        }
    }
}`

const testBetaAugmentForBusParticipants = `module ze-beta-conf {
    namespace "urn:ze:beta:conf";
    prefix beta;
    import ze-alpha-conf { prefix alpha; }

    augment "/alpha:alpha" {
        container extra {
            leaf enabled {
                type boolean;
            }
        }
    }
}`

// TestBusParticipantTargetsDeduplicatedAndSorted verifies that static targets
// and dynamic provider targets are normalized, deduplicated, and sorted.
//
// VALIDATES: BusParticipantTargets merges RegisterBusParticipantTarget and
// SetBusParticipantTargetsProvider deterministically.
// PREVENTS: duplicate augment blocks or non-deterministic YANG synthesis order.
func TestBusParticipantTargetsDeduplicatedAndSorted(t *testing.T) {
	restore := ResetBusParticipantTargetsForTest()
	t.Cleanup(restore)

	RegisterBusParticipantTarget("alpha/item/")
	RegisterBusParticipantTarget("/alpha/bucket")
	RegisterBusParticipantTarget("/alpha:alpha/alpha:item")
	RegisterBusParticipantTarget("")
	RegisterBusParticipantTarget("/alpha//bad")
	RegisterBusParticipantTarget("/alpha/bad:")
	SetBusParticipantTargetsProvider(func() []string {
		return []string{"/alpha/extra", "/alpha/bucket", "/alpha:alpha/alpha:bucket/alpha:item"}
	})

	got := BusParticipantTargets()
	require.Equal(t, []string{
		"/alpha/bucket",
		"/alpha/bucket/item",
		"/alpha/extra",
		"/alpha/item",
	}, got)
}

// TestSynthesizedBusParticipantsAugmentsDirectNestedAndAugmentedTargets verifies
// that loadBusParticipants synthesizes ze-bus-participants for direct children,
// nested lists, and nodes added by another module's augment, while skipping
// targets whose module is absent from the build.
//
// VALIDATES: AC-6 / A-6 synthetic participant bus augmentation across direct,
// nested, and cross-module augmented nodes, and dormancy on stripped builds.
// PREVENTS: broken augments on cross-module containers or errors when optional
// subsystems are compiled out.
func TestSynthesizedBusParticipantsAugmentsDirectNestedAndAugmentedTargets(t *testing.T) {
	restoreTargets := ResetBusParticipantTargetsForTest()
	t.Cleanup(restoreTargets)
	restoreAlpha := RegisterModuleForTest("ze-alpha-conf.yang", testAlphaConfForBusParticipants)
	t.Cleanup(restoreAlpha)
	restoreBeta := RegisterModuleForTest("ze-beta-conf.yang", testBetaAugmentForBusParticipants)
	t.Cleanup(restoreBeta)

	RegisterBusParticipantTarget("/alpha/item")
	RegisterBusParticipantTarget("/alpha/bucket")
	RegisterBusParticipantTarget("/alpha/bucket/item")
	RegisterBusParticipantTarget("/alpha/extra")
	RegisterBusParticipantTarget("/unloaded/missing")

	resolved, err := DefaultLoader()
	require.NoError(t, err)
	require.NotNil(t, resolved.GetModule(busParticipantsModuleName))
	assert.Empty(t, IdleModuleNames(resolved))
	assert.Empty(t, EmptyModuleNames(resolved))
	assert.NotContains(t, resolved.ConfModuleNames(), busParticipantsModuleName)

	alphaEntry := resolved.GetEntry("ze-alpha-conf")
	require.NotNil(t, alphaEntry)
	alphaRoot := alphaEntry.Dir["alpha"]
	require.NotNil(t, alphaRoot)

	for _, path := range [][]string{
		{"item", "bus"},
		{"bucket", "bus"},
		{"bucket", "item", "bus"},
		{"extra", "bus"},
	} {
		cur := alphaRoot
		for _, seg := range path {
			require.Contains(t, cur.Dir, seg, "missing %v in alpha tree", path)
			cur = cur.Dir[seg]
		}
		assert.Contains(t, cur.Dir, "send", "bus container at %v must carry send", path)
		assert.Contains(t, cur.Dir, "receive", "bus container at %v must carry receive", path)
	}
}

// TestSynthesizedBusParticipantsCoversExternalJoinedModule verifies that
// resolveJoined re-synthesizes ze-bus-participants over compiled plus external
// modules so an external plugin's participant node receives container bus.
//
// VALIDATES: Stage 1 external participant nodes receive the bus grouping when
// joined through Join.
// PREVENTS: stale compiled-in ze-bus-participants omitting external targets or
// failing with duplicate module errors during Join.
func TestSynthesizedBusParticipantsCoversExternalJoinedModule(t *testing.T) {
	restoreTargets := ResetBusParticipantTargetsForTest()
	t.Cleanup(restoreTargets)

	RegisterBusParticipantTarget("/alpha/item")

	l := NewLoader()
	require.NoError(t, l.LoadEmbedded())
	require.NoError(t, l.AddModuleFromText("ze-plugin-conf.yang", testPluginConfForBusParticipants))
	require.NoError(t, l.AddModuleFromText("ze-alpha-conf.yang", testAlphaConfForBusParticipants))
	require.NoError(t, l.loadBusParticipants())
	compiled, err := l.Resolve()
	require.NoError(t, err)

	// Simulate an external plugin adding its own participant node before Join.
	SetBusParticipantTargetsProvider(func() []string {
		return []string{"/acme/worker"}
	})

	extText := `module acme-worker-conf {
    namespace "urn:acme:worker:conf";
    prefix acmew;
    container acme {
        list worker {
            key "name";
            leaf name {
                type string;
            }
        }
    }
}`
	joined, err := Join(compiled, []ExternalModule{
		acmeModule("acme-worker-conf", "urn:acme:worker:conf", extText),
	}, joinValidators())
	require.NoError(t, err)

	acmeEntry := joined.GetEntry("acme-worker-conf")
	require.NotNil(t, acmeEntry)
	worker := acmeEntry.Dir["acme"].Dir["worker"]
	require.NotNil(t, worker)
	require.Contains(t, worker.Dir, "bus")
	assert.Contains(t, worker.Dir["bus"].Dir, "send")
	assert.Contains(t, worker.Dir["bus"].Dir, "receive")

	// Compiled-in target stays augmented too.
	alphaItem := joined.GetEntry("ze-alpha-conf").Dir["alpha"].Dir["item"]
	require.Contains(t, alphaItem.Dir, "bus")
	assert.True(t, slices.Equal(joined.FirstPassRoots(), []string{"bus"}))
}
