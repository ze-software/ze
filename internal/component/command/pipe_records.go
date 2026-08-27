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
// record and forgets it costs one record whatever the answer holds, and
// `| last N` costs at most the declared record and byte ceilings. RenderRecords
// keeps this path when transforms precede rendering and when NDJSON precedes a
// line transform. Other format-before-transform chains use a collapsed document.
// ChainBuffersRecords is what a consumer asks before it decides to render as it
// reads.
//
// Cancellation falls out of the shape. Ranging the returned sequence pulls from
// the records the daemon produces, so a consumer that stops ranging stops the
// walk. That is what makes `show bgp rib | first 10` cost ten rows rather than
// a whole RIB.

package command

import (
	"bytes"
	"encoding/json"
	"errors"
	"iter"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// recordsLastLimit reuses the answer encoder's bounded-document window, and
// recordsLastBytesLimit reuses the wire's one-message byte ceiling. The count is
// rejected before a sequence is built, the byte budget is enforced as records
// arrive, and the ring grows lazily rather than allocating either bound eagerly.
const (
	recordsLastLimit      = rpc.AnswerBufferThreshold
	recordsLastBytesLimit = rpc.MaxMessageSize
)

// applyPipesRecords runs the pipe chain in input over records pulled one at a
// time. It also returns the positional field schema the chain leaves, because
// display must narrow names and values together before collapse.
//
// The chain is read the way ApplyPipes reads it: parsePipeChain expands the
// aliases, and foldFilters drops the operators the command already applied at
// the source of the data. A chain ValidatePipes refuses answers one fault
// record and pulls nothing. A positional display that names no schema field
// returns a message before pulling a record.
//
// NDJSON turns positional rows into field-named objects when the chain reaches
// it. Other format operators change no record when they follow data transforms.
// RenderRecords runs most format-before-transform chains over one collapsed
// document. NDJSON plus line transforms deliberately stays on this path.
func applyPipesRecords(
	input string,
	fields []string,
	records iter.Seq[rpc.Record],
) (iter.Seq[rpc.Record], []string, string) {
	command, ops := parsePipeChain(input)
	_, ops, _ = foldFilters(command, ops)
	if msg := ValidatePipes(ops); msg != "" {
		return faultRecords(msg), fields, ""
	}
	if msg := validateDeclaredShape(command, ops); msg != "" {
		return faultRecords(msg), fields, ""
	}

	request := columnsInChain(ops)
	ndjsonRendered := false
	for _, op := range ops {
		if op.kind == pipeNDJSON {
			records = recordsNDJSONRendered(records, fields)
			fields = nil
			ndjsonRendered = true
			continue
		}
		if ndjsonRendered {
			switch op.kind { //nolint:exhaustive // only operators whose NDJSON line semantics differ.
			case pipeMatch:
				records = recordsMatchingRenderedJSON(records, op.arg)
				continue
			case pipeCount:
				records = recordsLinesCounted(records)
				continue
			case pipeFirst:
				records = recordsLinesFirst(records, op.arg)
				continue
			case pipeLast:
				records = recordsLinesLast(records, op.arg)
				continue
			}
		}
		if op.kind == pipeCount {
			records = recordsCounted(records)
			fields = nil
			continue
		}
		if op.kind == pipeDisplay {
			var msg string
			records, fields, msg = recordsSelected(records, request, fields)
			if msg != "" {
				return nil, fields, msg
			}
			continue
		}
		if len(fields) > 0 {
			if isAddressOp(op.kind) {
				records, fields = recordsPositionalAddressTransformed(records, fields, op)
				continue
			}
		}
		records = applyRecordOp(records, op)
	}
	return records, fields, ""
}

// applyRecordOp wraps records in the operator op asks for.
//
// The wrapper pulls from the sequence it was given, so a chain is these
// wrappers nested in chain order and the innermost pull reaches the daemon's
// generator.
func applyRecordOp(records iter.Seq[rpc.Record], op pipeOp) iter.Seq[rpc.Record] {
	switch op.kind {
	case pipeMatch:
		return recordsMatching(records, op.arg)
	case pipeFirst:
		return recordsFirst(records, op.arg)
	case pipeLast:
		return recordsLast(records, op.arg)
	case pipeDisplay:
		return records
	case pipeResolve:
		return recordsTransformed(records, func(v any) any {
			return resolveJSON(v, op.addressFields, op.allAddressFields)
		})
	case pipeOrigin:
		return recordsTransformed(records, func(v any) any {
			return originJSON(v, op.addressFields, op.allAddressFields)
		})
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

// recordsMatchingRenderedJSON compares against the canonical line `| ndjson`
// produced rather than the producer's original JSON spelling. Faults always
// pass through: filtering results must not hide a row the pipeline rejected.
func recordsMatchingRenderedJSON(records iter.Seq[rpc.Record], pattern string) iter.Seq[rpc.Record] {
	lower := strings.ToLower(pattern)
	return func(yield func(rpc.Record) bool) {
		for record := range records {
			if len(record.Fault) > 0 {
				if !yield(record) {
					return
				}
				continue
			}
			if !strings.Contains(strings.ToLower(string(recordPayload(record))), lower) {
				continue
			}
			if !yield(record) {
				return
			}
		}
	}
}

// recordsNDJSONRendered canonicalizes each record when the NDJSON operator is
// reached. Positional items become field-named objects here, so every later
// line transform reads and retains the exact bytes the renderer will write.
func recordsNDJSONRendered(records iter.Seq[rpc.Record], fields []string) iter.Seq[rpc.Record] {
	return func(yield func(rpc.Record) bool) {
		for record := range records {
			if len(record.Item) == 0 && len(record.Fault) == 0 {
				if !yield(record) {
					return
				}
				continue
			}
			rendered, err := marshalRecordJSONFields(record, fields)
			if err != nil {
				var tb textbuf.Buffer
				message := "NDJSON cannot render record: "
				if len(fields) > 0 && len(record.Item) > 0 {
					message = "NDJSON cannot render positional row: "
				}
				yield(pipeFault(tb.Str(message).Err(err).String()))
				return
			}
			if len(record.Fault) > 0 {
				record.Fault = rendered
			} else {
				record.Item = rendered
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
	return recordsCountedByLine(records, false)
}

func recordsLinesCounted(records iter.Seq[rpc.Record]) iter.Seq[rpc.Record] {
	return recordsCountedByLine(records, true)
}

func recordsCountedByLine(records iter.Seq[rpc.Record], faultsAreLines bool) iter.Seq[rpc.Record] {
	return func(yield func(rpc.Record) bool) {
		results := 0
		for record := range records {
			if len(record.Fault) > 0 && !faultsAreLines {
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
	return recordsFirstByLine(records, arg, false)
}

func recordsLinesFirst(records iter.Seq[rpc.Record], arg string) iter.Seq[rpc.Record] {
	return recordsFirstByLine(records, arg, true)
}

func recordsFirstByLine(records iter.Seq[rpc.Record], arg string, faultsAreLines bool) iter.Seq[rpc.Record] {
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
			if len(record.Fault) > 0 && !faultsAreLines {
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
// ValidatePipes refuses n above recordsLastLimit before this sequence is built.
// The ring grows only as records arrive, so an allowed request over a short
// answer allocates only for that short answer rather than n slots eagerly.
func recordsLast(records iter.Seq[rpc.Record], arg string) iter.Seq[rpc.Record] {
	return recordsLastByLine(records, arg, false)
}

func recordsLinesLast(records iter.Seq[rpc.Record], arg string) iter.Seq[rpc.Record] {
	return recordsLastByLine(records, arg, true)
}

func recordsLastByLine(records iter.Seq[rpc.Record], arg string, faultsAreLines bool) iter.Seq[rpc.Record] {
	n, err := strconv.Atoi(arg)
	if err != nil {
		return records
	}
	if n <= 0 {
		return records
	}
	if n > recordsLastLimit {
		return faultRecords(lastRetentionCountError(arg))
	}
	return func(yield func(rpc.Record) bool) {
		var ring []rpc.Record
		oldest := 0
		retainedBytes := 0
		for record := range records {
			if len(record.Fault) > 0 && !faultsAreLines {
				if !yield(record) {
					return
				}
				continue
			}
			recordBytes := len(recordPayload(record))
			if len(ring) < n {
				if retainedBytes+recordBytes > recordsLastBytesLimit {
					yield(pipeFault(lastRetentionBytesError()))
					return
				}
				ring = append(ring, record)
				retainedBytes += recordBytes
				continue
			}
			nextBytes := retainedBytes - len(recordPayload(ring[oldest])) + recordBytes
			if nextBytes > recordsLastBytesLimit {
				yield(pipeFault(lastRetentionBytesError()))
				return
			}
			ring[oldest] = record
			retainedBytes = nextBytes
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

func lastRetentionCountError(arg string) string {
	var tb textbuf.Buffer
	return tb.Str("last accepts at most ").Int(int64(recordsLastLimit)).
		Str(" rows, got ").Str(arg).String()
}

func lastRetentionBytesError() string {
	var tb textbuf.Buffer
	return tb.Str("last retention exceeds the ").Int(recordsLastBytesLimit).
		Str("-byte command answer limit; request fewer rows").String()
}

// recordsSelected keeps the fields `| display` named, one record at a time.
//
// Self-describing map rows carry their names in each item. Positional rows carry
// values only, so fields supplies the head schema and is narrowed with the
// values. A positional typo is refused before the walk begins.
func recordsSelected(
	records iter.Seq[rpc.Record],
	request columnRequest,
	fields []string,
) (iter.Seq[rpc.Record], []string, string) {
	if !request.selects() {
		return records, fields, ""
	}
	if len(fields) > 0 {
		indices, selectedFields := positionalDisplayFields(fields, request.display)
		if len(selectedFields) == 0 {
			return nil, fields, displayNoFieldError(request.display)
		}
		return recordsPositionalSelected(records, indices, len(fields)), selectedFields, ""
	}

	keep := keepFields(request.display)
	return func(yield func(rpc.Record) bool) {
		for record := range records {
			if len(record.Item) > 0 {
				var matched bool
				record.Item, matched = selectItem(record.Item, keep)
				if !matched {
					yield(pipeFault(displayNoFieldError(request.display)))
					return
				}
			}
			if !yield(record) {
				return
			}
		}
	}, fields, ""
}

func positionalDisplayFields(fields []string, display ColumnOrder) ([]int, []string) {
	indices := make([]int, 0, len(display))
	selected := make([]string, 0, len(display))
	seen := make([]bool, len(fields))
	for _, name := range display {
		for index, field := range fields {
			if !strings.EqualFold(name, field) {
				continue
			}
			if seen[index] {
				break
			}
			seen[index] = true
			indices = append(indices, index)
			selected = append(selected, field)
			break
		}
	}
	return indices, selected
}

func recordsPositionalSelected(
	records iter.Seq[rpc.Record],
	indices []int,
	fieldCount int,
) iter.Seq[rpc.Record] {
	return func(yield func(rpc.Record) bool) {
		for record := range records {
			if len(record.Item) > 0 {
				selected, msg := selectPositionalItem(record.Item, indices, fieldCount)
				if msg != "" {
					yield(pipeFault(msg))
					return
				}
				record.Item = selected
			}
			if !yield(record) {
				return
			}
		}
	}
}

func selectPositionalItem(item json.RawMessage, indices []int, fieldCount int) (json.RawMessage, string) {
	decoder := json.NewDecoder(bytes.NewReader(item))
	decoder.UseNumber()
	var values []any
	if err := decoder.Decode(&values); err != nil {
		return nil, "display cannot select a positional row: the row is not a JSON array"
	}
	if len(values) != fieldCount {
		var tb textbuf.Buffer
		return nil, tb.Str("display cannot select a positional row: row has ").
			Int(int64(len(values))).Str(" values, schema has ").
			Int(int64(fieldCount)).String()
	}
	selected := make([]any, 0, len(indices))
	for _, index := range indices {
		selected = append(selected, values[index])
	}
	encoded, err := json.Marshal(selected)
	if err != nil {
		return nil, "display cannot select a positional row: selected values cannot be encoded"
	}
	return encoded, ""
}

// selectItem keeps the named fields of one self-describing result and answers
// the result unchanged when it does not decode.
func selectItem(item json.RawMessage, keep map[string]struct{}) (json.RawMessage, bool) {
	decoder := json.NewDecoder(bytes.NewReader(item))
	decoder.UseNumber()
	var element any
	if err := decoder.Decode(&element); err != nil {
		return item, true
	}
	selected, matched := selectElement(element, keep)
	if !matched {
		return nil, false
	}
	encoded, err := json.Marshal(selected)
	if err != nil {
		return item, true
	}
	return encoded, true
}

// recordsPositionalAddressTransformed extends positional values and their head
// schema together. Existing derived columns are producer data and stay
// untouched. Every absent suffix contributes one field and its matching value,
// so lookup misses keep the same row width as hits.
func recordsPositionalAddressTransformed(
	records iter.Seq[rpc.Record],
	fields []string,
	op pipeOp,
) (iter.Seq[rpc.Record], []string) {
	transforms, derived := positionalAddressFields(fields, op)
	if len(transforms) == 0 {
		return records, fields
	}
	fieldCount := len(fields)
	fields = append(append([]string(nil), fields...), derived...)
	return func(yield func(rpc.Record) bool) {
		for record := range records {
			if len(record.Item) > 0 {
				transformed, msg := transformPositionalAddressItem(record.Item, transforms, fieldCount, op.kind)
				if msg != "" {
					yield(pipeFault(msg))
					return
				}
				record.Item = transformed
			}
			if !yield(record) {
				return
			}
		}
	}, fields
}

type positionalAddressTransform struct {
	sourceIndex  int
	valueIndices []int
}

func positionalAddressFields(fields []string, op pipeOp) ([]positionalAddressTransform, []string) {
	seen := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		seen[strings.ToLower(field)] = struct{}{}
	}

	var transforms []positionalAddressTransform
	var derived []string
	for index, field := range fields {
		if !addressFieldSelected(op.addressFields, field, op.allAddressFields) {
			continue
		}
		var suffixes []string
		switch op.kind { //nolint:exhaustive // recordsPositionalAddressTransformed passes only resolve or origin.
		case pipeResolve:
			suffixes = []string{"-name"}
		case pipeOrigin:
			suffixes = []string{"-asn", "-as-name", "-prefix"}
		}
		transform := positionalAddressTransform{sourceIndex: index}
		for valueIndex, suffix := range suffixes {
			name := derivedAddressField(field, suffix)
			folded := strings.ToLower(name)
			if _, exists := seen[folded]; exists {
				continue
			}
			seen[folded] = struct{}{}
			transform.valueIndices = append(transform.valueIndices, valueIndex)
			derived = append(derived, name)
		}
		if len(transform.valueIndices) > 0 {
			transforms = append(transforms, transform)
		}
	}
	return transforms, derived
}

func derivedAddressField(field, suffix string) string {
	var name textbuf.Buffer
	return name.Str(field).Str(suffix).String()
}

func transformPositionalAddressItem(
	item json.RawMessage,
	transforms []positionalAddressTransform,
	fieldCount int,
	kind pipeKind,
) (json.RawMessage, string) {
	decoder := json.NewDecoder(bytes.NewReader(item))
	decoder.UseNumber()
	var values []any
	if err := decoder.Decode(&values); err != nil {
		return nil, "address operator cannot transform a positional row: the row is not a JSON array"
	}
	if len(values) != fieldCount {
		var tb textbuf.Buffer
		return nil, tb.Str("address operator cannot transform a positional row: row has ").
			Int(int64(len(values))).Str(" values, schema has ").
			Int(int64(fieldCount)).String()
	}
	for _, transform := range transforms {
		address, _ := values[transform.sourceIndex].(string)
		switch kind { //nolint:exhaustive // positionalAddressFields produces transforms only for resolve or origin.
		case pipeResolve:
			name := ""
			if address != "*" {
				if isIPAddress(address) {
					name = ReverseLookup(address)
				}
			}
			values = append(values, name)
		case pipeOrigin:
			origin := OriginResult{}
			if address != "*" {
				if isIPAddress(address) {
					origin = LookupOrigin(address)
				}
			}
			derived := [...]any{origin.ASN, origin.Name, origin.Prefix}
			for _, valueIndex := range transform.valueIndices {
				values = append(values, derived[valueIndex])
			}
		}
	}
	encoded, err := json.Marshal(values)
	if err != nil {
		return nil, "address operator cannot transform a positional row: derived values cannot be encoded"
	}
	return encoded, ""
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

// recordsWithSingletonMetadata preserves the document path's observable
// metadata contract for NDJSON. A one-line result can carry metadata in that
// JSON object. Two or more lines cannot. The wrapper retains only the first
// record until it knows which shape it has.
func recordsWithSingletonMetadata(records iter.Seq[rpc.Record], meta pipeChainMeta) iter.Seq[rpc.Record] {
	return func(yield func(rpc.Record) bool) {
		var first rpc.Record
		seen := false
		emitted := false
		for record := range records {
			if !seen {
				first = record
				seen = true
				continue
			}
			if !emitted {
				if !yield(first) {
					return
				}
				emitted = true
			}
			if !yield(record) {
				return
			}
		}
		if seen && !emitted {
			yield(recordWithPipeMeta(first, meta))
		}
	}
}

func recordWithPipeMeta(record rpc.Record, meta pipeChainMeta) rpc.Record {
	rendered, msg := injectPipeMeta(string(recordPayload(record)), meta)
	if msg != "" {
		return pipeFault(msg)
	}
	if len(record.Item) > 0 {
		record.Item = json.RawMessage(rendered)
	} else {
		record.Fault = json.RawMessage(rendered)
	}
	return record
}

// faultRecords answers one rejected record carrying message, and pulls nothing.
//
// It is what a chain the validator refuses produces. The message reaches the
// operator as the answer's one fault, rather than as an empty answer they
// cannot tell from a command that found no data.
func faultRecords(message string) iter.Seq[rpc.Record] {
	fault := pipeFault(message)
	return func(yield func(rpc.Record) bool) {
		yield(fault)
	}
}

func pipeFault(message string) rpc.Record {
	fault, err := json.Marshal(struct {
		Message string `json:"message"`
	}{Message: message})
	if err != nil {
		return rpc.Record{Fault: json.RawMessage(`{"message":"pipe refused the chain"}`)}
	}
	return rpc.Record{Fault: fault}
}

// recordPayload answers the bytes a record carries, whichever of the two it is.
func recordPayload(record rpc.Record) json.RawMessage {
	if len(record.Item) > 0 {
		return record.Item
	}
	return record.Fault
}

func marshalRecordJSON(record rpc.Record) ([]byte, error) {
	var value any
	if err := json.Unmarshal(recordPayload(record), &value); err != nil {
		return nil, err
	}
	return json.Marshal(value)
}

func marshalRecordJSONFields(record rpc.Record, fields []string) ([]byte, error) {
	if len(fields) == 0 || len(record.Fault) > 0 {
		return marshalRecordJSON(record)
	}
	var values []any
	if err := json.Unmarshal(record.Item, &values); err != nil {
		return nil, err
	}
	if len(values) != len(fields) {
		return nil, errors.New("positional row and schema have different field counts")
	}
	object := make(map[string]any, len(fields))
	for index, field := range fields {
		object[field] = values[index]
	}
	return json.Marshal(object)
}
