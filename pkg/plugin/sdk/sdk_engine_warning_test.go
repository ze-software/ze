package sdk

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestDispatchCommandReadsWarningTerminator checks that a plugin dispatching an
// engine command over the socket reads a warning outcome as `warning`, the
// status the in-process bridge hands the same caller. The method: complete the
// five-stage startup against a fake engine, and have the engine answer the
// dispatch with a document head, the one document, and a terminator spelled
// `wrn` (rpc.AnswerKindWarning). The lines are written as bytes rather than
// through the appenders, so the test states the wire it expects.
//
// VALIDATES: journal row 2026-10-20 (silent-fall-through) -- a warning
// Response reaches a socket consumer as warning, never as done.
// PREVENTS: answerValue returning StatusDone for a route the engine sent to no
// peer, which only the in-process DirectBridge reported correctly.
func TestDispatchCommandReadsWarningTerminator(t *testing.T) {
	t.Parallel()

	const document = `{"message":"no peers have family negotiated"}`

	p, engine := newTestPair(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- p.Run(ctx, &Registration{})
	}()

	req := readEngineRequest(t, ctx, engine.mux)
	require.Equal(t, rpc.MethodDeclareRegistration, req.Method)
	require.NoError(t, engine.mux.SendOK(ctx, req.ID))

	completeStartupFromStage2(t, ctx, engine)

	// The engine answers the one dispatch this test makes. It touches no
	// *testing.T, because the test goroutine can leave before this one runs.
	go func() {
		var request *rpc.Request
		select {
		case request = <-engine.mux.Requests():
		case <-ctx.Done():
			return
		}
		id := "#" + strconv.FormatUint(request.ID, 10) + " "
		lines := [][]byte{
			[]byte(id + "top doc 0: 0:\n"),
			[]byte(id + "row " + strconv.Itoa(len(document)) + ":" + document + "\n"),
			[]byte(id + "wrn 1 0 0:\n"),
		}
		if err := writeAnswerLines(ctx, engine.mux, lines); err != nil {
			return
		}
	}()

	status, data, err := p.DispatchCommand(ctx, "update text nlri ipv4/flow destination 10.0.0.0/24")
	require.NoError(t, err, "a warning is an outcome the command completed with, never a failure")
	assert.Equal(t, "warning", status, "the socket must state what DirectBridge states for the same answer")
	assert.Equal(t, document, string(data), "the warning's document still reaches the caller")

	shutdownPlugin(t, ctx, engine, errCh)
}
