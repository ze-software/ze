package command

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"iter"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// commandRecords answers a generator of count rows shaped like a command list,
// which is the answer this path exists for. It records how many rows the walk
// has produced, so a test can read whether a consumer answered in lockstep or
// after the walk finished.
func commandRecords(count int, produced *int) iter.Seq[rpc.Record] {
	return func(yield func(rpc.Record) bool) {
		for i := range count {
			var b textbuf.Buffer
			b.Str(`{"value":"show cmd-`).Int(int64(i)).Str(`","help":"row `).Int(int64(i)).Str(`"}`)
			if produced != nil {
				*produced++
			}
			if !yield(rpc.Record{Item: json.RawMessage(b.String())}) {
				return
			}
		}
	}
}

// commandColumns is the schema commandColumnRecords produces its rows against.
// The names are in column order rather than in alphabetical order, so a
// rendering that sorted them would answer a different table.
var commandColumns = []string{"value", "help"}

// commandColumnRecords answers the rows commandRecords answers, as the
// positional arrays a handler that declares commandColumns yields: the names
// live on the head and each row carries values alone.
func commandColumnRecords(count int, produced *int) iter.Seq[rpc.Record] {
	return func(yield func(rpc.Record) bool) {
		for i := range count {
			var b textbuf.Buffer
			b.Str(`["show cmd-`).Int(int64(i)).Str(`","row `).Int(int64(i)).Str(`"]`)
			if produced != nil {
				*produced++
			}
			if !yield(rpc.Record{Item: json.RawMessage(b.String())}) {
				return
			}
		}
	}
}

// rejectedRecord is the one rejected row a walk carries when a test needs an
// answer that reports what it refused beside what it produced. A plugin's
// forwarded answer produces one for a row that is not JSON (checkedRecord,
// internal/component/plugin/server/command.go), so it reaches this renderer.
var rejectedRecord = rpc.Record{Fault: json.RawMessage(`{"message":"row 3 is not JSON"}`)}

// withRejection yields records and one rejected row after the first of them, so
// the walk reports a refusal beside the rows it produced.
func withRejection(records iter.Seq[rpc.Record]) iter.Seq[rpc.Record] {
	return func(yield func(rpc.Record) bool) {
		yielded := false
		for record := range records {
			if !yield(record) {
				return
			}
			if yielded {
				continue
			}
			yielded = true
			if !yield(rejectedRecord) {
				return
			}
		}
	}
}

// renderedDocument is what the string path answers for the same rows: the
// collapse the buffered surfaces read, put through the same chain. It is the
// comparison every case below is judged against, built by a different route.
func renderedDocument(t *testing.T, chain, key string, count int) string {
	t.Helper()
	return strings.TrimRight(renderedDocumentBytes(t, chain, key, count), "\n")
}

func renderedDocumentBytes(t *testing.T, chain, key string, count int) string {
	t.Helper()
	document := collapseForTest(t, key, count)
	_, format, errMsg := ProcessPipesDefaultFormatChecked(chain, "")
	if errMsg != "" {
		t.Fatalf("ProcessPipesDefaultFormatChecked(%q): %s", chain, errMsg)
	}
	return format(document)
}

// collapseForTest builds the one document the rows collapse to, by hand rather
// than through the collapse under test, so the comparison is not a tautology.
func collapseForTest(t *testing.T, key string, count int) string {
	t.Helper()
	items := make([]json.RawMessage, 0, count)
	for record := range commandRecords(count, nil) {
		items = append(items, record.Item)
	}
	var document []byte
	var err error
	if key == "" {
		document, err = json.Marshal(items)
	} else {
		document, err = json.Marshal(map[string][]json.RawMessage{key: items})
	}
	if err != nil {
		t.Fatalf("build document: %v", err)
	}
	return string(document)
}

// recordEnvelope is the key `system command list` answers under. One name for
// every case here, because the envelope is not what any of them is about.
const recordEnvelope = "commands"

// renderRecordsForTest runs the renderer over count rows and returns what an
// operator would see and what the answer turned out to be.
func renderRecordsForTest(t *testing.T, chain string, count int) (string, RecordAnswer, int) {
	t.Helper()
	produced := 0
	var out bytes.Buffer
	answer, err := RenderRecords(&out, chain, "", recordEnvelope, nil, commandRecords(count, &produced))
	if err != nil {
		t.Fatalf("RenderRecords(%q, %d rows): %v", chain, count, err)
	}
	return strings.TrimRight(out.String(), "\n"), answer, produced
}

// TestTheRenderingIsTheSameOnBothSidesOfTheThreshold is the control on the
// run-time type decision: the answer is one document under the threshold and a
// stream over it, and an operator must not be able to tell which by reading the
// output.
//
// The comparison is against a document built row by row in this file, so the
// two sides are produced by different code and agreeing is evidence.
//
// VALIDATES: AC-1b, AC-1c of the streaming answer protocol -- a bounded answer
//
//	renders as the document a command has always answered with, and a
//	streamed one renders the same bytes.
//
// PREVENTS:  a command's output changing the day its registry passes 256 rows,
//
//	which is the failure that would make the threshold operator-visible.
func TestTheRenderingIsTheSameOnBothSidesOfTheThreshold(t *testing.T) {
	t.Setenv("ze.cli.format", "text")
	env.ResetCache()
	t.Cleanup(env.ResetCache)

	for _, chain := range []string{
		"system command list | json",
		"system command list | json compact",
		"system command list | ndjson",
		"system command list | yaml",
		"system command list | raw",
	} {
		for _, count := range []int{rpc.AnswerBufferThreshold - 1, rpc.AnswerBufferThreshold, rpc.AnswerBufferThreshold + 1} {
			t.Run(chain+", "+textbuf.StringInt(int64(count))+" rows", func(t *testing.T) {
				got, answer, _ := renderRecordsForTest(t, chain, count)
				if want := renderedDocument(t, chain, recordEnvelope, count); got != want {
					t.Errorf("the rendering changed with the row count:\n got %.200q\nwant %.200q", got, want)
				}
				if answer.Count != uint64(count) {
					t.Errorf("the answer counted %d records, want %d", answer.Count, count)
				}

				wantType := rpc.AnswerTypeDocument
				if count > rpc.AnswerBufferThreshold {
					wantType = rpc.AnswerTypeMap
				}
				if answer.Type != wantType {
					t.Errorf("%d rows answered type %q, want %q", count, answer.Type, wantType)
				}
			})
		}
	}
}

// TestNDJSONPastTheThresholdAnswersInLockstep is the streaming control: the
// renderer writes a record while the walk is still producing them.
//
// The method is to count the rows the generator has produced at the moment each
// line reaches the writer. A renderer that collects answers nothing until the
// walk has produced every row, so the first line it writes carries the whole
// count with it.
//
// VALIDATES: AC-1c -- an answer past the threshold streams, and the records
//
//	held while the type was being decided go out in walk order.
//
// PREVENTS:  the exec channel collecting a long answer and spending the memory
//
//	the record path exists to bound.
func TestNDJSONPastTheThresholdAnswersInLockstep(t *testing.T) {
	const rows = rpc.AnswerBufferThreshold + 200

	produced := 0
	writer := &witnessWriter{produced: &produced}
	answer, err := RenderRecords(writer, "system command list | ndjson", "", "commands", nil, commandRecords(rows, &produced))
	if err != nil {
		t.Fatalf("RenderRecords: %v", err)
	}
	if answer.Type != rpc.AnswerTypeMap {
		t.Fatalf("the answer states type %q, want %q", answer.Type, rpc.AnswerTypeMap)
	}

	// The first write carries the records held while the type was decided, so
	// the walk is one past the threshold and no further.
	if writer.firstAt != rpc.AnswerBufferThreshold+1 {
		t.Errorf("the first line was written after %d rows, want %d", writer.firstAt, rpc.AnswerBufferThreshold+1)
	}
	if writer.lastAt != rows {
		t.Errorf("the last line was written after %d rows, want %d", writer.lastAt, rows)
	}

	lines := strings.Split(strings.TrimRight(writer.body.String(), "\n"), "\n")
	if len(lines) != rows {
		t.Fatalf("the answer rendered %d lines, want %d", len(lines), rows)
	}
	for i, line := range lines {
		want := `{"help":"row ` + textbuf.StringInt(int64(i)) + `","value":"show cmd-` + textbuf.StringInt(int64(i)) + `"}`
		if line != want {
			t.Fatalf("line %d is %q, want %q", i, line, want)
		}
	}
}

// witnessWriter records how far the walk had got when the first and the last
// piece of the rendering was written.
type witnessWriter struct {
	body     bytes.Buffer
	produced *int
	firstAt  int
	lastAt   int
}

func (w *witnessWriter) Write(p []byte) (int, error) {
	if w.firstAt == 0 {
		w.firstAt = *w.produced
	}
	w.lastAt = *w.produced
	return w.body.Write(p)
}

// TestABoundedChainStopsTheWalkAndAnswersOneDocument covers the two operators
// that make a long answer bounded, and the promise the spec makes about them:
// `| first 10` cancels the walk, and `| count` reduces it to one number.
//
// Each case is judged against what the SAME chain answers over the whole
// payload, because a bounded chain that answered a different document would
// make a command's output depend on whether its handler produces rows or
// builds them. `| count` is the case that carries that: the string path
// replaces the payload with {"count":N} and the record path must not file that
// number under the envelope naming the rows it counted.
//
// VALIDATES: AC-14, and the spec's statement that `| first 10` never reaches
//
//	the decision point because ten is under the threshold.
//
// PREVENTS:  a bounded chain over a long answer streaming anyway, which would
//
//	make `show bgp rib | first 10` walk a whole RIB, and `| count`
//	answering one shape on the exec channel and another everywhere else.
func TestABoundedChainStopsTheWalkAndAnswersOneDocument(t *testing.T) {
	t.Setenv("ze.cli.format", "text")
	env.ResetCache()
	t.Cleanup(env.ResetCache)

	tests := []struct {
		name         string
		chain        string
		available    int
		wantProduced int
		wantCount    uint64
	}{
		{
			name:         "first 10 of a thousand",
			chain:        "system command list | first 10 | ndjson",
			available:    1000,
			wantProduced: 10,
			wantCount:    10,
		},
		{
			name:         "count of a thousand",
			chain:        "system command list | count | json compact",
			available:    1000,
			wantProduced: 1000,
			wantCount:    1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, answer, produced := renderRecordsForTest(t, tt.chain, tt.available)
			if produced != tt.wantProduced {
				t.Errorf("the generator produced %d rows, want %d", produced, tt.wantProduced)
			}
			if answer.Count != tt.wantCount {
				t.Errorf("the answer counted %d records, want %d", answer.Count, tt.wantCount)
			}
			if answer.Type != rpc.AnswerTypeDocument {
				t.Errorf("the answer states type %q, want %q", answer.Type, rpc.AnswerTypeDocument)
			}
			if want := renderedDocument(t, tt.chain, recordEnvelope, tt.available); body != want {
				t.Errorf("the operator saw %q, want the whole-payload chain's %q", body, want)
			}
		})
	}
}

// TestCountThenNDJSONMatchesTheDocumentRunner checks that count replaces either
// record representation with the same top-level document the whole-payload
// runner produces. The four sizes cross both sides of the framing threshold.
func TestCountThenNDJSONMatchesTheDocumentRunner(t *testing.T) {
	const chain = "system command list | count | ndjson"
	for _, count := range []int{0, 1, rpc.AnswerBufferThreshold, rpc.AnswerBufferThreshold + 1} {
		t.Run(textbuf.StringInt(int64(count))+" rows", func(t *testing.T) {
			want := renderedDocumentBytes(t, chain, recordEnvelope, count)
			var exact textbuf.Buffer
			exact.Str(`{"count":`).Int(int64(count)).Str(`,"pipe":[{"op":"count"}]}`).Byte('\n')
			// Read once: String empties the buffer, so a second read inside the
			// message would print nothing (internal/core/textbuf, String).
			expected := exact.String()
			if want != expected {
				t.Fatalf("document runner = %q, want exact top-level count and pipe metadata %q", want, expected)
			}

			forms := []struct {
				name    string
				fields  []string
				records iter.Seq[rpc.Record]
			}{
				{name: "self-describing", records: commandRecords(count, nil)},
				{name: "positional", fields: commandColumns, records: commandColumnRecords(count, nil)},
			}
			for _, form := range forms {
				t.Run(form.name, func(t *testing.T) {
					var out bytes.Buffer
					answer, err := RenderRecords(&out, chain, "", recordEnvelope, form.fields, form.records)
					if err != nil {
						t.Fatalf("RenderRecords: %v", err)
					}
					if out.String() != want {
						t.Errorf("record runner = %q, whole-document runner = %q", out.String(), want)
					}
					if answer.Type != rpc.AnswerTypeDocument || answer.Count != 1 || answer.Faults != 0 {
						t.Errorf("answer = %+v, want one count document", answer)
					}
					if answer.Fields != nil {
						t.Errorf("count retained source fields %v, want nil", answer.Fields)
					}
				})
			}
		})
	}
}

func TestCountDocumentFollowersRefuseBeforeRecords(t *testing.T) {
	for _, follower := range []string{
		"match show",
		"first 1",
		"last 1",
		"display value",
		"fill alpha",
		"resolve",
		"origin",
		"ndjson | resolve",
	} {
		t.Run(follower, func(t *testing.T) {
			chain := "system command list | count | " + follower
			_, _, documentMsg := ProcessPipesDefaultFormatChecked(chain, "")
			if documentMsg == "" {
				t.Fatal("whole-document entry accepted a transform after count")
			}
			operator := strings.Fields(follower)
			later := operator[0]
			if follower == "ndjson | resolve" {
				later = "resolve"
			}
			for _, part := range []string{later, "count", "document"} {
				if !strings.Contains(documentMsg, part) {
					t.Errorf("refusal %q does not name %q", documentMsg, part)
				}
			}

			produced := 0
			var out bytes.Buffer
			_, err := RenderRecords(
				&out,
				chain,
				"",
				recordEnvelope,
				nil,
				commandRecords(3, &produced),
			)
			if err == nil {
				t.Fatal("record entry accepted a transform after count")
			}
			if err.Error() != documentMsg {
				t.Errorf("record refusal = %q, document refusal = %q", err, documentMsg)
			}
			if produced != 0 {
				t.Errorf("refused chain pulled %d records, want none", produced)
			}
			if out.Len() != 0 {
				t.Errorf("refused chain wrote %q, want no partial answer", out.String())
			}
		})
	}
}

func TestFormattedCountDocumentEnablesLineFollowers(t *testing.T) {
	for _, follower := range []string{"match count", "first 1", "last 1"} {
		t.Run(follower, func(t *testing.T) {
			chain := "system command list | count | ndjson | " + follower
			want := renderedDocumentBytes(t, chain, recordEnvelope, 3)
			var out bytes.Buffer
			if _, err := RenderRecords(
				&out,
				chain,
				"",
				recordEnvelope,
				nil,
				commandRecords(3, nil),
			); err != nil {
				t.Fatalf("RenderRecords: %v", err)
			}
			if out.String() != want {
				t.Errorf("record runner = %q, whole-document runner = %q", out.String(), want)
			}
		})
	}
}

func TestNDJSONLineTransformsMatchTheDocumentRunnerAtEveryCardinality(t *testing.T) {
	chains := []string{
		"system command list | ndjson | match show",
		"system command list | ndjson | count",
		"system command list | ndjson | first 1",
		"system command list | ndjson | last 1",
	}
	for _, chain := range chains {
		t.Run(chain, func(t *testing.T) {
			for _, count := range []int{0, 1, rpc.AnswerBufferThreshold, rpc.AnswerBufferThreshold + 1} {
				t.Run(textbuf.StringInt(int64(count))+" rows", func(t *testing.T) {
					want := renderedDocumentBytes(t, chain, recordEnvelope, count)
					var out bytes.Buffer
					if _, err := RenderRecords(
						&out,
						chain,
						"",
						recordEnvelope,
						nil,
						commandRecords(count, nil),
					); err != nil {
						t.Fatalf("RenderRecords: %v", err)
					}
					if out.String() != want {
						t.Errorf("record runner = %.300q, whole-document runner = %.300q", out.String(), want)
					}
				})
			}
		})
	}
}

// TestAPositionalAnswerRendersLikeItsSelfDescribingTwin is the control on the
// column schema: an answer whose head declares its columns and whose rows carry
// values alone renders exactly what the same rows render when each one carries
// its own names.
//
// The method is to run one chain twice over the same data in its two wire
// forms, and to compare the two renderings byte for byte. The self-describing
// side is what an operator sees today, so it is the answer the positional side
// owes. Neither side is built from the other.
//
// Both sides of the threshold are driven. Under it the answer is one document
// and the zip happens in the collapse; over it the head declares `tab` and the
// records reach the renderer as arrays.
//
// `| count` is in the list because it is the one operator that replaces the
// command's rows with a document of its own. The column schema then describes
// nothing the answer still carries, and a collapse that still read the records
// against it would refuse `{"count":N}` as a row that is not positional.
//
// VALIDATES: AC-6 and AC-7 of spec-record-answers-3-zero-alloc -- a declared
// schema reaches the operator as the same payload a schema-less answer does.
// PREVENTS: `show ... | table` over a schema-declaring command answering a
// different table from the same command's schema-less form, and `| count` over
// one failing outright.
func TestAPositionalAnswerRendersLikeItsSelfDescribingTwin(t *testing.T) {
	t.Setenv("ze.cli.format", "text")
	env.ResetCache()
	t.Cleanup(env.ResetCache)

	for _, chain := range []string{
		"system command list | json",
		"system command list | table",
		"system command list | text",
		"system command list | yaml",
		"system command list | ndjson",
		"system command list | raw",
		"system command list | display value | json",
		"system command list | match cmd-1 | json",
		"system command list | first 10 | json",
		"system command list | count | json compact",
	} {
		for _, count := range []int{rpc.AnswerBufferThreshold - 1, rpc.AnswerBufferThreshold + 1} {
			for _, rejects := range []bool{false, true} {
				// `| ndjson` is the one rendering a schema-less answer writes
				// as the records arrive, and a stream has no document to group
				// the rejected rows under: it writes each one as its own line
				// in walk order, where a buffered rendering files them under
				// the errors envelope. That difference is between a STREAMED
				// and a BUFFERED rendering rather than between the two row
				// forms, and it is already there for an answer that declares
				// no schema (writeRecordJSON, render_records.go).
				if rejects && strings.Contains(chain, "ndjson") {
					continue
				}
				name := chain + ", " + textbuf.StringInt(int64(count)) + " rows"
				if rejects {
					name += ", one rejected"
				}
				t.Run(name, func(t *testing.T) {
					self := commandRecords(count, nil)
					columns := commandColumnRecords(count, nil)
					if rejects {
						self, columns = withRejection(self), withRejection(columns)
					}

					var selfRendered bytes.Buffer
					wantAnswer, err := RenderRecords(&selfRendered, chain, "", recordEnvelope, nil, self)
					if err != nil {
						t.Fatalf("RenderRecords over self-describing rows: %v", err)
					}

					var positional bytes.Buffer
					gotAnswer, err := RenderRecords(&positional, chain, "", recordEnvelope, commandColumns, columns)
					if err != nil {
						t.Fatalf("RenderRecords over positional rows: %v", err)
					}

					if got, want := positional.String(), selfRendered.String(); got != want {
						t.Errorf("the positional answer rendered\n%.400q\nwant the self-describing answer's\n%.400q", got, want)
					}
					if gotAnswer.Count != wantAnswer.Count || gotAnswer.Faults != wantAnswer.Faults {
						t.Errorf("the positional answer counted %d records and %d rejections, want %d and %d",
							gotAnswer.Count, gotAnswer.Faults, wantAnswer.Count, wantAnswer.Faults)
					}
				})
			}
		}
	}
}

// TestADeclaredSchemaNamesTheTableType checks the fact the exec channel's frame
// reads off the rendering: a walk that passed the threshold with a schema
// declared is a table answer, and the same walk with no schema is a map answer.
//
// VALIDATES: AC-6 and AC-7 -- the item type follows the schema the handler
// declared, and nothing else.
// PREVENTS: the frame naming `map` for an answer whose rows are positional, so
// a client reading it by position would read the values as an object.
func TestADeclaredSchemaNamesTheTableType(t *testing.T) {
	const rows = rpc.AnswerBufferThreshold + 1

	var out bytes.Buffer
	answer, err := RenderRecords(&out, "system command list | json", "", recordEnvelope, commandColumns, commandColumnRecords(rows, nil))
	if err != nil {
		t.Fatalf("RenderRecords: %v", err)
	}
	if answer.Type != rpc.AnswerTypeTable {
		t.Errorf("a declared schema answered type %q, want %q", answer.Type, rpc.AnswerTypeTable)
	}

	out.Reset()
	answer, err = RenderRecords(&out, "system command list | json", "", recordEnvelope, nil, commandRecords(rows, nil))
	if err != nil {
		t.Fatalf("RenderRecords: %v", err)
	}
	if answer.Type != rpc.AnswerTypeMap {
		t.Errorf("a schema-less walk answered type %q, want %q", answer.Type, rpc.AnswerTypeMap)
	}
}

// TestAStreamedRenderingRefusesToDropThePipeMetadata pins the one reason a
// long answer still renders as a document: `| first`, `| last`, `| match` and
// `| count` each fold a note into the envelope for the renderer, and a stream
// has no envelope to carry it.
//
// VALIDATES: the pipe metadata reaching the operator whatever the row count.
// PREVENTS:  a chain that folds metadata streaming its records and dropping the
//
//	note, so `| match x` over a long answer would say less than the same
//	chain over a short one.
func TestAStreamedRenderingRefusesToDropThePipeMetadata(t *testing.T) {
	const rows = rpc.AnswerBufferThreshold + 10

	body, answer, _ := renderRecordsForTest(t, "system command list | match show | ndjson", rows)
	if answer.Type != rpc.AnswerTypeMap {
		t.Fatalf("the answer states type %q, want %q: the walk passed the threshold", answer.Type, rpc.AnswerTypeMap)
	}
	// Parsed rather than string-matched: the format operator round-trips the
	// document through a generic map, so the step's own keys come out in
	// alphabetical order and a literal comparison would pin that accident.
	var envelope struct {
		Pipe []struct {
			Op  string `json:"op"`
			Arg string `json:"arg"`
		} `json:"pipe"`
	}
	if err := json.Unmarshal([]byte(body), &envelope); err != nil {
		t.Fatalf("the rendering is not one JSON document: %v", err)
	}
	if len(envelope.Pipe) != 1 {
		t.Fatalf("the rendering carries %d pipe steps, want 1: %.200q", len(envelope.Pipe), body)
	}
	if envelope.Pipe[0].Op != "match" || envelope.Pipe[0].Arg != "show" {
		t.Errorf("the pipe metadata records %+v, want match show", envelope.Pipe[0])
	}
	if lines := strings.Count(body, "\n"); lines != 0 {
		t.Errorf("the rendering is %d lines, want the one document the metadata rides in", lines+1)
	}
}

// TestARefusedChainNamesItself checks that an unreadable chain is reported
// rather than rendered as an answer of nothing.
//
// VALIDATES: ai/rules/evidence.md, fail closed -- a chain nobody agreed to
//
//	produces no records, and the caller is told which operator was
//	refused.
//
// PREVENTS:  `show x | first zero` looking like a command that answered
//
//	nothing.
func TestARefusedChainNamesItself(t *testing.T) {
	var out bytes.Buffer
	answer, err := RenderRecords(&out, "system command list | match", "", "commands", nil, commandRecords(5, nil))
	if err == nil {
		t.Fatalf("a chain with no pattern rendered %q with answer %+v, want a refusal", out.String(), answer)
	}
	if !strings.Contains(err.Error(), "match requires a pattern") {
		t.Errorf("the refusal is %q, want it to name the operator", err.Error())
	}
}

// TestAFailedWriteStopsTheWalk checks that a transport that has gone away ends
// the answer instead of producing every remaining row for nobody.
//
// VALIDATES: the record path stopping at the first write failure.
// PREVENTS:  a disconnected operator costing the daemon a full walk of a large
//
//	table.
func TestAFailedWriteStopsTheWalk(t *testing.T) {
	const rows = rpc.AnswerBufferThreshold + 500

	produced := 0
	_, err := RenderRecords(&brokenWriter{allowed: 1}, "system command list | ndjson", "", "commands", nil, commandRecords(rows, &produced))
	if err == nil {
		t.Fatal("a writer that fails must end the answer")
	}
	if produced >= rows {
		t.Errorf("the generator produced %d of %d rows after the transport died, want it to stop", produced, rows)
	}
}

// brokenWriter accepts allowed writes and then reports the transport is gone.
type brokenWriter struct {
	allowed int
}

func (b *brokenWriter) Write(p []byte) (int, error) {
	if b.allowed <= 0 {
		return 0, io.ErrClosedPipe
	}
	b.allowed--
	return len(p), nil
}

// TestTheConfiguredFormatRendersARecordAnswer checks that a chain naming no
// format takes the daemon's configured one, exactly as a whole payload does.
//
// VALIDATES: AC-1b -- the rendering of a record answer is decided the same way
//
//	as the rendering of a document.
//
// PREVENTS:  the record path answering JSON to an operator who committed
//
//	`environment cli format default table`.
func TestTheConfiguredFormatRendersARecordAnswer(t *testing.T) {
	t.Setenv("ze.cli.format", "table")
	env.ResetCache()
	t.Cleanup(env.ResetCache)

	body, answer, _ := renderRecordsForTest(t, "system command list", 3)
	if answer.Type != rpc.AnswerTypeDocument {
		t.Fatalf("the answer states type %q, want %q", answer.Type, rpc.AnswerTypeDocument)
	}
	if !strings.Contains(body, "┌") && !strings.Contains(body, "│") {
		t.Errorf("the configured default did not render a table:\n%s", body)
	}
	if strings.Contains(body, `"value"`) {
		t.Errorf("the raw JSON reached the operator:\n%s", body)
	}
}

// TestAnEmptyWalkAnswersTheEmptyCollection checks that a command whose walk
// produced nothing still answers the collection it has always answered.
//
// VALIDATES: AC-3 -- a command that returns no data answers a zero count and the
//
//	envelope it named.
//
// PREVENTS:  an empty generator rendering as null, which reads like a command
//
//	that failed rather than one that found nothing.
func TestAnEmptyWalkAnswersTheEmptyCollection(t *testing.T) {
	var out bytes.Buffer
	// `| raw` is the identity rendering, so what reaches the operator is the
	// collapse itself. `| json` would unwrap the single-key envelope and hide
	// the fact this test is about.
	answer, err := RenderRecords(&out, "system command list | raw", "", "commands", nil, slices.Values([]rpc.Record(nil)))
	if err != nil {
		t.Fatalf("RenderRecords: %v", err)
	}
	if answer.Count != 0 {
		t.Errorf("the answer counted %d records, want 0", answer.Count)
	}
	if got := strings.TrimSpace(out.String()); got != `{"commands":[]}` {
		t.Errorf("an empty walk rendered %q, want %q", got, `{"commands":[]}`)
	}
}

// TestPositionalDisplayTypoRefusesBeforeWalking drives a field miss against the
// schema that gives positional values their names.
//
// VALIDATES: IR-5 on positional record answers -- a display field absent from
// the head schema is refused before any row is pulled.
// PREVENTS: bypassing fail-closed selection for arrays, which would restore the
// complete-row leak on the positional path.
func TestPositionalDisplayTypoRefusesBeforeWalking(t *testing.T) {
	produced := 0
	var out bytes.Buffer
	_, err := RenderRecords(
		&out,
		"system command list | display typo | json",
		"",
		recordEnvelope,
		commandColumns,
		commandColumnRecords(3, &produced),
	)
	if err == nil {
		t.Fatal("positional display typo was accepted")
	}
	if !strings.Contains(err.Error(), "typo") {
		t.Errorf("refusal %q does not name the requested field", err)
	}
	if produced != 0 {
		t.Errorf("refused positional display pulled %d rows, want none", produced)
	}
	if out.Len() != 0 {
		t.Errorf("refused positional display wrote %q, want no partial answer", out.String())
	}
}

type positionalOriginResolver struct {
	results map[string]OriginResult
}

func (r positionalOriginResolver) LookupOrigin(_ context.Context, address string) (OriginResult, error) {
	return r.results[address], nil
}

// TestPositionalAddressOperatorsExtendSchemaAndKeepRowWidths drives a lookup
// hit and miss through the positional record path.
//
// VALIDATES: IR2-13 -- declared address columns gain deterministic resolve and
// origin columns, and every row has the extended schema's width.
// PREVENTS: map-key-only enrichment leaving arrays unchanged, or lookup misses
// producing short positional rows that shift every later column.
func TestPositionalAddressOperatorsExtendSchemaAndKeepRowWidths(t *testing.T) {
	ResetShapesForTest()
	ResetAddressFieldsForTest()
	t.Cleanup(ResetShapesForTest)
	t.Cleanup(ResetAddressFieldsForTest)
	RegisterShape([]string{"show test positional addresses"}, ShapeTab)
	RegisterAddressFields([]string{"show test positional addresses"}, "address")

	SetPTRResolver(&mockPTRResolver{results: map[string][]string{
		"192.0.2.1": {"peer.example."},
	}})
	SetOriginResolver(positionalOriginResolver{results: map[string]OriginResult{
		"192.0.2.1": {ASN: 64501, Name: "Example", Prefix: "192.0.2.0/24"},
	}})
	t.Cleanup(func() {
		SetPTRResolver(nil)
		SetOriginResolver(nil)
	})

	rows := slices.Values([]rpc.Record{
		{Item: json.RawMessage(`["192.0.2.1","192.0.2.254"]`)},
		{Item: json.RawMessage(`["198.51.100.2","198.51.100.254"]`)},
	})
	records, fields, msg := applyPipesRecords(
		"show test positional addresses | resolve | origin",
		[]string{"address", "router-id"},
		rows,
	)
	if msg != "" {
		t.Fatalf("positional address chain was refused: %s", msg)
	}
	wantFields := []string{
		"address", "router-id", "address-name",
		"address-asn", "address-as-name", "address-prefix",
	}
	if !slices.Equal(fields, wantFields) {
		t.Fatalf("derived fields = %v, want %v", fields, wantFields)
	}

	got := collectRecords(records)
	if len(got) != 2 {
		t.Fatalf("address chain answered %d rows, want 2", len(got))
	}
	values := make([][]any, len(got))
	for i, record := range got {
		if err := json.Unmarshal(record.Item, &values[i]); err != nil {
			t.Fatalf("row %d is not a positional array: %v", i, err)
		}
		if len(values[i]) != len(fields) {
			t.Fatalf("row %d has %d values, schema has %d", i, len(values[i]), len(fields))
		}
	}
	wantHit := []any{"peer.example", float64(64501), "Example", "192.0.2.0/24"}
	if !slices.Equal(values[0][2:], wantHit) {
		t.Errorf("lookup hit derived values = %v, want %v", values[0][2:], wantHit)
	}
	wantMiss := []any{"", float64(0), "", ""}
	if !slices.Equal(values[1][2:], wantMiss) {
		t.Errorf("lookup miss derived values = %v, want fixed-width %v", values[1][2:], wantMiss)
	}

	streamRows := func(yield func(rpc.Record) bool) {
		for range rpc.AnswerBufferThreshold + 1 {
			if !yield(rpc.Record{Item: json.RawMessage(`["192.0.2.1","192.0.2.254"]`)}) {
				return
			}
		}
	}
	var output bytes.Buffer
	answer, err := RenderRecords(
		&output,
		"show test positional addresses | resolve | origin | ndjson",
		"",
		"addresses",
		[]string{"address", "router-id"},
		streamRows,
	)
	if err != nil {
		t.Fatalf("render enriched positional stream: %v", err)
	}
	if answer.Type != rpc.AnswerTypeMap {
		t.Errorf("enriched NDJSON stream type = %q, want map", answer.Type)
	}
	if len(answer.Fields) != 0 {
		t.Errorf("self-describing NDJSON fields = %v, want no positional schema", answer.Fields)
	}
	if !strings.Contains(output.String(), `"address-name":"peer.example"`) {
		t.Errorf("enriched NDJSON did not name its derived columns: %.200q", output.String())
	}
}

// TestPositionalAddressOperatorsPreserveProducerDerivedColumns covers both
// enrichment operators when a positional producer already declares their
// suffix fields. Field identity is case-insensitive, and producer values win.
func TestPositionalAddressOperatorsPreserveProducerDerivedColumns(t *testing.T) {
	ResetShapesForTest()
	ResetAddressFieldsForTest()
	t.Cleanup(ResetShapesForTest)
	t.Cleanup(ResetAddressFieldsForTest)
	const path = "show test positional existing derivation"
	RegisterShape([]string{path}, ShapeTab)
	RegisterAddressFields([]string{path}, "address")

	SetPTRResolver(&mockPTRResolver{results: map[string][]string{
		"192.0.2.1": {"lookup.example."},
	}})
	SetOriginResolver(positionalOriginResolver{results: map[string]OriginResult{
		"192.0.2.1": {ASN: 64501, Name: "Lookup", Prefix: "192.0.2.0/24"},
	}})
	t.Cleanup(func() {
		SetPTRResolver(nil)
		SetOriginResolver(nil)
	})

	tests := []struct {
		name       string
		chain      string
		fields     []string
		item       string
		wantFields []string
		wantItem   string
	}{
		{
			name:       "resolve",
			chain:      path + " | resolve",
			fields:     []string{"address", "Address-Name"},
			item:       `["192.0.2.1","producer.example"]`,
			wantFields: []string{"address", "Address-Name"},
			wantItem:   `["192.0.2.1","producer.example"]`,
		},
		{
			name:       "origin",
			chain:      path + " | origin",
			fields:     []string{"address", "ADDRESS-ASN", "address-As-Name", "ADDRESS-prefix"},
			item:       `["192.0.2.1",64496,"Producer","198.51.100.0/24"]`,
			wantFields: []string{"address", "ADDRESS-ASN", "address-As-Name", "ADDRESS-prefix"},
			wantItem:   `["192.0.2.1",64496,"Producer","198.51.100.0/24"]`,
		},
		{
			name:       "origin appends only absent suffix values",
			chain:      path + " | origin",
			fields:     []string{"address", "Address-AS-Name"},
			item:       `["192.0.2.1","Producer"]`,
			wantFields: []string{"address", "Address-AS-Name", "address-asn", "address-prefix"},
			wantItem:   `["192.0.2.1","Producer",64501,"192.0.2.0/24"]`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			records, fields, msg := applyPipesRecords(
				tt.chain,
				tt.fields,
				slices.Values([]rpc.Record{{Item: json.RawMessage(tt.item)}}),
			)
			if msg != "" {
				t.Fatalf("address chain was refused: %s", msg)
			}
			if !slices.Equal(fields, tt.wantFields) {
				t.Fatalf("fields = %v, want %v", fields, tt.wantFields)
			}
			got := collectRecords(records)
			if len(got) != 1 {
				t.Fatalf("address chain answered %d rows, want one", len(got))
			}
			if string(got[0].Item) != tt.wantItem {
				t.Errorf("producer row = %s, want %s", got[0].Item, tt.wantItem)
			}
		})
	}
}

// TestFormatBeforeRowTransformAlwaysUsesTheDocumentPath fixes the exact
// streaming boundary where operator order used to change.
//
// VALIDATES: IR2-15 -- text renders before first at 256 and 257 source rows.
// PREVENTS: the record path applying first to JSON rows before text once the
// answer crosses the streaming threshold.
func TestFormatBeforeRowTransformAlwaysUsesTheDocumentPath(t *testing.T) {
	const chain = "system command list | text | first 1"
	var bodies []string
	for _, count := range []int{rpc.AnswerBufferThreshold, rpc.AnswerBufferThreshold + 1} {
		body, answer, _ := renderRecordsForTest(t, chain, count)
		if want := renderedDocument(t, chain, recordEnvelope, count); body != want {
			t.Errorf("%d rows rendered %q, want document path %q", count, body, want)
		}
		if answer.Type != rpc.AnswerTypeDocument {
			t.Errorf("%d rows answered type %q, want document", count, answer.Type)
		}
		if answer.Count != 1 {
			t.Errorf("%d rows answered count %d, want the one rendered line", count, answer.Count)
		}
		bodies = append(bodies, body)
	}
	if bodies[0] != bodies[1] {
		t.Errorf("text | first 1 changed at the threshold: below %q, above %q", bodies[0], bodies[1])
	}
}

// TestTransformBeforeFormatStillStreams guards the other side of IR2-15: only
// chains that need a rendered value early become documents.
//
// VALIDATES: resolve before ndjson keeps ordinary per-record streaming.
// PREVENTS: fixing format-first order by collecting every chain containing both
// a transform and a format.
func TestTransformBeforeFormatStillStreams(t *testing.T) {
	ResetShapesForTest()
	ResetAddressFieldsForTest()
	t.Cleanup(ResetShapesForTest)
	t.Cleanup(ResetAddressFieldsForTest)
	RegisterShape([]string{"show test stream addresses"}, ShapeMap)
	RegisterAddressFields([]string{"show test stream addresses"}, "address")

	const rows = rpc.AnswerBufferThreshold + 10
	produced := 0
	records := func(yield func(rpc.Record) bool) {
		for range rows {
			produced++
			if !yield(rpc.Record{Item: json.RawMessage(`{"address":"*"}`)}) {
				return
			}
		}
	}
	writer := &witnessWriter{produced: &produced}
	answer, err := RenderRecords(
		writer,
		"show test stream addresses | resolve | ndjson",
		"",
		"addresses",
		nil,
		records,
	)
	if err != nil {
		t.Fatalf("RenderRecords: %v", err)
	}
	if answer.Type != rpc.AnswerTypeMap {
		t.Errorf("transform-before-format type = %q, want map stream", answer.Type)
	}
	if writer.firstAt != rpc.AnswerBufferThreshold+1 {
		t.Errorf("first write followed %d produced rows, want %d",
			writer.firstAt, rpc.AnswerBufferThreshold+1)
	}
}

func TestFormatTransformCompatibilityMatrix(t *testing.T) {
	formats := []struct {
		kind pipeKind
		name string
		ok   bool
	}{
		{pipeJSON, "json", true},
		{pipeRaw, "raw", true},
		{pipeNDJSON, "ndjson", false},
		{pipeYAML, "yaml", false},
		{pipeTable, "table", false},
		{pipeText, "text", false},
	}
	transforms := []struct {
		kind pipeKind
		name string
		arg  string
	}{
		{pipeDisplay, "display", "address"},
		{pipeFill, "fill", "address"},
		{pipeResolve, "resolve", ""},
		{pipeOrigin, "origin", ""},
	}

	for _, format := range formats {
		for _, transform := range transforms {
			t.Run(format.name+"/"+transform.name, func(t *testing.T) {
				ops := []pipeOp{
					{kind: format.kind},
					{kind: transform.kind, arg: transform.arg, allAddressFields: true},
				}
				msg := validateFormatTransformCompatibility(ops)
				if format.ok {
					if msg != "" {
						t.Fatalf("compatible chain was refused: %s", msg)
					}
					return
				}
				if !strings.Contains(msg, format.name) || !strings.Contains(msg, transform.name) {
					t.Errorf("refusal = %q, want %s and %s named", msg, format.name, transform.name)
				}
			})
		}
	}
}

// TestIncompatiblePostFormatTransformRefusesBeforeWalking proves that the
// compatibility matrix is checked at the record boundary, not after a source
// has been collected or rendered.
func TestIncompatiblePostFormatTransformRefusesBeforeWalking(t *testing.T) {
	ResetShapesForTest()
	ResetAddressFieldsForTest()
	t.Cleanup(ResetShapesForTest)
	t.Cleanup(ResetAddressFieldsForTest)
	RegisterShape([]string{"show test addresses"}, ShapeTab)
	RegisterAddressFields([]string{"show test addresses"}, "address")

	formats := []string{"ndjson", "yaml", "table", "text"}
	transforms := []string{"display address", "fill alpha", "resolve", "origin"}
	for _, format := range formats {
		for _, transform := range transforms {
			t.Run(format+"/"+transform, func(t *testing.T) {
				produced := 0
				var out bytes.Buffer
				_, err := RenderRecords(
					&out,
					"show test addresses | "+format+" | "+transform,
					"",
					"addresses",
					nil,
					commandRecords(3, &produced),
				)
				if err == nil {
					t.Fatal("incompatible post-format transform was accepted")
				}
				operator := strings.Fields(transform)[0]
				if !strings.Contains(err.Error(), format) || !strings.Contains(err.Error(), operator) {
					t.Errorf("refusal = %q, want %s and %s named", err, format, operator)
				}
				if produced != 0 {
					t.Errorf("refused chain pulled %d records, want none", produced)
				}
				if out.Len() != 0 {
					t.Errorf("refused chain wrote %q, want no partial answer", out.String())
				}
			})
		}
	}
}

// TestNDJSONLineTransformsStayOnTheRecordPath exercises each transform on one
// NDJSON line. The source document stays on the record path.
func TestNDJSONLineTransformsStayOnTheRecordPath(t *testing.T) {
	const available = 1000
	tests := []struct {
		name          string
		chain         string
		wantProduced  int
		wantCount     uint64
		wantLines     int
		wantMetadata  bool
		wantStreaming bool
	}{
		{
			name:         "first stops the source",
			chain:        "system command list | ndjson | first 1",
			wantProduced: 1,
			wantCount:    1,
			wantLines:    1,
			wantMetadata: true,
		},
		{
			name:         "count retains only its counter",
			chain:        "system command list | ndjson | count",
			wantProduced: available,
			wantCount:    1,
			wantLines:    1,
			wantMetadata: true,
		},
		{
			name:         "last retains its bound",
			chain:        "system command list | ndjson | last 3",
			wantProduced: available,
			wantCount:    3,
			wantLines:    3,
		},
		{
			name:          "match writes before the source ends",
			chain:         "system command list | ndjson | match show",
			wantProduced:  available,
			wantCount:     available,
			wantLines:     available,
			wantStreaming: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			produced := 0
			writer := &witnessWriter{produced: &produced}
			answer, err := RenderRecords(
				writer,
				tt.chain,
				"",
				recordEnvelope,
				nil,
				commandRecords(available, &produced),
			)
			if err != nil {
				t.Fatalf("RenderRecords: %v", err)
			}
			if produced != tt.wantProduced {
				t.Errorf("source pulled %d records, want %d", produced, tt.wantProduced)
			}
			if answer.Count != tt.wantCount {
				t.Errorf("answer count = %d, want %d", answer.Count, tt.wantCount)
			}
			if lines := strings.Count(strings.TrimRight(writer.body.String(), "\n"), "\n") + 1; lines != tt.wantLines {
				t.Errorf("rendered %d lines, want %d", lines, tt.wantLines)
			}
			hasMetadata := strings.Contains(writer.body.String(), `"pipe"`)
			if hasMetadata != tt.wantMetadata {
				t.Errorf("metadata present = %v, want %v: %s", hasMetadata, tt.wantMetadata, writer.body.String())
			}
			if tt.wantStreaming && writer.firstAt >= available {
				t.Errorf("first write followed %d source records, want before all %d", writer.firstAt, available)
			}
		})
	}
}

func TestNDJSONMatchReadsTheRenderedLine(t *testing.T) {
	produced := 0
	records := func(yield func(rpc.Record) bool) {
		produced++
		yield(rpc.Record{Item: json.RawMessage(`{"value": "show cmd"}`)})
	}
	var out bytes.Buffer
	answer, err := RenderRecords(
		&out,
		`system command list | ndjson | match "value":"show cmd"`,
		"",
		recordEnvelope,
		nil,
		records,
	)
	if err != nil {
		t.Fatalf("RenderRecords: %v", err)
	}
	if produced != 1 || answer.Count != 1 {
		t.Errorf("source produced %d and answer counted %d, want one matched line", produced, answer.Count)
	}
	if !strings.Contains(out.String(), `"value":"show cmd"`) {
		t.Errorf("canonical NDJSON line did not match: %q", out.String())
	}
}

func TestNDJSONMatchReadsRenderedPositionalFields(t *testing.T) {
	const rows = rpc.AnswerBufferThreshold + 10
	produced := 0
	writer := &witnessWriter{produced: &produced}
	answer, err := RenderRecords(
		writer,
		`system command list | ndjson | match "value":"show cmd`,
		"",
		recordEnvelope,
		commandColumns,
		commandColumnRecords(rows, &produced),
	)
	if err != nil {
		t.Fatalf("RenderRecords: %v", err)
	}
	if produced != rows || answer.Count != rows {
		t.Errorf("source produced %d and answer counted %d, want %d positional matches", produced, answer.Count, rows)
	}
	if writer.firstAt != rpc.AnswerBufferThreshold+1 {
		t.Errorf("first positional write followed %d source records, want %d", writer.firstAt, rpc.AnswerBufferThreshold+1)
	}
	if !strings.Contains(writer.body.String(), `"value":"show cmd-0"`) {
		t.Errorf("rendered positional NDJSON line did not match: %q", writer.body.String())
	}
}

func TestNDJSONLineTransformsTreatFaultsAsRenderedLines(t *testing.T) {
	source := []rpc.Record{
		{Fault: json.RawMessage(`{"message":"first fault"}`)},
		{Item: json.RawMessage(`{"value":"middle"}`)},
		{Item: json.RawMessage(`{"value":"last"}`)},
	}
	tests := []struct {
		name         string
		chain        string
		wantProduced int
		wantCount    uint64
		wantFaults   uint64
		wantText     string
	}{
		{
			name:         "match cannot hide a fault line",
			chain:        "system command list | ndjson | match absent",
			wantProduced: 3,
			wantFaults:   1,
			wantText:     "first fault",
		},
		{
			name:         "first stops on a fault line",
			chain:        "system command list | ndjson | first 1",
			wantProduced: 1,
			wantFaults:   1,
			wantText:     "first fault",
		},
		{
			name:         "count includes fault lines",
			chain:        "system command list | ndjson | count",
			wantProduced: 3,
			wantCount:    1,
			wantText:     `"count":3`,
		},
		{
			name:         "last can discard a fault line",
			chain:        "system command list | ndjson | last 1",
			wantProduced: 3,
			wantCount:    1,
			wantText:     `"value":"last"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			produced := 0
			records := func(yield func(rpc.Record) bool) {
				for _, record := range source {
					produced++
					if !yield(record) {
						return
					}
				}
			}
			var out bytes.Buffer
			answer, err := RenderRecords(&out, tt.chain, "", recordEnvelope, nil, records)
			if err != nil {
				t.Fatalf("RenderRecords: %v", err)
			}
			if produced != tt.wantProduced {
				t.Errorf("source produced %d lines, want %d", produced, tt.wantProduced)
			}
			if answer.Count != tt.wantCount || answer.Faults != tt.wantFaults {
				t.Errorf("answer = %+v, want count %d faults %d", answer, tt.wantCount, tt.wantFaults)
			}
			if !strings.Contains(out.String(), tt.wantText) {
				t.Errorf("rendering %q does not contain %q", out.String(), tt.wantText)
			}
		})
	}
}

// TestNDJSONLineSpellingDoesNotChangeAtTheThreshold keeps mixed result and
// fault answers as one canonical JSON value per line on both sides of the
// document/stream framing boundary.
func TestNDJSONLineSpellingDoesNotChangeAtTheThreshold(t *testing.T) {
	for _, count := range []int{rpc.AnswerBufferThreshold, rpc.AnswerBufferThreshold + 1} {
		t.Run(textbuf.StringInt(int64(count))+" records", func(t *testing.T) {
			records := func(yield func(rpc.Record) bool) {
				if !yield(rpc.Record{Fault: json.RawMessage(`{"message":"threshold fault"}`)}) {
					return
				}
				for range count - 1 {
					if !yield(rpc.Record{Item: json.RawMessage(`{"value":"kept"}`)}) {
						return
					}
				}
			}

			var out bytes.Buffer
			answer, err := RenderRecords(
				&out,
				"system command list | ndjson | first 300",
				"",
				recordEnvelope,
				nil,
				records,
			)
			if err != nil {
				t.Fatalf("RenderRecords: %v", err)
			}
			lines := strings.Split(strings.TrimSuffix(out.String(), "\n"), "\n")
			if len(lines) != count {
				t.Fatalf("rendered %d lines, want %d: %.200q", len(lines), count, out.String())
			}
			if lines[0] != `{"message":"threshold fault"}` {
				t.Errorf("fault line = %q, want canonical NDJSON", lines[0])
			}
			if lines[1] != `{"value":"kept"}` || lines[len(lines)-1] != lines[1] {
				t.Errorf("result line spelling changed: first %q last %q", lines[1], lines[len(lines)-1])
			}
			wantType := rpc.AnswerTypeDocument
			if count > rpc.AnswerBufferThreshold {
				wantType = rpc.AnswerTypeMap
			}
			if answer.Type != wantType || answer.Count != uint64(count-1) || answer.Faults != 1 {
				t.Errorf("answer = %+v, want type %q count %d faults 1", answer, wantType, count-1)
			}
		})
	}
}

// TestMalformedPositionalNDJSONCannotBeDiscardedByMatch proves canonical
// conversion happens before line filtering. The malformed first row fails the
// walk closed even though neither it nor the following valid row matches.
func TestMalformedPositionalNDJSONCannotBeDiscardedByMatch(t *testing.T) {
	produced := 0
	records := func(yield func(rpc.Record) bool) {
		for _, item := range []string{`["short"]`, `["still","absent"]`} {
			produced++
			if !yield(rpc.Record{Item: json.RawMessage(item)}) {
				return
			}
		}
	}
	var out bytes.Buffer
	answer, err := RenderRecords(
		&out,
		"system command list | ndjson | match never-present",
		"",
		recordEnvelope,
		commandColumns,
		records,
	)
	if err != nil {
		t.Fatalf("RenderRecords: %v", err)
	}
	if produced != 1 {
		t.Errorf("malformed row pulled %d source records, want fail-closed stop after one", produced)
	}
	if answer.Count != 0 || answer.Faults != 1 {
		t.Errorf("answer = %+v, want one propagated conversion fault", answer)
	}
	if !strings.Contains(out.String(), "NDJSON cannot render positional row") {
		t.Errorf("conversion fault was discarded by match: %q", out.String())
	}
}

// TestNDJSONLastMeasuresCanonicalPositionalBytes makes the field-named object
// exceed the retention ceiling while the producer's positional array is tiny.
// The bound therefore proves which representation last actually retained.
func TestNDJSONLastMeasuresCanonicalPositionalBytes(t *testing.T) {
	fields := []string{strings.Repeat("f", recordsLastBytesLimit)}
	rows := slices.Values([]rpc.Record{{Item: json.RawMessage(`["x"]`)}})
	var out bytes.Buffer
	answer, err := RenderRecords(
		&out,
		"show test rows | ndjson | last 1",
		"",
		"rows",
		fields,
		rows,
	)
	if err != nil {
		t.Fatalf("RenderRecords: %v", err)
	}
	if answer.Count != 0 || answer.Faults != 1 {
		t.Errorf("answer = %+v, want one retention fault", answer)
	}
	if !strings.Contains(out.String(), lastRetentionBytesError()) {
		t.Errorf("wide canonical object was retained: output has %d bytes", out.Len())
	}
}

// TestNDJSONCountAndLastKeepBoundedMemory extends the record wrappers' memory
// contract through RenderRecords when ndjson precedes the transform.
func TestNDJSONCountAndLastKeepBoundedMemory(t *testing.T) {
	for _, chain := range []string{
		"show test rows | ndjson | count",
		"show test rows | ndjson | last 8",
	} {
		t.Run(chain, func(t *testing.T) {
			var walk recordWalk
			if _, err := RenderRecords(
				io.Discard,
				chain,
				"",
				"rows",
				nil,
				walk.records(memoryWalkRecords, memoryWalkPayload),
			); err != nil {
				t.Fatalf("RenderRecords: %v", err)
			}
			if walk.produced != memoryWalkRecords {
				t.Errorf("source pulled %d records, want %d", walk.produced, memoryWalkRecords)
			}
			if held := walk.heldAtLastRecord(); held > memoryWalkBudget {
				t.Errorf("%s held %d bytes at the last record, want under %d of the %d walked",
					chain, held, uint64(memoryWalkBudget), uint64(memoryWalkTotal))
			}
		})
	}
}
