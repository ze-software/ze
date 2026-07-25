// Design: docs/architecture/bfd.md -- BFD CLI handlers
//
// Package bfd registers engine-side RPC handlers that expose the BFD
// plugin's observability surface to the CLI. Unlike the bgp-rib or
// sysrib proxies that hop through ForwardToPlugin, BFD is an
// in-process plugin and publishes its api.Service via
// internal/component/bfd/api. The handlers call GetService() directly
// and format the response; the plugin process boundary does not
// apply.
//
// Two package-level schemas register via init():
//
//   - internal/component/bfd/yang (ze-bfd-api.yang) -- RPC definitions
//   - internal/component/bfd/yang (ze-bfd-cmd.yang) -- CLI tree
//
// Both are imported here so a blank import of this package wires the
// CLI surface completely without touching the core dispatcher.
package cmd

import (
	"errors"
	"net/netip"
	"strings"

	bfdapi "github.com/ze-software/ze/internal/component/bfd/api"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"

	_ "github.com/ze-software/ze/internal/component/bfd/yang" // register ze-bfd-cmd.yang + ze-bfd-api.yang
)

// errBFDServiceUnavailable is returned when a show command runs while
// the BFD plugin has not published its Service (plugin not loaded, or
// shutting down). The handler converts it into a plugin.StatusError
// response so the CLI prints a clear message instead of a generic
// failure.
var errBFDServiceUnavailable = errors.New("bfd: plugin not loaded")

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{
			WireMethod: "ze-bfd-api:show-sessions",
			Handler:    handleShowSessions,
		},
		pluginserver.RPCRegistration{
			WireMethod: "ze-bfd-api:show-session",
			Handler:    handleShowSession,
		},
		pluginserver.RPCRegistration{
			WireMethod: "ze-bfd-api:show-profile",
			Handler:    handleShowProfile,
		},
	)
}

// handleShowSessions returns every live session as a JSON array.
// Called via `ze show bfd sessions` or the interactive CLI
// `show bfd sessions`.
func handleShowSessions(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	svc := bfdapi.GetService()
	if svc == nil {
		return &plugin.Response{Status: plugin.StatusError, Error: errBFDServiceUnavailable.Error()}, nil
	}
	return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Slice[bfdapi.SessionState](svc.Snapshot())}, nil
}

// handleShowSession returns one session matched by peer address.
// Called via `show bfd session address <peer>`.
func handleShowSession(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	peer := ""
	if ctx != nil {
		peer = ctx.Selector("address")
	}
	if peer == "" {
		for _, a := range args {
			if a == "" || strings.HasPrefix(a, "-") {
				continue
			}
			peer = a
			break
		}
	}
	if peer == "" {
		return &plugin.Response{Status: plugin.StatusError, Error: "usage: show bfd session address <peer>"}, nil
	}
	if _, err := netip.ParseAddr(peer); err != nil {
		return &plugin.Response{Status: plugin.StatusError, Error: "bfd: invalid peer address " + peer + ": " + err.Error()}, nil //nolint:nilerr // operational error in Response
	}
	svc := bfdapi.GetService()
	if svc == nil {
		return &plugin.Response{Status: plugin.StatusError, Error: errBFDServiceUnavailable.Error()}, nil
	}
	session, ok := svc.SessionDetail(peer)
	if !ok {
		return &plugin.Response{Status: plugin.StatusError, Error: "bfd: no session for peer " + peer}, nil
	}
	return &plugin.Response{Status: plugin.StatusDone, Data: session}, nil
}

// handleShowProfile returns the set of configured profiles. An empty
// argument list returns every profile; `show bfd profile name <name>`
// filters to one entry. An unknown profile returns an error so operators see
// a clear "not found" message.
func handleShowProfile(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	svc := bfdapi.GetService()
	if svc == nil {
		return &plugin.Response{Status: plugin.StatusError, Error: errBFDServiceUnavailable.Error()}, nil
	}
	profiles := svc.Profiles()
	wanted := ""
	if ctx != nil {
		wanted = ctx.Selector("name")
	}
	if wanted == "" {
		for _, a := range args {
			if a == "" || strings.HasPrefix(a, "-") {
				continue
			}
			wanted = a
			break
		}
	}
	if wanted != "" {
		for i := range profiles {
			if profiles[i].Name == wanted {
				return &plugin.Response{Status: plugin.StatusDone, Data: profiles[i]}, nil
			}
		}
		return &plugin.Response{Status: plugin.StatusError, Error: "bfd: no profile named " + wanted}, nil
	}
	return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Slice[bfdapi.ProfileState](profiles)}, nil
}
