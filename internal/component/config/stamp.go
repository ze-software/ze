// Design: docs/architecture/config/syntax.md -- config schema stamping
// Related: serialize_set.go -- serialization (stamp re-emitted at persistence site, not here)
// Related: setparser.go -- parser discards stamp as comment (intentional)

package config

import (
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
// successfully is written back as the current config (re-stamped with this
// binary's release) so the active config matches what is on disk.
//
// Returns the loaded result and true if recovery succeeded, or nil and false
// if no compatible rollback was found.
func RecoverConfig(store storage.Storage, configPath string, currentData []byte, cliPlugins []string) (*LoadConfigResult, bool) {
	logger := slogutil.Logger("config.recover")

	configRelease := ScanStampRelease(currentData)
	binaryRelease := version.Release()

	if !version.IsNewerRelease(configRelease, binaryRelease) {
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
		raw, readErr := store.ReadFile(v.Path)
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
		}

		writtenBack := false
		schema, schemaErr := YANGSchema()
		if schemaErr == nil {
			stamped := FormatSchemaStamp() + SerializeSetWithMeta(result.Tree, NewMetaTree(), schema)
			if writeErr := store.WriteFile(configPath, []byte(stamped), 0o600); writeErr != nil {
				logger.Error("write recovered config", "error", writeErr)
			} else {
				writtenBack = true
			}
		}

		if writtenBack {
			logger.Warn("recovered config from rollback",
				"rollback-release", rollbackRelease,
				"rollback-date", v.Date.Format("2006-01-02 15:04:05"))
		} else {
			logger.Warn("recovered config from rollback (write-back failed, will re-recover on next restart)",
				"rollback-release", rollbackRelease,
				"rollback-date", v.Date.Format("2006-01-02 15:04:05"))
		}

		return result, true
	}

	logger.Error("no compatible config found in rollback history",
		"config-release", configRelease,
		"binary-release", binaryRelease,
		"versions-checked", len(versions))

	return nil, false
}
