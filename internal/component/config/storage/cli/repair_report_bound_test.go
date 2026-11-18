// Design: docs/architecture/storage-backends.md -- bounded salvage answers.
package cli

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config/storage"
	zePlugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/zefs"
)

// TestRepairAnswerBoundsCompleteReport retains the successful salvage outcome
// when recovered keys or findings are too large for a command response.
func TestRepairAnswerBoundsCompleteReport(t *testing.T) {
	const output = "/tmp/saved-salvage"
	keys := make([]string, rpc.MaxMessageSize/100)
	for index := range keys {
		keys[index] = "meta/" + strings.Repeat("k", 120) + "/" + strconv.Itoa(index)
	}
	for _, test := range []struct {
		name     string
		report   *zefs.RepairReport
		findings []storage.HistoryFinding
	}{
		{name: "recovered keys", report: &zefs.RepairReport{Recovered: keys, RecoveredCount: len(keys)}},
		{name: "history detail", report: &zefs.RepairReport{RecoveredCount: 1}, findings: []storage.HistoryFinding{{
			Kind: storage.HistoryDanglingPointer, Severity: storage.HistorySeverityError,
			Key: "meta/config/router.conf/active", Detail: strings.Repeat("x", rpc.MaxMessageSize+1),
		}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			response := repairAnswer(output, test.report, test.findings)
			encoded, err := json.Marshal(response)
			require.NoError(t, err)
			require.LessOrEqual(t, len(encoded), command.BlobRemoteOctetsMax)
			require.Equal(t, zePlugin.StatusError, response.Status)
			require.Contains(t, response.Error, "written")
			require.Contains(t, response.Error, "ze data --path "+output+" check")
			data, ok := response.Data.(zePlugin.Map)
			require.True(t, ok)
			require.Equal(t, output, data[keywordPath])
			require.Equal(t, test.report.RecoveredCount, data["recovered"])
			require.Equal(t, true, data["salvage-written"])
			require.Equal(t, true, data["details-omitted"])
		})
	}
}
