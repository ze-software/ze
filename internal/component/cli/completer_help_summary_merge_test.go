// Detail: completer.go -- mergeHelpExts, the summary a merged row shows

package cli

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/yang"
)

// Two modules declaring the same two containers, which is the shape a plugin
// writes when it attaches leaves to a node another module also declares. The
// first module sorts first, so it is the one that used to decide the summary
// for both containers: with a text of its own on one, and with none on the
// other.
const (
	firstMergeProbeModule = `module ze-test-aaa-merge-conf {
    namespace "urn:ze:test-aaa-merge:conf";
    prefix tam;
    import ze-extensions { prefix ze; }

    revision 2026-01-01 { description "Probe module, sorts first."; }

    container merge-probe-both {
        ze:help "What the plugin attaches here.";
        leaf attached { type string; ze:help "A leaf the plugin attaches."; }
    }
    container merge-probe-silent {
        leaf attached { type string; ze:help "A leaf the plugin attaches."; }
    }
}`

	secondMergeProbeModule = `module ze-test-zzz-merge-conf {
    namespace "urn:ze:test-zzz-merge:conf";
    prefix tzm;
    import ze-extensions { prefix ze; }

    revision 2026-01-01 { description "Probe module, sorts last."; }

    container merge-probe-both {
        ze:help "What the node itself is.";
        leaf owned { type string; ze:help "A leaf the owning module declares."; }
    }
    container merge-probe-silent {
        ze:help "The only summary this node has.";
        leaf owned { type string; ze:help "A leaf the owning module declares."; }
    }
}`
)

// mergeProbeCompleter builds a Completer over the shipped model plus the two
// probe modules above.
func mergeProbeCompleter(t *testing.T) *Completer {
	t.Helper()

	loader := yang.NewLoader()
	require.NoError(t, loader.LoadEmbedded())
	require.NoError(t, loader.LoadRegistered())
	require.NoError(t, loader.AddModuleFromText("ze-test-aaa-merge-conf", firstMergeProbeModule))
	require.NoError(t, loader.AddModuleFromText("ze-test-zzz-merge-conf", secondMergeProbeModule))
	require.NoError(t, loader.Resolve())
	return &Completer{loader: loader}
}

// summaryOf answers the completion row's one-line summary for one top-level
// node, read the way an operator reads it: by asking for the completions of
// `set` and finding the row.
func summaryOf(t *testing.T, c *Completer, node string) string {
	t.Helper()

	for _, completion := range c.Complete("set ", nil) {
		if completion.Text == node {
			return completion.ShortHelp
		}
	}
	t.Fatalf("no completion row for %q", node)
	return ""
}

// TestMergedRowShowsEveryDeclarationsSummary drives the completion lookup an
// operator reaches by pressing TAB or ?.
//
// VALIDATES: the row for a node several modules declare carries every module's
// ze:help, so no declaration is unreachable and a silent module cannot erase a
// written one.
// PREVENTS:  the summary being decided by the alphabetical order of the module
// names, which showed `interface` as the cos plugin's scaffolding text
// (plan/journal/silent-fall-through.md, 2026-09-04).
func TestMergedRowShowsEveryDeclarationsSummary(t *testing.T) {
	c := mergeProbeCompleter(t)

	t.Run("two declarations both carry a summary", func(t *testing.T) {
		summary := summaryOf(t, c, "merge-probe-both")
		assert.Contains(t, summary, "What the plugin attaches here.", "the first declaration's summary is unreachable")
		assert.Contains(t, summary, "What the node itself is.", "the later declaration's summary is unreachable")
	})

	t.Run("the first declaration carries none", func(t *testing.T) {
		summary := summaryOf(t, c, "merge-probe-silent")
		assert.Equal(t, "The only summary this node has.", summary,
			"a module that declares no summary must not erase the one that does")
	})

	t.Run("a summary is never repeated", func(t *testing.T) {
		summary := summaryOf(t, c, "merge-probe-both")
		assert.Equal(t, 1, strings.Count(summary, "What the node itself is."))
	})
}
