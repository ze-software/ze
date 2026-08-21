// Design: docs/architecture/api/ipc_protocol.md -- the answer wire grammar
// Related: answer_row.go -- the positional row this collapse zips
//          types.go -- Answer, the arriving sequence a consumer collapses
//
// collapse.go turns the records of one answer into the single document a
// buffered consumer reads.
//
// It lives here rather than beside either consumer because both ends of the
// plugin connection need it and neither owns it. The wire writer collapses the
// rows it held when a walk ended inside AnswerBufferThreshold
// (WriteRecordAnswer, answer_write.go). The engine renders the same document
// for a surface that takes the whole answer as one string
// (Records.MarshalJSON, internal/component/plugin/types.go) and so does the
// SDK for a transport whose result is one value (Records.MarshalJSON,
// pkg/plugin/records.go). And the SDK collapses the records that arrive when a
// plugin asks for an answer as a value rather than as a walk
// (Plugin.DispatchCommand, pkg/plugin/sdk/sdk_engine.go).
//
// One function serves all of them, so a document is the same document whichever
// side of the connection built it.

package rpc

import (
	"encoding/json"
	"errors"
	"fmt"
	"iter"
)

// The two envelope names a buffered answer uses beside the handler's own.
//
// AnswerErrorsKey holds the rejected rows, as a sibling of the rows the command
// produced rather than a member of them: the terminator counts the two
// separately, so mixing them would make `| count` answer one number on the
// record path and another on the buffered one. It is written only when a row
// was rejected, so an answer that rejected none has the shape it always had and
// no consumer sees a new key.
//
// AnswerDefaultKey is where the rows go when the answer names no envelope AND a
// row was rejected. A bare array has nowhere to carry a sibling, so the rows
// move under a key rather than the rejected ones being dropped.
//
// A producer MUST NOT name its envelope AnswerErrorsKey. Every producer refuses
// it (ErrReservedEnvelopeKey), because the two collections would otherwise land
// under one key and one would overwrite the other.
const (
	AnswerErrorsKey  = "errors"
	AnswerDefaultKey = "data"
)

// ErrEmptyAnswerRecord is what a row carrying neither an item nor a fault
// earns, on the record path and on the buffered one alike. Record sets exactly
// one of the two; a row that sets neither reaches the wire as `item=` with no
// value, which no consumer can decode, and reaches a buffered consumer as null,
// which reads like a row the command produced. Refusing it names the producer
// rather than handing either consumer an empty-looking answer
// (`ai/rules/evidence.md`).
var ErrEmptyAnswerRecord = errors.New("answer record carries neither an item nor a fault")

// ErrReservedEnvelopeKey is what an answer naming its envelope AnswerErrorsKey
// earns, on the record path and on the buffered one alike. A buffered answer
// carries the rejected rows under that name beside the rows the command
// produced, so an envelope of the same name would collapse the two collections
// into one and lose whichever was written second.
var ErrReservedEnvelopeKey = fmt.Errorf("answer envelope key %q is reserved for the rejected rows", AnswerErrorsKey)

// CollapseRecords renders the rows of one answer as the single document a
// buffered consumer reads: the rows the command produced, in walk order, under
// key, and the rows it rejected under AnswerErrorsKey beside them. With no
// rejected row it is key over the items, or a bare array when key is empty,
// which is the shape a buffered surface has always seen.
//
// fields is the column schema an AnswerTypeStream answer declares on its head.
// When it is set, each item is read as a positional array and zipped against
// the names, so the document holds the same objects an AnswerTypeNDJSON answer
// would have carried. An answer that declares no fields carries self-describing
// rows, which pass through unchanged.
//
// The two collections stay separate, which is what keeps `| count` honest:
// count= counts the rows produced and faults= counts the rows rejected, and
// nothing lands in the item array that the terminator did not count.
//
// This walks the whole answer into memory, which is what a buffered rendering
// is. A caller that must bound the memory reads the records one at a time
// instead. rows is walked once, so one answer takes one path, never both.
func CollapseRecords(key string, fields []string, rows iter.Seq[Record]) ([]byte, error) {
	if key == AnswerErrorsKey {
		return nil, ErrReservedEnvelopeKey
	}
	names, err := quoteFields(fields)
	if err != nil {
		return nil, err
	}

	// A nil slice marshals to null, and a command that produced no rows
	// produced an empty collection. The record path says the same thing with
	// count=0, so the two must not disagree here. faults stays nil until a row
	// is rejected, because nil is what states that the sibling key is absent.
	items := []json.RawMessage{}
	var faults []json.RawMessage

	// row is the scratch the positional rows decode into. It is reused for
	// every row, so a walk of a million rows decodes into one slice.
	var row []json.RawMessage
	for record := range rows {
		switch {
		case len(record.Fault) > 0:
			faults = append(faults, record.Fault)
		case len(record.Item) > 0:
			item := record.Item
			if names != nil {
				if err = json.Unmarshal(item, &row); err != nil {
					return nil, fmt.Errorf("%w: %w", ErrRowNotPositional, err)
				}
				if item, err = zipRow(names, row); err != nil {
					return nil, err
				}
			}
			items = append(items, item)
		default:
			return nil, ErrEmptyAnswerRecord
		}
	}

	if faults == nil {
		if key == "" {
			return json.Marshal(items)
		}
		return json.Marshal(map[string][]json.RawMessage{key: items})
	}

	if key == "" {
		key = AnswerDefaultKey
	}
	return json.Marshal(map[string][]json.RawMessage{key: items, AnswerErrorsKey: faults})
}

// CollapseAnswer walks answer to its end and returns the one document its
// records carry.
//
// An answer of AnswerTypeJSON was collapsed by its producer: the walk ended
// within AnswerBufferThreshold records, so the answer is one document in one
// record, and a command that answered with no data at all carries none. Taking
// that record as it stands is what makes the value byte-identical to the value
// the same command produced before the record frame existed (AC-3 and AC-5 of
// spec-record-answers-1-sdk-path).
//
// Any other type is a walk that passed the threshold, and its records collapse
// through CollapseRecords, the same function the producer would have used had
// the walk been short enough. So one command answers one document whichever
// side of the threshold it lands on.
//
// A record line is a peer's bytes and the parser forwards its item= unread, so
// the document is checked here before it is handed to a caller that will treat
// it as JSON. The collapse checks the same thing for a streamed answer, where
// re-encoding each item is what refuses one that is not JSON.
//
// It is the buffered reading BOTH ends of the connection make. The SDK reads an
// engine answer as one value (answerValue, pkg/plugin/sdk/sdk_engine.go) and
// the engine reads a plugin's execute-command answer as one value
// (PluginConn.SendExecuteCommand, internal/component/plugin/ipc/rpc.go). A
// caller that must bound its memory reads Answer.Records itself.
//
// The caller MUST read Answer.Err and Answer.Message after this returns: the
// walk this runs is what fills them.
func CollapseAnswer(answer *Answer) (json.RawMessage, error) {
	if answer.Type != AnswerTypeJSON {
		return CollapseRecords(answer.Key, answer.Fields, answer.Records)
	}

	var document json.RawMessage
	records := 0
	faults := 0
	for record := range answer.Records {
		records++
		if len(record.Fault) > 0 {
			faults++
			continue
		}
		document = record.Item
	}

	switch {
	case records > 1:
		return nil, fmt.Errorf("answer states type=%s and carries %d records, want at most one", AnswerTypeJSON, records)
	case faults > 0:
		// The type says the whole answer is one document, and a document has
		// nowhere to carry a rejected row beside itself. A producer that sends
		// one contradicts its own head.
		return nil, fmt.Errorf("answer states type=%s and carries a rejected row", AnswerTypeJSON)
	case len(document) > 0 && !json.Valid(document):
		return nil, fmt.Errorf("answer document is not JSON: %d bytes starting %.32q", len(document), string(document))
	}
	return document, nil
}
