// Design: docs/architecture/api/commands.md — BGP reactor type assertion

package handler

import (
	"errors"

	bgptypes "codeberg.org/thomas-mangin/ze/internal/component/bgp/types"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

var (
	errReactorNotAvailable    = errors.New("reactor not available")
	errBgpReactorNotAvailable = errors.New("BGP reactor not available")
)

// requireBGPReactor returns the reactor as a BGPReactor or an error response.
// Use this when the handler needs BGP-specific operations (route announce,
// cache, RIB, raw message, etc.) that are not part of ReactorLifecycle.
func requireBGPReactor(ctx *pluginserver.CommandContext) (bgptypes.BGPReactor, *plugin.Response, error) {
	r := ctx.Reactor()
	if r == nil {
		return nil, &plugin.Response{
			Status: plugin.StatusError,
			Data:   "reactor not available",
		}, errReactorNotAvailable
	}
	bgp, ok := r.(bgptypes.BGPReactor)
	if !ok {
		return nil, &plugin.Response{
			Status: plugin.StatusError,
			Data:   "BGP reactor not available",
		}, errBgpReactorNotAvailable
	}
	return bgp, nil, nil
}
