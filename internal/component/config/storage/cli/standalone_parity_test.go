// Design: docs/architecture/storage-backends.md -- standalone command parity.
package cli

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/commandtest"
	"github.com/ze-software/ze/internal/component/config/storage"
	configyang "github.com/ze-software/ze/internal/component/config/yang"
	zePlugin "github.com/ze-software/ze/internal/component/plugin"
	_ "github.com/ze-software/ze/internal/plugins/config-storage/yang" // owner command model
	"github.com/ze-software/ze/pkg/zefs"
)

// TestStorageInspectionSelectorsDeclared binds the selectors generated forms
// send against the shipped model, rather than passing untyped tokens through.
func TestStorageInspectionSelectorsDeclared(t *testing.T) {
	model, err := configyang.DefaultLoader()
	require.NoError(t, err)
	defs := configyang.PathToArgDefs(model)
	for _, selector := range []struct {
		path, name, value string
	}{
		{"show data list", "prefix", "meta/"},
		{"show data registered", "pattern", zefs.KeyInstanceName.Pattern},
	} {
		args, err := command.ValidateArgs([]string{selector.name, selector.value}, defs[selector.path], nil)
		require.NoError(t, err)
		got, found := args.Keyed(selector.name)
		assert.True(t, found, "%s must declare %s", selector.path, selector.name)
		assert.Equal(t, selector.value, got)
	}
}

// TestRegisteredPatternRefusalMatchesStandalone proves an unknown pattern is
// the same actionable refusal on the standalone and registered data routes.
func TestRegisteredPatternRefusalMatchesStandalone(t *testing.T) {
	const unknown = "unknown/storage/pattern"
	code, _, diagnostic := captureCatStdio(t, func() int { return cmdRegistered("", []string{unknown}) })
	answer := dataRegistered(commandtest.Args(unknown))
	require.NotZero(t, code)
	assert.Equal(t, code, answer.Code)
	assert.Equal(t, diagnostic, answer.Diagnostic)
	assert.Contains(t, answer.Diagnostic, "show data registered")
	assert.Nil(t, answer.Payload)
}

// TestRegisteredPatternKeyedFormReturnsKnownPattern retains useful output
// while checking the keyword grammar supplied by generated command forms.
func TestRegisteredPatternKeyedFormReturnsKnownPattern(t *testing.T) {
	model, err := configyang.DefaultLoader()
	require.NoError(t, err)
	args, err := command.ValidateArgs([]string{"pattern", zefs.KeyInstanceName.Pattern},
		configyang.PathToArgDefs(model)["show data registered"], nil)
	require.NoError(t, err)
	answer := command.AnswerRemote(dataRegistered, args)
	require.Zero(t, answer.Code, answer.Diagnostic)
	raw, err := json.Marshal(answer.Payload)
	require.NoError(t, err)
	var document struct {
		Patterns []struct {
			Pattern string `json:"pattern"`
		} `json:"patterns"`
	}
	require.NoError(t, json.Unmarshal(raw, &document))
	require.Len(t, document.Patterns, 1)
	assert.Equal(t, zefs.KeyInstanceName.Pattern, document.Patterns[0].Pattern)
}

// TestRepairAnswerRetainsUnresolvedErrors exercises the result producer with
// a pointer repair cannot choose a replacement for, and an innocent orphan.
func TestRepairAnswerRetainsUnresolvedErrors(t *testing.T) {
	report := &zefs.RepairReport{Recovered: []string{"meta/test/kept"}, RecoveredCount: 1}
	for _, finding := range []storage.HistoryFinding{
		{Kind: storage.HistoryDanglingPointer, Key: "meta/config/router.conf/active",
			Severity: storage.HistorySeverityError, Detail: "active version does not resolve"},
		{Kind: storage.HistoryOrphanObject, Key: "object/orphan",
			Severity: storage.HistorySeverityWarning, Detail: "no history entry names it"},
	} {
		response := repairAnswer("/tmp/ze-salvage", report, []storage.HistoryFinding{finding})
		data, ok := response.Data.(zePlugin.Map)
		require.True(t, ok)
		assert.Equal(t, []map[string]any{{keywordKey: "meta/test/kept"}}, data["recovered-keys"])
		rows, ok := data["history"].([]map[string]any)
		require.True(t, ok)
		require.Len(t, rows, 1)
		if finding.Severity == storage.HistorySeverityError {
			assert.Equal(t, zePlugin.StatusError, response.Status)
			assert.Contains(t, response.Error, "ze data --path /tmp/ze-salvage check")
			assert.Equal(t, "error", rows[0][findingSeverity])
		} else {
			assert.Equal(t, zePlugin.StatusDone, response.Status, response.Error)
			assert.Equal(t, "warning", rows[0][findingSeverity])
		}
	}
}

// TestRepairRPCUnresolvedHistoryRefusesSuccess salvages a real store whose
// active pointer names a missing version. Neither route may call it repaired.
func TestRepairRPCUnresolvedHistoryRefusesSuccess(t *testing.T) {
	store, configPath := dataRPCStore(t, func(context.Context) error { return nil })
	const pointer = "meta/config/other.conf/active"
	require.NoError(t, store.WriteKey(pointer, []byte("20261117T120000.000000000Z")))
	output := filepath.Join(t.TempDir(), "daemon-salvage")
	response := repairCall(t, "path", output)
	assert.Equal(t, zePlugin.StatusError, response.Status, response.Error)
	assert.Contains(t, response.Error, "ze data --path "+output+" check")
	localOutput := filepath.Join(t.TempDir(), "local-salvage")
	code, _, diagnostic := captureCatStdio(t, func() int {
		return cmdRepair(filepath.Join(filepath.Dir(configPath), storage.TreeName), []string{"--output", localOutput})
	})
	assert.Equal(t, 1, code)
	assert.Contains(t, diagnostic, pointer)
}

// TestRepairAnswerKeepsDroppedHistoryWarning keeps a useful partial salvage
// as a warning while preserving the original finding's error severity.
func TestRepairAnswerKeepsDroppedHistoryWarning(t *testing.T) {
	report := &zefs.RepairReport{Recovered: []string{"meta/test/kept"}, RecoveredCount: 1}
	finding := storage.HistoryFinding{Kind: storage.HistoryMalformedEntry,
		Key: "file/version/bad/router.conf", Severity: storage.HistorySeverityError, Detail: "invalid history entry"}
	response := repairAnswer("/tmp/ze-salvage", report, []storage.HistoryFinding{finding})
	assert.Equal(t, zePlugin.StatusWarning, response.Status)
	assert.Empty(t, response.Error)
	require.NotEmpty(t, response.Warnings)
	data, ok := response.Data.(zePlugin.Map)
	require.True(t, ok)
	rows, ok := data["history"].([]map[string]any)
	require.True(t, ok)
	require.Len(t, rows, 1)
	assert.Equal(t, "dropped", rows[0]["action"])
	assert.Equal(t, "error", rows[0][findingSeverity])
}

// TestDataEncodeRegisteredMatchesStandalone reaches the owner registration
// and compares binary frame, header and CRC output against the local command.
func TestDataEncodeRegisteredMatchesStandalone(t *testing.T) {
	const data = "bytes to encode"
	handler, rest := command.LookupLocalData(strings.Fields("show data encode"))
	require.NotNil(t, handler, "the encoder must have a daemon-served owner registration")
	require.Empty(t, rest)
	model, err := configyang.DefaultLoader()
	require.NoError(t, err)
	defs := configyang.PathToArgDefs(model)["show data encode"]
	require.NotEmpty(t, defs, "the encoder input must be declared by its owner")
	for _, part := range []string{"full", "header", "crc"} {
		local := []string{"--cap", "32", data}
		if part != "full" {
			local = append([]string{"--" + part}, local...)
		}
		code, output, diagnostic := captureCatStdio(t, func() int { return cmdEncode("", local) })
		require.Zero(t, code, diagnostic)
		args, err := command.ValidateArgs([]string{"content", base64.StdEncoding.EncodeToString([]byte(data)),
			"part", part, "capacity", "32"}, defs, nil)
		require.NoError(t, err)
		answer := command.AnswerRemote(handler, args)
		require.Zero(t, answer.Code, answer.Diagnostic)
		blob, ok := answer.Payload.(command.Blob)
		require.True(t, ok, "encoded bytes must preserve the blob answer contract")
		assert.Equal(t, output, string(blob), part)
	}
}
