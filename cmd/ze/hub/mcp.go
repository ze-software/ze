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
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/config/yang"
	zemcp "codeberg.org/thomas-mangin/ze/internal/component/mcp"
)

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

// startMCPServer creates and starts an MCP HTTP server bound to every entry
// in addrs. Returns the single *http.Server on success, nil on failure
// (logged, non-fatal). Shutdown on the returned server closes every listener.
// MUST call Shutdown before stopping the reactor.
//
// Bind is all-or-nothing: if ANY listener fails to bind, the already-bound
// listeners are closed and the function returns nil.
//
// Speaks the MCP 2025-06-18 Streamable HTTP profile (sessions, SSE, GET/DELETE).
func startMCPServer(addrs []string, dispatch zemcp.CommandDispatcher, commands zemcp.CommandLister, token string) *http.Server {
	if len(addrs) == 0 {
		fmt.Fprintln(os.Stderr, "warning: MCP server disabled: no listen addresses")
		return nil
	}

	handler, err := zemcp.NewStreamable(zemcp.StreamableConfig{
		Dispatch: dispatch,
		Commands: commands,
		Token:    token,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: MCP server disabled: %v\n", err)
		return nil
	}

	srv := &http.Server{
		// Addr is informational; multi-listener serving uses Serve(ln).
		Addr:              addrs[0],
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	var lc net.ListenConfig
	listeners := make([]net.Listener, 0, len(addrs))
	for _, addr := range addrs {
		ln, err := lc.Listen(context.Background(), "tcp", addr)
		if err != nil {
			for _, prev := range listeners {
				if closeErr := prev.Close(); closeErr != nil {
					fmt.Fprintf(os.Stderr, "warning: MCP server: close partial listener: %v\n", closeErr)
				}
			}
			fmt.Fprintf(os.Stderr, "warning: MCP server disabled: bind %s: %v\n", addr, err)
			return nil
		}
		listeners = append(listeners, ln)
	}

	// Lifecycle goroutine per listener: started once at component startup,
	// runs for daemon lifetime. Shutdown closes every listener because all
	// were passed to the same http.Server.
	for _, ln := range listeners {
		go serveMCP(srv, ln)
		fmt.Fprintf(os.Stderr, "MCP server listening on http://%s/\n", ln.Addr().String())
	}

	return srv
}

// serveMCP runs the MCP HTTP server on one listener. Started once as a
// lifecycle goroutine per configured address.
func serveMCP(srv *http.Server, ln net.Listener) {
	if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		fmt.Fprintf(os.Stderr, "warning: MCP server: %v\n", err)
	}
}
