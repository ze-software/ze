// Design: docs/features/ai-first.md — shared storage and config resolution

package resolve

import (
	"fmt"
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

// EnvKeyStorageBlob selects the storage backend Storage returns: the blob
// database at {configDir}/database.zefs, or the filesystem when it reads false.
//
// The registration lives here, beside the only reader, as internal/core/version
// registers ze.hide-version and internal/core/privilege registers ze.user beside
// theirs. env.Get ends the process on an unregistered key, which is the right
// answer for a typo and the wrong failure for a package that reads a key and
// leaves the registration to a composition root it does not link: the daemon
// works and every test binary reaching this path dies, naming the key rather
// than the missing registration (plan/journal/env-key-read-but-never-registered.md).
const EnvKeyStorageBlob = "ze.storage.blob"

var _ = env.MustRegister(env.EnvEntry{
	Key:         EnvKeyStorageBlob,
	Type:        "bool",
	Default:     "true",
	Description: "Use blob storage (false = filesystem)",
})

// Storage creates the appropriate storage backend.
// Returns blob storage at {configDir}/database.zefs, or filesystem as fallback.
// On error, returns a filesystem backend and the error for the caller to handle.
func Storage() (storage.Storage, error) {
	if v := env.Get(EnvKeyStorageBlob); strings.EqualFold(v, "false") {
		return storage.NewFilesystem(), nil
	}
	configDir := paths.DefaultConfigDir()
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
