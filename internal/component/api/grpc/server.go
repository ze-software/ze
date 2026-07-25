// Design: docs/architecture/api/architecture.md -- gRPC API transport
//
// Package grpc provides a gRPC server that exposes the shared API engine
// via protobuf services. All logic lives in the engine; this package is a
// thin adapter handling protobuf marshaling, auth interceptors, and streaming.
package grpc

import (
	"context"
	"crypto/sha256"
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
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"

	zepb "github.com/ze-software/ze/api/proto"
	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/component/api"
	"github.com/ze-software/ze/internal/core/audit"
	"github.com/ze-software/ze/internal/core/slogutil"
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
	ListenAddrs   []string       // e.g. []string{"0.0.0.0:50051", "127.0.0.1:51051"}
	Token         string         // Single bearer token (empty = no auth). Ignored when Authenticator is set.
	Authenticator Authenticator  // Per-user auth callback. When set, Token is not checked.
	Authorizer    aaa.Authorizer // Optional per-command profile authorizer for config sessions.
	TLSCert       string         // Path to TLS certificate file (empty = plaintext)
	TLSKey        string         // Path to TLS key file (empty = plaintext)
	AuditRecorder audit.Recorder
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
	authorizer    aaa.Authorizer
	auditRecorder audit.Recorder
	srv           *grpc.Server
	// configured holds the addresses passed in by the caller, in original order.
	configured []string
	// bound holds the actual listen addresses once Serve has bound them.
	bound     []string
	listeners map[string]net.Listener // bound addr -> listener
	mu        sync.RWMutex
	stopped   bool // set by Stop; Reconfigure checks this
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

	hasTLS := cfg.TLSCert != "" && cfg.TLSKey != ""
	hasPartialTLS := (cfg.TLSCert != "") != (cfg.TLSKey != "")
	if hasPartialTLS {
		return nil, errors.New("both TLSCert and TLSKey must be set together")
	}

	for _, addr := range cfg.ListenAddrs {
		if !api.IsLoopbackAddr(addr) {
			if cfg.Token == "" && cfg.Authenticator == nil {
				return nil, fmt.Errorf("non-loopback gRPC listen address %q requires authentication (set token or users)", addr)
			}
			if !hasTLS {
				return nil, fmt.Errorf("non-loopback gRPC listen address %q with authentication requires TLS (set tls-cert and tls-key)", addr)
			}
		}
	}

	s := &GRPCServer{
		engine:        engine,
		sessions:      sessions,
		token:         cfg.Token,
		authenticator: cfg.Authenticator,
		authorizer:    cfg.Authorizer,
		auditRecorder: cfg.AuditRecorder,
		configured:    append([]string(nil), cfg.ListenAddrs...),
		listeners:     make(map[string]net.Listener),
	}

	opts := []grpc.ServerOption{
		grpc.ChainUnaryInterceptor(s.authUnaryInterceptor),
		grpc.ChainStreamInterceptor(s.authStreamInterceptor),
	}

	// Load TLS credentials if both cert and key are configured.
	if hasTLS {
		cert, err := tls.LoadX509KeyPair(cfg.TLSCert, cfg.TLSKey)
		if err != nil {
			return nil, fmt.Errorf("load TLS cert/key: %w", err)
		}
		creds := credentials.NewTLS(&tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
		})
		opts = append(opts, grpc.Creds(creds))
	}

	s.srv = grpc.NewServer(opts...)
	zepb.RegisterZeServiceServer(s.srv, &zeServiceImpl{engine: engine})
	zepb.RegisterZeConfigServiceServer(s.srv, &zeConfigServiceImpl{engine: engine, sessions: sessions, authorizer: cfg.Authorizer, auditRecorder: cfg.AuditRecorder})
	reflection.Register(s.srv)

	return s, nil
}

// Serve binds every configured listen address and starts serving. Blocks
// until the server is stopped or an unrecoverable serve error occurs on
// any listener. Bind is all-or-nothing: any bind failure rolls back the
// already-bound listeners and returns the error without entering the
// serve loop.
func (s *GRPCServer) Serve(ctx context.Context) error {
	listeners, err := s.listen(ctx)
	if err != nil {
		return err
	}
	return s.serve(listeners)
}

// Start binds every configured address before returning, then serves in a
// background goroutine. Bind errors are returned synchronously so daemon
// startup can fail closed when an explicit API listener is unavailable.
func (s *GRPCServer) Start(ctx context.Context) (<-chan error, error) {
	listeners, err := s.listen(ctx)
	if err != nil {
		return nil, err
	}
	errCh := make(chan error, 1)
	go func() {
		defer close(errCh)
		if serveErr := s.serve(listeners); serveErr != nil {
			errCh <- serveErr
		}
	}()
	return errCh, nil
}

func (s *GRPCServer) listen(ctx context.Context) ([]net.Listener, error) {
	var lc net.ListenConfig

	lnSlice := make([]net.Listener, 0, len(s.configured))
	lnMap := make(map[string]net.Listener, len(s.configured))
	bound := make([]string, 0, len(s.configured))
	for _, addr := range s.configured {
		ln, err := lc.Listen(ctx, "tcp", addr)
		if err != nil {
			for _, prev := range lnSlice {
				if closeErr := prev.Close(); closeErr != nil {
					logger.Warn("gRPC API: close partial listener", "error", closeErr)
				}
			}
			return nil, fmt.Errorf("listen %s: %w", addr, err)
		}
		resolvedAddr := ln.Addr().String()
		lnSlice = append(lnSlice, ln)
		lnMap[resolvedAddr] = ln
		bound = append(bound, resolvedAddr)
	}

	s.mu.Lock()
	s.bound = bound
	s.listeners = lnMap
	s.mu.Unlock()

	for _, addr := range bound {
		logger.Info("gRPC API server listening", "addr", addr)
	}
	return lnSlice, nil
}

func (s *GRPCServer) serve(listeners []net.Listener) error {
	errCh := make(chan error, len(listeners))
	var wg sync.WaitGroup
	for _, ln := range listeners {
		wg.Add(1)
		go func(ln net.Listener) {
			defer wg.Done()
			if serveErr := s.srv.Serve(ln); serveErr != nil &&
				!errors.Is(serveErr, grpc.ErrServerStopped) &&
				!isGRPCClosedConnError(serveErr) {
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
	s.mu.Lock()
	s.stopped = true
	s.mu.Unlock()
	s.srv.GracefulStop()
}

// Reconfigure migrates listeners to a new set of addresses.
// Bind-before-close: new listeners start serving before old ones are removed.
func (s *GRPCServer) Reconfigure(ctx context.Context, newAddrs []string) error {
	if len(newAddrs) == 0 {
		return errors.New("at least one listen address is required")
	}
	if slices.Contains(newAddrs, "") {
		return errors.New("listen address must not be empty")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.stopped {
		return errors.New("gRPC server has been shut down")
	}

	_, toAdd, toRemove := grpcListenerDiff(s.bound, newAddrs)
	if len(toAdd) == 0 && len(toRemove) == 0 {
		return nil
	}

	var lc net.ListenConfig
	newLns := make([]net.Listener, 0, len(toAdd))
	resolved := make(map[string]string, len(toAdd))
	for _, addr := range toAdd {
		ln, err := lc.Listen(ctx, "tcp", addr)
		if err != nil {
			for _, prev := range newLns {
				if closeErr := prev.Close(); closeErr != nil {
					logger.Warn("gRPC API: close partial listener", "error", closeErr)
				}
			}
			return fmt.Errorf("gRPC reconfigure bind %s: %w", addr, err)
		}
		newLns = append(newLns, ln)
		resolved[addr] = ln.Addr().String()
	}

	for _, ln := range newLns {
		resolvedAddr := ln.Addr().String()
		s.listeners[resolvedAddr] = ln
		logger.Info("gRPC API listener added", "addr", resolvedAddr)
		go func(ln net.Listener) {
			if serveErr := s.srv.Serve(ln); serveErr != nil &&
				!errors.Is(serveErr, grpc.ErrServerStopped) &&
				!isGRPCClosedConnError(serveErr) {
				logger.Error("gRPC API serve error", "error", serveErr)
			}
		}(ln)
	}

	for _, addr := range toRemove {
		if ln, ok := s.listeners[addr]; ok {
			logger.Info("gRPC API listener removed", "addr", addr)
			if closeErr := ln.Close(); closeErr != nil {
				logger.Warn("gRPC API close listener", "addr", addr, "error", closeErr)
			}
			delete(s.listeners, addr)
		}
	}

	bound := make([]string, 0, len(newAddrs))
	for _, a := range newAddrs {
		if r, ok := resolved[a]; ok {
			bound = append(bound, r)
		} else if _, ok := s.listeners[a]; ok {
			bound = append(bound, a)
		}
	}
	s.bound = bound
	s.configured = append([]string(nil), newAddrs...)
	return nil
}

func grpcListenerDiff(oldAddrs, newAddrs []string) (keep, add, remove []string) {
	oldSet := make(map[string]struct{}, len(oldAddrs))
	for _, a := range oldAddrs {
		oldSet[a] = struct{}{}
	}
	newSet := make(map[string]struct{}, len(newAddrs))
	for _, a := range newAddrs {
		newSet[a] = struct{}{}
	}
	for _, a := range newAddrs {
		if _, exists := oldSet[a]; exists {
			keep = append(keep, a)
		} else {
			add = append(add, a)
		}
	}
	for _, a := range oldAddrs {
		if _, exists := newSet[a]; !exists {
			remove = append(remove, a)
		}
	}
	return keep, add, remove
}

func isGRPCClosedConnError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "use of closed network connection")
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

type readOnlyKeyType struct{}

var readOnlyKey = readOnlyKeyType{}

// usernameFromContext extracts the authenticated username, defaulting to defaultUsername.
func usernameFromContext(ctx context.Context) string {
	if user, ok := ctx.Value(usernameKey).(string); ok {
		return user
	}
	return defaultUsername
}

func readOnlyFromContext(ctx context.Context) bool {
	readOnly, _ := ctx.Value(readOnlyKey).(bool)
	return readOnly
}

func callerIdentityFromContext(ctx context.Context) api.CallerIdentity {
	caller := api.CallerIdentity{Username: usernameFromContext(ctx), Surface: audit.GRPC, ReadOnly: readOnlyFromContext(ctx)}
	if p, ok := peer.FromContext(ctx); ok && p.Addr != nil {
		caller.RemoteAddr = p.Addr.String()
	}
	return caller
}

// wrappedStream overrides ServerStream.Context to inject the authenticated username.
type wrappedStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrappedStream) Context() context.Context { return w.ctx }

func (s *GRPCServer) authUnaryInterceptor(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	user, readOnly, err := s.checkAuth(ctx)
	if err != nil {
		return nil, err
	}
	ctx = context.WithValue(ctx, usernameKey, user)
	ctx = context.WithValue(ctx, readOnlyKey, readOnly)
	return handler(ctx, req)
}

func (s *GRPCServer) authStreamInterceptor(srv any, ss grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	user, readOnly, err := s.checkAuth(ss.Context())
	if err != nil {
		return err
	}
	ctx := context.WithValue(ss.Context(), usernameKey, user)
	ctx = context.WithValue(ctx, readOnlyKey, readOnly)
	wrapped := &wrappedStream{ServerStream: ss, ctx: ctx}
	return handler(srv, wrapped)
}

// checkAuth validates the Authorization metadata and returns the authenticated username.
func (s *GRPCServer) checkAuth(ctx context.Context) (string, bool, error) {
	if s.authenticator == nil && s.token == "" {
		return defaultUsername, true, nil
	}

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		s.recordAuthFailure(ctx, "")
		return "", false, status.Error(codes.Unauthenticated, "missing metadata")
	}
	tokens := md.Get("authorization")
	if len(tokens) == 0 {
		s.recordAuthFailure(ctx, "")
		return "", false, status.Error(codes.Unauthenticated, "missing authorization")
	}

	if s.authenticator != nil {
		user, ok := s.authenticator(tokens[0])
		if !ok {
			s.recordAuthFailure(ctx, attemptedBearerUser(tokens[0]))
			return "", false, status.Error(codes.Unauthenticated, "invalid credentials")
		}
		return user, false, nil
	}

	expected := "Bearer " + s.token
	got := sha256.Sum256([]byte(tokens[0]))
	want := sha256.Sum256([]byte(expected))
	if subtle.ConstantTimeCompare(got[:], want[:]) != 1 {
		s.recordAuthFailure(ctx, attemptedBearerUser(tokens[0]))
		return "", false, status.Error(codes.Unauthenticated, "invalid token")
	}
	return defaultUsername, false, nil
}

func (s *GRPCServer) recordAuthFailure(ctx context.Context, actor string) {
	if s.auditRecorder == nil {
		return
	}
	entry := audit.Entry{
		Actor:   actor,
		Surface: audit.GRPC,
		Action:  audit.ActionAuthFail,
		Outcome: audit.OutcomeDenied,
	}
	if p, ok := peer.FromContext(ctx); ok && p.Addr != nil {
		entry.RemoteAddr = p.Addr.String()
	}
	if err := s.auditRecorder.Record(entry); err != nil {
		logger.Warn("gRPC audit record failed", "action", entry.Action, "actor", entry.Actor, "error", err)
	}
}

func attemptedBearerUser(header string) string {
	raw, ok := strings.CutPrefix(header, "Bearer ")
	if !ok {
		return ""
	}
	username, _, ok := strings.Cut(raw, ":")
	if !ok {
		return ""
	}
	return username
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
	domainReq, buildErr := fromProtoExecuteRequest(req, callerIdentityFromContext(ctx))
	if buildErr != nil {
		return nil, status.Error(codes.InvalidArgument, buildErr.Error())
	}
	result, err := s.engine.Execute(ctx, domainReq)
	if errors.Is(err, api.ErrUnauthorized) {
		return nil, status.Error(codes.PermissionDenied, result.Error)
	}
	return execResultToProto(result), nil
}

func (s *zeServiceImpl) Stream(req *zepb.CommandRequest, stream zepb.ZeService_StreamServer) error {
	if req.GetCommand() == "" {
		return status.Error(codes.InvalidArgument, "command is required")
	}
	domainReq, buildErr := fromProtoStreamRequest(req, callerIdentityFromContext(stream.Context()))
	if buildErr != nil {
		return status.Error(codes.InvalidArgument, buildErr.Error())
	}
	ch, cancel, err := s.engine.Stream(stream.Context(), domainReq)
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
	cmds := s.engine.ListCommands(fromProtoListCommandsRequest(req))
	resp := &zepb.ListCommandsResponse{
		Commands: make([]*zepb.CommandInfo, len(cmds)),
	}
	for i, cmd := range cmds {
		resp.Commands[i] = commandMetaToProto(cmd)
	}
	return resp, nil
}

func (s *zeServiceImpl) DescribeCommand(_ context.Context, req *zepb.DescribeCommandRequest) (*zepb.CommandDescription, error) {
	cmd, err := s.engine.DescribeCommand(fromProtoDescribeCommandRequest(req))
	if errors.Is(err, api.ErrNotFound) {
		return nil, status.Error(codes.NotFound, "command not found: "+req.GetPath())
	}
	return &zepb.CommandDescription{Info: commandMetaToProto(cmd)}, nil
}

func (s *zeServiceImpl) Complete(_ context.Context, _ *zepb.CompleteRequest) (*zepb.CompleteResponse, error) {
	return nil, status.Error(codes.Unimplemented, "completion not yet implemented")
}

// --- ZeConfigService implementation ---

type zeConfigServiceImpl struct {
	zepb.UnimplementedZeConfigServiceServer
	engine        *api.APIEngine
	sessions      *api.ConfigSessionManager
	auditRecorder audit.Recorder
	authorizer    aaa.Authorizer
}

func (s *zeConfigServiceImpl) requireWriteAccess(ctx context.Context, command string) error {
	if readOnlyFromContext(ctx) {
		return status.Error(codes.PermissionDenied, "read-only API caller cannot modify configuration")
	}
	if s.authorizer == nil {
		return nil
	}
	caller := callerIdentityFromContext(ctx)
	if s.authorizer.Authorize(caller.Username, caller.RemoteAddr, command, false) {
		return nil
	}
	return status.Error(codes.PermissionDenied, "API caller is not authorized to modify configuration")
}

func (s *zeConfigServiceImpl) GetRunningConfig(ctx context.Context, _ *zepb.Empty) (*zepb.ConfigResponse, error) {
	result, err := s.engine.Execute(ctx, &api.ExecuteRequest{
		Caller:  callerIdentityFromContext(ctx),
		Command: "show config dump",
	})
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	b, jsonErr := json.Marshal(result.Data)
	if jsonErr != nil {
		return nil, status.Error(codes.Internal, jsonErr.Error())
	}
	// The running config arrives as typed Data on the unified envelope. It may
	// marshal to a bare JSON string literal (raw config text) or to a JSON
	// object; unwrap the string literal so the config text is returned verbatim,
	// matching the prior marshal-then-reparse behavior.
	var str string
	if json.Unmarshal(b, &str) == nil {
		return &zepb.ConfigResponse{Config: str}, nil
	}
	return &zepb.ConfigResponse{Config: string(b)}, nil
}

func (s *zeConfigServiceImpl) EnterSession(ctx context.Context, _ *zepb.Empty) (*zepb.SessionResponse, error) {
	if s.sessions == nil {
		return nil, status.Error(codes.Unavailable, "config sessions not available")
	}
	if err := s.requireWriteAccess(ctx, api.ConfigAuthEdit); err != nil {
		return nil, err
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
	if err := s.requireWriteAccess(ctx, api.ConfigAuthSet); err != nil {
		return nil, err
	}
	if err := s.sessions.Set(fromProtoConfigSetRequest(req, usernameFromContext(ctx))); err != nil {
		return nil, sessionStatusError(err)
	}
	return &zepb.ConfigSetResponse{Success: true}, nil
}

func (s *zeConfigServiceImpl) DeleteConfig(ctx context.Context, req *zepb.ConfigDeleteRequest) (*zepb.ConfigDeleteResponse, error) {
	if s.sessions == nil {
		return nil, status.Error(codes.Unavailable, "config sessions not available")
	}
	if err := s.requireWriteAccess(ctx, api.ConfigAuthDelete); err != nil {
		return nil, err
	}
	if err := s.sessions.Delete(fromProtoConfigDeleteRequest(req, usernameFromContext(ctx))); err != nil {
		return nil, sessionStatusError(err)
	}
	return &zepb.ConfigDeleteResponse{Success: true}, nil
}

func (s *zeConfigServiceImpl) DiffSession(ctx context.Context, req *zepb.SessionRequest) (*zepb.DiffResponse, error) {
	if s.sessions == nil {
		return nil, status.Error(codes.Unavailable, "config sessions not available")
	}
	diff, err := s.sessions.Diff(fromProtoSessionRequest(req, usernameFromContext(ctx)))
	if err != nil {
		return nil, sessionStatusError(err)
	}
	return &zepb.DiffResponse{Diff: diff}, nil
}

func (s *zeConfigServiceImpl) CommitSession(ctx context.Context, req *zepb.CommitRequest) (*zepb.CommitResponse, error) {
	if s.sessions == nil {
		return nil, status.Error(codes.Unavailable, "config sessions not available")
	}
	if err := s.requireWriteAccess(ctx, api.ConfigAuthCommit); err != nil {
		return nil, err
	}
	caller := callerIdentityFromContext(ctx)
	detail, _ := s.sessions.Diff(fromProtoSessionRequest(&zepb.SessionRequest{SessionId: req.GetSessionId()}, caller.Username))
	if err := s.sessions.Commit(fromProtoCommitRequest(req, caller.Username)); err != nil {
		return nil, sessionStatusError(err)
	}
	s.recordAudit(audit.Entry{
		Actor:      caller.Username,
		RemoteAddr: caller.RemoteAddr,
		Surface:    audit.GRPC,
		Action:     audit.ActionConfigCommit,
		Detail:     detail,
		Outcome:    audit.OutcomeSuccess,
	})
	return &zepb.CommitResponse{Success: true}, nil
}

func (s *zeConfigServiceImpl) DiscardSession(ctx context.Context, req *zepb.SessionRequest) (*zepb.DiscardResponse, error) {
	if s.sessions == nil {
		return nil, status.Error(codes.Unavailable, "config sessions not available")
	}
	if err := s.requireWriteAccess(ctx, api.ConfigAuthDiscard); err != nil {
		return nil, err
	}
	caller := callerIdentityFromContext(ctx)
	detail, _ := s.sessions.Diff(fromProtoSessionRequest(req, caller.Username))
	if err := s.sessions.Discard(fromProtoDiscardRequest(req, caller.Username)); err != nil {
		return nil, sessionStatusError(err)
	}
	s.recordAudit(audit.Entry{
		Actor:      caller.Username,
		RemoteAddr: caller.RemoteAddr,
		Surface:    audit.GRPC,
		Action:     audit.ActionConfigDiscard,
		Detail:     detail,
		Outcome:    audit.OutcomeSuccess,
	})
	return &zepb.DiscardResponse{Success: true}, nil
}

func (s *zeConfigServiceImpl) recordAudit(entry audit.Entry) {
	if s.auditRecorder == nil {
		return
	}
	if err := s.auditRecorder.Record(entry); err != nil {
		logger.Warn("gRPC audit record failed", "action", entry.Action, "actor", entry.Actor, "error", err)
	}
}

// sessionStatusError maps config session errors to gRPC status codes.
// ErrSessionForbidden becomes PermissionDenied, other errors become InvalidArgument.
func sessionStatusError(err error) error {
	if errors.Is(err, api.ErrSessionForbidden) {
		return status.Error(codes.PermissionDenied, err.Error())
	}
	return status.Error(codes.InvalidArgument, err.Error())
}
