// Design: docs/features/ai-first.md — shared storage and config resolution

package resolve

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/paths"
	"github.com/ze-software/ze/pkg/zefs"
)

// DefaultConfigName is the config filename a store with no usable instance
// name answers with. It is derived from the storage package's constant, which
// the name rule (storage.InstanceConfigName) owns, so the two cannot differ.
const DefaultConfigName = storage.DefaultConfigName

// StoreDir resolves the live store's config folder. A supplied config path
// outranks the environment; stdin has no folder and uses the same resolution
// as a bare start.
func StoreDir(configPath string) string {
	if configPath != "" {
		if configPath != "-" {
			return filepath.Dir(configPath)
		}
	}
	if dir := env.Get("ze.config.dir"); dir != "" {
		return dir
	}
	return paths.DefaultConfigDir()
}

// StorageFor opens the one writable live store for this configuration.
// The caller MUST close the returned handle when its ownership ends.
func StorageFor(configPath string) (storage.Storage, error) {
	return storage.Open(StoreDir(configPath))
}

// DefaultConfig returns the config filename from meta/instance/name, or
// "ze.conf" when no store is given, the key is absent or blank, or the stored
// name is not a valid file stem (storage.InstanceConfigName holds that rule,
// which the store's freeze applies too). A read that fails for any other
// reason (a CRC mismatch, an I/O error) is an error: answering "ze.conf" for
// it served the default config on a device whose own name could not be read.
func DefaultConfig(store storage.Storage) (string, error) {
	if store == nil {
		return DefaultConfigName, nil
	}
	data, err := store.ReadFile(zefs.KeyInstanceName.Pattern)
	if errors.Is(err, fs.ErrNotExist) {
		return DefaultConfigName, nil
	}
	if err != nil {
		return "", fmt.Errorf("cannot read the instance name (%s), so the config to use is unknown; "+
			"check the store with `ze doctor` or set it with `ze init`: %w", zefs.KeyInstanceName.Pattern, err)
	}
	return storage.InstanceConfigName(data), nil
}
