// Design: docs/architecture/config/syntax.md -- config schema stamping
// Related: serialize_set.go -- serialization (stamp re-emitted at persistence site, not here)
// Related: setparser.go -- parser discards stamp as comment (intentional)

package config

import (
	"strconv"
	"strings"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/config/storage"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
)

const SchemaStamp = 1

const schemaStampPrefix = "# ze-schema: "

func FormatSchemaStamp(stamp int) string {
	return schemaStampPrefix + strconv.Itoa(stamp) + "\n"
}

func ScanSchemaStamp(raw []byte) int {
	if len(raw) == 0 {
		return 0
	}
	line := firstLine(raw)
	if !strings.HasPrefix(line, schemaStampPrefix) {
		return 0
	}
	s := line[len(schemaStampPrefix):]
	v, err := strconv.Atoi(s)
	if err != nil || v < 0 {
		return 0
	}
	return v
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
// config has a schema stamp newer than this binary supports. It walks rollback
// versions newest-first, skipping those with stamps above SchemaStamp, and
// attempts a full parse on each candidate. The first version that parses
// successfully is written back as the current config (stamped with the current
// binary's SchemaStamp) so the displayed/active config matches what is on disk.
//
// Returns the loaded result and true if recovery succeeded, or nil and false
// if no compatible rollback was found.
func RecoverConfig(store storage.Storage, configPath string, currentData []byte, cliPlugins []string) (*LoadConfigResult, bool) {
	logger := slogutil.Logger("config.recover")

	currentStamp := ScanSchemaStamp(currentData)
	if currentStamp <= SchemaStamp {
		return nil, false
	}

	logger.Warn("config schema newer than binary",
		"config-stamp", currentStamp,
		"binary-stamp", SchemaStamp,
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
		stamp := ScanSchemaStamp(raw)
		if stamp > SchemaStamp {
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
			stamped := FormatSchemaStamp(SchemaStamp) + SerializeSetWithMeta(result.Tree, NewMetaTree(), schema)
			if writeErr := store.WriteFile(configPath, []byte(stamped), 0o600); writeErr != nil {
				logger.Error("write recovered config", "error", writeErr)
			} else {
				writtenBack = true
			}
		}

		if writtenBack {
			logger.Warn("recovered config from rollback",
				"rollback-stamp", v.Stamp,
				"rollback-date", v.Date.Format("2006-01-02 15:04:05"),
				"schema-stamp", stamp)
		} else {
			logger.Warn("recovered config from rollback (write-back failed, will re-recover on next restart)",
				"rollback-stamp", v.Stamp,
				"rollback-date", v.Date.Format("2006-01-02 15:04:05"),
				"schema-stamp", stamp)
		}

		return result, true
	}

	logger.Error("no compatible config found in rollback history",
		"config-stamp", currentStamp,
		"binary-stamp", SchemaStamp,
		"versions-checked", len(versions))

	return nil, false
}
