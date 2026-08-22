// Design: docs/architecture/api/ipc_protocol.md -- the answer wire grammar
// Related: message.go -- AppendAnswerHead and its siblings, the lines this writes
//          collapse.go -- the document a walk under the threshold collapses to
//          answer_row.go -- checkRowArity, the schema a positional row is held to
//          types.go -- NewAnswer, the in-process sibling of this writer
//
// answer_write.go writes one answer to the wire: a head line carrying the
// verdict and the type, one line for each record, and a terminator carrying the
// counts. A reader never branches on how many records arrive, and nothing
// states a count the records can contradict.
//
// It lives here rather than beside either producer because BOTH ends of the
// plugin connection write answers and neither owns the grammar. The engine
// writes the answer to a command an operator ran (WriteAnswer,
// internal/component/plugin/dispatch.go) and the SDK writes the answer to a
// command the engine asked a plugin to run (Records.WriteAnswer,
// pkg/plugin/records.go). One writer is what makes the two agree about what a
// head, a record and a terminator are, exactly as one collapse makes the two
// agree about what a document is.

package rpc

import (
	"encoding/json"
	"fmt"
	"io"
	"iter"
	"slices"
	"strconv"
)

// WriteRecordAnswer writes the answer rows produce, under id, to w.
//
// The head's type= is decided HERE, from the walk, and never by the producer.
// The encoder holds up to AnswerBufferThreshold records: a walk that ends
// within them is answered as one AnswerTypeDocument document, which is the JSON a
// command answered with before it produced records at all, so no consumer of an
// existing command meets a new shape. A walk that passes them is streamed, as
// AnswerTypeTable when head declares its columns and AnswerTypeMap when it
// does not, and the held records are written first in walk order, so the switch
// loses none of them.
//
// head states what the producer knows before the walk runs: Key is the envelope
// the head opens the answer under, Fields is the column schema its positional
// rows are read against, and Message is the operational text the TERMINATOR
// carries. Type, Count and Faults are the walk's and are ignored here. head
// states no outcome at all: the terminator is the one line that carries one.
//
// A rejected row is written and the walk goes on, so one answer can report 97
// rows applied and 3 rejected. A row too wide for one line is rejected the same
// way (appendAnswerRow), because refusing one row must not cost the operator the
// rows around it. A failed WRITE ends the walk: the transport is gone, and every
// later row would be produced for nobody. Returning stops the generator, which
// the range does by refusing the next yield.
//
// rows is walked once and never stored, and nothing here starts a goroutine for
// the answer or for a row (ai/rules/goroutine-lifecycle.md). A nil rows is a
// producer that named an envelope and yielded nothing, which is an empty
// collection rather than a missing answer.
//
// A row appends itself into a buffer this writer owns and is never kept, so a
// streamed row costs no allocation at all: the encoder holds the row's LENGTH
// and nothing else about it (Row, types.go).
//
// Two pooled buffers carry one answer (AnswerLineBuffer): the line every line
// is written through, and the one the rows are held in until the threshold
// decides the answer's type. The deferred releases cover the ways out that
// reach no terminator: a row this producer refuses, a row that does not fit the
// columns the head declares, a collapse that fails, and the transport dying
// under any line.
func WriteRecordAnswer(w io.Writer, id uint64, head AnswerTail, rows iter.Seq[RowRecord]) error {
	// The reserved envelope is refused before the first line, so a producer
	// that names it is told rather than half-answered. The collapse refuses the
	// same name (CollapseRecords), and a streamed answer never reaches it.
	if head.Key == AnswerErrorsKey {
		return ErrReservedEnvelopeKey
	}
	fields, err := marshalAnswerFields(head.Fields)
	if err != nil {
		return err
	}
	if rows == nil {
		rows = noAnswerRows
	}

	line := AnswerLineBuffer()
	defer ReleaseAnswerLineBuffer(line)

	holding := AnswerLineBuffer()
	defer ReleaseAnswerLineBuffer(holding)

	var (
		held      []Record
		count     uint64
		faults    uint64
		streaming bool
	)
	for row := range rows {
		// The ordinal is the row's position in the walk, counted from one, and
		// it is how a rejected row names the row it stands for.
		ordinal := count + faults + 1

		if streaming {
			kind, writeErr := writeAnswerRow(w, line, id, ordinal, row, head.Fields)
			if writeErr != nil {
				return writeErr
			}
			// A row the width refused is counted as a rejection whatever its
			// producer said it was, because a rejection is what the wire
			// carried.
			if kind == AnswerKindFault {
				faults++
			} else {
				count++
			}
			continue
		}

		record, kind, holdErr := holdAnswerRow(holding, id, ordinal, row)
		if holdErr != nil {
			return holdErr
		}
		if kind == AnswerKindFault {
			faults++
		} else {
			count++
		}
		held = append(held, record)
		if len(held) <= AnswerBufferThreshold {
			continue
		}

		// This record is the one past the threshold, so the answer cannot be
		// one document. The head opens the stream and every record held goes
		// out behind it, in the order the walk produced them.
		*line = AppendAnswerHead((*line)[:0], id, AnswerStreamType(head.Fields), head.Key, fields)
		if err := writeAnswerLine(w, line); err != nil {
			return err
		}
		for i := range held {
			if err := writeRecordLine(w, line, id, held[i], head.Fields); err != nil {
				return err
			}
		}
		held = nil
		streaming = true
	}

	if !streaming {
		document, collapseErr := CollapseRecords(head.Key, head.Fields, slices.Values(held))
		if collapseErr != nil {
			return collapseErr
		}
		return writeDocumentLines(w, line, id, head, document, count, faults)
	}

	*line = AppendAnswerTerminator((*line)[:0], id, count, faults, head.Message)
	return writeAnswerLine(w, line)
}

// WriteDocumentAnswer writes the answer of a producer that built its whole
// payload before the answer opened: a head naming AnswerTypeDocument, that one
// document as the single record, and the terminator. Only head's Message is
// read, because the document already carries its own envelope and two
// statements of one fact can disagree.
//
// A payload built whole is one row, and a producer that answered with no data
// at all is none, so the terminator counts one or zero. It rejects nothing
// unless the document is wider than one wire message, which is the one refusal
// this answer can carry and the one case where its head names another type
// (writeDocumentLines).
//
// It is WriteRecordAnswer's sibling for the answer no walk produced, and the
// two write the same three lines for the same document, through a line buffer
// from the same pool.
func WriteDocumentAnswer(w io.Writer, id uint64, head AnswerTail, document json.RawMessage) error {
	var count uint64
	if len(document) > 0 {
		count = 1
	}

	line := AnswerLineBuffer()
	defer ReleaseAnswerLineBuffer(line)

	return writeDocumentLines(w, line, id, head, document, count, 0)
}

// writeDocumentLines writes the head, the one item line and the terminator of
// an answer that fits one document, through the line buffer its caller took
// from the pool and gives back.
//
// An empty document is a producer that answered with no data at all, and it
// writes no item line: nothing is not the same answer as an empty collection.
// count and faults are the WALK's, so the terminator states what the answer
// produced whichever type carried it, which is what a collapsed short walk and
// a streamed long one have in common.
//
// The document is bounded here for the reason every record line is bounded
// (boundedRecord): it IS a record line, and a line holds at most
// MaxMessageSize. Writing a wider one fails the write, so the answer would stop
// before its terminator and a consumer would read an answer the daemon produced
// whole as a truncated one (R-7 of spec-record-answers-2-only-encoding).
//
// A document with no line makes this not a document answer at all, so the head
// says so. What reaches the consumer is the one rejected row that stands in for
// the document, and that is the record a streamed answer already carries a
// rejection in: head, one `bad`, terminator counting no record and one
// rejection. No reader meets a new shape, and the buffered one refuses the
// alternative by name, because a document has nowhere to carry a rejected row
// beside itself (CollapseAnswer, collapse.go).
//
// The counts then state what the wire carried rather than what the walk
// produced. The rows the walk rejected traveled INSIDE that document, so
// counting them would name rows no consumer received.
func writeDocumentLines(w io.Writer, line *[]byte, id uint64, head AnswerTail, document json.RawMessage, count, faults uint64) error {
	// The document is the answer's first and only record, so it is the record
	// boundedRecord names when it rejects one.
	answerType, record := AnswerTypeDocument, Record{Item: document}
	if len(document) > 0 {
		record = boundedRecord(id, 1, record)
	}
	if len(record.Fault) > 0 {
		answerType, count, faults = AnswerTypeMap, 0, 1
	}

	*line = AppendAnswerHead((*line)[:0], id, answerType, "", nil)
	if err := writeAnswerLine(w, line); err != nil {
		return err
	}
	if len(document) > 0 {
		if err := writeRecordLine(w, line, id, record, nil); err != nil {
			return err
		}
	}
	*line = AppendAnswerTerminator((*line)[:0], id, count, faults, head.Message)
	return writeAnswerLine(w, line)
}

// writeRecordLine writes one record line through the answer's line buffer. Its
// caller has already refused a record carrying neither an item nor a fault, so
// an item here is never empty.
//
// It serves both shapes an answer takes: one line for each row of a streamed
// answer, and the one line a collapsed answer's document occupies. Both write
// the same two kinds for the same two cases, so one writer is what keeps a
// rejected row spelled the same way whichever shape carried it.
//
// A row of an answer that declares columns is checked against them here,
// because the row reaches the wire unchanged and a consumer reading it by
// position cannot tell a short row from a value it should have had. A document
// declares none: it is one value rather than a positional row.
func writeRecordLine(w io.Writer, line *[]byte, id uint64, record Record, fields []string) error {
	if len(record.Fault) > 0 {
		*line = AppendAnswerFault((*line)[:0], id, record.Fault)
		return writeAnswerLine(w, line)
	}
	if err := checkRowArity(record.Item, fields); err != nil {
		return err
	}
	*line = AppendAnswerItem((*line)[:0], id, record.Item)
	return writeAnswerLine(w, line)
}

// writeAnswerRow appends one row of a STREAMED answer into the line buffer and
// writes the line it makes.
//
// The prefix states how many payload bytes follow, and a row states its width
// only by appending itself, so the line is built back to front: the gap is
// reserved, the row appends behind it, and the prefix is then written to END
// exactly where the payload begins. The line therefore starts INSIDE the
// buffer, and the bytes of the gap the prefix did not need are never written to
// w. That is the skip-and-backfill of ai/rules/performance.md, right-aligned
// because a decimal count has no fixed width.
//
// The row is built once and reaches the wire from where it was built. Nothing
// copies it, and the width rejection does not rebuild it (appendAnswerRow).
//
// It reports which record the row turned out to be. The caller then counts a
// rejected row as a rejection, whether the producer stated one or the width
// did.
func writeAnswerRow(w io.Writer, line *[]byte, id, ordinal uint64, row RowRecord, fields []string) (string, error) {
	*line = append((*line)[:0], answerRecordGap[:]...)

	appended, kind, err := appendAnswerRow(*line, id, ordinal, row)
	*line = appended
	if err != nil {
		return "", err
	}

	payload := (*line)[answerRecordPrefixWidth:]
	if kind == AnswerKindRecord {
		if err := checkRowArity(payload, fields); err != nil {
			return "", err
		}
	}

	var scratch [answerRecordPrefixWidth]byte
	prefix := appendAnswerRecordPrefix(scratch[:0], id, kind, uint64(len(payload)))
	start := answerRecordPrefixWidth - len(prefix)
	copy((*line)[start:], prefix)

	return kind, writeAnswerLineFrom(w, line, start)
}

// holdAnswerRow appends one row into the buffer the answer holds its rows in
// until the threshold decides its type, and reports the record those bytes are.
//
// The record's payload is a slice of that buffer, capped to its own length so
// the row after it cannot be appended over it. It stays valid for the whole
// answer: a later row that has to grow the buffer copies the rows already held
// into the new array and leaves the old one where the records already handed
// out still point.
func holdAnswerRow(holding *[]byte, id, ordinal uint64, row RowRecord) (Record, string, error) {
	start := len(*holding)

	appended, kind, err := appendAnswerRow(*holding, id, ordinal, row)
	*holding = appended
	if err != nil {
		return Record{}, "", err
	}

	end := len(*holding)
	payload := (*holding)[start:end:end]
	if kind == AnswerKindFault {
		return Record{Fault: payload}, kind, nil
	}
	return Record{Item: payload}, kind, nil
}

// appendAnswerRow appends the bytes row writes into buf. It reports which
// record line they make: the row the producer stated, or the rejection that
// stands in for a row wider than one wire message.
//
// The row is built ONCE. A row too wide for a line is rejected in place, by
// rewinding the buffer to where the row started and writing the rejection
// there, so a rejection costs the walk the row it already built and never a
// second one (AC-9 of spec-record-answers-3-zero-alloc). The buffer keeps the
// width that row grew it to for the rest of the answer, and the pool drops it
// afterwards rather than holding it (ReleaseAnswerLineBuffer).
//
// A row that appends nothing is refused by name: it reaches the wire as a line
// with no value, which no consumer can decode, and reaches a buffered consumer
// as null, which reads like a row the command produced.
func appendAnswerRow(buf []byte, id, ordinal uint64, row RowRecord) ([]byte, string, error) {
	start := len(buf)
	kind := AnswerKindRecord
	switch {
	case row.Fault != nil:
		kind = AnswerKindFault
		buf = row.Fault.AppendTo(buf)
	case row.Item != nil:
		buf = row.Item.AppendTo(buf)
	}
	if len(buf) == start {
		return buf, "", ErrEmptyAnswerRecord
	}

	size := answerRowLineSize(id, kind, len(buf)-start)
	if size <= MaxMessageSize {
		return buf, kind, nil
	}
	return appendAnswerRecordTooLargeFault(buf[:start], ordinal, size), AnswerKindFault, nil
}

// boundedRecord is record when its line fits one wire message, and the rejected
// row that stands in for it when it does not. A record is one line, so a record
// wider than MaxMessageSize has no wire form at all.
//
// Rejecting it rather than failing the write is what keeps the rest of the
// answer. A walk that stopped there would discard every later row and reach no
// terminator, and a consumer reads a missing terminator as a truncated answer
// (Verdict), so one wide row would be reported as a lost connection. The answer
// instead reaches its terminator, which counts the rejection, and the
// verdict derives to partial.
//
// It is measured from the record's bytes, which the caller already holds: this
// runs over the DOCUMENT a short walk collapsed to, and over the payload a
// producer built whole. A row on its way to a line of its own is measured from
// what it appended instead (appendAnswerRow), because a row states its width by
// writing itself and by nothing else.
//
// A buffered rendering writes no lines and rejects nothing, so a record this
// wide is the one payload whose two renderings differ, and they differ because
// one transport bounds a line and the other does not.
//
// It runs over the collapsed document as well as over each row
// (writeDocumentLines), because the document is itself one line. Bounding the
// rows alone leaves 256 rows within the limit collapsing to a document past it,
// which no line can carry either.
//
// ordinal is the record's position in the walk, counted from one, and it is how
// the rejected row names the record it stands for. A document is the answer's
// only record, so it is record one.
func boundedRecord(id, ordinal uint64, record Record) Record {
	size := AnswerRecordLineSize(id, record)
	if size <= MaxMessageSize {
		return record
	}
	fault := appendAnswerRecordTooLargeFault(make([]byte, 0, answerFaultCapacity), ordinal, size)
	return Record{Fault: fault}
}

// answerFaultCapacity is the capacity a record-too-large fault needs: 99 bytes
// of fixed text and three decimal numbers of at most 20 digits each, so 192
// holds every fault appendAnswerRecordTooLargeFault can write without growing
// the slice.
const answerFaultCapacity = 192

// appendAnswerRecordTooLargeFault appends the rejected row that stands in for a
// record wider than one wire message. It names the record by its position in
// the walk and states the two sizes, so an operator can find the row that was
// rejected.
//
// It quotes nothing of the record. A fault carrying the row it rejected would be
// rejected for the same reason, and the row can be 16 MB. Its own length is
// bounded by construction (answerFaultCapacity), so it always fits the line the
// record did not.
//
// It appends rather than returning a slice because the row it replaces was
// appended: the rejection is written where that row was built, over the bytes
// the answer is about to forget.
func appendAnswerRecordTooLargeFault(buf []byte, ordinal uint64, size int) []byte {
	buf = append(buf, `{"message":"answer record does not fit one wire message","record":`...)
	buf = strconv.AppendUint(buf, ordinal, 10)
	buf = append(buf, `,"encoded-bytes":`...)
	buf = strconv.AppendInt(buf, int64(size), 10)
	buf = append(buf, `,"limit-bytes":`...)
	buf = strconv.AppendInt(buf, MaxMessageSize, 10)
	return append(buf, '}')
}

// writeAnswerLine frames the line already appended into the answer's buffer
// with the newline that ends every wire message, and writes it in one call, so
// a reader never takes delivery of half a line.
//
// It writes back through the pointer, so the buffer a line grew is the buffer
// the next line of that answer writes into and the buffer the answer returns to
// the pool. w takes the bytes and keeps none: the buffer belongs to another
// answer once this one releases it.
func writeAnswerLine(w io.Writer, line *[]byte) error {
	return writeAnswerLineFrom(w, line, 0)
}

// writeAnswerLineFrom writes the line that starts at start in the answer's
// buffer, framed with its newline.
//
// A line starts at start rather than at zero when its prefix was written
// backwards into a gap, which is how a record line is built around a row that
// had to append itself before its width was known (writeAnswerRow). The bytes
// in front of start are the part of that gap the prefix did not need, and they
// are not part of any line.
func writeAnswerLineFrom(w io.Writer, line *[]byte, start int) error {
	*line = append(*line, '\n')
	if _, err := w.Write((*line)[start:]); err != nil {
		return fmt.Errorf("write answer line: %w", err)
	}
	return nil
}

// marshalAnswerFields encodes the column names a streamed answer's head
// carries. It runs once for the answer, before any line is written, so a schema
// that cannot be encoded is named instead of half an answer being written.
func marshalAnswerFields(fields []string) (json.RawMessage, error) {
	if len(fields) == 0 {
		return nil, nil
	}
	encoded, err := json.Marshal(fields)
	if err != nil {
		return nil, fmt.Errorf("marshal answer fields: %w", err)
	}
	return encoded, nil
}

// AnswerStreamType is how a streamed answer's records are read: positional
// arrays against the head's fields when the answer declares a column schema,
// self-describing objects when it does not.
//
// It is exported because a producer that hands its answer to an in-process
// consumer states the type itself (NewAnswer, types.go) where a producer
// writing the wire has it decided by the walk, and the two must name the same
// type for the same schema.
func AnswerStreamType(fields []string) string {
	if len(fields) == 0 {
		return AnswerTypeMap
	}
	return AnswerTypeTable
}

// noAnswerRows is the empty row sequence. A producer that names an envelope and
// carries no generator produced an empty collection, which is what a command
// that produced nothing answered with; ranging a nil iter.Seq panics instead of
// saying so.
func noAnswerRows(func(RowRecord) bool) {}
