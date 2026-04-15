// Design: docs/architecture/api/architecture.md -- API server startup
// Overview: main.go -- hub CLI entry point

package hub

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"

	"codeberg.org/thomas-mangin/ze/internal/component/api"
	apigrpc "codeberg.org/thomas-mangin/ze/internal/component/api/grpc"
	"codeberg.org/thomas-mangin/ze/internal/component/api/rest"
	"codeberg.org/thomas-mangin/ze/internal/component/authz"
	"codeberg.org/thomas-mangin/ze/internal/component/cli"
	zeconfig "codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/component/config/storage"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

// apiServers holds running API servers for shutdown.
type apiServers struct {
	rest *rest.RESTServer
	grpc *apigrpc.GRPCServer
}

// startAPIServers creates the shared API engine and starts REST and/or gRPC
// servers based on the config. Returns nil if neither transport is enabled.
func startAPIServers(cfg zeconfig.APIConfig, server *pluginserver.Server, store storage.Storage, configPath string, users []authz.UserConfig) *apiServers {
	engine := buildAPIEngine(server)
	sessions := api.NewConfigSessionManager(func() (api.ConfigEditor, error) {
		ed, err := cli.NewEditorWithStorage(store, configPath)
		if err != nil {
			return nil, fmt.Errorf("create editor: %w", err)
		}
		return ed, nil
	})

	authenticator := buildUserAuthenticator(users)

	// Generate OpenAPI spec lazily so it captures all plugin commands
	// (plugins may still be registering during startup).
	var (
		specOnce sync.Once
		specData []byte
	)
	lazySpec := func() []byte {
		specOnce.Do(func() {
			cmds := engine.ListCommands("")
			var err error
			specData, err = api.OpenAPISchema(cmds)
			if err != nil {
				fmt.Fprintf(os.Stderr, "warning: API OpenAPI generation failed: %v\n", err)
				specData = []byte(`{"openapi":"3.1.0","info":{"title":"Ze API","version":"1.0.0"},"paths":{}}`)
			}
		})
		return specData
	}

	var servers apiServers

	// REST now accepts a slice of listen addresses; every YANG list entry
	// becomes a bound listener on the same *http.Server. The slice is
	// guaranteed non-empty when RESTOn is true (ExtractAPIConfig synthesizes
	// a default entry when no YANG list entry is present).
	if cfg.RESTOn && len(cfg.REST) > 0 {
		addrs := make([]string, 0, len(cfg.REST))
		for _, ep := range cfg.REST {
			addrs = append(addrs, ep.Listen())
		}
		srv, restErr := rest.NewRESTServer(rest.RESTConfig{
			ListenAddrs:   addrs,
			Token:         cfg.Token,
			Authenticator: authenticator,
			CORSOrigin:    cfg.RESTCORSOrigin,
		}, engine, sessions, lazySpec)
		if restErr != nil {
			fmt.Fprintf(os.Stderr, "warning: REST API disabled: %v\n", restErr)
		} else {
			go serveREST(srv)
			servers.rest = srv
			for _, addr := range addrs {
				fmt.Fprintf(os.Stderr, "REST API server starting on http://%s/\n", addr)
			}
		}
	}

	if cfg.GRPCOn && len(cfg.GRPC) > 0 {
		addrs := make([]string, 0, len(cfg.GRPC))
		for _, ep := range cfg.GRPC {
			addrs = append(addrs, ep.Listen())
		}
		srv, grpcErr := apigrpc.NewGRPCServer(apigrpc.GRPCConfig{
			ListenAddrs:   addrs,
			Token:         cfg.Token,
			Authenticator: authenticator,
			TLSCert:       cfg.GRPCTLSCert,
			TLSKey:        cfg.GRPCTLSKey,
		}, engine, sessions)
		if grpcErr != nil {
			fmt.Fprintf(os.Stderr, "warning: gRPC API disabled: %v\n", grpcErr)
		} else {
			go serveGRPC(srv)
			servers.grpc = srv
			for _, addr := range addrs {
				fmt.Fprintf(os.Stderr, "gRPC API server starting on %s\n", addr)
			}
		}
	}

	return &servers
}

// Shutdown stops all running API servers.
func (s *apiServers) Shutdown(ctx context.Context) {
	if s == nil {
		return
	}
	if s.rest != nil {
		if err := s.rest.Shutdown(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "warning: REST API shutdown: %v\n", err)
		}
	}
	if s.grpc != nil {
		s.grpc.Stop()
	}
}

// buildUserAuthenticator returns an Authenticator that parses
// "Bearer <username>:<password>" and validates against the user list.
// Returns nil if no users are configured (caller falls back to Token or no-auth).
func buildUserAuthenticator(users []authz.UserConfig) func(string) (string, bool) {
	if len(users) == 0 {
		return nil
	}
	auth := &authz.LocalAuthenticator{Users: users}
	return func(header string) (string, bool) {
		raw, ok := strings.CutPrefix(header, "Bearer ")
		if !ok {
			return "", false
		}
		username, password, ok := strings.Cut(raw, ":")
		if !ok || username == "" {
			return "", false
		}
		result, err := auth.Authenticate(username, password)
		if err != nil || !result.Authenticated {
			return "", false
		}
		return username, true
	}
}

// buildAPIEngine creates the shared API engine wired to the plugin server.
func buildAPIEngine(server *pluginserver.Server) *api.APIEngine {
	exec := apiExecutor(server)
	cmds := apiCommandLister(server)
	auth := func(_, _ string) bool {
		// Bearer token auth handled at transport level.
		return true
	}
	return api.NewAPIEngine(exec, cmds, auth, nil)
}

// apiExecutor creates an Executor from the plugin server's dispatcher.
func apiExecutor(s *pluginserver.Server) api.Executor {
	return func(username, command string) (string, error) {
		d := s.Dispatcher()
		if d == nil {
			return "", fmt.Errorf("server not ready")
		}
		ctx := &pluginserver.CommandContext{Server: s, Username: username}
		resp, err := d.Dispatch(ctx, command)
		if err != nil {
			return "", err
		}
		if resp == nil {
			return "", nil
		}
		data, ok := resp.Data.(string)
		if !ok {
			b, jsonErr := json.Marshal(resp.Data)
			if jsonErr != nil {
				return "", fmt.Errorf("marshal response: %w", jsonErr)
			}
			return string(b), nil
		}
		return data, nil
	}
}

// apiCommandLister creates a CommandSource by converting MCP's command list.
// Reuses serverCommandLister (mcp.go) to avoid duplicating dispatcher traversal.
func apiCommandLister(s *pluginserver.Server) api.CommandSource {
	mcpLister := serverCommandLister(s)

	return func() []api.CommandMeta {
		mcpCmds := mcpLister()
		if mcpCmds == nil {
			return nil
		}
		infos := make([]api.CommandMeta, len(mcpCmds))
		for i, cmd := range mcpCmds {
			infos[i] = api.CommandMeta{
				Name:        cmd.Name,
				Description: cmd.Help,
				ReadOnly:    cmd.ReadOnly,
			}
			for _, p := range cmd.Params {
				infos[i].Params = append(infos[i].Params, api.ParamMeta{
					Name:        p.Name,
					Type:        p.Type,
					Description: p.Description,
					Required:    p.Required,
				})
			}
		}
		return infos
	}
}

// serveREST runs the REST API server. Started once as a lifecycle goroutine.
func serveREST(srv *rest.RESTServer) {
	if err := srv.ListenAndServe(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "warning: REST API server: %v\n", err)
	}
}

// serveGRPC runs the gRPC API server. Started once as a lifecycle goroutine.
// GRPCServer.Serve binds every address configured on the server and blocks
// until Stop is called.
func serveGRPC(srv *apigrpc.GRPCServer) {
	if err := srv.Serve(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "warning: gRPC API server: %v\n", err)
	}
}
