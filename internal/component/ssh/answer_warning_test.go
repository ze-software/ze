package ssh

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/cli/sshclient"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestAWarningReachesTheOperatorOverTheExecChannel drives a response that
// warns about the answer it still gives (the parse warnings of
// `show config dump` on the daemon route) over a real SSH connection, and
// reads it with the client `ze cli` uses.
//
// VALIDATES: the warnings reach the remote caller beside the answer, on the
// channel the local route prints them on (stderr), and a warning is not read
// as the command's failure.
// PREVENTS: the daemon route dropping a warning `ze <command>` prints, which
// answers the same payload with less said about it.
func TestAWarningReachesTheOperatorOverTheExecChannel(t *testing.T) {
	srv := answerServer(t, func(string) (*plugin.Response, error) {
		return &plugin.Response{
			Status:   plugin.StatusWarning,
			Data:     plugin.Map{"answer": "kept"},
			Warnings: []string{"warning: first line", "warning: second line"},
		}, nil
	})
	creds := answerCredentials(t, srv)

	var body strings.Builder
	answer, err := sshclient.ExecCommandStream(creds, "show config dump | json compact", &body)
	require.NoError(t, err, "a warning is not a failure")
	assert.Equal(t, rpc.VerdictWarning, answer.Verdict)
	assert.Equal(t, "warning: first line\nwarning: second line", answer.Warnings,
		"every warning line reaches the caller, in the local route's bytes")
	assert.Equal(t, `{"answer":"kept"}`, strings.TrimSpace(body.String()),
		"the answer on stdout stays the payload a pipe reads")

	output, err := sshclient.ExecCommand(creds, "show config dump | json compact")
	require.NoError(t, err, "the buffered client must not read the warning as a failure either")
	assert.Equal(t, `{"answer":"kept"}`, output)
}
