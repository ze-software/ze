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

// imageStartTargets binds the HTTP servers (a var so tests can stub the bind).
var imageStartTargets = startTargets

// imageServer is the running image server: one HTTP server per bound
// interface, and the temporary directory holding the zefs database when SSH
// credentials are configured. It is written only by install, which
// sdk.ConfigTx serializes, and by stop after Run returned. Not safe for
// concurrent use on its own.
type imageServer struct {
	httpServers []*http.Server
	zefsDir     string
	log         *slog.Logger
}

// install makes cfg the running config. It closes every HTTP server first,
// and http.Server.Close closes the active connections too, so a download in
// flight is cut and the client retries it against the new config. A server
// that fails to build its zefs database or to bind is logged, not returned,
// as at boot.
func (s *imageServer) install(cfg imageConfig) error {
	s.stop()

	if !cfg.Enabled {
		s.log.Debug("imageserver: disabled in config")
		return nil
	}
	s.log.Info("imageserver: config applied",
		"image-directory", cfg.ImageDirectory, "boot-directory", cfg.BootDirectory)

	var zefsPath string
	if cfg.SSHUsername != "" && cfg.SSHPasswordHash != "" {
		tmpDir, tmpErr := os.MkdirTemp("", "imageserver-zefs-*")
		if tmpErr != nil {
			s.log.Error("imageserver: create zefs temp dir failed", "error", tmpErr)
			return nil
		}
		s.zefsDir = tmpDir

		var buildErr error
		zefsPath, buildErr = buildZefsDB(tmpDir, cfg.SSHUsername, cfg.SSHPasswordHash)
		if buildErr != nil {
			s.log.Error("imageserver: build zefs database failed", "error", buildErr)
			s.stop()
			return nil
		}
	}

	s.httpServers = imageStartTargets(cfg, zefsPath, listenTargets(cfg.ListenInterfaces, s.log), s.log)
	if len(s.httpServers) == 0 {
		// Every named interface failed to resolve or to bind. Report that
		// instead of logging "started" over a server that serves nothing.
		s.log.Error("imageserver: no interfaces bound; server not serving",
			"interfaces", cfg.ListenInterfaces)
		s.stop()
		return nil
	}

	logAvailableFiles(s.log, "image-directory", cfg.ImageDirectory)
	logAvailableFiles(s.log, "boot-directory", cfg.BootDirectory)
	logServedImage(s.log, cfg.ImageDirectory)

	s.log.Info("imageserver: started",
		"interfaces", cfg.ListenInterfaces,
		"listeners", len(s.httpServers),
		"image-directory", cfg.ImageDirectory,
		"boot-directory", cfg.BootDirectory)
	return nil
}

// stop closes every HTTP server, with its active connections, and removes the
// zefs directory.
func (s *imageServer) stop() {
	for _, srv := range s.httpServers {
		if err := srv.Close(); err != nil {
			s.log.Debug("imageserver: http close failed", "addr", srv.Addr, "error", err)
		}
	}
	s.httpServers = nil
	if s.zefsDir != "" {
		if err := os.RemoveAll(s.zefsDir); err != nil {
			s.log.Debug("imageserver: remove zefs dir failed", "error", err)
		}
		s.zefsDir = ""
	}
}
