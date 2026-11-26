// Design: docs/architecture/config/syntax.md -- schema evolution at startup
// Related: main.go -- runYANGConfig calls applyEvolutions in Phase 1b

package hub

import (
	"log/slog"
	"time"

	zeconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/migration"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/report"
)

// evolveEventSource is the report-bus source of a behavior-changing evolution.
const evolveEventSource = "hub.evolve"

// evolveOutcome holds the result of applyEvolutions.
type evolveOutcome struct {
	tree    *zeconfig.Tree
	data    []byte
	applied []string
}

// applyEvolutions runs schema evolutions on tree and writes back if any applied
// and saveToStore is true. When saveToStore is false (for example, when start
// excluded invalid service blocks from the in-memory tree), the evolved tree is
// returned in memory while the stored config file and data stay untouched.
// Returns the (possibly updated) tree and data. On error, returns the originals.
func applyEvolutions(logger *slog.Logger, store storage.Storage, configPath string, data []byte, tree *zeconfig.Tree, stampRelease string, saveToStore bool) (evolveOutcome, error) {
	result, err := migration.Evolve(tree, stampRelease)
	if err != nil {
		return evolveOutcome{tree: tree, data: data}, err
	}
	if result == nil {
		return evolveOutcome{tree: tree, data: data}, nil
	}

	logger.Info("applied schema evolutions",
		"count", len(result.Applied), "evolutions", result.Applied)

	if configPath == "" {
		return unsavedEvolution(logger, result, data), nil
	}
	if configPath == "-" {
		return unsavedEvolution(logger, result, data), nil
	}
	if !saveToStore {
		return unsavedExcludedEvolution(logger, configPath, result, data), nil
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
	warnEvolutionNotices(logger, configPath, "saved: the file now states the change, re-stamped with this release", result.Notices)

	return evolveOutcome{tree: result.Tree, data: []byte(stamped), applied: result.Applied}, nil
}

// unsavedEvolution answers the evolved tree of a config that came on stdin,
// which has no file to write back, after warning of each behavior change.
func unsavedEvolution(logger *slog.Logger, result *migration.EvolveResult, data []byte) evolveOutcome {
	warnEvolutionNotices(logger, "stdin", "not saved: the config came on stdin, so this change repeats at every start until the source carries it", result.Notices)
	return evolveOutcome{tree: result.Tree, data: data, applied: result.Applied}
}

// unsavedExcludedEvolution answers the evolved tree of a config whose start
// excluded invalid service blocks, leaving the stored file untouched so those
// blocks are not erased from disk, after warning that the evolution was not
// saved.
func unsavedExcludedEvolution(logger *slog.Logger, configPath string, result *migration.EvolveResult, data []byte) evolveOutcome {
	const unsavedReason = "not saved: start excluded one or more invalid service blocks, so the stored file stays untouched until those blocks are corrected"
	warnEvolutionNotices(logger, configPath, unsavedReason, result.Notices)
	if len(result.Notices) == 0 {
		fix := "correct the excluded service blocks reported by `ze config validate " + configPath + "`, then restart ze"
		report.RaiseWarningFix(evolveEventSource, codeDoctorConfigEvolved, configPath,
			"schema evolutions applied in memory were not saved because start excluded invalid service blocks",
			fix, map[string]any{"config": configPath, "saved": unsavedReason})
		logger.Warn("config evolution was not saved because start excluded invalid service blocks",
			"code", codeDoctorConfigEvolved,
			"config", configPath,
			"evolutions", result.Applied,
			"saved", unsavedReason,
			"fix", fix)
	}
	return evolveOutcome{tree: result.Tree, data: data, applied: result.Applied}
}

// warnEvolutionNotices logs, at Warn, each evolution that changed how the router
// behaves, and raises it on the report bus so `ze doctor` reports it. Warn is
// the default level (slogutil.DefaultLevel), so the operator hears of a change
// nobody asked for; the notice says what was written, why, and how to undo it.
// A shape-only evolution carries no notice and stays at Info.
func warnEvolutionNotices(logger *slog.Logger, source, saved string, notices []migration.EvolveNotice) {
	for _, notice := range notices {
		report.RaiseWarningFix(evolveEventSource, codeDoctorConfigEvolved, notice.Evolution, notice.Text,
			"review the change; to keep the old behavior, write the setting it names explicitly",
			map[string]any{"config": source, "saved": saved})
		logger.Warn("config evolution changed behavior", "code", codeDoctorConfigEvolved,
			"config", source, "evolution", notice.Evolution, "change", notice.Text, "saved", saved)
	}
}
