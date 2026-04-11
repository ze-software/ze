// Design: docs/architecture/api/architecture.md -- gRPC API transport
//
// Package grpc provides a gRPC server that exposes the shared API engine
// via protobuf services. All logic lives in the engine; this package is a
// thin adapter handling protobuf marshaling, auth interceptors, and streaming.
package grpc

import (
	"context"
	"crypto/subtle"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"slices"
	"strings"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"

	zepb "codeberg.org/thomas-mangin/ze/api/proto"
	"codeberg.org/thomas-mangin/ze/internal/component/api"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
)

var logger = slogutil.Logger("api.grpc")

// defaultUsername is used when no authenticator is configured (unauthenticated mode)
// or when the single-token path authenticates without identifying a specific user.
const defaultUsername = "api"

// Authenticator validates an Authorization header value and returns the
// authenticated username. Returns ("", false) on invalid credentials.
// When nil, the server accepts all requests with no authentication.
type Authenticator func(authHeader string) (username string, ok bool)

// GRPCConfig holds gRPC server configuration.
// ListenAddrs must contain at least one entry; every entry becomes a
// separate listener on the same *grpc.Server. Stop closes all of them.
type GRPCConfig struct {
	ListenAddrs   []string      // e.g. []string{"0.0.0.0:50051", "127.0.0.1:51051"}
	Token         string        // Single bearer token (empty = no auth). Ignored when Authenticator is set.
	Authenticator Authenticator // Per-user auth callback. When set, Token is not checked.
	TLSCert       string        // Path to TLS certificate file (empty = plaintext)
	TLSKey        string        // Path to TLS key file (empty = plaintext)
}

// GRPCServer is the gRPC API server.
// Caller MUST call Stop when done.
// Serve binds every address in GRPCConfig.ListenAddrs before starting any
// serve goroutine; if ANY bind fails the already-bound listeners are closed
// and Serve returns the error.
type GRPCServer struct {
	engine        *api.APIEngine
	sessions      *api.ConfigSessionManager
	token         string
	authenticator Authenticator
	srv           *grpc.Server
	// configured holds the addresses passed in by the caller, in original order.
	configured []string
	// bound holds the actual listen addresses once Serve has bound them.
	bound []string
	mu    sync.RWMutex
}

// NewGRPCServer creates a gRPC API server with auth interceptor and reflection.
// Requires at least one entry in cfg.ListenAddrs.
func NewGRPCServer(cfg GRPCConfig, engine *api.APIEngine, sessions *api.ConfigSessionManager) (*GRPCServer, error) {
	if engine == nil {
		return nil, errors.New("engine is required")
	}
	if len(cfg.ListenAddrs) == 0 {
		return nil, errors.New("at least one listen address is required")
	}
	if slices.Contains(cfg.ListenAddrs, "") {
		return nil, errors.New("listen address must not be empty")
	}

	s := &GRPCServer{
		engine:        engine,
		sessions:      sessions,
		token:         cfg.Token,
		authenticator: cfg.Authenticator,
		configured:    append([]string(nil), cfg.ListenAddrs...),
	}

	opts := []grpc.ServerOption{
		grpc.ChainUnaryInterceptor(s.authUnaryInterceptor),
		grpc.ChainStreamInterceptor(s.authStreamInterceptor),
	}

	// Load TLS credentials if both cert and key are configured.
	if cfg.TLSCert != "" && cfg.TLSKey != "" {
		cert, err := tls.LoadX509KeyPair(cfg.TLSCert, cfg.TLSKey)
		if err != nil {
			return nil, fmt.Errorf("load TLS cert/key: %w", err)
		}
		creds := credentials.NewTLS(&tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
		})
		opts = append(opts, grpc.Creds(creds))
	} else if cfg.TLSCert != "" || cfg.TLSKey != "" {
		return nil, errors.New("both TLSCert and TLSKey must be set together")
	}

	s.srv = grpc.NewServer(opts...)
	zepb.RegisterZeServiceServer(s.srv, &zeServiceImpl{engine: engine})
	zepb.RegisterZeConfigServiceServer(s.srv, &zeConfigServiceImpl{engine: engine, sessions: sessions})
	reflection.Register(s.srv)

	return s, nil
}

// Serve binds every configured listen address and starts serving. Blocks
// until the server is stopped or an unrecoverable serve error occurs on
// any listener. Bind is all-or-nothing: any bind failure rolls back the
// already-bound listeners and returns the error without entering the
// serve loop.
func (s *GRPCServer) Serve(ctx context.Context) error {
	var lc net.ListenConfig

	listeners := make([]net.Listener, 0, len(s.configured))
	bound := make([]string, 0, len(s.configured))
	for _, addr := range s.configured {
		ln, err := lc.Listen(ctx, "tcp", addr)
		if err != nil {
			for _, prev := range listeners {
				if closeErr := prev.Close(); closeErr != nil {
					logger.Warn("gRPC API: close partial listener", "error", closeErr)
				}
			}
			return fmt.Errorf("listen %s: %w", addr, err)
		}
		listeners = append(listeners, ln)
		bound = append(bound, ln.Addr().String())
	}

	s.mu.Lock()
	s.bound = bound
	s.mu.Unlock()

	for _, addr := range bound {
		logger.Info("gRPC API server listening", "addr", addr)
	}

	// grpc.Server tracks every listener internally, so GracefulStop closes
	// all of them. Serve is called once per listener in its own goroutine.
	errCh := make(chan error, len(listeners))
	var wg sync.WaitGroup
	for _, ln := range listeners {
		wg.Add(1)
		go func(ln net.Listener) {
			defer wg.Done()
			if serveErr := s.srv.Serve(ln); serveErr != nil && !errors.Is(serveErr, grpc.ErrServerStopped) {
				errCh <- serveErr
			}
		}(ln)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			return err
		}
	}
	return nil
}

// Stop gracefully stops the server, closing every bound listener.
func (s *GRPCServer) Stop() {
	s.srv.GracefulStop()
}

// Addresses returns every bound listen address in configured order.
// Before Serve binds, Addresses returns the configured addresses.
func (s *GRPCServer) Addresses() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.bound) > 0 {
		out := make([]string, len(s.bound))
		copy(out, s.bound)
		return out
	}
	out := make([]string, len(s.configured))
	copy(out, s.configured)
	return out
}

// Address returns the first bound listen address. Retained for callers that
// only want the primary endpoint.
func (s *GRPCServer) Address() string {
	addrs := s.Addresses()
	if len(addrs) == 0 {
		return ""
	}
	return addrs[0]
}

// --- Auth interceptors ---

// usernameKeyType is the context key for the authenticated username.
type usernameKeyType struct{}

var usernameKey = usernameKeyType{}

// usernameFromContext extracts the authenticated username, defaulting to defaultUsername.
func usernameFromContext(ctx context.Context) string {
	if user, ok := ctx.Value(usernameKey).(string); ok {
		return user
	}
	return defaultUsername
}

// wrappedStream overrides ServerStream.Context to inject the authenticated username.
type wrappedStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrappedStream) Context() context.Context { return w.ctx }

func (s *GRPCServer) authUnaryInterceptor(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	user, err := s.checkAuth(ctx)
	if err != nil {
		return nil, err
	}
	return handler(context.WithValue(ctx, usernameKey, user), req)
}

func (s *GRPCServer) authStreamInterceptor(srv any, ss grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	user, err := s.checkAuth(ss.Context())
	if err != nil {
		return err
	}
	wrapped := &wrappedStream{ServerStream: ss, ctx: context.WithValue(ss.Context(), usernameKey, user)}
	return handler(srv, wrapped)
}

// checkAuth validates the Authorization metadata and returns the authenticated username.
func (s *GRPCServer) checkAuth(ctx context.Context) (string, error) {
	if s.authenticator == nil && s.token == "" {
		return defaultUsername, nil
	}

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", status.Error(codes.Unauthenticated, "missing metadata")
	}
	tokens := md.Get("authorization")
	if len(tokens) == 0 {
		return "", status.Error(codes.Unauthenticated, "missing authorization")
	}

	if s.authenticator != nil {
		user, ok := s.authenticator(tokens[0])
		if !ok {
			return "", status.Error(codes.Unauthenticated, "invalid credentials")
		}
		return user, nil
	}

	expected := "Bearer " + s.token
	if subtle.ConstantTimeCompare([]byte(tokens[0]), []byte(expected)) != 1 {
		return "", status.Error(codes.Unauthenticated, "invalid token")
	}
	return defaultUsername, nil
}

// --- ZeService implementation ---

type zeServiceImpl struct {
	zepb.UnimplementedZeServiceServer
	engine *api.APIEngine
}

func (s *zeServiceImpl) Execute(ctx context.Context, req *zepb.CommandRequest) (*zepb.CommandResponse, error) {
	if req.GetCommand() == "" {
		return nil, status.Error(codes.InvalidArgument, "command is required")
	}

	// Append params as "key value" pairs, same as REST transport.
	command := buildCommand(req.GetCommand(), req.GetParams())

	result, err := s.engine.Execute(api.AuthContext{Username: usernameFromContext(ctx)}, command)
	if errors.Is(err, api.ErrUnauthorized) {
		return nil, status.Error(codes.PermissionDenied, result.Error)
	}
	return execResultToProto(result), nil
}

func (s *zeServiceImpl) Stream(req *zepb.CommandRequest, stream zepb.ZeService_StreamServer) error {
	if req.GetCommand() == "" {
		return status.Error(codes.InvalidArgument, "command is required")
	}
	ch, cancel, err := s.engine.Stream(stream.Context(), api.AuthContext{Username: usernameFromContext(stream.Context())}, req.GetCommand())
	if errors.Is(err, api.ErrUnauthorized) {
		return status.Error(codes.PermissionDenied, "unauthorized")
	}
	if err != nil {
		return status.Error(codes.Internal, err.Error())
	}
	defer cancel()

	for {
		select {
		case event, ok := <-ch:
			if !ok {
				return nil
			}
			resp := &zepb.CommandResponse{
				Status: api.StatusDone,
				Data:   []byte(event),
			}
			if sendErr := stream.Send(resp); sendErr != nil {
				return sendErr
			}
		case <-stream.Context().Done():
			return stream.Context().Err()
		}
	}
}

func (s *zeServiceImpl) ListCommands(_ context.Context, req *zepb.ListCommandsRequest) (*zepb.ListCommandsResponse, error) {
	cmds := s.engine.ListCommands(req.GetPrefix())
	resp := &zepb.ListCommandsResponse{
		Commands: make([]*zepb.CommandInfo, len(cmds)),
	}
	for i, cmd := range cmds {
		resp.Commands[i] = commandMetaToProto(cmd)
	}
	return resp, nil
}

func (s *zeServiceImpl) DescribeCommand(_ context.Context, req *zepb.DescribeCommandRequest) (*zepb.CommandDescription, error) {
	cmd, err := s.engine.DescribeCommand(req.GetPath())
	if errors.Is(err, api.ErrNotFound) {
		return nil, status.Error(codes.NotFound, "command not found: "+req.GetPath())
	}
	return &zepb.CommandDescription{Info: commandMetaToProto(cmd)}, nil
}

func (s *zeServiceImpl) Complete(_ context.Context, _ *zepb.CompleteRequest) (*zepb.CompleteResponse, error) {
	// Completion not yet wired to engine.
	return &zepb.CompleteResponse{}, nil
}

// --- ZeConfigService implementation ---

type zeConfigServiceImpl struct {
	zepb.UnimplementedZeConfigServiceServer
	engine   *api.APIEngine
	sessions *api.ConfigSessionManager
}

func (s *zeConfigServiceImpl) GetRunningConfig(ctx context.Context, _ *zepb.Empty) (*zepb.ConfigResponse, error) {
	result, err := s.engine.Execute(api.AuthContext{Username: usernameFromContext(ctx)}, "show config dump")
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	// Data may be string (plain text) or structured (parsed JSON). Marshal non-string data.
	if str, ok := result.Data.(string); ok {
		return &zepb.ConfigResponse{Config: str}, nil
	}
	b, jsonErr := json.Marshal(result.Data)
	if jsonErr != nil {
		return nil, status.Error(codes.Internal, jsonErr.Error())
	}
	return &zepb.ConfigResponse{Config: string(b)}, nil
}

func (s *zeConfigServiceImpl) EnterSession(ctx context.Context, _ *zepb.Empty) (*zepb.SessionResponse, error) {
	if s.sessions == nil {
		return nil, status.Error(codes.Unavailable, "config sessions not available")
	}
	id, err := s.sessions.Enter(usernameFromContext(ctx))
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &zepb.SessionResponse{SessionId: id}, nil
}

func (s *zeConfigServiceImpl) SetConfig(ctx context.Context, req *zepb.ConfigSetRequest) (*zepb.ConfigSetResponse, error) {
	if s.sessions == nil {
		return nil, status.Error(codes.Unavailable, "config sessions not available")
	}
	if err := s.sessions.Set(usernameFromContext(ctx), req.GetSessionId(), req.GetPath(), req.GetValue()); err != nil {
		return nil, sessionStatusError(err)
	}
	return &zepb.ConfigSetResponse{Success: true}, nil
}

func (s *zeConfigServiceImpl) DeleteConfig(ctx context.Context, req *zepb.ConfigDeleteRequest) (*zepb.ConfigDeleteResponse, error) {
	if s.sessions == nil {
		return nil, status.Error(codes.Unavailable, "config sessions not available")
	}
	if err := s.sessions.Delete(usernameFromContext(ctx), req.GetSessionId(), req.GetPath()); err != nil {
		return nil, sessionStatusError(err)
	}
	return &zepb.ConfigDeleteResponse{Success: true}, nil
}

func (s *zeConfigServiceImpl) DiffSession(ctx context.Context, req *zepb.SessionRequest) (*zepb.DiffResponse, error) {
	if s.sessions == nil {
		return nil, status.Error(codes.Unavailable, "config sessions not available")
	}
	diff, err := s.sessions.Diff(usernameFromContext(ctx), req.GetSessionId())
	if err != nil {
		return nil, sessionStatusError(err)
	}
	return &zepb.DiffResponse{Diff: diff}, nil
}

func (s *zeConfigServiceImpl) CommitSession(ctx context.Context, req *zepb.CommitRequest) (*zepb.CommitResponse, error) {
	if s.sessions == nil {
		return nil, status.Error(codes.Unavailable, "config sessions not available")
	}
	if err := s.sessions.Commit(usernameFromContext(ctx), req.GetSessionId()); err != nil {
		return nil, sessionStatusError(err)
	}
	return &zepb.CommitResponse{Success: true}, nil
}

func (s *zeConfigServiceImpl) DiscardSession(ctx context.Context, req *zepb.SessionRequest) (*zepb.DiscardResponse, error) {
	if s.sessions == nil {
		return nil, status.Error(codes.Unavailable, "config sessions not available")
	}
	if err := s.sessions.Discard(usernameFromContext(ctx), req.GetSessionId()); err != nil {
		return nil, sessionStatusError(err)
	}
	return &zepb.DiscardResponse{Success: true}, nil
}

// sessionStatusError maps config session errors to gRPC status codes.
// ErrSessionForbidden becomes PermissionDenied, other errors become InvalidArgument.
func sessionStatusError(err error) error {
	if errors.Is(err, api.ErrSessionForbidden) {
		return status.Error(codes.PermissionDenied, err.Error())
	}
	return status.Error(codes.InvalidArgument, err.Error())
}

// --- Helpers ---

func execResultToProto(r *api.ExecResult) *zepb.CommandResponse {
	if r == nil {
		return &zepb.CommandResponse{Status: api.StatusError, Error: "nil result"}
	}
	resp := &zepb.CommandResponse{
		Status: r.Status,
		Error:  r.Error,
	}
	if r.Data != nil {
		data, err := json.Marshal(r.Data)
		if err == nil {
			resp.Data = data
		}
	}
	return resp
}

// buildCommand appends params as "key value" pairs to a command string.
// Matches the REST transport's param handling for equivalence.
func buildCommand(command string, params map[string]string) string {
	if len(params) == 0 {
		return command
	}
	var b strings.Builder
	b.WriteString(command)
	for key, val := range params {
		if val == "" {
			continue
		}
		b.WriteString(" ")
		b.WriteString(key)
		b.WriteString(" ")
		b.WriteString(val)
	}
	return b.String()
}

func commandMetaToProto(cmd api.CommandMeta) *zepb.CommandInfo {
	info := &zepb.CommandInfo{
		Name:        cmd.Name,
		Description: cmd.Description,
		ReadOnly:    cmd.ReadOnly,
	}
	if len(cmd.Params) > 0 {
		info.Params = make([]*zepb.ParamInfo, len(cmd.Params))
		for i, p := range cmd.Params {
			info.Params[i] = &zepb.ParamInfo{
				Name:        p.Name,
				Type:        p.Type,
				Description: p.Description,
				Required:    p.Required,
			}
		}
	}
	return info
}
