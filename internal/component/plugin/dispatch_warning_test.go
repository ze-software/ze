package plugin

import (
	"bytes"
	"context"
	"net"
	"strings"
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
		WriteAnswer(engineEnd, req.ID, warningResponse(), rpc.ProtocolVersion) //nolint:errcheck // the reader below reports a lost answer as truncated
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

// TestWriteAnswerWarningCarriesText checks that the warning lines a Response
// states reach the plugin that dispatched the command, on the socket and in
// process. The method: the engine end of a pipe answers with WriteAnswer and
// the plugin end reads through rpc.MuxConn.CallAnswer, then AnswerFor answers
// the same Response in process; both are read after their records.
//
// VALIDATES: journal row 2026-11-02 (format-cannot-hold-a-state-the-tree-holds)
// -- a plugin reads the warning text the operator's surface prints.
// PREVENTS: the wrn terminator stating a warning whose text nobody receives.
func TestWriteAnswerWarningCarriesText(t *testing.T) {
	t.Parallel()

	resp := warningResponse()
	resp.Warnings = []string{`line 3: unknown leaf "x"`, "line 9: deprecated"}

	pluginEnd, engineEnd := net.Pipe()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func() {
		engineConn := rpc.NewConn(engineEnd, engineEnd)
		req, err := engineConn.ReadRequest(ctx)
		if err != nil {
			return
		}
		WriteAnswer(engineEnd, req.ID, resp, rpc.ProtocolVersion) //nolint:errcheck // the reader below reports a lost answer as truncated
	}()

	mux := rpc.NewMuxConn(rpc.NewConn(pluginEnd, pluginEnd))
	defer func() {
		if err := mux.Close(); err != nil {
			t.Logf("mux close: %v", err)
		}
	}()

	answer, err := mux.CallAnswer(ctx, rpc.MethodDispatchCommand, nil)
	require.NoError(t, err)
	for range answer.Records {
	}
	require.NoError(t, answer.Err(), "the answer reached its terminator")
	assert.Equal(t, "warning", answer.Verdict())
	assert.Empty(t, answer.Message(), "the warning text is no failure text")
	assert.Equal(t, resp.Warnings, answer.Warnings(), "the socket carries every warning line")

	inProcess, err := AnswerFor(resp)
	require.NoError(t, err)
	for range inProcess.Records {
	}
	assert.Equal(t, "warning", inProcess.Verdict())
	assert.Equal(t, resp.Warnings, inProcess.Warnings(), "the in-process answer carries the same lines")
}

// TestWriteAnswerWarningVersionOneReader checks that a plugin which negotiated
// protocol version 1, such as a binary built outside the tree on an SDK older
// than the warning lines, is sent exactly the bytes it always read and reads
// status warning, never a failure. The method: WriteAnswer answers one
// Response carrying warning lines for a version 1 reader and the same Response
// with no lines; the two byte streams must match. Each terminator is then read
// with readTerminatorVersionOne, the reading of an SDK before
// rpc.ProtocolVersionWarningLines. The last step reads the version 2 bytes with
// the same old reader and expects the failure, which proves the fixture tells
// the two apart.
//
// VALIDATES: the warning lines are additive on the wire: only a reader that
// negotiated rpc.ProtocolVersionWarningLines receives them.
// PREVENTS: an older plugin turning a command that completed with a warning
// into an error because the wrn message field carried text.
func TestWriteAnswerWarningVersionOneReader(t *testing.T) {
	t.Parallel()

	withLines := warningResponse()
	withLines.Warnings = []string{`line 3: unknown leaf "x"`, "line 9: deprecated"}

	var versionOne, unwarned bytes.Buffer
	require.NoError(t, WriteAnswer(&versionOne, 7, withLines, 1))
	require.NoError(t, WriteAnswer(&unwarned, 7, warningResponse(), 1))
	assert.Equal(t, unwarned.String(), versionOne.String(), "a version 1 reader gets the bytes of an answer with no lines")
	assert.NotContains(t, versionOne.String(), "unknown leaf", "no warning line reaches a version 1 reader")
	assert.Equal(t, rpc.StatusWarning, readTerminatorVersionOne(t, versionOne.String()), "a version 1 reader reads warning")

	var current bytes.Buffer
	require.NoError(t, WriteAnswer(&current, 7, withLines, rpc.ProtocolVersionWarningLines))
	assert.Contains(t, current.String(), "unknown leaf", "a reader that negotiated the lines receives them")
	assert.Equal(t, rpc.StatusError, readTerminatorVersionOne(t, current.String()),
		"the old reader takes text in the wrn field as a failure, which is why the lines are gated")
}

// readTerminatorVersionOne is the status an SDK before
// rpc.ProtocolVersionWarningLines derives from an answer's terminator: it read
// a wrn line with the plain terminator grammar (rpc.ParseAnswerTail of
// AnswerKindTerminator, its parseAnswerTerminator), and its answerValue took
// any message as the command's failure, else read wrn as warning and end as
// done.
func readTerminatorVersionOne(t *testing.T, wire string) string {
	t.Helper()

	lines := strings.Split(strings.TrimSuffix(wire, "\n"), "\n")
	_, line, found := strings.Cut(lines[len(lines)-1], " ")
	require.True(t, found, "terminator line %q names an id", lines[len(lines)-1])
	kind, payload, found := strings.Cut(line, " ")
	require.True(t, found, "terminator line %q names a kind", line)
	tail, err := rpc.ParseAnswerTail(rpc.AnswerKindTerminator, []byte(payload))
	require.NoError(t, err, "the version 1 grammar reads the terminator")
	if tail.Message != "" {
		return rpc.StatusError
	}
	if kind == rpc.AnswerKindWarning {
		return rpc.StatusWarning
	}
	return rpc.StatusDone
}
