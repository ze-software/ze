// Design: plan/spec-cp-survival-4-flowspec-origination.md -- BGP reactor type assertion for announce cmd

package announce

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
