// Design: docs/architecture/api/architecture.md -- API server startup
// Overview: main.go -- hub CLI entry point

package hub

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/ze-software/ze/internal/component/api"
	"github.com/ze-software/ze/internal/component/authz"
	"github.com/ze-software/ze/internal/component/cli"
	zeconfig "github.com/ze-software/ze/internal/component/config"
	zeconfigcmd "github.com/ze-software/ze/internal/component/config/cli"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

var errServerNotReady = errors.New("server not ready")

// API non-loopback classification is now handled by the shared boot-time guard
// (mgmt_guard.go: listenAddrIsNonLoopback + checkMgmtListeners), which the API
// server's listener declaration in runYANGConfig feeds. The former
// apiHasNonLoopback helper was folded into that single classifier so exactly
// one non-loopback rule exists across every management surface.

func configValidationHook(configPath string) api.ConfigValidationHook {
	return func(previous, candidate string) error {
		if err := zeconfigcmd.ValidateContent(candidate, configPath); err != nil {
			return err
		}
		return zeconfig.VerifyPluginConfigContentTransition(previous, candidate)
	}
}

// buildAPIShared builds the API engine, config session manager, and
// authenticator shared by the REST and gRPC transports. It uses only the parent
// internal/component/api package and other always-on helpers, so it stays
// always-on; the gated transport builders (service_rest.go / service_grpc.go)
// construct their own server from the returned shared state. Starts the session
// cleanup goroutine (tied to the server context). Called by the hub only when at
// least one transport is compiled in.
func buildAPIShared(in *apiBuildInputs) *apiShared {
	engine := buildAPIEngine(in.Server)
	sessions := api.NewConfigSessionManager(func() (api.ConfigEditor, error) {
		ed, err := cli.NewEditorWithStorage(in.Store, in.ConfigPath)
		if err != nil {
			return nil, fmt.Errorf("create editor: %w", err)
		}
		return ed, nil
	})
	sessions.SetValidationHook(configValidationHook(in.ConfigPath))
	sessions.SetCommitHook(in.ReloadHook)
	go sessions.RunCleanup(in.Server.Context())
	return &apiShared{
		Engine:        engine,
		Sessions:      sessions,
		Authenticator: buildUserAuthenticator(in.Users),
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
		result, err := auth.Authenticate(authz.AuthRequest{
			Username: username,
			Password: password,
		})
		if err != nil || !result.Authenticated {
			return "", false
		}
		return username, true
	}
}

// buildAPIEngine creates the shared API engine wired to the plugin server.
// The executor is the unified serverDispatcher: the API engine consumes the
// same command dispatcher every surface shares, and the REST/gRPC transports
// set the audit surface per request via CallerIdentity.Surface (the "" fixed
// default here is never used because the transports always set it).
func buildAPIEngine(server *pluginserver.Server) *api.APIEngine {
	exec := serverDispatcher(server, "")
	cmds := apiCommandLister(server)
	auth := func(_, _ string) bool {
		// Bearer token auth handled at transport level.
		return true
	}
	return api.NewAPIEngine(exec, cmds, auth, apiStreamSource(server))
}

const (
	apiStreamBuffer     = 64
	apiStreamMaxLineLen = 1 << 20 // 1 MiB
)

// apiStreamSource adapts pluginserver streaming handlers to the API engine.
func apiStreamSource(s *pluginserver.Server) api.StreamSource {
	return func(ctx context.Context, caller api.CallerIdentity, command string) (<-chan string, func(), error) {
		if s == nil {
			return nil, nil, errServerNotReady
		}
		d := s.Dispatcher()
		if d == nil {
			return nil, nil, errServerNotReady
		}

		handler, args := pluginserver.GetStreamingHandlerForCommand(command)
		if handler == nil {
			return nil, nil, fmt.Errorf("unknown streaming command: %q", command)
		}

		cmdCtx := &pluginserver.CommandContext{
			Server:         s,
			RequestContext: ctx,
			Username:       caller.Username,
			RemoteAddr:     caller.RemoteAddr,
			Surface:        caller.Surface,
		}
		// Streaming commands are monitor-style read-only commands today.
		// If write-capable streams are added, the registry must carry metadata.
		if !d.IsAuthorized(cmdCtx, command, true) {
			return nil, nil, api.ErrUnauthorized
		}

		streamCtx, cancel := context.WithCancel(ctx)
		ch := make(chan string, apiStreamBuffer)
		writer := newAPIStreamLineWriter(streamCtx, ch)

		go func() {
			defer close(ch)
			defer d.BeginAccounting(cmdCtx, command)()
			defer func() {
				if r := recover(); r != nil {
					writer.close(fmt.Errorf("streaming handler panic: %v", r))
				}
			}()
			err := handler(streamCtx, s, writer, caller.Username, args)
			writer.close(err)
		}()

		select {
		case <-writer.ready():
			if err := writer.startError(); err != nil {
				cancel()
				return nil, nil, err
			}
			return ch, cancel, nil
		case <-ctx.Done():
			cancel()
			return nil, nil, ctx.Err()
		}
	}
}

type apiStreamLineWriter struct {
	ctx context.Context
	ch  chan<- string

	readyCh chan struct{}
	mu      sync.Mutex
	started bool
	err     error

	buf []byte
}

func newAPIStreamLineWriter(ctx context.Context, ch chan<- string) *apiStreamLineWriter {
	return &apiStreamLineWriter{
		ctx:     ctx,
		ch:      ch,
		readyCh: make(chan struct{}),
	}
}

func (w *apiStreamLineWriter) Write(p []byte) (int, error) {
	total := len(p)
	for len(p) > 0 {
		idx := bytes.IndexByte(p, '\n')
		if idx < 0 {
			w.buf = append(w.buf, p...)
			if len(w.buf) > apiStreamMaxLineLen {
				w.buf = nil
				return total, fmt.Errorf("streaming line exceeds %d bytes", apiStreamMaxLineLen)
			}
			return total, nil
		}

		// Build line string without merging into w.buf so a send
		// failure leaves w.buf intact for correct retry semantics.
		var line string
		if len(w.buf) > 0 {
			line = string(w.buf) + string(p[:idx])
		} else {
			line = string(p[:idx])
		}
		if n := len(line); n > 0 && line[n-1] == '\r' {
			line = line[:n-1]
		}
		if err := w.send(line); err != nil {
			return total - len(p), err
		}
		w.buf = nil
		p = p[idx+1:]
	}
	return total, nil
}

func (w *apiStreamLineWriter) ready() <-chan struct{} {
	return w.readyCh
}

func (w *apiStreamLineWriter) startError() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.err
}

func (w *apiStreamLineWriter) close(err error) {
	if err == nil && len(w.buf) > 0 {
		err = w.send(string(w.buf))
		w.buf = nil
	}
	w.markReady(err)
}

func (w *apiStreamLineWriter) send(line string) error {
	select {
	case w.ch <- line:
		w.markReady(nil)
		return nil
	case <-w.ctx.Done():
		return w.ctx.Err()
	}
}

func (w *apiStreamLineWriter) markReady(err error) {
	var closeReady bool
	w.mu.Lock()
	if !w.started {
		w.started = true
		w.err = err
		closeReady = true
	}
	w.mu.Unlock()
	if closeReady {
		close(w.readyCh)
	}
}

// apiCommandLister creates a CommandSource from the neutral, always-on command
// metadata source (command_meta.go). The same source backs the MCP command
// lister (service_mcp.go) when ze_mcp is compiled in; sharing the neutral type
// -- not a zemcp type -- is what lets the API command lister stay always-on
// while MCP is compiled out.
func apiCommandLister(s *pluginserver.Server) api.CommandSource {
	metaSource := commandMetaSource(s)

	return func() []api.CommandMeta {
		cmds := metaSource()
		if cmds == nil {
			return nil
		}
		infos := make([]api.CommandMeta, len(cmds))
		for i, cmd := range cmds {
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
