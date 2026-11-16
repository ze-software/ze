//go:build ze_web

// Design: docs/architecture/cli/command-completion.md -- request-bound browser hints.

package hub

import (
	"context"

	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	zeweb "github.com/ze-software/ze/internal/component/web"
)

// webCompletionSource shares immutable metadata, binding I/O only to the caller
// authenticated for this request. Web-only mode has no runtime producer.
func webCompletionSource(tree *command.Node, entries func() []command.CommandEntry, arguments func(*command.Node, command.ArgumentQuery) *command.Node, dispatch plugin.CommandDispatcher) zeweb.CommandCompleterFactory {
	return func(ctx context.Context, caller plugin.CallerIdentity) zeweb.CommandCompleter {
		if entries == nil {
			return cli.NewCommandCompleter(tree)
		}
		var hints func(*command.Node) *command.Node
		if arguments != nil {
			query := func(line string) (string, error) {
				answer, err := dispatch.JSON(ctx, caller, line)
				defer answer.TransportComplete()
				return answer.Output, err
			}
			hints = func(base *command.Node) *command.Node { return arguments(base, query) }
		}
		return newPluginAwareCommandCompleter(tree, entries, hints, command.SurfaceWebCompletion)
	}
}
