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

// WriteAnswer writes resp to w as the answer sequence every consumer reads the
// same way: a head line naming the item type and the envelope, one line for each
// record, and a terminator carrying the counts and the message. A reader never
// branches on how many records arrive, and nothing states a count that the
// records can contradict. The head states no outcome, so the terminator is the
// one line an outcome is read from.
//
// It projects one *Response onto the answer grammar and hands the writing to
// rpc, which is the ONE writer both ends of the plugin connection use: the
// engine writes the answer to a command an operator ran, and the SDK writes the
// answer to a command the engine asked a plugin to run (Records.WriteAnswer,
// pkg/plugin/records.go). What this file still owns is the projection -- what a
// built payload renders to, and what the terminator says about a failure.
//
// A generator answers through rpc.WriteRecordAnswer, which decides the head's
// item type from the walk. A payload the handler built before the answer opened
// is one document and answers through rpc.WriteDocumentAnswer.
//
// It is the record-path sibling of ResponseJSON, which stays the path for a
// surface that takes the whole answer as one string (REST, gRPC, web, MCP, the
// looking glass). Both project one *Response, and AC-10 of
// spec-streaming-answer-protocol holds them to the same JSON.
func WriteAnswer(w io.Writer, id uint64, resp *Response) error {
	records, generated, err := answerRecords(resp)
	if err != nil {
		return err
	}
	head := rpc.AnswerTail{Message: answerMessage(resp)}
	if generated {
		head.Key = records.Key
		head.Fields = records.Fields
		return rpc.WriteRecordAnswer(w, id, head, records.rows())
	}

	document, err := builtDocument(resp)
	if err != nil {
		return err
	}
	return rpc.WriteDocumentAnswer(w, id, head, document)
}

// AnswerFor returns the answer resp produces for a consumer in THIS process:
// the head a peer on the socket would read, the records in walk order, and the
// terminator that walk earns.
//
// It is WriteAnswer's in-process sibling and makes the same two decisions from
// the same constant. The head's item type is decided here, from the output: a walk
// that ends within rpc.AnswerBufferThreshold records is one rpc.AnswerTypeDocument
// document, and a walk that passes them is streamed, so one command answers one
// shape whichever transport carried it (AC-7 of
// spec-record-answers-1-sdk-path).
//
// The two differ in one place, and they differ because one transport bounds a
// line and the other does not: a record too wide for one wire message is
// rejected by the wire writer (boundedRecord, pkg/plugin/rpc/answer_write.go)
// and carried whole here, exactly as ResponseJSON carries it.
//
// It walks the generator to its end before it returns, because the head must
// state the type before the consumer reads the first record and only the row
// count decides that type. The bound is the answer the engine just produced,
// which is the bound the in-process path already accepts: the projection this
// replaces marshaled the same walk into one buffer
// (responseToDispatchOutput, internal/component/plugin/server/dispatch.go).
// Nothing here starts a goroutine, for the answer or for a row
// (ai/rules/goroutine-lifecycle.md).
//
// The consumer MUST range over Answer.Records to its end or stop it
// deliberately, and MUST read Verdict, Err and Message after that range
// (Answer, pkg/plugin/rpc/types.go).
func AnswerFor(resp *Response) (*rpc.Answer, error) {
	records, generated, err := answerRecords(resp)
	if err != nil {
		return nil, err
	}
	if !generated {
		document, buildErr := builtDocument(resp)
		if buildErr != nil {
			return nil, buildErr
		}
		// A payload the handler built before the answer opened is one row, and
		// a response carrying no data at all is none.
		var count uint64
		if len(document) > 0 {
			count = 1
		}
		return documentAnswer(resp, document, count, 0), nil
	}

	held, count, faults, walkErr := heldRecords(records)
	if walkErr != nil {
		return nil, walkErr
	}
	if len(held) <= rpc.AnswerBufferThreshold {
		document, collapseErr := rpc.CollapseRecords(records.Key, records.Fields, slices.Values(held))
		if collapseErr != nil {
			return nil, collapseErr
		}
		return documentAnswer(resp, document, count, faults), nil
	}

	head := rpc.AnswerTail{
		Type:   rpc.AnswerStreamType(records.Fields),
		Key:    records.Key,
		Fields: records.Fields,
	}
	terminator := rpc.AnswerTail{Count: count, Faults: faults, Message: answerMessage(resp)}
	return rpc.NewAnswer(head, terminator, slices.Values(held)), nil
}

// documentAnswer is the answer whose records fit one document: a head naming
// rpc.AnswerTypeDocument, that one document as the single record, and the
// terminator. The head names no envelope, because the document already carries
// it, and two statements of one fact can disagree.
//
// An empty document is a command that answered with no data at all, and it
// carries no record: nothing is not the same answer as an empty collection.
//
// count and faults are the WALK's, not the record's, so the terminator states
// what the command produced whichever type carried it. That is what
// rpc.WriteDocumentAnswer states on the wire for the same answer.
func documentAnswer(resp *Response, document json.RawMessage, count, faults uint64) *rpc.Answer {
	head := rpc.AnswerTail{Type: rpc.AnswerTypeDocument}
	terminator := rpc.AnswerTail{Count: count, Faults: faults, Message: answerMessage(resp)}
	if len(document) == 0 {
		return rpc.NewAnswer(head, terminator, nil)
	}
	return rpc.NewAnswer(head, terminator, slices.Values([]rpc.Record{{Item: document}}))
}

// heldRecords walks the generator to its end and reports the records it
// produced, in walk order, with the rows it produced and the rows it rejected.
//
// A row carrying neither an item nor a fault is refused here rather than
// carried, which is the refusal the wire producer makes for the same row. It is
// refused BEFORE the answer is handed out, so the producer is named by a
// returned error rather than by a walk that stops half way.
//
// The consumer holds every record past the walk, so each row is appended into a
// slice of its own (rpc.HeldRecords). The wire writer keeps none and appends
// into its own buffers instead, which is why a streamed row costs nothing and
// an in-process one costs this.
func heldRecords(records Records) (held []rpc.Record, count, faults uint64, err error) {
	for record := range rpc.HeldRecords(records.rows()) {
		switch {
		case len(record.Fault) > 0:
			faults++
		case len(record.Item) > 0:
			count++
		default:
			return nil, 0, 0, rpc.ErrEmptyAnswerRecord
		}
		held = append(held, record)
	}
	return held, count, faults, nil
}

// answerRecords reports the row generator resp answers with, and whether it
// answers with one at all. A payload the handler built before the answer opened
// is not a generator: it is one document, and builtDocument renders it.
//
// The reserved envelope key is refused here as well as by the two producers it
// reaches (rpc.WriteRecordAnswer and rpc.CollapseRecords), because AnswerFor
// hands an in-process consumer a walk that neither of them writes: a handler
// owes the same refusal whatever its row count and whatever its transport turn
// out to be.
func answerRecords(resp *Response) (Records, bool, error) {
	records, generated := RecordRows(resp)
	if !generated {
		return Records{}, false, nil
	}
	if records.Key == rpc.AnswerErrorsKey {
		return Records{}, false, rpc.ErrReservedEnvelopeKey
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

// noRecords is the empty row sequence. A command that produced nothing still
// writes a head and a terminator, so its answer is complete rather than short.
func noRecords(func(rpc.RowRecord) bool) {}

// answerMessage is the operational text the terminator carries: why a walk
// produced fewer records than it set out to, why it produced none at all, or
// nothing when it ran to its end.
//
// A response that FAILED always states one. The terminator is the one line an
// answer states its outcome on, and a terminator carrying no message and no
// counts derives to done (rpc.Verdict), so a failure that named no reason would
// reach a consumer as a success. That state is reachable rather than
// theoretical: responseFailure carries errStatusErrorNoMessage for a response
// whose Status is StatusError and whose Error is empty. Naming it here is what
// keeps the outcome on one line instead of splitting it across two that can
// disagree (A-5).
func answerMessage(resp *Response) string {
	if resp == nil {
		return "no response"
	}
	if resp.Error != "" {
		return resp.Error
	}
	if resp.Status == StatusError {
		return rpc.AnswerFailureUnstated
	}
	return ""
}

// errStatusErrorNoMessage is the failure a Status=error response with no Error
// message reports. It is the text the terminator states for the same response
// (rpc.AnswerFailureUnstated), so the string surface and the record surface name
// one failure one way.
var errStatusErrorNoMessage = errors.New(rpc.AnswerFailureUnstated)

// The two refusals a record producer earns live in pkg/plugin/rpc, beside the
// writer, the collapse and the appenders that share them:
// rpc.ErrEmptyAnswerRecord for a row carrying neither an item nor a fault, and
// rpc.ErrReservedEnvelopeKey for an answer naming rpc.AnswerErrorsKey as its
// envelope. Every path a handler's answer can take refuses them -- the wire
// writer, the collapse a buffered surface reads through, and answerRecords for
// the in-process walk -- so a handler learns on its first answer rather than on
// its first rejected row.
