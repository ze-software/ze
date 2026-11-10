// Fixture: the daemon-command and config-verify sites.
package fixture

import (
	"errors"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/rpc"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// handlePeer is the daemon-command site. Its return is also a gRPC status
// writer call on the same line, so two sources name it (AC-2).
func handlePeer(_ *pluginserver.CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
	return nil, status.Error(codes.NotFound, "peer missing: run ze show bgp peer") // site:overlap
}

// verifyFixture is the config-verify site.
func verifyFixture(_ []rpc.ConfigSection) error {
	return errors.New("fixture section invalid")
}
