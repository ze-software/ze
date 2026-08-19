// Design: docs/architecture/api/ipc_protocol.md — RPC wire message types
// Related: conn.go — Conn uses line format for RPC framing
// Related: framing.go — newline-delimited frame reader/writer
// Related: types.go — domain-specific RPC input/output types

package rpc

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Request represents a parsed incoming RPC request line: #<id> <method> [<json>].
type Request struct {
	ID     uint64          // Correlation ID from #<id> prefix
	Method string          // module:rpc-name
	Params json.RawMessage // JSON payload (may be nil)
}

// RPCCallError represents an error returned by the remote side via #<id> error [<json>].
type RPCCallError struct {
	Code    string // Short kebab-case identifier (may be empty)
	Message string // Human-readable detail
}

func (e *RPCCallError) Error() string {
	if e.Message != "" {
		return "rpc error: " + e.Message
	}
	if e.Code != "" {
		return "rpc error: " + e.Code
	}
	return "rpc error: (no message)"
}

// CodedError is a Go error that carries a short machine-readable code.
// Used to pass structured error information through the dispatch chain
// so that Dispatch can construct an error response with a proper code.
type CodedError struct {
	Code    string // Short kebab-case identifier (e.g., "unknown-command")
	message string
}

// NewCodedError creates an error with a code and human-readable message.
func NewCodedError(code, message string) *CodedError {
	return &CodedError{Code: code, message: message}
}

func (e *CodedError) Error() string { return e.message }

// ExtractErrorMessage extracts the human-readable message from error payload JSON.
// Returns the message if present, or empty string.
func ExtractErrorMessage(payload json.RawMessage) string {
	if len(payload) == 0 {
		return ""
	}
	var detail struct {
		Message string `json:"message"`
	}
	if json.Unmarshal(payload, &detail) == nil {
		return detail.Message
	}
	return ""
}

// ParseLine parses a wire line into id, verb, and payload.
// Format: #<id> <verb> [<payload>].
func ParseLine(line []byte) (id uint64, verb string, payload []byte, err error) {
	s := string(line)
	if !strings.HasPrefix(s, "#") {
		return 0, "", nil, fmt.Errorf("line missing # prefix: %q", truncate(s, 80))
	}
	s = s[1:] // strip #

	// Extract ID
	idStr, rest, hasRest := strings.Cut(s, " ")
	id, err = strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		return 0, "", nil, fmt.Errorf("invalid id %q: %w", idStr, err)
	}

	if !hasRest || rest == "" {
		return 0, "", nil, fmt.Errorf("line has no verb after #%d", id)
	}

	// Extract verb and optional payload
	verb, payloadStr, _ := strings.Cut(rest, " ")
	if payloadStr != "" {
		payload = []byte(payloadStr)
	}

	return id, verb, payload, nil
}

// AppendRequest appends a request line (#<id> <method> [<json>]) to buf
// and returns the extended slice. Newline is NOT appended. Callers on
// the hot path should supply a pool buffer; tests and one-shot callers
// can pass nil.
func AppendRequest(buf []byte, id uint64, method string, params json.RawMessage) []byte {
	buf = append(buf, '#')
	buf = strconv.AppendUint(buf, id, 10)
	buf = append(buf, ' ')
	buf = append(buf, method...)
	if len(params) == 0 || string(params) == "null" {
		return buf
	}
	buf = append(buf, ' ')
	buf = append(buf, params...)
	return buf
}

// AppendResult appends a success response line (#<id> ok [<json>]) to
// buf. Newline is NOT appended.
func AppendResult(buf []byte, id uint64, result json.RawMessage) []byte {
	buf = append(buf, '#')
	buf = strconv.AppendUint(buf, id, 10)
	if len(result) == 0 || string(result) == "null" {
		return append(buf, ' ', 'o', 'k')
	}
	buf = append(buf, ' ', 'o', 'k', ' ')
	buf = append(buf, result...)
	return buf
}

// AppendOK appends an empty success response line (#<id> ok) to buf.
// Newline is NOT appended.
func AppendOK(buf []byte, id uint64) []byte {
	buf = append(buf, '#')
	buf = strconv.AppendUint(buf, id, 10)
	return append(buf, ' ', 'o', 'k')
}

// AppendError appends an error response line (#<id> error [<json>]) to
// buf. Newline is NOT appended.
func AppendError(buf []byte, id uint64, errPayload json.RawMessage) []byte {
	buf = append(buf, '#')
	buf = strconv.AppendUint(buf, id, 10)
	if len(errPayload) == 0 {
		return append(buf, " error"...)
	}
	buf = append(buf, " error "...)
	buf = append(buf, errPayload...)
	return buf
}

// Answer lines. Every answer is a head, zero or more records, and a
// terminator, whatever its record count, so a reader follows one path and
// nothing declares a shape the payload can contradict. Each line is
// `#<id> ok <tail>`, and the tail is bare key=value pairs, so a reader decides
// how to read the answer without a JSON decoder.

// Answer tail key names. The appenders below write these and ParseAnswerTail
// reads them, so a key cannot be spelled one way on the wire and another way in
// the reader. answerKeyEnvelope is spelled `key` on the wire and names the
// envelope the records belong under.
const (
	answerKeyStatus   = "status"
	answerKeyEnvelope = "key"
	answerKeyItem     = "item"
	answerKeyFault    = "fault"
	answerKeyCount    = "count"
	answerKeyFaults   = "faults"
	answerKeyMessage  = "message"
	answerKeyCode     = "code"
)

// AppendAnswerHead appends an answer head line
// (#<id> ok status=<status> [key=<key>]) to buf and returns the extended slice.
// Newline is NOT appended. status is StatusDone or StatusError, and it states
// what the daemon knows when the answer opens, so a consumer that renders a
// failure differently commits to a rendering on the first line rather than
// buffering the whole answer. key names the envelope the records belong under,
// and an empty key writes no key= at all.
func AppendAnswerHead(buf []byte, id uint64, status, key string) []byte {
	buf = appendAnswerPrefix(buf, id)
	buf = appendAnswerKey(buf, answerKeyStatus)
	buf = append(buf, status...)
	if key == "" {
		return buf
	}
	buf = appendAnswerKey(buf, answerKeyEnvelope)
	return append(buf, key...)
}

// AppendAnswerItem appends a result record line (#<id> ok item=<json>) to buf
// and returns the extended slice. Newline is NOT appended. item takes the rest
// of the line verbatim, so a JSON value holding `=` or a space needs no
// quoting and no escaping.
func AppendAnswerItem(buf []byte, id uint64, item json.RawMessage) []byte {
	buf = appendAnswerPrefix(buf, id)
	buf = appendAnswerKey(buf, answerKeyItem)
	start := len(buf)
	buf = append(buf, item...)
	return replaceNewlines(buf, start)
}

// AppendAnswerFault appends an error record line (#<id> ok fault=<json>) to buf
// and returns the extended slice. Newline is NOT appended. A fault records one
// rejected row and does not end the walk, which is what lets an answer report
// 97 rows applied and 3 rejected.
func AppendAnswerFault(buf []byte, id uint64, fault json.RawMessage) []byte {
	buf = appendAnswerPrefix(buf, id)
	buf = appendAnswerKey(buf, answerKeyFault)
	start := len(buf)
	buf = append(buf, fault...)
	return replaceNewlines(buf, start)
}

// AppendAnswerTerminator appends the terminator line
// (#<id> ok count=<count> [faults=<faults>] [message=<message>]) to buf and
// returns the extended slice. Newline is NOT appended. The terminator is the
// line carrying count=, so head and terminator are told apart by a key rather
// than by position. It states no status: the verdict is derived from the counts
// it carries, which is one source of truth instead of two that can disagree.
// count counts result records, faults counts rejected ones, and message states
// that the walk aborted after count records. A faults of 0 and an empty message
// write no key, so their absence is what states them.
func AppendAnswerTerminator(buf []byte, id, count, faults uint64, message string) []byte {
	buf = appendAnswerPrefix(buf, id)
	buf = appendAnswerKey(buf, answerKeyCount)
	buf = strconv.AppendUint(buf, count, 10)
	if faults > 0 {
		buf = appendAnswerKey(buf, answerKeyFaults)
		buf = strconv.AppendUint(buf, faults, 10)
	}
	if message == "" {
		return buf
	}
	buf = appendAnswerKey(buf, answerKeyMessage)
	start := len(buf)
	buf = append(buf, message...)
	return replaceNewlines(buf, start)
}

// AppendAnswerNotUnderstood appends the answer to a command the daemon did not
// understand (#<id> error [code=<code>] message=<message>) to buf and returns
// the extended slice. Newline is NOT appended. It is the whole answer for its
// id: the verb says the conversation was valid and the command was not, which
// is what lets a client offer completion here and an operational message for a
// command that was understood and then failed.
func AppendAnswerNotUnderstood(buf []byte, id uint64, code, message string) []byte {
	buf = append(buf, '#')
	buf = strconv.AppendUint(buf, id, 10)
	buf = append(buf, " error"...)
	if code != "" {
		buf = appendAnswerKey(buf, answerKeyCode)
		buf = append(buf, code...)
	}
	buf = appendAnswerKey(buf, answerKeyMessage)
	start := len(buf)
	buf = append(buf, message...)
	return replaceNewlines(buf, start)
}

// appendAnswerPrefix appends the `#<id> ok` every answer line opens with.
func appendAnswerPrefix(buf []byte, id uint64) []byte {
	buf = append(buf, '#')
	buf = strconv.AppendUint(buf, id, 10)
	return append(buf, ' ', 'o', 'k')
}

// appendAnswerKey appends the ` <name>=` that opens one tail pair.
func appendAnswerKey(buf []byte, name string) []byte {
	buf = append(buf, ' ')
	buf = append(buf, name...)
	return append(buf, '=')
}

// replaceNewlines overwrites every newline in buf from start onwards with a
// space. An open-ended value runs to the end of the line, so a newline inside
// one would split a single answer line into two and leave the reader taking the
// second half as a line of its own. A newline is insignificant whitespace
// between JSON tokens and can never appear inside a JSON string, so this cannot
// change what an item or a fault decodes to.
func replaceNewlines(buf []byte, start int) []byte {
	for i := start; i < len(buf); i++ {
		if buf[i] == '\n' || buf[i] == '\r' {
			buf[i] = ' '
		}
	}
	return buf
}

// Verdicts an answer derives from its terminator. VerdictDone and VerdictError
// are the status vocabulary the head already uses, so the two lines say the
// same word for the same outcome.
const (
	VerdictDone      = StatusDone
	VerdictPartial   = "partial"
	VerdictError     = StatusError
	VerdictAborted   = "aborted"
	VerdictTruncated = "truncated"
)

// AnswerTail is one answer line's decoded key=value tail. A consumer reads it
// without a JSON decoder, and it needs no state beyond "have I seen the head":
// the line carrying count= is the terminator, whatever its position.
//
// Item and Fault reference the payload they were parsed from rather than
// copying it, so a consumer that forwards a record parses and copies nothing.
type AnswerTail struct {
	// Status is the head's status=: StatusDone or StatusError. A terminator
	// states none.
	Status string
	// Key is the head's key=, the envelope the records belong under. It is
	// empty when the head names none.
	Key string
	// Item is a result record's item=, and Fault an error record's fault=. One
	// line carries at most one of them.
	Item  json.RawMessage
	Fault json.RawMessage
	// Count and Faults are the terminator's counts: result records produced,
	// and rows the walk rejected.
	Count  uint64
	Faults uint64
	// Message is the terminator's message=, the operational text that states an
	// aborted walk, or the not-understood answer's message=.
	Message string
	// Code is the not-understood answer's code=.
	Code string

	hasCount bool
}

// IsTerminator reports whether this line ends the answer. The terminator is the
// line carrying count=, so a reader tells it from a head by a key rather than
// by counting lines, and needs no lookahead.
func (t AnswerTail) IsTerminator() bool { return t.hasCount }

// ParseAnswerTail decodes the payload of an answer line into its keys. payload
// is what ParseLine returns: everything after the verb.
//
// It refuses an unknown key rather than skipping it, so a line this build
// cannot fully read is reported instead of half-read, and it refuses a
// terminator that states a status, which is what keeps the counts the single
// source of the verdict.
func ParseAnswerTail(payload []byte) (AnswerTail, error) {
	var tail AnswerTail

	// Each pass consumes at least a key and its =, so the loop is bounded by
	// the payload length, which the frame reader has already bounded.
	rest := payload
	for len(rest) > 0 {
		name, value, remainder, err := answerTailToken(rest)
		if err != nil {
			return AnswerTail{}, err
		}
		rest = remainder

		switch string(name) {
		case answerKeyStatus:
			tail.Status = string(value)
		case answerKeyEnvelope:
			tail.Key = string(value)
		case answerKeyItem:
			tail.Item = value
		case answerKeyFault:
			tail.Fault = value
		case answerKeyMessage:
			tail.Message = string(value)
		case answerKeyCode:
			tail.Code = string(value)
		case answerKeyCount:
			count, countErr := strconv.ParseUint(string(value), 10, 64)
			if countErr != nil {
				return AnswerTail{}, fmt.Errorf("answer tail count=%q: %w", truncate(string(value), 40), countErr)
			}
			tail.Count = count
			tail.hasCount = true
		case answerKeyFaults:
			faults, faultsErr := strconv.ParseUint(string(value), 10, 64)
			if faultsErr != nil {
				return AnswerTail{}, fmt.Errorf("answer tail faults=%q: %w", truncate(string(value), 40), faultsErr)
			}
			tail.Faults = faults
		default:
			return AnswerTail{}, fmt.Errorf("answer tail carries unknown key %q", truncate(string(name), 40))
		}
	}

	if tail.hasCount && tail.Status != "" {
		return AnswerTail{}, fmt.Errorf("answer terminator states status=%q: its verdict derives from its counts", truncate(tail.Status, 40))
	}
	if len(tail.Item) > 0 && len(tail.Fault) > 0 {
		return AnswerTail{}, errors.New("answer record carries both item= and fault=")
	}
	return tail, nil
}

// The two bytes an answer tail is cut on: a key from its value, and one pair
// from the next.
var (
	answerTailAssign    = []byte{'='}
	answerTailSeparator = []byte{' '}
)

// answerTailToken splits tail at its first key=value pair and returns the key
// name, its value, and what follows the pair. item=, fault= and message= take
// the rest of the line verbatim and are last, so a value holding = or a space
// needs no quoting and no escaping, and nothing follows one of them.
func answerTailToken(tail []byte) (name, value, rest []byte, err error) {
	var found bool
	name, value, found = bytes.Cut(tail, answerTailAssign)
	if !found {
		return nil, nil, nil, fmt.Errorf("answer tail carries a token with no =: %q", truncate(string(tail), 40))
	}
	if isOpenEndedKey(name) {
		return name, value, nil, nil
	}
	value, rest, _ = bytes.Cut(value, answerTailSeparator)
	return name, value, rest, nil
}

// isOpenEndedKey reports whether name's value runs to the end of the line.
func isOpenEndedKey(name []byte) bool {
	switch string(name) {
	case answerKeyItem, answerKeyFault, answerKeyMessage:
		return true
	}
	return false
}

// Verdict derives an answer's outcome from its terminator, which is the only
// place that outcome is stated: a terminator carries no status=, so nothing can
// disagree with the counts. terminator is nil when none arrived, and a line
// that is not a terminator is read the same way, because both mean the answer
// stopped before it ended and a consumer must not read what it got as complete.
func Verdict(terminator *AnswerTail) string {
	if terminator == nil || !terminator.IsTerminator() {
		return VerdictTruncated
	}
	if terminator.Message != "" {
		return VerdictAborted
	}
	if terminator.Faults == 0 {
		return VerdictDone
	}
	if terminator.Count == 0 {
		return VerdictError
	}
	return VerdictPartial
}

// FormatRequest returns a request line (#<id> <method> [<json>]) in a
// freshly-allocated slice. Retained for tests and low-rate callers;
// hot-path senders should use AppendRequest with a pool buffer.
func FormatRequest(id uint64, method string, params json.RawMessage) []byte {
	return AppendRequest(make([]byte, 0, 2+20+1+len(method)+1+len(params)), id, method, params)
}

// FormatResult returns a success response in a freshly-allocated slice.
// Retained for tests; hot-path senders should use AppendResult.
func FormatResult(id uint64, result json.RawMessage) []byte {
	return AppendResult(make([]byte, 0, 2+20+4+len(result)), id, result)
}

// FormatOK returns an empty success response in a freshly-allocated
// slice. Retained for tests; hot-path senders should use AppendOK.
func FormatOK(id uint64) []byte {
	return AppendOK(make([]byte, 0, 2+20+3), id)
}

// FormatError returns an error response in a freshly-allocated slice.
// Retained for tests; hot-path senders should use AppendError.
func FormatError(id uint64, errPayload json.RawMessage) []byte {
	return AppendError(make([]byte, 0, 2+20+7+len(errPayload)), id, errPayload)
}

// NewErrorPayload creates a JSON error payload with code and message fields.
func NewErrorPayload(code, message string) json.RawMessage {
	data, _ := json.Marshal(struct {
		Code    string `json:"code,omitempty"`
		Message string `json:"message"`
	}{Code: code, Message: message})
	return data
}

// parseRPCError parses an error payload JSON into an RPCCallError.
func parseRPCError(payload []byte) *RPCCallError {
	if len(payload) == 0 {
		return &RPCCallError{}
	}
	var detail struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if json.Unmarshal(payload, &detail) == nil {
		return &RPCCallError{Code: detail.Code, Message: detail.Message}
	}
	// A not-understood answer states its text as a key=value tail rather than as
	// JSON, so the tail is read before the fallback below: without this the
	// operator reads the whole tail, `message=` included, as the message.
	if tail, tailErr := ParseAnswerTail(payload); tailErr == nil {
		if tail.Message != "" || tail.Code != "" {
			return &RPCCallError{Code: tail.Code, Message: tail.Message}
		}
	}
	// Payload is neither JSON nor a tail — use it as the message directly.
	return &RPCCallError{Message: string(payload)}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
