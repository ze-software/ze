// Design: docs/architecture/cli/command-completion.md -- explanation after arguments.

package command

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// TestExplainCompletedArguments keeps the declared command explanation after
// ordinary terminal arguments, without treating an unknown grouping path as one.
func TestExplainCompletedArguments(t *testing.T) {
	const explanation = "Restart the selected configured plugin."
	tree := &Node{Children: map[string]*Node{
		"request": {Description: "Request operations", Children: map[string]*Node{
			"plugin": {Children: map[string]*Node{
				"restart": {WireMethod: "plugin-restart", Description: explanation},
			}},
		}},
	}}
	completer := NewTreeCompleter(tree)
	for _, input := range []string{
		"request plugin restart", "request plugin restart alpha", "request plugin restart alpha ",
		`request plugin restart "two words"`, `request plugin restart "pipe|name" | json`,
	} {
		text, found := completer.Explain(input)
		assert.True(t, found, input)
		assert.Equal(t, explanation, text, input)
	}
	for _, input := range []string{"request plugin unknown alpha", "request unknown", "request plugin rest", "unknown"} {
		text, found := completer.Explain(input)
		assert.False(t, found, input)
		assert.Empty(t, text, input)
	}
}

// TestExplainPluginArguments checks completion-only plugin leaves too; an
// arbitrary described grouping node must not swallow an unknown child.
func TestExplainPluginArguments(t *testing.T) {
	tree := &Node{}
	MergeCommandPaths(tree, []CommandEntry{{Name: "show sample", Description: "Read sample state."}})
	text, found := NewTreeCompleter(tree).Explain(`show sample "two words"`)
	assert.True(t, found)
	assert.Equal(t, "Read sample state.", text)
	text, found = NewTreeCompleter(tree).Explain("show unknown")
	assert.False(t, found)
	assert.Empty(t, text)
}

// TestCompleteArgumentQuoting keeps literal whitespace and pipes inside an
// argument instead of treating them as command words or pipe operators.
func TestCompleteArgumentQuoting(t *testing.T) {
	tree := &Node{Children: map[string]*Node{
		"request": {Children: map[string]*Node{
			"probe": {WireMethod: "probe", ValueHints: func() []Suggestion {
				return []Suggestion{{Text: "two words", Type: SuggestionValue}, {Text: "pipe|name", Type: SuggestionValue}}
			}},
		}},
	}}
	for _, tc := range []struct{ input, want string }{
		{`request probe "two w`, "two words"},
		{`request probe "pipe|n`, "pipe|name"},
	} {
		assert.Equal(t, []Suggestion{{Text: tc.want, Type: SuggestionValue}}, NewTreeCompleter(tree).Complete(tc.input))
	}
	assert.Empty(t, NewTreeCompleter(tree).Complete(`request unknown "two w`))
}

// TestInvocationEmptyQuotedArgument preserves an explicit empty partial after
// completed arguments, while ordinary token-separating whitespace adds nothing.
func TestInvocationEmptyQuotedArgument(t *testing.T) {
	var line textbuf.Buffer
	line.Str("system command complete args ")
	WriteInvocationValue(&line, "show sample")
	line.Byte(' ')
	WriteInvocationValue(&line, "two words")
	line.Byte(' ')
	WriteInvocationValue(&line, "")
	tokens, pipeAt, open, err := ScanInvocation(line.String())
	require.NoError(t, err)
	assert.False(t, open)
	assert.Equal(t, -1, pipeAt)
	assert.Equal(t, []string{"system", "command", "complete", "args", "show sample", "two words", ""}, tokens)
	tokens, _, _, err = ScanInvocation("show sample   ")
	require.NoError(t, err)
	assert.Equal(t, []string{"show", "sample"}, tokens)
}
