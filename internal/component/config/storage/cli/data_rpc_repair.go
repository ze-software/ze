// Design: docs/architecture/hub-architecture.md -- request data RPCs beside request reload.
// Related: data_rpc.go -- DataRPCTarget, the store the handler salvages.
// Related: cmd_integrity.go -- cmdRepair, the offline form, and salvageHistory, the history step both run.
// Related: internal/component/config/storage/salvage.go -- storage.Salvage holds the daemon's writers for the walk.

package cli

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config/storage"
	zePlugin "github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/zefs"
)

// repairVanished and repairVanishedKeys are the `request data repair` answer
// keys for the keys a writer removed from the source while the walk ran: their
// count, and one row per key.
const (
	repairVanished     = "vanished"
	repairVanishedKeys = "vanished-keys"
)

// handleDataRepair answers `request data repair path <absolute-output>`: the
// salvage `ze data repair --output <path>` writes, taken from the store the
// daemon owns while it runs. The output path is judged by the rules of a
// backup path (storage.CheckArtifactPath), then storage.Salvage refuses a name
// the store owns, a path inside the live tree and a path that exists, and
// walks the live tree with the daemon's writers held. The history of the new
// store is then repaired as the offline command repairs it. The live store is
// not changed, and installing the salvage needs ze stopped.
func handleDataRepair(_ *pluginserver.CommandContext, validated command.ValidatedArgs) (*zePlugin.Response, error) {
	target := dataRPC.Load()
	if target == nil {
		return zePlugin.OperatorErrorResponse(opererr.New(
			"request data repair: this daemon serves no store",
			"the daemon publishes its store to this command only while the store is open, so it is starting, stopping, or runs with no store",
			"ze data repair --output <path>")), nil
	}
	output, found := validated.Value(keywordPath)
	if !found {
		return zePlugin.OperatorErrorResponse(opererr.New(
			"request data repair: no output path was given",
			"the salvage is written as a new store at the absolute path typed after path",
			"request data repair path <absolute-output>")), nil
	}
	if err := storage.CheckArtifactPath(output); err != nil {
		return zePlugin.OperatorErrorResponse(opererr.Wrap(err,
			"request data repair: the output path "+output+" is refused",
			"the salvage is written on the daemon's host, so the path must be absolute, hold no '..' element and not be a symlink",
			"request data repair path <absolute-output>")), nil
	}
	// Salvage walks under storage.HoldWrites: no writer of this daemon changes
	// the tree, and no other process opens it as a writer while the daemon
	// owns it, so no key vanishes from this walk. repairAnswer still shows any
	// report.Vanished names, as `ze data repair` does, rather than assume none.
	report, err := storage.Salvage(target.Store, output)
	if err != nil {
		return zePlugin.OperatorErrorResponse(opererr.Wrap(err,
			"request data repair: no salvage was written to "+output,
			"the output must be a new path outside the store's own entries and its live tree; the live store is unchanged",
			"request data repair path <new-absolute-output>")), nil
	}
	findings, err := salvageHistory(output)
	if err != nil {
		return zePlugin.OperatorErrorResponse(opererr.Wrap(err,
			"request data repair: the salvage at "+output+" was written, its history was not repaired",
			"the history step opens the new store as its writer and could not; the live store is unchanged",
			"ze data --path "+output+" check")), nil
	}
	return repairAnswer(output, report, findings), nil
}

// repairAnswer is the answer of a written salvage: the path, the recovered,
// skipped and vanished counts, a row per skipped entry, per key a writer
// removed from the source while the walk ran, and per history finding. A
// skipped entry or a history key the repair dropped is not in the salvage, so
// either makes the answer a warning that says so, as the offline command
// exits 1. A vanished key is not in the salvage either, but the store no
// longer holds it, so it is listed and changes neither the status nor, offline,
// the exit code.
func repairAnswer(output string, report *zefs.RepairReport, findings []storage.HistoryFinding) *zePlugin.Response {
	skipped := make([]map[string]any, 0, len(report.Skipped))
	for _, entry := range report.Skipped {
		skipped = append(skipped, map[string]any{keywordKey: entry.Key, "status": entry.Status, "error": entry.Error})
	}
	vanished := make([]map[string]any, 0, len(report.Vanished))
	for _, key := range report.Vanished {
		vanished = append(vanished, map[string]any{keywordKey: key})
	}
	history := make([]map[string]any, 0, len(findings))
	dropped := 0
	for _, finding := range findings {
		action := "reported"
		if historyDropped(finding) {
			action = "dropped"
			dropped++
		}
		history = append(history, map[string]any{
			"action": action, findingKind: finding.Kind, keywordKey: finding.Key, findingDetail: finding.Detail,
		})
	}
	response := &zePlugin.Response{
		Status: zePlugin.StatusDone,
		Data: zePlugin.Map{
			keywordPath:        output,
			"recovered":        report.RecoveredCount,
			"skipped":          report.SkippedCount,
			"skipped-keys":     skipped,
			repairVanished:     len(report.Vanished),
			repairVanishedKeys: vanished,
			"history":          history,
		},
	}
	if report.SkippedCount > 0 {
		var tb textbuf.Buffer
		tb.Int(int64(report.SkippedCount)).Str(" entries did not read and are not in the salvage")
		response.Warnings = append(response.Warnings, tb.String())
	}
	if dropped > 0 {
		var tb textbuf.Buffer
		tb.Int(int64(dropped)).Str(" history keys were dropped from the salvage")
		response.Warnings = append(response.Warnings, tb.String())
	}
	if len(response.Warnings) > 0 {
		response.Status = zePlugin.StatusWarning
	}
	return response
}
