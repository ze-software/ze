package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command/commandtest"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

const (
	// ownedWarningCode is the code a fixture plugin's runtime check declares
	// and the same plugin raises on the report bus while the fault lasts.
	ownedWarningCode = "doctor-fixture-unsaved"

	// ownedWarningMessage marks the bus warning, so a row recent-events
	// rendered from it can be told from the plugin check's own row.
	ownedWarningMessage = "fixture answer is not saved on the bus"

	// ownedCheckMessage marks the plugin check's own finding.
	ownedCheckMessage = "fixture answer is not saved per the check"
)

// owningDoctorCheckSource is a running-plugin set holding one plugin whose
// runtime check declares ownedWarningCode. With fail set its callback errors,
// so the check reports nothing.
type owningDoctorCheckSource struct {
	fail bool
}

func (owningDoctorCheckSource) DoctorCheckPlugins() map[string][]plugin.DoctorCheckRegistration {
	return map[string][]plugin.DoctorCheckRegistration{
		"fixture-owning-plugin": {{
			Name: "fixture-owning-check", Phase: rpc.DoctorPhaseRuntime,
			Platforms: []string{"any"}, Codes: []string{ownedWarningCode},
		}},
	}
}

func (s owningDoctorCheckSource) CallDoctorCheck(context.Context, string, string) (*rpc.DoctorCheckOutput, error) {
	if s.fail {
		return nil, errors.New("fixture-owning-connection-lost")
	}
	return &rpc.DoctorCheckOutput{Diagnostics: []rpc.DoctorCheckDiagnostic{{
		Code: ownedWarningCode, Severity: "warning", Message: ownedCheckMessage,
	}}}, nil
}

// TestShowDoctorListsAPluginOwnedWarningOnce raises a warning on the report
// bus under the code a running plugin's runtime check declares, then answers
// show doctor twice: once with the check answering, once with its callback
// failing.
//
// VALIDATES: a fault a plugin's runtime check reports is one row: the
// recent-events check leaves the bus warning under that code to the plugin
// check that answered, as it does for an in-tree runtime check. When the
// plugin check does not answer, the warning still renders through
// recent-events, so the fault is never hidden.
// PREVENTS: show doctor listing one plugin fault twice (its runtime check and
// recent-events), because only in-tree runtime checks' codes were consulted.
func TestShowDoctorListsAPluginOwnedWarningOnce(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	report.RaiseWarning("fixture-owning-plugin", ownedWarningCode, "unit-1", ownedWarningMessage)

	busRows, checkRows := showDoctorRows(t, owningDoctorCheckSource{})
	require.Equal(t, 1, checkRows, "the answering plugin check's finding is listed %d times", checkRows)
	require.Equal(t, 0, busRows, "recent-events repeated a warning the answering plugin check owns")

	busRows, checkRows = showDoctorRows(t, owningDoctorCheckSource{fail: true})
	require.Equal(t, 0, checkRows, "a failed plugin check produced its finding")
	require.Equal(t, 1, busRows, "the warning of a plugin check that did not answer is hidden")
}

// showDoctorRows answers show doctor over source and counts the rows that
// carry the bus warning and the rows that carry the plugin check's finding.
func showDoctorRows(t *testing.T, source doctorCheckSource) (busRows, checkRows int) {
	t.Helper()
	resp, err := answerDaemonDoctor(source, "", commandtest.Args())
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, resp.Status)
	got, ok := resp.Data.(plugin.RawJSON)
	require.True(t, ok, "daemon data is %T, want the encoded producer answer", resp.Data)

	var result diagnostic.DoctorResult
	require.NoError(t, json.Unmarshal([]byte(got), &result))
	for i := range result.Diagnostics {
		if strings.Contains(result.Diagnostics[i].Message, ownedWarningMessage) {
			busRows++
		}
		if strings.Contains(result.Diagnostics[i].Message, ownedCheckMessage) {
			checkRows++
		}
	}
	return busRows, checkRows
}
