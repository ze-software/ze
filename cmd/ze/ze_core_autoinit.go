// Design: docs/architecture/appliance/on-device-installer.md -- gokrazy first-boot auto-init fallback

//go:build ze_core

package main

import (
	"fmt"
	"os"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/paths"
	internalresolve "github.com/ze-software/ze/internal/core/resolve"
)

// gokrazyAutoInit creates the config directory and blob storage when
// running on a gokrazy appliance whose /perm/ze is missing. Returns a
// blob-backed Storage on success. On failure (e.g. read-only /perm),
// returns nil and a diagnostic error.
//
// Connectivity-only: no SSH/web credentials are written (AC-7).
// The caller falls through to the existing bootstrap which creates
// network config from template or interface discovery.
func gokrazyAutoInit() (storage.Storage, error) {
	configDir := paths.DefaultConfigDir()
	if configDir == "" {
		return nil, fmt.Errorf("no config dir (ze.config.dir unset, binary location unknown)")
	}

	if err := os.MkdirAll(configDir, 0o700); err != nil {
		msg := fmt.Errorf("gokrazy auto-init: cannot create %s (read-only /perm? check ext4 mountability): %w", configDir, err)
		writeKmsg(msg.Error())
		return nil, msg
	}

	store, err := internalresolve.Storage()
	if err != nil {
		return nil, fmt.Errorf("blob creation failed after mkdir: %w", err)
	}
	if !storage.IsBlobStorage(store) {
		store.Close() //nolint:errcheck // closing filesystem fallback
		return nil, fmt.Errorf("storage is not blob after mkdir %s", configDir)
	}

	return store, nil
}

// writeKmsg writes a diagnostic message to /dev/kmsg (kernel log) for
// visibility on gokrazy serial console. Best-effort; silently ignored
// if /dev/kmsg is unavailable.
func writeKmsg(msg string) {
	f, err := os.OpenFile("/dev/kmsg", os.O_WRONLY, 0) //nolint:gosec // fixed kernel path
	if err != nil {
		return
	}
	defer f.Close()     //nolint:errcheck // best-effort
	f.WriteString(msg)  //nolint:errcheck // best-effort
	f.WriteString("\n") //nolint:errcheck // best-effort
}
