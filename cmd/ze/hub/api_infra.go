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
	"fmt"
	"net"

	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/component/api"
	"github.com/ze-software/ze/internal/component/authz"
	zeconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/audit"
	"github.com/ze-software/ze/internal/core/env"
)

// resolveAPIListeners resolves each API transport's enable flag, effective
// listen addresses, shared token and gRPC TLS pair from env vars and the config
// tree, applying the same precedence and YANG refine defaults the gated
// builders (service_rest.go, service_grpc.go) use to bind. It is always-on so
// the boot-time management-listener guard can see the API's (address, auth)
// pairs before anything binds, and the builders consume the same struct, so
// exactly one resolver exists.
//
// ok=true means at least one transport will bind. ze.api-server.token is
// applied by the caller, which keeps that env value for the reload path
// (mgmt_auth_reload.go); the config token wins over it, unchanged.
func resolveAPIListeners(tree *zeconfig.Tree) (zeconfig.APIConfig, bool, error) {
	// Two questions, deliberately asked separately (see ExtractAPISettings).
	//
	// SETTINGS (token, cors-origin, the gRPC TLS pair, and the addresses each
	// transport names) apply whenever the block exists, whatever said START.
	// Gating them on `enabled` discarded the operator's instruction when
	// ze.api-server.rest.enabled or ze.api-server.grpc.enabled started the
	// transport: the daemon then refused to boot naming the token the operator
	// had written, bound the 0.0.0.0 default over the loopback address the
	// block named, or served gRPC in clear while that token crossed the wire
	// (ai/rules/protocol.md).
	cfg, _ := zeconfig.ExtractAPISettings(tree)

	// START comes only from a transport that asks for a listener, so a block
	// whose rest{} says nothing still means "config does not start REST".
	// cfg.RESTOn / cfg.GRPCOn carry the config half of that answer.
	enabled := cfg.RESTOn || cfg.GRPCOn

	if env.IsEnabled("ze.api-server.rest.enabled") && !cfg.RESTOn {
		cfg.RESTOn = true
		enabled = true
		// No rest{} block at all, so nothing named an address: bind the same
		// default extractAPIServerList synthesizes for a transport that names
		// no server.
		if len(cfg.REST) == 0 {
			cfg.REST = []zeconfig.APIListenConfig{{Host: "0.0.0.0", Port: "8081"}}
		}
	}
	if listen := env.Get("ze.api-server.rest.listen"); listen != "" && cfg.RESTOn {
		host, port, parseErr := net.SplitHostPort(listen)
		if parseErr != nil {
			return zeconfig.APIConfig{}, false, fmt.Errorf("ze.api-server.rest.listen: %w", parseErr)
		}
		// Env-var override replaces the config-provided list with one entry.
		cfg.REST = []zeconfig.APIListenConfig{{Host: host, Port: port}}
	}

	if env.IsEnabled("ze.api-server.grpc.enabled") && !cfg.GRPCOn {
		cfg.GRPCOn = true
		enabled = true
		if len(cfg.GRPC) == 0 {
			cfg.GRPC = []zeconfig.APIListenConfig{{Host: "0.0.0.0", Port: "50051"}}
		}
	}
	if listen := env.Get("ze.api-server.grpc.listen"); listen != "" && cfg.GRPCOn {
		host, port, parseErr := net.SplitHostPort(listen)
		if parseErr != nil {
			return zeconfig.APIConfig{}, false, fmt.Errorf("ze.api-server.grpc.listen: %w", parseErr)
		}
		cfg.GRPC = []zeconfig.APIListenConfig{{Host: host, Port: port}}
	}

	return cfg, enabled, nil
}

// apiGuardAddrs returns the addresses the API will actually bind: a transport
// that does not start contributes none. The address list of a dormant transport
// is parsed (ExtractAPISettings) and must never reach the management-listener
// guard, which would refuse a listener nothing binds.
func apiGuardAddrs(cfg zeconfig.APIConfig) []string {
	var addrs []string
	if cfg.RESTOn {
		addrs = append(addrs, apiListenToAddrs(cfg.REST)...)
	}
	if cfg.GRPCOn {
		addrs = append(addrs, apiListenToAddrs(cfg.GRPC)...)
	}
	return addrs
}

// apiBuildInputs carries the generic inputs the API seam needs. No
// internal/component/api/rest or /grpc type crosses it; every field is an
// always-on type resolved by the hub.
type apiBuildInputs struct {
	Config     zeconfig.APIConfig
	Server     *pluginserver.Server
	Store      storage.Storage
	ConfigPath string

	// Users is the boot user list, and answers one question only: whether this
	// daemon authenticates API callers per user at all. It is a snapshot and
	// must never decide WHICH credentials are valid.
	Users []authz.UserConfig

	// UsersLive returns the credentials valid right now. It is what the API
	// authenticator answers from, so a user an operator removes and reloads
	// loses REST and gRPC access with no restart (AC-13).
	UsersLive func() ([]authz.UserConfig, error)

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
