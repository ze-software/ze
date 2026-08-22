// Design: docs/architecture/api/ipc_protocol.md — the Answer Protocol, plugin side
// Overview: plugin.go — the command handlers a record answer is returned from
// Related: ../../internal/component/plugin/types.go — Records, the engine-side twin
// Related: rpc/answer_write.go — WriteRecordAnswer, the one writer both ends use
// Related: rpc/collapse.go — CollapseRecords, the document a buffered reader gets
//
// The record producer a plugin command handler answers with. A handler that
// walks a large collection returns the walk rather than the collection, so the
// SDK writes one row at a time and neither the plugin nor the engine holds the
// whole answer.

package plugin

import (
	"io"
	"iter"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// Row is one row of a record answer, as the bytes it appends rather than as the
// bytes it allocates. AppendTo appends the row's JSON to buf and returns the
// extended slice, which is the buffer-ownership shape Ze uses everywhere a
// value reaches a wire: the caller owns the buffer and the row writes into it
// (ai/rules/performance.md, family.Family.AppendTo).
//
// A row type that returned a fresh []byte for each row would put one allocation
// on every row of every walk, and no later work could take it back off. That is
// the single reason this is an appender and not a []byte.
//
// AppendTo MUST append one valid JSON value, and MUST NOT write over the bytes
// already in buf: those are the rows written before it.
//
// The writer appends the row before the yield that carried it returns, so a
// producer MAY hand back one Row value for every row of a walk and refill it in
// place.
//
// It is rpc.Row under the name a plugin author reads, because one contract
// serves both producers: a plugin's handler and the engine's own handlers write
// their answers through the one writer (rpc.WriteRecordAnswer).
type Row = rpc.Row

// Record is one line of a record answer. Exactly one of Item and Fault is set:
// Item is a row the command produced, Fault is a row it rejected while the walk
// continued. The terminator counts the two separately, which is what lets one
// answer report the rows applied beside the rows refused.
//
// It is rpc.RowRecord under the name a plugin author reads, and it is what the
// engine's own handlers yield too.
type Record = rpc.RowRecord

// Records is what a command handler answers with when its walk produces rows.
// The engine writes them as a head, one line for each record, and a terminator
// carrying the counts, so a consumer reading `| first 10` over a table of a
// million rows pays for the rows it keeps.
//
// Key names the envelope the rows belong under, and the head line carries it. A
// handler MUST NOT name "errors": that envelope holds the rejected rows, and
// both producers refuse the collision rather than letting one collection
// overwrite the other.
//
// Fields names the columns of an answer whose rows share one schema, in column
// order. A handler that declares them yields each row as a JSON array of values
// in that order, and the head carries the names once instead of every row
// carrying them. A handler that declares none yields self-describing objects.
//
// A handler that declares fields MUST yield every row as a JSON array carrying
// exactly one value for each name, in the same order. The engine reads the two
// against each other by POSITION, so a short row would gain a column it never
// carried and a long one would lose a value. Neither is repaired: the row is
// refused here rather than at whichever consumer meets it
// (rpc.ErrRowArity, rpc.ErrRowNotPositional).
//
// A Records is walked on the one goroutine serving the command. It is not safe
// for concurrent use, and the walk starts no goroutine of its own: a row costs
// an append, never a scheduler.
type Records struct {
	Key    string
	Fields []string

	// Rows is the walk itself, and the two sides of it owe each other one
	// obligation each (R-3 of spec-record-answers-1-sdk-path).
	//
	// The SDK MUST walk Rows to its end, or stop the walk deliberately, before
	// the command call that carried this Records returns to the engine.
	//
	// The handler MUST keep whatever Rows reads alive until that walk ends, and
	// MUST NOT store the sequence for a later reader. It is the answer being
	// written, not a collection that can be read again: a second range over it
	// yields what the underlying walk yields the second time, which for a live
	// registry is a different answer.
	Rows iter.Seq[Record]
}

// WriteAnswer writes r to w as the answer sequence for the request under id:
// the head that states how each record is read, one line for each row of the
// walk, and a terminator carrying the counts.
//
// message is the operational text the TERMINATOR carries: why the handler
// produced fewer rows than it set out to, or why it produced none. It is empty
// for a walk that ran to its end with nothing to say. The terminator is the one
// line an answer states its outcome on, so a handler that failed states it here
// and no other line can disagree with it.
//
// Which SHAPE the answer takes is not the handler's to choose:
// rpc.WriteRecordAnswer decides it from the row count, so a walk that ends
// within rpc.AnswerBufferThreshold rows is the one document a command answered
// with before it produced rows at all.
//
// The walk runs to its end here, before this returns, which is the SDK's half of
// the obligation Rows states: the handler keeps what Rows reads alive until this
// returns, and nothing stores the sequence past it. Nothing here starts a
// goroutine, for the answer or for a row (ai/rules/goroutine-lifecycle.md).
//
// A row wider than one wire message is reported as a rejected row and the walk
// goes on, so refusing one row does not cost the operator the rows around it.
// An answer naming rpc.AnswerErrorsKey is refused before its first line.
func (r Records) WriteAnswer(w io.Writer, id uint64, message string) error {
	head := rpc.AnswerTail{Message: message, Key: r.Key, Fields: r.Fields}
	return rpc.WriteRecordAnswer(w, id, head, r.Rows)
}

// MarshalJSON renders the walk as the single document a buffered consumer
// reads: the rows the command produced, in walk order, under Key, and the rows
// it rejected under rpc.AnswerErrorsKey beside them.
//
// It is the buffered half of this type, and it exists because not every
// transport carries a line for each record. An in-process caller reached over
// the direct bridge takes one marshaled value, and a handler that answers with
// a walk must reach it as the same document the record path collapses to. The
// collapse is rpc.CollapseRecords, the one both ends of the connection use.
//
// Rows is walked ONCE, here or by WriteAnswer, never by both: a second range
// over a live registry yields a different answer, which is why Rows states the
// walk as the answer being written rather than as a collection.
// A collapse holds every row until the document is built, so each row is
// appended into a slice of its own here (rpc.HeldRecords). That is the one
// allocation the record path still pays for a row, and it is the price of
// holding: the walk that WRITES an answer appends into the encoder's buffers
// and pays none.
//
// A nil Rows is a handler that named an envelope and produced nothing, which is
// an empty collection rather than a missing answer.
//
// A record carrying neither an item nor a fault is refused by name
// (rpc.ErrEmptyAnswerRecord) rather than read as a row that carries nothing.
func (r Records) MarshalJSON() ([]byte, error) {
	return rpc.CollapseRecords(r.Key, r.Fields, rpc.HeldRecords(r.Rows))
}
