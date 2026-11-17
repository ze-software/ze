package ssh

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/cli/sshclient"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// refusedVerdictLine is the refusal a local-data command writes whole beside
// its payload, as `show config validation` does for a config it rejects.
const refusedVerdictLine = "error: bad.conf is not a valid configuration: 1 diagnostic(s) in the answer say why\n"

// TestARefusalWithAPayloadWritesTheDocumentOverTheExecChannel: a command that
// refuses and still answers a document has the document written on stdout,
// and the frame counts it. Method: an executor that answers a refusal
// (plugin.RefusalError) beside response data, read by a framed client.
//
// VALIDATES: the client's body holds the payload, the terminator counts one
// record under the refusal's message, and the verdict is the one a client
// reads as "refused, and what arrived is real" (rpc.VerdictAborted).
// PREVENTS: the exec channel dropping the diagnostics of a config the daemon
// rejects, so the refusal names an answer the channel never sent. Without
// writeExecRefusedAnswer the body assertion goes red first (the body is
// empty), then the count (0) and the verdict (rpc.VerdictError).
func TestARefusalWithAPayloadWritesTheDocumentOverTheExecChannel(t *testing.T) {
	srv := answerServer(t, func(string) (*plugin.Response, error) {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   plugin.RawJSON(`{"valid":false,"path":"bad.conf"}`),
		}, plugin.NewRefusalError(refusedVerdictLine)
	})
	creds := answerCredentials(t, srv)

	var body strings.Builder
	answer, err := sshclient.ExecCommandStream(creds, sshclient.RawCommand("show config validation bad.conf"), &body)

	require.Error(t, err, "a refusal read as an answer")
	assert.Equal(t, "bad.conf is not a valid configuration: 1 diagnostic(s) in the answer say why", err.Error())
	require.NotEmpty(t, strings.TrimSpace(body.String()), "the payload beside the refusal was not written on stdout")
	assert.JSONEq(t, `{"valid":false,"path":"bad.conf"}`, body.String())
	assert.Equal(t, uint64(1), answer.Count, "the terminator does not count the document the channel wrote")
	assert.Equal(t, rpc.VerdictAborted, answer.Verdict, "a message over one record: the command refused and the document is real")
}

// TestARefusalWithNoPayloadWritesNothingOnStdout: the negative of the test
// above. Method: the same refusal with no response data, then a plain error
// (no RefusalError) that does carry data.
//
// VALIDATES: a refusal with no payload writes nothing on stdout and counts no
// record, as before; and data beside an error that is not a command's own
// refusal is not written either, because it is not an answer the command
// chose to give beside its verdict.
// PREVENTS: writeExecRefusedAnswer writing on every failure.
func TestARefusalWithNoPayloadWritesNothingOnStdout(t *testing.T) {
	cases := []struct {
		name    string
		respond func(string) (*plugin.Response, error)
	}{
		{
			name: "a refusal with no payload",
			respond: func(string) (*plugin.Response, error) {
				return &plugin.Response{Status: plugin.StatusError}, plugin.NewRefusalError(refusedVerdictLine)
			},
		},
		{
			name: "a plain error beside data",
			respond: func(string) (*plugin.Response, error) {
				return &plugin.Response{Status: plugin.StatusError, Data: plugin.RawJSON(`{"valid":false}`)},
					errors.New("the daemon could not run the command")
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := answerServer(t, tc.respond)
			creds := answerCredentials(t, srv)

			var body strings.Builder
			answer, err := sshclient.ExecCommandStream(creds, sshclient.RawCommand("show config validation bad.conf"), &body)

			require.Error(t, err)
			assert.Empty(t, body.String(), "a failure with no document of its own wrote on stdout")
			assert.Zero(t, answer.Count)
			assert.Equal(t, rpc.VerdictError, answer.Verdict)
		})
	}
}
