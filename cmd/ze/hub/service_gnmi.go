// Design: ai/rules/plugins.md -- ze_gnmi compile-out seam
//
// The gNMI side of the compile-out seam (see gnmi_infra.go). This file is the
// only hub code that imports internal/component/gnmi; absent ze_gnmi, the seam
// hooks stay nil and the linker can drop the gNMI service, schema, and show RPC.

//go:build ze_gnmi

package hub

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/ze-software/ze/internal/component/api"
	"github.com/ze-software/ze/internal/component/cli"
	zeconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	yangloader "github.com/ze-software/ze/internal/component/config/yang"
	zegnmi "github.com/ze-software/ze/internal/component/gnmi"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/slogutil"
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
	// builder agree on exactly the (address, token) pair that binds. The TLS
	// pair comes from zeconfig.GNMITLSPaths, which the gnmi-tls-material
	// doctor check reads too, so doctor judges the pair this builder loads.
	gnmiAddr, gnmiToken, gnmiEnabled := resolveGNMIListeners(in.Tree)
	gnmiTLSCert, gnmiTLSKey := zeconfig.GNMITLSPaths(in.Tree)

	// The extra-listener warning belongs to the ADDRESS question: only a block
	// that asks for a listener supplies addresses at all.
	if listenCfg, ok := zeconfig.ExtractGNMIConfig(in.Tree); ok {
		if env.Get("ze.gnmi.listen") == "" && len(listenCfg.Servers) > 1 {
			slog.Warn("gNMI: only first server listener is used, extra listeners ignored", "configured", len(listenCfg.Servers))
		}
	}
	if !gnmiEnabled {
		return nil
	}

	// The server reads the TLS files itself, before it listens, and refuses to
	// serve when they cannot be read or parsed (zegnmi tls.go). Reading them
	// here left an empty PEM on a read error, and the server then served clear
	// text over a config that asked for TLS.
	gnmiCfg := zegnmi.Config{
		ListenAddr: gnmiAddr,
		Token:      gnmiToken,
		CertFile:   gnmiTLSCert,
		KeyFile:    gnmiTLSKey,
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

	serveFailed := make(chan error, 1)
	go serveGNMI(gnmiCtx, gnmiSrv, serveFailed)
	readyCtx, readyCancel := context.WithTimeout(context.Background(), 5*time.Second)
	bindErr := waitForGNMIBind(readyCtx, gnmiSrv, serveFailed)
	readyCancel()
	if bindErr == nil {
		fmt.Fprintf(os.Stderr, "gNMI server listening on %s\n", gnmiSrv.Address())
		return &builtGNMIServer{srv: gnmiSrv, cancel: gnmiCancel}
	}
	if errors.Is(bindErr, errGNMIBindPending) {
		fmt.Fprintf(os.Stderr, "warning: gNMI server failed to bind on %s\n", gnmiAddr)
		return &builtGNMIServer{srv: gnmiSrv, cancel: gnmiCancel}
	}
	// Serve returned before it bound: the TLS material did not load, or the
	// address did not bind. Nothing listens, so nothing is served in clear
	// text. The daemon keeps running without gNMI, as it does for the other
	// management listeners; serveGNMI logged the error at Error.
	gnmiCancel()
	zegnmi.RegisterGlobal(nil)
	activeGNMINotifier = nil
	fmt.Fprintln(os.Stderr, "error:", bindErr.Error())
	return nil
}

func gnmiReloadNotifyImpl() {
	if activeGNMINotifier != nil {
		activeGNMINotifier.NotifyConfigReload()
	}
}

// errGNMIBindPending is waitForGNMIBind's answer when the server neither
// bound nor failed before the wait ended.
var errGNMIBindPending = errors.New("gNMI server has not bound yet")

// serveGNMI runs the gNMI server's Serve in a background goroutine and hands
// its error to failed, which MUST be buffered for one value: nothing may be
// reading it once the builder has returned. This is a one-time component
// startup, not a per-event goroutine.
func serveGNMI(ctx context.Context, srv *zegnmi.Server, failed chan<- error) {
	serveErr := srv.Serve(ctx)
	if serveErr == nil {
		return
	}
	failed <- serveErr
	// The refusal carries the doctor code the gnmi-tls-material check reports,
	// so the log line and `ze doctor` name the same cause (zegnmi doctor.go).
	if refusal, ok := errors.AsType[*opererr.Error](serveErr); ok {
		slogutil.Logger("gnmi.server").LogAttrs(ctx, slog.LevelError, refusal.Problem(),
			refusal.LogAttrs(slog.String("code", diagnostic.CodeDoctorGNMITLSMaterial))...)
		return
	}
	slogutil.Logger("gnmi.server").Error("gNMI server error", "error", serveErr)
}

// waitForGNMIBind polls until the gNMI server has a bound address, Serve
// returns, or ctx expires. It answers nil once bound, Serve's error when Serve
// returned first, and errGNMIBindPending when ctx expired first.
func waitForGNMIBind(ctx context.Context, srv *zegnmi.Server, failed <-chan error) error {
	for {
		if addr := srv.Address(); addr != "" {
			return nil
		}
		select {
		case serveErr := <-failed:
			return serveErr
		case <-ctx.Done():
			return errGNMIBindPending
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
	sessions.SetConfirmWindow(daemonConfirmWindow.Load)
	return sessions
}
