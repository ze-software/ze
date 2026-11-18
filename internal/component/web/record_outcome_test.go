// Design: docs/architecture/web-interface.md -- failed command progress.
package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// The web terminal must use the failed record walk's retained rendering once.
// Re-marshaling its consumed Records walks it again and loses its real output.
func TestWebRecordOutcomeTerminalKeepsProgressWithoutSecondWalk(t *testing.T) {
	dispatch, walks := webRecordOutcomeDispatch(t)
	output, rendered, selected, complete, err := executeTerminalOperationalResult(t.Context(), dispatch, "alice", "127.0.0.1", "show cleanup", "json")
	require.Error(t, err)
	defer rendered.TransportComplete()
	assert.Nil(t, selected)
	assert.False(t, complete)
	assert.Contains(t, output, "complete-native")
	assert.Contains(t, output, "missing-native-ack")
	assert.Contains(t, output, "partial")
	assert.Equal(t, 1, *walks, "terminal re-entered the producer after ResponseJSON consumed it")
}

// A tool page can show both failure and structured progress. Neither one may
// replace the other in its separate error and output slots.
func TestWebRecordOutcomeToolPageKeepsFailureAndProgress(t *testing.T) {
	dispatch, walks := webRecordOutcomeDispatch(t)
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/show/tools/ping/", nil)
	data := dispatchToolCommand(request, dispatch, "show cleanup")
	assert.Contains(t, data.Error, "partial")
	assert.NotContains(t, data.Error, "complete-native", "the payload replaced the failure explanation")
	assert.Contains(t, data.Output, "complete-native")
	assert.Contains(t, data.Output, "missing-native-ack")
	assert.Equal(t, 1, *walks)
}

// A resolved related tool remains an error overlay when it partially failed,
// but the already rendered rows and their independent causes remain visible.
func TestWebRecordOutcomeRelatedToolKeepsFailureAndProgress(t *testing.T) {
	schema, err := config.YANGSchema()
	require.NoError(t, err)
	tree := buildPeerTree(t, "thomas", "10.0.0.1")
	dispatch, walks := webRecordOutcomeDispatch(t)
	handler := HandleRelatedToolRun(testRenderer(t), schema, tree, nil, dispatch)
	form := url.Values{"tool_id": {"peer-detail"}, "context_path": {"bgp/peer/thomas"}}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, toolsRequest(t, "/tools/related/run", form, "alice"))
	assert.Equal(t, http.StatusOK, recorder.Code)
	body := recorder.Body.String()
	assert.Contains(t, body, "tool-overlay-error-message")
	assert.Contains(t, body, "partial")
	assert.Contains(t, body, "complete-native")
	assert.Contains(t, body, "missing-native-ack")
	assert.Equal(t, 1, *walks)
}

// This modeled generator publishes the real generic completion API only when
// the walk ends. A second walk yields no data, matching one-shot RPC answers.
func webRecordOutcomeDispatch(t *testing.T) (CommandDispatcher, *int) {
	t.Helper()
	walks := 0
	response := plugin.NewResponse(plugin.StatusDone, nil)
	response.Data = plugin.Records{Key: "cleanup", Rows: func(yield func(rpc.RowRecord) bool) {
		walks++
		if walks != 1 {
			return
		}
		item := rpc.RawRow(`{"phase":"complete-native"}`)
		if !yield(rpc.RowRecord{Item: &item}) {
			return
		}
		fault := rpc.RawRow(`{"cause":"missing-native-ack"}`)
		if !yield(rpc.RowRecord{Fault: &fault}) {
			return
		}
		require.NoError(t, response.CompleteRecordAnswer(&rpc.ExecuteCommandRecordAnswer{Count: 1, Faults: 1}))
	}}
	return func(context.Context, plugin.CallerIdentity, string) (*plugin.Response, error) { return response, nil }, &walks
}

// Authorization failures must never expose an attached diagnostic payload.
// Pair the existing terminal guard with both generic tool entry points.
func TestWebRecordOutcomePermissionDenialKeepsPayloadPrivate(t *testing.T) {
	denied := plugin.UnauthorizedMessage
	dispatch := CommandDispatcher(func(context.Context, plugin.CallerIdentity, string) (*plugin.Response, error) {
		return &plugin.Response{Status: plugin.StatusError, Error: denied, Data: plugin.Map{"private": "sensitive-native-state"}}, pluginserver.ErrUnauthorized
	})
	output, rendered, _, _, err := executeTerminalOperationalResult(t.Context(), dispatch, "alice", "127.0.0.1", "show cleanup", "json")
	require.Error(t, err)
	defer rendered.TransportComplete()
	assert.NotContains(t, output, "sensitive-native-state")
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/show/tools/ping/", nil)
	data := dispatchToolCommand(request, dispatch, "show cleanup")
	assert.Equal(t, denied, data.Error)
	assert.Empty(t, data.Output)
	assert.NotContains(t, strings.Join([]string{data.Error, data.Output}, "\n"), "sensitive-native-state")
}

// Both generic tool entry points preserve warning lines beside the data they
// display. Warning text must not replace the successful answer or disappear.
func TestWebRecordOutcomeToolsKeepWarningLines(t *testing.T) {
	const warning = "retry the retained cleanup"
	dispatch := CommandDispatcher(func(context.Context, plugin.CallerIdentity, string) (*plugin.Response, error) {
		return &plugin.Response{
			Status:   plugin.StatusWarning,
			Data:     plugin.Map{"phase": "complete-native"},
			Warnings: []string{warning},
		}, nil
	})
	t.Run("page", func(t *testing.T) {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/show/tools/ping/", nil)
		data := dispatchToolCommand(request, dispatch, "show cleanup")
		assert.Empty(t, data.Error)
		assert.Contains(t, data.Output, "complete-native")
		assert.Contains(t, data.Output, warning)
	})
	t.Run("related overlay", func(t *testing.T) {
		schema, err := config.YANGSchema()
		require.NoError(t, err)
		tree := buildPeerTree(t, "thomas", "10.0.0.1")
		handler := HandleRelatedToolRun(testRenderer(t), schema, tree, nil, dispatch)
		form := url.Values{"tool_id": {"peer-detail"}, "context_path": {"bgp/peer/thomas"}}
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, toolsRequest(t, "/tools/related/run", form, "alice"))
		assert.Equal(t, http.StatusOK, recorder.Code)
		body := recorder.Body.String()
		assert.Contains(t, body, "complete-native")
		assert.Contains(t, body, warning)
	})
}

// A completed warning with no diagnostic lines still has a visible verdict.
// Exercise each HTTP surface, pairing built and consumed record answers with
// healthy and explicit-warning controls. The label must not enter the export.
func TestWebRecordOutcomeWarningVerdictVisible(t *testing.T) {
	const warning = "retained owner needs cleanup"
	const failure = "native acknowledgement missing"
	for _, test := range []struct {
		name     string
		status   string
		record   bool
		warning  bool
		warnings []string
		failed   bool
		label    bool
	}{
		{name: "status nil lines", status: plugin.StatusWarning, label: true},
		{name: "status empty lines", status: plugin.StatusWarning, warnings: []string{}, label: true},
		{name: "record nil lines", record: true, warning: true, label: true},
		{name: "record empty lines", record: true, warning: true, warnings: []string{}, label: true},
		{name: "failed record warning", record: true, warning: true, failed: true, label: true},
		{name: "explicit status warning", status: plugin.StatusWarning, warnings: []string{warning}},
		{name: "explicit record warning", record: true, warning: true, warnings: []string{warning}},
		{name: "healthy status", status: plugin.StatusDone},
		{name: "healthy record", record: true},
	} {
		for _, route := range []string{"terminal", "page", "related"} {
			t.Run(test.name+"/"+route, func(t *testing.T) {
				dispatch := CommandDispatcher(func(context.Context, plugin.CallerIdentity, string) (*plugin.Response, error) {
					response := plugin.NewResponse(test.status, plugin.Map{"phase": "complete-native"})
					response.Warnings = test.warnings
					if test.record {
						response.Status = plugin.StatusDone
						response.Data = plugin.Records{Key: "cleanup", Rows: func(yield func(rpc.RowRecord) bool) {
							item := rpc.RawRow(`{"phase":"complete-native"}`)
							if !yield(rpc.RowRecord{Item: &item}) {
								return
							}
							metadata := &rpc.ExecuteCommandRecordAnswer{Count: 1, Warning: test.warning, Warnings: test.warnings}
							if test.failed {
								metadata.Message = failure
							}
							require.NoError(t, response.CompleteRecordAnswer(metadata))
						}}
					}
					return response, nil
				})
				var body string
				if route == "terminal" {
					mgr, schema, tree, _ := setupCLITerminalYANGTest(t)
					handler := HandleCLITerminalWithDispatchAuthorizerAndAudit(mgr, schema, tree, dispatch, nil, nil)
					answer := runTerminalCommandForm(t, handler, url.Values{
						"mode": {terminalModeOperational}, "command": {"show cleanup"},
					})
					body = answer.Output
					assert.Equal(t, !test.failed, answer.OutputComplete)
					assert.NotContains(t, string(answer.OutputBytes), "Warning")
					assert.NotContains(t, string(answer.OutputBytes), warning)
				} else {
					body = webAbsentRecordOutcomeBody(t, dispatch, route)
					marker := "wb-tool-error"
					if route == "related" {
						marker = "tool-overlay-error-message"
					}
					assert.Equal(t, test.failed, strings.Contains(body, marker))
				}
				assert.Contains(t, body, "complete-native")
				assert.Equal(t, test.failed, strings.Contains(body, failure))
				if test.label {
					assert.Equal(t, 1, strings.Count(body, "Warning\n"), "status-only warning must remain visible once")
				} else {
					assert.NotContains(t, body, "Warning\n")
				}
				for _, line := range test.warnings {
					assert.Equal(t, 1, strings.Count(body, line), "producer warning must remain visible once")
				}
			})
		}
	}
}

// The terminal must retain a permission failure without exposing diagnostic
// warnings attached to its refused answer, just as both tool handlers do.
func TestWebRecordOutcomeTerminalPermissionDenialKeepsWarningsPrivate(t *testing.T) {
	const warning = "private cleanup diagnostic"
	dispatch := CommandDispatcher(func(context.Context, plugin.CallerIdentity, string) (*plugin.Response, error) {
		return &plugin.Response{
			Status: plugin.StatusError, Error: plugin.UnauthorizedMessage, Warnings: []string{warning},
		}, pluginserver.ErrUnauthorized
	})
	mgr, schema, tree, _ := setupCLITerminalYANGTest(t)
	handler := HandleCLITerminalWithDispatchAuthorizerAndAudit(mgr, schema, tree, dispatch, nil, nil)
	answer := runTerminalCommandForm(t, handler, url.Values{
		"mode": {terminalModeOperational}, "command": {"show cleanup"},
	})
	assert.False(t, answer.OutputComplete)
	assert.Contains(t, answer.Output, plugin.UnauthorizedMessage)
	assert.NotContains(t, answer.Output, warning)
	assert.Empty(t, answer.OutputBytes)
}

// Public tool page and related-tool handlers must keep warnings beside a
// failed completed answer even when its original document was absent.
func TestWebRecordOutcomeAbsentDocumentKeepsOutcome(t *testing.T) {
	const failure = "evaluation stopped before the final acknowledgement"
	warnings := []string{"retained cleanup warning", "the original owner must retry cleanup"}
	for _, test := range []struct {
		name     string
		message  string
		warnings []string
		failed   bool
	}{
		{name: "failed with warnings", message: failure, warnings: warnings, failed: true},
		{name: "failed without warnings", message: failure, failed: true},
		{name: "warning only", warnings: warnings},
		{name: "healthy absent"},
	} {
		for _, route := range []string{"page", "related"} {
			t.Run(test.name+"/"+route, func(t *testing.T) {
				response := plugin.NewResponse(plugin.StatusDone, nil)
				require.NoError(t, response.CompleteRecordAnswer(&rpc.ExecuteCommandRecordAnswer{
					DocumentAbsent: true, Message: test.message, Warning: len(test.warnings) != 0, Warnings: test.warnings,
				}))
				dispatch := CommandDispatcher(func(context.Context, plugin.CallerIdentity, string) (*plugin.Response, error) {
					return response, nil
				})
				body := webAbsentRecordOutcomeBody(t, dispatch, route)
				if route == "page" {
					assert.Equal(t, test.failed, strings.Contains(body, "wb-tool-error"))
				} else {
					assert.Equal(t, test.failed, strings.Contains(body, "tool-overlay-error-message"))
				}
				if test.failed {
					assert.Equal(t, 1, strings.Count(body, failure))
				} else {
					assert.NotContains(t, body, failure)
				}
				for _, warning := range test.warnings {
					assert.Equal(t, 1, strings.Count(body, warning), "warning line must reach the operator once")
				}
				assert.NotContains(t, body, ">null<", "absent data became a public null document")
				assert.Nil(t, response.Data, "rendering manufactured a document")
			})
		}
	}
}

// Both public handlers must hide warning diagnostics on an authorization
// refusal, including an absent-document response carrying no payload to hide.
func TestWebRecordOutcomeAbsentPermissionDenialKeepsWarningsPrivate(t *testing.T) {
	for _, route := range []string{"page", "related"} {
		t.Run(route, func(t *testing.T) {
			dispatch := CommandDispatcher(func(context.Context, plugin.CallerIdentity, string) (*plugin.Response, error) {
				return &plugin.Response{
					Status: plugin.StatusError, Error: plugin.UnauthorizedMessage,
					Warnings: []string{"private cleanup diagnostic"},
				}, pluginserver.ErrUnauthorized
			})
			body := webAbsentRecordOutcomeBody(t, dispatch, route)
			assert.Contains(t, body, plugin.UnauthorizedMessage)
			assert.NotContains(t, body, "private cleanup diagnostic")
		})
	}
}

// Drive the registered HTTP handlers rather than testing only the shared
// command helper, so error and output slots must survive actual rendering.
func webAbsentRecordOutcomeBody(t *testing.T, dispatch CommandDispatcher, route string) string {
	t.Helper()
	if route == "page" {
		form := url.Values{"destination": {"192.0.2.1"}, "count": {"5"}, "timeout": {"5"}}
		recorder := httptest.NewRecorder()
		workbenchForTools(t, dispatch).ServeHTTP(recorder, toolsRequest(t, "/show/tools/ping/", form, "alice"))
		require.Equal(t, http.StatusOK, recorder.Code)
		return recorder.Body.String()
	}
	schema, err := config.YANGSchema()
	require.NoError(t, err)
	tree := buildPeerTree(t, "thomas", "10.0.0.1")
	handler := HandleRelatedToolRun(testRenderer(t), schema, tree, nil, dispatch)
	form := url.Values{"tool_id": {"peer-detail"}, "context_path": {"bgp/peer/thomas"}}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, toolsRequest(t, "/tools/related/run", form, "alice"))
	require.Equal(t, http.StatusOK, recorder.Code)
	return recorder.Body.String()
}
