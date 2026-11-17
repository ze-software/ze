// Design: docs/architecture/cli/command-completion.md -- immutable caller overlays.

package server

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
)

// TestArgumentCompletionOverlayKeepsBaseAndCallerQueriesIndependent proves the
// new callback is absent from the shared base, each request owns its path, and
// building a later caller's overlay cannot replace the earlier caller's query.
func TestArgumentCompletionOverlayKeepsBaseAndCallerQueriesIndependent(t *testing.T) {
	d := NewDispatcher()
	require.NoError(t, d.RegisterWithOptions("show isolated arguments", func(*CommandContext, command.ValidatedArgs) (*plugin.Response, error) {
		t.Fatal("completion executed the target")
		return plugin.NewResponse(plugin.StatusDone, plugin.Map{}), nil
	}, "Inspect argument values", RegisterOptions{Complete: func(*Server, []string) []Completion {
		t.Fatal("overlay construction bypassed its bound query")
		return nil
	}}))
	baseLeaf := &command.Node{ValueHints: func() []command.Suggestion {
		return []command.Suggestion{{Text: "static", Type: command.SuggestionValue}}
	}}
	unrelated := &command.Node{}
	base := &command.Node{Children: map[string]*command.Node{
		"show": {Children: map[string]*command.Node{
			"isolated": {Children: map[string]*command.Node{"arguments": baseLeaf}},
		}},
		"unrelated": unrelated,
	}}
	s := &Server{dispatcher: d}
	firstCalls, secondCalls := 0, 0
	first := s.WithArgCompletion(base, func(string) (string, error) {
		firstCalls++
		return `{"completions":[{"value":"first-caller"}]}`, nil
	})
	second := s.WithArgCompletion(base, func(string) (string, error) {
		secondCalls++
		return `{"completions":[{"value":"second-caller"}]}`, nil
	})
	path := []string{"show", "isolated", "arguments"}
	assert.Nil(t, baseLeaf.ArgumentHints)
	assert.NotSame(t, baseLeaf, command.FindNode(first, path))
	assert.NotSame(t, baseLeaf, command.FindNode(second, path))
	assert.NotSame(t, command.FindNode(first, path), command.FindNode(second, path))
	assert.Same(t, unrelated, first.Children["unrelated"])
	assert.Same(t, unrelated, second.Children["unrelated"])
	assert.Zero(t, firstCalls)
	assert.Zero(t, secondCalls)
	firstValues, err := command.NewTreeCompleter(first).CompleteChecked("show isolated arguments ")
	require.NoError(t, err)
	secondValues, err := command.NewTreeCompleter(second).CompleteChecked("show isolated arguments ")
	require.NoError(t, err)
	assert.Equal(t, []command.Suggestion{{Text: "static", Type: command.SuggestionValue}, {Text: "first-caller", Type: command.SuggestionValue}}, firstValues)
	assert.Equal(t, []command.Suggestion{{Text: "static", Type: command.SuggestionValue}, {Text: "second-caller", Type: command.SuggestionValue}}, secondValues)
	assert.Equal(t, 1, firstCalls)
	assert.Equal(t, 1, secondCalls)
	assert.Nil(t, baseLeaf.ArgumentHints)
	assert.Equal(t, []command.Suggestion{{Text: "static", Type: command.SuggestionValue}}, baseLeaf.ValueHints())
}
