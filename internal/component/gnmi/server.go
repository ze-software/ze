// Design: docs/architecture/api/architecture.md -- gNMI transport for YANG-modeled config
// Related: capabilities.go -- Capabilities RPC handler
// Related: get.go -- Get RPC handler
// Related: set.go -- Set RPC handler
// Related: subscribe.go -- Subscribe RPC handler
// Related: path.go -- gNMI path translation
// Related: errors.go -- sentinel errors
// Related: tls.go -- loadTLSCredentials, the TLS material Serve refuses to serve without

package gnmi

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"fmt"
	"net"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	gpb "github.com/openconfig/gnmi/proto/gnmi"

	"github.com/ze-software/ze/internal/component/api"
	zeconfig "github.com/ze-software/ze/internal/component/config"
	yangloader "github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/audit"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/opererr"
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
//
// Non-loopback listeners MUST configure both Token and TLS (CertFile and
// KeyFile). Loopback listeners (127.0.0.0/8, ::1) may omit TLS and Token; when
// Token is empty, the server is read-only and refuses Set with PermissionDenied.
// Naming either CertFile or KeyFile asks for TLS, and Serve then serves TLS or
// nothing: it never falls back to clear text (tls.go).
type Config struct {
	ListenAddr    string
	Token         string
	CertFile      string
	KeyFile       string
	AuditRecorder audit.Recorder
}

// tlsNamed reports whether the config names any TLS material, so the
// listener owes TLS. One named file is enough: a lone key is a broken TLS
// config, never a request for clear text.
func (c *Config) tlsNamed() bool {
	if c.CertFile != "" {
		return true
	}
	return c.KeyFile != ""
}

// checkGNMIListenAddr enforces defense-in-depth on non-loopback gNMI listeners
// with the one rule the boot guard, `ze config validate` and `ze doctor` apply
// (zeconfig.GNMIExposure): every non-loopback listener MUST gate requests with
// Token and encrypt the transport with TLS, so credentials and configuration
// never cross the network in clear text. Naming one TLS file counts as asking
// for TLS here: loadTLSCredentials then refuses the half pair with its own
// message, so Serve never falls back to clear text.
func checkGNMIListenAddr(cfg Config) error {
	refusals := zeconfig.GNMIExposure([]string{cfg.ListenAddr}, cfg.Token != "", cfg.tlsNamed()).Refusals()
	if len(refusals) == 0 {
		return nil
	}
	errs := make([]error, 0, len(refusals))
	for _, r := range refusals {
		errs = append(errs, r)
	}
	return errors.Join(errs...)
}

// Server implements the gNMI gRPC service.
type Server struct {
	gpb.UnimplementedGNMIServer

	config        Config
	tree          func() *zeconfig.Tree
	sessions      *api.ConfigSessionManager
	loader        func() (*yangloader.Resolved, error)
	notifier      *ChangeNotifier
	auditRecorder audit.Recorder
	srv           *grpc.Server
	listener      net.Listener
	mu            sync.Mutex
	stopped       bool
	metrics       *gnmiMetrics

	modelsOnce sync.Once
	models     []*gpb.ModelData

	schemaOnce sync.Once
	schema     *zeconfig.Schema
	schemaErr  error
}

// configSchema returns the config schema the path translation reads, built
// once from the registered YANG. Safe for concurrent use.
//
// Built once for the server's life, as the model list is (modelsOnce): both
// are correct because the daemon builds its gNMI server after it published
// the modules of its external plugins. NewServer records the server with
// yang.KeepsModuleSet, so a publish after it is refused and neither can
// answer from the set before the publish.
func (s *Server) configSchema() (*zeconfig.Schema, error) {
	s.schemaOnce.Do(func() {
		s.schema, s.schemaErr = zeconfig.YANGSchema()
	})
	return s.schema, s.schemaErr
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
		TLSConfigured: s.config.tlsNamed(), // Serve refuses to listen on material it cannot load
	}
	if s.notifier != nil {
		s.notifier.mu.RLock()
		st.Subscribers = len(s.notifier.clients)
		s.notifier.mu.RUnlock()
	}
	return st
}

// moduleSetKeeperName names, in the refusal of a publish that comes too late
// (yang.PublishExternalModules), what a Server keeps from the module set.
const moduleSetKeeperName = "the gNMI server's config schema and model list"

// NewServer creates a gNMI server.
//
// The server builds its config schema and its model list once, on first use,
// and keeps them. In a process that publishes the modules of external plugins
// (yang.PublishExternalModules), it MUST be called after that publish.
func NewServer(cfg Config, tree func() *zeconfig.Tree, sessions *api.ConfigSessionManager, loader func() (*yangloader.Resolved, error), notifier *ChangeNotifier) *Server {
	yangloader.KeepsModuleSet(moduleSetKeeperName)
	return &Server{
		config:        cfg,
		tree:          tree,
		sessions:      sessions,
		loader:        loader,
		notifier:      notifier,
		auditRecorder: cfg.AuditRecorder,
	}
}

// SetMetricsRegistry enables Prometheus instrumentation for gNMI RPCs and
// publishes the RPC panic boundary's ze_task_panics_total{component="gnmi"}.
// Must be called before Serve. Nil registry disables metrics.
func (s *Server) SetMetricsRegistry(reg metrics.Registry) {
	if reg != nil {
		s.metrics = initGNMIMetrics(reg)
		rpcGuard.SetMetricsRegistry(reg)
	}
}

// Serve starts the gNMI gRPC server on the configured address.
func (s *Server) Serve(ctx context.Context) error {
	// Refuse non-loopback listeners without authentication and TLS before
	// loading credentials or opening any socket.
	if err := checkGNMIListenAddr(s.config); err != nil {
		return err
	}

	// One chain per kind, the panic boundary first: grpc-go runs a chain's
	// first interceptor outermost, and places a plain UnaryInterceptor outside
	// any chain, so a separate option would leave auth unguarded.
	unary := []grpc.UnaryServerInterceptor{s.containUnaryPanic}
	stream := []grpc.StreamServerInterceptor{s.containStreamPanic}
	if s.config.Token != "" {
		unary = append(unary, s.authUnaryInterceptor)
		stream = append(stream, s.authStreamInterceptor)
	}
	opts := []grpc.ServerOption{
		grpc.ChainUnaryInterceptor(unary...),
		grpc.ChainStreamInterceptor(stream...),
	}

	// The material loads before anything listens, so a config that names TLS
	// and a file that cannot be read or parsed leaves no socket accepting
	// clear text: Serve returns the operator error and binds nothing.
	if s.config.tlsNamed() {
		creds, err := loadTLSCredentials(s.config.CertFile, s.config.KeyFile)
		if err != nil {
			return err
		}
		opts = append(opts, grpc.Creds(creds))
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

func (s *Server) recordAudit(entry audit.Entry) {
	if s.auditRecorder == nil {
		return
	}
	if err := s.auditRecorder.Record(entry); err != nil {
		logger.Warn("gNMI audit record failed", "code", "doctor-audit-record-failed", "action", entry.Action, "actor", entry.Actor, "error", err)
	}
}

// recordAuthFailure writes an auth-fail audit entry for a rejected bearer
// token. An opaque bearer token carries no trustworthy actor name, so no part
// of the rejected credential is written into the audit log.
func (s *Server) recordAuthFailure(ctx context.Context) {
	entry := audit.Entry{
		Surface: audit.GNMI,
		Action:  audit.ActionAuthFail,
		Outcome: audit.OutcomeDenied,
	}
	if p, ok := peer.FromContext(ctx); ok {
		if p.Addr != nil {
			entry.RemoteAddr = p.Addr.String()
		}
	}
	s.recordAudit(entry)
}

// The refusals checkAuth answers share one reason and one fix. Each names what
// was wrong with the request, and a wrong token is only said not to match:
// never whether it was close, nor anything about the configured token beyond
// the fact that the server requires one, which the Unauthenticated code
// already tells the caller. Each problem keeps its historic first words
// ("missing authorization", "invalid token"), which gNMI clients and
// gnmic_docker_test.go match on.
const (
	gnmiAuthWhy = "this gNMI server accepts only requests that carry its bearer token"
	gnmiAuthFix = "send gRPC metadata 'authorization: Bearer <token>' with the token set by environment gnmi token on the router"
)

// gnmiAuthRefusal answers one Unauthenticated refusal with its reason and fix.
func gnmiAuthRefusal(problem string) error {
	return status.Error(codes.Unauthenticated, opererr.New(problem, gnmiAuthWhy, gnmiAuthFix).Error())
}

func (s *Server) checkAuth(ctx context.Context) error {
	if s.config.Token == "" {
		return nil
	}
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		s.recordAuthFailure(ctx)
		return gnmiAuthRefusal("missing metadata: the gNMI request carries no gRPC metadata")
	}
	vals := md.Get("authorization")
	if len(vals) == 0 {
		s.recordAuthFailure(ctx)
		return gnmiAuthRefusal("missing authorization: the gNMI request carries no authorization metadata")
	}
	const prefix = "Bearer "
	if len(vals[0]) <= len(prefix) {
		s.recordAuthFailure(ctx)
		return gnmiAuthRefusal("invalid authorization format: expected 'Bearer <token>'")
	}
	if vals[0][:len(prefix)] != prefix {
		s.recordAuthFailure(ctx)
		return gnmiAuthRefusal("invalid authorization format: expected 'Bearer <token>'")
	}
	got := sha256.Sum256([]byte(vals[0][len(prefix):]))
	want := sha256.Sum256([]byte(s.config.Token))
	if subtle.ConstantTimeCompare(got[:], want[:]) != 1 {
		s.recordAuthFailure(ctx)
		return gnmiAuthRefusal("invalid token: the bearer token does not match this server's token")
	}
	return nil
}
