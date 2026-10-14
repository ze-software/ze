package cli

import (
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/ze-software/ze/internal/component/bfd/yang" // Register the BFD YANG: single-hop-session is keyed `peer vrf interface`.
	"github.com/ze-software/ze/internal/component/config"
)

// sessionKeys returns the stored keys of bfd single-hop-session, sorted.
func sessionKeys(t *testing.T, ed *Editor) []string {
	t.Helper()
	bfd := ed.Tree().GetContainer("bfd")
	require.NotNil(t, bfd, "bfd container")
	keys := make([]string, 0, len(bfd.GetList("single-hop-session")))
	for key := range bfd.GetList("single-hop-session") {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

// sessionEntry returns the single-hop-session entry stored under key.
func sessionEntry(t *testing.T, ed *Editor, key string) *config.Tree {
	t.Helper()
	entry := ed.Tree().GetContainer("bfd").GetList("single-hop-session")[key]
	require.NotNil(t, entry, "entry %q", key)
	return entry
}

// splitArgs is the command line as the cmd* handlers receive it.
func splitArgs(line string) []string { return strings.Fields(line) }

// TestEditorSetsTwoEntriesForOnePeerOnTwoInterfaces is AC-11 and the first
// case of AC-15b of spec-bfd-link-local-peer-zone.
//
// GOAL: the CLI editor addresses an entry of a multi-leaf-key list by its
// whole address. METHOD: drive cmdSet, cmdShow and cmdDelete over the
// registered BFD YANG and read the stored keys and leaves.
//
// VALIDATES: two `set` commands naming one peer on two links make two
// entries; a leaf set under one address lands on that entry only; the
// address with no `interface` is the separate entry `fe80::b`; a key leaf
// given twice is refused; a full-address `delete` removes one entry; the
// brace form prints the first value on each opening line.
// PREVENTS: the second `set` storing `interface zbl1` in the body of an
// entry keyed `fe80::b` (key and body disagree), or rewriting the first.
func TestEditorSetsTwoEntriesForOnePeerOnTwoInterfaces(t *testing.T) {
	m := newTestModel(t)
	ed := m.editor

	for _, line := range []string{
		"bfd single-hop-session fe80::b interface zbl0",
		"bfd single-hop-session fe80::b interface zbl1",
		"bfd single-hop-session fe80::b interface zbl1 profile fast",
	} {
		_, err := m.cmdSet(splitArgs(line))
		require.NoError(t, err, line)
	}

	assert.Equal(t, []string{"fe80::b interface zbl0", "fe80::b interface zbl1"}, sessionKeys(t, ed))
	zbl0 := sessionEntry(t, ed, "fe80::b interface zbl0")
	zbl1 := sessionEntry(t, ed, "fe80::b interface zbl1")
	value, _ := zbl0.Get("interface")
	assert.Equal(t, "zbl0", value, "the zbl0 entry holds its key leaf")
	value, _ = zbl1.Get("interface")
	assert.Equal(t, "zbl1", value, "the zbl1 entry holds its key leaf")
	_, hasProfile := zbl0.Get("profile")
	assert.False(t, hasProfile, "profile fast must not land on the zbl0 entry")
	value, _ = zbl1.Get("profile")
	assert.Equal(t, "fast", value)

	content := ed.WorkingContent()
	assert.Equal(t, 2, strings.Count(content, "single-hop-session fe80::b {"), "first value on each opening line:\n%s", content)
	assert.NotContains(t, content, "#1")
	assert.NotContains(t, content, "fe80::b interface", "an opening line never carries the whole address")

	t.Run("show one entry by its address", func(t *testing.T) {
		_, err := m.cmdShow(splitArgs("bfd single-hop-session fe80::b interface zbl1"))
		require.NoError(t, err)
		shown := ed.ContentAtPath(splitArgs("bfd single-hop-session fe80::b interface zbl1"))
		assert.Contains(t, shown, "profile fast")
		assert.Contains(t, shown, "zbl1")
		assert.NotContains(t, shown, "zbl0")
	})

	t.Run("address with no interface is its own entry", func(t *testing.T) {
		_, err := m.cmdSet(splitArgs("bfd single-hop-session fe80::b profile fast"))
		require.NoError(t, err)
		assert.Equal(t, []string{"fe80::b", "fe80::b interface zbl0", "fe80::b interface zbl1"}, sessionKeys(t, ed))
		value, _ := sessionEntry(t, ed, "fe80::b").Get("profile")
		assert.Equal(t, "fast", value)
		_, hasProfile := sessionEntry(t, ed, "fe80::b interface zbl0").Get("profile")
		assert.False(t, hasProfile, "the zbl0 entry is untouched")
	})

	t.Run("set display of entry fe80::b shows that entry only", func(t *testing.T) {
		shown := m.renderTreeAtPath(ed.Tree(), splitArgs("bfd single-hop-session fe80::b"), fmtConfig)
		require.NotEmpty(t, shown)
		for line := range strings.SplitSeq(strings.TrimSpace(shown), "\n") {
			assert.NotContains(t, line, "interface", "a line of a link entry leaked into entry fe80::b: %q", line)
		}
		assert.Contains(t, shown, "set bfd single-hop-session fe80::b profile fast")
		both := m.renderTreeAtPath(ed.Tree(), splitArgs("bfd single-hop-session fe80::b interface zbl1"), fmtConfig)
		assert.Contains(t, both, "set bfd single-hop-session fe80::b interface zbl1 profile fast")
		assert.NotContains(t, both, "zbl0")
	})

	t.Run("key leaf given twice is refused", func(t *testing.T) {
		before := sessionKeys(t, ed)
		_, err := m.cmdSet(splitArgs("bfd single-hop-session fe80::b interface zbl0 interface zbl1"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "interface is given twice")
		assert.Equal(t, before, sessionKeys(t, ed))
		value, _ := sessionEntry(t, ed, "fe80::b interface zbl0").Get("interface")
		assert.Equal(t, "zbl0", value, "the zbl0 entry's key leaf is unchanged")
	})

	t.Run("delete by full address removes one entry", func(t *testing.T) {
		_, err := m.cmdDelete(splitArgs("bfd single-hop-session fe80::b interface zbl0"))
		require.NoError(t, err)
		assert.Equal(t, []string{"fe80::b", "fe80::b interface zbl1"}, sessionKeys(t, ed))
	})
}

// twoLinkModel returns a model whose tree holds the AC-2 entries: fe80::b
// on zbl0 (profile slow) and on zbl1 (profile fast).
func twoLinkModel(t *testing.T) *Model {
	t.Helper()
	m := newTestModel(t)
	for _, line := range []string{
		"bfd single-hop-session fe80::b interface zbl0 profile slow",
		"bfd single-hop-session fe80::b interface zbl1 profile fast",
	} {
		_, err := m.cmdSet(splitArgs(line))
		require.NoError(t, err, line)
	}
	return m
}

// TestRenameAndCopyOfMultiLeafKeyEntryChangeOnlyTheFirstValue is AC-18 of
// spec-bfd-link-local-peer-zone through the CLI editor in direct mode.
//
// GOAL: a rename or copy changes only the first key value. METHOD: drive
// cmdRename and cmdCopy over the AC-2 tree and read the stored keys.
//
// VALIDATES: a destination that changes `interface` is refused naming the
// list and the leaf and giving the delete and set lines, and changes
// nothing; a rename changing `peer` keeps `interface` in the key and the
// body; a copy adds the new entry and keeps the source; a destination equal
// to the source after carry-over is refused as "new key must differ from
// current key".
// PREVENTS: an entry stored under `fe80::c` whose body holds `interface
// zbl0` (key and body disagree).
func TestRenameAndCopyOfMultiLeafKeyEntryChangeOnlyTheFirstValue(t *testing.T) {
	for _, verb := range []struct {
		name string
		run  func(m *Model, args []string) (commandResult, error)
	}{{"rename", (*Model).cmdRename}, {"copy", (*Model).cmdCopy}} {
		t.Run(verb.name+" changing interface is refused", func(t *testing.T) {
			m := twoLinkModel(t)
			_, err := verb.run(m, splitArgs("bfd single-hop-session fe80::b interface zbl0 to fe80::b interface zbl2"))
			require.Error(t, err)
			msg := err.Error()
			assert.Contains(t, msg, verb.name+" refused: interface is part of the key of single-hop-session, and "+verb.name+" changes only peer.")
			assert.Contains(t, msg, "To move the entry to zbl2, delete it and create it again:")
			assert.Contains(t, msg, "delete bfd single-hop-session fe80::b interface zbl0")
			assert.Contains(t, msg, "set bfd single-hop-session fe80::b interface zbl2")
			assert.Equal(t, []string{"fe80::b interface zbl0", "fe80::b interface zbl1"}, sessionKeys(t, m.editor))
		})
	}

	t.Run("rename changing peer carries interface over", func(t *testing.T) {
		m := twoLinkModel(t)
		_, err := m.cmdRename(splitArgs("bfd single-hop-session fe80::b interface zbl0 to fe80::c"))
		require.NoError(t, err)
		assert.Equal(t, []string{"fe80::b interface zbl1", "fe80::c interface zbl0"}, sessionKeys(t, m.editor))
		value, _ := sessionEntry(t, m.editor, "fe80::c interface zbl0").Get("interface")
		assert.Equal(t, "zbl0", value, "the key leaf carries over in the body")
		value, _ = sessionEntry(t, m.editor, "fe80::c interface zbl0").Get("profile")
		assert.Equal(t, "slow", value)
	})

	t.Run("copy changing peer keeps the source", func(t *testing.T) {
		m := twoLinkModel(t)
		_, err := m.cmdCopy(splitArgs("bfd single-hop-session fe80::b interface zbl1 to fe80::d interface zbl1"))
		require.NoError(t, err)
		assert.Equal(t, []string{"fe80::b interface zbl0", "fe80::b interface zbl1", "fe80::d interface zbl1"}, sessionKeys(t, m.editor))
		value, _ := sessionEntry(t, m.editor, "fe80::d interface zbl1").Get("interface")
		assert.Equal(t, "zbl1", value)
	})

	t.Run("session mode records read back with the same keys", func(t *testing.T) {
		configPath := writeTestConfig(t, "")
		store := newTestTreeStore(t, configPath)
		ed, err := NewEditorWithStorage(store, configPath)
		require.NoError(t, err)
		defer ed.Close() //nolint:errcheck // Best effort cleanup.
		session := NewEditSession("thomas", "local")
		ed.SetSession(session)
		for _, address := range [][]string{{"fe80::b", "interface", "zbl0"}, {"fe80::b", "interface", "zbl1"}} {
			require.NoError(t, ed.ensureListEntry([]string{"bfd"}, "single-hop-session", address))
		}

		require.NoError(t, ed.RenameListEntry([]string{"bfd"}, "single-hop-session", "fe80::b interface zbl0", []string{"fe80::c"}))
		require.NoError(t, ed.CopyListEntry([]string{"bfd"}, "single-hop-session", "fe80::b interface zbl1", []string{"fe80::d", "interface", "zbl1"}))
		err = ed.RenameListEntry([]string{"bfd"}, "single-hop-session", "fe80::b interface zbl1", []string{"fe80::b", "interface", "zbl2"})
		require.Error(t, err, "the refusal runs before the session branch")
		assert.Contains(t, err.Error(), "interface is part of the key of single-hop-session")

		changeData, err := store.ReadFile(ChangePath(configPath, session.User))
		require.NoError(t, err)
		_, _, ops, err := config.ParseChangeFile(string(changeData), config.NewSetParser(ed.schema))
		require.NoError(t, err)
		require.Len(t, ops, 2, "%s", changeData)
		assert.Equal(t, "fe80::b interface zbl0", ops[0].OldKey)
		assert.Equal(t, "fe80::c interface zbl0", ops[0].NewKey)
		assert.Equal(t, "fe80::b interface zbl1", ops[1].OldKey)
		assert.Equal(t, "fe80::d interface zbl1", ops[1].NewKey)
		assert.Equal(t, []string{"fe80::b interface zbl1", "fe80::c interface zbl0", "fe80::d interface zbl1"}, sessionKeys(t, ed))
	})

	t.Run("destination equal to the source is refused", func(t *testing.T) {
		m := twoLinkModel(t)
		_, err := m.cmdRename(splitArgs("bfd single-hop-session fe80::b interface zbl0 to fe80::b"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "new key must differ from current key")
		assert.Equal(t, []string{"fe80::b interface zbl0", "fe80::b interface zbl1"}, sessionKeys(t, m.editor))
	})
}

// keylessSchema holds a keyless list `update` with a keyed list `rule`
// under it.
func keylessSchema() *config.Schema {
	schema := config.NewSchema()
	rule := config.List(config.TypeString, config.Field("action", config.Leaf(config.TypeString)))
	rule.KeyName = "name"
	update := config.List(config.TypeString,
		config.Field("origin", config.Leaf(config.TypeString)),
		config.Field("rule", rule),
	)
	schema.Define("update", update) // KeyName empty: keyless.
	return schema
}

// TestRenameAndCopyRefuseAKeylessListEntry is AC-19 of
// spec-bfd-link-local-peer-zone through the CLI editor.
//
// GOAL: one keyless-list rule for rename and copy. METHOD: drive cmdRename
// and cmdCopy over a tree holding a keyless list's entry with a keyed entry
// under it, and compare the tree before and after.
//
// VALIDATES: the keyless list's entry, and the keyed entry under it, are
// each refused with one message naming the keyless list; the tree is
// unchanged.
// PREVENTS: the next word after a keyless list being read as its key (at
// HEAD the CLI renamed a keyless list's entry when no child name followed).
func TestRenameAndCopyRefuseAKeylessListEntry(t *testing.T) {
	cases := []struct {
		name string
		args string
	}{
		{"keyless entry", "update a to b"},
		{"keyed entry under a keyless list", "update rule r1 to r2"},
	}
	for _, verb := range []struct {
		name string
		run  func(m *Model, args []string) (commandResult, error)
	}{{"rename", (*Model).cmdRename}, {"copy", (*Model).cmdCopy}} {
		for _, tc := range cases {
			t.Run(verb.name+" "+tc.name, func(t *testing.T) {
				m := newTestModel(t)
				m.editor.schema = keylessSchema()
				tree := config.NewTree()
				entry := config.NewTree()
				entry.Set("origin", "igp")
				rule := config.NewTree()
				rule.Set("action", "accept")
				entry.AddListEntry("rule", "r1", rule)
				tree.AddListEntry("update", "a", entry)
				m.editor.tree = tree
				before := config.Serialize(tree, m.editor.schema)

				_, err := verb.run(m, splitArgs(tc.args))
				require.Error(t, err)
				assert.Equal(t, verb.name+" refused: update has no key, so its entries cannot be renamed or copied.", err.Error())
				assert.Equal(t, before, config.Serialize(m.editor.tree, m.editor.schema), "tree unchanged")
			})
		}
	}
}
