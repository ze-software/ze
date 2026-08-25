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
	"github.com/ze-software/ze/internal/core/slogutil"
	tftpyang "github.com/ze-software/ze/internal/plugins/tftpserver/yang"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const configRootService = "service"

var loggerPtr atomic.Pointer[slog.Logger]

func init() {
	loggerPtr.Store(slogutil.DiscardLogger())

	reg := registry.Registration{
		Name:                    "tftpserver",
		Description:             "TFTP server: read-only file serving for PXE boot (RFC 1350, RFC 2347 option negotiation)",
		Features:                "yang",
		YANG:                    tftpyang.ZeTFTPServerConfYANG,
		ConfigRoots:             []string{configRootService},
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
	if err := registry.Register(reg); err != nil {
		fmt.Fprintf(os.Stderr, "tftpserver: registration failed: %v\n", err)
		os.Exit(1)
	}
}

func verifyTFTPConfig(sections []sdk.ConfigSection) error {
	for _, s := range sections {
		if s.Root != configRootService {
			continue
		}
		cfg, err := parseConfig(s.Data)
		if err != nil {
			return fmt.Errorf("tftpserver: %w", err)
		}
		if err := verifyConfig(cfg); err != nil {
			return err
		}
	}
	return nil
}

func runTFTPServerPlugin(conn net.Conn) int {
	log := loggerPtr.Load()
	log.Debug("tftpserver plugin starting")

	p := sdk.NewWithConn("tftpserver", conn)
	defer closeLogged(p, log, "plugin conn")

	var listeners []*net.UDPConn

	stopListeners := func() {
		for _, ln := range listeners {
			closeLogged(ln, log, "udp listener")
		}
		listeners = nil
	}

	var sem chan struct{}

	startServer := func(cfg tftpConfig) {
		stopListeners()

		if !cfg.Enabled {
			log.Debug("tftpserver: disabled in config")
			return
		}

		sem = make(chan struct{}, cfg.MaxTransfers)

		for _, ifName := range cfg.ListenInterfaces {
			// Resolve the logical name to its kernel device so SO_BINDTODEVICE
			// binds the right interface when name != os device (os-name /
			// mac-match selectors). Best-effort: when no iface backend is
			// loaded -- the install/provision path configures the interface
			// directly via netlink without starting the iface component --
			// bindDeviceFor returns the name verbatim, which is the kernel
			// device in that case.
			device, rerr := bindDeviceFor(ifName)
			if rerr != nil {
				log.Debug("tftpserver: iface backend unavailable, binding by name",
					"interface", ifName, "device", device, "error", rerr)
			}
			ln, err := listenTFTP(device)
			if err != nil {
				log.Error("tftpserver: listen failed",
					"interface", ifName, "device", device, "error", err)
				continue
			}
			listeners = append(listeners, ln)
			go serve(ln, cfg.RootDirectory, sem, log)
		}

		if len(listeners) == 0 {
			log.Error("tftpserver: no interfaces bound; server not serving",
				"interfaces", cfg.ListenInterfaces)
			return
		}

		log.Info("tftpserver: started",
			"root-directory", cfg.RootDirectory,
			"interfaces", cfg.ListenInterfaces,
			"listeners", len(listeners),
			"max-transfers", cfg.MaxTransfers)
	}

	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		for _, s := range sections {
			if s.Root != configRootService {
				continue
			}
			cfg, err := parseConfig(s.Data)
			if err != nil {
				return fmt.Errorf("tftpserver: %w", err)
			}
			startServer(cfg)
			return nil
		}
		return nil
	})

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	if err := p.Run(ctx, sdk.Registration{
		WantsConfig:  []string{configRootService},
		VerifyBudget: 2,
		ApplyBudget:  5,
	}); err != nil {
		log.Error("tftpserver plugin failed", "error", err)
		stopListeners()
		return 1
	}

	stopListeners()
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
