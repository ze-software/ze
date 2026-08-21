package sdk

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// answerLines are the lines a peer writes for one record answer under id: the
// head, one line for each row, and the terminator carrying the counts. It is
// the wire WriteAnswer produces (internal/component/plugin/dispatch.go), spelled
// here so the SDK's reader is driven by the bytes and not by that producer.
func answerLines(id uint64, key string, items, faults []string) [][]byte {
	lines := make([][]byte, 0, len(items)+len(faults)+2)
	lines = append(lines, rpc.AppendAnswerHead(nil, id, rpc.StatusDone, rpc.AnswerTypeNDJSON, key, nil))
	for _, item := range items {
		lines = append(lines, rpc.AppendAnswerItem(nil, id, []byte(item)))
	}
	for _, fault := range faults {
		lines = append(lines, rpc.AppendAnswerFault(nil, id, []byte(fault)))
	}
	lines = append(lines, rpc.AppendAnswerTerminator(nil, id, uint64(len(items)), uint64(len(faults)), ""))
	for i := range lines {
		lines[i] = append(lines[i], '\n')
	}
	return lines
}

// documentAnswerLines are the lines a peer writes for the answer of a walk that
// ended within rpc.AnswerBufferThreshold records: a head naming type=json, the
// one document that walk collapsed to, and the terminator carrying the walk's
// counts. It is the wire writeDocumentAnswer produces
// (internal/component/plugin/dispatch.go).
func documentAnswerLines(id uint64, document string, count, faults uint64) [][]byte {
	lines := [][]byte{rpc.AppendAnswerHead(nil, id, rpc.StatusDone, rpc.AnswerTypeJSON, "", nil)}
	if document != "" {
		lines = append(lines, rpc.AppendAnswerItem(nil, id, []byte(document)))
	}
	lines = append(lines, rpc.AppendAnswerTerminator(nil, id, count, faults, ""))
	for i := range lines {
		lines[i] = append(lines[i], '\n')
	}
	return lines
}

// writeAnswerLines puts every line of one answer on the engine's answer writer.
func writeAnswerLines(ctx context.Context, mux *rpc.MuxConn, lines [][]byte) error {
	writer := mux.AnswerWriter(ctx)
	for _, line := range lines {
		if _, err := writer.Write(line); err != nil {
			return err
		}
	}
	return nil
}

// TestDispatchCommandAnswerYieldsRows checks that a plugin dispatching an engine
// command receives the rows of the answer one at a time, in the order the walk
// produced them. The method: complete the five-stage startup against a fake
// engine, dispatch one command, and have the engine answer with a head, three
// record lines and a terminator.
//
// VALIDATES: AC-2 -- the plugin receives every row in walk order and the
// terminator's verdict, holding one row at a time rather than the collection.
// PREVENTS: an SDK plugin having no way to consume a record answer, so
// MuxConn.CallAnswer stays a reader with no caller outside its own test.
func TestDispatchCommandAnswerYieldsRows(t *testing.T) {
	t.Parallel()

	const command = "show bgp neighbor summary"
	items := []string{`{"peer":"10.0.0.1"}`, `{"peer":"10.0.0.2"}`}
	faults := []string{`{"peer":"10.0.0.3","reason":"unreadable"}`}

	p, engine := newTestPair(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- p.Run(ctx, Registration{})
	}()

	req := readEngineRequest(t, ctx, engine.mux)
	assert.Equal(t, "ze-plugin-engine:declare-registration", req.Method)
	require.NoError(t, engine.mux.SendOK(ctx, req.ID))

	completeStartupFromStage2(t, ctx, engine)

	// The engine answers the one dispatch this test makes. It touches no *testing.T:
	// the test goroutine can leave on a failed assertion before this one runs, and
	// a t.Fatal after that would fail the run in another test's name.
	dispatched := make(chan string, 1)
	go func() {
		var request *rpc.Request
		select {
		case request = <-engine.mux.Requests():
		case <-ctx.Done():
			return
		}
		dispatched <- request.Method
		writer := engine.mux.AnswerWriter(ctx)
		for _, line := range answerLines(request.ID, "peers", items, faults) {
			if _, err := writer.Write(line); err != nil {
				return
			}
		}
	}()

	answer, err := p.DispatchCommandAnswer(ctx, command)
	require.NoError(t, err, "the SDK must be able to ask for a command's answer as records")

	var got []string
	for record := range answer.Records {
		if len(record.Fault) > 0 {
			got = append(got, "fault "+string(record.Fault))
			continue
		}
		got = append(got, string(record.Item))
	}

	want := []string{items[0], items[1], "fault " + faults[0]}
	assert.Equal(t, want, got, "every row of the answer reaches the plugin, in walk order")
	assert.Equal(t, "peers", answer.Key, "the head names the envelope the rows belong under")
	assert.Equal(t, rpc.VerdictPartial, answer.Verdict(), "two rows produced and one rejected is a partial walk")
	assert.NoError(t, answer.Err(), "an answer that reached its terminator ended with no fault")

	select {
	case method := <-dispatched:
		assert.Equal(t, rpc.MethodDispatchCommand, method)
	case <-time.After(time.Second):
		t.Fatal("the engine received no dispatch-command request")
	}

	shutdownPlugin(t, ctx, engine, errCh)
}

// TestDispatchCommandAnswerBoundedIsDocument checks that a walk ending at or
// under rpc.AnswerBufferThreshold still reads as ONE document, and that the
// value the plugin sees is the value the same command produced before the
// record frame existed. The method: complete the five-stage startup against a
// fake engine, answer one dispatch with the head, item and terminator
// writeDocumentAnswer writes for a bounded walk, and read it through both the
// answer-returning call and the value-returning one.
//
// VALIDATES: AC-3 -- a bounded walk is one rpc.AnswerTypeJSON record whose item
// is the whole document, and DispatchCommand rebuilds that document byte for
// byte.
// PREVENTS: the threshold turning a short answer into a row sequence, which
// would change the payload of every existing command for no gain.
func TestDispatchCommandAnswerBoundedIsDocument(t *testing.T) {
	t.Parallel()

	const document = `{"peers":[{"peer":"10.0.0.1"},{"peer":"10.0.0.2"}]}`

	p, engine := newTestPair(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- p.Run(ctx, Registration{})
	}()

	req := readEngineRequest(t, ctx, engine.mux)
	require.Equal(t, "ze-plugin-engine:declare-registration", req.Method)
	require.NoError(t, engine.mux.SendOK(ctx, req.ID))

	completeStartupFromStage2(t, ctx, engine)

	// The engine answers both dispatches this test makes. It touches no
	// *testing.T: the test goroutine can leave on a failed assertion before
	// this one runs, and a t.Fatal after that would fail the run in another
	// test's name.
	go func() {
		for {
			var request *rpc.Request
			select {
			case request = <-engine.mux.Requests():
			case <-ctx.Done():
				return
			}
			// Two rows produced, so the terminator counts two even though one
			// record carried them: the counts are the walk's, not the wire's.
			if err := writeAnswerLines(ctx, engine.mux, documentAnswerLines(request.ID, document, 2, 0)); err != nil {
				return
			}
		}
	}()

	answer, err := p.DispatchCommandAnswer(ctx, "show bgp neighbor summary")
	require.NoError(t, err)
	assert.Equal(t, rpc.AnswerTypeJSON, answer.Type, "a bounded walk states it is one document")

	var records []string
	for record := range answer.Records {
		records = append(records, string(record.Item))
	}
	assert.Equal(t, []string{document}, records, "the answer is one record carrying the whole document")
	assert.Equal(t, rpc.VerdictDone, answer.Verdict())
	assert.NoError(t, answer.Err())

	status, data, err := p.DispatchCommand(ctx, "show bgp neighbor summary")
	require.NoError(t, err)
	assert.Equal(t, rpc.StatusDone, status)
	assert.Equal(t, document, string(data), "the value equals what the command produced before this spec")

	shutdownPlugin(t, ctx, engine, errCh)
}

// TestCollapseAnswerRefusesAnUnreadableDocument checks that the buffered
// reading of an answer refuses a record it cannot hand on as JSON. The method:
// three answers a peer can write and no consumer can use -- a document that is
// not JSON, a type=json answer carrying two records, and one carrying a
// rejected row -- are collapsed, and each is expected to be named rather than
// forwarded.
//
// A record line's item= reaches the reader unread (ParseAnswerTail,
// pkg/plugin/rpc/message.go), which is what lets a forwarding consumer parse
// nothing. A consumer that hands the bytes on as a value cannot take that
// unread, so the check lives at the point where the value is built.
//
// VALIDATES: the Security Review Checklist's input-validation row for the read
// side: a row that is not valid JSON is refused rather than forwarded.
// PREVENTS: a plugin unmarshaling whatever a peer put after `item=`, which the
// whole-document unmarshal this replaced would have refused.
func TestCollapseAnswerRefusesAnUnreadableDocument(t *testing.T) {
	t.Parallel()

	head := rpc.AnswerTail{Status: rpc.StatusDone, Type: rpc.AnswerTypeJSON}

	cases := []struct {
		name    string
		records []rpc.Record
		want    string
	}{
		{
			name:    "a document that is not JSON",
			records: []rpc.Record{{Item: []byte(`{"peer":`)}},
			want:    "not JSON",
		},
		{
			name:    "two records under one document",
			records: []rpc.Record{{Item: []byte(`{"a":1}`)}, {Item: []byte(`{"b":2}`)}},
			want:    "want at most one",
		},
		{
			name:    "a rejected row under one document",
			records: []rpc.Record{{Fault: []byte(`{"reason":"unreadable"}`)}},
			want:    "carries a rejected row",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rows := slices.Values(tc.records)
			terminator := rpc.AnswerTail{Count: uint64(len(tc.records))}
			_, err := rpc.CollapseAnswer(rpc.NewAnswer(head, terminator, rows))
			require.Error(t, err, "the reader must name a record it cannot hand on")
			assert.Contains(t, err.Error(), tc.want)
		})
	}

	// The same reader accepts the document a bounded walk really produces.
	document := `{"peers":[{"peer":"10.0.0.1"}]}`
	rows := slices.Values([]rpc.Record{{Item: []byte(document)}})
	got, err := rpc.CollapseAnswer(rpc.NewAnswer(head, rpc.AnswerTail{Count: 1}, rows))
	require.NoError(t, err)
	assert.Equal(t, document, string(got))
}
