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
	"iter"

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

// ResponseJSON is the single flatten sequence shared by every text surface and
// by CommandDispatcher.JSON. It is the one place the (error / nil / Status /
// Data) projection lives after unification. See JSON for the exact semantics.
func ResponseJSON(resp *Response, err error) (string, error) {
	if err != nil {
		return "", err
	}
	if resp == nil {
		return "", nil
	}
	if resp.Error != "" {
		return "", errors.New(resp.Error)
	}
	if resp.Status == StatusError {
		return "", errStatusErrorNoMessage
	}
	if resp.Data == nil {
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
// same way: a head line carrying the verdict, one line for each record, and a
// terminator carrying the counts. An answer of one record is that shape
// carrying one record, so a reader never branches on how many records arrive,
// and nothing states a count that the records can contradict.
//
// It is the record-path sibling of ResponseJSON, which stays the path for a
// surface that takes the whole answer as one string (REST, gRPC, web, MCP, the
// looking glass). Both project one *Response, and AC-10 of
// plan/spec-streaming-answer-protocol.md holds them to the same JSON.
//
// Each line is `#<id> ok` and a key=value tail, which rpc.AppendAnswerHead and
// its siblings write. The head carries status= and the terminator carries
// count=, so a reader tells the two apart by a key rather than by position.
func WriteAnswer(w io.Writer, id uint64, resp *Response) error {
	key, rows, err := answerRows(resp)
	if err != nil {
		return err
	}

	buf := make([]byte, 0, answerLineCapacity)
	buf, err = writeAnswerLine(w, rpc.AppendAnswerHead(buf, id, answerStatus(resp), key))
	if err != nil {
		return err
	}

	// A rejected row is written and the walk goes on, so one answer can report
	// 97 rows applied and 3 rejected. A failed WRITE ends the walk: the
	// transport is gone, and every later row would be produced for nobody.
	// Returning here stops the generator, which the range does by refusing the
	// next yield.
	var count, faults uint64
	for record := range rows {
		switch {
		case len(record.Fault) > 0:
			faults++
			buf, err = writeAnswerLine(w, rpc.AppendAnswerFault(buf[:0], id, record.Fault))
		case len(record.Item) > 0:
			count++
			buf, err = writeAnswerLine(w, rpc.AppendAnswerItem(buf[:0], id, record.Item))
		default:
			return errEmptyAnswerRecord
		}
		if err != nil {
			return err
		}
	}

	_, err = writeAnswerLine(w, rpc.AppendAnswerTerminator(buf[:0], id, count, faults, answerMessage(resp)))
	return err
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

// answerRows returns the envelope key the head names and the rows the answer
// carries. A payload that is not a generator is one record, which is what keeps
// the reader on one path whatever the handler answered with.
func answerRows(resp *Response) (string, iter.Seq[rpc.Record], error) {
	if resp == nil {
		return "", noRecords, nil
	}
	if resp.Data == nil {
		return "", noRecords, nil
	}
	if records, isGenerator := resp.Data.(Records); isGenerator {
		if records.Key == answerErrorsKey {
			return "", nil, errReservedEnvelopeKey
		}
		return records.Key, records.rows(), nil
	}
	item, err := json.Marshal(resp.Data)
	if err != nil {
		return "", nil, fmt.Errorf("marshal response: %w", err)
	}
	return "", oneRecord(rpc.Record{Item: item}), nil
}

// noRecords is the empty row sequence. A command that produced nothing still
// writes a head and a terminator, so its answer is complete rather than short.
func noRecords(func(rpc.Record) bool) {}

// oneRecord is the row sequence of an answer that carries exactly one record.
func oneRecord(record rpc.Record) iter.Seq[rpc.Record] {
	return func(yield func(rpc.Record) bool) {
		yield(record)
	}
}

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
