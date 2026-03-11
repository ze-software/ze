// Design: (none -- new SSH server component)
// Detail: auth.go -- password authentication
// Detail: session.go -- per-session unified CLI model creation

package ssh

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/charmbracelet/wish/activeterm"
	"github.com/charmbracelet/wish/bubbletea"

	"codeberg.org/thomas-mangin/ze/internal/core/paths"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

// Compile-time interface check.
var _ ze.Subsystem = (*Server)(nil)

// Default configuration values.
const (
	defaultListen      = "0.0.0.0:2222"
	defaultMaxSessions = 8
	defaultIdleTimeout = 600 // seconds
	defaultHostKeyFile = "ssh_host_ed25519_key"
)

// CommandExecutor executes an operational command and returns the output.
// The SSH server calls this directly — no socket indirection needed
// since the server runs as a goroutine inside the daemon process.
type CommandExecutor func(input string) (string, error)

// CommandExecutorFactory creates a per-session CommandExecutor.
// The username is the authenticated SSH user; the returned executor
// can use it for authorization context.
type CommandExecutorFactory func(username string) CommandExecutor

// Config holds SSH server configuration parsed from the config file.
type Config struct {
	Listen      string
	HostKeyPath string
	ConfigDir   string // directory of the config file; used for host-key default
	IdleTimeout uint32
	MaxSessions int
	Users       []UserConfig
	Executor    CommandExecutor // injected by daemon, not from config
}

// Server is the SSH server subsystem.
// It serves the config editor over SSH with password authentication.
type Server struct {
	config          Config
	mu              sync.Mutex // protects wish field and executorFactory
	wish            *ssh.Server
	listener        net.Listener // bound listener (for address resolution)
	logger          *slog.Logger
	activeSessions  atomic.Int32
	executorFactory CommandExecutorFactory // set after reactor starts; creates per-session executors
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
func (s *Server) Users() []UserConfig {
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

// Start launches the SSH server. It implements ze.Subsystem.
// The listener is created synchronously so bind failures are reported immediately.
func (s *Server) Start(ctx context.Context, _ ze.Bus, _ ze.ConfigProvider) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.wish != nil {
		return fmt.Errorf("SSH server already started")
	}

	users := s.config.Users

	// Always register a password auth handler.
	// When no users are configured, reject all attempts (never allow NoClientAuth).
	opts := []ssh.Option{
		wish.WithHostKeyPath(s.config.HostKeyPath),
		wish.WithMaxTimeout(time.Duration(s.config.IdleTimeout) * time.Second),
		wish.WithMiddleware(
			s.maxSessionsMiddleware(),
			bubbletea.Middleware(s.teaHandler),
			activeterm.Middleware(),
		),
		wish.WithPasswordAuth(func(ctx ssh.Context, pass string) bool {
			ok := AuthenticateUser(users, ctx.User(), pass)
			if ok {
				s.logger.Info("SSH auth success", "username", ctx.User(), "remote", ctx.RemoteAddr().String())
			} else {
				s.logger.Warn("SSH auth failure", "username", ctx.User(), "remote", ctx.RemoteAddr().String())
			}
			return ok
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
	return err
}

// Reload applies configuration changes. It implements ze.Subsystem.
func (s *Server) Reload(_ context.Context, _ ze.ConfigProvider) error {
	// SSH server config changes require restart; no hot-reload for v1.
	return nil
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

// teaHandler creates a per-session Bubble Tea model using the unified cli.Model.
// Each SSH session gets a command-mode model with an executor wired.
// If an executor factory is set, it creates a per-session executor with the
// authenticated username (for authorization context). Falls back to config.Executor.
func (s *Server) teaHandler(sess ssh.Session) (tea.Model, []tea.ProgramOption) {
	username := sess.User()
	model := s.createSessionModel(username)
	s.logger.Info("SSH session started", "user", username, "remote", sess.RemoteAddr().String())
	return model, []tea.ProgramOption{tea.WithAltScreen()}
}
