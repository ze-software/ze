// Design: docs/architecture/api/architecture.md -- gRPC API transport
//
// Package grpc provides a gRPC server that exposes the shared API engine
// via protobuf services. All logic lives in the engine; this package is a
// thin adapter handling protobuf marshaling, auth interceptors, and streaming.
package grpc

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"

	zepb "codeberg.org/thomas-mangin/ze/api/proto"
	"codeberg.org/thomas-mangin/ze/internal/component/api"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
)

var logger = slogutil.Logger("api.grpc")

// GRPCConfig holds gRPC server configuration.
type GRPCConfig struct {
	ListenAddr string // e.g. "0.0.0.0:50051"
	Token      string // Bearer token for auth (empty = no auth)
}

// GRPCServer is the gRPC API server.
// Caller MUST call Stop when done.
type GRPCServer struct {
	engine   *api.APIEngine
	sessions *api.ConfigSessionManager
	token    string
	srv      *grpc.Server
	address  string
}

// NewGRPCServer creates a gRPC API server with auth interceptor and reflection.
func NewGRPCServer(cfg GRPCConfig, engine *api.APIEngine, sessions *api.ConfigSessionManager) (*GRPCServer, error) {
	if engine == nil {
		return nil, errors.New("engine is required")
	}

	s := &GRPCServer{
		engine:   engine,
		sessions: sessions,
		token:    cfg.Token,
	}

	opts := []grpc.ServerOption{
		grpc.ChainUnaryInterceptor(s.authUnaryInterceptor),
		grpc.ChainStreamInterceptor(s.authStreamInterceptor),
	}

	s.srv = grpc.NewServer(opts...)
	zepb.RegisterZeServiceServer(s.srv, &zeServiceImpl{engine: engine})
	zepb.RegisterZeConfigServiceServer(s.srv, &zeConfigServiceImpl{engine: engine, sessions: sessions})
	reflection.Register(s.srv)

	return s, nil
}

// Serve starts the gRPC server on the given address. Blocks until stopped.
func (s *GRPCServer) Serve(ctx context.Context, addr string) error {
	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", addr, err)
	}
	s.address = ln.Addr().String()
	logger.Info("gRPC API server listening", "addr", s.address)

	return s.srv.Serve(ln)
}

// Stop gracefully stops the server.
func (s *GRPCServer) Stop() {
	s.srv.GracefulStop()
}

// Address returns the listen address (available after Serve starts).
func (s *GRPCServer) Address() string { return s.address }

// --- Auth interceptors ---

func (s *GRPCServer) authUnaryInterceptor(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	if err := s.checkAuth(ctx); err != nil {
		return nil, err
	}
	return handler(ctx, req)
}

func (s *GRPCServer) authStreamInterceptor(srv any, ss grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	if err := s.checkAuth(ss.Context()); err != nil {
		return err
	}
	return handler(srv, ss)
}

func (s *GRPCServer) checkAuth(ctx context.Context) error {
	if s.token == "" {
		return nil
	}
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "missing metadata")
	}
	tokens := md.Get("authorization")
	if len(tokens) == 0 {
		return status.Error(codes.Unauthenticated, "missing authorization")
	}
	expected := "Bearer " + s.token
	if subtle.ConstantTimeCompare([]byte(tokens[0]), []byte(expected)) != 1 {
		return status.Error(codes.Unauthenticated, "invalid token")
	}
	return nil
}

// --- ZeService implementation ---

type zeServiceImpl struct {
	zepb.UnimplementedZeServiceServer
	engine *api.APIEngine
}

func (s *zeServiceImpl) Execute(_ context.Context, req *zepb.CommandRequest) (*zepb.CommandResponse, error) {
	if req.GetCommand() == "" {
		return nil, status.Error(codes.InvalidArgument, "command is required")
	}

	// Append params as "key value" pairs, same as REST transport.
	command := buildCommand(req.GetCommand(), req.GetParams())

	result, err := s.engine.Execute(api.AuthContext{Username: "api"}, command)
	if errors.Is(err, api.ErrUnauthorized) {
		return nil, status.Error(codes.PermissionDenied, result.Error)
	}
	return execResultToProto(result), nil
}

func (s *zeServiceImpl) Stream(req *zepb.CommandRequest, stream zepb.ZeService_StreamServer) error {
	if req.GetCommand() == "" {
		return status.Error(codes.InvalidArgument, "command is required")
	}
	ch, cancel, err := s.engine.Stream(stream.Context(), api.AuthContext{Username: "api"}, req.GetCommand())
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

func (s *zeConfigServiceImpl) GetRunningConfig(_ context.Context, _ *zepb.Empty) (*zepb.ConfigResponse, error) {
	result, err := s.engine.Execute(api.AuthContext{Username: "api"}, "show config dump")
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

func (s *zeConfigServiceImpl) EnterSession(_ context.Context, _ *zepb.Empty) (*zepb.SessionResponse, error) {
	if s.sessions == nil {
		return nil, status.Error(codes.Unavailable, "config sessions not available")
	}
	id, err := s.sessions.Enter("api")
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &zepb.SessionResponse{SessionId: id}, nil
}

func (s *zeConfigServiceImpl) SetConfig(_ context.Context, req *zepb.ConfigSetRequest) (*zepb.ConfigSetResponse, error) {
	if s.sessions == nil {
		return nil, status.Error(codes.Unavailable, "config sessions not available")
	}
	if err := s.sessions.Set(req.GetSessionId(), req.GetPath(), req.GetValue()); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	return &zepb.ConfigSetResponse{Success: true}, nil
}

func (s *zeConfigServiceImpl) DeleteConfig(_ context.Context, req *zepb.ConfigDeleteRequest) (*zepb.ConfigDeleteResponse, error) {
	if s.sessions == nil {
		return nil, status.Error(codes.Unavailable, "config sessions not available")
	}
	if err := s.sessions.Delete(req.GetSessionId(), req.GetPath()); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	return &zepb.ConfigDeleteResponse{Success: true}, nil
}

func (s *zeConfigServiceImpl) DiffSession(_ context.Context, req *zepb.SessionRequest) (*zepb.DiffResponse, error) {
	if s.sessions == nil {
		return nil, status.Error(codes.Unavailable, "config sessions not available")
	}
	diff, err := s.sessions.Diff(req.GetSessionId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	return &zepb.DiffResponse{Diff: diff}, nil
}

func (s *zeConfigServiceImpl) CommitSession(_ context.Context, req *zepb.CommitRequest) (*zepb.CommitResponse, error) {
	if s.sessions == nil {
		return nil, status.Error(codes.Unavailable, "config sessions not available")
	}
	if err := s.sessions.Commit(req.GetSessionId()); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	return &zepb.CommitResponse{Success: true}, nil
}

func (s *zeConfigServiceImpl) DiscardSession(_ context.Context, req *zepb.SessionRequest) (*zepb.DiscardResponse, error) {
	if s.sessions == nil {
		return nil, status.Error(codes.Unavailable, "config sessions not available")
	}
	if err := s.sessions.Discard(req.GetSessionId()); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	return &zepb.DiscardResponse{Success: true}, nil
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
