// Design: docs/architecture/api/ipc_protocol.md -- SSH exec channel.
// Related: ../command/render_records.go -- pipe-selected answer counts.
package ssh

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gossh "golang.org/x/crypto/ssh"

	"github.com/ze-software/ze/internal/component/cli/sshclient"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// A pipe that changes cardinality must report its actual output count rather
// than the original walk's count. A first bound beyond the input keeps both rows.
func TestExecCountedDocumentKeepsPipeSelectedCounts(t *testing.T) {
	for _, question := range []struct {
		name  string
		chain string
		count uint64
		body  string
	}{
		{name: "count result", chain: "count | json", count: 1, body: `{"count":2,"pipe":[{"op":"count"}]}`},
		{name: "first one", chain: "first 1 | json", count: 1, body: `{"cleanup":[{"id":1}],"pipe":[{"op":"first","arg":"1"}]}`},
		{name: "first beyond input", chain: "first 100 | json", count: 2, body: `{"cleanup":[{"id":1},{"id":2}],"pipe":[{"op":"first","arg":"100"}]}`},
		{name: "format before count", chain: "json | count", count: 1, body: `{"count":2,"pipe":[{"op":"count"}]}`},
	} {
		t.Run(question.name, func(t *testing.T) {
			srv := answerServer(t, func(string) (*plugin.Response, error) {
				response := plugin.NewResponse(plugin.StatusDone, plugin.RawJSON(`{"cleanup":[{"id":1},{"id":2}]}`))
				completionError := response.CompleteRecordAnswer(&rpc.ExecuteCommandRecordAnswer{Count: 2})
				return response, completionError
			})
			var body strings.Builder
			answer, err := sshclient.ExecCommandStream(answerCredentials(t, srv), "show cleanup | "+question.chain, &body)
			require.NoError(t, err)
			assert.Equal(t, rpc.VerdictDone, answer.Verdict)
			assert.Equal(t, question.count, answer.Count)
			assert.Zero(t, answer.Faults)
			assert.JSONEq(t, question.body, body.String())
		})
	}
}

// Presentation alone cannot reconstruct an original count from document keys
// or its visible collection length. The completed producer remains authoritative.
func TestExecCountedDocumentFormattingKeepsOriginalCounts(t *testing.T) {
	for _, question := range []struct {
		chain string
		body  string
	}{
		{chain: "json", body: `[{"id":1},{"id":2}]`},
		{chain: "raw", body: `{"cleanup":[{"id":1},{"id":2}]}`},
	} {
		t.Run(question.chain, func(t *testing.T) {
			srv := answerServer(t, func(string) (*plugin.Response, error) {
				response := plugin.NewResponse(plugin.StatusDone, plugin.RawJSON(`{"cleanup":[{"id":1},{"id":2}]}`))
				completionError := response.CompleteRecordAnswer(&rpc.ExecuteCommandRecordAnswer{Count: 7})
				return response, completionError
			})
			var body strings.Builder
			answer, err := sshclient.ExecCommandStream(answerCredentials(t, srv), "show cleanup | "+question.chain, &body)
			require.NoError(t, err)
			assert.Equal(t, rpc.VerdictDone, answer.Verdict)
			assert.Equal(t, uint64(7), answer.Count)
			assert.JSONEq(t, question.body, body.String())
		})
	}
}

// A configured text format is still presentation after a row selection. Its
// rendered table lines must not replace the selected record count on the frame.
func TestExecCountedDocumentConfiguredFormatKeepsSelectedCount(t *testing.T) {
	t.Setenv("ze.cli.format", "text")
	env.ResetCache()
	t.Cleanup(env.ResetCache)
	srv := answerServer(t, func(string) (*plugin.Response, error) {
		response := plugin.NewResponse(plugin.StatusDone, plugin.RawJSON(`{"cleanup":[{"id":1},{"id":2}]}`))
		completionError := response.CompleteRecordAnswer(&rpc.ExecuteCommandRecordAnswer{Count: 2})
		return response, completionError
	})
	var body strings.Builder
	answer, err := sshclient.ExecCommandStream(answerCredentials(t, srv), "show cleanup | first 1", &body)
	require.NoError(t, err)
	assert.Equal(t, rpc.VerdictDone, answer.Verdict)
	assert.Equal(t, uint64(1), answer.Count)
	assert.Zero(t, answer.Faults)
	assert.Contains(t, body.String(), "1")
	assert.NotContains(t, body.String(), "2")
}

// A display request before the selection still controls the text columns
// after it. Splitting the chain must not reconstruct an alphabetical order.
func TestExecCountedDocumentKeepsWholeChainColumnRequest(t *testing.T) {
	srv := answerServer(t, func(string) (*plugin.Response, error) {
		response := plugin.NewResponse(plugin.StatusDone, plugin.RawJSON(`{"cleanup":[{"state":"up","address":"192.0.2.1","ignored":"drop me"}]}`))
		completionError := response.CompleteRecordAnswer(&rpc.ExecuteCommandRecordAnswer{Count: 2})
		return response, completionError
	})
	var body strings.Builder
	answer, err := sshclient.ExecCommandStream(answerCredentials(t, srv), "show cleanup | display state address | first 1 | text", &body)
	require.NoError(t, err)
	assert.Equal(t, rpc.VerdictDone, answer.Verdict)
	assert.Equal(t, uint64(1), answer.Count)
	assert.Zero(t, answer.Faults)
	foundHeader := false
	for line := range strings.SplitSeq(body.String(), "\n") {
		state := strings.Index(line, "state")
		address := strings.Index(line, "address")
		if state >= 0 {
			if address >= 0 {
				foundHeader = true
				assert.Less(t, state, address, "explicit display order was lost at the selection frontier")
			}
		}
	}
	assert.True(t, foundHeader, "text rendering lost its selected header")
	assert.Contains(t, body.String(), "up")
	assert.Contains(t, body.String(), "192.0.2.1")
	assert.NotContains(t, body.String(), "ignored")
	assert.NotContains(t, body.String(), "drop me")
}

// A refused-row document contains two independent lists. The canonical count
// operator refuses it by name rather than choosing one list and reporting done.
func TestExecCountedDocumentPipeRefusesAmbiguousPartialData(t *testing.T) {
	const warning = "retry the cleanup retained by its original owner"
	srv := answerServer(t, func(string) (*plugin.Response, error) {
		response := plugin.NewResponse(plugin.StatusDone, plugin.RawJSON(`{"cleanup":[{"id":1},{"id":2}],"errors":[{"id":3,"cause":"acknowledgement missing"}]}`))
		completionError := response.CompleteRecordAnswer(&rpc.ExecuteCommandRecordAnswer{
			Count: 2, Faults: 1, Warning: true, Warnings: []string{warning},
		})
		return response, completionError
	})
	var body strings.Builder
	answer, err := sshclient.ExecCommandStream(answerCredentials(t, srv), "show cleanup | count", &body)
	require.Error(t, err)
	assert.NotEqual(t, rpc.VerdictDone, answer.Verdict)
	assert.NotEqual(t, rpc.VerdictWarning, answer.Verdict)
	text := body.String() + "\n" + err.Error() + "\n" + answer.Warnings
	assert.Contains(t, text, "count needs rows")
	assert.Contains(t, text, "several lists (cleanup, errors)")
	assert.Contains(t, text, warning)
}

// Formatting the two-list result as one NDJSON line makes a line count valid.
// Producing that one count result must not clear the source's refused record.
func TestExecCountedDocumentCountRetainsSourceFaults(t *testing.T) {
	const warning = "retry the cleanup retained by its original owner"
	srv := answerServer(t, func(string) (*plugin.Response, error) {
		response := plugin.NewResponse(plugin.StatusDone, plugin.RawJSON(`{"cleanup":[{"id":1},{"id":2}],"errors":[{"id":3,"cause":"acknowledgement missing"}]}`))
		completionError := response.CompleteRecordAnswer(&rpc.ExecuteCommandRecordAnswer{
			Count: 2, Faults: 1, Warning: true, Warnings: []string{warning},
		})
		return response, completionError
	})
	var body strings.Builder
	answer, err := sshclient.ExecCommandStream(answerCredentials(t, srv), "show cleanup | ndjson | count", &body)
	require.Error(t, err, "a count result cannot make refused source records successful")
	assert.Equal(t, rpc.VerdictPartial, answer.Verdict)
	assert.Equal(t, uint64(1), answer.Count)
	assert.Equal(t, uint64(1), answer.Faults)
	assert.JSONEq(t, `{"count":1,"pipe":[{"op":"count"}]}`, body.String())
	assert.Contains(t, answer.Warnings+"\n"+err.Error(), warning)
}

// A pipe evaluation that refuses before transport I/O gets a completed failed
// frame, rather than masquerading as lost transport. The same document still
// renders successfully when the operator requests presentation alone.
func TestExecCountedDocumentKnownPipeFailureHasCompleteFrame(t *testing.T) {
	srv := answerServer(t, func(string) (*plugin.Response, error) {
		response := plugin.NewResponse(plugin.StatusDone, plugin.RawJSON(`{"label":"no row collection"}`))
		completionError := response.CompleteRecordAnswer(&rpc.ExecuteCommandRecordAnswer{Count: 1})
		return response, completionError
	})
	credentials := answerCredentials(t, srv)
	var body strings.Builder
	answer, err := sshclient.ExecCommandStream(credentials, "show cleanup | count", &body)
	require.Error(t, err)
	assert.Equal(t, rpc.VerdictError, answer.Verdict)
	assert.Zero(t, answer.Count)
	assert.Zero(t, answer.Faults)
	assert.Contains(t, answer.Message, "count needs rows")
	assert.Empty(t, body.String())
	answer, err = sshclient.ExecCommandStream(credentials, "show cleanup | json", &body)
	require.NoError(t, err)
	assert.Equal(t, rpc.VerdictDone, answer.Verdict)
	assert.Equal(t, uint64(1), answer.Count)
	assert.JSONEq(t, `{"label":"no row collection"}`, body.String())
}

// The client helper reports a frame failure independently of SSH's exit code.
// Check the real channel status too, with a healthy warning as the zero control.
func TestExecProgressCompletionExitStatus(t *testing.T) {
	for _, question := range []struct {
		name     string
		message  string
		warnings []string
		status   int
	}{
		{name: "late failure", message: "late evaluation failure", status: 1},
		{name: "healthy warning", warnings: []string{"the original warning"}},
	} {
		t.Run(question.name, func(t *testing.T) {
			srv := answerServer(t, func(string) (*plugin.Response, error) {
				return execProgressResponse(rpc.AnswerBufferThreshold+3, question.message, question.warnings), nil
			})
			stderr, err := runPanicTest(t, dialPanicTest(t, srv), "show cleanup | ndjson")
			if question.status == 0 {
				require.NoError(t, err)
				assert.Contains(t, stderr, "the original warning")
				return
			}
			var exit *gossh.ExitError
			require.ErrorAs(t, err, &exit)
			assert.Equal(t, question.status, exit.ExitStatus())
			assert.Contains(t, stderr, question.message)
		})
	}
}
