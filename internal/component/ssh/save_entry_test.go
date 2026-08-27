package ssh

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gossh "golang.org/x/crypto/ssh"

	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/plugin"
	sshclient "github.com/ze-software/ze/internal/core/ssh/client"
	"github.com/ze-software/ze/internal/core/textbuf"
)

type synchronizedBuffer struct {
	mu sync.Mutex
	bytes.Buffer
}

func (b *synchronizedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.Buffer.Write(p)
}

func (b *synchronizedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.Buffer.String()
}

// TestSSHExecRefusesSaveAtTheEntryPoint drives a real authenticated SSH exec
// channel through execMiddleware. The refusal must happen before dispatch,
// because honoring save there would write as the daemon.
//
// VALIDATES: IR-18 -- remote SSH save exits with an error naming save and writes nothing.
// PREVENTS: the SSH entry point using the local-allowed pipe validator.
func TestSSHExecRefusesSaveAtTheEntryPoint(t *testing.T) {
	var dispatched atomic.Bool
	server := answerServer(t, func(string) (*plugin.Response, error) {
		dispatched.Store(true)
		return plugin.NewResponse(plugin.StatusDone, plugin.RawJSON(`{"version":"test"}`)), nil
	})
	path := filepath.Join(t.TempDir(), "ssh-save.json")

	var commandLine textbuf.Buffer
	output, err := sshclient.ExecCommand(answerCredentials(t, server),
		commandLine.Str("show version | save ").Str(path).String())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "save")
	assert.Contains(t, err.Error(), "refused")
	assert.Empty(t, output)
	assert.False(t, dispatched.Load(), "a refused save must not reach the command dispatcher")
	_, statErr := os.Stat(path)
	assert.ErrorIs(t, statErr, os.ErrNotExist)
}

// TestAuthenticatedSSHPTYRefusesSaveBeforeModelDispatch drives a real password
// authentication and PTY shell through SessionModelFactory, Bubble Tea, and
// Model.executeOperationalCommand. The model refusal must precede dispatch.
//
// VALIDATES: IR2-1 -- authenticated SSH PTY models cannot write daemon paths.
// PREVENTS: exec being safe while the interactive SSH rail remains writable.
func TestAuthenticatedSSHPTYRefusesSaveBeforeModelDispatch(t *testing.T) {
	var factoryCalled atomic.Bool
	var dispatched atomic.Bool
	path := filepath.Join(t.TempDir(), "ssh-pty-save.json")
	server := answerServer(t, func(string) (*plugin.Response, error) {
		return plugin.NewResponse(plugin.StatusDone, plugin.RawJSON(`{"version":"unused"}`)), nil
	})
	server.SetSessionModelFactory(
		func(username, _ string, _ plugin.Authorizer) tea.Model {
			factoryCalled.Store(username == "operator")
			model := cli.NewCommandModel(cli.FilesystemAuthorityUnknown)
			model.SetCommandExecutor(func(string) (cli.CommandOutput, error) {
				dispatched.Store(true)
				return cli.CommandOutput{Text: `{"version":"test"}`}, nil
			})
			return model
		},
	)

	client, err := gossh.Dial("tcp", server.Address(), &gossh.ClientConfig{
		User:            "operator",
		Auth:            []gossh.AuthMethod{gossh.Password("read-pass")},
		HostKeyCallback: gossh.InsecureIgnoreHostKey(), //nolint:gosec // test server key is generated per run.
		Timeout:         5 * time.Second,
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, client.Close()) })

	session, err := client.NewSession()
	require.NoError(t, err)
	t.Cleanup(func() { _ = session.Close() })

	var screen synchronizedBuffer
	session.Stdout = &screen
	session.Stderr = &screen
	stdin, err := session.StdinPipe()
	require.NoError(t, err)
	require.NoError(t, session.RequestPty("xterm", 24, 80, gossh.TerminalModes{
		gossh.ECHO: 0,
	}))
	require.NoError(t, session.Shell())
	require.Eventually(t, func() bool {
		return strings.Contains(screen.String(), "welcome to ze!")
	}, 5*time.Second, 10*time.Millisecond, "PTY model did not render its initial view")
	var input textbuf.Buffer
	_, err = io.WriteString(stdin,
		input.Str("show version | json compact | save ").Str(path).Byte('\r').String())
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		output := screen.String()
		return strings.Contains(output, "save") && strings.Contains(output, "refused")
	}, 5*time.Second, 10*time.Millisecond, "PTY did not render the named save refusal")

	assert.True(t, factoryCalled.Load(), "authenticated username did not reach the model factory")
	assert.False(t, dispatched.Load(), "a refused PTY save must not reach the command dispatcher")
	_, statErr := os.Stat(path)
	assert.ErrorIs(t, statErr, os.ErrNotExist)
}
