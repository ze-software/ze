// Design: docs/architecture/web-interface.md -- shared CLI session preferences
package web

import (
	"context"

	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/plugin"
)

// sessionFormatCommand changes only this authenticated user's local preference.
// The existing session lock serializes concurrent requests for the same user.
func (m *EditorManager) sessionFormatCommand(username, input string) (string, error) {
	us, err := m.GetOrCreate(username)
	if err != nil {
		return "", err
	}
	us.mu.Lock()
	defer us.mu.Unlock()
	answer, _, err := cli.ExecuteSessionFormat(m.schema, input, us.cliFormat)
	if err != nil {
		return "", err
	}
	us.cliFormat = answer.Format
	return answer.Output, nil
}

func executeTerminalOperationalForUser(ctx context.Context, mgr *EditorManager, dispatch CommandDispatcher, username, remoteAddr, input string) (string, *plugin.RenderedResponse) {
	us, err := mgr.GetOrCreate(username)
	if err != nil {
		return "error: " + err.Error(), nil
	}
	us.mu.Lock()
	format := us.cliFormat
	us.mu.Unlock()
	return executeTerminalOperational(ctx, dispatch, username, remoteAddr, input, format)
}
