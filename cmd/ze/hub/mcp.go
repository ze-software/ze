// Design: docs/architecture/hub-architecture.md -- MCP server startup
// Overview: main.go -- hub CLI entry point

package hub

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	bgpreactor "codeberg.org/thomas-mangin/ze/internal/component/bgp/reactor"
	"codeberg.org/thomas-mangin/ze/internal/component/config/yang"
	zemcp "codeberg.org/thomas-mangin/ze/internal/component/mcp"
)

// commandLister returns a CommandLister that queries the reactor's dispatcher
// for all registered commands. Called at tools/list time so it always reflects
// the current set of registered YANG commands and plugin commands.
//
// YANG RPC input parameters are extracted once (lazily) and cached. The
// parameters appear as typed JSON Schema properties in auto-generated tools.
func commandLister(r *bgpreactor.Reactor) zemcp.CommandLister {
	// Lazy-init: build path->params map once from YANG RPC metadata.
	var (
		paramOnce    sync.Once
		paramsByPath map[string][]zemcp.ParamInfo // CLI path -> params
	)

	initParams := func() {
		paramOnce.Do(func() {
			paramsByPath = buildParamMap()
		})
	}

	return func() []zemcp.CommandInfo {
		api := r.APIServer()
		if api == nil {
			return nil
		}
		d := api.Dispatcher()
		if d == nil {
			return nil
		}

		initParams()

		var infos []zemcp.CommandInfo

		// Builtin commands from YANG registrations.
		for _, cmd := range d.Commands() {
			infos = append(infos, zemcp.CommandInfo{
				Name:     cmd.Name,
				Help:     cmd.Help,
				ReadOnly: cmd.ReadOnly,
				Params:   paramsByPath[cmd.Name],
			})
		}

		// Plugin commands from runtime registrations.
		for _, cmd := range d.Registry().All() {
			infos = append(infos, zemcp.CommandInfo{
				Name: cmd.Name,
				Help: cmd.Description,
			})
		}

		return infos
	}
}

// buildParamMap creates a YANG loader, extracts all RPC metadata, and builds
// a map from CLI command path to input parameters.
func buildParamMap() map[string][]zemcp.ParamInfo {
	loader, err := yang.DefaultLoader()
	if err != nil {
		return nil
	}

	// Build reverse map: CLI path -> wire method.
	wireToPath := yang.WireMethodToPath(loader)
	pathToWire := make(map[string]string, len(wireToPath))
	for wire, path := range wireToPath {
		pathToWire[path] = wire
	}

	// Extract RPC input params for each command path.
	result := make(map[string][]zemcp.ParamInfo)
	for path, wire := range pathToWire {
		// Wire method format: "module:rpc-name". Extract module, add "-api" suffix.
		module := wireModule(wire)
		rpcName := wireRPC(wire)
		if module == "" || rpcName == "" {
			continue
		}

		rpcs := yang.ExtractRPCs(loader, module+"-api")
		if rpcs == nil {
			// Try without -api suffix (some modules use -cmd).
			rpcs = yang.ExtractRPCs(loader, module+"-cmd")
		}
		for _, rpc := range rpcs {
			if rpc.Name != rpcName {
				continue
			}
			if len(rpc.Input) == 0 {
				break
			}
			params := make([]zemcp.ParamInfo, len(rpc.Input))
			for i, leaf := range rpc.Input {
				params[i] = zemcp.ParamInfo{
					Name:        leaf.Name,
					Type:        leaf.Type,
					Description: leaf.Description,
					Required:    leaf.Mandatory,
				}
			}
			result[path] = params
			break
		}
	}

	return result
}

// wireModule extracts the module prefix from a wire method (e.g. "ze-bgp:peer-list" -> "ze-bgp").
func wireModule(wire string) string {
	mod, _, ok := strings.Cut(wire, ":")
	if !ok {
		return ""
	}
	return mod
}

// wireRPC extracts the RPC name from a wire method (e.g. "ze-bgp:peer-list" -> "peer-list").
func wireRPC(wire string) string {
	_, rpc, ok := strings.Cut(wire, ":")
	if !ok {
		return ""
	}
	return rpc
}

// startMCPServer creates and starts an MCP HTTP server on the given address.
// Returns the server on success, nil on failure (logged, non-fatal).
// MUST call Shutdown on the returned server before stopping the reactor.
func startMCPServer(addr string, dispatch zemcp.CommandDispatcher, commands zemcp.CommandLister, token string) *http.Server {
	srv := &http.Server{
		Addr:              addr,
		Handler:           zemcp.Handler(dispatch, commands, token),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", addr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: MCP server disabled: %v\n", err)
		return nil
	}

	// Lifecycle goroutine: started once at component startup, runs for daemon lifetime.
	go serveMCP(srv, ln)

	fmt.Fprintf(os.Stderr, "MCP server listening on http://%s/\n", addr)
	return srv
}

// serveMCP runs the MCP HTTP server. Started once as a lifecycle goroutine.
func serveMCP(srv *http.Server, ln net.Listener) {
	if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		fmt.Fprintf(os.Stderr, "warning: MCP server: %v\n", err)
	}
}
