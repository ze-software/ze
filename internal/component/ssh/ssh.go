// Design: docs/architecture/system-architecture.md -- SSH server subsystem
// Detail: session.go -- per-session unified CLI model creation

package ssh

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/wish/v2"
	"charm.land/wish/v2/activeterm"
	"charm.land/wish/v2/bubbletea"
	"github.com/charmbracelet/ssh"

	"codeberg.org/thomas-mangin/ze/internal/component/authz"
	"codeberg.org/thomas-mangin/ze/internal/component/cli/contract"
	"codeberg.org/thomas-mangin/ze/internal/component/config/storage"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
	"codeberg.org/thomas-mangin/ze/internal/core/paths"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

// Compile-time interface check.
var _ ze.Subsystem = (*Server)(nil)

// Default configuration values.
const (
	defaultListen      = "127.0.0.1:2222"
	defaultMaxSessions = 8
	defaultIdleTimeout = 600 // seconds
	defaultHostKeyFile = "ssh_host_ed25519_key"
)

// CommandExecutor executes an operational command and returns the output.
// The SSH server calls this directly — no socket indirection needed
// since the server runs as a goroutine inside the daemon process.
type CommandExecutor func(input string) (string, error)

// CommandExecutorFactory creates a per-session CommandExecutor.
// The username is the authenticated SSH user; remoteAddr is the SSH client's
// IP:port. The returned executor can use both for authorization and accounting.
type CommandExecutorFactory func(username, remoteAddr string) CommandExecutor

// StreamingExecutor executes a streaming command, writing output line-by-line
// to the writer until the context is canceled or a write error occurs.
// Used by the monitor command to stream live BGP events over SSH.
type StreamingExecutor func(ctx context.Context, w io.Writer, args []string) error

// StreamingExecutorFactory creates a StreamingExecutor.
// Called when execMiddleware detects a streaming command (e.g., "bgp monitor").
type StreamingExecutorFactory func(username string) StreamingExecutor

// PluginProtocolFunc handles a "plugin protocol" SSH session by running
// the 5-stage plugin handshake and runtime command loop over the SSH channel.
// The SSH channel provides bidirectional io.ReadCloser/io.WriteCloser transport.
// Blocks until the session ends. Called by execMiddleware.
type PluginProtocolFunc func(ctx context.Context, reader io.ReadCloser, writer io.WriteCloser) error

// Config holds SSH server configuration parsed from the config file.
type Config struct {
	Listen        string
	ListenAddrs   []string // all listen addresses (first == Listen)
	HostKeyPath   string
	ConfigDir     string          // directory of the config file; used for host-key default
	ConfigPath    string          // path to config file; used by SSH sessions for concurrent editing
	Storage       storage.Storage // when set, host key is read from/stored to blob
	IdleTimeout   uint32
	MaxSessions   int
	Users         []authz.UserConfig
	Authenticator authz.Authenticator // pluggable auth backend (nil = use Users with bcrypt)
	Executor      CommandExecutor     // injected by daemon, not from config
}

// ShutdownFunc is called when the SSH server receives a "stop" exec command.
type ShutdownFunc func()

// RestartFunc is called when the SSH server receives a "restart" exec command.
// It writes the GR marker to zefs, then shuts down the daemon.
type RestartFunc func()

// RebootFunc is called when the SSH server receives a "reboot" exec command.
// It initiates graceful shutdown followed by an OS-level system reboot.
type RebootFunc func()

// Server is the SSH server subsystem.
// It serves the config editor over SSH with password authentication.
// Exec commands (non-interactive) are dispatched through the executor.
type Server struct {
	config                   Config
	mu                       sync.Mutex // protects wish field, executorFactory, shutdownFunc, and loginWarningsFunc
	wish                     *ssh.Server
	listener                 net.Listener   // bound listener (for address resolution)
	extraListeners           []net.Listener // additional listeners for multi-address binding
	logger                   *slog.Logger
	activeSessions           atomic.Int32
	executorFactory          CommandExecutorFactory       // set after reactor starts; creates per-session executors
	streamingExecutorFactory StreamingExecutorFactory     // set after reactor starts; for monitor commands
	pluginProtocolFunc       PluginProtocolFunc           // set after reactor starts; for plugin debug shell
	monitorFactory           contract.MonitorFactory      // set after reactor starts; for TUI monitor mode
	sessionModelFactory      contract.SessionModelFactory // set by hub; creates TUI model per session
	shutdownFunc             ShutdownFunc                 // set by daemon; called on "stop" exec command
	restartFunc              RestartFunc                  // set by daemon; called on "restart" exec command
	rebootFunc               RebootFunc                   // set by daemon; called on "reboot" exec command
	loginWarningsFunc        LoginWarningsFunc            // set by daemon; returns login warnings for SSH sessions
}

// NewServer creates a new SSH server with the given configuration.
// It applies defaults for unset fields but does not start listening.
// If HostKeyPath is empty, it defaults to ssh_host_ed25519_key in ConfigDir.
// If ConfigDir is also empty, it resolves from the binary location via paths.DefaultConfigDir().
// Wish auto-generates the key file if it does not exist.
func NewServer(cfg Config) (*Server, error) {
	if cfg.HostKeyPath == "" {
		dir := cfg.ConfigDir
		if dir == "" {
			dir = paths.DefaultConfigDir()
		}
		if dir == "" {
			return nil, fmt.Errorf("host-key path cannot be resolved: set host-key in config or run from a standard install location")
		}
		cfg.HostKeyPath = filepath.Join(dir, defaultHostKeyFile)
	}
	if cfg.Listen == "" {
		cfg.Listen = defaultListen
	}
	if cfg.MaxSessions <= 0 {
		cfg.MaxSessions = defaultMaxSessions
	}
	if cfg.IdleTimeout == 0 {
		cfg.IdleTimeout = defaultIdleTimeout
	}

	return &Server{
		config: cfg,
		logger: slog.Default().With("subsystem", "ssh"),
	}, nil
}

// Name returns the subsystem identifier.
func (s *Server) Name() string {
	return "ssh"
}

// Address returns the listen address. If the server has bound to a port (e.g., port 0),
// it returns the actual bound address. Otherwise returns the configured address.
func (s *Server) Address() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.listener != nil {
		return s.listener.Addr().String()
	}
	return s.config.Listen
}

// MaxSessions returns the configured maximum concurrent sessions.
func (s *Server) MaxSessions() int {
	return s.config.MaxSessions
}

// Users returns the configured user list.
func (s *Server) Users() []authz.UserConfig {
	return s.config.Users
}

// ActiveSessions returns the current number of active SSH sessions.
func (s *Server) ActiveSessions() int32 {
	return s.activeSessions.Load()
}

// SetExecutorFactory sets the per-session command executor factory.
// Called after the reactor starts and the Dispatcher is available.
// New sessions created after this call will use the factory to create
// per-session executors with the authenticated username.
func (s *Server) SetExecutorFactory(f CommandExecutorFactory) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.executorFactory = f
}

// SetStreamingExecutorFactory sets the factory for streaming commands (e.g., monitor).
// Called after the reactor starts, alongside SetExecutorFactory.
func (s *Server) SetStreamingExecutorFactory(f StreamingExecutorFactory) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.streamingExecutorFactory = f
}

// SetPluginProtocolFunc sets the handler for "plugin protocol" SSH sessions.
// Called after the reactor starts to enable plugin debug shell.
func (s *Server) SetPluginProtocolFunc(f PluginProtocolFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pluginProtocolFunc = f
}

// SetMonitorFactory sets the factory for TUI monitor sessions.
// Called after the reactor starts to wire streaming monitor into interactive SSH sessions.
func (s *Server) SetMonitorFactory(f contract.MonitorFactory) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.monitorFactory = f
}

// SetSessionModelFactory sets the factory that creates TUI models for SSH sessions.
// Called by the hub before starting the SSH server.
func (s *Server) SetSessionModelFactory(f contract.SessionModelFactory) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessionModelFactory = f
}

// SetShutdownFunc sets the callback for "stop" exec commands.
// Called by the daemon to wire graceful shutdown via SSH.
func (s *Server) SetShutdownFunc(f ShutdownFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.shutdownFunc = f
}

// SetRestartFunc sets the callback for "restart" exec commands.
// Called by the daemon to wire graceful restart (GR marker + shutdown) via SSH.
func (s *Server) SetRestartFunc(f RestartFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.restartFunc = f
}

// SetRebootFunc sets the callback for "reboot" exec commands.
// Called by the daemon to wire graceful shutdown + OS reboot via SSH.
func (s *Server) SetRebootFunc(f RebootFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rebootFunc = f
}

// SetLoginWarnings sets the function that returns login warnings for new SSH sessions.
// Called by the daemon after the reactor starts, alongside SetExecutorFactory.
// The function is called once per session in createSessionModel.
func (s *Server) SetLoginWarnings(f LoginWarningsFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.loginWarningsFunc = f
}

// ExecutorForUser returns a CommandExecutor for the given username.
// Returns nil if no executor factory is set.
func (s *Server) ExecutorForUser(username string) CommandExecutor {
	s.mu.Lock()
	factory := s.executorFactory
	s.mu.Unlock()
	if factory == nil {
		if s.config.Executor != nil {
			return s.config.Executor
		}
		return nil
	}
	return factory(username, "")
}

// MonitorFactoryFunc returns the monitor factory, or nil if not set.
func (s *Server) MonitorFactoryFunc() contract.MonitorFactory {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.monitorFactory
}

// ShutdownFunc returns the shutdown callback, or nil if not set.
func (s *Server) ShutdownFunc() ShutdownFunc {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.shutdownFunc
}

// RestartFunc returns the restart callback, or nil if not set.
func (s *Server) RestartFunc() RestartFunc {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.restartFunc
}

// LoginWarningsFunc returns the login warnings function, or nil if not set.
func (s *Server) LoginWarningsFunc() LoginWarningsFunc {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loginWarningsFunc
}

// Start launches the SSH server. It implements ze.Subsystem.
// The listener is created synchronously so bind failures are reported immediately.
func (s *Server) Start(ctx context.Context, _ ze.EventBus, _ ze.ConfigProvider) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.wish != nil {
		return fmt.Errorf("SSH server already started")
	}

	// Resolve host key: from storage (blob mode) or filesystem path.
	hostKeyOpt, err := s.resolveHostKeyOption()
	if err != nil {
		return fmt.Errorf("resolve host key: %w", err)
	}

	authenticator := s.config.Authenticator
	if authenticator == nil {
		// No explicit authenticator: wrap local users as a LocalAuthenticator.
		authenticator = &authz.LocalAuthenticator{Users: s.config.Users}
	}

	// Always register a password auth handler.
	// When no users are configured, reject all attempts (never allow NoClientAuth).
	opts := []ssh.Option{
		hostKeyOpt,
		wish.WithMaxTimeout(time.Duration(s.config.IdleTimeout) * time.Second),
		// Wish composes middleware from first to last: last = outermost = runs first.
		// Order: maxSessions → exec → bubbletea → activeterm (innermost).
		// Exec middleware intercepts non-interactive sessions before they
		// reach bubbletea/activeterm (which require a PTY).
		wish.WithMiddleware(
			activeterm.Middleware(),
			bubbletea.Middleware(s.teaHandler),
			s.execMiddleware(),
			s.maxSessionsMiddleware(),
		),
		wish.WithPasswordAuth(func(ctx ssh.Context, pass string) bool {
			username := ctx.User()
			remote := ctx.RemoteAddr().String()

			result, err := authenticator.Authenticate(username, pass)
			if err == nil && result.Authenticated {
				s.logger.Info("SSH auth success", "username", username, "remote", remote, "source", result.Source)
				return true
			}
			s.logger.Warn("SSH auth failure", "username", username, "remote", remote)
			return false
		}),
	}

	srv, err := wish.NewServer(opts...)
	if err != nil {
		return fmt.Errorf("create SSH server: %w", err)
	}

	// Bind synchronously so Start() returns an error if the port is unavailable.
	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", s.config.Listen)
	if err != nil {
		return fmt.Errorf("bind SSH server: %w", err)
	}

	s.wish = srv
	s.listener = ln

	// Serve in a goroutine (lifecycle goroutine, not per-event).
	go func() {
		s.logger.Info("SSH server listening", "address", ln.Addr().String())
		if err := srv.Serve(ln); err != nil {
			if !errors.Is(err, ssh.ErrServerClosed) {
				s.logger.Error("SSH server error", "error", err)
			}
		}
	}()

	// Bind additional listen addresses (if configured).
	// The first address is already bound above via config.Listen.
	if len(s.config.ListenAddrs) > 1 {
		for _, addr := range s.config.ListenAddrs[1:] {
			extraLn, listenErr := lc.Listen(ctx, "tcp", addr)
			if listenErr != nil {
				s.logger.Error("SSH extra listener failed to bind", "address", addr, "error", listenErr)
				continue
			}
			s.extraListeners = append(s.extraListeners, extraLn)

			// Serve each extra listener in its own lifecycle goroutine.
			go func() {
				s.logger.Info("SSH server listening", "address", extraLn.Addr().String())
				if err := srv.Serve(extraLn); err != nil {
					if !errors.Is(err, ssh.ErrServerClosed) {
						s.logger.Error("SSH server error", "address", extraLn.Addr().String(), "error", err)
					}
				}
			}()
		}
	}

	return nil
}

// Stop gracefully shuts down the SSH server. It implements ze.Subsystem.
func (s *Server) Stop(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.wish == nil {
		return nil
	}
	s.logger.Info("SSH server stopping")
	err := s.wish.Shutdown(ctx)
	s.wish = nil
	s.listener = nil
	s.extraListeners = nil
	return err
}

// Reload applies configuration changes. It implements ze.Subsystem.
func (s *Server) Reload(_ context.Context, _ ze.ConfigProvider) error {
	// SSH server config changes require restart; no hot-reload for v1.
	return nil
}

// resolveHostKeyOption returns the Wish host key option.
// The key is always read from (or generated into) the storage layer and
// served from memory via WithHostKeyPEM. Never uses WithHostKeyPath, which
// would auto-generate both .key and .pub files on the physical filesystem,
// bypassing the zefs blob store.
func (s *Server) resolveHostKeyOption() (ssh.Option, error) {
	store := s.config.Storage
	keyPath := s.config.HostKeyPath

	if store != nil && store.Exists(keyPath) {
		data, err := store.ReadFile(keyPath)
		if err != nil {
			return nil, fmt.Errorf("read host key from storage: %w", err)
		}
		return wish.WithHostKeyPEM(data), nil
	}

	// Key not found in storage -- generate a new Ed25519 key.
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate host key: %w", err)
	}
	pkcs8Bytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return nil, fmt.Errorf("marshal host key: %w", err)
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8Bytes})

	if store != nil {
		if err := store.WriteFile(keyPath, pemBytes, 0o600); err != nil {
			return nil, fmt.Errorf("store host key: %w", err)
		}
	}
	s.logger.Info("generated SSH host key", "path", keyPath)

	return wish.WithHostKeyPEM(pemBytes), nil
}

// maxSessionsMiddleware returns middleware that enforces the max sessions limit.
func (s *Server) maxSessionsMiddleware() wish.Middleware {
	return func(next ssh.Handler) ssh.Handler {
		return func(sess ssh.Session) {
			current := s.activeSessions.Add(1)
			defer s.activeSessions.Add(-1)

			if int(current) > s.config.MaxSessions {
				fmt.Fprintf(sess, "maximum sessions reached (%d), try again later\n", s.config.MaxSessions) //nolint:errcheck // best-effort message
				return
			}
			next(sess)
		}
	}
}

// execMiddleware handles non-interactive SSH exec commands (e.g., "ssh daemon stop").
// If the session has a command, it dispatches through the executor.
// Interactive sessions (no command) pass through to the next middleware (BubbleTea).
func (s *Server) execMiddleware() wish.Middleware {
	return func(next ssh.Handler) ssh.Handler {
		return func(sess ssh.Session) {
			cmd := sess.Command()
			if len(cmd) == 0 {
				next(sess) // interactive session
				return
			}

			input := strings.Join(cmd, " ")
			s.logger.Info("SSH exec command", "user", sess.User(), "command", input, "remote", sess.RemoteAddr().String())

			// Handle lifecycle commands directly.
			// Note: stop/restart/reboot bypass RPC authorization by design -- any
			// authenticated SSH user can shut down or reboot. Restrict via SSH user config.
			lcInput := strings.ToLower(strings.TrimSpace(input))
			if lcInput == "stop" {
				s.mu.Lock()
				fn := s.shutdownFunc
				s.mu.Unlock()
				if fn != nil {
					fmt.Fprintln(sess, "stopping daemon") //nolint:errcheck // best-effort response
					sess.Exit(0)                          //nolint:errcheck // best-effort exit status
					fn()
				} else {
					fmt.Fprintln(sess.Stderr(), "error: shutdown not available") //nolint:errcheck // best-effort
					sess.Exit(1)                                                 //nolint:errcheck // best-effort
				}
				return
			}
			if lcInput == "restart" {
				s.mu.Lock()
				fn := s.restartFunc
				s.mu.Unlock()
				if fn != nil {
					fmt.Fprintln(sess, "restarting daemon") //nolint:errcheck // best-effort response
					sess.Exit(0)                            //nolint:errcheck // best-effort exit status
					fn()
				} else {
					fmt.Fprintln(sess.Stderr(), "error: restart not available") //nolint:errcheck // best-effort
					sess.Exit(1)                                                //nolint:errcheck // best-effort
				}
				return
			}
			if lcInput == "reboot" {
				s.mu.Lock()
				fn := s.rebootFunc
				s.mu.Unlock()
				if fn != nil {
					fmt.Fprintln(sess, "rebooting system") //nolint:errcheck // best-effort response
					sess.Exit(0)                           //nolint:errcheck // best-effort exit status
					fn()
				} else {
					fmt.Fprintln(sess.Stderr(), "error: reboot not available") //nolint:errcheck // best-effort
					sess.Exit(1)                                               //nolint:errcheck // best-effort
				}
				return
			}

			s.mu.Lock()
			factory := s.executorFactory
			streamFactory := s.streamingExecutorFactory
			pluginProto := s.pluginProtocolFunc
			s.mu.Unlock()

			// Handle plugin protocol sessions (debug shell).
			// The SSH channel becomes bidirectional plugin transport.
			if lcInput == "plugin protocol" {
				if pluginProto == nil {
					fmt.Fprintln(sess.Stderr(), "error: plugin protocol not available (daemon still starting)") //nolint:errcheck // best-effort
					sess.Exit(1)                                                                                //nolint:errcheck // best-effort
					return
				}
				s.logger.Info("SSH plugin protocol session", "user", sess.User(), "remote", sess.RemoteAddr().String())
				if err := pluginProto(sess.Context(), sess, sess); err != nil {
					fmt.Fprintf(sess.Stderr(), "error: %v\n", err) //nolint:errcheck // best-effort
					sess.Exit(1)                                   //nolint:errcheck // best-effort
				}
				return
			}

			// Check for streaming commands (e.g., "monitor event ...").
			// Pass the full input to the executor; the executor does handler lookup.
			if streamFactory != nil && pluginserver.IsStreamingCommand(input) {
				streamExec := streamFactory(sess.User())
				if err := streamExec(sess.Context(), sess, []string{input}); err != nil {
					fmt.Fprintf(sess.Stderr(), "error: %v\n", err) //nolint:errcheck // best-effort
					sess.Exit(1)                                   //nolint:errcheck // best-effort
				}
				return
			}

			if streamFactory == nil && pluginserver.IsStreamingCommand(input) {
				fmt.Fprintln(sess.Stderr(), "error: streaming not available (daemon still starting)") //nolint:errcheck // best-effort
				sess.Exit(1)                                                                          //nolint:errcheck // best-effort
				return
			}

			// Dispatch through command executor.
			if factory == nil {
				fmt.Fprintln(sess.Stderr(), "error: command executor not ready") //nolint:errcheck // best-effort
				sess.Exit(1)                                                     //nolint:errcheck // best-effort
				return
			}

			executor := factory(sess.User(), sess.RemoteAddr().String())
			result, err := executor(input)
			if err != nil {
				fmt.Fprintf(sess.Stderr(), "error: %v\n", err) //nolint:errcheck // best-effort
				sess.Exit(1)                                   //nolint:errcheck // best-effort
				return
			}

			if result != "" {
				fmt.Fprintln(sess, result) //nolint:errcheck // best-effort
			}
		}
	}
}

// Streaming command detection and argument extraction are handled by
// pluginserver.IsStreamingCommand() and pluginserver.GetStreamingHandlerForCommand().
// These use a prefix-keyed registry instead of a hardcoded prefix constant.

// teaHandler creates a per-session Bubble Tea model using the session model factory.
// Each SSH session gets a command-mode model with an executor wired.
// If an executor factory is set, it creates a per-session executor with the
// authenticated username (for authorization context). Falls back to config.Executor.
func (s *Server) teaHandler(sess ssh.Session) (tea.Model, []tea.ProgramOption) {
	username := sess.User()
	model := s.createSessionModel(username)
	s.logger.Info("SSH session started", "user", username, "remote", sess.RemoteAddr().String())
	return model, []tea.ProgramOption{}
}
