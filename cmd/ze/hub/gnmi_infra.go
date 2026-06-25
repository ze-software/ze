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
)

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
