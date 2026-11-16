//go:build ze_web || ze_ssh

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
// negligible. SSH uses the same live overlay with a session-bound query.
type pluginAwareCommandCompleter struct {
	base      *command.Node                     // YANG tree; never mutated
	entries   func() []command.CommandEntry     // live, non-Hidden plugin commands
	arguments func(*command.Node) *command.Node // Request-owned argument hints.
}

// newPluginAwareCommandCompleter wraps the YANG tree and a live plugin-command
// source. entries must be non-nil.
func newPluginAwareCommandCompleter(tree *command.Node, entries func() []command.CommandEntry, arguments func(*command.Node) *command.Node, surface command.Surface) *pluginAwareCommandCompleter {
	tree = command.WithoutCommandPaths(tree, streamingPathsOff(surface))
	return &pluginAwareCommandCompleter{base: tree, entries: entries, arguments: arguments}
}

// Complete satisfies zeweb.CommandCompleter (cli.Completion is a type alias of
// contract.Completion). YANG completions win on name collision, preserving
// builtin precedence at the completion layer; plugin entries only add tokens the
// YANG tree did not already offer for this input.
func (c *pluginAwareCommandCompleter) Complete(input string) []cli.Completion {
	return c.completer().Complete(input)
}

func (c *pluginAwareCommandCompleter) CompleteChecked(input string) ([]cli.Completion, error) {
	return c.completer().CompleteChecked(input)
}

func (c *pluginAwareCommandCompleter) GhostText(input string) string {
	return c.completer().GhostText(input)
}

// Explain reads the same live overlay as Complete, preserving builtin help
// precedence and removing a departed plugin's explanation on the next request.
func (c *pluginAwareCommandCompleter) Explain(input string) (string, bool) {
	return c.completer().Explain(input)
}

func (c *pluginAwareCommandCompleter) completer() *cli.CommandCompleter {
	tree := command.OverlayCommandPaths(c.base, c.entries())
	if c.arguments != nil {
		tree = c.arguments(tree)
	}
	return cli.NewCommandCompleter(tree)
}
