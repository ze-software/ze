// Design: docs/architecture/api/ipc_protocol.md -- the answer wire grammar
// Related: message.go -- AppendAnswerHead and its siblings, the lines this writes
//          collapse.go -- the document a walk under the threshold collapses to
//          answer_row.go -- CheckRowArity, the schema a positional row is held to
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

// answerLineCapacity is the initial byte capacity of the one line buffer an
// answer reuses for every line it writes. A longer record grows the slice once,
// and every later line of that answer reuses the grown slice.
const answerLineCapacity = 512

// WriteRecordAnswer writes the answer rows produce, under id, to w.
//
// The head's type= is decided HERE, from the walk, and never by the producer.
// The encoder holds up to AnswerBufferThreshold records: a walk that ends
// within them is answered as one AnswerTypeJSON document, which is the JSON a
// command answered with before it produced records at all, so no consumer of an
// existing command meets a new shape. A walk that passes them is streamed, as
// AnswerTypeStream when head declares its columns and AnswerTypeNDJSON when it
// does not, and the held records are written first in walk order, so the switch
// loses none of them.
//
// head states what the producer knows before the walk runs: Status and Key open
// the answer, Fields is the column schema its positional rows are read against,
// and Message is the operational text the TERMINATOR carries. Type, Count and
// Faults are the walk's and are ignored here.
//
// A rejected row is written and the walk goes on, so one answer can report 97
// rows applied and 3 rejected. A row too wide for one line is rejected the same
// way (boundedRecord), because refusing one row must not cost the operator the
// rows around it. A failed WRITE ends the walk: the transport is gone, and every
// later row would be produced for nobody. Returning stops the generator, which
// the range does by refusing the next yield.
//
// rows is walked once and never stored, and nothing here starts a goroutine for
// the answer or for a row (ai/rules/goroutine-lifecycle.md). A nil rows is a
// producer that named an envelope and yielded nothing, which is an empty
// collection rather than a missing answer.
func WriteRecordAnswer(w io.Writer, id uint64, head AnswerTail, rows iter.Seq[Record]) error {
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
		rows = noAnswerRecords
	}

	buf := make([]byte, 0, answerLineCapacity)
	var (
		held      []Record
		count     uint64
		faults    uint64
		streaming bool
	)
	for record := range rows {
		record = boundedRecord(id, count+faults+1, record)
		switch {
		case len(record.Fault) > 0:
			faults++
		case len(record.Item) > 0:
			count++
		default:
			return ErrEmptyAnswerRecord
		}

		if !streaming && len(held) < AnswerBufferThreshold {
			held = append(held, record)
			continue
		}
		if !streaming {
			// This record is the one past the threshold, so the answer cannot
			// be one document. The head opens the stream and the records
			// already held go out ahead of it, in the order the walk produced
			// them.
			buf, err = writeAnswerLine(w, AppendAnswerHead(buf[:0], id, head.Status, AnswerStreamType(head.Fields), head.Key, fields))
			if err != nil {
				return err
			}
			for i := range held {
				buf, err = writeRecordLine(w, buf, id, held[i], head.Fields)
				if err != nil {
					return err
				}
			}
			held = nil
			streaming = true
		}

		buf, err = writeRecordLine(w, buf, id, record, head.Fields)
		if err != nil {
			return err
		}
	}

	if !streaming {
		document, collapseErr := CollapseRecords(head.Key, head.Fields, slices.Values(held))
		if collapseErr != nil {
			return collapseErr
		}
		return writeDocumentLines(w, buf, id, head, document, count, faults)
	}

	_, err = writeAnswerLine(w, AppendAnswerTerminator(buf[:0], id, count, faults, head.Message))
	return err
}

// WriteDocumentAnswer writes the answer of a producer that built its whole
// payload before the answer opened: a head naming AnswerTypeJSON, that one
// document as the single record, and the terminator. Only head's Status and
// Message are read, because the document already carries its own envelope and
// two statements of one fact can disagree.
//
// A payload built whole is one row, and a producer that answered with no data
// at all is none, so the terminator counts one or zero and rejects nothing.
//
// It is WriteRecordAnswer's sibling for the answer no walk produced, and the
// two write the same three lines for the same document.
func WriteDocumentAnswer(w io.Writer, id uint64, head AnswerTail, document json.RawMessage) error {
	var count uint64
	if len(document) > 0 {
		count = 1
	}
	return writeDocumentLines(w, make([]byte, 0, answerLineCapacity), id, head, document, count, 0)
}

// writeDocumentLines writes the head, the one item line and the terminator of
// an answer that fits one document, and returns the line buffer's fate to the
// caller through w alone.
//
// An empty document is a producer that answered with no data at all, and it
// writes no item line: nothing is not the same answer as an empty collection.
// count and faults are the WALK's, so the terminator states what the answer
// produced whichever type carried it, which is what a collapsed short walk and
// a streamed long one have in common.
func writeDocumentLines(w io.Writer, buf []byte, id uint64, head AnswerTail, document json.RawMessage, count, faults uint64) error {
	buf, err := writeAnswerLine(w, AppendAnswerHead(buf[:0], id, head.Status, AnswerTypeJSON, "", nil))
	if err != nil {
		return err
	}
	if len(document) > 0 {
		buf, err = writeAnswerLine(w, AppendAnswerItem(buf[:0], id, document))
		if err != nil {
			return err
		}
	}
	_, err = writeAnswerLine(w, AppendAnswerTerminator(buf[:0], id, count, faults, head.Message))
	return err
}

// writeRecordLine writes one record of a streamed answer and returns the line
// buffer for the next one. Its caller has already refused a record carrying
// neither an item nor a fault, so an item here is never empty.
//
// A row of an answer that declares columns is checked against them here,
// because the row reaches the wire unchanged and a consumer reading it by
// position cannot tell a short row from a value it should have had.
func writeRecordLine(w io.Writer, buf []byte, id uint64, record Record, fields []string) ([]byte, error) {
	if len(record.Fault) > 0 {
		return writeAnswerLine(w, AppendAnswerFault(buf[:0], id, record.Fault))
	}
	if err := CheckRowArity(record.Item, fields); err != nil {
		return buf, err
	}
	return writeAnswerLine(w, AppendAnswerItem(buf[:0], id, record.Item))
}

// boundedRecord is record when its line fits one wire message, and the rejected
// row that stands in for it when it does not. A record is one line, so a record
// wider than MaxMessageSize has no wire form at all.
//
// Rejecting it rather than failing the write is what keeps the rest of the
// answer. A walk that stopped there would discard every later row and reach no
// terminator, and a consumer reads a missing terminator as a truncated answer
// (Verdict), so one wide row would be reported as a lost connection. The answer
// instead reaches its terminator, which counts the rejection in faults=, and the
// verdict derives to partial.
//
// It is measured before the line is built, so the row is never copied into a
// line buffer that would be thrown away and left grown for the rest of the
// answer.
//
// It runs before the answer type is known, so the rejection reaches both forms
// the answer can take: the record's own line when the walk streams, and the
// collapsed document when it does not. A buffered rendering writes no lines and
// rejects nothing, so a record this wide is the one payload whose two renderings
// differ, and they differ because one transport bounds a line and the other does
// not.
//
// ordinal is the record's position in the walk, counted from one, and it is how
// the rejected row names the record it stands for.
func boundedRecord(id, ordinal uint64, record Record) Record {
	size := AnswerRecordLineSize(id, record)
	if size <= MaxMessageSize {
		return record
	}
	return Record{Fault: answerRecordTooLargeFault(ordinal, size)}
}

// answerFaultCapacity is the capacity answerRecordTooLargeFault builds into: 99
// bytes of fixed text and three decimal numbers of at most 20 digits each, so
// 192 holds every fault it can write without growing the slice.
const answerFaultCapacity = 192

// answerRecordTooLargeFault is the rejected row that stands in for a record
// wider than one wire message. It names the record by its position in the walk
// and states the two sizes, so an operator can find the row that was rejected.
//
// It quotes nothing of the record. A fault carrying the row it rejected would be
// rejected for the same reason, and the row can be 16 MB. Its own length is
// bounded by construction (answerFaultCapacity), so it always fits the line the
// record did not.
func answerRecordTooLargeFault(ordinal uint64, size int) json.RawMessage {
	fault := make([]byte, 0, answerFaultCapacity)
	fault = append(fault, `{"message":"answer record does not fit one wire message","record":`...)
	fault = strconv.AppendUint(fault, ordinal, 10)
	fault = append(fault, `,"encoded-bytes":`...)
	fault = strconv.AppendInt(fault, int64(size), 10)
	fault = append(fault, `,"limit-bytes":`...)
	fault = strconv.AppendInt(fault, MaxMessageSize, 10)
	return append(fault, '}')
}

// writeAnswerLine frames line with the newline that ends every wire message and
// writes it in one call, so a reader never takes delivery of half a line. It
// returns the framed slice, which the caller reuses for the next line.
func writeAnswerLine(w io.Writer, line []byte) ([]byte, error) {
	line = append(line, '\n')
	if _, err := w.Write(line); err != nil {
		return line, fmt.Errorf("write answer line: %w", err)
	}
	return line, nil
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
		return AnswerTypeNDJSON
	}
	return AnswerTypeStream
}

// noAnswerRecords is the empty row sequence. A producer that names an envelope
// and carries no generator produced an empty collection, which is what a command
// that produced nothing answered with; ranging a nil iter.Seq panics instead of
// saying so.
func noAnswerRecords(func(Record) bool) {}
