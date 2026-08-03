// Design: ai/rules/plugins.md -- compile-out-able services (feature-gate)
// Related: api.go -- always-on API helpers (engine, auth, config-validation hook, buildAPIShared)
//
// API compile-out seam. REST and gRPC are independent encodings, each
// compile-out-able via its own build tag (ze_rest / ze_grpc) so an operator can
// ship gRPC-without-REST or vice-versa. They SHARE one API engine + config
// session manager (built always-on by buildAPIShared, using only the parent
// internal/component/api package and other always-on helpers), then each gated
// transport builds its own server from that shared state.
//
// This always-on file declares the generic seam surface; service_rest.go
// (ze_rest) and service_grpc.go (ze_grpc) install their build hooks via
// register_rest.go / register_grpc.go. With a transport's tag off its hook is
// nil, its server is not built, and its package (internal/component/api/rest or
// /grpc, plus its YANG schema) is linked nowhere -- so the linker drops it.
//
// The PARENT package internal/component/api (ConfigSessionManager, shared types)
// and the base api-server YANG (api-server { token }) stay always-on: gNMI and
// other surfaces use the parent, and the base parent must exist for the gated
// rest{}/grpc{} schema containers to merge into. See plan/spec-feature-gate-6-api.md.

package hub

import (
	"context"

	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/component/api"
	"github.com/ze-software/ze/internal/component/authz"
	zeconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/audit"
)

// apiBuildInputs carries the generic inputs the API seam needs. No
// internal/component/api/rest or /grpc type crosses it; every field is an
// always-on type resolved by the hub.
type apiBuildInputs struct {
	Config     zeconfig.APIConfig
	Server     *pluginserver.Server
	Store      storage.Storage
	ConfigPath string
	Users      []authz.UserConfig
	Authorizer aaa.Authorizer
	ReloadHook func() error
	Recorder   audit.Recorder
}

// apiShared is the engine + config session manager + authenticator shared by
// both transports. Built once, always-on (buildAPIShared) from the parent api
// package, so a transport's gated builder only constructs its own server. The
// fields are parent internal/component/api types (always-on), never rest/grpc.
type apiShared struct {
	Engine        *api.APIEngine
	Sessions      *api.ConfigSessionManager
	Authenticator func(string) (string, bool)
}

// apiServerHandle is one built transport: the running server as Reconfigurable
// (so the always-on ListenerMigrator drives it without naming a server type)
// plus its shutdown. A zero handle (nil Server) means the transport was not
// enabled in config.
type apiServerHandle struct {
	Server   Reconfigurable
	Shutdown func(context.Context)
}

// restBuild / grpcBuild are installed by register_rest.go / register_grpc.go
// under //go:build ze_rest / ze_grpc. Either is nil when that transport is
// compiled out, in which case the hub skips it.
var (
	restBuild func(*apiBuildInputs, *apiShared) (apiServerHandle, error)
	grpcBuild func(*apiBuildInputs, *apiShared) (apiServerHandle, error)
)

// setRESTInfra installs the gated REST build hook (ze_rest).
func setRESTInfra(build func(*apiBuildInputs, *apiShared) (apiServerHandle, error)) {
	restBuild = build
}

// setGRPCInfra installs the gated gRPC build hook (ze_grpc).
func setGRPCInfra(build func(*apiBuildInputs, *apiShared) (apiServerHandle, error)) {
	grpcBuild = build
}
