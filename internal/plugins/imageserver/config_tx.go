// Design: docs/architecture/config/transaction-protocol.md -- a plugin's verify, apply and rollback of one parsed config
// Related: register.go -- runImageServerPlugin, which registers the transaction

package imageserver

import (
	"fmt"
	"log/slog"
	"net/http"
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
	httpServers []*http.Server
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
// so the transaction rolls back. When those cannot be bound either, the error
// says so: a rollback that answers it is reported broken and the plugin is
// restarted.
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
	s.closeServers()
	servers, err := bindImage(cfg, zefsPath, s.log, !s.boot)
	if err != nil {
		removeZefsDir(zefsDir, s.log)
		return s.restore(err)
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

// restore binds the running config's servers again after an install whose
// own servers failed with cause, and answers the error install returns.
func (s *imageServer) restore(cause error) error {
	if !s.running.Enabled {
		return cause
	}
	servers, err := bindImage(s.running, s.zefsPath, s.log, true)
	s.httpServers = servers
	if err != nil {
		return fmt.Errorf("%w\nThe previous image server config could not be bound again either, so the image server is not serving: %w", cause, err)
	}
	return cause
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
