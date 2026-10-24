// Design: docs/features/ai-first.md — shared storage and config resolution

package resolve

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/paths"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/zefs"
)

// DefaultConfigName is the config filename a store with no usable instance
// name answers with. DefaultConfig is the only producer, so the name is
// written once here rather than at each arm that falls back to it.
const DefaultConfigName = "ze.conf"

// validInstanceName matches alphanumeric names with hyphens, max 64 chars.
// Prevents path traversal in blob keys.
var validInstanceName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9-]{0,63}$`)

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
// name is not a valid file stem. A read that fails for any other reason (a CRC
// mismatch, an I/O error) is an error: answering "ze.conf" for it served the
// default config on a device whose own name could not be read.
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
	name := strings.TrimSpace(string(data))
	if name == "" {
		return DefaultConfigName, nil
	}
	// Guard: `ze init` stores the host name as the instance name, and a host
	// name such as "edge.example" is not a valid file stem, so such a store
	// names its config "ze.conf". The regex also keeps "../evil" out of a key.
	if !validInstanceName.MatchString(name) {
		return DefaultConfigName, nil
	}
	var tb textbuf.Buffer
	return tb.Str(name).Str(".conf").String(), nil
}
