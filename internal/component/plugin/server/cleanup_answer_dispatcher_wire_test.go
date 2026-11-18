// Design: docs/architecture/api/process-protocol.md -- command execution.
// Related: command.go -- the process answer forwarded by the dispatcher.
// Related: ../dispatch.go -- the daemon's actual answer writer.

package server

import (
	"context"
	"net"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// The daemon's real writer must preserve original counts across the second
// socket hop, including a bounded document and a streamed record collection.
func TestCleanupAnswerDispatcherWireKeepsOriginalCounts(t *testing.T) {
	for _, count := range []int{2, rpc.AnswerBufferThreshold + 3} {
		t.Run(strconv.Itoa(count), func(t *testing.T) {
			s := cleanupAnswerDispatcher(t, rpc.AnswerTail{}, cleanupAnswerRows(count, 1, nil))
			answer := cleanupAnswerDispatchWire(t, s, nil)
			document, err := rpc.CollapseAnswer(answer)
			require.NoError(t, err)
			require.NoError(t, answer.Err())
			metadata, err := answer.RecordAnswerMetadata()
			require.NoError(t, err)
			assert.Equal(t, uint64(count), metadata.Count)
			assert.Equal(t, uint64(1), metadata.Faults)
			assert.Equal(t, rpc.VerdictPartial, answer.Verdict())
			cleanupAnswerDocument(t, document, count, 1)
		})
	}
}

// A fault-only bounded document is still structured progress on the wire;
// neither forwarding hop may replace it with an unstructured failure string.
func TestCleanupAnswerDispatcherWireKeepsFaultOnlyData(t *testing.T) {
	for _, count := range []int{2, rpc.AnswerBufferThreshold + 3} {
		t.Run(strconv.Itoa(count), func(t *testing.T) {
			s := cleanupAnswerDispatcher(t, rpc.AnswerTail{}, cleanupAnswerRows(0, count, nil))
			answer := cleanupAnswerDispatchWire(t, s, nil)
			document, err := rpc.CollapseAnswer(answer)
			require.NoError(t, err)
			require.NoError(t, answer.Err())
			metadata, err := answer.RecordAnswerMetadata()
			require.NoError(t, err)
			assert.Zero(t, metadata.Count)
			assert.Equal(t, uint64(count), metadata.Faults)
			assert.Equal(t, rpc.VerdictError, answer.Verdict())
			cleanupAnswerDocument(t, document, 0, count)
		})
	}
}

// The public command failure must arrive on the actual forwarded terminator,
// with every completed record still present on either side of the threshold.
func TestCleanupAnswerDispatcherWireKeepsFailureTerminator(t *testing.T) {
	for _, count := range []int{2, rpc.AnswerBufferThreshold + 3} {
		t.Run(strconv.Itoa(count), func(t *testing.T) {
			const message = "evaluation stopped before all native acknowledgements arrived"
			s := cleanupAnswerDispatcher(t, rpc.AnswerTail{Message: message}, cleanupAnswerRows(count, 0, nil))
			answer := cleanupAnswerDispatchWire(t, s, nil)
			document, err := rpc.CollapseAnswer(answer)
			require.NoError(t, err)
			require.NoError(t, answer.Err())
			assert.Equal(t, rpc.VerdictAborted, answer.Verdict())
			assert.Equal(t, message, answer.Message())
			metadata, err := answer.RecordAnswerMetadata()
			require.NoError(t, err)
			assert.Equal(t, uint64(count), metadata.Count)
			assert.Zero(t, metadata.Faults)
			cleanupAnswerDocument(t, document, count, 0)
		})
	}
}

// A warning written by the recipient reaches the downstream socket unchanged,
// rather than disappearing because the daemon captured its head too early.
func TestCleanupAnswerDispatcherWireKeepsHealthyWarning(t *testing.T) {
	for _, count := range []int{2, rpc.AnswerBufferThreshold + 3} {
		t.Run(strconv.Itoa(count), func(t *testing.T) {
			lines := []string{"cleanup remains owned until its acknowledgement", "retry the retained cleanup"}
			s := cleanupAnswerDispatcher(t, rpc.AnswerTail{Warning: true, Warnings: lines}, cleanupAnswerRows(count, 0, nil))
			answer := cleanupAnswerDispatchWire(t, s, nil)
			document, err := rpc.CollapseAnswer(answer)
			require.NoError(t, err)
			require.NoError(t, answer.Err())
			assert.Equal(t, rpc.VerdictWarning, answer.Verdict())
			assert.Equal(t, lines, answer.Warnings())
			assert.Empty(t, answer.Message())
			metadata, err := answer.RecordAnswerMetadata()
			require.NoError(t, err)
			assert.Equal(t, uint64(count), metadata.Count)
			assert.Zero(t, metadata.Faults)
			cleanupAnswerDocument(t, document, count, 0)
		})
	}
}

// The second hop must open its stream before the recipient releases the end,
// then retain the failure which was unknowable when that head was written.
func TestCleanupAnswerDispatcherWireKeepsLateTerminator(t *testing.T) {
	atTail := make(chan struct{})
	release := make(chan struct{})
	unblock := sync.OnceFunc(func() { close(release) })
	defer unblock()
	const count = rpc.AnswerBufferThreshold + 3
	s := cleanupAnswerDispatcher(t, rpc.AnswerTail{Message: "late evaluation failure"},
		cleanupAnswerRows(count, 0, func() { close(atTail); <-release }))
	answer := cleanupAnswerDispatchWire(t, s, unblock)
	require.Equal(t, rpc.AnswerTypeMap, answer.Type, "daemon buffered until the held terminator")
	select {
	case <-atTail:
	case <-time.After(5 * time.Second):
		t.Fatal("recipient never reached its held terminator")
	}
	unblock()
	document, err := rpc.CollapseAnswer(answer)
	require.NoError(t, err)
	require.NoError(t, answer.Err())
	assert.Equal(t, rpc.VerdictAborted, answer.Verdict())
	assert.Equal(t, "late evaluation failure", answer.Message())
	cleanupAnswerDocument(t, document, count, 0)
}

// cleanupAnswerDispatchWire uses a real downstream MuxConn reader and the
// production WriteAnswer producer. Cleanup closes both pipe ends, releases a
// held recipient when supplied and joins the writer even after a Fatal.
func cleanupAnswerDispatchWire(t *testing.T, s *Server, unblock func()) *rpc.Answer {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	response, err := s.dispatcher.Dispatch(&CommandContext{Server: s, RequestContext: ctx}, "show cleanup answer")
	require.NoError(t, err)
	require.NotNil(t, response)
	reader, writer := net.Pipe()
	mux := rpc.NewMuxConn(rpc.NewConn(reader, reader))
	done := make(chan error, 1)
	t.Cleanup(func() {
		cancel()
		if unblock != nil {
			unblock()
		}
		assert.NoError(t, mux.Close())
		assert.NoError(t, writer.Close())
		if err := <-done; err != nil {
			if t.Failed() {
				t.Logf("writer completed during failed-test cleanup: %v", err)
			} else {
				assert.NoError(t, err, "successful answer's writer did not complete")
			}
		}
	})
	go func() {
		connection := rpc.NewConn(writer, writer)
		request, err := connection.ReadRequest(ctx)
		if err != nil {
			done <- err
			return
		}
		done <- plugin.WriteAnswer(writer, request.ID, response)
	}()
	answer, err := mux.CallAnswer(ctx, rpc.MethodDispatchCommand, nil)
	require.NoError(t, err)
	return answer
}
