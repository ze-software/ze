package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
)

// warningDispatcher answers every command with a payload and the two warning
// lines `show config dump` states about a config it still resolved.
func warningDispatcher() CommandDispatcher {
	return func(_ context.Context, _ plugin.CallerIdentity, _ string) (*plugin.Response, error) {
		return &plugin.Response{
			Status:   plugin.StatusWarning,
			Data:     plugin.Map{"answer": "kept"},
			Warnings: []string{"warning: first line", "warning: second line"},
		}, nil
	}
}

// TestAdminCommandShowsTheWarning runs a command from the web admin, as JSON
// and as the HTMX result card, against an answer that warns.
//
// VALIDATES: the JSON result carries "warnings" beside the bare output, and
// the card shows the warning above the answer, where `ze <command>` prints it
// first on stderr.
// PREVENTS: the web admin dropping a warning the other surfaces show.
func TestAdminCommandShowsTheWarning(t *testing.T) {
	renderer, err := NewRenderer()
	require.NoError(t, err)
	handler := HandleAdminExecute(renderer, warningDispatcher(), StaticCommandTree(nil))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/admin/peer/192.168.1.1/teardown?format=json", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var data struct {
		Output   string   `json:"output"`
		Warnings []string `json:"warnings"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &data))
	assert.JSONEq(t, `{"answer":"kept"}`, data.Output, "the output stays the bare answer")
	assert.Equal(t, []string{"warning: first line", "warning: second line"}, data.Warnings)

	req = httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/admin/peer/192.168.1.1/teardown", http.NoBody)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	first := strings.Index(body, "warning: first line")
	second := strings.Index(body, "warning: second line")
	answer := strings.Index(body, "answer")
	require.GreaterOrEqual(t, first, 0, "the card must show the first warning line: %s", body)
	require.GreaterOrEqual(t, second, 0, "the card must show the second warning line: %s", body)
	assert.Less(t, second, answer, "the warning reads above the answer")
}

// TestTerminalOperationalShowsTheWarning runs a command from the web CLI
// terminal against an answer that warns.
//
// VALIDATES: the terminal prints the warning lines, then the answer.
// PREVENTS: the web terminal dropping a warning the SSH session shows.
func TestTerminalOperationalShowsTheWarning(t *testing.T) {
	output, rendered := executeTerminalOperational(t.Context(), warningDispatcher(), "admin", "127.0.0.1:1", "show config dump | json compact")
	require.NotNil(t, rendered)
	assert.Equal(t, "warning: first line\nwarning: second line\n{\"answer\":\"kept\"}", strings.TrimRight(output, "\n"))
}
