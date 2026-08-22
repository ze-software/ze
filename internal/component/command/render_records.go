// Design: docs/architecture/api/commands.md — CLI pipe operators
// Overview: pipe_records.go — the chain the records pass through before this
// Related: pipe.go — the same operator chain applied to one whole payload
//
// render_records.go turns the records of one answer into the bytes an operator
// reads. It is the consumer pipe_records.go names: a format operator changes no
// record, so the rendering happens here, once, at the edge that owns the writer.
//
// Two decisions live in this file and they are independent.
//
// The first is what the answer IS, and it is read from the walk: a walk that
// ends within rpc.AnswerBufferThreshold records is one document, and a walk that
// passes it is a stream. The plugin connection's encoder makes the same decision
// from the same constant (WriteAnswer, internal/component/plugin/dispatch.go), so
// the two channels agree about what a bounded answer is.
//
// The second is how the answer is RENDERED, and it is read from the chain. Only
// `| ndjson` names one JSON value per line, so only `| ndjson` can be written as
// the records arrive. Every other format renders a document: `| json` and
// `| yaml` serialize one, `| table` and `| text` measure a column over every row,
// and `| raw` is the document itself. Those collect, which is the cost R-6 of
// spec-streaming-answer-protocol accepts and states rather than hides.

package command

import (
	"encoding/json"
	"errors"
	"io"
	"iter"
	"slices"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// RecordAnswer is what a walk turned out to be, for the caller that frames it.
//
// Type is rpc.AnswerTypeDocument when the walk ended within the threshold and the
// answer is one document, and rpc.AnswerTypeMap or rpc.AnswerTypeTable when
// it passed the threshold. Count and Faults are the records the CHAIN produced,
// not the records the command did: `| count` answers one record whatever it
// counted, and the operator's terminator must state what the operator received.
type RecordAnswer struct {
	Type   string
	Count  uint64
	Faults uint64
}

// RenderRecords runs the pipe chain in input over rows and writes the rendering
// to w, one record at a time where the chain allows it.
//
// key and fields are the envelope and the column schema the answer declares.
// sessionFormat is the caller's `set cli format` override, empty for none; the
// configured default (ze.cli.format) applies when the chain names no format,
// exactly as it does for a whole payload.
//
// The returned RecordAnswer describes the answer for a caller that frames it.
// An error means the rendering did not reach w whole: the chain was refused
// before any record was pulled, or a write failed part way and the walk stopped
// because every later record would be produced for nobody.
func RenderRecords(w io.Writer, input, sessionFormat, key string, fields []string, rows iter.Seq[rpc.Record]) (RecordAnswer, error) {
	command, ops := parsePipeChain(input)
	columns := ColumnsForCommand(command)
	_, chain, meta := foldFilters(command, ops)
	if msg := ValidatePipes(chain); msg != "" {
		return RecordAnswer{}, errors.New(msg)
	}
	answered := chainAnswersItsOwnDocument(chain)
	ops = renderOps(chain, sessionFormat)

	// A chain that answers a document of its own has replaced the command's
	// rows, so the command's column schema describes nothing the answer still
	// carries. `system command list | count` answers {"count":N}, which is not
	// a positional row and would be refused as one by the collapse that zips
	// them (rpc.CollapseRecords). The schema is dropped once, here, so the
	// threshold, the item type and the collapse all read the same answer.
	if answered {
		fields = nil
	}

	// The chain runs over the records BEFORE the threshold is measured, so the
	// threshold is measured over what the operator receives. `| first 10` and
	// `| count` are therefore bounded answers however long the walk behind them
	// is, which is what makes them cost ten rows and one integer.
	records := ApplyPipesRecords(input, rows)

	answer := RecordAnswer{Type: rpc.AnswerTypeDocument}
	streamed := streamsPerRecord(ops, fields, meta)

	// held is the window the answer is decided in. It grows to the threshold
	// and stops there when the rendering can stream, and it grows to the whole
	// answer when the rendering needs a document.
	var held []rpc.Record
	writing := false
	for record := range records {
		switch {
		case len(record.Fault) > 0:
			answer.Faults++
		case len(record.Item) > 0:
			answer.Count++
		default:
			return answer, errEmptyRenderedRecord
		}

		if writing {
			if err := writeRecordJSON(w, record); err != nil {
				return answer, err
			}
			continue
		}

		held = append(held, record)
		if len(held) <= rpc.AnswerBufferThreshold {
			continue
		}

		// One record past the threshold, so this answer is not one document.
		// The records already held go out first, in the order the walk
		// produced them, and the rest follow as they arrive.
		answer.Type = rpc.AnswerStreamType(fields)
		if !streamed {
			continue
		}
		for i := range held {
			if err := writeRecordJSON(w, held[i]); err != nil {
				return answer, err
			}
		}
		held = nil
		writing = true
	}

	if writing {
		return answer, nil
	}
	return answer, writeDocument(w, held, key, fields, answered, ops, meta, columns)
}

// chainAnswersItsOwnDocument reports whether the chain answers a document of
// its own rather than the rows of the command's answer.
//
// `| count` is the one operator that does. It answers {"count":N} whatever it
// counted, and the string path replaces the whole payload for the same reason
// (applyCount, pipe.go). Every other operator on the record path filters,
// shapes or decorates the rows it was given, and those rows still belong to the
// command that produced them.
func chainAnswersItsOwnDocument(ops []pipeOp) bool {
	for _, op := range ops {
		if op.kind == pipeCount {
			return true
		}
	}
	return false
}

// writeDocument collapses the records the walk held into the one document a
// buffered rendering needs, renders it through the format the chain named, and
// writes it.
func writeDocument(w io.Writer, held []rpc.Record, key string, fields []string, answered bool, ops []pipeOp, meta map[string]any, columns []ColumnOrder) error {
	document, err := answerDocument(held, key, fields, answered)
	if err != nil {
		return err
	}
	rendered, errMsg := ApplyPipes(string(document), ops, meta, columns)
	if errMsg != "" {
		return errors.New(errMsg)
	}
	if rendered == "" {
		return nil
	}
	if _, err := io.WriteString(w, rendered); err != nil {
		return err
	}
	if rendered[len(rendered)-1] == '\n' {
		return nil
	}
	_, err = io.WriteString(w, "\n")
	return err
}

// answerDocument is the one document the held records collapse to.
//
// A chain that filtered, shaped or decorated the command's rows still answers
// rows, so they collapse under the answer's envelope. That collapse is
// rpc.CollapseRecords, which is what a surface reading the whole answer as
// one string gets from the same records (Records.MarshalJSON,
// internal/component/plugin/types.go), and one collapse for both is what keeps
// `| raw` answering the dispatcher's JSON byte for byte.
//
// A chain that answered for itself has already produced the whole document in
// one record, and that record IS the answer. Filing it under the envelope would
// put `{"count":N}` beneath a key naming the rows it counted, so
// `system command list | count` would answer one shape on the exec channel and
// another on every surface that reads the payload whole.
//
// A rejected row alongside it keeps the collapse. The envelope is then wrong
// about one record and right about the rejected ones, which is the lesser of
// the two, because a fault an operator never sees is one they act as if had not
// happened. Nothing on this channel produces a fault today.
func answerDocument(held []rpc.Record, key string, fields []string, answered bool) (json.RawMessage, error) {
	if answered && len(held) == 1 && len(held[0].Item) > 0 {
		return held[0].Item, nil
	}
	return rpc.CollapseRecords(key, fields, slices.Values(held))
}

// writeRecordJSON writes one record as one line of newline-delimited JSON.
//
// The payload is decoded and re-encoded rather than copied, because that is
// what `| ndjson` does to every element of a document (marshalNDJSON, pipe.go).
// A copy would keep the producer's key order and the string path's would be
// alphabetical, so the same answer would read differently depending on how many
// records it turned out to hold.
//
// A rejected row is written as its own line. A stream has no document to group
// them under, which is where a buffered rendering puts them, and the
// terminator's fault count states how many there were either way.
func writeRecordJSON(w io.Writer, record rpc.Record) error {
	payload := record.Item
	if len(record.Fault) > 0 {
		payload = record.Fault
	}
	var value any
	if err := json.Unmarshal(payload, &value); err != nil {
		return err
	}
	line, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if _, err := w.Write(append(line, '\n')); err != nil {
		return err
	}
	return nil
}

// streamsPerRecord reports whether the chain can be rendered one record at a
// time. Three properties must hold together, and each one is a shape a stream
// has no room for.
//
// The format must be `| ndjson`, the one operator that names a line for each
// value rather than one document for the answer.
//
// The answer must carry self-describing rows. A positional row is read against
// the head's column names, and the zip that turns it back into an object belongs to
// the collapse, so an answer that declares columns renders through it.
//
// The chain must have folded no display metadata. Metadata rides in the
// envelope beside the rows (injectPipeMeta, pipe.go), and a stream has no
// envelope, so `| first 10 | ndjson` renders its document rather than dropping
// the `first` the operator asked to be told about.
func streamsPerRecord(ops []pipeOp, fields []string, meta map[string]any) bool {
	if len(fields) > 0 || len(meta) > 0 {
		return false
	}
	for _, op := range ops {
		if op.kind == pipeNDJSON {
			return true
		}
	}
	return false
}

// renderOps returns the operators the RENDERING still owes once the records
// have passed through the chain, with the configured format appended when the
// chain named none.
//
// The data-shaping operators are dropped, because ApplyPipesRecords has already
// applied each of them to every record. Running `| count` twice would count the
// count, and running `| first 10` twice would keep the first ten of ten. What is
// left is the format, plus the two operators a renderer reads rather than
// applies: `| display` names the columns and `| fill` names the rest of them,
// and both are read out of the chain by columnsInChain.
func renderOps(ops []pipeOp, sessionFormat string) []pipeOp {
	kept := make([]pipeOp, 0, len(ops)+1)
	for _, op := range ops {
		if isFormatOp(op.kind) || op.kind == pipeDisplay || op.kind == pipeFill {
			kept = append(kept, op)
		}
	}
	if !hasFormatOp(kept) {
		kept = append(kept, pipeOp{kind: configuredDefault(sessionFormat)})
	}
	return kept
}

// errEmptyRenderedRecord is what a record carrying neither a result nor a
// rejection earns. rpc.Record sets exactly one of the two, and the encoder
// refuses the same shape (rpc.ErrEmptyAnswerRecord);
// rendering it would put an empty line in front of the operator and count a row
// that carries nothing.
var errEmptyRenderedRecord = errors.New("answer record carries neither an item nor a fault")
