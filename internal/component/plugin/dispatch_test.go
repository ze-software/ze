package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// sampleData is a typed ResponseData payload used to check that ResponseJSON
// marshals Data exactly as the historical hub adapters did (json.Marshal).
type sampleData struct {
	DataMarker
	PeerCount int    `json:"peer-count"`
	Name      string `json:"name"`
}

// VALIDATES: AC-3 / R-3 -- the single flatten helper produces the exact client
// bytes the two old hub adapters produced for done / error / nil / nil-Data /
// typed-Data cases.
// PREVENTS: byte-drift on the text surfaces after collapsing the two adapters.
func TestResponseJSON(t *testing.T) {
	tests := []struct {
		name    string
		resp    *Response
		err     error
		want    string
		wantErr string
	}{
		{
			name: "typed data",
			resp: NewResponse(StatusDone, &sampleData{PeerCount: 3, Name: "edge"}),
			want: `{"peer-count":3,"name":"edge"}`,
		},
		{
			name: "raw json data passes through",
			resp: NewResponse(StatusDone, RawJSON(`{"a":1}`)),
			want: `{"a":1}`,
		},
		{
			// The one escape from the structured-data invariant was a Text
			// payload that ResponseJSON returned verbatim. It is gone: a
			// payload carrying newlines is marshaled like any other, so
			// "| json", "| yaml" and "| table" all have data to render.
			name: "text-carrying data is marshaled, never passed through verbatim",
			resp: NewResponse(StatusDone, Map{"lines": "line one\nline two"}),
			want: `{"lines":"line one\nline two"}`,
		},
		{
			name: "nil response yields empty",
			resp: nil,
			want: "",
		},
		{
			name: "done with nil data yields empty",
			resp: &Response{Status: StatusDone},
			want: "",
		},
		{
			name:    "error message surfaces as go error",
			resp:    newErrorResponse("boom"),
			wantErr: "boom",
		},
		{
			name:    "status error without message is unknown error",
			resp:    &Response{Status: StatusError},
			wantErr: "unknown error",
		},
		{
			name:    "dispatch error propagates",
			resp:    nil,
			err:     errors.New("server not ready"),
			wantErr: "server not ready",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ResponseJSON(tc.resp, tc.err)
			if tc.wantErr != "" {
				if err == nil || err.Error() != tc.wantErr {
					t.Fatalf("want error %q, got %v", tc.wantErr, err)
				}
				if got != "" {
					t.Fatalf("want empty output on error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("want %q, got %q", tc.want, got)
			}
		})
	}
}

// VALIDATES: CommandDispatcher.JSON dispatches then flattens via ResponseJSON,
// threading the caller and command through to the underlying dispatcher.
// PREVENTS: text surfaces diverging from the shared flatten path.
func TestCommandDispatcherJSON(t *testing.T) {
	var gotCaller CallerIdentity
	var gotCmd string
	completed := false
	d := CommandDispatcher(func(_ context.Context, caller CallerIdentity, cmd string) (*Response, error) {
		gotCaller = caller
		gotCmd = cmd
		resp := NewResponse(StatusDone, RawJSON(`{"ok":true}`))
		resp.OnTransportComplete(func() { completed = true })
		return resp, nil
	})

	out, err := d.JSON(context.Background(), CallerIdentity{Username: "admin", Surface: "web"}, "show status")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Output != `{"ok":true}` {
		t.Fatalf("want flattened data, got %q", out.Output)
	}
	if completed {
		t.Fatal("JSON conversion completed the response before its transport wrote it")
	}
	out.TransportComplete()
	if !completed {
		t.Fatal("transport completion did not release the accepted action")
	}
	if gotCmd != "show status" {
		t.Fatalf("command not threaded: %q", gotCmd)
	}
	if gotCaller.Username != "admin" || gotCaller.Surface != "web" {
		t.Fatalf("caller not threaded: %+v", gotCaller)
	}
}

// VALIDATES: JSON surfaces an error Response as a Go error with empty output.
// PREVENTS: error responses being rendered as command output on text surfaces.
func TestCommandDispatcherJSONError(t *testing.T) {
	d := CommandDispatcher(func(_ context.Context, _ CallerIdentity, _ string) (*Response, error) {
		return newErrorResponse("denied"), nil
	})
	out, err := d.JSON(context.Background(), CallerIdentity{}, "request reload")
	if err == nil || err.Error() != "denied" {
		t.Fatalf("want denied error, got %v", err)
	}
	if out.Output != "" {
		t.Fatalf("want empty output, got %q", out.Output)
	}
	out.TransportComplete()
}

// TestGeneratorAnswerReachesTheEncoder checks that a handler answering with a
// row generator reaches the answer encoder, and that the encoder writes the one
// shape every answer has. The method: a generator of one row past the buffering
// threshold is written to a buffer, and its first and last lines are compared
// with the wire grammar.
//
// VALIDATES: AC-6 -- a walk past the threshold produces one item line for each
// row and a terminator carrying count=N, with no count stated before it.
// PREVENTS: a generator payload being flattened into one line, which is the
// whole-answer materialization this protocol exists to remove.
func TestGeneratorAnswerReachesTheEncoder(t *testing.T) {
	rowCount := answerBufferThreshold + 1

	var answer bytes.Buffer
	if err := WriteAnswer(&answer, 7, NewResponse(StatusDone, Records{Key: "peers", Rows: peerRows(rowCount)})); err != nil {
		t.Fatalf("WriteAnswer: %v", err)
	}

	got := strings.Split(strings.TrimSuffix(answer.String(), "\n"), "\n")
	if len(got) != rowCount+2 {
		t.Fatalf("answer has %d lines, want %d: a head, %d records and a terminator", len(got), rowCount+2, rowCount)
	}

	want := map[int]string{
		0:            "#7 ok status=done type=ndjson key=peers",
		1:            `#7 ok item={"peer":"10.0.0.0"}`,
		2:            `#7 ok item={"peer":"10.0.0.1"}`,
		rowCount:     `#7 ok item={"peer":"10.0.0.` + strconv.Itoa(rowCount-1) + `"}`,
		rowCount + 1: "#7 ok count=" + strconv.Itoa(rowCount),
	}
	for index, line := range want {
		if got[index] != line {
			t.Errorf("answer line %d is %q, want %q", index+1, got[index], line)
		}
	}
}

// answerLine is one line of an answer as a consumer holds it: the id and verb
// rpc.ParseLine cut off, and the tail rpc.ParseAnswerTail decoded. The tests
// below read an answer with the shipped reader rather than with string
// matching, so a grammar change that breaks a consumer breaks them too.
type answerLine struct {
	id   uint64
	verb string
	tail rpc.AnswerTail
}

// readAnswer decodes every line WriteAnswer produced.
func readAnswer(t *testing.T, wire []byte) []answerLine {
	t.Helper()
	text := strings.TrimSuffix(string(wire), "\n")
	if text == "" {
		return nil
	}
	var lines []answerLine
	for raw := range strings.SplitSeq(text, "\n") {
		id, verb, payload, err := rpc.ParseLine([]byte(raw))
		if err != nil {
			t.Fatalf("ParseLine(%q): %v", raw, err)
		}
		tail, err := rpc.ParseAnswerTail(payload)
		if err != nil {
			t.Fatalf("ParseAnswerTail(%q): %v", raw, err)
		}
		lines = append(lines, answerLine{id: id, verb: verb, tail: tail})
	}
	return lines
}

// assertAnswerShape checks the one shape every answer has: a head carrying the
// verdict and the type, then the records, then a terminator carrying the
// counts. It takes the record counts as arguments and branches on nothing else,
// which is the property AC-2 states: a reader of a one-record answer runs the
// same code as a reader of a thousand-record one.
//
// The counts come from the terminator rather than from the line count, because
// the type the encoder chose decides how many lines carry them: a walk within
// the buffering threshold puts every record in one document.
func assertAnswerShape(t *testing.T, lines []answerLine, wantID, wantItems, wantFaults uint64) {
	t.Helper()
	if len(lines) < 2 {
		t.Fatalf("answer has %d lines, want a head and a terminator at least", len(lines))
	}
	for i := range lines {
		if lines[i].id != wantID {
			t.Errorf("line %d carries id %d, want %d", i+1, lines[i].id, wantID)
		}
		if lines[i].verb != rpc.StatusOK {
			t.Errorf("line %d carries verb %q, want %q", i+1, lines[i].verb, rpc.StatusOK)
		}
	}

	head := lines[0].tail
	if head.Status == "" {
		t.Error("the head states no status=, so a consumer must buffer the answer to know how to render it")
	}
	if head.Type == "" {
		t.Error("the head states no type=, so a consumer cannot read the items that follow it")
	}
	if head.IsTerminator() {
		t.Error("the head carries count=, so a reader cannot tell it from the terminator")
	}
	records := lines[1 : len(lines)-1]
	for i := range records {
		tail := &records[i].tail
		if len(tail.Item) == 0 && len(tail.Fault) == 0 {
			t.Errorf("record %d carries neither item= nor fault=", i+1)
		}
		if tail.IsTerminator() {
			t.Errorf("record %d carries count=, which only the terminator carries", i+1)
		}
		if tail.Status != "" {
			t.Errorf("record %d carries status=, which only the head carries", i+1)
		}
	}

	terminator := lines[len(lines)-1].tail
	if !terminator.IsTerminator() {
		t.Fatalf("the last line carries no count=, so the answer reads as truncated: %+v", terminator)
	}
	if terminator.Count != wantItems {
		t.Errorf("the terminator states count=%d, want %d", terminator.Count, wantItems)
	}
	if terminator.Faults != wantFaults {
		t.Errorf("the terminator states faults=%d, want %d", terminator.Faults, wantFaults)
	}
}

// peerRows is a generator of count result records. It stops when the consumer
// stops, which is what a walk over a table nobody wants whole must do.
func peerRows(count int) iter.Seq[rpc.Record] {
	return func(yield func(rpc.Record) bool) {
		for i := range count {
			item := json.RawMessage(`{"peer":"10.0.0.` + strconv.Itoa(i) + `"}`)
			if !yield(rpc.Record{Item: item}) {
				return
			}
		}
	}
}

// peerColumns is the column schema columnRows produces its rows against.
var peerColumns = []string{"peer", "as", "state"}

// columnRows is a generator of count positional rows, each an array of values
// in peerColumns order. It is what a handler that declares its columns yields:
// the names live on the head, and the rows carry values alone.
func columnRows(count int) iter.Seq[rpc.Record] {
	return func(yield func(rpc.Record) bool) {
		for i := range count {
			row := json.RawMessage(`["10.0.0.` + strconv.Itoa(i) + `",6500` + strconv.Itoa(i%10) + `,"established"]`)
			if !yield(rpc.Record{Item: row}) {
				return
			}
		}
	}
}

// mixedRows is a generator of itemCount result records and faultCount rejected
// ones, interleaved rather than arriving in two blocks. The walk ends when both
// pools are empty, and each pass takes one row from one of them.
func mixedRows(itemCount, faultCount int) iter.Seq[rpc.Record] {
	return func(yield func(rpc.Record) bool) {
		items, faults := itemCount, faultCount
		for i := 0; items > 0 || faults > 0; i++ {
			leaf := strconv.Itoa(i)
			record := rpc.Record{Item: json.RawMessage(`{"leaf":` + leaf + `}`)}
			if faults > 0 && (items == 0 || i%2 == 1) {
				faults--
				record = rpc.Record{Fault: json.RawMessage(`{"leaf":` + leaf + `,"message":"invalid"}`)}
			} else {
				items--
			}
			if !yield(record) {
				return
			}
		}
	}
}

// TestSingleRecordUsesTheSameReaderPath checks that the record count changes
// nothing but how the records are framed. The method: the same generator answer
// is written for 0, 1, 2 and 1000 rows, and one assertion that takes the counts
// as arguments reads all four; a payload that is no generator at all is read by
// that same assertion as a one-record answer.
//
// VALIDATES: AC-2 -- a one-record answer is the shared shape carrying one
// record, and no reader branches on how many records arrive.
// PREVENTS: a short-answer special case, which is the branch that later
// disagrees with the general path about what an answer means.
func TestSingleRecordUsesTheSameReaderPath(t *testing.T) {
	for _, rowCount := range []int{0, 1, 2, 1000} {
		t.Run(fmt.Sprintf("%d rows", rowCount), func(t *testing.T) {
			var answer bytes.Buffer
			resp := NewResponse(StatusDone, Records{Key: "peers", Rows: peerRows(rowCount)})
			if err := WriteAnswer(&answer, 3, resp); err != nil {
				t.Fatalf("WriteAnswer: %v", err)
			}

			// A walk within the threshold is one document, whatever the number
			// of rows in it; a walk past the threshold is one line for each.
			wantLines := 3
			if rowCount > answerBufferThreshold {
				wantLines = rowCount + 2
			}
			lines := readAnswer(t, answer.Bytes())
			if len(lines) != wantLines {
				t.Fatalf("answer has %d lines, want %d for %d rows", len(lines), wantLines, rowCount)
			}
			assertAnswerShape(t, lines, 3, uint64(rowCount), 0)
		})
	}

	t.Run("a payload that is no generator", func(t *testing.T) {
		var answer bytes.Buffer
		if err := WriteAnswer(&answer, 3, NewResponse(StatusDone, Map{"version": "1.0"})); err != nil {
			t.Fatalf("WriteAnswer: %v", err)
		}
		assertAnswerShape(t, readAnswer(t, answer.Bytes()), 3, 1, 0)
	})
}

// TestFaultDoesNotEndTheWalk checks that a rejected row is recorded and the
// walk goes on. The method: a five-row generator rejects rows 2 and 4, and the
// test counts what the generator produced as well as what reached the wire.
//
// VALIDATES: AC-8 -- item and fault lines interleave, the terminator carries
// both counts, and the verdict derives to partial.
// PREVENTS: one rejected leaf ending a commit of a hundred, which collapses
// 97-of-100 into the 0-of-100 this protocol exists to tell apart.
func TestFaultDoesNotEndTheWalk(t *testing.T) {
	produced := 0
	rows := func(yield func(rpc.Record) bool) {
		for i := range 5 {
			leaf := strconv.Itoa(i)
			record := rpc.Record{Item: json.RawMessage(`{"leaf":` + leaf + `}`)}
			if i%2 == 1 {
				record = rpc.Record{Fault: json.RawMessage(`{"leaf":` + leaf + `,"message":"invalid"}`)}
			}
			produced++
			if !yield(record) {
				return
			}
		}
	}

	var answer bytes.Buffer
	resp := NewResponse(StatusDone, Records{Key: "leaves", Rows: rows})
	if err := WriteAnswer(&answer, 9, resp); err != nil {
		t.Fatalf("WriteAnswer: %v", err)
	}

	if produced != 5 {
		t.Errorf("the walk produced %d of 5 rows, so a rejected row ended it", produced)
	}
	lines := readAnswer(t, answer.Bytes())
	assertAnswerShape(t, lines, 9, 3, 2)

	terminator := lines[len(lines)-1].tail
	if got := rpc.Verdict(&terminator); got != rpc.VerdictPartial {
		t.Errorf("the terminator derives verdict %q, want %q", got, rpc.VerdictPartial)
	}
}

// errTransportGone is the transport failure failingWriter reports: the
// connection the answer was being written to has died.
var errTransportGone = errors.New("connection reset by peer")

// failingWriter accepts writes until failAt, then reports a dead transport. It
// keeps what it accepted, so a test can read how far the answer got.
type failingWriter struct {
	failAt   int
	writes   int
	accepted bytes.Buffer
}

func (w *failingWriter) Write(p []byte) (int, error) {
	w.writes++
	if w.writes == w.failAt {
		return 0, errTransportGone
	}
	return w.accepted.Write(p)
}

// TestTransportErrorEndsTheWalk checks the one thing that DOES end a walk: the
// transport failing. The method: a writer that dies on its third write takes a
// hundred-row answer, and the test counts what the generator produced.
//
// The Go error slot carries transport failure alone. A rejected ROW is a
// fault= line and the walk continues (TestFaultDoesNotEndTheWalk); a dead
// connection has nobody left to produce rows for, so the walk stops and no
// terminator is claimed.
//
// VALIDATES: the error return of WriteAnswer is transport-only, and it stops
// the generator.
// PREVENTS: a daemon walking a million-row table into a socket that closed.
func TestTransportErrorEndsTheWalk(t *testing.T) {
	available := answerBufferThreshold + 100
	produced := 0
	rows := func(yield func(rpc.Record) bool) {
		for i := range available {
			produced++
			if !yield(rpc.Record{Item: json.RawMessage(`{"row":` + strconv.Itoa(i) + `}`)}) {
				return
			}
		}
	}

	// The walk holds answerBufferThreshold records and passes the threshold on
	// the next one, which is where the first line is written. Write 1 is the
	// head and write 2 is the first held record, so the transport dies while
	// the second one is being written.
	transport := &failingWriter{failAt: 3}
	err := WriteAnswer(transport, 4, NewResponse(StatusDone, Records{Key: "rows", Rows: rows}))
	if !errors.Is(err, errTransportGone) {
		t.Fatalf("WriteAnswer returned %v, want the transport error", err)
	}
	if produced != answerBufferThreshold+1 {
		t.Errorf("the generator produced %d of %d rows, want %d: a dead transport ends the walk",
			produced, available, answerBufferThreshold+1)
	}
	written := readAnswer(t, transport.accepted.Bytes())
	for i := range written {
		if written[i].tail.IsTerminator() {
			t.Errorf("line %d is a terminator, so a truncated answer claims to be complete", i+1)
		}
	}
}

// reassembleAnswer rebuilds the JSON a consumer of the record path holds once
// the terminator arrives. It reads the head's type= and takes the one route
// that type names: a document arrives whole in a single item, self-describing
// objects go under the head's envelope, and positional rows are zipped with the
// head's fields on the way. It derives all of that from the WIRE rather than
// calling the producer, so an agreement between the two paths is evidence and
// not a tautology.
func reassembleAnswer(t *testing.T, wire []byte) string {
	t.Helper()
	lines := readAnswer(t, wire)
	if len(lines) < 2 {
		t.Fatalf("answer has %d lines, want a head and a terminator at least", len(lines))
	}
	head := lines[0].tail
	records := lines[1 : len(lines)-1]

	if head.Type == rpc.AnswerTypeJSON {
		switch len(records) {
		case 0:
			return ""
		case 1:
			return string(records[0].tail.Item)
		default:
			t.Fatalf("a type=%s answer carries %d item lines, want one document", rpc.AnswerTypeJSON, len(records))
		}
	}

	items := []json.RawMessage{}
	var faults []json.RawMessage
	for i := range records {
		if fault := records[i].tail.Fault; len(fault) > 0 {
			faults = append(faults, fault)
			continue
		}
		items = append(items, zipStreamedRow(t, head, records[i].tail.Item))
	}

	var (
		reassembled []byte
		err         error
	)
	key := head.Key
	switch {
	case faults == nil && key == "":
		reassembled, err = json.Marshal(items)
	case faults == nil:
		reassembled, err = json.Marshal(map[string][]json.RawMessage{key: items})
	default:
		if key == "" {
			key = answerDefaultKey
		}
		reassembled, err = json.Marshal(map[string][]json.RawMessage{key: items, answerErrorsKey: faults})
	}
	if err != nil {
		t.Fatalf("marshal the reassembled answer: %v", err)
	}
	return string(reassembled)
}

// zipStreamedRow renders one row the way a consumer of a type=stream answer
// must: the head's field names over the row's values, in the head's column
// order. A type=ndjson row describes itself and passes through.
//
// It is written here rather than called from the encoder, because a consumer of
// the wire has only the wire: an agreement reached by calling the producer's
// own zip would prove nothing about what a consumer can rebuild.
func zipStreamedRow(t *testing.T, head rpc.AnswerTail, item json.RawMessage) json.RawMessage {
	t.Helper()
	if head.Type != rpc.AnswerTypeStream {
		return item
	}

	var values []json.RawMessage
	if err := json.Unmarshal(item, &values); err != nil {
		t.Fatalf("a type=%s row is not a positional array: %v", rpc.AnswerTypeStream, err)
	}
	if len(values) != len(head.Fields) {
		t.Fatalf("the row carries %d values and the head declares %d fields", len(values), len(head.Fields))
	}

	var object bytes.Buffer
	object.WriteByte('{')
	for i, field := range head.Fields {
		if i > 0 {
			object.WriteByte(',')
		}
		name, err := json.Marshal(field)
		if err != nil {
			t.Fatalf("marshal field %q: %v", field, err)
		}
		object.Write(name)
		object.WriteByte(':')
		object.Write(values[i])
	}
	object.WriteByte('}')
	return object.Bytes()
}

// TestStreamedAndBufferedAnswersAreIdentical is the control of the whole
// protocol. The method: one generator payload is taken through WriteAnswer and
// reassembled from its lines, and through ResponseJSON, and the two renderings
// are compared byte for byte.
//
// VALIDATES: AC-10 / R-7 -- the two paths cannot drift, because a consumer
// reads the same JSON whichever one it took.
// PREVENTS: two ledgers of one answer, where what an operator sees depends on
// which surface asked.
func TestStreamedAndBufferedAnswersAreIdentical(t *testing.T) {
	tests := []struct {
		name   string
		key    string
		fields []string
		rows   func() iter.Seq[rpc.Record]
	}{
		{name: "an envelope over many rows", key: "peers", rows: func() iter.Seq[rpc.Record] { return peerRows(3) }},
		{name: "an envelope over one row", key: "peers", rows: func() iter.Seq[rpc.Record] { return peerRows(1) }},
		{name: "an envelope over no rows", key: "peers", rows: func() iter.Seq[rpc.Record] { return peerRows(0) }},
		{name: "no envelope", key: "", rows: func() iter.Seq[rpc.Record] { return peerRows(3) }},
		{name: "an envelope over items and faults", key: "leaves", rows: func() iter.Seq[rpc.Record] { return mixedRows(3, 2) }},
		{name: "no envelope over items and faults", key: "", rows: func() iter.Seq[rpc.Record] { return mixedRows(3, 2) }},
		{name: "an envelope over faults alone", key: "leaves", rows: func() iter.Seq[rpc.Record] { return mixedRows(0, 2) }},

		// The threshold decides how the SAME answer is framed, so each shape is
		// taken once on each side of it. A pair that agrees only below the
		// threshold would leave every long answer unproven.
		{name: "an envelope over a walk within the threshold", key: "peers", rows: func() iter.Seq[rpc.Record] { return peerRows(answerBufferThreshold) }},
		{name: "an envelope over a walk past the threshold", key: "peers", rows: func() iter.Seq[rpc.Record] { return peerRows(answerBufferThreshold + 1) }},
		{name: "no envelope past the threshold", key: "", rows: func() iter.Seq[rpc.Record] { return peerRows(answerBufferThreshold + 1) }},
		{name: "items and faults past the threshold", key: "leaves", rows: func() iter.Seq[rpc.Record] { return mixedRows(answerBufferThreshold, 3) }},

		// A declared column schema changes the wire and not the answer: the
		// rows travel as positional arrays, and both paths render the objects.
		{name: "columns within the threshold", key: "peers", fields: peerColumns, rows: func() iter.Seq[rpc.Record] { return columnRows(3) }},
		{name: "columns past the threshold", key: "peers", fields: peerColumns, rows: func() iter.Seq[rpc.Record] { return columnRows(answerBufferThreshold + 1) }},
		{name: "columns with no envelope", key: "", fields: peerColumns, rows: func() iter.Seq[rpc.Record] { return columnRows(answerBufferThreshold + 1) }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// One Records value walks once, so each path gets its own over the
			// same rows.
			var answer bytes.Buffer
			streamedResp := NewResponse(StatusDone, Records{Key: tc.key, Fields: tc.fields, Rows: tc.rows()})
			if err := WriteAnswer(&answer, 11, streamedResp); err != nil {
				t.Fatalf("WriteAnswer: %v", err)
			}
			streamed := reassembleAnswer(t, answer.Bytes())

			bufferedResp := NewResponse(StatusDone, Records{Key: tc.key, Fields: tc.fields, Rows: tc.rows()})
			buffered, err := ResponseJSON(bufferedResp, nil)
			if err != nil {
				t.Fatalf("ResponseJSON: %v", err)
			}

			if streamed != buffered {
				t.Errorf("the two paths disagree:\n  record path:   %s\n  buffered path: %s", streamed, buffered)
			}
		})
	}
}

// TestBufferedAnswerCarriesRejectedRowsBesideTheRows pins the buffered
// collapse. The method: three answers are rendered through ResponseJSON, and
// each rendering is compared with the exact JSON a buffered consumer receives.
//
// The rejected rows go under a sibling key, never into the item array: the
// terminator counts the two separately, so mixing them would make `| count`
// answer one number on the record path and another on this one. The sibling is
// written only when a row was rejected, so an ordinary answer keeps the shape
// every buffered surface already reads.
//
// VALIDATES: a commit that applied 97 leaves and rejected 3 renders both on a
// buffered surface, rather than the 97 being lost with the error.
// PREVENTS: web, MCP, REST, gRPC and the looking glass rendering nothing for an
// answer that partly succeeded.
func TestBufferedAnswerCarriesRejectedRowsBesideTheRows(t *testing.T) {
	tests := []struct {
		name string
		key  string
		rows iter.Seq[rpc.Record]
		want string
	}{
		{
			name: "a rejected row rides beside the rows it did not join",
			key:  "leaves",
			rows: mixedRows(1, 1),
			want: `{"errors":[{"leaf":1,"message":"invalid"}],"leaves":[{"leaf":0}]}`,
		},
		{
			name: "with no envelope the rows move under the default key",
			key:  "",
			rows: mixedRows(1, 1),
			want: `{"data":[{"leaf":0}],"errors":[{"leaf":1,"message":"invalid"}]}`,
		},
		{
			name: "an answer that rejected nothing gains no key",
			key:  "leaves",
			rows: mixedRows(1, 0),
			want: `{"leaves":[{"leaf":0}]}`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ResponseJSON(NewResponse(StatusDone, Records{Key: tc.key, Rows: tc.rows}), nil)
			if err != nil {
				t.Fatalf("ResponseJSON: %v", err)
			}
			if got != tc.want {
				t.Errorf("buffered answer is %s, want %s", got, tc.want)
			}
		})
	}
}

// TestReservedEnvelopeKeyIsRefusedOnBothPaths checks that a handler cannot name
// its envelope after the sibling the rejected rows use. The method: the same
// Records is taken through both paths.
//
// Both refuse, and they refuse whether or not a row is rejected, so a handler
// learns on its first answer rather than on the first answer that happens to
// reject a row.
//
// VALIDATES: the two collections of a buffered answer cannot land under one
// key.
// PREVENTS: an answer's rows silently replacing its rejected rows, or the
// reverse, in a map with one key.
func TestReservedEnvelopeKeyIsRefusedOnBothPaths(t *testing.T) {
	if err := WriteAnswer(&bytes.Buffer{}, 8, NewResponse(StatusDone, Records{Key: answerErrorsKey, Rows: peerRows(2)})); !errors.Is(err, errReservedEnvelopeKey) {
		t.Errorf("WriteAnswer returned %v, want %v", err, errReservedEnvelopeKey)
	}
	if _, err := ResponseJSON(NewResponse(StatusDone, Records{Key: answerErrorsKey, Rows: peerRows(2)}), nil); !errors.Is(err, errReservedEnvelopeKey) {
		t.Errorf("ResponseJSON returned %v, want %v", err, errReservedEnvelopeKey)
	}
}

// TestEmptyRecordIsRefusedOnBothPaths checks that a row carrying neither an
// item nor a fault is named rather than written. The method: the same generator
// is taken through both paths.
//
// VALIDATES: rpc.Record's contract that exactly one of Item and Fault is set.
// PREVENTS: an `item=` line with no value, which no consumer can decode, and a
// null in the buffered array, which reads like a row the command produced.
func TestEmptyRecordIsRefusedOnBothPaths(t *testing.T) {
	empty := func(yield func(rpc.Record) bool) { yield(rpc.Record{}) }

	if err := WriteAnswer(&bytes.Buffer{}, 5, NewResponse(StatusDone, Records{Rows: empty})); !errors.Is(err, errEmptyAnswerRecord) {
		t.Errorf("WriteAnswer returned %v, want %v", err, errEmptyAnswerRecord)
	}
	if _, err := ResponseJSON(NewResponse(StatusDone, Records{Rows: empty}), nil); !errors.Is(err, errEmptyAnswerRecord) {
		t.Errorf("ResponseJSON returned %v, want %v", err, errEmptyAnswerRecord)
	}
}

// TestRecordsWithNoGeneratorIsAnEmptyAnswer checks that a Records naming an
// envelope and carrying no generator is an empty collection on both paths.
//
// VALIDATES: a command that produced nothing answers completely.
// PREVENTS: a panic on ranging a nil iter.Seq, which would take the daemon down
// on a handler's omission rather than answering `count=0`.
func TestRecordsWithNoGeneratorIsAnEmptyAnswer(t *testing.T) {
	var answer bytes.Buffer
	if err := WriteAnswer(&answer, 6, NewResponse(StatusDone, Records{Key: "peers"})); err != nil {
		t.Fatalf("WriteAnswer: %v", err)
	}
	assertAnswerShape(t, readAnswer(t, answer.Bytes()), 6, 0, 0)

	buffered, err := ResponseJSON(NewResponse(StatusDone, Records{Key: "peers"}), nil)
	if err != nil {
		t.Fatalf("ResponseJSON: %v", err)
	}
	if buffered != `{"peers":[]}` {
		t.Errorf("buffered answer is %s, want {\"peers\":[]}", buffered)
	}
}

// TestTheThresholdChoosesTheAnswerType is the decision the encoder exists to
// make. The method: the same generator is written one row short of the
// buffering threshold and one row past it, and each answer is read back for the
// type its head states, the number of lines it took, and the ORDER of the
// records the encoder was holding when it decided.
//
// The order is the half most easily lost: the records held while the encoder
// waits are written after the head and before the row that ended the wait, so a
// flush that dropped them, reversed them, or wrote them after the rest would
// leave an answer that is complete on the terminator and wrong in the middle.
//
// VALIDATES: a walk within the threshold is one document, and a walk past it
// streams every record it held, in walk order, ahead of the rest.
// PREVENTS: an answer whose first records are lost to the decision that framed
// them, which no count and no terminator can report.
func TestTheThresholdChoosesTheAnswerType(t *testing.T) {
	t.Run("a walk within the threshold is one document", func(t *testing.T) {
		rowCount := answerBufferThreshold - 1

		var answer bytes.Buffer
		if err := WriteAnswer(&answer, 2, NewResponse(StatusDone, Records{Key: "peers", Rows: peerRows(rowCount)})); err != nil {
			t.Fatalf("WriteAnswer: %v", err)
		}

		lines := readAnswer(t, answer.Bytes())
		if len(lines) != 3 {
			t.Fatalf("answer has %d lines, want 3: a head, one document and a terminator", len(lines))
		}
		head := lines[0].tail
		if head.Type != rpc.AnswerTypeJSON {
			t.Errorf("the head states type=%q, want %q", head.Type, rpc.AnswerTypeJSON)
		}
		if head.Key != "" {
			t.Errorf("the head names envelope %q, which the document already carries", head.Key)
		}

		var document map[string][]json.RawMessage
		if err := json.Unmarshal(lines[1].tail.Item, &document); err != nil {
			t.Fatalf("the item is not the answer document: %v", err)
		}
		if len(document["peers"]) != rowCount {
			t.Errorf("the document carries %d rows, want %d", len(document["peers"]), rowCount)
		}
		assertAnswerShape(t, lines, 2, uint64(rowCount), 0)
	})

	t.Run("a walk past the threshold streams every record in order", func(t *testing.T) {
		rowCount := answerBufferThreshold + 1

		var answer bytes.Buffer
		if err := WriteAnswer(&answer, 2, NewResponse(StatusDone, Records{Key: "peers", Rows: peerRows(rowCount)})); err != nil {
			t.Fatalf("WriteAnswer: %v", err)
		}

		lines := readAnswer(t, answer.Bytes())
		if len(lines) != rowCount+2 {
			t.Fatalf("answer has %d lines, want %d: a head, %d records and a terminator", len(lines), rowCount+2, rowCount)
		}
		head := lines[0].tail
		if head.Type != rpc.AnswerTypeNDJSON {
			t.Errorf("the head states type=%q, want %q", head.Type, rpc.AnswerTypeNDJSON)
		}
		if head.Key != "peers" {
			t.Errorf("the head names envelope %q, want peers: a streamed record carries none", head.Key)
		}

		records := lines[1 : len(lines)-1]
		for i := range records {
			want := `{"peer":"10.0.0.` + strconv.Itoa(i) + `"}`
			if got := string(records[i].tail.Item); got != want {
				t.Fatalf("record %d is %s, want %s: the held records did not reach the wire in walk order", i+1, got, want)
			}
		}
		assertAnswerShape(t, lines, 2, uint64(rowCount), 0)
	})

	t.Run("a declared schema streams as positional rows", func(t *testing.T) {
		rowCount := answerBufferThreshold + 1

		var answer bytes.Buffer
		records := Records{Key: "peers", Fields: peerColumns, Rows: columnRows(rowCount)}
		if err := WriteAnswer(&answer, 2, NewResponse(StatusDone, records)); err != nil {
			t.Fatalf("WriteAnswer: %v", err)
		}

		lines := readAnswer(t, answer.Bytes())
		head := lines[0].tail
		if head.Type != rpc.AnswerTypeStream {
			t.Errorf("the head states type=%q, want %q", head.Type, rpc.AnswerTypeStream)
		}
		if !slices.Equal(head.Fields, peerColumns) {
			t.Errorf("the head declares fields %q, want %q", head.Fields, peerColumns)
		}
		if got := string(lines[1].tail.Item); got != `["10.0.0.0",65000,"established"]` {
			t.Errorf("the first record is %s, want the positional row", got)
		}
	})

	t.Run("a declared schema within the threshold is still one document", func(t *testing.T) {
		var answer bytes.Buffer
		records := Records{Key: "peers", Fields: peerColumns, Rows: columnRows(2)}
		if err := WriteAnswer(&answer, 2, NewResponse(StatusDone, records)); err != nil {
			t.Fatalf("WriteAnswer: %v", err)
		}

		lines := readAnswer(t, answer.Bytes())
		if len(lines) != 3 {
			t.Fatalf("answer has %d lines, want 3: a head, one document and a terminator", len(lines))
		}
		if head := lines[0].tail; head.Type != rpc.AnswerTypeJSON || len(head.Fields) != 0 {
			t.Errorf("the head states type=%q with %d fields, want %q and none", head.Type, len(head.Fields), rpc.AnswerTypeJSON)
		}
		want := `{"peers":[{"peer":"10.0.0.0","as":65000,"state":"established"},{"peer":"10.0.0.1","as":65001,"state":"established"}]}`
		if got := string(lines[1].tail.Item); got != want {
			t.Errorf("the document is %s, want %s", got, want)
		}
	})
}

// TestABoundedStageStopsTheWalkDuringBuffering checks that the buffering the
// encoder does to choose a type does not defeat cancellation. The method: a
// stage that stops after ten rows is put between a thousand-row generator and
// the encoder, and the test counts what the generator produced.
//
// Ten is under the threshold, so the whole exchange happens while the encoder
// is still holding records: the walk ends before the decision point is reached,
// and the answer is the document those ten rows collapse to. This is the
// encoder's half of `| first 10`, whose pipe half is TestFirstNStopsTheGenerator
// (internal/component/command/pipe_test.go).
//
// VALIDATES: AC-14 -- a consumer that stops reading stops the walk.
// PREVENTS: the encoder pulling rows a stage above it has already refused,
// which would put the daemon back to walking a table nobody reads.
func TestABoundedStageStopsTheWalkDuringBuffering(t *testing.T) {
	produced := 0
	source := func(yield func(rpc.Record) bool) {
		for i := range 1000 {
			produced++
			if !yield(rpc.Record{Item: json.RawMessage(`{"peer":"10.0.0.` + strconv.Itoa(i) + `"}`)}) {
				return
			}
		}
	}
	firstTen := func(yield func(rpc.Record) bool) {
		kept := 0
		for record := range source {
			if !yield(record) {
				return
			}
			kept++
			if kept == 10 {
				return
			}
		}
	}

	var answer bytes.Buffer
	if err := WriteAnswer(&answer, 12, NewResponse(StatusDone, Records{Key: "peers", Rows: firstTen})); err != nil {
		t.Fatalf("WriteAnswer: %v", err)
	}

	if produced != 10 {
		t.Errorf("the generator produced %d rows, want 10: the encoder pulled past the stage that stopped", produced)
	}
	lines := readAnswer(t, answer.Bytes())
	if len(lines) != 3 {
		t.Fatalf("answer has %d lines, want 3: ten rows are one document", len(lines))
	}
	if head := lines[0].tail; head.Type != rpc.AnswerTypeJSON {
		t.Errorf("the head states type=%q, want %q", head.Type, rpc.AnswerTypeJSON)
	}
	assertAnswerShape(t, lines, 12, 10, 0)
}

// TestRowArityIsRefusedOnBothPaths checks that a row and the schema it is read
// against must agree. The method: a short row and a long one are taken through
// both paths, once inside the buffering threshold and once past it, and a row
// that is no array at all follows them.
//
// Neither path repairs the row. A short row padded with a null would invent a
// value the command never produced, and a long row truncated would drop one it
// did, and a consumer reading by position cannot tell either from real data.
//
// VALIDATES: a row whose length disagrees with the head's fields is refused.
// PREVENTS: a column schema and its rows drifting apart silently, which turns
// every later column of that answer into the wrong value.
func TestRowArityIsRefusedOnBothPaths(t *testing.T) {
	oneRow := func(item string) iter.Seq[rpc.Record] {
		return func(yield func(rpc.Record) bool) {
			yield(rpc.Record{Item: json.RawMessage(item)})
		}
	}
	pastThreshold := func(item string) iter.Seq[rpc.Record] {
		return func(yield func(rpc.Record) bool) {
			for record := range columnRows(answerBufferThreshold + 1) {
				if !yield(record) {
					return
				}
			}
			yield(rpc.Record{Item: json.RawMessage(item)})
		}
	}

	tests := []struct {
		name string
		rows func(string) iter.Seq[rpc.Record]
		item string
		want error
	}{
		{name: "a short row in the document", rows: oneRow, item: `["10.0.0.1",65001]`, want: errRowArity},
		{name: "a long row in the document", rows: oneRow, item: `["10.0.0.1",65001,"established","extra"]`, want: errRowArity},
		{name: "a row that is no array", rows: oneRow, item: `{"peer":"10.0.0.1"}`, want: errRowNotPositional},
		{name: "a short row in the stream", rows: pastThreshold, item: `["10.0.0.1",65001]`, want: errRowArity},
		{name: "a long row in the stream", rows: pastThreshold, item: `["10.0.0.1",65001,"established","extra"]`, want: errRowArity},
		{name: "a row that is no array in the stream", rows: pastThreshold, item: `{"peer":"10.0.0.1"}`, want: errRowNotPositional},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			records := Records{Key: "peers", Fields: peerColumns, Rows: tc.rows(tc.item)}
			if err := WriteAnswer(&bytes.Buffer{}, 1, NewResponse(StatusDone, records)); !errors.Is(err, tc.want) {
				t.Errorf("WriteAnswer returned %v, want %v", err, tc.want)
			}

			records.Rows = tc.rows(tc.item)
			if _, err := ResponseJSON(NewResponse(StatusDone, records), nil); !errors.Is(err, tc.want) {
				t.Errorf("ResponseJSON returned %v, want %v", err, tc.want)
			}
		})
	}
}
