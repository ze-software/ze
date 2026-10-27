// Design: docs/architecture/provisioning/tftp-server.md -- TFTP server plugin registration

package tftpserver

import (
	"fmt"
	"log/slog"
	"net"
	"os"
	"sync/atomic"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/slogutil"
	tftpyang "github.com/ze-software/ze/internal/plugins/tftpserver/yang"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const configRootService = "service"

var loggerPtr atomic.Pointer[slog.Logger]

func init() {
	loggerPtr.Store(slogutil.DiscardLogger())

	// StartAfter: the server binds each listen-interface in its Stage 2
	// configure, and the interface plugin creates the VLANs, bridges and dummies
	// of ze's own `interface {}` config in its own Stage 2 configure, so the
	// server starts in a later tier whenever both run. It is an order, not a
	// dependency: a config with no `interface` section does not load interface.
	reg := registry.Registration{
		Name:                    "tftpserver",
		Description:             "TFTP server: read-only file serving for PXE boot (RFC 1350, RFC 2347 option negotiation)",
		Features:                "yang",
		YANG:                    tftpyang.ZeTFTPServerConfYANG,
		ConfigRoots:             []string{configRootService},
		StartAfter:              []string{"interface"},
		InProcessConfigVerifier: verifyTFTPConfig,
		RunEngine:               runTFTPServerPlugin,
	}
	reg.CLIHandler = func(_ []string) int { return 1 }
	reg.ConfigureEngineLogger = func(loggerName string) {
		l := slogutil.Logger(loggerName)
		if l != nil {
			loggerPtr.Store(l)
		}
	}
	reg.ConfigureMetrics = func(metricsRegistry metrics.Registry) { tftpGuard.SetMetricsRegistry(metricsRegistry) }
	if err := registry.Register(reg); err != nil {
		fmt.Fprintf(os.Stderr, "tftpserver: registration failed: %v\n", err)
		os.Exit(1)
	}
}

func verifyTFTPConfig(sections []sdk.ConfigSection) error {
	_, _, err := parseTFTPSections(sections)
	return err
}

func runTFTPServerPlugin(conn net.Conn) int {
	log := loggerPtr.Load()
	log.Debug("tftpserver plugin starting")

	p := sdk.NewWithConn("tftpserver", conn)
	defer closeLogged(p, log, "plugin conn")

	srv := &tftpServer{log: log}
	tx := sdk.NewConfigTx(tftpConfig{}, parseTFTPSections, srv.install)
	tx.Register(p)
	p.OnConfigure(func(sections []sdk.ConfigSection) error { return srv.configureAtBoot(tx, sections) })

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	if err := p.Run(ctx, &sdk.Registration{
		WantsConfig:  []string{configRootService},
		VerifyBudget: 2,
		ApplyBudget:  5,
	}); err != nil {
		log.Error("tftpserver plugin failed", "error", err)
		srv.stop()
		return 1
	}

	srv.stop()
	log.Info("tftpserver plugin stopped")
	return 0
}

type closer interface {
	Close() error
}

func closeLogged(c closer, log *slog.Logger, what string) {
	if err := c.Close(); err != nil {
		log.Debug("tftpserver: close failed", "what", what, "error", err)
	}
}

// bindDeviceFor resolves a logical interface name to its kernel device for
// SO_BINDTODEVICE. It is best-effort, mirroring iface.ResolveDevice: when the iface
// backend is not loaded -- the install/provision path configures the interface
// directly via netlink without starting the iface component -- it returns the
// name verbatim (and the resolve error, so the caller can log the fallback).
// The returned name is the kernel device in that case.
func bindDeviceFor(ifName string) (string, error) {
	b, err := iface.Resolve(ifName)
	if err == nil && b.OsName != "" {
		return b.OsName, nil
	}
	return ifName, err
}
