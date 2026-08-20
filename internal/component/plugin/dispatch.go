// Design: docs/architecture/api/commands.md -- shared command dispatch contract

// Unified command-result envelope and dispatcher.
//
// This file holds the single, shared command surface every user-facing entry
// point (web, mcp, looking-glass, REST/gRPC, chaos, SSH) consumes: the
// CommandDispatcher func type, the CallerIdentity value it carries, and the one
// flatten helper that projects a typed *Response into the JSON string the text
// surfaces render. Before unification these were declared five times across
// component packages with incompatible shapes; they live here now because the
// plugin package is shared infrastructure that every surface may import, while
// internal/core (the bottom tier) may not hold a type that returns *Response.

package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// Authorizer is the request-carried policy decision used by the shared command
// dispatcher. AAA authorizers satisfy it without making this infrastructure
// package depend on the AAA component.
type Authorizer interface {
	Authorize(username, remoteAddr, command string, isReadOnly bool) bool
}

// CallerIdentity carries trusted caller metadata for a command request.
// Populated by the transport after authentication; it is not an auth
// credential. Surface names the originating transport for audit attribution;
// when empty the dispatcher constructor supplies a fixed per-surface default.
type CallerIdentity struct {
	Username   string
	RemoteAddr string
	Surface    string
	// ReadOnly means the transport admitted the caller with read-only access
	// only. Used by the API engine to deny writes from no-auth/read-only callers.
	ReadOnly bool
	// Authorizer is the policy generation accepted with this identity. Carrying
	// it with the request prevents a reload publication between authentication
	// and dispatch from authorizing the caller against a different generation.
	Authorizer Authorizer
}

type callerAuthorizerContextKey struct{}

// WithCallerAuthorizer carries a session-bound authorizer through handlers that
// construct CallerIdentity at their dispatch edge.
func WithCallerAuthorizer(ctx context.Context, authorizer Authorizer) context.Context {
	if authorizer == nil {
		return ctx
	}
	return context.WithValue(ctx, callerAuthorizerContextKey{}, authorizer)
}

// CallerAuthorizer returns the session-bound authorizer carried by ctx.
func CallerAuthorizer(ctx context.Context) Authorizer {
	if ctx == nil {
		return nil
	}
	authorizer, _ := ctx.Value(callerAuthorizerContextKey{}).(Authorizer)
	return authorizer
}

// CommandDispatcher executes a command on behalf of an authenticated caller and
// returns the typed response. It is the single dispatcher every surface
// consumes; the hub supplies one built from the plugin server dispatcher.
//
// Returning *Response (not a flattened string) lets the API surface carry typed
// Data to its transport edge unchanged, and lets text surfaces render at their
// own edge via CommandDispatcher.JSON.
type CommandDispatcher func(ctx context.Context, caller CallerIdentity, command string) (*Response, error)

// RenderedResponse carries flattened text and the accepted action that belongs
// to the transport writing that text.
type RenderedResponse struct {
	Output   string
	Response *Response
}

// TransportComplete releases the accepted action after Output reaches the
// caller. Repeated calls are harmless.
func (r *RenderedResponse) TransportComplete() {
	if r != nil && r.Response != nil {
		r.Response.TransportComplete()
	}
}

// JSON dispatches a command and flattens the typed response while retaining
// transport completion ownership. The caller must call TransportComplete only
// after it writes Output to its response transport.
func (d CommandDispatcher) JSON(ctx context.Context, caller CallerIdentity, command string) (*RenderedResponse, error) {
	resp, err := d(ctx, caller, command)
	output, renderErr := ResponseJSON(resp, err)
	return &RenderedResponse{Output: output, Response: resp}, renderErr
}

// Answer dispatches a command for a surface that writes the answer record by
// record, which today is the SSH exec channel.
//
// It is JSON's sibling and differs in exactly one way: a payload that is a row
// generator is NOT flattened. Records is walked once, so flattening it here
// would consume the rows the surface exists to write, and the surface would
// render an answer of no rows. Output is empty for such a payload and Response
// carries the generator.
//
// A generator that FAILED still reports its failure, through the same
// projection JSON applies (responseFailure): a command that reported an error
// has no rows to write, and handing the generator back would let the surface
// answer done over an empty walk.
//
// Every other payload takes the same flatten JSON takes, so a surface that
// calls this reads what it always read for every command that builds its answer
// before the answer opens.
func (d CommandDispatcher) Answer(ctx context.Context, caller CallerIdentity, command string) (*RenderedResponse, error) {
	resp, err := d(ctx, caller, command)
	if err != nil {
		return &RenderedResponse{Response: resp}, err
	}
	if _, generated := RecordRows(resp); generated {
		return &RenderedResponse{Response: resp}, responseFailure(resp)
	}
	output, renderErr := ResponseJSON(resp, nil)
	return &RenderedResponse{Output: output, Response: resp}, renderErr
}

// responseFailure is the failure the (error / nil / Status / Data) projection
// reports for resp, and nil when resp reports none.
//
// It is the one spelling of that test, read by both dispatch renderings, so a
// command that failed reads the same whether its answer is flattened into a
// string or written record by record.
func responseFailure(resp *Response) error {
	if resp == nil {
		return nil
	}
	if resp.Error != "" {
		return errors.New(resp.Error)
	}
	if resp.Status == StatusError {
		return errStatusErrorNoMessage
	}
	return nil
}

// RecordRows reports the row generator resp answers with, and whether it
// answers with one at all. A caller that must not walk the rows asks here
// first, because Records is walked once: a surface that flattens the answer
// into a string consumes the rows a record surface would have written.
func RecordRows(resp *Response) (Records, bool) {
	if resp == nil || resp.Data == nil {
		return Records{}, false
	}
	records, generated := resp.Data.(Records)
	return records, generated
}

// ResponseJSON is the single flatten sequence shared by every text surface and
// by CommandDispatcher.JSON. It is the one place the (error / nil / Status /
// Data) projection lives after unification. See JSON for the exact semantics.
func ResponseJSON(resp *Response, err error) (string, error) {
	if err != nil {
		return "", err
	}
	if failure := responseFailure(resp); failure != nil {
		return "", failure
	}
	if resp == nil || resp.Data == nil {
		return "", nil
	}
	b, jsonErr := json.Marshal(resp.Data)
	if jsonErr != nil {
		return "", fmt.Errorf("marshal response: %w", jsonErr)
	}
	return string(b), nil
}

// answerLineCapacity is the initial byte capacity of the one line buffer
// WriteAnswer reuses for every line of an answer. A longer record grows the
// slice once, and every later line of that answer reuses the grown slice.
const answerLineCapacity = 512

// WriteAnswer writes resp to w as the answer sequence every consumer reads the
// same way: a head line carrying the verdict and the type, one line for each
// record, and a terminator carrying the counts. A reader never branches on how
// many records arrive, and nothing states a count that the records can
// contradict.
//
// The head's type= is decided HERE, from the output, and never by the command.
// The encoder holds up to rpc.AnswerBufferThreshold records: a walk that ends
// within them is answered as one rpc.AnswerTypeJSON document, which is the JSON
// a command answered with before it produced records at all, so no consumer of
// an existing command meets a new shape. A walk that passes them is streamed,
// as rpc.AnswerTypeStream when the answer declares its columns and
// rpc.AnswerTypeNDJSON when it does not, and the held records are written first
// in walk order, so the switch loses none of them.
//
// It is the record-path sibling of ResponseJSON, which stays the path for a
// surface that takes the whole answer as one string (REST, gRPC, web, MCP, the
// looking glass). Both project one *Response, and AC-10 of
// spec-streaming-answer-protocol holds them to the same JSON.
//
// Each line is `#<id> ok` and a key=value tail, which rpc.AppendAnswerHead and
// its siblings write. The head carries status= and the terminator carries
// count=, so a reader tells the two apart by a key rather than by position.
func WriteAnswer(w io.Writer, id uint64, resp *Response) error {
	buf := make([]byte, 0, answerLineCapacity)

	records, generated, err := answerRecords(resp)
	if err != nil {
		return err
	}
	if generated {
		return writeRecordAnswer(w, buf, id, resp, records)
	}

	document, err := builtDocument(resp)
	if err != nil {
		return err
	}
	var count uint64
	if len(document) > 0 {
		count = 1
	}
	return writeDocumentAnswer(w, buf, id, resp, document, count, 0)
}

// writeRecordAnswer walks the generator and writes the answer it turns out to
// be. It holds the first rpc.AnswerBufferThreshold records rather than committing
// to a type it cannot take back: the head states the type, so the head is
// written once the walk has said which type this answer is.
//
// A rejected row is written and the walk goes on, so one answer can report 97
// rows applied and 3 rejected. A row too wide for one line is rejected the same
// way (boundedRecord), because refusing one row must not cost the operator the
// rows around it. A failed WRITE ends the walk: the transport is gone, and
// every later row would be produced for nobody. Returning stops the generator,
// which the range does by refusing the next yield.
func writeRecordAnswer(w io.Writer, buf []byte, id uint64, resp *Response, records Records) error {
	fields, err := marshalFields(records.Fields)
	if err != nil {
		return err
	}

	var (
		held      []rpc.Record
		count     uint64
		faults    uint64
		streaming bool
	)
	for record := range records.rows() {
		record = boundedRecord(id, count+faults+1, record)
		switch {
		case len(record.Fault) > 0:
			faults++
		case len(record.Item) > 0:
			count++
		default:
			return errEmptyAnswerRecord
		}

		if !streaming && len(held) < rpc.AnswerBufferThreshold {
			held = append(held, record)
			continue
		}
		if !streaming {
			// This record is the one past the threshold, so the answer cannot
			// be one document. The head opens the stream and the records
			// already held go out ahead of it, in the order the walk produced
			// them.
			buf, err = writeAnswerLine(w, rpc.AppendAnswerHead(buf[:0], id, answerStatus(resp), answerStreamType(records.Fields), records.Key, fields))
			if err != nil {
				return err
			}
			for i := range held {
				buf, err = writeRecordLine(w, buf, id, held[i], records.Fields)
				if err != nil {
					return err
				}
			}
			held = nil
			streaming = true
		}

		buf, err = writeRecordLine(w, buf, id, record, records.Fields)
		if err != nil {
			return err
		}
	}

	if !streaming {
		document, collapseErr := CollapseRecords(records.Key, records.Fields, slices.Values(held))
		if collapseErr != nil {
			return collapseErr
		}
		return writeDocumentAnswer(w, buf, id, resp, document, count, faults)
	}

	_, err = writeAnswerLine(w, rpc.AppendAnswerTerminator(buf[:0], id, count, faults, answerMessage(resp)))
	return err
}

// writeDocumentAnswer writes the answer whose records fit in one document: a
// head naming rpc.AnswerTypeJSON, one item line carrying that document, and the
// terminator. The head names no envelope, because the document already carries
// it, and two statements of one fact can disagree.
//
// An empty document is a command that answered with no data at all, and it
// writes no item line: nothing is not the same answer as an empty collection.
// The counts are the walk's, so the terminator states what the answer produced
// whichever type carried it.
func writeDocumentAnswer(w io.Writer, buf []byte, id uint64, resp *Response, document json.RawMessage, count, faults uint64) error {
	buf, err := writeAnswerLine(w, rpc.AppendAnswerHead(buf[:0], id, answerStatus(resp), rpc.AnswerTypeJSON, "", nil))
	if err != nil {
		return err
	}
	if len(document) > 0 {
		buf, err = writeAnswerLine(w, rpc.AppendAnswerItem(buf[:0], id, document))
		if err != nil {
			return err
		}
	}
	_, err = writeAnswerLine(w, rpc.AppendAnswerTerminator(buf[:0], id, count, faults, answerMessage(resp)))
	return err
}

// writeRecordLine writes one record of a streamed answer and returns the line
// buffer for the next one. Its caller has already refused a record carrying
// neither an item nor a fault, so an item here is never empty.
//
// A row of an answer that declares columns is checked against them here,
// because the row reaches the wire unchanged and a consumer reading it by
// position cannot tell a short row from a value it should have had.
func writeRecordLine(w io.Writer, buf []byte, id uint64, record rpc.Record, fields []string) ([]byte, error) {
	if len(record.Fault) > 0 {
		return writeAnswerLine(w, rpc.AppendAnswerFault(buf[:0], id, record.Fault))
	}
	if err := checkRowArity(record.Item, fields); err != nil {
		return buf, err
	}
	return writeAnswerLine(w, rpc.AppendAnswerItem(buf[:0], id, record.Item))
}

// boundedRecord is record when its line fits one wire message, and the rejected
// row that stands in for it when it does not. A record is one line, so a record
// wider than rpc.MaxMessageSize has no wire form at all.
//
// Rejecting it rather than failing the write is what keeps the rest of the
// answer. A walk that stopped there would discard every later row and reach no
// terminator, and a consumer reads a missing terminator as a truncated answer
// (rpc.Verdict), so one wide row would be reported as a lost connection. The
// answer instead reaches its terminator, which counts the rejection in faults=,
// and the verdict derives to partial.
//
// It is measured before the line is built, so the row is never copied into a
// line buffer that would be thrown away and left grown for the rest of the
// answer.
//
// It runs before the answer type is known, so the rejection reaches both forms
// the answer can take: the record's own line when the walk streams, and the
// collapsed document when it does not. The buffered path (ResponseJSON) writes
// no lines and rejects nothing, so a record this wide is the one payload whose
// two renderings differ, and they differ because one transport bounds a line
// and the other does not.
//
// ordinal is the record's position in the walk, counted from one, and it is how
// the rejected row names the record it stands for.
func boundedRecord(id, ordinal uint64, record rpc.Record) rpc.Record {
	size := rpc.AnswerRecordLineSize(id, record)
	if size <= rpc.MaxMessageSize {
		return record
	}
	return rpc.Record{Fault: answerRecordTooLargeFault(ordinal, size)}
}

// answerFaultCapacity is the capacity answerRecordTooLargeFault builds into: 99
// bytes of fixed text and three decimal numbers of at most 20 digits each, so
// 192 holds every fault it can write without growing the slice.
const answerFaultCapacity = 192

// answerRecordTooLargeFault is the rejected row that stands in for a record
// wider than one wire message. It names the record by its position in the walk
// and states the two sizes, so an operator can find the row that was rejected.
//
// It quotes nothing of the record. A fault carrying the row it rejected would
// be rejected for the same reason, and the row can be 16 MB. Its own length is
// bounded by construction (answerFaultCapacity), so it always fits the line the
// record did not.
func answerRecordTooLargeFault(ordinal uint64, size int) json.RawMessage {
	fault := make([]byte, 0, answerFaultCapacity)
	fault = append(fault, `{"message":"answer record does not fit one wire message","record":`...)
	fault = strconv.AppendUint(fault, ordinal, 10)
	fault = append(fault, `,"encoded-bytes":`...)
	fault = strconv.AppendInt(fault, int64(size), 10)
	fault = append(fault, `,"limit-bytes":`...)
	fault = strconv.AppendInt(fault, rpc.MaxMessageSize, 10)
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

// answerRecords reports the row generator resp answers with, and whether it
// answers with one at all. A payload the handler built before the answer opened
// is not a generator: it is one document, and builtDocument renders it.
//
// The reserved envelope key is refused here as well as in the collapse, because
// a streamed answer never reaches the collapse and a handler owes the same
// refusal whatever its row count turns out to be.
func answerRecords(resp *Response) (Records, bool, error) {
	records, generated := RecordRows(resp)
	if !generated {
		return Records{}, false, nil
	}
	if records.Key == answerErrorsKey {
		return Records{}, false, errReservedEnvelopeKey
	}
	return records, true, nil
}

// builtDocument marshals a payload the handler built before the answer opened.
// It is the whole answer in one document, byte for byte what ResponseJSON
// renders for the same payload. A response carrying no data at all has no
// document, and the answer then carries no item line.
func builtDocument(resp *Response) (json.RawMessage, error) {
	if resp == nil || resp.Data == nil {
		return nil, nil
	}
	document, err := json.Marshal(resp.Data)
	if err != nil {
		return nil, fmt.Errorf("marshal response: %w", err)
	}
	return document, nil
}

// marshalFields encodes the column names a streamed answer's head carries. It
// runs once for the answer, before any line is written, so a schema that cannot
// be encoded is named instead of half an answer being written.
func marshalFields(fields []string) (json.RawMessage, error) {
	if len(fields) == 0 {
		return nil, nil
	}
	encoded, err := json.Marshal(fields)
	if err != nil {
		return nil, fmt.Errorf("marshal answer fields: %w", err)
	}
	return encoded, nil
}

// answerStreamType is how a streamed answer's records are read: positional
// arrays against the head's fields when the answer declares a column schema,
// self-describing objects when it does not.
func answerStreamType(fields []string) string {
	if len(fields) == 0 {
		return rpc.AnswerTypeNDJSON
	}
	return rpc.AnswerTypeStream
}

// noRecords is the empty row sequence. A command that produced nothing still
// writes a head and a terminator, so its answer is complete rather than short.
func noRecords(func(rpc.Record) bool) {}

// answerStatus is what the head declares: what the daemon knows when the answer
// opens. A command that failed before its first row says so on the first line,
// so a consumer that renders a failure differently commits to a rendering
// without buffering the answer to find out.
func answerStatus(resp *Response) string {
	if resp == nil {
		return StatusError
	}
	if resp.Status == StatusError {
		return StatusError
	}
	if resp.Error != "" {
		return StatusError
	}
	return StatusDone
}

// answerMessage is the operational text the terminator carries: why a walk
// produced fewer records than it set out to, or none at all.
func answerMessage(resp *Response) string {
	if resp == nil {
		return "no response"
	}
	return resp.Error
}

// errStatusErrorNoMessage matches the historical "unknown error" text the hub
// adapters returned for a Status=error response that carried no Error message.
var errStatusErrorNoMessage = errors.New("unknown error")

// errEmptyAnswerRecord is what a row carrying neither an item nor a fault
// earns, on the record path and on the buffered one alike. rpc.Record sets
// exactly one of the two; a row that sets neither reaches the wire as `item=`
// with no value, which no consumer can decode, and reaches a buffered consumer
// as null, which reads like a row the command produced. Refusing it names the
// producer rather than handing either consumer an empty-looking answer
// (`ai/rules/evidence.md`).
var errEmptyAnswerRecord = errors.New("answer record carries neither an item nor a fault")

// errReservedEnvelopeKey is what a Records naming its envelope answerErrorsKey
// earns, on the record path and on the buffered one alike. A buffered answer
// carries the rejected rows under that name beside the rows the command
// produced, so an envelope of the same name would put the two collections under
// one key and lose one of them. Both producers refuse it, so a handler learns
// on its first answer rather than on its first rejected row.
var errReservedEnvelopeKey = fmt.Errorf("answer envelope key %q is reserved for the rejected rows", answerErrorsKey)
