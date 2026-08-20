// Design: docs/architecture/api/commands.md — CLI pipe operators
// Overview: pipe.go — the same operator chain applied to one whole payload
// Related: pipe_columns.go — the per-record field selection `| display` asks for
//
// pipe_records.go runs a pipe chain over the records of a streamed answer, one
// record at a time. It is the record half of pipe.go: the same chain, the same
// operator names, and the same folded filters, reading a sequence instead of a
// string.
//
// The memory shape is the reason this half exists. An operator that reads a
// record and forgets it costs one record whatever the answer holds, `| last N`
// costs the N records it was asked for, and `| table` costs the whole answer
// because a column width is measured over every row. ChainBuffersRecords is
// what a consumer asks before it decides to render as it reads.
//
// Cancellation falls out of the shape. Ranging the returned sequence pulls from
// the records the daemon produces, so a consumer that stops ranging stops the
// walk. That is what makes `show bgp rib | first 10` cost ten rows rather than
// a whole RIB.

package command

import (
	"bytes"
	"encoding/json"
	"iter"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// ApplyPipesRecords runs the pipe chain in input over records pulled one at a
// time, and returns the records the chain leaves.
//
// The chain is read the way ApplyPipes reads it: parsePipeChain expands the
// aliases, and foldFilters drops the operators the command already applied at
// the source of the data. A chain ValidatePipes refuses answers one fault
// record and pulls nothing, so an unreadable chain produces no records rather
// than the records of a chain nobody agreed to.
//
// A format operator changes no record. What `| json`, `| ndjson`, `| yaml`,
// `| table` and `| text` decide is how a consumer renders the records it is
// handed, and that consumer is the SSH exec channel rather than this function.
func ApplyPipesRecords(input string, records iter.Seq[rpc.Record]) iter.Seq[rpc.Record] {
	command, ops := parsePipeChain(input)
	_, ops, _ = foldFilters(command, ops)
	if msg := ValidatePipes(ops); msg != "" {
		return faultRecords(msg)
	}

	request := columnsInChain(ops)
	for _, op := range ops {
		records = applyRecordOp(records, op, request)
	}
	return records
}

// applyRecordOp wraps records in the operator op asks for.
//
// The wrapper pulls from the sequence it was given, so a chain is these
// wrappers nested in chain order and the innermost pull reaches the daemon's
// generator.
func applyRecordOp(records iter.Seq[rpc.Record], op pipeOp, request columnRequest) iter.Seq[rpc.Record] {
	switch op.kind {
	case pipeMatch:
		return recordsMatching(records, op.arg)
	case pipeCount:
		return recordsCounted(records)
	case pipeFirst:
		return recordsFirst(records, op.arg)
	case pipeLast:
		return recordsLast(records, op.arg)
	case pipeDisplay:
		return recordsSelected(records, request)
	case pipeResolve:
		return recordsTransformed(records, resolveJSON)
	case pipeOrigin:
		return recordsTransformed(records, originJSON)
	case pipeTable, pipeText:
		// The two operators that cannot answer from the record in hand, and the
		// one limit R-6 of spec-streaming-answer-protocol accepts: a
		// column is as wide as its widest cell, so the header line depends on
		// the last row. The buffering that costs stays where the measuring
		// happens, in applyTableStyled, and the records reach it unchanged and
		// one at a time. Collecting them here would spend the same memory one
		// stage earlier and hide where it goes.
		return records
	default:
		// Every remaining kind leaves the data alone: the five formats, raw,
		// fill, log and no-more. The default arm is what keeps that true for a
		// kind nobody thought of here, and passing the records through is the
		// answer a reader of the chain expects from an operator that shapes
		// nothing.
		return records
	}
}

// recordsMatching keeps the records whose payload contains pattern, compared
// without case as `| match` compares a line.
//
// A fault carries text an operator reads too, so the comparison is over
// whichever of the two a record carries rather than over its result alone.
func recordsMatching(records iter.Seq[rpc.Record], pattern string) iter.Seq[rpc.Record] {
	lower := strings.ToLower(pattern)
	return func(yield func(rpc.Record) bool) {
		for record := range records {
			if !strings.Contains(strings.ToLower(string(recordPayload(record))), lower) {
				continue
			}
			if !yield(record) {
				return
			}
		}
	}
}

// recordsCounted answers one record carrying the number of results, in the
// {"count":N} spelling applyCount writes.
//
// The walk runs to its end and holds no record: the count is one integer,
// whatever the answer's size. A fault is forwarded rather than counted, because
// the terminator counts results and faults apart and `| count` must not answer
// a number that disagrees with it.
func recordsCounted(records iter.Seq[rpc.Record]) iter.Seq[rpc.Record] {
	return func(yield func(rpc.Record) bool) {
		results := 0
		for record := range records {
			if len(record.Fault) > 0 {
				if !yield(record) {
					return
				}
				continue
			}
			results++
		}
		counted := textbuf.StrIntStr(`{"count":`, int64(results), "}")
		yield(rpc.Record{Item: json.RawMessage(counted)})
	}
}

// recordsFirst answers the first n results and then stops the walk.
//
// Stopping is the point: returning from the range makes the yield that produced
// the record report false, and the daemon's generator ends there rather than
// walking rows nobody reads.
//
// A fault is forwarded and is not one of the n, so `| first 10` answers ten
// results whether or not a row was rejected on the way. ValidatePipes refuses a
// chain whose argument is not a positive number, and an argument that reaches
// here anyway is ignored, which is the tolerance applyFirst carries.
func recordsFirst(records iter.Seq[rpc.Record], arg string) iter.Seq[rpc.Record] {
	n, err := strconv.Atoi(arg)
	if err != nil || n <= 0 {
		return records
	}
	return func(yield func(rpc.Record) bool) {
		taken := 0
		for record := range records {
			if !yield(record) {
				return
			}
			if len(record.Fault) > 0 {
				continue
			}
			taken++
			if taken >= n {
				return
			}
		}
	}
}

// recordsLast answers the last n results.
//
// The walk has to reach the end before the answer is known, so this operator
// costs the n records it was asked for and nothing more: the ring holds n
// results and overwrites the oldest. A fault is forwarded as it arrives, so an
// operator sees every rejected row rather than the n that happen to fall at the
// end.
//
// The ring grows to the records that arrive rather than to n, so `| last
// 1000000000` over an answer of ten rows costs ten. n is an operator's word
// about an answer they have not seen, and a bound taken from it is a bound the
// operator sets.
func recordsLast(records iter.Seq[rpc.Record], arg string) iter.Seq[rpc.Record] {
	n, err := strconv.Atoi(arg)
	if err != nil || n <= 0 {
		return records
	}
	return func(yield func(rpc.Record) bool) {
		var ring []rpc.Record
		oldest := 0
		for record := range records {
			if len(record.Fault) > 0 {
				if !yield(record) {
					return
				}
				continue
			}
			if len(ring) < n {
				ring = append(ring, record)
				continue
			}
			ring[oldest] = record
			oldest++
			if oldest == n {
				oldest = 0
			}
		}
		// oldest is still zero for a ring that never filled, so one loop reads
		// both cases in arrival order.
		for i := range ring {
			if !yield(ring[(oldest+i)%len(ring)]) {
				return
			}
		}
	}
}

// recordsSelected keeps the fields `| display` named, one record at a time.
//
// Selection needs no record but the one in hand, which is what makes `| display`
// stream where `| table` cannot. A record that does not decode is forwarded
// whole, as a payload that is not JSON passes through the string path.
func recordsSelected(records iter.Seq[rpc.Record], request columnRequest) iter.Seq[rpc.Record] {
	if !request.selects() {
		return records
	}
	keep := keepFields(request.display)
	return func(yield func(rpc.Record) bool) {
		for record := range records {
			if len(record.Item) > 0 {
				record.Item = selectItem(record.Item, keep)
			}
			if !yield(record) {
				return
			}
		}
	}
}

// selectItem keeps the named fields of one result, and answers the result
// unchanged when it does not decode.
//
// Numbers are decoded as json.Number so the digits the dispatcher wrote survive
// the round trip, which is the rule applyDisplaySelect follows on the whole
// payload.
func selectItem(item json.RawMessage, keep map[string]struct{}) json.RawMessage {
	decoder := json.NewDecoder(bytes.NewReader(item))
	decoder.UseNumber()
	var element any
	if err := decoder.Decode(&element); err != nil {
		return item
	}
	selected, err := json.Marshal(selectElement(element, keep))
	if err != nil {
		return item
	}
	return selected
}

// recordsTransformed rewrites each result with transform, which is how
// `| resolve` and `| origin` decorate the values of one record.
//
// Each lookup is per record and the transforms hold their own caches, so the
// decoration costs the record in hand. A record that does not decode, or that
// does not re-marshal, is forwarded whole.
func recordsTransformed(records iter.Seq[rpc.Record], transform func(any) any) iter.Seq[rpc.Record] {
	return func(yield func(rpc.Record) bool) {
		for record := range records {
			if len(record.Item) > 0 {
				record.Item = transformItem(record.Item, transform)
			}
			if !yield(record) {
				return
			}
		}
	}
}

// transformItem applies transform to one result, decoding it the way
// applyJSONTransform decodes a whole payload so the two answer alike.
func transformItem(item json.RawMessage, transform func(any) any) json.RawMessage {
	var element any
	if err := json.Unmarshal(item, &element); err != nil {
		return item
	}
	transformed, err := json.Marshal(transform(element))
	if err != nil {
		return item
	}
	return transformed
}

// faultRecords answers one rejected record carrying message, and pulls nothing.
//
// It is what a chain the validator refuses produces. The message reaches the
// operator as the answer's one fault, rather than as an empty answer they
// cannot tell from a command that found no data.
func faultRecords(message string) iter.Seq[rpc.Record] {
	fault, err := json.Marshal(struct {
		Message string `json:"message"`
	}{Message: message})
	if err != nil {
		// json.Marshal of a struct of one string cannot fail. Answering no
		// record is still the safe reading of a chain nobody could validate.
		return func(func(rpc.Record) bool) {}
	}
	return func(yield func(rpc.Record) bool) {
		yield(rpc.Record{Fault: fault})
	}
}

// recordPayload answers the bytes a record carries, whichever of the two it is.
func recordPayload(record rpc.Record) json.RawMessage {
	if len(record.Item) > 0 {
		return record.Item
	}
	return record.Fault
}
