package cli

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/ze-software/ze/internal/component/bfd/yang" // Register the BFD YANG: single-hop-session is keyed `peer vrf interface`.
	"github.com/ze-software/ze/internal/component/config"
)

// TestDiffAfterDeletingTheFirstOfTwoMultiLeafKeyEntries is AC-16 of
// spec-bfd-link-local-peer-zone.
//
// GOAL: the CLI diff, prune and secret mask pair a multi-leaf-key entry with
// itself across two trees. METHOD: parse the AC-2 tree (one peer on `zbl0`
// and `zbl1`), delete the `zbl0` entry, re-parse the tree from its own
// brace-form output, then diff and prune both directions. The secret report
// is the next test: these sessions carry no sensitive leaf.
//
// VALIDATES: the diff removes the `zbl0` entry's lines and adds none, and
// marks no line of the `zbl1` entry; pruning the new tree against the old
// keeps no session, pruning the old against the new keeps the `zbl0` entry
// only.
// PREVENTS: a positional `#N` pairing, under which the re-parsed `zbl1` entry
// would take the first word's slot and pair with the old `zbl0` entry.
func TestDiffAfterDeletingTheFirstOfTwoMultiLeafKeyEntries(t *testing.T) {
	schema, err := config.YANGSchema()
	require.NoError(t, err)
	const original = `bfd {
	profile fast { desired-min-tx-us 100000; }
	single-hop-session fe80::b { interface zbl0; local fe80::a; }
	single-hop-session fe80::b { interface zbl1; local fe80::a; profile fast; }
}
`
	base, err := config.NewParser(schema).Parse(original)
	require.NoError(t, err)
	work := base.Clone()
	require.NotNil(t, work.GetContainer("bfd").RemoveListEntry("single-hop-session", "fe80::b interface zbl0"),
		"the zbl0 entry is stored under its whole address")
	modified := config.Serialize(work, schema)
	reparsed, err := config.NewParser(schema).Parse(modified)
	require.NoError(t, err, modified)

	lines, err := computeTreeAnnotatedDiff(original, modified, schema)
	require.NoError(t, err)
	var removed []string
	for _, line := range lines {
		switch line.Marker {
		case diffUnchanged:
		case diffRemoved:
			removed = append(removed, line.Text)
		case diffAdded, diffModified:
			assert.Failf(t, "only the zbl0 entry changes", "line %q marked %q", line.Text, string(line.Marker))
		}
	}
	joined := strings.Join(removed, "\n")
	assert.Contains(t, joined, "zbl0", "the deleted entry is removed: %q", removed)
	assert.NotContains(t, joined, "zbl1", "the zbl1 entry is unchanged: %q", removed)
	assert.NotContains(t, joined, "profile fast", "the zbl1 entry is unchanged: %q", removed)

	added := reparsed.Clone()
	config.PruneUnchanged(added, base, schema)
	if bfd := added.GetContainer("bfd"); bfd != nil {
		assert.Empty(t, bfd.GetList("single-hop-session"), "nothing was added or modified")
	}
	gone := base.Clone()
	config.PruneUnchanged(gone, reparsed, schema)
	sessions := gone.GetContainer("bfd").GetList("single-hop-session")
	assert.Len(t, sessions, 1, "%v", sessions)
	assert.NotNil(t, sessions["fe80::b interface zbl0"], "only the deletion is kept: %v", sessions)
}

// TestChangedSecretPathsPairsMultiLeafKeyEntriesByAddress is the secret leg
// of AC-16. No multi-leaf-key list in the YANG holds a sensitive leaf, so the
// schema is a hand-built copy of the BFD `single-hop-session` key with one.
//
// VALIDATES: deleting the `zbl0` entry reports no secret change, and a new
// secret on the `zbl1` entry is reported under that entry's address alone.
// PREVENTS: a positional pairing, under which the surviving `zbl1` entry
// would be compared with the deleted `zbl0` one and report a change that
// never happened.
func TestChangedSecretPathsPairsMultiLeafKeyEntriesByAddress(t *testing.T) {
	secret := config.Leaf(config.TypeString)
	secret.Sensitive = true
	session := config.List(config.TypeString,
		config.Field("peer", config.Leaf(config.TypeString)),
		config.Field("vrf", config.LeafWithDefault(config.TypeString, "default")),
		config.Field("interface", config.Leaf(config.TypeString)),
		config.Field("secret", secret),
	)
	session.KeyName = "peer vrf interface"
	schema := config.NewSchema()
	schema.Define("session", session)
	parse := func(text string) *config.Tree {
		tree, err := config.NewParser(schema).Parse(text)
		require.NoError(t, err, text)
		return tree
	}

	base := parse("session fe80::b { interface zbl0; secret alpha; }\nsession fe80::b { interface zbl1; secret bravo; }\n")
	deleted := parse("session fe80::b { interface zbl1; secret bravo; }\n")
	assert.Empty(t, config.ChangedSecretPaths(base, deleted, schema),
		"the zbl1 secret did not change")

	rotated := parse("session fe80::b { interface zbl1; secret charlie; }\n")
	assert.Equal(t, []string{"session.fe80::b interface zbl1.secret"},
		config.ChangedSecretPaths(base, rotated, schema))
}
