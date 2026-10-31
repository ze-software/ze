// Design: docs/architecture/config/transaction-protocol.md -- a plugin's verify, apply and rollback of one parsed config
// Related: register.go -- runTFTPServerPlugin, which registers the transaction

package tftpserver

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"slices"
	"sync/atomic"
	"time"

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
	listeners []tftpListener
	sem       chan struct{}
	// generation ends when a new config serves or the server stops, which
	// cuts every transfer started under the running config. It is nil until
	// the first install, and endTransfers names that guard.
	generation *transferGeneration
	// limits holds the transfer limits of the running config, which each
	// listener reads when a request arrives. An install that changes only
	// them swaps this value and keeps the listeners and the transfers.
	limits atomic.Pointer[transferLimits]
	log    *slog.Logger
}

// limitsOf answers the transfer limits cfg sets.
func limitsOf(cfg tftpConfig) *transferLimits {
	return &transferLimits{
		timeout:       time.Duration(cfg.Timeout) * time.Second,
		windowsizeMax: cfg.MaxWindowsize,
	}
}

// onlyLimitsChange reports whether cfg differs from the running config in the
// transfer limits alone, while every interface of the running config is
// served: the listeners, the root directory and the semaphore then stay.
func (s *tftpServer) onlyLimitsChange(cfg tftpConfig) bool {
	running := s.running
	if !running.Enabled {
		return false
	}
	if !cfg.Enabled {
		return false
	}
	if len(s.listeners) != len(running.ListenInterfaces) {
		return false
	}
	if running.RootDirectory != cfg.RootDirectory {
		return false
	}
	if running.MaxTransfers != cfg.MaxTransfers {
		return false
	}
	return slices.Equal(running.ListenInterfaces, cfg.ListenInterfaces)
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
// is returned, so the transaction rolls back. When one of those cannot be
// bound either, the error wraps sdk.ErrRunningConfigLost (restore), so the
// transaction's rollback installs the running config again or restarts the
// plugin.
//
// An install that changes only the transfer limits (timeout, max-windowsize)
// stores them and keeps the listeners, the semaphore and the generation: no
// transfer is cut, a transfer in flight keeps the limits it started with, and
// the next request takes the new ones.
func (s *tftpServer) install(cfg tftpConfig) error {
	if s.onlyLimitsChange(cfg) {
		s.limits.Store(limitsOf(cfg))
		s.running = cfg
		s.log.Info("tftpserver: config applied",
			"timeout", cfg.Timeout, "max-windowsize", cfg.MaxWindowsize)
		return nil
	}
	served := s.listeners
	s.closeListeners()
	s.limits.Store(limitsOf(cfg))
	generation := newTransferGeneration()
	sem := make(chan struct{}, cfg.MaxTransfers)
	listeners, err := bindTFTP(generation, cfg, sem, &s.limits, s.log, !s.boot)
	if err != nil {
		generation.end(s.log)
		s.limits.Store(limitsOf(s.running))
		return s.restore(served, err)
	}
	s.endTransfers()
	s.running, s.listeners, s.sem, s.generation = cfg, listeners, sem, generation
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

// restore binds again, one by one under the running generation and
// semaphore, the listeners that served before an install whose own listeners
// failed with cause, and answers the error install returns. A listener that
// binds serves again, so losing one interface does not close the others.
// When one cannot be bound, the running config is no longer what runs: the
// error wraps sdk.ErrRunningConfigLost and names each interface lost and
// why, and never says the running config was kept.
func (s *tftpServer) restore(served []tftpListener, cause error) error {
	var lost []error
	for _, previous := range served {
		conn, err := listenInterface(previous.iface, s.log)
		if err != nil {
			lost = append(lost, fmt.Errorf("interface %q: %w", previous.iface, err))
			continue
		}
		s.listeners = append(s.listeners, tftpListener{iface: previous.iface, conn: conn})
		go serveLimited(s.generation, conn, s.running.RootDirectory, s.sem, &s.limits, s.log)
	}
	if len(lost) == 0 {
		return cause
	}
	return fmt.Errorf("%w\n%w: the TFTP server could not bind %d of the %d interface(s) it served again, "+
		"so they stopped serving (%w).\n%s",
		cause, sdk.ErrRunningConfigLost, len(lost), len(served), errors.Join(lost...), sdk.RunningConfigLostNext)
}

// tftpListener is one bound listener and the interface it serves, which a
// refused reload binds again.
type tftpListener struct {
	iface string
	conn  *net.UDPConn
}

// listenInterface binds the TFTP listener of one logical interface. It
// resolves the name to its kernel device so SO_BINDTODEVICE binds the right
// interface when name != os device (os-name / mac-match selectors).
// Best-effort: when no iface backend is loaded -- the install/provision path
// configures the interface directly via netlink without starting the iface
// component -- bindDeviceFor returns the name verbatim, which is the kernel
// device in that case.
func listenInterface(ifName string, log *slog.Logger) (*net.UDPConn, error) {
	device, rerr := bindDeviceFor(ifName)
	if rerr != nil {
		log.Debug("tftpserver: iface backend unavailable, binding by name",
			"interface", ifName, "device", device, "error", rerr)
	}
	return tftpListen(device)
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
func bindTFTP(generation *transferGeneration, cfg tftpConfig, sem chan struct{}, limits *atomic.Pointer[transferLimits], log *slog.Logger, strict bool) ([]tftpListener, error) {
	if !cfg.Enabled {
		return nil, nil
	}
	var listeners []tftpListener
	for _, ifName := range cfg.ListenInterfaces {
		ln, err := listenInterface(ifName, log)
		if err != nil && !strict {
			log.Error("tftpserver: listen failed", "interface", ifName, "error", err)
			continue
		}
		if err != nil {
			for _, bound := range listeners {
				closeLogged(bound.conn, log, "udp listener")
			}
			return nil, fmt.Errorf("tftpserver: cannot listen on interface %q: %w. "+
				"The TFTP server does not take this config.\n"+
				"Fix: name an interface that exists in service tftp-server listen-interface, or remove %q from it", ifName, err, ifName)
		}
		listeners = append(listeners, tftpListener{iface: ifName, conn: ln})
	}

	if len(listeners) == 0 {
		log.Error("tftpserver: no interfaces bound; server not serving",
			"interfaces", cfg.ListenInterfaces)
		return nil, nil
	}
	for _, ln := range listeners {
		go serveLimited(generation, ln.conn, cfg.RootDirectory, sem, limits, log)
	}
	log.Info("tftpserver: started",
		"root-directory", cfg.RootDirectory,
		"interfaces", cfg.ListenInterfaces,
		"listeners", len(listeners),
		"max-transfers", cfg.MaxTransfers,
		"timeout", cfg.Timeout,
		"max-windowsize", cfg.MaxWindowsize)
	return listeners, nil
}

// closeListeners closes every listener, which ends its serve loop.
func (s *tftpServer) closeListeners() {
	for _, ln := range s.listeners {
		closeLogged(ln.conn, s.log, "udp listener")
	}
	s.listeners = nil
}

// endTransfers ends the running generation, which closes the socket of every
// transfer it started before it returns. Before the first install no
// generation exists and no transfer runs, so there is nothing to cut.
func (s *tftpServer) endTransfers() {
	if s.generation == nil {
		return
	}
	s.generation.end(s.log)
}

// stop closes every listener and cuts every transfer. It MUST be called only
// after Run returned, when no install can run.
func (s *tftpServer) stop() {
	s.closeListeners()
	s.endTransfers()
}
