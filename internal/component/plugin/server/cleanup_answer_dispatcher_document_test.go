// Design: docs/architecture/api/process-protocol.md -- interrupted commands.
// Related: command.go -- the bounded process-answer reader.

package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// A genuine document arrives before its connection fails without a terminator.
// The Mux reader drains queued records before its terminal error, so no cancel
// races a merely queued write. All projections must retain that observed
// document and the fixed public fault, without inventing its original row count.
func TestCleanupAnswerDispatcherInterruptedDocumentKeepsObservedProgress(t *testing.T) {
	for _, surface := range []string{"answer", "wire", "text"} {
		t.Run(surface, func(t *testing.T) {
			const original = `{"cleanup":[{"ordinal":0},{"ordinal":1}]}`
			s := cleanupAnswerInterruptedDocument(t, json.RawMessage(original))
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			response, err := s.dispatcher.Dispatch(&CommandContext{Server: s, RequestContext: ctx}, "show cleanup answer")
			require.NoError(t, err)
			require.NotNil(t, response)
			var document json.RawMessage
			switch surface {
			case "answer":
				answer, err := plugin.AnswerFor(response)
				require.NoError(t, err)
				document, err = rpc.CollapseAnswer(answer)
				require.NoError(t, err)
				require.NoError(t, answer.Err())
				assert.Equal(t, rpc.VerdictPartial, answer.Verdict())
				metadata, err := answer.RecordAnswerMetadata()
				require.NoError(t, err)
				assert.Equal(t, uint64(1), metadata.Count, "one document was observed; its original row count is unknown")
				assert.Equal(t, uint64(1), metadata.Faults)
				assert.Empty(t, metadata.Message)
			case "wire":
				var wire bytes.Buffer
				require.NoError(t, plugin.WriteAnswer(&wire, 7, response))
				lines := strings.Split(strings.TrimSuffix(wire.String(), "\n"), "\n")
				require.Len(t, lines, 3)
				for _, line := range lines {
					id, kind, payload, err := rpc.ParseLine([]byte(line))
					require.NoError(t, err)
					assert.Equal(t, uint64(7), id)
					tail, err := rpc.ParseAnswerTail(kind, payload)
					require.NoError(t, err)
					if kind == rpc.AnswerKindRecord {
						document = tail.Item
					}
					if kind == rpc.AnswerKindTerminator {
						assert.Equal(t, uint64(1), tail.Count)
						assert.Equal(t, uint64(1), tail.Faults)
						assert.Equal(t, rpc.VerdictPartial, rpc.Verdict(&tail))
						assert.Empty(t, tail.Message)
					}
				}
			case "text":
				output, err := plugin.ResponseJSON(response, nil)
				require.Error(t, err)
				assert.NotContains(t, err.Error(), "private reader cause")
				assert.NotContains(t, err.Error(), "EOF")
				document = json.RawMessage(output)
			}
			var progress struct {
				Data   []json.RawMessage `json:"data"`
				Faults []json.RawMessage `json:"errors"`
			}
			require.NoError(t, json.Unmarshal(document, &progress))
			require.Len(t, progress.Data, 1)
			assert.JSONEq(t, original, string(progress.Data[0]))
			require.Len(t, progress.Faults, 1)
			assert.JSONEq(t, `{"message":"plugin answer ended before its terminator","records":1}`, string(progress.Faults[0]))
			assert.NotContains(t, string(document), "private reader cause")
			assert.NotContains(t, string(document), "EOF")
		})
	}
}

// This recipient runs the real startup protocol, writes a real document through
// the existing writer, then closes its connection at the terminator. Its
// engine lifetime ends with the test context; the process manager joins it.
func cleanupAnswerInterruptedDocument(t *testing.T, document json.RawMessage) *Server {
	t.Helper()
	snapshot := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snapshot) })
	const name = "cleanup-document-recipient"
	writeResult := make(chan error, 1)
	registerLifecyclePlugin(t, name, nil, func(connection net.Conn) int {
		mux := rpc.NewMuxConn(rpc.NewConn(connection, connection))
		ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
		defer cancel()
		drivePluginHandshakeWith(ctx, mux, &rpc.DeclareRegistrationInput{
			ProtocolVersion: rpc.ProtocolVersion,
			Commands:        []rpc.CommandDecl{{Name: "show cleanup answer", ShortHelp: "show cleanup answer"}},
		}, &rpc.ReadyInput{})
		request, err := readMuxRequest(ctx, mux)
		if err != nil {
			writeResult <- err
			return 1
		}
		writer := cleanupAnswerDocumentDisconnect{writer: mux.AnswerWriter(ctx)}
		writeResult <- rpc.WriteDocumentAnswer(writer, request.ID, rpc.AnswerTail{}, document)
		if err := mux.Close(); err != nil {
			return 1
		}
		<-t.Context().Done()
		return 0
	})
	var s *Server
	t.Cleanup(func() {
		if s != nil {
			s.wg.Wait()
		}
		select {
		case err := <-writeResult:
			assert.ErrorIs(t, err, errCleanupAnswerDocumentEndLost)
		default:
			t.Error("recipient did not reach its deliberately withheld terminator")
		}
	})
	s, _ = newLifecycleStartupServer(t)
	require.NoError(t, s.runPluginPhase([]plugin.PluginConfig{{Name: name, Internal: true, Encoder: plugin.EncodingJSON}}))
	s.dispatcher.Registry().Freeze()
	require.NotNil(t, s.dispatcher.Registry().Lookup("show cleanup answer"))
	return s
}

var errCleanupAnswerDocumentEndLost = errors.New("private reader cause: deliberate missing terminator")

// Reject only the actual end frame, after the same writer sent head and data.
// No invented count or process-admission state enters the fixture.
type cleanupAnswerDocumentDisconnect struct{ writer io.Writer }

func (writer cleanupAnswerDocumentDisconnect) Write(line []byte) (int, error) {
	_, kind, _, err := rpc.ParseLine(bytes.TrimSuffix(line, []byte{'\n'}))
	if err != nil {
		return 0, err
	}
	if kind == rpc.AnswerKindTerminator {
		return 0, errCleanupAnswerDocumentEndLost
	}
	return writer.writer.Write(line)
}
