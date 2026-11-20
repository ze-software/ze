package command

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// resetPluginOperationRegistries empties the two registries a plugin's pipe
// declaration touches, before and after the test. No package of this test
// binary registers into either from init().
func resetPluginOperationRegistries(t *testing.T) {
	t.Helper()
	ResetPipeFiltersForTest()
	ResetAliasesForTest()
	t.Cleanup(func() {
		ResetPipeFiltersForTest()
		ResetAliasesForTest()
	})
}

// TestPluginOperationCollisionsRefused proves every collision of a plugin's
// declared operation refuses the whole batch with an error naming the
// operation and what it collides with, and records nothing of the batch.
//
// Method: each row prepares the registries, then declares a batch whose FIRST
// entry is a valid operation on another path (`show y`), and whose second
// collides. After the refusal, `show y` must hold nothing.
//
// VALIDATES: spec-pipe-chain-honored-on-every-command-surface AC-30, A-6.
// PREVENTS: a colliding plugin declaration panicking the daemon, taking a name
// an alias or another owner answers to, or leaving half a batch recorded.
func TestPluginOperationCollisionsRefused(t *testing.T) {
	rows := []struct {
		name    string
		prepare func(t *testing.T)
		clash   PluginPipeFilter
		want    []string
	}{
		{
			name:  "a generic the catalog does not let a command override",
			clash: PluginPipeFilter{Command: "show x", Filter: PipeFilter{Name: "json", Argument: ArgOptional}},
			want:  []string{"json", "cannot override"},
		},
		{
			name:  "an override with another argument kind",
			clash: PluginPipeFilter{Command: "show x", Filter: PipeFilter{Name: "count", Argument: ArgText}},
			want:  []string{"count", "the generic takes none"},
		},
		{
			name: "an in-tree alias on an overlapping path",
			prepare: func(*testing.T) {
				RegisterAliases([]string{"show x"}, Alias{Name: "brief", Expansion: "table"})
			},
			clash: PluginPipeFilter{Command: "show x rows", Filter: PipeFilter{Name: "brief"}},
			want:  []string{"brief", `a pipe alias of "show x"`},
		},
		{
			name: "another plugin's alias on the same path",
			prepare: func(t *testing.T) {
				require.NoError(t, RegisterPluginAliases("other", []string{"show x"},
					[]PluginAlias{{Command: "show x", Alias: Alias{Name: "brief", Expansion: "table"}}}))
			},
			clash: PluginPipeFilter{Command: "show x", Filter: PipeFilter{Name: "brief"}},
			want:  []string{"brief", `a pipe alias of "show x"`},
		},
		{
			name: "a global alias",
			prepare: func(*testing.T) {
				RegisterAliases(nil, Alias{Name: "terse", Expansion: "table"})
			},
			clash: PluginPipeFilter{Command: "show x", Filter: PipeFilter{Name: "terse"}},
			want:  []string{"terse", "a global pipe alias"},
		},
		{
			name: "a path holding a compiled-in command's set",
			prepare: func(*testing.T) {
				RegisterPipeFilters([]string{"show x"}, PipeFilter{Name: "peer", Argument: ArgText})
			},
			clash: PluginPipeFilter{Command: "show x", Filter: PipeFilter{Name: "top"}},
			want:  []string{`"show x"`, "peer", "a compiled-in command", "top"},
		},
		{
			name: "a path holding another plugin's set",
			prepare: func(t *testing.T) {
				require.NoError(t, RegisterPluginPipeFilters("other", []string{"show x"},
					[]PluginPipeFilter{{Command: "show x", Filter: PipeFilter{Name: "peer", Argument: ArgText}}}))
			},
			clash: PluginPipeFilter{Command: "show x", Filter: PipeFilter{Name: "top"}},
			want:  []string{`"show x"`, "peer", `plugin "other"`, "top"},
		},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			resetPluginOperationRegistries(t)
			if row.prepare != nil {
				row.prepare(t)
			}
			before := PipeFiltersForCommand("show x")

			batch := []PluginPipeFilter{{Command: "show y", Filter: PipeFilter{Name: "sibling"}}, row.clash}
			err := RegisterPluginPipeFilters("mine", []string{"show x", "show x rows", "show y"}, batch)
			require.Error(t, err)
			for _, word := range row.want {
				assert.Contains(t, err.Error(), word)
			}
			assert.Nil(t, PipeFiltersForCommand("show y"), "nothing of the refused batch is recorded")
			assert.Equal(t, before, PipeFiltersForCommand("show x"), "the colliding path keeps what it held")
		})
	}

	t.Run("one name twice in one message", func(t *testing.T) {
		resetPluginOperationRegistries(t)
		batch := []PluginPipeFilter{
			{Command: "show y", Filter: PipeFilter{Name: "sibling"}},
			{Command: "show x", Filter: PipeFilter{Name: "top"}},
			{Command: "show x", Filter: PipeFilter{Name: "top", Argument: ArgCount}},
		}
		err := RegisterPluginPipeFilters("mine", []string{"show x", "show y"}, batch)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "top")
		assert.Contains(t, err.Error(), "twice")
		assert.Nil(t, PipeFiltersForCommand("show x"))
		assert.Nil(t, PipeFiltersForCommand("show y"))
	})
}

// TestPluginOperationAccepted is the positive half of the collision table: a
// plugin's own name, and an override stating the generic's argument kind, are
// recorded under the plugin, on the path the plugin named.
//
// VALIDATES: AC-14 (recorded into pipeFilterRegistry), AC-30 (an allowed
// override is not a collision).
// PREVENTS: a refusal that refuses every plugin declaration passing the table
// above.
func TestPluginOperationAccepted(t *testing.T) {
	resetPluginOperationRegistries(t)
	t.Cleanup(func() { UnregisterPluginPipeFilters("mine") })

	err := RegisterPluginPipeFilters("mine", []string{"show x"}, []PluginPipeFilter{
		{Command: "show x", Filter: PipeFilter{Name: "Top-Talkers", Description: "Rank by bytes.", Argument: ArgCount}},
		{Command: "show x", Filter: PipeFilter{Name: "count", Argument: ArgNone}},
	})
	require.NoError(t, err)
	assert.Equal(t, []PipeFilter{
		{Name: "count", Argument: ArgNone},
		{Name: "top-talkers", Description: "Rank by bytes.", Argument: ArgCount},
	}, PipeFiltersForCommand("show x"))
	assert.Equal(t, "mine", pipeFilterRegistry.ownerOf("show x"))
}

// TestPipeFilterArgumentKindRecorded proves an in-tree PipeFilter and a
// plugin's declared argument spelling record the same field, and that a
// spelling or a kind outside the closed vocabulary is refused when recorded.
//
// VALIDATES: the TDD row of the same name (the plugin half), AC-14.
// PREVENTS: a plugin's "count" read as another kind than the in-tree ArgCount,
// or an unknown spelling defaulting to none.
func TestPipeFilterArgumentKindRecorded(t *testing.T) {
	resetPluginOperationRegistries(t)
	t.Cleanup(func() { UnregisterPluginPipeFilters("p") })

	for _, kind := range declarableArgKinds {
		named, known := PipeArgKindNamed(kind.String())
		require.True(t, known, kind.String())
		assert.Equal(t, kind, named, "the spelling names its own kind")
	}
	for _, spelling := range []string{"", "number", "Count", "words"} {
		_, known := PipeArgKindNamed(spelling)
		assert.False(t, known, "%q is outside the vocabulary", spelling)
	}

	RegisterPipeFilters([]string{"show a"}, PipeFilter{Name: "top", Argument: ArgCount})
	count, known := PipeArgKindNamed("count")
	require.True(t, known)
	require.NoError(t, RegisterPluginPipeFilters("p", []string{"show b"},
		[]PluginPipeFilter{{Command: "show b", Filter: PipeFilter{Name: "top", Argument: count}}}))
	assert.Equal(t, PipeFiltersForCommand("show a"), PipeFiltersForCommand("show b"))

	err := RegisterPluginPipeFilters("q", []string{"show c"},
		[]PluginPipeFilter{{Command: "show c", Filter: PipeFilter{Name: "top", Argument: PipeArgKind(99)}}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "outside none, optional, text, count, fields and path")
	assert.Nil(t, PipeFiltersForCommand("show c"))
}

// TestPluginOperationWithdrawnRestoresThePath proves withdrawal puts back what
// each path held: an in-tree empty barrier returns as a barrier, and a path
// the plugin found undeclared, with the barrier derived for the plugin's
// command below it, becomes undeclared again.
//
// VALIDATES: A-6, the withdrawal the three server sites call.
// PREVENTS: a plugin's operations outliving the plugin, or a withdrawal that
// removes an in-tree barrier and lets a child inherit its parent's set.
func TestPluginOperationWithdrawnRestoresThePath(t *testing.T) {
	resetPluginOperationRegistries(t)

	RegisterPipeFilters([]string{"show z"})
	require.NoError(t, RegisterPluginPipeFilters("p", []string{"show z", "show q", "show q rows"}, []PluginPipeFilter{
		{Command: "show z", Filter: PipeFilter{Name: "top"}},
		{Command: "show q", Filter: PipeFilter{Name: "top"}},
	}))
	assert.Len(t, PipeFiltersForCommand("show z"), 1, "the plugin's set replaces the in-tree barrier")
	assert.Len(t, PipeFiltersForCommand("show q"), 1)
	assert.Nil(t, PipeFiltersForCommand("show q rows"), "the derived barrier stops show q's set at show q")
	_, barrier := pipeFilterRegistry.get("show q rows")
	assert.True(t, barrier, "the barrier is a declaration of its own")

	UnregisterPluginPipeFilters("p")
	held, found := pipeFilterRegistry.get("show z")
	assert.True(t, found, "the in-tree barrier is back")
	assert.Empty(t, held.filters)
	_, found = pipeFilterRegistry.get("show q")
	assert.False(t, found, "a path the plugin found undeclared is undeclared again")
	_, found = pipeFilterRegistry.get("show q rows")
	assert.False(t, found, "the derived barrier leaves with the plugin")
}
