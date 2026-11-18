// Design: docs/architecture/web-interface.md -- shared CLI mode-control grammar.
package web

import (
	"context"
	"errors"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
)

// TestWebCLIConfigureRejectsTrailingInput compares exact configure entry
// with malformed forms that must remain operational and reach normal refusal.
func TestWebCLIConfigureRejectsTrailingInput(t *testing.T) {
	for _, tc := range []struct {
		name string
		line string
		mode string
		want string
	}{
		{name: "operational-argument", line: "configure garbage", mode: terminalModeOperational, want: "configure garbage"},
		{name: "operational-pipe", line: "configure | json", mode: terminalModeOperational, want: "configure"},
		{name: "config-argument", line: "configure garbage", mode: terminalModeConfig},
		{name: "config-pipe", line: "configure | json", mode: terminalModeConfig},
		{name: "config-bare", line: "configure", mode: terminalModeConfig},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mgr, schema, tree, _ := setupCLITerminalYANGTest(t)
			var seen []string
			handler := HandleCLITerminalWithDispatchAuthorizerAndAudit(mgr, schema, tree, func(_ context.Context, _ plugin.CallerIdentity, input string) (*plugin.Response, error) {
				seen = append(seen, input)
				return nil, errors.New("configure input was refused")
			}, nil, nil)
			runTerminalCommandForm(t, handler, url.Values{
				"command": {"set bgp router-id 192.0.2.44"}, "mode": {terminalModeConfig},
			})
			before, err := mgr.Diff("testuser")
			require.NoError(t, err)
			require.NotEmpty(t, before, "the refusal must preserve a real pending candidate")
			answer := runTerminalCommandForm(t, handler, url.Values{
				"command": {tc.line}, "mode": {tc.mode},
			})
			require.Equal(t, tc.mode, answer.Mode)
			require.False(t, answer.OutputComplete)
			require.Contains(t, answer.Output, "configure")
			after, err := mgr.Diff("testuser")
			require.NoError(t, err)
			require.Equal(t, before, after)
			if tc.want == "" {
				require.Empty(t, seen, "configuration input must not dispatch operationally")
				return
			}
			require.Equal(t, "ze> ", answer.Prompt)
			require.Contains(t, answer.Output, "configure input was refused")
			require.Equal(t, []string{tc.want}, seen)
			exact := runTerminalCommandForm(t, handler, url.Values{
				"command": {"configure"}, "mode": {terminalModeOperational},
			})
			require.Equal(t, terminalModeConfig, exact.Mode)
			require.True(t, exact.OutputComplete)
			require.Equal(t, []string{tc.want}, seen, "the exact local control must not dispatch")
		})
	}
}
