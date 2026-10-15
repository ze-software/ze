// Design: docs/architecture/config/transaction-protocol.md -- a plugin's verify, apply and rollback of one parsed config
// Related: register.go -- runTFTPServerPlugin, which registers the transaction

package tftpserver

import (
	"fmt"
	"log/slog"
	"net"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// parseTFTPSections reads and checks the service section of one delivery.
// present is false when the delivery carries no service section. It changes
// no state, so it is also the static verifier (verifyTFTPConfig).
func parseTFTPSections(sections []sdk.ConfigSection) (tftpConfig, bool, error) {
	for _, s := range sections {
		if s.Root != configRootService {
			continue
		}
		cfg, err := parseConfig(s.Data)
		if err != nil {
			return tftpConfig{}, true, fmt.Errorf("tftpserver: %w", err)
		}
		if err := verifyConfig(cfg); err != nil {
			return tftpConfig{}, true, err
		}
		return cfg, true, nil
	}
	return tftpConfig{}, false, nil
}

// tftpListen is the per-device listener factory (a var so tests can stub the
// bind without privileges or a real interface).
var tftpListen = listenTFTP

// tftpServer is the running TFTP server: one listener per bound interface and
// the semaphore that bounds its transfers. It is written only by install,
// which sdk.ConfigTx serializes, and by stop after Run returned. Not safe for
// concurrent use on its own.
type tftpServer struct {
	listeners []*net.UDPConn
	log       *slog.Logger
}

// install makes cfg the running config: the listeners are closed and bound
// again with a new transfer semaphore. A transfer already running is not cut:
// it holds its own socket and its slot in the replaced semaphore, so it
// finishes under the config it started with, and until it does the running
// transfers can exceed the new max-transfers by the number still in flight.
// A listener that fails to bind is logged, not returned, as at boot.
func (s *tftpServer) install(cfg tftpConfig) error {
	s.closeListeners()

	if !cfg.Enabled {
		s.log.Debug("tftpserver: disabled in config")
		return nil
	}
	s.log.Info("tftpserver: config applied",
		"root-directory", cfg.RootDirectory, "max-transfers", cfg.MaxTransfers)

	sem := make(chan struct{}, cfg.MaxTransfers)
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
			s.log.Debug("tftpserver: iface backend unavailable, binding by name",
				"interface", ifName, "device", device, "error", rerr)
		}
		ln, err := tftpListen(device)
		if err != nil {
			s.log.Error("tftpserver: listen failed",
				"interface", ifName, "device", device, "error", err)
			continue
		}
		s.listeners = append(s.listeners, ln)
		go serve(ln, cfg.RootDirectory, sem, s.log)
	}

	if len(s.listeners) == 0 {
		s.log.Error("tftpserver: no interfaces bound; server not serving",
			"interfaces", cfg.ListenInterfaces)
		return nil
	}

	s.log.Info("tftpserver: started",
		"root-directory", cfg.RootDirectory,
		"interfaces", cfg.ListenInterfaces,
		"listeners", len(s.listeners),
		"max-transfers", cfg.MaxTransfers)
	return nil
}

// closeListeners closes every listener, which ends its serve loop.
func (s *tftpServer) closeListeners() {
	for _, ln := range s.listeners {
		closeLogged(ln, s.log, "udp listener")
	}
	s.listeners = nil
}
