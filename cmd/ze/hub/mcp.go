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
	"time"

	bgpreactor "codeberg.org/thomas-mangin/ze/internal/component/bgp/reactor"
	zemcp "codeberg.org/thomas-mangin/ze/internal/component/mcp"
)

// commandLister returns a CommandLister that queries the reactor's dispatcher
// for all registered commands. Called at tools/list time so it always reflects
// the current set of registered YANG commands and plugin commands.
func commandLister(r *bgpreactor.Reactor) zemcp.CommandLister {
	return func() []zemcp.CommandInfo {
		api := r.APIServer()
		if api == nil {
			return nil
		}
		d := api.Dispatcher()
		if d == nil {
			return nil
		}

		var infos []zemcp.CommandInfo

		// Builtin commands from YANG registrations.
		for _, cmd := range d.Commands() {
			infos = append(infos, zemcp.CommandInfo{
				Name:     cmd.Name,
				Help:     cmd.Help,
				ReadOnly: cmd.ReadOnly,
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
