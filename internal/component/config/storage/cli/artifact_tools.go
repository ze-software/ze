// Design: docs/architecture/storage-backends.md -- native check and salvage on supplied artifacts.
package cli

import (
	"encoding/json"
	"errors"
	"os"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/pkg/zefs"
)

// CheckSourceAnswer shares the standalone frame/history checker with an
// owner-held supplied artifact. The caller MUST retain the source during it.
func CheckSourceAnswer(path string) command.LocalAnswer {
	verdict, err := integrityVerdict(path)
	if err != nil {
		return storageRefusal(err)
	}
	return verdict.answer()
}

// RepairSourceToArtifact runs the standalone salvage and history producers,
// then exports all surviving keys into one normal blob. output MUST be a new
// private filename. The caller owns its directory and cleanup on every result.
func RepairSourceToArtifact(source, output string) (*zefs.RepairReport, []storage.HistoryFinding, error) {
	salvage := output + ".repair"
	report, err := zefs.RepairPath(source, salvage)
	if err != nil {
		return nil, nil, err
	}
	findings, err := salvageHistory(salvage)
	if err != nil {
		return nil, nil, err
	}
	store, err := openStore(salvage, true)
	if err != nil {
		return nil, nil, err
	}
	_, copyErr := storage.Backup(store, output, false, zefs.Spare(0))
	if err := errors.Join(copyErr, store.Close()); err != nil {
		return nil, nil, err
	}
	if err := os.RemoveAll(salvage); err != nil {
		return nil, nil, err
	}
	if err := os.Remove(salvage + ".lock"); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, nil, err
		}
	}
	return report, findings, nil
}

// ArtifactRepairAnswer retains the standalone repair grading and bounded
// document, naming the private artifact and a reachable follow-up command.
func ArtifactRepairAnswer(id string, report *zefs.RepairReport, findings []storage.HistoryFinding) *plugin.Response {
	label, fix := "artifact "+id, "ze show config artifact check artifact "+id
	response := repairAnswerWithFix(label, fix, report, findings)
	data, ok := response.Data.(plugin.Map)
	if !ok {
		panic("BUG: native repair response payload is not a map")
	}
	data["artifact"] = id
	encoded, err := json.Marshal(response)
	if err != nil || len(encoded) > command.BlobRemoteOctetsMax {
		response = repairReportRefusalWithFix(label, fix, newRepairVerdict(report, findings), err)
		data, ok := response.Data.(plugin.Map)
		if !ok {
			panic("BUG: bounded repair refusal payload is not a map")
		}
		data["artifact"] = id
	}
	return response
}
