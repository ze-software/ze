// Design: docs/architecture/appliance/on-device-installer.md -- gokrazy first-boot auto-init fallback

//go:build ze_core

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ze-software/ze/internal/component/config/storage"
	internalresolve "github.com/ze-software/ze/internal/core/resolve"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// gokrazyAutoInit explicitly imports the appliance seed, or creates an empty
// store for discovery bootstrap. ImportBlob owns interrupted-import recovery
// and seed retirement; live-store open never converts an artifact.
func gokrazyAutoInit() (storage.Storage, error) {
	configDir := internalresolve.StoreDir("")
	if configDir == "" {
		return nil, fmt.Errorf("no config dir (ze.config.dir unset, binary location unknown)")
	}

	seed := filepath.Join(configDir, "database.zefs")
	if _, err := os.Lstat(seed); err == nil {
		store, importErr := storage.ImportBlob(seed, configDir)
		if importErr != nil {
			return nil, fmt.Errorf("gokrazy import seed %s: %w", seed, importErr)
		}
		slogutil.Logger("startup").Info("gokrazy: imported seed into live store", "seed", seed, "directory", configDir)
		return store, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("gokrazy inspect seed %s: %w", seed, err)
	}
	store, err := storage.Open(configDir)
	if !errors.Is(err, storage.ErrNoStore) {
		return store, err
	}
	store, err = storage.Create(configDir)
	if err != nil {
		return nil, fmt.Errorf("gokrazy create live store (read-only /perm? check ext4 mountability): %w", err)
	}
	slogutil.Logger("startup").Info("gokrazy: created live store", "directory", configDir)
	return store, nil
}
