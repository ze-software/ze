package command

import (
	"bytes"
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

// renderedDocument is what the string path answers for the same rows: the
// collapse the buffered surfaces read, put through the same chain. It is the
// comparison every case below is judged against, built by a different route.
func renderedDocument(t *testing.T, chain, key string, count int) string {
	t.Helper()
	document := collapseForTest(t, key, count)
	_, format, errMsg := ProcessPipesDefaultFormatChecked(chain, "")
	if errMsg != "" {
		t.Fatalf("ProcessPipesDefaultFormatChecked(%q): %s", chain, errMsg)
	}
	return strings.TrimRight(format(document), "\n")
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
	if !strings.Contains(body, `"pipe":{"match":"show"}`) {
		t.Errorf("the rendering dropped the pipe metadata: %.200q", body)
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
// VALIDATES: AC-3 -- a command that returns no data answers count=0 and the
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
