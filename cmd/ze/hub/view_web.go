//go:build ze_web

// Design: docs/architecture/web-interface.md -- admitted caller-bound browser views.
// Related: view_factory.go -- same factory composition used by SSH.

package hub

import (
	"context"

	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/plugin"
	zeweb "github.com/ze-software/ze/internal/component/web"
	"github.com/ze-software/ze/internal/core/opererr"
)

func webViewBinding(dispatch plugin.CommandDispatcher) zeweb.CLIViewBinding {
	return func(ctx context.Context, caller plugin.CallerIdentity, model *cli.Model) error {
		if dispatch == nil {
			return opererr.New("The daemon cannot answer live monitor requests.", "No command dispatcher is available for this web server.", "Start the daemon and open /cli again")
		}
		execute := func(input string) (*plugin.RenderedResponse, error) {
			return dispatch.JSON(ctx, caller, input)
		}
		model.SetCommandExecutor(cliExecutor(execute))
		injectViewFactories(model, execute)
		return nil
	}
}
