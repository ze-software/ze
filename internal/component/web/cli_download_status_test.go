// Design: docs/architecture/web-interface.md -- authoritative finite output completion.
package web

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
)

// TestWebCLIDownloadCompletionUsesProducerOutcome distinguishes a successful
// literal beginning with error-like words from actual dispatcher/pipe failures.
func TestWebCLIDownloadCompletionUsesProducerOutcome(t *testing.T) {
	literal, err := json.Marshal(command.Blob("error: a successful diagnostic document\n"))
	require.NoError(t, err)
	for _, tc := range []struct {
		name     string
		line     string
		response *plugin.Response
		failure  error
		complete bool
	}{
		{name: "literal-error-prefix", line: "run show download literal | text", response: plugin.NewResponse(plugin.StatusDone, plugin.RawJSON(literal)), complete: true},
		{name: "warning-success", line: "run show download warning | json compact", response: &plugin.Response{Status: plugin.StatusWarning, Data: plugin.Map{"answer": "kept"}, Warnings: []string{"warning: useful detail"}}, complete: true},
		{name: "dispatcher-error", line: "run show download failure", failure: errors.New("producer failed")},
		{name: "response-error", line: "run show download denied", response: &plugin.Response{Status: plugin.StatusError, Error: plugin.UnauthorizedMessage}},
		{name: "pipe-application-error", line: "run show download document | count", response: plugin.NewResponse(plugin.StatusDone, plugin.Map{"answer": "document with no rows"})},
		{name: "pipe-validation-error", line: "run show download document | save forbidden-daemon-path", response: plugin.NewResponse(plugin.StatusDone, plugin.Map{"answer": "kept"})},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mgr, schema, tree, _ := setupCLITerminalYANGTest(t)
			calls := 0
			handler := HandleCLITerminalWithDispatchAuthorizerAndAudit(mgr, schema, tree, func(context.Context, plugin.CallerIdentity, string) (*plugin.Response, error) {
				calls++
				return tc.response, tc.failure
			}, nil, nil)
			rec := httptest.NewRecorder()
			handler(rec, authedRequest(t, http.MethodPost, "/cli/terminal", url.Values{"command": {tc.line}}))
			require.Equal(t, http.StatusOK, rec.Code)
			var answer struct {
				Output   string `json:"output"`
				Complete bool   `json:"output-complete"`
			}
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &answer))
			require.Equal(t, tc.complete, answer.Complete, answer.Output)
			if tc.name == "literal-error-prefix" {
				require.True(t, strings.HasPrefix(answer.Output, "error:"))
			}
			if tc.name == "pipe-validation-error" {
				require.Zero(t, calls, "daemon save refusal must precede dispatch")
				return
			}
			require.Equal(t, 1, calls)
		})
	}
}

// TestWebCLIDownloadEditorCompletionUsesReturnedError checks the shared editor
// result instead of deriving success from rendered feedback words.
func TestWebCLIDownloadEditorCompletionUsesReturnedError(t *testing.T) {
	mgr, schema, tree, _ := setupCLITerminalYANGTest(t)
	handler := HandleCLITerminalWithDispatchAuthorizerAndAudit(mgr, schema, tree, nil, nil, nil)
	for _, tc := range []struct {
		line     string
		complete bool
	}{
		{line: "show", complete: true},
		{line: "show | no-such-pipe"},
		{line: "load terminal absolute merge"},
		{line: "restart"},
	} {
		t.Run(tc.line, func(t *testing.T) {
			rec := httptest.NewRecorder()
			handler(rec, authedRequest(t, http.MethodPost, "/cli/terminal", url.Values{"command": {tc.line}}))
			require.Equal(t, http.StatusOK, rec.Code)
			var answer struct {
				Complete bool `json:"output-complete"`
			}
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &answer))
			require.Equal(t, tc.complete, answer.Complete, rec.Body.String())
		})
	}
}
