// Design: docs/architecture/config/syntax.md -- config schema stamping
// Related: serialize_set.go -- serialization (stamp re-emitted at persistence site, not here)
// Related: setparser.go -- parser discards stamp as comment (intentional)

package config

import (
	"slices"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/version"
)

const schemaStampPrefix = "# ze-schema: "

// FormatSchemaStamp returns the stamp header line for a config file.
// Format: "# ze-schema: <release>\n" where release is YY.MM.DD.
func FormatSchemaStamp() string {
	return schemaStampPrefix + version.Release() + "\n"
}

// SerializeStamped prints tree as Serialize does, headed by this binary's
// schema stamp, for a writer that persists the hierarchical form. The parser
// keeps a stamp it read on line 1 as the root's first head comment, so
// Serialize prints it again (fmt keeps a file's stamp). A writer that prepended
// a fresh stamp to that output would stack one stamp more on every write, so
// the stamp the tree carries is replaced: a file names one release, the one
// that wrote it.
func SerializeStamped(tree *Tree, schema *Schema) string {
	return FormatSchemaStamp() + Serialize(withoutSchemaStamp(tree), schema)
}

// withoutSchemaStamp answers tree without the stamp it carries as its first
// head comment: a copy when it has one, tree itself when it has none.
func withoutSchemaStamp(tree *Tree) *Tree {
	tree.mu.RLock()
	head := tree.comments.Head
	stamped := len(head) > 0 && strings.HasPrefix("#"+head[0], schemaStampPrefix)
	tree.mu.RUnlock()
	if !stamped {
		return tree
	}
	copied := tree.Clone()
	copied.mu.Lock()
	copied.comments.Head = slices.Clone(copied.comments.Head[1:])
	copied.mu.Unlock()
	return copied
}

// ScanStampRelease extracts the Ze release from the first line of raw config.
// Returns empty string if no valid stamp is found.
func ScanStampRelease(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	line := firstLine(raw)
	if !strings.HasPrefix(line, schemaStampPrefix) {
		return ""
	}
	return strings.TrimSpace(line[len(schemaStampPrefix):])
}

func firstLine(raw []byte) string {
	for i, b := range raw {
		if b == '\n' {
			return string(raw[:i])
		}
	}
	return string(raw)
}

// RecoverConfig attempts to load config from rollback history when the current
// config was written by a newer binary (its stamp release > this binary's release).
// It walks rollback versions newest-first, skipping those from newer binaries,
// and attempts a full parse on each candidate. The first version that parses
// successfully is published through the caller's authority-aware transaction
// (re-stamped with this binary's release) before it is returned.
//
// Returns the loaded result and true if recovery succeeded, or nil and false
// if no compatible rollback was found.
func RecoverConfig(store storage.Storage, configPath string, currentData []byte, cliPlugins []string, publish func([]byte) error) (*LoadConfigResult, bool) {
	logger := slogutil.Logger("config.recover")

	configRelease := ScanStampRelease(currentData)
	binaryRelease := version.Release()

	if !version.IsNewerRelease(configRelease, binaryRelease) {
		return nil, false
	}
	if store == nil || publish == nil {
		logger.Error("config recovery requires persistent history and a publication handler")
		return nil, false
	}

	logger.Warn("config from newer binary",
		"config-release", configRelease,
		"binary-release", binaryRelease,
		"action", "walking rollback history")

	versions, err := store.ListVersions(configPath)
	if err != nil {
		logger.Error("list rollback versions", "error", err)
		return nil, false
	}

	for _, v := range versions {
		raw, readErr := store.ReadVersion(configPath, v.Stamp)
		if readErr != nil {
			continue
		}
		rollbackRelease := ScanStampRelease(raw)
		if version.IsNewerRelease(rollbackRelease, binaryRelease) {
			continue
		}

		result, loadErr := LoadConfig(string(raw), configPath, cliPlugins)
		if loadErr != nil {
			continue
		}

		if backupErr := store.WriteVersion(configPath, currentData, time.Now()); backupErr != nil {
			logger.Error("backup current config before recovery", "error", backupErr)
			return nil, false
		}
		schema, schemaErr := YANGSchema()
		if schemaErr != nil {
			logger.Error("load schema for recovered config", "error", schemaErr)
			return nil, false
		}
		stamped := FormatSchemaStamp() + SerializeSetWithMeta(result.Tree, NewMetaTree(), schema)
		if publishErr := publish([]byte(stamped)); publishErr != nil {
			logger.Error("publish recovered config", "error", publishErr)
			return nil, false
		}
		logger.Warn("recovered config from rollback",
			"rollback-release", rollbackRelease,
			"rollback-date", v.Date.Format("2006-01-02 15:04:05"))

		return result, true
	}

	logger.Error("no compatible config found in rollback history",
		"config-release", configRelease,
		"binary-release", binaryRelease,
		"versions-checked", len(versions))

	return nil, false
}
