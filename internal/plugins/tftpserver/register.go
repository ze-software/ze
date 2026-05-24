// Design: plan/spec-install-2-tftpserver.md -- TFTP server plugin registration

package tftpserver

import (
	"fmt"
	"log/slog"
	"net"
	"os"
	"sync/atomic"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	tftpschema "codeberg.org/thomas-mangin/ze/internal/plugins/tftpserver/schema"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
)

const configRootService = "service"

var loggerPtr atomic.Pointer[slog.Logger]

func init() {
	loggerPtr.Store(slogutil.DiscardLogger())

	reg := registry.Registration{
		Name:                    "tftpserver",
		Description:             "TFTP server: read-only file serving for PXE boot (RFC 1350)",
		Features:                "yang",
		YANG:                    tftpschema.ZeTFTPServerConfYANG,
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

		for _, iface := range cfg.ListenInterfaces {
			ln, err := listenTFTP(iface)
			if err != nil {
				log.Error("tftpserver: listen failed",
					"interface", iface, "error", err)
				continue
			}
			listeners = append(listeners, ln)
			go serve(ln, cfg.RootDirectory, sem, log)
		}

		log.Info("tftpserver: started",
			"root-directory", cfg.RootDirectory,
			"interfaces", cfg.ListenInterfaces,
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
