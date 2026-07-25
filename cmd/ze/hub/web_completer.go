//go:build ze_web

// Design: docs/architecture/api/commands.md -- plugin command completion injection
// Overview: service_web.go -- web service that builds the completer
// Related: session_factory.go -- the SSH counterpart (mergePluginCommands)
package hub

import (
	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/command"
)

// pluginAwareCommandCompleter serves operational completions from an immutable
// YANG command tree PLUS a live overlay of plugin-registered commands rebuilt on
// every request. The web command tree is long-lived and shared, so plugin
// commands are deliberately NOT merged into it (that would race with other
// readers and, worse, go stale). Instead a throwaway overlay tree is built per
// request from the current registry, so a plugin that registered or exited since
// the last keystroke is reflected immediately -- this is the spec's R-1
// mitigation ("clear and rebuild on re-registration") and also covers the R-2
// startup race (plugins register after the web service is built; the first
// requests simply see fewer commands). The overlay is a handful of nodes and
// completion runs at human-typing frequency, so the per-request cost is
// negligible. The SSH surface achieves the same liveness by rebuilding its whole
// tree per session (session_factory.go).
type pluginAwareCommandCompleter struct {
	base    *cli.CommandCompleter         // YANG tree; never mutated
	entries func() []command.CommandEntry // live, non-Hidden plugin commands
}

// newPluginAwareCommandCompleter wraps the YANG tree and a live plugin-command
// source. entries must be non-nil.
func newPluginAwareCommandCompleter(tree *command.Node, entries func() []command.CommandEntry) *pluginAwareCommandCompleter {
	return &pluginAwareCommandCompleter{base: cli.NewCommandCompleter(tree), entries: entries}
}

// Complete satisfies zeweb.CommandCompleter (cli.Completion is a type alias of
// contract.Completion). YANG completions win on name collision, preserving
// builtin precedence at the completion layer; plugin entries only add tokens the
// YANG tree did not already offer for this input.
func (c *pluginAwareCommandCompleter) Complete(input string) []cli.Completion {
	out := c.base.Complete(input)
	overlay := &command.Node{Children: map[string]*command.Node{}}
	command.MergeCommandPaths(overlay, c.entries())
	seen := make(map[string]bool, len(out))
	for _, s := range out {
		seen[s.Text] = true
	}
	for _, s := range cli.NewCommandCompleter(overlay).Complete(input) {
		if !seen[s.Text] {
			out = append(out, s)
			seen[s.Text] = true
		}
	}
	return out
}
