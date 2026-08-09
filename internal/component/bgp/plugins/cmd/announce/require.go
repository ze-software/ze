// Design: docs/architecture/bgp/on-demand-origination.md -- BGP reactor type assertion for announce cmd

package announce

import (
	"errors"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

var (
	errReactorNotAvailable    = errors.New("reactor not available")
	errBgpReactorNotAvailable = errors.New("BGP reactor not available")
)

func requireBGPReactor(ctx *pluginserver.CommandContext) (bgptypes.BGPReactor, *plugin.Response, error) {
	r := ctx.Reactor()
	if r == nil {
		return nil, &plugin.Response{
			Status: plugin.StatusError,
			Error:  "reactor not available",
		}, errReactorNotAvailable
	}
	bgp, ok := r.(bgptypes.BGPReactor)
	if !ok {
		return nil, &plugin.Response{
			Status: plugin.StatusError,
			Error:  "BGP reactor not available",
		}, errBgpReactorNotAvailable
	}
	return bgp, nil, nil
}
