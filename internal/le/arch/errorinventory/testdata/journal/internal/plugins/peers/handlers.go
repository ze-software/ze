// Fixture: a registered handler's StatusError answer, before and after its
// fix (plan/journal/error-names-no-fix-command.md).
package peers

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/opererr"
)

// answerBefore is the pre-fix form: a plain string in Response.Error.
func answerBefore(_ *pluginserver.CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
	return &plugin.Response{Status: plugin.StatusError, Error: "peer not found"}, nil // shape:handler-before
}

// answerAfter is the fixed form: the operator error through the one typed
// route into Response.Error.
func answerAfter(_ *pluginserver.CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
	return plugin.OperatorErrorResponse(opererr.New("peer not found", "no session is configured for that address", "ze config edit")), nil // shape:handler-after
}
