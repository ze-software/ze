// Design: docs/architecture/config/transaction-protocol.md -- a plugin's verify, apply and rollback of one parsed config
// Related: register.go -- runImageServerPlugin, which registers the transaction

package imageserver

import (
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// parseImageSections reads and checks the service section of one delivery.
// present is false when the delivery carries no service section. It changes
// no state, so it is also the static verifier (verifyImageConfig).
func parseImageSections(sections []sdk.ConfigSection) (imageConfig, bool, error) {
	for _, s := range sections {
		if s.Root != configRootService {
			continue
		}
		cfg, err := parseConfig(s.Data)
		if err != nil {
			return imageConfig{}, true, fmt.Errorf("imageserver: %w", err)
		}
		if err := verifyConfig(cfg); err != nil {
			return imageConfig{}, true, err
		}
		return cfg, true, nil
	}
	return imageConfig{}, false, nil
}

// imageServer is the running image server: one HTTP server per bound
// interface, and the temporary directory holding the zefs database when SSH
// credentials are configured. It is written only by install, which
// sdk.ConfigTx serializes, and by stop after Run returned. Not safe for
// concurrent use on its own.
type imageServer struct {
	// running is the config the servers serve, which a failed install binds
	// again.
	running imageConfig
	// boot is true only while the Stage 2 configure installs, whose binds
	// are not strict (bindImage).
	boot        bool
	httpServers []imageListener
	zefsDir     string
	zefsPath    string
	log         *slog.Logger
}

// install makes cfg the running config, or returns an error and leaves the
// running config serving. The zefs database is built first, in a new
// directory, so a failure there touches nothing that runs. Every HTTP server
// is then closed, with its active connections, so a download in flight is
// cut and the client retries it against the new config, and new servers are
// bound for cfg.
//
// When an interface of cfg cannot be resolved or bound, the replaced config's
// servers are bound again over its zefs database and the error is returned,
// so the transaction rolls back. When one of those cannot be bound either,
// the error wraps sdk.ErrRunningConfigLost (restore), so the transaction's
// rollback installs the running config again or restarts the plugin.
func (s *imageServer) install(cfg imageConfig) error {
	zefsDir, zefsPath, err := prepareZefs(cfg)
	if err != nil && s.boot {
		// Boot has no previous config to keep: report and serve nothing,
		// as the daemon did before reload existed.
		s.log.Error("imageserver: not serving", "error", err)
		return nil
	}
	if err != nil {
		return err
	}
	served := s.httpServers
	s.closeServers()
	servers, err := bindImage(cfg, zefsPath, s.log, !s.boot)
	if err != nil {
		removeZefsDir(zefsDir, s.log)
		return s.restore(served, err)
	}
	removeZefsDir(s.zefsDir, s.log)
	s.running, s.httpServers, s.zefsDir, s.zefsPath = cfg, servers, zefsDir, zefsPath
	if !cfg.Enabled {
		s.log.Debug("imageserver: disabled in config")
		return nil
	}
	s.log.Info("imageserver: config applied",
		"image-directory", cfg.ImageDirectory, "boot-directory", cfg.BootDirectory)
	if len(servers) == 0 {
		return nil
	}

	logAvailableFiles(s.log, "image-directory", cfg.ImageDirectory)
	logAvailableFiles(s.log, "boot-directory", cfg.BootDirectory)
	logServedImage(s.log, cfg.ImageDirectory)

	s.log.Info("imageserver: started",
		"interfaces", cfg.ListenInterfaces,
		"listeners", len(servers),
		"image-directory", cfg.ImageDirectory,
		"boot-directory", cfg.BootDirectory)
	return nil
}

// configureAtBoot runs the Stage 2 configure of tx with binds that are not
// strict, so an interface that cannot be bound at boot is logged and the
// daemon starts with the interfaces that can.
func (s *imageServer) configureAtBoot(tx *sdk.ConfigTx[imageConfig], sections []sdk.ConfigSection) error {
	s.boot = true
	defer func() { s.boot = false }()
	return tx.Configure(sections)
}

// restore binds again, one by one over the running zefs database, the
// servers that served before an install whose own servers failed with cause,
// and answers the error install returns. Each binds the address it served,
// with no new resolve. A server that binds serves again, so losing one
// interface does not close the others. When one cannot be bound, the running
// config is no longer what runs: the error wraps sdk.ErrRunningConfigLost and
// names each interface lost and why, and never says the running config was
// kept.
func (s *imageServer) restore(served []imageListener, cause error) error {
	var lost []error
	for _, previous := range served {
		srv, err := imageServeTarget(s.running, s.zefsPath, previous.target)
		if err != nil {
			lost = append(lost, lostImageTarget(previous.target, s.running.ListenPort, err))
			continue
		}
		s.httpServers = append(s.httpServers, imageListener{Server: srv, target: previous.target})
	}
	if len(lost) == 0 {
		return cause
	}
	return fmt.Errorf("%w\n%w: the image server could not bind %d of the %d interface(s) it served again, "+
		"so it no longer serves them (%w).\nFix: commit the running config again, or restart the plugin",
		cause, sdk.ErrRunningConfigLost, len(lost), len(served), errors.Join(lost...))
}

// lostImageTarget names a served target restore could not bind again. An
// empty interface is the "no interface named" target, which binds every
// address of the host.
func lostImageTarget(target listenTarget, port int, err error) error {
	if target.iface == "" {
		return fmt.Errorf("every address (%s): %w", listenAddr(target.ip, port), err)
	}
	return fmt.Errorf("interface %q (%s): %w", target.iface, listenAddr(target.ip, port), err)
}

// prepareZefs builds the zefs database of cfg in a new temporary directory
// and answers the directory and the database path, both empty when cfg
// carries no SSH credentials.
func prepareZefs(cfg imageConfig) (dir, path string, err error) {
	if !cfg.Enabled {
		return "", "", nil
	}
	// No SSH user: the image carries no zefs database, which is not an error.
	if cfg.SSHUsername == "" {
		return "", "", nil
	}
	// No password hash: the same, since a user without one cannot log in.
	if cfg.SSHPasswordHash == "" {
		return "", "", nil
	}
	dir, err = os.MkdirTemp("", "imageserver-zefs-*")
	if err != nil {
		return "", "", fmt.Errorf("imageserver: cannot create the directory for the zefs database: %w. "+
			"The image server does not take this config.\n"+
			"Fix: make the system temporary directory writable", err)
	}
	path, err = buildZefsDB(dir, cfg.SSHUsername, cfg.SSHPasswordHash)
	if err != nil {
		if rmErr := os.RemoveAll(dir); rmErr != nil {
			loggerPtr.Load().Debug("imageserver: remove zefs dir failed", "error", rmErr)
		}
		return "", "", fmt.Errorf("imageserver: cannot build the zefs database for SSH user %q: %w. "+
			"The image server does not take this config.\n"+
			"Fix: check the SSH username and password hash under service image-server", cfg.SSHUsername, err)
	}
	return dir, path, nil
}

// removeZefsDir removes a zefs directory prepareZefs made. An empty dir is
// none.
func removeZefsDir(dir string, log *slog.Logger) {
	if dir == "" {
		return
	}
	if err := os.RemoveAll(dir); err != nil {
		log.Debug("imageserver: remove zefs dir failed", "error", err)
	}
}

// closeServers closes every HTTP server, with its active connections.
func (s *imageServer) closeServers() {
	for _, srv := range s.httpServers {
		if err := srv.Close(); err != nil {
			s.log.Debug("imageserver: http close failed", "addr", srv.Addr, "error", err)
		}
	}
	s.httpServers = nil
}

// stop closes every HTTP server and removes the zefs directory. It MUST be
// called only after Run returned, when no install can run.
func (s *imageServer) stop() {
	s.closeServers()
	removeZefsDir(s.zefsDir, s.log)
	s.zefsDir, s.zefsPath = "", ""
}
