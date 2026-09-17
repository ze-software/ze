// Design: docs/features/ai-first.md — shared storage and config resolution

package resolve

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/paths"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/zefs"
)

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

// DefaultConfig returns the config filename from meta/instance/name or "ze.conf".
func DefaultConfig(store storage.Storage) string {
	if store == nil {
		return "ze.conf"
	}
	data, err := store.ReadFile(zefs.KeyInstanceName.Pattern)
	if err != nil || len(data) == 0 {
		return "ze.conf"
	}
	name := strings.TrimSpace(string(data))
	if name == "" || !validInstanceName.MatchString(name) {
		return "ze.conf"
	}
	var tb textbuf.Buffer
	return tb.Str(name).Str(".conf").String()
}
