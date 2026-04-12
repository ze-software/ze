// Design: docs/architecture/bfd.md -- BFD CLI handlers
//
// Package bfd registers engine-side RPC handlers that expose the BFD
// plugin's observability surface to the CLI. Unlike the bgp-rib or
// sysrib proxies that hop through ForwardToPlugin, BFD is an
// in-process plugin and publishes its api.Service via
// internal/plugins/bfd/api. The handlers call GetService() directly
// and format the response; the plugin process boundary does not
// apply.
//
// Two package-level schemas register via init():
//
//   - internal/plugins/bfd/schema (ze-bfd-api.yang) -- RPC definitions
//   - internal/component/cmd/bfd/schema (ze-bfd-cmd.yang) -- CLI tree
//
// Both are imported here so a blank import of this package wires the
// CLI surface completely without touching the core dispatcher.
package bfd

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
	bfdapi "codeberg.org/thomas-mangin/ze/internal/plugins/bfd/api"

	_ "codeberg.org/thomas-mangin/ze/internal/component/cmd/bfd/schema" // register ze-bfd-cmd.yang
	_ "codeberg.org/thomas-mangin/ze/internal/plugins/bfd/schema"       // register ze-bfd-api.yang
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
		return &plugin.Response{Status: plugin.StatusError, Data: errBFDServiceUnavailable.Error()}, nil
	}
	sessions := svc.Snapshot()
	data, err := json.Marshal(sessions)
	if err != nil {
		return nil, fmt.Errorf("bfd show sessions: marshal: %w", err)
	}
	return &plugin.Response{Status: plugin.StatusDone, Data: string(data)}, nil
}

// handleShowSession returns one session matched by peer address. The
// peer is read either from the first positional argument (interactive
// CLI) or the `peer` YANG input leaf (programmatic callers); this
// handler accepts the positional form because `ze show bfd session
// <peer>` is how operators type it.
func handleShowSession(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	peer := ""
	for _, a := range args {
		if a == "" || strings.HasPrefix(a, "-") {
			continue
		}
		peer = a
		break
	}
	if peer == "" {
		return &plugin.Response{Status: plugin.StatusError, Data: "bfd show session: missing peer argument"}, nil
	}
	if _, err := netip.ParseAddr(peer); err != nil {
		return &plugin.Response{Status: plugin.StatusError, Data: fmt.Sprintf("bfd show session: invalid peer %q: %v", peer, err)}, nil
	}
	svc := bfdapi.GetService()
	if svc == nil {
		return &plugin.Response{Status: plugin.StatusError, Data: errBFDServiceUnavailable.Error()}, nil
	}
	session, ok := svc.SessionDetail(peer)
	if !ok {
		return &plugin.Response{Status: plugin.StatusError, Data: fmt.Sprintf("bfd: no session for peer %s", peer)}, nil
	}
	data, err := json.Marshal(session)
	if err != nil {
		return nil, fmt.Errorf("bfd show session: marshal: %w", err)
	}
	return &plugin.Response{Status: plugin.StatusDone, Data: string(data)}, nil
}

// handleShowProfile returns the set of configured profiles. An empty
// argument list returns every profile; a single profile name filters
// to one entry. An unknown profile returns an error so operators see
// a clear "not found" message.
func handleShowProfile(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	svc := bfdapi.GetService()
	if svc == nil {
		return &plugin.Response{Status: plugin.StatusError, Data: errBFDServiceUnavailable.Error()}, nil
	}
	profiles := svc.Profiles()
	wanted := ""
	for _, a := range args {
		if a == "" || strings.HasPrefix(a, "-") {
			continue
		}
		wanted = a
		break
	}
	if wanted != "" {
		for i := range profiles {
			if profiles[i].Name == wanted {
				data, err := json.Marshal(profiles[i])
				if err != nil {
					return nil, fmt.Errorf("bfd show profile: marshal: %w", err)
				}
				return &plugin.Response{Status: plugin.StatusDone, Data: string(data)}, nil
			}
		}
		return &plugin.Response{Status: plugin.StatusError, Data: fmt.Sprintf("bfd: no profile named %q", wanted)}, nil
	}
	data, err := json.Marshal(profiles)
	if err != nil {
		return nil, fmt.Errorf("bfd show profile: marshal: %w", err)
	}
	return &plugin.Response{Status: plugin.StatusDone, Data: string(data)}, nil
}
