// Design: ai/rules/plugins.md -- ze_gnmi compile-out seam
//
// The gNMI compile-out seam. Always-on daemon startup code asks this seam to
// build gNMI if an implementation is registered, but never imports the concrete
// gNMI component or names a gNMI concrete type. When ze_gnmi is absent, the hook
// vars stay nil, the reload hook is a no-op, and gNMI is dropped from the binary.

package hub

import (
	zeconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/audit"
)

// tlsPathsConfigured reports whether both a certificate path and a private-key
// path are configured. A half-named pair is never valid TLS.
func tlsPathsConfigured(certFile, keyFile string) bool {
	if certFile == "" {
		return false
	}
	return keyFile != ""
}

// gnmiMgmtListener builds the boot-time management-listener guard declaration
// for gNMI when gNMI is enabled. Non-loopback gNMI listeners require both
// authentication (token) and TLS (both cert and key paths).
func gnmiMgmtListener(tree *zeconfig.Tree) (mgmtListener, bool) {
	addr, token, enabled := zeconfig.ResolveGNMIListener(tree)
	if !enabled {
		return mgmtListener{}, false
	}
	certFile, keyFile := zeconfig.GNMITLSPaths(tree)
	return mgmtListener{
		service:       "gNMI",
		addrs:         []string{addr},
		authenticated: token != "",
		remedy:        zeconfig.GNMIAuthFix,
		requireTLS:    true,
		tlsConfigured: tlsPathsConfigured(certFile, keyFile),
		tlsRemedy:     zeconfig.GNMITLSFix,
	}, true
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
	Recorder          audit.Recorder
}

// gnmiBuild builds and starts gNMI, returning an opaque handle. Nil means
// ze_gnmi is not compiled in, gNMI is disabled, or startup failed.
var gnmiBuild func(in *gnmiBuildInputs) gnmiServer

// gnmiReloadNotify notifies gNMI subscribers after a config reload. It is nil
// when ze_gnmi is compiled out.
var gnmiReloadNotify func()
