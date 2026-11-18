// Design: docs/architecture/api/process-protocol.md -- command execution.
// Related: command.go -- routeToProcess and pluginAnswerRows.
// Related: ../dispatch.go -- AnswerFor and ResponseJSON.

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"net"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// A native plugin's short walk and streamed walk must keep the same original
// counts after the real Dispatcher routes to its registered process. The
// document contains every item and fault; its one container is not one item.
func TestCleanupAnswerDispatcherKeepsOriginalRecordCounts(t *testing.T) {
	for _, count := range []int{2, rpc.AnswerBufferThreshold + 3} {
		t.Run(strconv.Itoa(count), func(t *testing.T) {
			s := cleanupAnswerDispatcher(t, rpc.AnswerTail{}, cleanupAnswerRows(count, 1, nil))
			answer, document := cleanupAnswerDispatchProjection(t, s)
			metadata, err := answer.RecordAnswerMetadata()
			require.NoError(t, err)
			assert.Equal(t, uint64(count), metadata.Count, "original produced count changed")
			assert.Equal(t, uint64(1), metadata.Faults, "original refused count changed")
			assert.Equal(t, rpc.VerdictPartial, answer.Verdict())
			cleanupAnswerDocument(t, document, count, 1)
		})
	}
}

// Rejected records are still the command's data when every attempted record
// failed. Exercise both wire shapes through real startup, Dispatch and AnswerFor.
func TestCleanupAnswerDispatcherKeepsFaultOnlyData(t *testing.T) {
	for _, count := range []int{2, rpc.AnswerBufferThreshold + 3} {
		t.Run(strconv.Itoa(count), func(t *testing.T) {
			s := cleanupAnswerDispatcher(t, rpc.AnswerTail{}, cleanupAnswerRows(0, count, nil))
			answer, document := cleanupAnswerDispatchProjection(t, s)
			metadata, err := answer.RecordAnswerMetadata()
			require.NoError(t, err)
			assert.Zero(t, metadata.Count)
			assert.Equal(t, uint64(count), metadata.Faults)
			assert.Equal(t, rpc.VerdictError, answer.Verdict())
			cleanupAnswerDocument(t, document, 0, count)
		})
	}
}

// A plugin that completed its RPC but stopped its evaluation names that fact
// on the terminator. No Go transport error is needed to retain its failure.
func TestCleanupAnswerDispatcherKeepsFailureTerminator(t *testing.T) {
	for _, count := range []int{2, rpc.AnswerBufferThreshold + 3} {
		t.Run(strconv.Itoa(count), func(t *testing.T) {
			const message = "evaluation stopped before all native acknowledgements arrived"
			s := cleanupAnswerDispatcher(t, rpc.AnswerTail{Message: message}, cleanupAnswerRows(count, 0, nil))
			answer, document := cleanupAnswerDispatchProjection(t, s)
			assert.Equal(t, rpc.VerdictAborted, answer.Verdict(), "completed RPC concealed incomplete evaluation")
			assert.Equal(t, message, answer.Message(), "command's own failure was lost")
			metadata, err := answer.RecordAnswerMetadata()
			require.NoError(t, err)
			assert.Equal(t, uint64(count), metadata.Count)
			assert.Zero(t, metadata.Faults)
			cleanupAnswerDocument(t, document, count, 0)
		})
	}
}

// A healthy answer with a warning must reach the operator with the same lines
// and original count, whether the plugin's writer buffered or streamed it.
func TestCleanupAnswerDispatcherKeepsHealthyWarning(t *testing.T) {
	for _, count := range []int{2, rpc.AnswerBufferThreshold + 3} {
		t.Run(strconv.Itoa(count), func(t *testing.T) {
			lines := []string{"cleanup remains owned until its acknowledgement", "retry the retained cleanup"}
			s := cleanupAnswerDispatcher(t, rpc.AnswerTail{Warning: true, Warnings: lines}, cleanupAnswerRows(count, 0, nil))
			answer, document := cleanupAnswerDispatchProjection(t, s)
			assert.Equal(t, rpc.VerdictWarning, answer.Verdict(), "healthy warning became done")
			assert.Equal(t, lines, answer.Warnings(), "original warning lines changed")
			assert.Empty(t, answer.Message())
			metadata, err := answer.RecordAnswerMetadata()
			require.NoError(t, err)
			assert.Equal(t, uint64(count), metadata.Count)
			assert.Zero(t, metadata.Faults)
			cleanupAnswerDocument(t, document, count, 0)
		})
	}
}

// A warning cannot overwrite a rejected-row verdict, but its original lines
// still belong beside that structured partial answer on both wire shapes.
func TestCleanupAnswerDispatcherKeepsWarningsBesideFaults(t *testing.T) {
	for _, count := range []int{2, rpc.AnswerBufferThreshold + 3} {
		t.Run(strconv.Itoa(count), func(t *testing.T) {
			lines := []string{"one cleanup object still requires acknowledgement"}
			s := cleanupAnswerDispatcher(t, rpc.AnswerTail{Warning: true, Warnings: lines}, cleanupAnswerRows(count, 1, nil))
			answer, document := cleanupAnswerDispatchProjection(t, s)
			assert.Equal(t, rpc.VerdictPartial, answer.Verdict())
			assert.Equal(t, lines, answer.Warnings(), "partial result lost its warning lines")
			metadata, err := answer.RecordAnswerMetadata()
			require.NoError(t, err)
			assert.True(t, metadata.Warning)
			assert.Equal(t, uint64(count), metadata.Count)
			assert.Equal(t, uint64(1), metadata.Faults)
			cleanupAnswerDocument(t, document, count, 1)
		})
	}
}

// The shared text surface must report incomplete evaluation as an error after
// consuming the actual response. An aborted answer cannot render as success.
func TestCleanupAnswerDispatcherTextReportsIncompleteEvaluation(t *testing.T) {
	for _, count := range []int{2, rpc.AnswerBufferThreshold + 3} {
		t.Run(strconv.Itoa(count), func(t *testing.T) {
			const message = "evaluation stopped before all native acknowledgements arrived"
			s := cleanupAnswerDispatcher(t, rpc.AnswerTail{Message: message}, cleanupAnswerRows(count, 0, nil))
			dispatch := plugin.CommandDispatcher(func(ctx context.Context, _ plugin.CallerIdentity, line string) (*plugin.Response, error) {
				return s.dispatcher.Dispatch(&CommandContext{Server: s, RequestContext: ctx}, line)
			})
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			rendered, err := dispatch.JSON(ctx, plugin.CallerIdentity{}, "show cleanup answer")
			require.Error(t, err, "text surface presented incomplete evaluation as success")
			assert.Equal(t, message, err.Error())
			require.NotNil(t, rendered)
			require.NotNil(t, rendered.Response, "surface discarded the typed response")
			assert.NotNil(t, rendered.Response.Data, "surface discarded completed progress")
		})
	}
}

// The shared text surface must expose healthy warning lines beside the data it
// rendered, including when those lines arrived after the streamed records.
func TestCleanupAnswerDispatcherTextKeepsWarningLines(t *testing.T) {
	for _, count := range []int{2, rpc.AnswerBufferThreshold + 3} {
		t.Run(strconv.Itoa(count), func(t *testing.T) {
			lines := []string{"cleanup remains owned until its acknowledgement", "retry the retained cleanup"}
			s := cleanupAnswerDispatcher(t, rpc.AnswerTail{Warning: true, Warnings: lines}, cleanupAnswerRows(count, 0, nil))
			dispatch := plugin.CommandDispatcher(func(ctx context.Context, _ plugin.CallerIdentity, line string) (*plugin.Response, error) {
				return s.dispatcher.Dispatch(&CommandContext{Server: s, RequestContext: ctx}, line)
			})
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			rendered, err := dispatch.JSON(ctx, plugin.CallerIdentity{}, "show cleanup answer")
			require.NoError(t, err)
			require.NotNil(t, rendered.Response)
			assert.Equal(t, plugin.StatusWarning, rendered.Response.Status)
			assert.Equal(t, lines, rendered.Response.Warnings)
			cleanupAnswerDocument(t, json.RawMessage(rendered.Output), count, 0)
		})
	}
}

// Dispatch must return the streaming head without waiting for completion, then
// the actual response projection must retain the terminator that arrives late.
func TestCleanupAnswerDispatcherKeepsLateTerminator(t *testing.T) {
	atTail := make(chan struct{})
	release := make(chan struct{})
	unblock := sync.OnceFunc(func() { close(release) })
	defer unblock()
	const count = rpc.AnswerBufferThreshold + 3
	s := cleanupAnswerDispatcher(t, rpc.AnswerTail{Message: "late evaluation failure"},
		cleanupAnswerRows(count, 0, func() { close(atTail); <-release }))
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	response, err := s.dispatcher.Dispatch(&CommandContext{Server: s, RequestContext: ctx}, "show cleanup answer")
	require.NoError(t, err)
	require.NotNil(t, response)
	_, streamed := plugin.RecordRows(response)
	require.True(t, streamed, "Dispatch did not return the streamed head")
	projected := make(chan *rpc.Answer, 1)
	projectionError := make(chan error, 1)
	projectionDone := make(chan struct{})
	t.Cleanup(func() {
		cancel()
		unblock()
		// Completion is independent of the buffered result already read by
		// the success path, so every Fatal/timeout path also joins the worker.
		<-projectionDone
	})
	go func() {
		defer close(projectionDone)
		answer, err := plugin.AnswerFor(response)
		if err != nil {
			projectionError <- err
			return
		}
		// AnswerFor's final walk still has to be consumed before its verdict.
		for range answer.Records {
		}
		projected <- answer
	}()
	select {
	case <-atTail:
	case err := <-projectionError:
		t.Fatal(err)
	case <-ctx.Done():
		t.Fatal("the response never consumed the records preceding the terminator")
	}
	select {
	case <-projected:
		t.Fatal("response completed while the plugin still held its terminator")
	default:
	}
	unblock()
	select {
	case answer := <-projected:
		assert.Equal(t, rpc.VerdictAborted, answer.Verdict())
		assert.Equal(t, "late evaluation failure", answer.Message())
	case err := <-projectionError:
		t.Fatal(err)
	case <-ctx.Done():
		t.Fatal("response did not complete after the plugin released its terminator")
	}
}

// An interrupted RPC keeps the established sanitized public fault, rather than
// treating a missing terminator as completion or exposing the transport cause.
// Cancel only after AnswerFor has accepted all 259 rows; the separate producer
// gate holds the terminator, without claiming that a write proves consumption.
func TestCleanupAnswerDispatcherInterruptedAnswerKeepsSanitizedFault(t *testing.T) {
	atTail := make(chan struct{})
	release := make(chan struct{})
	unblock := sync.OnceFunc(func() { close(release) })
	defer unblock()
	const count = rpc.AnswerBufferThreshold + 3
	s := cleanupAnswerDispatcher(t, rpc.AnswerTail{},
		cleanupAnswerRows(count, 0, func() { close(atTail); <-release }))
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	response, err := s.dispatcher.Dispatch(&CommandContext{Server: s, RequestContext: ctx}, "show cleanup answer")
	require.NoError(t, err)
	require.NotNil(t, response)
	records, streamed := plugin.RecordRows(response)
	require.True(t, streamed, "interruption control did not reach the streamed response")
	acceptedLast := make(chan struct{})
	original := records.Rows
	records.Rows = func(yield func(rpc.RowRecord) bool) {
		accepted := 0
		for row := range original {
			if !yield(row) {
				return
			}
			accepted++
			if accepted == count {
				// AnswerFor's existing heldRecords consumer has appended this
				// row before yield returns true through HeldRecords. Signal
				// after that acceptance, not after a write or Mux enqueue.
				close(acceptedLast)
			}
		}
	}
	response.Data = records
	projected := make(chan *rpc.Answer, 1)
	projectionError := make(chan error, 1)
	projectionDone := make(chan struct{})
	t.Cleanup(func() {
		cancel()
		unblock()
		// Do not read projected/projectionError again: the test may already
		// have consumed its one result before a later assertion fails.
		<-projectionDone
	})
	go func() {
		defer close(projectionDone)
		answer, err := plugin.AnswerFor(response)
		if err != nil {
			projectionError <- err
			return
		}
		projected <- answer
	}()
	bound, boundCancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer boundCancel()
	select {
	case <-atTail:
	case err := <-projectionError:
		t.Fatal(err)
	case <-bound.Done():
		t.Fatal("the writer never reached the held terminator")
	}
	select {
	case <-acceptedLast:
	case err := <-projectionError:
		t.Fatal(err)
	case <-bound.Done():
		t.Fatal("AnswerFor never accepted the last completed record")
	}
	cancel()
	select {
	case answer := <-projected:
		document, err := rpc.CollapseAnswer(answer)
		require.NoError(t, err)
		assert.Equal(t, rpc.VerdictPartial, answer.Verdict())
		metadata, err := answer.RecordAnswerMetadata()
		require.NoError(t, err)
		assert.Equal(t, uint64(count), metadata.Count)
		assert.Equal(t, uint64(1), metadata.Faults)
		var got struct {
			Items  []json.RawMessage `json:"cleanup"`
			Faults []json.RawMessage `json:"errors"`
		}
		require.NoError(t, json.Unmarshal(document, &got))
		require.Len(t, got.Items, count)
		require.Len(t, got.Faults, 1)
		assert.JSONEq(t, `{"message":"plugin answer ended before its terminator","records":259}`, string(got.Faults[0]))
		assert.NotContains(t, string(document), context.Canceled.Error(), "transport cause leaked into public data")
	case err := <-projectionError:
		t.Fatal(err)
	case <-bound.Done():
		t.Fatal("interrupted answer did not release its consumer")
	}
}

// cleanupAnswerDispatcher starts a native recipient through the real process
// manager and five-stage startup. The recipient uses the common wire writer,
// and the production startup sink registers its command. No process-running
// flag, private admission capability or authorizer result is fabricated here.
// The process manager owns and joins this one process at test cleanup.
func cleanupAnswerDispatcher(t *testing.T, tail rpc.AnswerTail, rows iter.Seq[rpc.RowRecord]) *Server {
	t.Helper()
	snapshot := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snapshot) })
	const name = "cleanup-answer-recipient"
	registerLifecyclePlugin(t, name, nil, func(connection net.Conn) int {
		mux := rpc.NewMuxConn(rpc.NewConn(connection, connection))
		defer func() { _ = mux.Close() }()
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		drivePluginHandshakeWith(ctx, mux, &rpc.DeclareRegistrationInput{
			ProtocolVersion: rpc.ProtocolVersion,
			Commands:        []rpc.CommandDecl{{Name: "show cleanup answer", ShortHelp: "show cleanup answer"}},
		}, &rpc.ReadyInput{})
		// The one command and the eventual stop are the only two runtime calls.
		for range 2 {
			request, err := readMuxRequest(ctx, mux)
			if err != nil {
				return 0 // Cleanup closes the captured connection.
			}
			if request.Method == "ze-plugin-callback:bye" {
				if err := mux.SendOK(ctx, request.ID); err != nil {
					return 1
				}
				return 0
			}
			if request.Method != "ze-plugin-callback:execute-command" {
				return 1
			}
			var input rpc.ExecuteCommandInput
			if err := json.Unmarshal(request.Params, &input); err != nil {
				return 1
			}
			if input.Command != "show cleanup answer" {
				return 1
			}
			head := tail
			head.Key = "cleanup"
			if err := rpc.WriteRecordAnswer(mux.AnswerWriter(ctx), request.ID, head, rows); err != nil {
				return 1
			}
		}
		return 0
	})
	var s *Server
	t.Cleanup(func() {
		if s != nil {
			s.wg.Wait()
		}
	})
	s, _ = newLifecycleStartupServer(t)
	require.NoError(t, s.runPluginPhase([]plugin.PluginConfig{{Name: name, Internal: true, Encoder: plugin.EncodingJSON}}))
	s.dispatcher.Registry().Freeze()
	registered := s.dispatcher.Registry().Lookup("show cleanup answer")
	require.NotNil(t, registered, "production startup did not publish the command")
	require.True(t, registered.Process.Running(), "recipient did not complete startup")
	return s
}

// cleanupAnswerRows emits literal item and fault records independently of the
// counts a consumer reports. atTail, when supplied, holds only completion.
func cleanupAnswerRows(items, faults int, atTail func()) iter.Seq[rpc.RowRecord] {
	return func(yield func(rpc.RowRecord) bool) {
		for index := range items {
			row := rpc.RawRow(fmt.Sprintf(`{"ordinal":%d,"phase":"complete"}`, index))
			if !yield(rpc.RowRecord{Item: &row}) {
				return
			}
		}
		for index := range faults {
			row := rpc.RawRow(fmt.Sprintf(`{"ordinal":%d,"cause":"native acknowledgement missing"}`, index))
			if !yield(rpc.RowRecord{Fault: &row}) {
				return
			}
		}
		if atTail != nil {
			atTail()
		}
	}
}

// cleanupAnswerDispatchProjection exercises the real command entry and the
// shared Response projection used by a downstream record-answer surface.
func cleanupAnswerDispatchProjection(t *testing.T, s *Server) (*rpc.Answer, json.RawMessage) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	response, err := s.dispatcher.Dispatch(&CommandContext{Server: s, RequestContext: ctx}, "show cleanup answer")
	require.NoError(t, err)
	require.NotNil(t, response)
	answer, err := plugin.AnswerFor(response)
	require.NoError(t, err)
	document, err := rpc.CollapseAnswer(answer)
	require.NoError(t, err)
	require.NoError(t, answer.Err(), "fixture sent a completed RPC answer")
	return answer, document
}

// cleanupAnswerDocument verifies the structured rows, without deriving an
// authoritative terminator count from a human-facing JSON document.
func cleanupAnswerDocument(t *testing.T, document json.RawMessage, items, faults int) {
	t.Helper()
	var got struct {
		Items  []json.RawMessage `json:"cleanup"`
		Faults []json.RawMessage `json:"errors"`
	}
	require.NoError(t, json.Unmarshal(document, &got))
	require.Len(t, got.Items, items)
	require.Len(t, got.Faults, faults)
	for index, row := range got.Items {
		assert.JSONEq(t, fmt.Sprintf(`{"ordinal":%d,"phase":"complete"}`, index), string(row))
	}
	for index, row := range got.Faults {
		assert.JSONEq(t, fmt.Sprintf(`{"ordinal":%d,"cause":"native acknowledgement missing"}`, index), string(row))
	}
}
