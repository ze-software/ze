// Design: docs/architecture/api/ipc_protocol.md -- SSH exec channel.
package ssh

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"iter"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/cli/sshclient"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// A collapsed record answer must keep its original count on the real exec
// frame. Its one document is the representation, not one produced record.
func TestExecProgressKeepsBoundedOriginalCounts(t *testing.T) {
	const document = `{"cleanup":[{"id":1},{"id":2}]}`
	srv := answerServer(t, func(string) (*plugin.Response, error) {
		response := plugin.NewResponse(plugin.StatusDone, plugin.RawJSON(document))
		completionError := response.CompleteRecordAnswer(&rpc.ExecuteCommandRecordAnswer{Count: 2})
		return response, completionError
	})
	var body strings.Builder
	answer, err := sshclient.ExecCommandStream(answerCredentials(t, srv), "show cleanup | json", &body)
	require.NoError(t, err)
	assert.Equal(t, rpc.VerdictDone, answer.Verdict)
	assert.Equal(t, uint64(2), answer.Count)
	assert.Zero(t, answer.Faults)
	assert.JSONEq(t, `[{"id":1},{"id":2}]`, body.String())
}

// A completed document beside incomplete evaluation remains visible on stdout,
// while the real SSH frame and exit status retain its failure and count.
func TestExecProgressKeepsBoundedFailedData(t *testing.T) {
	const document = `{"cleanup":[{"id":1},{"id":2}]}`
	const message = "evaluation stopped before the final acknowledgement"
	srv := answerServer(t, func(string) (*plugin.Response, error) {
		response := plugin.NewResponse(plugin.StatusDone, plugin.RawJSON(document))
		completionError := response.CompleteRecordAnswer(&rpc.ExecuteCommandRecordAnswer{Count: 2, Message: message})
		return response, completionError
	})
	var body strings.Builder
	answer, err := sshclient.ExecCommandStream(answerCredentials(t, srv), "show cleanup | json", &body)
	require.Error(t, err)
	assert.Equal(t, message, err.Error())
	assert.Equal(t, rpc.VerdictAborted, answer.Verdict)
	assert.Equal(t, uint64(2), answer.Count)
	assert.Zero(t, answer.Faults)
	assert.JSONEq(t, `[{"id":1},{"id":2}]`, body.String())
}

// A partial bounded result keeps both collections and their distinct counts
// when the text adapter also reports the refused records as a Go error.
func TestExecProgressKeepsBoundedPartialData(t *testing.T) {
	const document = `{"cleanup":[{"id":1}],"errors":[{"id":2,"cause":"acknowledgement missing"}]}`
	srv := answerServer(t, func(string) (*plugin.Response, error) {
		response := plugin.NewResponse(plugin.StatusDone, plugin.RawJSON(document))
		completionError := response.CompleteRecordAnswer(&rpc.ExecuteCommandRecordAnswer{Count: 1, Faults: 1})
		return response, completionError
	})
	var body strings.Builder
	answer, err := sshclient.ExecCommandStream(answerCredentials(t, srv), "show cleanup | json", &body)
	require.Error(t, err)
	assert.Equal(t, rpc.VerdictPartial, answer.Verdict)
	assert.Equal(t, uint64(1), answer.Count)
	assert.Equal(t, uint64(1), answer.Faults)
	assert.JSONEq(t, document, body.String())
}

// Completion is published by the producer only after its last row. The SSH
// renderer must read that failure after walking, rather than snapshotting done.
func TestExecProgressKeepsLateFailure(t *testing.T) {
	for _, count := range []int{2, rpc.AnswerBufferThreshold + 3} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			const message = "evaluation stopped before the final acknowledgement"
			srv := answerServer(t, func(string) (*plugin.Response, error) {
				return execProgressResponse(count, message, nil), nil
			})
			var body strings.Builder
			answer, err := sshclient.ExecCommandStream(answerCredentials(t, srv), "show cleanup | ndjson", &body)
			require.Error(t, err)
			assert.Equal(t, message, err.Error())
			assert.Equal(t, rpc.VerdictAborted, answer.Verdict)
			assert.Equal(t, uint64(count), answer.Count)
			assert.Zero(t, answer.Faults)
			assert.Len(t, strings.Split(strings.TrimSpace(body.String()), "\n"), count)
		})
	}
}

// A warning learned after streamed rows reaches a real client as both the
// warning verdict and the original plain warning lines beside its data.
func TestExecProgressKeepsLateWarning(t *testing.T) {
	for _, count := range []int{2, rpc.AnswerBufferThreshold + 3} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			lines := []string{"cleanup remains owned until its acknowledgement", "retry the retained cleanup"}
			srv := answerServer(t, func(string) (*plugin.Response, error) {
				return execProgressResponse(count, "", lines), nil
			})
			var body strings.Builder
			answer, err := sshclient.ExecCommandStream(answerCredentials(t, srv), "show cleanup | ndjson", &body)
			require.NoError(t, err)
			assert.Equal(t, rpc.VerdictWarning, answer.Verdict)
			assert.Equal(t, strings.Join(lines, "\n"), strings.TrimSpace(answer.Warnings))
			assert.Equal(t, uint64(count), answer.Count)
			assert.Len(t, strings.Split(strings.TrimSpace(body.String()), "\n"), count)
		})
	}
}

// A record pipe changes the answer's count. Original metadata cannot override
// the renderer's one count result with the 259 input rows it consumed.
func TestExecProgressKeepsPipeSelectedCount(t *testing.T) {
	const count = rpc.AnswerBufferThreshold + 3
	srv := answerServer(t, func(string) (*plugin.Response, error) {
		return execProgressResponse(count, "", nil), nil
	})
	var body strings.Builder
	answer, err := sshclient.ExecCommandStream(answerCredentials(t, srv), "show cleanup | count", &body)
	require.NoError(t, err)
	assert.Equal(t, rpc.VerdictDone, answer.Verdict)
	assert.Equal(t, uint64(1), answer.Count)
	assert.Zero(t, answer.Faults)
	assert.Contains(t, body.String(), "259")
}

// One bounded walk publishes its original completion only on exhaustion, as
// the daemon's forwarded record response does. It starts no worker or timer.
func execProgressResponse(count int, message string, warnings []string) *plugin.Response {
	response := &plugin.Response{Status: plugin.StatusDone}
	rows := iter.Seq[rpc.RowRecord](func(yield func(rpc.RowRecord) bool) {
		for ordinal := range count {
			row := rpc.RawRow(fmt.Sprintf(`{"ordinal":%d}`, ordinal))
			if !yield(rpc.RowRecord{Item: &row}) {
				return
			}
		}
		if err := response.CompleteRecordAnswer(&rpc.ExecuteCommandRecordAnswer{
			Count: uint64(count), Message: message, Warning: len(warnings) != 0, Warnings: warnings,
		}); err != nil {
			response.Status = plugin.StatusError
			response.Error = err.Error()
		}
	})
	response.Data = plugin.Records{Key: "cleanup", Rows: rows}
	return response
}

// Goal: selecting one row must retain a failure learned at the end of a long
// producer. Method: use the real SSH frame and check the selected body and tail.
func TestExecFirstKeepsLateFailure(t *testing.T) {
	const message = "evaluation stopped before the final acknowledgement"
	warnings := []string{"the original cleanup owner remains", "retry its acknowledgement"}
	srv := answerServer(t, func(string) (*plugin.Response, error) {
		return execFirstProgressResponse(0, message, warnings), nil
	})
	var body strings.Builder
	answer, err := sshclient.ExecCommandStream(answerCredentials(t, srv), "show cleanup | first 1 | ndjson", &body)
	require.EqualError(t, err, message)
	assert.Equal(t, rpc.VerdictAborted, answer.Verdict)
	assert.Equal(t, uint64(1), answer.Count)
	assert.Zero(t, answer.Faults)
	assert.Contains(t, answer.Warnings, strings.Join(warnings, "\n"))
	assert.JSONEq(t, `{"cleanup":[{"ordinal":0}],"pipe":[{"arg":"1","op":"first"}]}`, strings.TrimSpace(body.String()))
}

// Goal: a refused record after the selected row must remain a failed outcome.
// Method: the producer appends its fault and warning only after 259 item rows.
func TestExecFirstKeepsLateFault(t *testing.T) {
	warnings := []string{"cleanup still owns the rejected acknowledgement"}
	srv := answerServer(t, func(string) (*plugin.Response, error) {
		return execFirstProgressResponse(1, "", warnings), nil
	})
	var body strings.Builder
	answer, err := sshclient.ExecCommandStream(answerCredentials(t, srv), "show cleanup | first 1 | ndjson", &body)
	require.Error(t, err)
	assert.Equal(t, rpc.VerdictPartial, answer.Verdict)
	assert.Equal(t, uint64(1), answer.Count)
	assert.Equal(t, uint64(1), answer.Faults)
	assert.Contains(t, answer.Warnings, warnings[0])
	assert.JSONEq(t, `{"cleanup":[{"ordinal":0}],"pipe":[{"arg":"1","op":"first"}]}`, strings.TrimSpace(body.String()))
}

// Goal: selection must keep a late warning without turning it into a failure.
// Method: compare the real client's warning verdict, text and one selected row.
func TestExecFirstKeepsLateWarning(t *testing.T) {
	warnings := []string{"the original cleanup owner remains", "retry its acknowledgement"}
	srv := answerServer(t, func(string) (*plugin.Response, error) {
		return execFirstProgressResponse(0, "", warnings), nil
	})
	var body strings.Builder
	answer, err := sshclient.ExecCommandStream(answerCredentials(t, srv), "show cleanup | first 1 | ndjson", &body)
	require.NoError(t, err)
	assert.Equal(t, rpc.VerdictWarning, answer.Verdict)
	assert.Equal(t, strings.Join(warnings, "\n"), strings.TrimSpace(answer.Warnings))
	assert.Equal(t, uint64(1), answer.Count)
	assert.Zero(t, answer.Faults)
	assert.JSONEq(t, `{"cleanup":[{"ordinal":0}],"pipe":[{"arg":"1","op":"first"}]}`, strings.TrimSpace(body.String()))
}

// Goal: original completion must not replace the selected count with 259.
// Method: a healthy long producer is the positive control for all first cases.
func TestExecFirstKeepsSelectedCount(t *testing.T) {
	srv := answerServer(t, func(string) (*plugin.Response, error) {
		return execFirstProgressResponse(0, "", nil), nil
	})
	var body strings.Builder
	answer, err := sshclient.ExecCommandStream(answerCredentials(t, srv), "show cleanup | first 1 | ndjson", &body)
	require.NoError(t, err)
	assert.Equal(t, rpc.VerdictDone, answer.Verdict)
	assert.Equal(t, uint64(1), answer.Count)
	assert.Zero(t, answer.Faults)
	assert.Empty(t, answer.Warnings)
	assert.JSONEq(t, `{"cleanup":[{"ordinal":0}],"pipe":[{"arg":"1","op":"first"}]}`, strings.TrimSpace(body.String()))
}

// This finite producer has the same exhaustion-only publication as
// pluginserver.pluginAnswerRows. The late fault comes after every item.
func execFirstProgressResponse(faults int, message string, warnings []string) *plugin.Response {
	const count = rpc.AnswerBufferThreshold + 3
	response := &plugin.Response{Status: plugin.StatusDone}
	response.Data = plugin.Records{Key: "cleanup", Rows: func(yield func(rpc.RowRecord) bool) {
		for ordinal := range count {
			row := rpc.RawRow(fmt.Sprintf(`{"ordinal":%d}`, ordinal))
			if !yield(rpc.RowRecord{Item: &row}) {
				return
			}
		}
		for range faults {
			fault := rpc.RawRow(`{"cause":"acknowledgement refused"}`)
			if !yield(rpc.RowRecord{Fault: &fault}) {
				return
			}
		}
		if err := response.CompleteRecordAnswer(&rpc.ExecuteCommandRecordAnswer{
			Count: count, Faults: uint64(faults), Message: message, Warning: len(warnings) != 0, Warnings: warnings,
		}); err != nil {
			response.Status, response.Error = plugin.StatusError, err.Error()
		}
	}}
	return response
}

// Goal: draining an unselected suffix must stop at session cancellation.
// Method: cancel on the second source row and require no completed frame.
func TestExecFirstStopsOnSessionCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	session := &execFirstSession{historySession: &historySession{ctx: &historySessionContext{Context: ctx}}}
	frame := newAnswerFrame(session.Stderr())
	defer frame.release()
	response := execFirstProgressResponse(0, "", nil)
	records, ok := plugin.RecordRows(response)
	require.True(t, ok)
	produced := 0
	original := records.Rows
	records.Rows = func(yield func(rpc.RowRecord) bool) {
		for row := range original {
			produced++
			if produced == 2 {
				cancel()
			}
			if !yield(row) {
				return
			}
		}
	}
	_, err := writeExecRecords(session, frame, "show cleanup | first 1 | ndjson", records, response)
	require.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, 2, produced)
	assert.Empty(t, session.body.String())
	assert.Empty(t, session.diagnostics.String())
	_, completed := response.RecordAnswerMetadata()
	assert.False(t, completed)
}

// Goal: a dead writer must stop the source instead of draining it for nobody.
// Method: fail the first streamed write and check the exact threshold prefix.
func TestExecFirstStopsOnWriterFailure(t *testing.T) {
	session := &execFirstSession{
		historySession: &historySession{ctx: &historySessionContext{Context: t.Context()}},
		writeErr:       io.ErrClosedPipe,
	}
	frame := newAnswerFrame(session.Stderr())
	defer frame.release()
	response := execFirstProgressResponse(0, "", nil)
	records, ok := plugin.RecordRows(response)
	require.True(t, ok)
	produced := 0
	original := records.Rows
	records.Rows = func(yield func(rpc.RowRecord) bool) {
		for row := range original {
			produced++
			if !yield(row) {
				return
			}
		}
	}
	_, err := writeExecRecords(session, frame, "show cleanup | ndjson", records, response)
	require.ErrorIs(t, err, io.ErrClosedPipe)
	assert.Equal(t, rpc.AnswerBufferThreshold+1, produced)
	assert.Empty(t, session.body.String())
	assert.Empty(t, session.diagnostics.String())
	_, completed := response.RecordAnswerMetadata()
	assert.False(t, completed)
}

type execFirstSession struct {
	*historySession
	body        strings.Builder
	diagnostics bytes.Buffer
	writeErr    error
}

func (s *execFirstSession) Stderr() io.ReadWriter { return &s.diagnostics }
func (s *execFirstSession) Write(data []byte) (int, error) {
	if s.writeErr != nil {
		return 0, s.writeErr
	}
	return s.body.Write(data)
}
