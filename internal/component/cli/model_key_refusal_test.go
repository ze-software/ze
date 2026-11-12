// Design: docs/guide/config-editor.md -- refusals and their fix lines
// Related: key_refusal.go -- FramedKeyRefusal, InEntryKeyLeafRefusal
// Related: model_commands_edit.go -- cmdSet, createListEntry

package cli

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	configyang "github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/core/opererr"

	_ "github.com/ze-software/ze/internal/plugins/ospf/yang"
)

// ospfSources is the alternation of the OSPF redistribute key, in declared order.
const ospfSources = "<connected|static|kernel|bgp|isis>"

// refusedLine types line on m and answers the refusal it must give.
func refusedLine(t *testing.T, m Model, line string) (Model, error) {
	t.Helper()
	out := enterLine(t, m, line)
	require.Error(t, out.err, "line %q", line)
	return out, out.err
}

// TestKeyRefusalNamesTheWholeLine proves a key refused where its entry would
// be created answers the shared refusal, with the whole line typed as the
// fix, the key corrected, and creates nothing.
//
// VALIDATES: AC-25, AC-30 -- the path through the refused key, every source
// value, `fix: set ospf redistribute <...> nssa-propagate true` at the root
// and the relative form inside `edit ospf`; nothing stored.
// PREVENTS: `set failed: ...` naming the key alone, and a fix that drops the
// rest of the line the operator typed.
func TestKeyRefusalNamesTheWholeLine(t *testing.T) {
	cases := []struct {
		name    string
		context string
		line    string
		fix     string
	}{
		{name: "at the root", line: "set ospf redistribute bogus nssa-propagate true",
			fix: "set ospf redistribute " + ospfSources + " nssa-propagate true"},
		{name: "in a context", context: "edit ospf", line: "set redistribute bogus nssa-propagate true",
			fix: "set redistribute " + ospfSources + " nssa-propagate true"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m, _ := topTestModel(t)
			if tc.context != "" {
				// `edit` enters only a block the candidate holds.
				m = enterLine(t, m, "set ospf redistribute source connected")
				require.NoError(t, m.err)
				m = enterLine(t, m, tc.context)
				require.NoError(t, m.err)
			}
			out, err := refusedLine(t, m, tc.line)
			refusal, ok := errors.AsType[*configyang.ValidationError](err)
			require.True(t, ok, "the shared refusal: %v", err)
			assert.Equal(t, configyang.ErrTypeEnum, refusal.Type)
			text := err.Error()
			assert.True(t, strings.HasPrefix(text, `ospf redistribute bogus: "bogus" is not an accepted value`), "text %q", text)
			for _, source := range []string{"connected", "static", "kernel", "bgp", "isis"} {
				assert.Contains(t, text, "\n  "+source+" ", "value %s listed", source)
			}
			assert.Equal(t, "fix: "+tc.fix, lastLine(text))
			assert.NotContains(t, text, "set failed")
			assert.NotContains(t, out.editor.WorkingContent(), "bogus", "no entry created")
		})
	}
}

// TestKeyLeafFormJudgesTheKey proves the key-leaf form `redistribute source
// <key>` judges the key as the creator does, and creates the keyed entry for
// an accepted one.
//
// VALIDATES: AC-33 (SSH editor) -- `bogus` refused with `fix: set ospf
// redistribute source <...>`; `connected` stores the `connected` entry of
// `redistribute`, which the file form writes inside the list's block.
// PREVENTS: a fix that names the key in the place of the key leaf keyword.
func TestKeyLeafFormJudgesTheKey(t *testing.T) {
	m, _ := topTestModel(t)
	out, err := refusedLine(t, m, "set ospf redistribute source bogus")
	assert.Equal(t, "fix: set ospf redistribute source "+ospfSources, lastLine(err.Error()))
	assert.NotContains(t, out.editor.WorkingContent(), "bogus")

	created := enterLine(t, m, "set ospf redistribute source connected")
	require.NoError(t, created.err)
	assert.Contains(t, created.editor.WorkingContent(), "redistribute {\n\t\tconnected\n\t}")
}

// TestRenameCopyRefuseTheDestinationKey proves rename and copy refuse a
// destination key the list does not take, with the line typed as the fix.
//
// VALIDATES: AC-26 -- `fix: rename ospf redistribute connected to <...>`
// and the same for copy; the configuration unchanged.
// PREVENTS: `rename failed: ...` naming the key alone.
func TestRenameCopyRefuseTheDestinationKey(t *testing.T) {
	for _, verb := range []string{"rename", "copy"} {
		t.Run(verb, func(t *testing.T) {
			m, _ := topTestModel(t)
			m = enterLine(t, m, "set ospf redistribute connected nssa-propagate true")
			require.NoError(t, m.err)
			before := m.editor.WorkingContent()
			out, err := refusedLine(t, m, verb+" ospf redistribute connected to bogus")
			assert.Equal(t, "fix: "+verb+" ospf redistribute connected to "+ospfSources, lastLine(err.Error()))
			assert.NotContains(t, err.Error(), verb+" failed")
			assert.Equal(t, before, out.editor.WorkingContent(), "nothing changed")
		})
	}
}

// TestInEntryKeyLeafRefusal proves setting an entry's own key leaf inside the
// entry is refused with the structural refusal, its fix a rename framed for
// the operator's position, and stores nothing.
//
// VALIDATES: AC-33, AC-34 (file mode) -- problem, why and `fix: rename ...`
// as the Text samples give them, at the root, inside `edit ospf` and inside
// the entry (behind `top`).
// PREVENTS: the key stored as a second value beside the entry's key, which
// the change file then cannot read back.
func TestInEntryKeyLeafRefusal(t *testing.T) {
	const (
		problem = "ospf redistribute connected source: source is the key of list redistribute and cannot be set inside an entry"
		why     = "rename changes the key of an existing entry, and set would store source as a second value"
	)
	cases := []struct {
		name    string
		context string
		line    string
		fix     string
	}{
		{name: "at the root", line: "set ospf redistribute connected source static",
			fix: "rename ospf redistribute connected to <new-key>"},
		{name: "in a context", context: "edit ospf", line: "set redistribute connected source static",
			fix: "rename redistribute connected to <new-key>"},
		{name: "inside the entry", context: "edit ospf redistribute connected", line: "set source static",
			fix: "top rename ospf redistribute connected to <new-key>"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m, _ := topTestModel(t)
			m = enterLine(t, m, "set ospf redistribute connected nssa-propagate true")
			require.NoError(t, m.err)
			if tc.context != "" {
				m = enterLine(t, m, tc.context)
				require.NoError(t, m.err)
			}
			before := m.editor.WorkingContent()
			out, err := refusedLine(t, m, tc.line)
			refusal, ok := errors.AsType[*opererr.Error](err)
			require.True(t, ok, "an operator error: %v", err)
			assert.Equal(t, problem, refusal.Problem())
			assert.Equal(t, why, refusal.Why())
			assert.Equal(t, tc.fix, refusal.Fix())
			assert.Equal(t, before, out.editor.WorkingContent(), "nothing stored")
		})
	}
}
