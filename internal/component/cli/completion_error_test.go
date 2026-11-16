// Design: docs/architecture/cli/command-completion.md -- runtime query failures.

package cli

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ze-software/ze/internal/component/command"
)

// TestModelCompletionQueriesOnceAndShowsFailure proves one prompt update reads
// its live argument producer once, derives ghost text, and surfaces a failure.
func TestModelCompletionQueriesOnceAndShowsFailure(t *testing.T) {
	queries := 0
	fail := false
	tree := &command.Node{Children: map[string]*command.Node{"probe": {
		WireMethod: "probe", ArgumentHints: func([]string, string) ([]command.Suggestion, error) {
			queries++
			if fail {
				return nil, context.Canceled
			}
			return []command.Suggestion{{Text: "alpha", Type: command.SuggestionValue}}, nil
		},
	}}}
	m := NewCommandModel(FilesystemAuthorityUnknown)
	m.SetCommandCompleter(NewCommandCompleter(tree))
	m.SetInput("probe a")
	m.UpdateCompletions()
	assert.Equal(t, 1, queries)
	assert.Equal(t, "lpha", m.GhostText())
	fail = true
	m.UpdateCompletions()
	assert.Equal(t, 2, queries)
	assert.Empty(t, m.GhostText())
	assert.Empty(t, m.Completions())
	assert.Contains(t, m.StatusMessage(), context.Canceled.Error())
}
