// Design: docs/architecture/cli/command-completion.md -- shared live argument producer.

package command

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestArgumentHintsRetainArgumentTailAndStaticPrecedence checks both legal
// hint sources together, then a quoted completed argument and empty partial.
func TestArgumentHintsRetainArgumentTailAndStaticPrecedence(t *testing.T) {
	var completed []string
	var partial string
	node := &Node{WireMethod: "probe", ValueHints: func() []Suggestion {
		return []Suggestion{{Text: "alpha", Description: "Static", Type: SuggestionValue}}
	}, ArgumentHints: func(done []string, typed string) ([]Suggestion, error) {
		completed, partial = done, typed
		return []Suggestion{{Text: "alpha", Description: "Live", Type: SuggestionValue}, {Text: "beta", Type: SuggestionValue}}, nil
	}}
	completer := NewTreeCompleter(&Node{Children: map[string]*Node{"probe": node}})
	got, err := completer.CompleteChecked("probe ")
	require.NoError(t, err)
	assert.Equal(t, []Suggestion{{Text: "alpha", Description: "Static", Type: SuggestionValue}, {Text: "beta", Type: SuggestionValue}}, got)
	got, err = completer.CompleteChecked(`probe "two words|one" `)
	require.NoError(t, err)
	assert.Equal(t, []string{"two words|one"}, completed)
	assert.Empty(t, partial)
	assert.Equal(t, "Live", got[0].Description)
	_, err = completer.CompleteChecked(`probe "two words|one" be`)
	require.NoError(t, err)
	assert.Equal(t, "be", partial)
}

// TestArgumentSuggestionsUseSharedWire preserves quoted arguments, short help
// and kind; failed queries and undecodable responses remain failures.
func TestArgumentSuggestionsUseSharedWire(t *testing.T) {
	got, err := ArgumentSuggestions(func(line string) (string, error) {
		words, pipeAt, _, scanErr := ScanInvocation(line)
		require.NoError(t, scanErr)
		assert.Equal(t, -1, pipeAt)
		assert.Equal(t, []string{"system", "command", "complete", "args", "show sample", "two words|one", ""}, words)
		return `{"completions":[{"value":"alpha","short-help":"Live value"}]}`, nil
	}, "show sample", []string{"two words|one"}, "")
	require.NoError(t, err)
	assert.Equal(t, []Suggestion{{Text: "alpha", Description: "Live value", Type: SuggestionValue}}, got)
	got, err = ArgumentSuggestions(func(string) (string, error) { return "", context.Canceled }, "show sample", nil, "")
	assert.ErrorIs(t, err, context.Canceled)
	assert.Nil(t, got)
	got, err = ArgumentSuggestions(func(string) (string, error) { return "not JSON", nil }, "show sample", nil, "")
	assert.Error(t, err)
	assert.Nil(t, got)
	got, err = ArgumentSuggestions(func(string) (string, error) { return "{}", nil }, "show sample", nil, "")
	assert.Error(t, err)
	assert.Nil(t, got)
}
