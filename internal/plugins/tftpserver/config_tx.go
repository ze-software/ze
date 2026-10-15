// Design: docs/architecture/config/transaction-protocol.md -- a plugin's verify, apply and rollback of one parsed config
// Related: register.go -- runTFTPServerPlugin, which registers the transaction

package tftpserver

import (
	"context"
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

// tftpServer is the running TFTP server: one listener per bound interface,
// the semaphore that bounds its transfers, and the generation every transfer
// it started runs under. It is written only by install, which sdk.ConfigTx
// serializes, and by stop after Run returned. Not safe for concurrent use on
// its own.
type tftpServer struct {
	// running is the config the listeners serve, which a failed install
	// binds again.
	running tftpConfig
	// boot is true only while the Stage 2 configure installs, whose binds
	// are not strict (bindTFTP).
	boot      bool
	listeners []*net.UDPConn
	sem       chan struct{}
	// generation is canceled when a new config serves or the server stops,
	// which cuts every transfer started under the running config. cancel is
	// nil until the first install.
	generation context.Context
	cancel     context.CancelFunc
	log        *slog.Logger
}

// install makes cfg the running config, or returns an error and leaves the
// running config serving. The listeners are closed and bound again with a
// new transfer semaphore and generation; a request that arrives during the
// swap is lost and the client retransmits it. Once the new listeners serve,
// the replaced generation is canceled, which cuts every transfer still in
// flight: no file is served from a root directory the operator replaced, and
// max-transfers holds at once. A disabled cfg stops serving the same way.
//
// When an interface of cfg cannot be bound, the replaced listeners are bound
// again under the replaced generation, so its transfers go on, and the error
// is returned, so the transaction rolls back. When those cannot be bound
// either, the error says so: a rollback that answers it is reported broken
// and the plugin is restarted.
func (s *tftpServer) install(cfg tftpConfig) error {
	s.closeListeners()
	generation, cancel := context.WithCancel(context.Background())
	sem := make(chan struct{}, cfg.MaxTransfers)
	listeners, err := bindTFTP(generation, cfg, sem, s.log, !s.boot)
	if err != nil {
		cancel()
		return s.restore(err)
	}
	s.endTransfers()
	s.running, s.listeners, s.sem, s.generation, s.cancel = cfg, listeners, sem, generation, cancel
	if !cfg.Enabled {
		s.log.Debug("tftpserver: disabled in config")
		return nil
	}
	s.log.Info("tftpserver: config applied",
		"root-directory", cfg.RootDirectory, "max-transfers", cfg.MaxTransfers)
	return nil
}

// configureAtBoot runs the Stage 2 configure of tx with binds that are not
// strict, so an interface that cannot be bound at boot is logged and the
// daemon starts with the interfaces that can.
func (s *tftpServer) configureAtBoot(tx *sdk.ConfigTx[tftpConfig], sections []sdk.ConfigSection) error {
	s.boot = true
	defer func() { s.boot = false }()
	return tx.Configure(sections)
}

// restore binds the running config's listeners again, under its own
// generation and semaphore, after an install whose own listeners failed with
// cause, and answers the error install returns.
func (s *tftpServer) restore(cause error) error {
	if !s.running.Enabled {
		return cause
	}
	listeners, err := bindTFTP(s.generation, s.running, s.sem, s.log, true)
	s.listeners = listeners
	if err != nil {
		return fmt.Errorf("%w\nThe previous TFTP config could not be bound again either, so the TFTP server is not serving: %w", cause, err)
	}
	return cause
}

// bindTFTP binds a listener on every interface of cfg and serves each under
// generation. A disabled cfg binds nothing.
//
// strict is the reload mode: it binds every interface or none, and when one
// cannot be bound it closes the listeners already bound and returns an error
// naming the interface and the fix. Boot is not strict: it has no previous
// config to keep, and a configure error at boot fails the plugin and aborts
// the whole daemon start, so a failed interface is logged and the others
// serve. A device ze's own `interface {}` config creates exists by then,
// because the server starts after the interface plugin (StartAfter in init);
// a device that appears later is served only after the next reload.
func bindTFTP(generation context.Context, cfg tftpConfig, sem chan struct{}, log *slog.Logger, strict bool) ([]*net.UDPConn, error) {
	if !cfg.Enabled {
		return nil, nil
	}
	var listeners []*net.UDPConn
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
		ln, err := tftpListen(device)
		if err != nil && !strict {
			log.Error("tftpserver: listen failed",
				"interface", ifName, "device", device, "error", err)
			continue
		}
		if err != nil {
			for _, bound := range listeners {
				closeLogged(bound, log, "udp listener")
			}
			return nil, fmt.Errorf("tftpserver: cannot listen on interface %q: %w. "+
				"The TFTP server does not take this config.\n"+
				"Fix: name an interface that exists in service tftp-server listen-interface, or remove %q from it", ifName, err, ifName)
		}
		listeners = append(listeners, ln)
	}

	if len(listeners) == 0 {
		log.Error("tftpserver: no interfaces bound; server not serving",
			"interfaces", cfg.ListenInterfaces)
		return nil, nil
	}
	for _, ln := range listeners {
		go serve(generation, ln, cfg.RootDirectory, sem, log)
	}
	log.Info("tftpserver: started",
		"root-directory", cfg.RootDirectory,
		"interfaces", cfg.ListenInterfaces,
		"listeners", len(listeners),
		"max-transfers", cfg.MaxTransfers)
	return listeners, nil
}

// closeListeners closes every listener, which ends its serve loop.
func (s *tftpServer) closeListeners() {
	for _, ln := range s.listeners {
		closeLogged(ln, s.log, "udp listener")
	}
	s.listeners = nil
}

// endTransfers cancels the running generation, which cuts every transfer it
// started.
func (s *tftpServer) endTransfers() {
	if s.cancel != nil {
		s.cancel()
	}
}

// stop closes every listener and cuts every transfer. It MUST be called only
// after Run returned, when no install can run.
func (s *tftpServer) stop() {
	s.closeListeners()
	s.endTransfers()
}
