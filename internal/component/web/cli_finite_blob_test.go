// Design: docs/architecture/web-interface.md -- byte-exact finite result exports.
package web

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	_ "github.com/ze-software/ze/internal/component/config/storage/cli" // Register the finite Blob command and its answer shape.
	"github.com/ze-software/ze/internal/component/plugin"
)

// TestWebCLIFiniteBlobPreservesOriginalBytes observes the actual HTTP envelope
// after a finite Blob goes through the shared formatter. Display JSON may
// replace invalid UTF-8; the export must retain the selected format's bytes.
func TestWebCLIFiniteBlobPreservesOriginalBytes(t *testing.T) {
	original := []byte("FINITE-FIRST\xff\x00\xc3(\nFINITE-LAST")
	wire, err := json.Marshal(command.Blob(original))
	require.NoError(t, err)
	var document map[string]any
	require.NoError(t, json.Unmarshal(wire, &document))
	compact, err := json.Marshal(document)
	require.NoError(t, err)
	for _, tc := range []struct {
		name    string
		pipe    string
		warning string
		want    []byte
	}{
		{name: "default", want: original},
		{name: "text", pipe: " | text", want: original},
		{name: "json", pipe: " | json compact", want: compact},
		{name: "warning", pipe: " | text", warning: "warning: retained detail", want: original},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mgr, schema, tree, _ := setupCLITerminalYANGTest(t)
			calls := 0
			handler := HandleCLITerminalWithDispatchAuthorizerAndAudit(mgr, schema, tree, func(_ context.Context, caller plugin.CallerIdentity, input string) (*plugin.Response, error) {
				calls++
				require.Equal(t, "testuser", caller.Username)
				require.Equal(t, "show data cat meta/finite-blob", input)
				response := plugin.NewResponse(plugin.StatusDone, plugin.RawJSON(wire))
				if tc.warning != "" {
					response.Status = plugin.StatusWarning
					response.Warnings = []string{tc.warning}
				}
				return response, nil
			}, nil, nil)
			recorder := httptest.NewRecorder()
			handler(recorder, authedRequest(t, http.MethodPost, "/cli/terminal", url.Values{
				"command": {"run show data cat meta/finite-blob" + tc.pipe},
			}))
			require.Equal(t, http.StatusOK, recorder.Code)
			var answer struct {
				Output   string          `json:"output"`
				Bytes    json.RawMessage `json:"output-bytes"`
				Complete bool            `json:"output-complete"`
			}
			require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &answer))
			require.True(t, answer.Complete, answer.Output)
			if tc.warning != "" {
				require.Contains(t, answer.Output, tc.warning)
			}
			require.NotEmpty(t, answer.Bytes, "a successful finite result needs its original formatted bytes")
			var carrier struct {
				Blob json.RawMessage `json:"blob"`
			}
			require.NoError(t, json.Unmarshal(answer.Bytes, &carrier))
			exported, err := command.DecodeBlob(carrier.Blob)
			require.NoError(t, err)
			require.Equal(t, tc.want, []byte(exported))
			require.Equal(t, 1, calls)
		})
	}
}

// TestWebCLIFiniteBlobFailureWithholdsExport checks the HTTP completion gate
// while a failed producer still carries useful diagnostic data.
func TestWebCLIFiniteBlobFailureWithholdsExport(t *testing.T) {
	wire, err := json.Marshal(command.Blob("partial\xff"))
	require.NoError(t, err)
	mgr, schema, tree, _ := setupCLITerminalYANGTest(t)
	handler := HandleCLITerminalWithDispatchAuthorizerAndAudit(mgr, schema, tree, func(context.Context, plugin.CallerIdentity, string) (*plugin.Response, error) {
		return plugin.NewResponse(plugin.StatusError, plugin.RawJSON(wire)), errors.New("finite producer failed")
	}, nil, nil)
	recorder := httptest.NewRecorder()
	handler(recorder, authedRequest(t, http.MethodPost, "/cli/terminal", url.Values{
		"command": {"run show data cat meta/finite-blob | text"},
	}))
	var answer struct {
		Bytes    json.RawMessage `json:"output-bytes"`
		Complete bool            `json:"output-complete"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &answer))
	require.False(t, answer.Complete)
	require.Empty(t, answer.Bytes, "failed data cannot become a complete export")
}
