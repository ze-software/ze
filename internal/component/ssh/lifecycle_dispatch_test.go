// Design: docs/architecture/api/commands.md -- SSH exec aliases retain shared admission.
package ssh

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	charmssh "charm.land/ssh"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
)

// TestSSHExecLifecycleCanonicalDispatch drives real exec middleware without a
// socket. An alias invokes only its session executor, and its accepted action
// runs through response completion rather than an unscoped global callback.
func TestSSHExecLifecycleCanonicalDispatch(t *testing.T) {
	for _, tc := range []struct{ alias, canonical string }{
		{"stop", "request shutdown"},
		{"restart", "request restart"},
		{"reboot", "request reboot"},
	} {
		t.Run(tc.alias, func(t *testing.T) {
			server, err := NewServer(Config{HostKeyPath: filepath.Join(t.TempDir(), "host-key")})
			require.NoError(t, err)
			accepted := 0
			input := ""
			server.SetExecutorFactory(func(context.Context, string, string, plugin.Authorizer) CommandExecutor {
				return func(command string) (*plugin.RenderedResponse, error) {
					input = command
					response := plugin.NewResponse(plugin.StatusDone, plugin.Map{"accepted": true})
					response.OnTransportComplete(func() { accepted++ })
					return &plugin.RenderedResponse{Output: "accepted", Response: response}, nil
				}
			})
			session := &historySession{
				ctx: &historySessionContext{Context: t.Context()}, command: []string{tc.alias},
			}
			server.execMiddleware()(func(charmssh.Session) { t.Error("alias escaped exec handling") })(session)
			assert.Equal(t, tc.canonical, input)
			assert.Equal(t, 1, accepted)
			assert.Zero(t, session.exit)
		})
	}
}

// TestSSHExecLifecycleRefusalNeverStops covers refusal classes the old alias
// branch ignored. Neither a transport failure nor an unknown command grants
// permission to run an unrelated stop callback.
func TestSSHExecLifecycleRefusalNeverStops(t *testing.T) {
	for _, refusal := range []error{context.Canceled, errors.New("unknown command"), errors.New("lifecycle unavailable")} {
		t.Run(refusal.Error(), func(t *testing.T) {
			server, err := NewServer(Config{HostKeyPath: filepath.Join(t.TempDir(), "host-key")})
			require.NoError(t, err)
			server.SetExecutorFactory(func(context.Context, string, string, plugin.Authorizer) CommandExecutor {
				return func(string) (*plugin.RenderedResponse, error) {
					return &plugin.RenderedResponse{}, refusal
				}
			})
			session := &historySession{
				ctx: &historySessionContext{Context: t.Context()}, command: []string{"restart"},
			}
			server.execMiddleware()(func(charmssh.Session) { t.Error("refusal escaped exec handling") })(session)
			assert.Equal(t, 1, session.exit)
		})
	}
}
