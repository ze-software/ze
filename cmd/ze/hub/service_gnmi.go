// Design: ai/rules/feature-gate-registration.md -- ze_gnmi compile-out seam
//
// The gNMI side of the compile-out seam (see gnmi_infra.go). This file is the
// only hub code that imports internal/component/gnmi; absent ze_gnmi, the seam
// hooks stay nil and the linker can drop the gNMI service, schema, and show RPC.

//go:build ze_gnmi

package hub

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/api"
	"codeberg.org/thomas-mangin/ze/internal/component/cli"
	zeconfig "codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/component/config/storage"
	yangloader "codeberg.org/thomas-mangin/ze/internal/component/config/yang"
	zegnmi "codeberg.org/thomas-mangin/ze/internal/component/gnmi"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	"codeberg.org/thomas-mangin/ze/internal/core/env"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
)

var activeGNMINotifier *zegnmi.ChangeNotifier

type builtGNMIServer struct {
	srv    *zegnmi.Server
	cancel context.CancelFunc
}

func (s *builtGNMIServer) Stop() {
	if s.srv != nil {
		s.srv.Stop()
	}
	if s.cancel != nil {
		s.cancel()
	}
	zegnmi.RegisterGlobal(nil)
}

func gnmiBuildImpl(in *gnmiBuildInputs) gnmiServer {
	if in == nil || in.Tree == nil {
		return nil
	}

	// Resolve enable/address/token through the always-on shared resolver
	// (gnmi_infra.go) so the boot-time management-listener guard and this
	// builder agree on exactly the (address, token) pair that binds. TLS
	// cert/key stay here because the guard does not read cert files.
	gnmiAddr, gnmiToken, gnmiEnabled := resolveGNMIListeners(in.Tree)
	gnmiTLSCert := env.Get("ze.gnmi.tls.cert")
	gnmiTLSKey := env.Get("ze.gnmi.tls.key")

	if gnmiYANG, ok := zeconfig.ExtractGNMIConfig(in.Tree); ok {
		if env.Get("ze.gnmi.listen") == "" && len(gnmiYANG.Servers) > 1 {
			slog.Warn("gNMI: only first server listener is used, extra listeners ignored", "configured", len(gnmiYANG.Servers))
		}
		if gnmiTLSCert == "" {
			gnmiTLSCert = gnmiYANG.TLS.Cert
		}
		if gnmiTLSKey == "" {
			gnmiTLSKey = gnmiYANG.TLS.Key
		}
	}

	if !gnmiEnabled {
		return nil
	}

	gnmiCfg := zegnmi.Config{
		ListenAddr: gnmiAddr,
		Token:      gnmiToken,
	}
	if gnmiTLSCert != "" {
		var err error
		if gnmiCfg.CertPEM, err = os.ReadFile(gnmiTLSCert); err != nil { //nolint:gosec // operator-configured cert path
			fmt.Fprintf(os.Stderr, "warning: gNMI TLS cert: %v\n", err)
		}
	}
	if gnmiTLSKey != "" {
		var err error
		if gnmiCfg.KeyPEM, err = os.ReadFile(gnmiTLSKey); err != nil { //nolint:gosec // operator-configured key path
			fmt.Fprintf(os.Stderr, "warning: gNMI TLS key: %v\n", err)
		}
	}

	gnmiCtx, gnmiCancel := context.WithCancel(context.Background())
	gnmiSessions := buildGNMISessionManager(in.Store, in.ConfigPath, in.ReloadAfterCommit)
	go gnmiSessions.RunCleanup(gnmiCtx)

	activeGNMINotifier = zegnmi.NewChangeNotifier()
	treeFn := in.TreeFn
	if treeFn == nil {
		treeFn = func() *zeconfig.Tree { return in.Tree }
	}
	gnmiSrv := zegnmi.NewServer(gnmiCfg, treeFn, gnmiSessions, yangloader.DefaultLoader, activeGNMINotifier)
	if reg := registry.GetMetricsRegistry(); reg != nil {
		gnmiSrv.SetMetricsRegistry(reg)
	}
	zegnmi.RegisterGlobal(gnmiSrv)

	go serveGNMI(gnmiCtx, gnmiSrv)
	readyCtx, readyCancel := context.WithTimeout(context.Background(), 5*time.Second)
	gnmiReady := waitForGNMIBind(readyCtx, gnmiSrv)
	readyCancel()
	if gnmiReady {
		fmt.Fprintf(os.Stderr, "gNMI server listening on %s\n", gnmiSrv.Address())
	} else {
		fmt.Fprintf(os.Stderr, "warning: gNMI server failed to bind on %s\n", gnmiAddr)
	}
	return &builtGNMIServer{srv: gnmiSrv, cancel: gnmiCancel}
}

func gnmiReloadNotifyImpl() {
	if activeGNMINotifier != nil {
		activeGNMINotifier.NotifyConfigReload()
	}
}

// serveGNMI runs the gNMI server's Serve in a background goroutine.
// This is a one-time component startup, not a per-event goroutine.
func serveGNMI(ctx context.Context, srv *zegnmi.Server) {
	if serveErr := srv.Serve(ctx); serveErr != nil {
		slogutil.Logger("gnmi.server").Error("gNMI server error", "error", serveErr)
	}
}

// waitForGNMIBind polls until the gNMI server has a bound address or ctx expires.
func waitForGNMIBind(ctx context.Context, srv *zegnmi.Server) bool {
	for {
		if addr := srv.Address(); addr != "" {
			return true
		}
		select {
		case <-ctx.Done():
			return false
		case <-time.After(10 * time.Millisecond):
		}
	}
}

// buildGNMISessionManager creates a ConfigSessionManager for gNMI Set operations.
// Same factory + hooks as the API session manager. It lives behind ze_gnmi
// because gNMI is its only caller; an always-on copy would be dead code in a
// no-gnmi build.
func buildGNMISessionManager(store storage.Storage, configPath string, reloadAfterCommit func() error) *api.ConfigSessionManager {
	sessions := api.NewConfigSessionManager(func() (api.ConfigEditor, error) {
		ed, err := cli.NewEditorWithStorage(store, configPath)
		if err != nil {
			return nil, fmt.Errorf("create editor: %w", err)
		}
		return ed, nil
	})
	sessions.SetValidationHook(configValidationHook(configPath))
	sessions.SetCommitHook(reloadAfterCommit)
	return sessions
}
