// Design: ai/rules/feature-gate-registration.md -- ze_gnmi compile-out seam
//
// The gNMI compile-out seam. Always-on daemon startup code asks this seam to
// build gNMI if an implementation is registered, but never imports the concrete
// gNMI component or names a gNMI concrete type. When ze_gnmi is absent, the hook
// vars stay nil, the reload hook is a no-op, and gNMI is dropped from the binary.

package hub

import (
	zeconfig "codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/component/config/storage"
	"codeberg.org/thomas-mangin/ze/internal/core/env"
)

// resolveGNMIListeners resolves the gNMI enable flag, effective listen address,
// and token from env vars and the config tree, applying the same precedence and
// 0.0.0.0:9339 default the gated builder (service_gnmi.go) uses to bind. It is
// always-on so the boot-time management-listener guard can see gNMI's
// (address, token) pair before anything binds, and the gated builder calls the
// SAME function so exactly one resolver exists (no drift between what the guard
// classifies and what actually binds). TLS cert/key resolution stays in the
// gated builder because the guard does not read cert files.
func resolveGNMIListeners(tree *zeconfig.Tree) (addr, token string, enabled bool) {
	addr = env.Get("ze.gnmi.listen")
	token = env.Get("ze.gnmi.token")
	if env.IsEnabled("ze.gnmi.enabled") {
		enabled = true
	}

	if gnmiYANG, ok := zeconfig.ExtractGNMIConfig(tree); ok {
		enabled = true
		if addr == "" {
			if addrs := endpointsToAddrs(gnmiYANG.Servers); len(addrs) > 0 {
				addr = addrs[0]
			}
		}
		if token == "" {
			token = gnmiYANG.Token
		}
	}

	if enabled && addr == "" {
		addr = "0.0.0.0:9339"
	}
	return addr, token, enabled
}

// gnmiServer is the always-on view of a built gNMI server. The real
// *gnmi.Server stays behind the ze_gnmi-gated implementation.
type gnmiServer interface {
	Stop()
}

// gnmiBuildInputs carries generic inputs needed by the gated gNMI builder.
// No field may name a concrete gNMI component type.
type gnmiBuildInputs struct {
	Tree              *zeconfig.Tree
	TreeFn            func() *zeconfig.Tree
	Store             storage.Storage
	ConfigPath        string
	ReloadAfterCommit func() error
}

// gnmiBuild builds and starts gNMI, returning an opaque handle. Nil means
// ze_gnmi is not compiled in, gNMI is disabled, or startup failed.
var gnmiBuild func(in *gnmiBuildInputs) gnmiServer

// gnmiReloadNotify notifies gNMI subscribers after a config reload. It is nil
// when ze_gnmi is compiled out.
var gnmiReloadNotify func()

func setGNMIInfra(build func(in *gnmiBuildInputs) gnmiServer, reloadNotify func()) {
	gnmiBuild = build
	gnmiReloadNotify = reloadNotify
}
