// Design: docs/architecture/config/syntax.md -- schema evolution at startup
// Related: main.go -- runYANGConfig calls applyEvolutions in Phase 1b

package hub

import (
	"log/slog"
	"time"

	zeconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/migration"
	"github.com/ze-software/ze/internal/component/config/storage"
)

// evolveOutcome holds the result of applyEvolutions.
type evolveOutcome struct {
	tree    *zeconfig.Tree
	data    []byte
	applied []string
}

// applyEvolutions runs schema evolutions on tree and writes back if any applied.
// Returns the (possibly updated) tree and data. On error, returns the originals.
func applyEvolutions(logger *slog.Logger, store storage.Storage, configPath string, data []byte, tree *zeconfig.Tree, stampRelease string) (evolveOutcome, error) {
	result, err := migration.Evolve(tree, stampRelease)
	if err != nil {
		return evolveOutcome{tree: tree, data: data}, err
	}
	if result == nil {
		return evolveOutcome{tree: tree, data: data}, nil
	}

	logger.Info("applied schema evolutions",
		"count", len(result.Applied), "evolutions", result.Applied)

	if configPath == "" || configPath == "-" {
		return evolveOutcome{tree: result.Tree, data: data, applied: result.Applied}, nil
	}

	if backupErr := initializeConfigSource(store, configPath, data); backupErr != nil {
		return evolveOutcome{tree: tree, data: data}, backupErr
	}

	schema, schemaErr := zeconfig.YANGSchema()
	if schemaErr != nil {
		return evolveOutcome{tree: result.Tree, data: data, applied: result.Applied}, schemaErr
	}

	stamped := zeconfig.FormatSchemaStamp() +
		zeconfig.SerializeSetWithMeta(result.Tree, zeconfig.NewMetaTree(), schema)
	if _, writeErr := storage.WriteCandidateVersion(store, configPath, []byte(stamped), time.Now()); writeErr != nil {
		return evolveOutcome{tree: tree, data: data}, writeErr
	}
	if writeErr := promoteConfigCandidate(store, configPath); writeErr != nil {
		return evolveOutcome{tree: tree, data: data}, writeErr
	}

	return evolveOutcome{tree: result.Tree, data: []byte(stamped), applied: result.Applied}, nil
}
