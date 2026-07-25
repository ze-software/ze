// Design: docs/architecture/api/architecture.md -- gNMI transport for YANG-modeled config
// Related: capabilities.go -- Capabilities RPC handler
// Related: get.go -- Get RPC handler
// Related: set.go -- Set RPC handler
// Related: subscribe.go -- Subscribe RPC handler
// Related: path.go -- gNMI path translation
// Related: errors.go -- sentinel errors

package gnmi

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"fmt"
	"net"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	gpb "github.com/openconfig/gnmi/proto/gnmi"

	"github.com/ze-software/ze/internal/component/api"
	zeconfig "github.com/ze-software/ze/internal/component/config"
	yangloader "github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/slogutil"
)

var (
	logger = slogutil.Logger("gnmi")

	globalMu     sync.Mutex
	globalServer *Server
)

// RegisterGlobal stores a reference to the gNMI server for show command access.
func RegisterGlobal(s *Server) {
	globalMu.Lock()
	globalServer = s
	globalMu.Unlock()
}

// LookupServer returns the registered gNMI server, or nil if not running.
func LookupServer() *Server {
	globalMu.Lock()
	defer globalMu.Unlock()
	return globalServer
}

// Config holds gNMI server configuration.
type Config struct {
	ListenAddr string
	Token      string
	CertPEM    []byte
	KeyPEM     []byte
}

// Server implements the gNMI gRPC service.
type Server struct {
	gpb.UnimplementedGNMIServer

	config   Config
	tree     func() *zeconfig.Tree
	sessions *api.ConfigSessionManager
	loader   func() (*yangloader.Loader, error)
	notifier *ChangeNotifier
	srv      *grpc.Server
	listener net.Listener
	mu       sync.Mutex
	stopped  bool
	metrics  *gnmiMetrics

	modelsOnce sync.Once
	models     []*gpb.ModelData
}

// ServerStatus holds gNMI server state for the show command.
type ServerStatus struct {
	plugin.DataMarker
	Enabled       bool   `json:"enabled"`
	ListenAddress string `json:"listen-address"`
	TokenSet      bool   `json:"token-set"`
	TLSConfigured bool   `json:"tls-configured"`
	Subscribers   int    `json:"subscribers"`
}

// Status returns the current gNMI server state.
func (s *Server) Status() ServerStatus {
	st := ServerStatus{
		Enabled:       true,
		ListenAddress: s.Address(),
		TokenSet:      s.config.Token != "",
		TLSConfigured: len(s.config.CertPEM) > 0 && len(s.config.KeyPEM) > 0,
	}
	if s.notifier != nil {
		s.notifier.mu.RLock()
		st.Subscribers = len(s.notifier.clients)
		s.notifier.mu.RUnlock()
	}
	return st
}

// NewServer creates a gNMI server.
func NewServer(cfg Config, tree func() *zeconfig.Tree, sessions *api.ConfigSessionManager, loader func() (*yangloader.Loader, error), notifier *ChangeNotifier) *Server {
	return &Server{
		config:   cfg,
		tree:     tree,
		sessions: sessions,
		loader:   loader,
		notifier: notifier,
	}
}

// SetMetricsRegistry enables Prometheus instrumentation for gNMI RPCs.
// Must be called before Serve. Nil registry disables metrics.
func (s *Server) SetMetricsRegistry(reg metrics.Registry) {
	if reg != nil {
		s.metrics = initGNMIMetrics(reg)
	}
}

// Serve starts the gNMI gRPC server on the configured address.
func (s *Server) Serve(ctx context.Context) error {
	var opts []grpc.ServerOption

	if s.config.Token != "" {
		opts = append(opts,
			grpc.UnaryInterceptor(s.authUnaryInterceptor),
			grpc.StreamInterceptor(s.authStreamInterceptor),
		)
	}

	if len(s.config.CertPEM) > 0 && len(s.config.KeyPEM) > 0 {
		cert, err := tls.X509KeyPair(s.config.CertPEM, s.config.KeyPEM)
		if err != nil {
			return fmt.Errorf("gnmi: TLS key pair: %w", err)
		}
		opts = append(opts, grpc.Creds(credentials.NewTLS(&tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
		})))
	}

	s.srv = grpc.NewServer(opts...)
	gpb.RegisterGNMIServer(s.srv, s)

	var lc net.ListenConfig
	lis, err := lc.Listen(ctx, "tcp", s.config.ListenAddr)
	if err != nil {
		return fmt.Errorf("gnmi: listen %s: %w", s.config.ListenAddr, err)
	}
	s.mu.Lock()
	s.listener = lis
	s.mu.Unlock()

	logger.Info("gNMI server listening", "addr", lis.Addr().String())

	go func() {
		<-ctx.Done()
		s.Stop()
	}()

	if err := s.srv.Serve(lis); err != nil {
		s.mu.Lock()
		stopped := s.stopped
		s.mu.Unlock()
		if stopped {
			return nil
		}
		return fmt.Errorf("gnmi: serve: %w", err)
	}
	return nil
}

// Stop gracefully stops the gNMI server.
func (s *Server) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped {
		return
	}
	s.stopped = true
	if s.srv != nil {
		s.srv.GracefulStop()
	}
}

// Address returns the bound listener address, or empty if not serving.
func (s *Server) Address() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.listener == nil {
		return ""
	}
	return s.listener.Addr().String()
}

// authUnaryInterceptor validates bearer token on unary RPCs.
func (s *Server) authUnaryInterceptor(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	if err := s.checkAuth(ctx); err != nil {
		return nil, err
	}
	return handler(ctx, req)
}

// authStreamInterceptor validates bearer token on streaming RPCs.
func (s *Server) authStreamInterceptor(srv any, ss grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	if err := s.checkAuth(ss.Context()); err != nil {
		return err
	}
	return handler(srv, ss)
}

func (s *Server) recordError(rpc string, code codes.Code) {
	if s.metrics != nil {
		s.metrics.errorsTotal.With(rpc, code.String()).Inc()
	}
}

func (s *Server) checkAuth(ctx context.Context) error {
	if s.config.Token == "" {
		return nil
	}
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "missing metadata")
	}
	vals := md.Get("authorization")
	if len(vals) == 0 {
		return status.Error(codes.Unauthenticated, "missing authorization")
	}
	const prefix = "Bearer "
	if len(vals[0]) <= len(prefix) || vals[0][:len(prefix)] != prefix {
		return status.Error(codes.Unauthenticated, "invalid authorization format")
	}
	got := sha256.Sum256([]byte(vals[0][len(prefix):]))
	want := sha256.Sum256([]byte(s.config.Token))
	if subtle.ConstantTimeCompare(got[:], want[:]) != 1 {
		return status.Error(codes.Unauthenticated, "invalid token")
	}
	return nil
}
