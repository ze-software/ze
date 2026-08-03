// Design: ai/rules/plugins.md -- compile-out-able services (feature-gate)
// Related: api_infra.go -- the always-on seam (grpcBuild hook / apiShared) this installs
//
// ze_grpc-gated gRPC API server construction. This file (with register_grpc.go)
// is the ONLY place always-on-buildable code reaches internal/component/api/grpc,
// and it is compiled only under //go:build ze_grpc. With ze_grpc off the hook is
// nil, no gRPC server is built, and the grpc server package + its YANG schema
// (api/grpc/yang) are linked nowhere -- so the linker drops them and a grpc{}
// config block is rejected as unknown.
//
// The shared engine/session manager is built once, always-on (buildAPIShared in
// api.go); this file only constructs the gRPC server from it. See
// plan/spec-feature-gate-6-api.md.

//go:build ze_grpc

package hub

import (
	"context"
	"fmt"
	"os"

	apigrpc "github.com/ze-software/ze/internal/component/api/grpc"
)

// grpcBuildImpl is the ze_grpc implementation of the grpcBuild seam hook. It
// builds and starts the gRPC server from the resolved config and the shared
// engine/sessions, returning it as a Reconfigurable handle (zero handle when
// gRPC is not enabled in config).
func grpcBuildImpl(in *apiBuildInputs, sh *apiShared) (apiServerHandle, error) {
	cfg := in.Config
	if !cfg.GRPCOn || len(cfg.GRPC) == 0 {
		return apiServerHandle{}, nil
	}

	addrs := make([]string, 0, len(cfg.GRPC))
	for _, ep := range cfg.GRPC {
		addrs = append(addrs, ep.Listen())
	}

	srv, err := apigrpc.NewGRPCServer(apigrpc.GRPCConfig{
		ListenAddrs:   addrs,
		Token:         cfg.Token,
		Authenticator: sh.Authenticator,
		Authorizer:    in.Authorizer,
		TLSCert:       cfg.GRPCTLSCert,
		TLSKey:        cfg.GRPCTLSKey,
		AuditRecorder: in.Recorder,
	}, sh.Engine, sh.Sessions)
	if err != nil {
		return apiServerHandle{}, fmt.Errorf("create gRPC API: %w", err)
	}
	errCh, startErr := srv.Start(in.Server.Context())
	if startErr != nil {
		return apiServerHandle{}, fmt.Errorf("start gRPC API: %w", startErr)
	}
	go logGRPCServerErrors(errCh)
	for _, addr := range srv.Addresses() {
		fmt.Fprintf(os.Stderr, "gRPC API server starting on %s\n", addr)
	}
	return apiServerHandle{
		Server: srv,
		Shutdown: func(context.Context) {
			// gRPC stop is graceful and does not take a deadline.
			srv.Stop()
		},
	}, nil
}

// logGRPCServerErrors logs runtime serving failures after startup already
// bound every requested gRPC listener successfully.
func logGRPCServerErrors(errCh <-chan error) {
	for err := range errCh {
		fmt.Fprintf(os.Stderr, "warning: gRPC API server: %v\n", err)
	}
}
