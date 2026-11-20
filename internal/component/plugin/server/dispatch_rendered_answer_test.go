// Design: docs/architecture/api/json-format.md — what `data` holds for a piped line
// Related: command.go -- applyAnswerStep and renderedAnswer, the producer under test.

package server

import (
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestRenderedAnswerMarshalsAsJSONString asks whether the engine's rendered
// answer marshals to the text as one JSON string, the data D-6 decided, while
// plugin.RawJSON keeps refusing the same string from a handler.
//
// Method: marshal a response carrying a renderedAnswer of a two-line table and
// decode it back; marshal the quoted text as RawJSON and expect the refusal.
//
// Red before the fix: applyAnswerStep carried the text as RawJSON, so the
// first marshal failed with "must be a JSON object or an array".
func TestRenderedAnswerMarshalsAsJSONString(t *testing.T) {
	t.Parallel()
	const text = "NAME  STATE\na     up\n"
	encoded, err := plugin.ResponseJSON(&plugin.Response{Status: plugin.StatusDone, Data: renderedAnswer{text: text}}, nil)
	require.NoError(t, err)
	var decoded string
	require.NoError(t, json.Unmarshal([]byte(encoded), &decoded), "the data is one JSON string")
	assert.Equal(t, text, decoded)

	quoted, err := json.Marshal(text)
	require.NoError(t, err)
	_, err = json.Marshal(plugin.RawJSON(quoted))
	require.Error(t, err, "RawJSON still refuses finished text from a handler")
}

// TestDispatchJSONFormatAnswersTheDocument asks whether a chain ending in a
// format that prints JSON (`| json`, `| raw`) answers a data caller the JSON
// document the rendering surface prints, not that document quoted as a string.
//
// Method: Dispatch each line for a data caller, then compare the data with what
// the surface's formatter prints for the same line over the same rows.
//
// Red before the fix: ApplyAnswerStep called every format text, so the
// document was quoted into RawJSON and the marshal was refused.
func TestDispatchJSONFormatAnswersTheDocument(t *testing.T) {
	t.Parallel()
	for _, line := range []string{"test rows | json", "test rows | raw", "test rows | count | json"} {
		t.Run(line, func(t *testing.T) {
			t.Parallel()
			d, calls := rowsTarget(t)
			resp, err := d.Dispatch(&CommandContext{Username: "alice"}, line)
			require.NoError(t, err)
			assert.Equal(t, int32(1), calls.Load())
			assert.IsType(t, plugin.RawJSON(""), resp.Data, "a JSON format answers a document")
			assert.JSONEq(t, surfaceAnswer(t, line, rowsDocument), dataOf(t, resp),
				"the document the rendering surface prints for the line")
		})
	}
}

// TestDispatchCommandCarriesRenderedText asks whether a plugin's
// dispatch-command of a line with a text format receives the rendered text as
// a JSON string on every transport: the DirectBridge value, the DirectBridge
// answer and the socket (T-1, AC-5).
//
// Method: one rows handler; `test rows | table` sent through each transport;
// each data decoded as a string and compared with the surface's text.
//
// Red before the fix: the response could not be marshaled (RawJSON refused the
// quoted text), so each transport answered an error.
func TestDispatchCommandCarriesRenderedText(t *testing.T) {
	t.Parallel()
	const line = "test rows | table"
	want := surfaceAnswer(t, line, rowsDocument)

	t.Run("dispatch-command", func(t *testing.T) {
		t.Parallel()
		d, _ := rowsTarget(t)
		s := &Server{subscriptions: newSubscriptionManager(), dispatcher: d}
		s.ctx, s.cancel = context.WithCancel(context.Background())
		defer s.cancel()
		proc := process.NewProcess(plugin.PluginConfig{Name: "rendered-bridge"})
		out, err := pipeChainBridge(s, proc).DispatchCommand(t.Context(), line)
		require.NoError(t, err)
		require.NotNil(t, out)
		assert.Equal(t, plugin.StatusDone, out.Status, out.Error)
		var text string
		require.NoError(t, json.Unmarshal(out.Data, &text), "the data is one JSON string")
		assert.Equal(t, want, text)
	})

	t.Run("dispatch-command answer", func(t *testing.T) {
		t.Parallel()
		d, _ := rowsTarget(t)
		s := &Server{subscriptions: newSubscriptionManager(), dispatcher: d}
		s.ctx, s.cancel = context.WithCancel(context.Background())
		defer s.cancel()
		proc := process.NewProcess(plugin.PluginConfig{Name: "rendered-answer"})
		answer, err := pipeChainBridge(s, proc).DispatchCommandAnswer(t.Context(), line)
		require.NoError(t, err)
		require.NotNil(t, answer)
		var items [][]byte
		for record := range answer.Records {
			items = append(items, record.Item)
		}
		require.Len(t, items, 1, "the text is one document")
		var text string
		require.NoError(t, json.Unmarshal(items[0], &text))
		assert.Equal(t, want, text)
	})

	t.Run("socket", func(t *testing.T) {
		t.Parallel()
		d, _ := rowsTarget(t)
		pluginSide, engineSide := net.Pipe()
		t.Cleanup(func() { _ = pluginSide.Close() })
		t.Cleanup(func() { _ = engineSide.Close() })
		proc := process.NewProcess(plugin.PluginConfig{Name: "rendered-socket"})
		proc.SetConn(ipc.NewPluginConn(engineSide, engineSide))
		s := &Server{subscriptions: newSubscriptionManager(), dispatcher: d}
		s.ctx, s.cancel = context.WithCancel(context.Background())
		defer s.cancel()
		handlerDone := make(chan struct{})
		go func() {
			defer close(handlerDone)
			s.handleSingleProcessCommandsRPC(proc)
		}()

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		status, data, err := dispatchAnswerOverSocket(ctx, pluginSide, &rpc.DispatchCommandInput{Command: line})
		require.NoError(t, err)
		assert.Equal(t, rpc.StatusDone, status)
		var text string
		require.NoError(t, json.Unmarshal(data, &text), "the data is one JSON string")
		assert.Equal(t, want, text)

		require.NoError(t, pluginSide.Close())
		select {
		case <-handlerDone:
		case <-time.After(2 * time.Second):
			t.Fatal("handler did not exit")
		}
	})
}
