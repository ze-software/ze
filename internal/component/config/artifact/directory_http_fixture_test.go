// Design: docs/architecture/web-interface.md -- backup settings captured from actual running configuration.
package artifact_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/artifact"
	configcli "github.com/ze-software/ze/internal/component/config/cli"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/component/web"
)

func backupDirectoryHTTPFixture(t *testing.T, running map[string]any) (*artifact.Owner, policyHTTP) {
	t.Helper()
	server, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, plugin.NewCoordinator(running))
	require.NoError(t, err)
	t.Cleanup(server.Stop)
	owner := artifact.New()
	require.NoError(t, configcli.AttachArtifactOwner(server, owner))
	dispatch := func(ctx context.Context, caller plugin.CallerIdentity, line string) (*plugin.Response, error) {
		return server.Dispatcher().Dispatch(&pluginserver.CommandContext{Server: server, Username: caller.Username,
			Authorizer: caller.Authorizer, RequestContext: ctx, Surface: "web", Sender: plugin.OperatorSender()}, line)
	}
	grant := policyGrant{edit: true}
	handler := web.HandleBackupWorkspace(web.RouteDeps{Dispatch: dispatch, Authorizer: grant})
	authenticated := web.AuthMiddlewareWithAudit(web.NewSessionStore(nil), policyAuthentication{grant: grant},
		func(http.ResponseWriter, *http.Request) {}, handler, nil)
	return owner, policyHTTP{owner: owner, server: server, handler: authenticated, dispatch: dispatch,
		source: newPolicyHTTP(t, nil, false).source}
}
