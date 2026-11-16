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

func executeTerminalOperationalForUser(ctx context.Context, mgr *EditorManager, dispatch CommandDispatcher, username, remoteAddr, input string) (string, *plugin.RenderedResponse, bool) {
	if _, err := mgr.GetOrCreate(username); err != nil {
		return "error: " + err.Error(), nil, false
	}
	output, rendered, complete, _ := executeTerminalOperationalResult(ctx, dispatch, username, remoteAddr, input, mgr.currentCLIFormat(username)) //nolint:errcheck // Failure is already represented in Output and complete.
	return output, rendered, complete
}

// currentCLIFormat reads an existing user's preference without opening an
// editing session. An absent preference inherits the configured operational
// format, represented by the empty string in the shared pipe processor.
func (m *EditorManager) currentCLIFormat(username string) string {
	m.mu.RLock()
	us := m.sessions[username]
	m.mu.RUnlock()
	if us == nil {
		return ""
	}
	us.mu.Lock()
	defer us.mu.Unlock()
	return us.cliFormat
}
