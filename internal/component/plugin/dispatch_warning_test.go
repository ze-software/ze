package plugin

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// warningResponse is the Response a handler returns for a route it accepted
// and sent to no peer (update_text.go, route.ErrNoPeersAcceptedFamily).
func warningResponse() *Response {
	return &Response{Status: "warning", Data: Map{"message": "no peers have family negotiated"}}
}

// TestWriteAnswerWarningRoundTrip checks that a warning Response written by the
// engine's answer writer reaches a socket consumer as a warning. The method:
// the engine end of a pipe reads one request and answers it with WriteAnswer;
// the plugin end reads the answer through rpc.MuxConn.CallAnswer, the reader
// every SDK caller uses, and reads its verdict after the records.
//
// VALIDATES: journal row 2026-10-20 (silent-fall-through) -- the record wire
// states a warning outcome, so the socket and the in-process bridge agree.
// PREVENTS: WriteAnswer dropping Status=warning, so the terminator derived done
// on every socket consumer.
func TestWriteAnswerWarningRoundTrip(t *testing.T) {
	t.Parallel()

	pluginEnd, engineEnd := net.Pipe()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func() {
		engineConn := rpc.NewConn(engineEnd, engineEnd)
		req, err := engineConn.ReadRequest(ctx)
		if err != nil {
			return
		}
		WriteAnswer(engineEnd, req.ID, warningResponse()) //nolint:errcheck // the reader below reports a lost answer as truncated
	}()

	mux := rpc.NewMuxConn(rpc.NewConn(pluginEnd, pluginEnd))
	defer func() {
		if err := mux.Close(); err != nil {
			t.Logf("mux close: %v", err)
		}
	}()

	answer, err := mux.CallAnswer(ctx, rpc.MethodDispatchCommand, nil)
	require.NoError(t, err)

	var items []string
	for record := range answer.Records {
		items = append(items, string(record.Item))
	}
	require.NoError(t, answer.Err(), "the answer reached its terminator")
	assert.Equal(t, []string{`{"message":"no peers have family negotiated"}`}, items,
		"the warning's document is the answer's one record")
	assert.Equal(t, "warning", answer.Verdict(), "a warning Response must arrive as a warning, not as done")
	assert.Empty(t, answer.Message(), "a warning is no failure, so the terminator states no failure text")
}

// TestAnswerForWarning checks that the in-process typed answer slot reads the
// same verdict the socket does for a warning Response. The method: AnswerFor
// over the Response the handler returns, ranging its records before reading
// the verdict, as Answer requires.
//
// VALIDATES: AnswerFor and WriteAnswer make the same decision for one Response.
// PREVENTS: an in-process consumer of the typed slot reading done where the
// socket reads warning.
func TestAnswerForWarning(t *testing.T) {
	t.Parallel()

	answer, err := AnswerFor(warningResponse())
	require.NoError(t, err)
	for range answer.Records {
	}
	assert.Equal(t, "warning", answer.Verdict())
}
