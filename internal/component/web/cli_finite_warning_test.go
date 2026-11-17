// Design: docs/architecture/web-interface.md -- warnings stay outside finite exports.
package web

import (
	"bytes"
	"context"
	"encoding/json"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
)

// TestWebCLIFiniteWarningKeepsSelectedBytes checks the real HTTP producer in
// both terminal modes. An advisory stays visible while the download contains
// only the selected Blob rendering, including invalid UTF-8 and explicit JSON.
func TestWebCLIFiniteWarningKeepsSelectedBytes(t *testing.T) {
	original := []byte("WARNING-FIRST\xff\x00\xc3(\nWARNING-LAST")
	wire, err := json.Marshal(command.Blob(original))
	require.NoError(t, err)
	var document map[string]any
	require.NoError(t, json.Unmarshal(wire, &document))
	compact, err := json.Marshal(document)
	require.NoError(t, err)
	warnings := []string{"warning: the snapshot excludes expired records", "run show data cat meta/finite-warning-blob"}
	for _, mode := range []struct {
		name   string
		value  string
		prefix string
	}{
		{name: "configuration", value: terminalModeConfig, prefix: "run "},
		{name: "operational", value: terminalModeOperational},
	} {
		for _, format := range []struct {
			name    string
			pipe    string
			want    []byte
			display string
		}{
			{name: "default", want: original, display: "WARNING-LAST"},
			{name: "text", pipe: " | text", want: original, display: "WARNING-LAST"},
			{name: "json", pipe: " | json compact", want: compact, display: string(compact)},
		} {
			t.Run(mode.name+"/"+format.name, func(t *testing.T) {
				mgr, schema, tree, _ := setupCLITerminalYANGTest(t)
				calls := 0
				handler := HandleCLITerminalWithDispatchAuthorizerAndAudit(mgr, schema, tree, func(_ context.Context, caller plugin.CallerIdentity, input string) (*plugin.Response, error) {
					calls++
					require.Equal(t, "testuser", caller.Username)
					require.Equal(t, "show data cat meta/finite-warning-blob", input)
					response := plugin.NewResponse(plugin.StatusWarning, plugin.RawJSON(wire))
					response.Warnings = warnings
					return response, nil
				}, nil, nil)
				answer := runTerminalCommandForm(t, handler, url.Values{
					"mode":    {mode.value},
					"command": {mode.prefix + "show data cat meta/finite-warning-blob" + format.pipe},
				})
				require.True(t, answer.OutputComplete, answer.Output)
				require.True(t, strings.HasPrefix(answer.Output, strings.Join(warnings, "\n")+"\n"), "the advisory must remain above the answer: %q", answer.Output)
				require.Contains(t, answer.Output, format.display)
				require.NotEmpty(t, answer.OutputBytes)
				var carrier struct {
					Blob json.RawMessage `json:"blob"`
				}
				require.NoError(t, json.Unmarshal(answer.OutputBytes, &carrier))
				exported, err := command.DecodeBlob(carrier.Blob)
				require.NoError(t, err)
				require.Equal(t, format.want, []byte(exported), "display warnings are not selected stdout")
				require.Equal(t, len(format.want), answer.OutputByteCount)
				require.Equal(t, 1, calls, "the carrier must reuse the original answer")
			})
		}
	}
}

// TestWebCLIFiniteWarningDoesNotConsumeCarrierBudget keeps an exact-limit
// successful answer downloadable even when its separately displayed advisory
// makes the rendered text larger than the finite carrier limit.
func TestWebCLIFiniteWarningDoesNotConsumeCarrierBudget(t *testing.T) {
	original := bytes.Repeat([]byte("x"), cliStreamOutputBytesMax)
	original[0] = 0xff
	wire, err := json.Marshal(command.Blob(original))
	require.NoError(t, err)
	mgr, schema, tree, _ := setupCLITerminalYANGTest(t)
	handler := HandleCLITerminalWithDispatchAuthorizerAndAudit(mgr, schema, tree, func(context.Context, plugin.CallerIdentity, string) (*plugin.Response, error) {
		response := plugin.NewResponse(plugin.StatusWarning, plugin.RawJSON(wire))
		response.Warnings = []string{"warning: the snapshot excludes expired records"}
		return response, nil
	}, nil, nil)
	answer := runTerminalCommandForm(t, handler, url.Values{
		"command": {"run show data cat meta/finite-warning-bound | text"},
	})
	require.True(t, answer.OutputComplete)
	require.Contains(t, answer.Output, "warning: the snapshot excludes expired records")
	require.NotEmpty(t, answer.OutputBytes, "the selected output fits the complete-result budget")
	var carrier struct {
		Blob json.RawMessage `json:"blob"`
	}
	require.NoError(t, json.Unmarshal(answer.OutputBytes, &carrier))
	exported, err := command.DecodeBlob(carrier.Blob)
	require.NoError(t, err)
	require.Equal(t, original, []byte(exported))
	require.Equal(t, len(original), answer.OutputByteCount)
}

// TestWebCLIFiniteEmptyWarningKeepsEmptyArtifact distinguishes a successful
// empty selected answer from an absent artifact while its advisory stays visible.
func TestWebCLIFiniteEmptyWarningKeepsEmptyArtifact(t *testing.T) {
	wire, err := json.Marshal(command.Blob(nil))
	require.NoError(t, err)
	mgr, schema, tree, _ := setupCLITerminalYANGTest(t)
	handler := HandleCLITerminalWithDispatchAuthorizerAndAudit(mgr, schema, tree, func(context.Context, plugin.CallerIdentity, string) (*plugin.Response, error) {
		response := plugin.NewResponse(plugin.StatusWarning, plugin.RawJSON(wire))
		response.Warnings = []string{"warning: the snapshot has no current records"}
		return response, nil
	}, nil, nil)
	answer := runTerminalCommandForm(t, handler, url.Values{
		"command": {"run show data cat meta/finite-warning-empty | text"},
	})
	require.True(t, answer.OutputComplete)
	require.Contains(t, answer.Output, "warning: the snapshot has no current records")
	require.NotEmpty(t, answer.OutputBytes, "an empty successful answer still needs its carrier")
	var carrier struct {
		Blob json.RawMessage `json:"blob"`
	}
	require.NoError(t, json.Unmarshal(answer.OutputBytes, &carrier))
	exported, err := command.DecodeBlob(carrier.Blob)
	require.NoError(t, err)
	require.Empty(t, exported, "the warning must not replace the empty selected answer")
	require.Zero(t, answer.OutputByteCount)
}
