// Design: docs/architecture/cli/command-completion.md -- builtin argument completion
// Related: system.go -- handleArgComplete, the RPC every caller outside the daemon asks
// Related: register_plugin_restart.go -- completePluginRestart, a completer it serves
//
// A builtin command whose argument names something only the running daemon
// knows (a configured plugin, say) declares an ArgCompleter beside its handler
// in RegisterRPCs. One completer then answers every interactive surface: the
// `system command complete args` RPC for a caller outside the daemon, and the
// value hint WireArgCompletion hangs on the SSH session's own tree.

package server

import (
	"strings"

	"github.com/ze-software/ze/internal/component/command"
)

// ArgCompleter answers every value the command's argument may take now, read
// from the daemon's runtime state. It never filters by what was typed: each
// caller matches its own prefix (handleArgComplete, command.TreeCompleter). It
// MUST be safe for concurrent use, because every session's tab calls it.
type ArgCompleter func(s *Server) []Completion

// builtinArgCompletions answers the candidates the builtin command name
// completes that start with partial. ok is false when no builtin of that name
// declares a completer, so the caller can try the plugin commands instead.
func (d *Dispatcher) builtinArgCompletions(s *Server, name, partial string) (candidates []Completion, ok bool) {
	builtin := d.Lookup(name)
	if builtin == nil {
		return nil, false
	}
	if builtin.Complete == nil {
		return nil, false
	}
	candidates = []Completion{}
	for _, candidate := range builtin.Complete(s) {
		if strings.HasPrefix(candidate.Value, partial) {
			candidates = append(candidates, candidate)
		}
	}
	return candidates, true
}

// WireArgCompletion hangs the completer of every builtin that declares one on
// its node in tree, as the node's value hints, so Tab after the command offers
// what the running daemon holds. A command tree carries no node for a command
// it does not offer, and such a command is skipped. A nil server or dispatcher
// wires nothing: no daemon state is there to complete from.
func (s *Server) WireArgCompletion(tree *command.Node) {
	if s == nil {
		return
	}
	d := s.Dispatcher()
	if d == nil {
		return
	}
	for _, builtin := range d.Commands() {
		if builtin.Complete == nil {
			continue
		}
		node := command.FindNode(tree, strings.Fields(builtin.Name))
		if node == nil {
			continue
		}
		complete := builtin.Complete
		node.ValueHints = func() []command.Suggestion {
			return suggestionsOf(complete(s))
		}
	}
}

// suggestionsOf converts completer candidates to the command tree's value
// suggestions, the form a tab completion lists.
func suggestionsOf(candidates []Completion) []command.Suggestion {
	suggestions := make([]command.Suggestion, 0, len(candidates))
	for _, candidate := range candidates {
		suggestions = append(suggestions, command.Suggestion{
			Text:        candidate.Value,
			Description: candidate.ShortHelp,
			Type:        command.SuggestionValue,
		})
	}
	return suggestions
}
