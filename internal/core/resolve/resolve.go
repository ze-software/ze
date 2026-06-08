// Design: docs/features/ai-first.md — shared storage and config resolution

package resolve

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/config/storage"
	"codeberg.org/thomas-mangin/ze/internal/core/env"
	"codeberg.org/thomas-mangin/ze/internal/core/paths"
	"codeberg.org/thomas-mangin/ze/internal/core/textbuf"
	"codeberg.org/thomas-mangin/ze/pkg/zefs"
)

// validInstanceName matches alphanumeric names with hyphens, max 64 chars.
// Prevents path traversal in blob keys.
var validInstanceName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9-]{0,63}$`)

// Storage creates the appropriate storage backend.
// Returns blob storage at {configDir}/database.zefs, or filesystem as fallback.
// On error, returns a filesystem backend and the error for the caller to handle.
func Storage() (storage.Storage, error) {
	if v := env.Get("ze.storage.blob"); strings.EqualFold(v, "false") {
		return storage.NewFilesystem(), nil
	}
	configDir := env.Get("ze.config.dir")
	if configDir == "" {
		configDir = paths.DefaultConfigDir()
	}
	if configDir == "" {
		return storage.NewFilesystem(), nil
	}
	blobPath := filepath.Join(configDir, "database.zefs")
	s, err := storage.NewBlob(blobPath, configDir)
	if err != nil {
		return storage.NewFilesystem(), fmt.Errorf("blob storage at %s: %w", blobPath, err)
	}
	return s, nil
}

// DefaultConfig returns the config filename from meta/instance/name or "ze.conf".
func DefaultConfig(store storage.Storage) string {
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
