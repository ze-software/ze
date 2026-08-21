package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"iter"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDirectBridgeDeliverEvents verifies direct event delivery bypasses socket I/O.
//
// VALIDATES: AC-1 — Plugin's onEvent called directly without JSON-RPC envelope.
// PREVENTS: Events going through JSON marshal + NUL framing + net.Pipe for internal plugins.
func TestDirectBridgeDeliverEvents(t *testing.T) {
	t.Parallel()

	bridge := NewDirectBridge()

	// Register plugin-side event handler
	var received []string
	bridge.SetDeliverEvents(func(events []string) error {
		received = append(received, events...)
		return nil
	})
	bridge.SetReady()

	events := []string{
		`{"type":"bgp","bgp":{"peer":{"address":"10.0.0.1"}}}`,
		`{"type":"bgp","bgp":{"peer":{"address":"10.0.0.2"}}}`,
	}

	err := bridge.DeliverEvents(events)
	require.NoError(t, err)
	assert.Equal(t, events, received)
}

// TestDirectBridgeDispatchRPC verifies direct RPC dispatch bypasses socket I/O.
//
// VALIDATES: AC-2 — Engine dispatcher called directly without JSON marshal or net.Pipe I/O.
// PREVENTS: Plugin→engine RPCs going through JSON + socket for internal plugins.
func TestDirectBridgeDispatchRPC(t *testing.T) {
	t.Parallel()

	bridge := NewDirectBridge()

	// Register engine-side RPC handler
	bridge.SetDispatchRPC(func(method string, params json.RawMessage) (json.RawMessage, error) {
		if method == "ze-plugin-engine:update-route" {
			return json.RawMessage(`{"announced":2,"withdrawn":4}`), nil
		}
		return nil, errors.New("unknown method: " + method)
	})
	bridge.SetReady()

	result, err := bridge.DispatchRPC("ze-plugin-engine:update-route", json.RawMessage(`{"peer-selector":"*","command":"update text origin set igp"}`))
	require.NoError(t, err)
	assert.JSONEq(t, `{"announced":2,"withdrawn":4}`, string(result))
}

// TestDirectBridgeDispatchCommandArgs verifies typed command dispatch preserves
// the exact command name, pre-tokenized args, and peer selector.
//
// VALIDATES: typed inter-plugin dispatch reaches the DirectBridge handler without command-string tokenization.
// PREVENTS: args containing spaces, quotes, or backslashes being split or rejected before the handler sees them.
func TestDirectBridgeDispatchCommandArgs(t *testing.T) {
	t.Parallel()

	bridge := NewDirectBridge()
	args := []string{"peer key with spaces", `quote"inside`, `slash\inside`}

	var gotCommand string
	var gotArgs []string
	var gotPeer string
	bridge.SetDispatchCommandArgs(func(command string, args []string, peer string) (*DispatchCommandOutput, error) {
		gotCommand = command
		gotArgs = append(gotArgs, args...)
		gotPeer = peer
		return &DispatchCommandOutput{
			Status: StatusDone,
			Data:   json.RawMessage(`{"ok":true}`),
		}, nil
	})

	assert.False(t, bridge.HasDispatchCommandArgs(), "handler must not be available before bridge readiness")
	bridge.SetReady()
	assert.True(t, bridge.HasDispatchCommandArgs(), "handler should be available after bridge readiness")

	out, err := bridge.DispatchCommandArgs("bgp rib accept-routes", args, "peer selector")
	require.NoError(t, err)
	require.NotNil(t, out)
	assert.Equal(t, StatusDone, out.Status)
	assert.JSONEq(t, `{"ok":true}`, string(out.Data))
	assert.Equal(t, "bgp rib accept-routes", gotCommand)
	assert.Equal(t, args, gotArgs)
	assert.Equal(t, "peer selector", gotPeer)
}

// TestDirectBridgeExecuteCommand verifies typed execute-command callbacks
// preserve command args without bridge callback JSON.
//
// VALIDATES: engine-to-plugin execute-command reaches the DirectBridge typed handler.
// PREVENTS: internal command forwarding falling back to JSON callback envelopes.
func TestDirectBridgeExecuteCommand(t *testing.T) {
	t.Parallel()

	bridge := NewDirectBridge()
	args := []string{"peer key with spaces", `quote"inside`, `slash\inside`}

	var gotSerial string
	var gotCommand string
	var gotArgs []string
	var gotPeer string
	bridge.SetExecuteCommand(func(serial, command string, args []string, peer string) (*ExecuteCommandOutput, error) {
		gotSerial = serial
		gotCommand = command
		gotArgs = append(gotArgs, args...)
		gotPeer = peer
		return &ExecuteCommandOutput{
			Status: StatusDone,
			Data:   json.RawMessage(`{"ok":true}`),
		}, nil
	})

	assert.False(t, bridge.HasExecuteCommand(), "handler must not be available before bridge readiness")
	bridge.SetReady()
	assert.True(t, bridge.HasExecuteCommand(), "handler should be available after bridge readiness")
	go func() {
		req := <-bridge.ExecuteCommandRequests()
		out, err := bridge.RunExecuteCommand(req)
		req.Result <- ExecuteCommandResult{Output: out, Err: err}
	}()

	out, err := bridge.ExecuteCommand(context.Background(), "serial-1", "bgp rib accept-routes", args, "peer selector")
	require.NoError(t, err)
	require.NotNil(t, out)
	assert.Equal(t, StatusDone, out.Status)
	assert.JSONEq(t, `{"ok":true}`, string(out.Data))
	assert.Equal(t, "serial-1", gotSerial)
	assert.Equal(t, "bgp rib accept-routes", gotCommand)
	assert.Equal(t, args, gotArgs)
	assert.Equal(t, "peer selector", gotPeer)
}

// TestDirectBridgeExecuteCommandCancellation verifies typed execute-command
// preserves caller cancellation while the plugin callback loop is busy.
//
// VALIDATES: ExecuteCommand returns context errors without requiring handler completion.
// PREVENTS: DirectBridge command fast path pinning callers in plugin handlers past timeout.
func TestDirectBridgeExecuteCommandCancellation(t *testing.T) {
	t.Parallel()

	bridge := NewDirectBridge()
	bridge.SetExecuteCommand(func(serial, command string, args []string, peer string) (*ExecuteCommandOutput, error) {
		return &ExecuteCommandOutput{Status: StatusDone}, nil
	})
	bridge.SetReady()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	out, err := bridge.ExecuteCommand(ctx, "", "show-routes", nil, "*")
	require.ErrorIs(t, err, context.Canceled)
	assert.Nil(t, out)

	waitCtx, waitCancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer waitCancel()
	out, err = bridge.ExecuteCommand(waitCtx, "", "show-routes", nil, "*")
	require.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Nil(t, out)
}

// TestDirectBridgeExecuteCommandClosed verifies typed execute-command observes
// bridge shutdown instead of calling plugin handlers after CloseCallbacks.
//
// VALIDATES: ExecuteCommand returns ErrBridgeClosed after bridge callback shutdown.
// PREVENTS: command fast path invoking plugin code after the plugin event loop exited.
func TestDirectBridgeExecuteCommandClosed(t *testing.T) {
	t.Parallel()

	bridge := NewDirectBridge()
	bridge.SetExecuteCommand(func(serial, command string, args []string, peer string) (*ExecuteCommandOutput, error) {
		return &ExecuteCommandOutput{Status: StatusDone}, nil
	})
	bridge.SetReady()
	bridge.CloseCallbacks()

	out, err := bridge.ExecuteCommand(context.Background(), "", "show-routes", nil, "*")
	require.ErrorIs(t, err, ErrBridgeClosed)
	assert.Nil(t, out)
}

// TestDirectBridgeDeliverError verifies error propagation from onEvent.
//
// VALIDATES: AC-5 — Error propagated back to deliverBatch and reflected in EventResult.
// PREVENTS: Errors from plugin event handlers being swallowed by direct transport.
func TestDirectBridgeDeliverError(t *testing.T) {
	t.Parallel()

	bridge := NewDirectBridge()

	handlerErr := errors.New("plugin processing failed")
	bridge.SetDeliverEvents(func(events []string) error {
		return handlerErr
	})
	bridge.SetReady()

	err := bridge.DeliverEvents([]string{`{"event":"test"}`})
	require.Error(t, err)
	assert.Equal(t, handlerErr, err)
}

// TestDirectBridgeDispatchRPCError verifies error propagation from RPC handler.
//
// VALIDATES: AC-6 — Error propagated to SDK caller correctly.
// PREVENTS: Errors from engine RPC handlers being lost in direct transport.
func TestDirectBridgeDispatchRPCError(t *testing.T) {
	t.Parallel()

	bridge := NewDirectBridge()

	bridge.SetDispatchRPC(func(method string, params json.RawMessage) (json.RawMessage, error) {
		return nil, errors.New("dispatch failed")
	})
	bridge.SetReady()

	_, err := bridge.DispatchRPC("ze-plugin-engine:update-route", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dispatch failed")
}

// TestBridgedConnDiscovery verifies SDK discovers bridge via type assertion.
//
// VALIDATES: AC-9 — SDK discovers bridge via BridgedConn type assertion.
// PREVENTS: Bridge reference lost when passing through InternalPluginRunner signature.
func TestBridgedConnDiscovery(t *testing.T) {
	t.Parallel()

	bridge := NewDirectBridge()
	inner1, inner2 := net.Pipe()
	defer closePipe(t, "inner1", inner1)
	defer closePipe(t, "inner2", inner2)

	wrapped := NewBridgedConn(inner1, bridge)

	// Type assertion should discover the bridge
	bridger, ok := wrapped.(Bridger)
	require.True(t, ok, "BridgedConn must implement Bridger")
	assert.Equal(t, bridge, bridger.Bridge())

	// BridgedConn should still work as a net.Conn (compile-time check via Bridger assertion above)
}

// TestBridgedConnFallback verifies plain net.Conn falls back to socket path.
//
// VALIDATES: AC-9 — plain net.Conn falls back to socket transport.
// PREVENTS: Nil bridge panic when external plugin passes plain net.Conn.
func TestBridgedConnFallback(t *testing.T) {
	t.Parallel()

	conn1, conn2 := net.Pipe()
	defer closePipe(t, "conn1", conn1)
	defer closePipe(t, "conn2", conn2)

	// Plain net.Conn should NOT implement Bridger
	_, ok := conn1.(Bridger)
	assert.False(t, ok, "plain net.Conn must not implement Bridger")
}

// TestDirectBridgeNotReady verifies bridge returns error before SetReady.
//
// VALIDATES: AC-4 — Bridge doesn't activate before startup completes.
// PREVENTS: Direct transport racing with 5-stage startup protocol.
func TestDirectBridgeNotReady(t *testing.T) {
	t.Parallel()

	bridge := NewDirectBridge()

	// Register handlers but don't call SetReady
	bridge.SetDeliverEvents(func(events []string) error {
		return nil
	})

	assert.False(t, bridge.Ready(), "bridge should not be ready before SetReady()")

	bridge.SetReady()
	assert.True(t, bridge.Ready(), "bridge should be ready after SetReady()")
}

// TestStructuredEventPool verifies pool get/put cycle and field clearing.
//
// VALIDATES: AC-9 — StructuredEvent pool clears all fields on put (no stale data leaks).
// PREVENTS: Stale data from previous event leaking to next consumer.
func TestStructuredEventPool(t *testing.T) {
	t.Parallel()

	// Get a StructuredEvent, fill all fields
	se := GetStructuredEvent()
	se.PeerAddress = "10.0.0.1"
	se.PeerName = "peer1"
	se.PeerGroup = "group1"
	se.PeerAS = 65001
	se.LocalAS = 65000
	se.LocalAddress = "10.0.0.254"
	se.EventType = EventKindUpdate
	se.Direction = DirectionReceived
	se.MessageID = 42
	se.State = SessionStateUp
	se.Reason = "reconnect"
	se.RawMessage = "sentinel"
	se.Meta = map[string]any{"key": "val"}

	// Return to pool
	PutStructuredEvent(se)

	// Get again — all fields must be cleared
	se2 := GetStructuredEvent()
	assert.Empty(t, se2.PeerAddress, "PeerAddress not cleared")
	assert.Empty(t, se2.PeerName, "PeerName not cleared")
	assert.Empty(t, se2.PeerGroup, "PeerGroup not cleared")
	assert.Zero(t, se2.PeerAS, "PeerAS not cleared")
	assert.Zero(t, se2.LocalAS, "LocalAS not cleared")
	assert.Empty(t, se2.LocalAddress, "LocalAddress not cleared")
	assert.Empty(t, se2.EventType, "EventType not cleared")
	assert.Empty(t, se2.Direction, "Direction not cleared")
	assert.Zero(t, se2.MessageID, "MessageID not cleared")
	assert.Empty(t, se2.State, "State not cleared")
	assert.Empty(t, se2.Reason, "Reason not cleared")
	assert.Nil(t, se2.RawMessage, "RawMessage not cleared")
	assert.Nil(t, se2.Meta, "Meta not cleared")
	PutStructuredEvent(se2)
}

// TestStructuredEventDeliverViaDirectBridge verifies structured event delivery through DirectBridge.
//
// VALIDATES: AC-1 — Internal plugin receives *StructuredEvent with fields populated.
// PREVENTS: StructuredEvent not reaching plugin's OnStructuredEvent handler.
func TestStructuredEventDeliverViaDirectBridge(t *testing.T) {
	t.Parallel()

	bridge := NewDirectBridge()

	var received []any
	bridge.SetDeliverStructured(func(events []any) error {
		received = append(received, events...)
		return nil
	})
	bridge.SetReady()

	se := &StructuredEvent{
		PeerAddress: "10.0.0.1",
		PeerAS:      65001,
		EventType:   EventKindUpdate,
		Direction:   DirectionReceived,
		MessageID:   1,
		RawMessage:  "test-payload",
	}

	err := bridge.DeliverStructured([]any{se})
	require.NoError(t, err)
	require.Len(t, received, 1)

	got, ok := received[0].(*StructuredEvent)
	require.True(t, ok, "received event must be *StructuredEvent")
	assert.Equal(t, "10.0.0.1", got.PeerAddress)
	assert.Equal(t, uint32(65001), got.PeerAS)
	assert.Equal(t, EventKindUpdate, got.EventType)
	assert.Equal(t, DirectionReceived, got.Direction)
	assert.Equal(t, uint64(1), got.MessageID)
	assert.Equal(t, "test-payload", got.RawMessage)
}

// VALIDATES: typed direct dispatch transfers accepted-action completion
// ownership with the returned output.
// PREVENTS: lifecycle teardown running before the result consumer has read it.
func TestDirectBridgeDispatchCommandTransfersCompletionOwnership(t *testing.T) {
	b := NewDirectBridge()
	handlerReturned := false
	completed := false
	b.SetDispatchCommand(func(string) (output *DispatchCommandOutput, err error) {
		defer func() { handlerReturned = true }()
		output = &DispatchCommandOutput{Status: "done"}
		output.OnTransportComplete(func() {
			assert.True(t, handlerReturned)
			completed = true
		})
		return output, nil
	})
	b.SetReady()

	output, err := b.DispatchCommand("request shutdown")

	require.NoError(t, err)
	require.Equal(t, "done", output.Status)
	assert.False(t, completed, "DirectBridge must not complete an action before its caller consumes the result")
	output.TransportComplete()
	assert.True(t, completed)
}

// TestDirectBridgeStopDispatchRejectsPluginCalls verifies bridge shutdown
// closes the plugin-to-engine direction without invoking registered handlers.
//
// VALIDATES: StopDispatch makes generic and typed plugin calls return ErrBridgeClosed.
// PREVENTS: A stopped plugin publishing engine state after runtime cleanup starts.
func TestDirectBridgeStopDispatchRejectsPluginCalls(t *testing.T) {
	b := NewDirectBridge()
	b.SetDispatchRPC(func(string, json.RawMessage) (json.RawMessage, error) {
		t.Fatal("generic handler ran after dispatch shutdown")
		return nil, assert.AnError
	})
	b.SetDispatchCommand(func(string) (*DispatchCommandOutput, error) {
		t.Fatal("typed handler ran after dispatch shutdown")
		return nil, assert.AnError
	})
	b.SetReady()

	b.StopDispatch()

	_, err := b.DispatchRPC("test", nil)
	require.ErrorIs(t, err, ErrBridgeClosed)
	_, err = b.DispatchCommand("show test")
	require.ErrorIs(t, err, ErrBridgeClosed)
	assert.False(t, b.Ready())
}

// TestDirectBridgeWaitDispatchDrainsInflightCall verifies shutdown waits for
// a plugin-to-engine call that entered before dispatch admission closed.
//
// VALIDATES: WaitDispatch returns only after an admitted generic call finishes.
// PREVENTS: Runtime cleanup racing with an in-flight direct bridge handler.
func TestDirectBridgeWaitDispatchDrainsInflightCall(t *testing.T) {
	b := NewDirectBridge()
	entered := make(chan struct{})
	release := make(chan struct{})
	b.SetDispatchRPC(func(string, json.RawMessage) (json.RawMessage, error) {
		close(entered)
		<-release
		return json.RawMessage(`{"status":"done"}`), nil
	})
	b.SetReady()

	callDone := make(chan error, 1)
	go func() {
		_, err := b.DispatchRPC("test", nil)
		callDone <- err
	}()
	<-entered

	b.StopDispatch()
	_, err := b.DispatchRPC("rejected", nil)
	require.ErrorIs(t, err, ErrBridgeClosed)

	waitDone := make(chan struct{})
	go func() {
		b.WaitDispatch()
		close(waitDone)
	}()
	select {
	case <-waitDone:
		t.Fatal("WaitDispatch returned before the admitted call finished")
	default:
	}

	close(release)
	waitCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	select {
	case err := <-callDone:
		require.NoError(t, err)
	case <-waitCtx.Done():
		t.Fatal("admitted direct bridge call did not finish")
	}
	select {
	case <-waitDone:
	case <-waitCtx.Done():
		t.Fatal("WaitDispatch did not return after the admitted call finished")
	}
}

// answerRowSeq yields the rows of one answer, items first and then the rows the
// walk rejected. It is the in-process producer a DirectBridge answer handler
// hands back, and it starts no goroutine: the caller's own range pulls each row
// (ai/rules/goroutine-lifecycle.md).
func answerRowSeq(items, faults []string) iter.Seq[Record] {
	return func(yield func(Record) bool) {
		for _, item := range items {
			if !yield(Record{Item: json.RawMessage(item)}) {
				return
			}
		}
		for _, fault := range faults {
			if !yield(Record{Fault: json.RawMessage(fault)}) {
				return
			}
		}
	}
}

// collectAnswer ranges an answer to its end and returns each row as text, the
// rejected rows marked, so two transports can be compared row for row.
func collectAnswer(answer *Answer) []string {
	var rows []string
	for record := range answer.Records {
		if len(record.Fault) > 0 {
			rows = append(rows, "fault "+string(record.Fault))
			continue
		}
		rows = append(rows, string(record.Item))
	}
	return rows
}

// TestDirectBridgeDispatchCommandAnswer checks that one command answered over
// the in-process bridge and over the socket produces the same answer. The
// method: one row table drives both transports -- a peer writes its lines on a
// pipe that MuxConn.CallAnswer reads, and a bridge handler yields the same rows
// to DirectBridge.DispatchCommandAnswer -- and the two answers are compared row
// for row and verdict for verdict.
//
// VALIDATES: AC-7 -- the same command served over the socket and over
// DirectBridge produces the same row sequence and the same terminator counts.
// PREVENTS: an internal plugin reading a projection where an external one reads
// records. That is what the JSON-shaped in-process path still does, by design
// (serveEngineOpDirect, internal/component/plugin/server/dispatch_registry.go),
// so the typed answer slot is the only thing keeping the two transports equal.
func TestDirectBridgeDispatchCommandAnswer(t *testing.T) {
	t.Parallel()

	const command = "show bgp neighbor summary"
	items := []string{`{"peer":"10.0.0.1"}`, `{"peer":"10.0.0.2"}`}
	faults := []string{`{"peer":"10.0.0.3","reason":"unreadable"}`}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Socket transport: the peer writes the answer this walk produces.
	pluginEnd, engineEnd := net.Pipe()
	defer closePipe(t, "pluginEnd", pluginEnd)
	defer closePipe(t, "engineEnd", engineEnd)

	go func() {
		engineConn := NewConn(engineEnd, engineEnd)
		request, err := engineConn.ReadRequest(ctx)
		if err != nil {
			return
		}
		lines := [][]byte{
			AppendAnswerHead(nil, request.ID, StatusDone, AnswerTypeNDJSON, "peers", nil),
			AppendAnswerItem(nil, request.ID, json.RawMessage(items[0])),
			AppendAnswerItem(nil, request.ID, json.RawMessage(items[1])),
			AppendAnswerFault(nil, request.ID, json.RawMessage(faults[0])),
			AppendAnswerTerminator(nil, request.ID, uint64(len(items)), uint64(len(faults)), ""),
		}
		for _, line := range lines {
			if _, writeErr := engineEnd.Write(append(line, '\n')); writeErr != nil {
				return
			}
		}
	}()

	mux := NewMuxConn(NewConn(pluginEnd, pluginEnd))
	defer func() {
		if err := mux.Close(); err != nil {
			t.Logf("mux close: %v", err)
		}
	}()

	overSocket, err := mux.CallAnswer(ctx, MethodDispatchCommand, &DispatchCommandInput{Command: command})
	require.NoError(t, err)
	socketRows := collectAnswer(overSocket)

	// Bridge transport: the engine hands back the same walk in process.
	bridge := NewDirectBridge()
	bridge.SetDispatchCommandAnswer(func(dispatched string) (*Answer, error) {
		assert.Equal(t, command, dispatched)
		head := AnswerTail{Status: StatusDone, Type: AnswerTypeNDJSON, Key: "peers"}
		terminator := AnswerTail{Count: uint64(len(items)), Faults: uint64(len(faults))}
		return NewAnswer(head, terminator, answerRowSeq(items, faults)), nil
	})
	bridge.SetReady()

	overBridge, err := bridge.DispatchCommandAnswer(command)
	require.NoError(t, err)
	bridgeRows := collectAnswer(overBridge)

	assert.Equal(t, socketRows, bridgeRows, "both transports carry the same rows in the same order")
	assert.Equal(t, overSocket.Key, overBridge.Key, "both heads name the same envelope")
	assert.Equal(t, overSocket.Type, overBridge.Type, "both heads state how a row is read")
	assert.Equal(t, overSocket.Verdict(), overBridge.Verdict(),
		"both answers end on the same terminator counts: two rows produced and one rejected")
}

// TestDirectBridgeWaitDispatchSpansAnswerWalk checks that the dispatch
// admission an answer takes is held until the walk that reads it ends, and is
// released exactly once whether that walk ran out or the consumer stopped it.
// The method: take an answer, stop dispatch, and watch WaitDispatch while the
// records are still unread; then end the range and watch it return.
//
// The rows of an in-process answer are pulled from engine state after
// DispatchCommandAnswer returns, and StopDispatch plus WaitDispatch is the
// rollback barrier that must cover them.
//
// VALIDATES: the admission spans the walk, and the pair releases once on both
// exits from the range.
// PREVENTS: a rollback tearing down the state a live walk is reading, which is
// what releasing at the call would allow. This test fails if the release moves
// back to DispatchCommandAnswer.
func TestDirectBridgeWaitDispatchSpansAnswerWalk(t *testing.T) {
	t.Parallel()

	rows := []string{`{"peer":"10.0.0.1"}`, `{"peer":"10.0.0.2"}`, `{"peer":"10.0.0.3"}`}

	walked := func(t *testing.T, read func(*Answer)) {
		t.Helper()

		b := NewDirectBridge()
		b.SetDispatchCommandAnswer(func(string) (*Answer, error) {
			head := AnswerTail{Status: StatusDone, Type: AnswerTypeNDJSON, Key: "peers"}
			terminator := AnswerTail{Count: uint64(len(rows))}
			return NewAnswer(head, terminator, answerRowSeq(rows, nil)), nil
		})
		b.SetReady()

		answer, err := b.DispatchCommandAnswer("show bgp neighbor summary")
		require.NoError(t, err)

		b.StopDispatch()

		waitDone := make(chan struct{})
		go func() {
			b.WaitDispatch()
			close(waitDone)
		}()

		select {
		case <-waitDone:
			t.Fatal("WaitDispatch returned while the answer's records were still unread")
		case <-time.After(50 * time.Millisecond):
		}

		read(answer)

		select {
		case <-waitDone:
		case <-time.After(time.Second):
			t.Fatal("WaitDispatch did not return after the walk ended")
		}

		// Ranging again must not release the admission a second time: a second
		// endDispatch drives the wait group negative and panics.
		assert.NotPanics(t, func() { collectAnswer(answer) },
			"a second range must not release the admission again")
	}

	t.Run("the walk runs out", func(t *testing.T) {
		t.Parallel()

		walked(t, func(answer *Answer) {
			assert.Equal(t, rows, collectAnswer(answer))
			assert.Equal(t, VerdictDone, answer.Verdict())
		})
	})

	t.Run("the consumer stops", func(t *testing.T) {
		t.Parallel()

		walked(t, func(answer *Answer) {
			read := 0
			for range answer.Records {
				read++
				break
			}
			assert.Equal(t, 1, read)
			assert.Equal(t, VerdictTruncated, answer.Verdict())
		})
	})
}
