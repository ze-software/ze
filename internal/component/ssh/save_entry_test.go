package ssh

import (
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	sshclient "github.com/ze-software/ze/internal/core/ssh/client"
)

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

	output, err := sshclient.ExecCommand(answerCredentials(t, server), "show version | save "+path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "save")
	assert.Contains(t, err.Error(), "refused")
	assert.Empty(t, output)
	assert.False(t, dispatched.Load(), "a refused save must not reach the command dispatcher")
	_, statErr := os.Stat(path)
	assert.ErrorIs(t, statErr, os.ErrNotExist)
}
