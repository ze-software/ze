// Design: docs/architecture/cli/command-completion.md -- builtin argument completion
// Related: system.go -- handleArgComplete, the RPC every caller outside the daemon asks
// Related: register_plugin_restart.go -- completePluginRestart, a completer it serves
//
// A command whose argument names runtime state declares its completer with its
// handler. Every interactive surface asks the same system completion RPC under
// its own authenticated caller; no tree callback bypasses that authorization.

package server

import (
	"strings"

	"github.com/ze-software/ze/internal/component/command"
)

// ArgCompleter answers the values permitted at the position after completed,
// read from the daemon's runtime state. completed contains whole argument
// values, not command path words or the current partial. The producer MUST
// neither mutate nor retain that caller-owned slice. Prefix filtering remains
// with handleArgComplete and command.TreeCompleter. Safe for concurrent use:
// every session can call the same registered producer.
type ArgCompleter func(s *Server, completed []string) []Completion

// builtinArgCompletions answers the candidates the builtin command name
// completes that start with partial. ok is false when no builtin of that name
// declares a completer, so the caller can try the plugin commands instead.
func (d *Dispatcher) builtinArgCompletions(s *Server, name string, completed []string, partial string) (candidates []Completion, ok bool) {
	builtin := d.Lookup(name)
	if builtin == nil {
		return nil, false
	}
	if builtin.Complete == nil {
		return nil, false
	}
	candidates = []Completion{}
	for _, candidate := range builtin.Complete(s, completed) {
		if strings.HasPrefix(candidate.Value, partial) {
			candidates = append(candidates, candidate)
		}
	}
	return candidates, true
}

// WireArgCompletion hangs the shared RPC producer of every completable command on
// its node in tree, as argument hints, so Tab after the command offers
// what the running daemon holds. A command tree carries no node for a command
// it does not offer, and such a command is skipped. A nil server or dispatcher
// wires nothing: no daemon state is there to complete from.
func (s *Server) WireArgCompletion(tree *command.Node, query command.ArgumentQuery) {
	if s == nil {
		return
	}
	d := s.Dispatcher()
	if d == nil {
		return
	}
	wireArgumentPaths(tree, s.argumentPaths(tree), query)
}

// wireArgumentPaths consumes one registration snapshot. The immutable overlay
// copies these exact paths before this writer sees them, even during reload.
func wireArgumentPaths(tree *command.Node, paths []command.CommandEntry, query command.ArgumentQuery) {
	for _, path := range paths {
		node := command.FindNode(tree, strings.Fields(path.Name))
		if node == nil {
			continue
		}
		name := path.Name
		node.ArgumentHints = func(completed []string, partial string) ([]command.Suggestion, error) {
			return command.ArgumentSuggestions(query, name, completed, partial)
		}
	}
}

// WithArgCompletion returns a request-owned overlay carrying live builtin/plugin
// argument producers. Only paths WireArgCompletion writes are copied; unrelated
// subtrees stay immutable and shared. The input tree MUST NOT be mutated by the
// caller while this function reads it. Safe for concurrent completion requests.
func (s *Server) WithArgCompletion(tree *command.Node, query command.ArgumentQuery) *command.Node {
	if s == nil {
		return tree
	}
	paths := s.argumentPaths(tree)
	overlay := command.OverlayCommandPaths(tree, paths)
	wireArgumentPaths(overlay, paths, query)
	return overlay
}

// argumentPaths reads actual registrations and only paths the surface offers.
// A builtin wins a collision just as it does in command dispatch.
func (s *Server) argumentPaths(tree *command.Node) []command.CommandEntry {
	d := s.Dispatcher()
	if d == nil {
		return nil
	}
	var paths []command.CommandEntry
	for _, builtin := range d.Commands() {
		if builtin.Complete == nil {
			continue
		}
		if command.FindNode(tree, strings.Fields(builtin.Name)) == nil {
			continue
		}
		paths = append(paths, command.CommandEntry{Name: builtin.Name})
	}
	for _, registered := range d.Registry().All() {
		if !registered.Completable {
			continue
		}
		if builtin := d.Lookup(registered.Name); builtin != nil {
			if builtin.Complete != nil {
				continue
			}
		}
		if command.FindNode(tree, strings.Fields(registered.Name)) == nil {
			continue
		}
		paths = append(paths, command.CommandEntry{Name: registered.Name})
	}
	return paths
}
