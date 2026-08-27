package web

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ze-software/ze/internal/component/plugin"
)

// TestWebTerminalRefusesSaveAtTheEntryPoint drives the authenticated
// POST /cli/terminal handler in operational mode. Web expands the chain in the
// daemon process, so the pipe must fail before command dispatch.
//
// VALIDATES: IR-18 -- remote web save answers an error naming save and writes nothing.
// PREVENTS: the web terminal using the local-allowed pipe validator.
func TestWebTerminalRefusesSaveAtTheEntryPoint(t *testing.T) {
	manager, schema, tree, _ := setupCLITerminalYANGTest(t)
	var dispatched atomic.Bool
	dispatch := func(context.Context, plugin.CallerIdentity, string) (*plugin.Response, error) {
		dispatched.Store(true)
		return plugin.NewResponse(plugin.StatusDone, plugin.RawJSON(`{"version":"test"}`)), nil
	}
	handler := HandleCLITerminalWithDispatchAuthorizerAndAudit(
		manager, schema, tree, dispatch, nil, nil,
	)
	path := filepath.Join(t.TempDir(), "web-save.json")

	response := runTerminalCommandForm(t, handler, url.Values{
		"command": {"show version | save " + path},
		"mode":    {"operational"},
	})

	assert.Contains(t, response.Output, "pipe error:")
	assert.Contains(t, response.Output, "save")
	assert.Contains(t, response.Output, "refused")
	assert.False(t, dispatched.Load(), "a refused save must not reach the command dispatcher")
	_, statErr := os.Stat(path)
	assert.ErrorIs(t, statErr, os.ErrNotExist)
}
