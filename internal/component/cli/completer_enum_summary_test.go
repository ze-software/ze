// Design: docs/architecture/config/yang-config-design.md -- an enum value's
// summary reaches the completion row.

package cli

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/yang"
)

// enumSummaryProbeModule declares one enumeration leaf whose values differ in
// what they declare, and one union leaf, so the test can read both branches
// of value completion without depending on what the shipped model declares
// on any given day.
const enumSummaryProbeModule = `module ze-test-enum-summary-conf {
    namespace "urn:ze:test-enum-summary:conf";
    prefix tes;
    import ze-extensions { prefix ze; }

    revision 2026-01-01 { description "Probe module for enum value summaries."; }

    container test-enum-summary {
        ze:help "The container declaring the probe leaves.";
        leaf mixed {
            type enumeration {
                enum told { ze:help "The value declares a summary"; }
                enum silent;
            }
            ze:help "A leaf whose values differ in what they declare.";
        }
        leaf either {
            type union {
                type enumeration {
                    enum auto { ze:help "Ze picks the value"; }
                }
                type uint16;
            }
            ze:help "A union whose enumeration member declares a summary.";
        }
    }
}`

// enumSummaryCompleter builds a Completer over the shipped model plus the
// probe module above.
func enumSummaryCompleter(t *testing.T) *Completer {
	t.Helper()

	loader := yang.NewLoader()
	require.NoError(t, loader.LoadEmbedded())
	require.NoError(t, loader.LoadRegistered())
	require.NoError(t, loader.AddModuleFromText("ze-test-enum-summary-conf", enumSummaryProbeModule))
	require.NoError(t, loader.Resolve())
	return &Completer{loader: loader}
}

// VALIDATES: AC-4, AC-5 -- the value row of an enumeration leaf carries the
// ze:help the value declares, a value that declares none carries an empty
// summary, and a union's enumeration member is read the same way.
// PREVENTS: the literal "enum value" placeholder standing where a declaration
// exists, and a summary invented for a value that declares none.
func TestEnumValueCompletionShowsDeclaredSummary(t *testing.T) {
	c := enumSummaryCompleter(t)

	mixed := c.Complete("set test-enum-summary mixed ", nil)
	byText := make(map[string]Completion, len(mixed))
	for _, completion := range mixed {
		byText[completion.Text] = completion
	}
	require.Contains(t, byText, "told")
	require.Contains(t, byText, "silent")
	assert.Equal(t, "The value declares a summary", byText["told"].ShortHelp)
	assert.Equal(t, "", byText["silent"].ShortHelp)
	for _, completion := range mixed {
		assert.NotEqual(t, "enum value", completion.ShortHelp, "%s carries the placeholder", completion.Text)
	}

	either := c.Complete("set test-enum-summary either ", nil)
	require.NotEmpty(t, either)
	assert.Equal(t, "auto", either[0].Text)
	assert.Equal(t, "Ze picks the value", either[0].ShortHelp)

	// The summaries are resolved once per leaf and then served from the
	// cache, so a second keystroke reads the same map rather than the AST.
	// Equal contents would also hold for two fresh walks, so the assertion is
	// on the map's identity: both calls answer the one map the cache holds.
	entry := c.getEntry([]string{"test-enum-summary", "mixed"})
	require.NotNil(t, entry)
	first := c.enumValueSummaries(entry)
	second := c.enumValueSummaries(entry)
	assert.Equal(t, reflect.ValueOf(first).UnsafePointer(), reflect.ValueOf(second).UnsafePointer(),
		"the second call must serve the cached map, not a fresh walk")
	assert.Equal(t, reflect.ValueOf(c.enumSummaries[entry]).UnsafePointer(), reflect.ValueOf(first).UnsafePointer())
	assert.Equal(t, "The value declares a summary", first["told"])
}
