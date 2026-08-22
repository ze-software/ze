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
	"slices"
	"strconv"
	"strings"
)

// Request represents a parsed incoming RPC request line: #<len>:<id> <method> [<json>].
type Request struct {
	ID     uint64          // Correlation ID from the #<len>:<id> prefix
	Method string          // module:rpc-name
	Params json.RawMessage // JSON payload (may be nil)
}

// RPCCallError represents an error returned by the remote side via
// #<len>:<id> error [<json>].
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

// The id field every line carrying an id opens with: `#<len>:<id> `, where
// <len> is one base-36 character stating how many decimal digits the id
// occupies. A reader takes that one byte and reaches every later field by
// addition, and searches no line for a separator.
//
// idDigitsMax is the widest id: a uint64 is at most 20 decimal digits.
// idPrefixMax is the whole field at that width: `#`, the length, `:`, twenty
// digits, and the space that ends the field.
const (
	idDigitsMax = 20
	idPrefixMax = 24
)

// idLengthAlphabet spells the length character, `0` to `9` then `A` to `Z`, so
// one byte states a length up to 35. A uint64 needs 20 decimal digits at most,
// so the whole id range is expressible with room left and no counter has to
// wrap to keep its id readable.
const idLengthAlphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"

// appendID appends the `#<len>:<id> ` field naming which conversation a line
// belongs to. Requests, responses and answer lines open with it alike, so the
// protocol carries ONE id encoding rather than one for each direction.
//
// The digits are formatted once and the length is read off the very bytes that
// reach the wire, so the two halves of the field cannot disagree (R-2).
func appendID(buf []byte, id uint64) []byte {
	var scratch [idDigitsMax]byte
	digits := strconv.AppendUint(scratch[:0], id, 10)
	buf = append(buf, '#', idLengthAlphabet[len(digits)], ':')
	buf = append(buf, digits...)
	return append(buf, ' ')
}

// idLengthValue reads one base-36 length character and reports whether it is
// one. Anything outside `0` to `9` and `A` to `Z` is refused, lower case
// included: one spelling of a length is what keeps the offset arithmetic the
// same at both ends of the wire.
func idLengthValue(c byte) (int, bool) {
	if c >= '0' && c <= '9' {
		return int(c - '0'), true
	}
	if c >= 'A' && c <= 'Z' {
		return int(c-'A') + 10, true
	}
	return 0, false
}

// cutID takes the `#<len>:<id> ` field off the front of line and returns the
// id's decimal digits and everything after the space that ends the field.
//
// The length character states the digit count, so the id and the field after
// it are both reached by addition. The byte the length names as the end of the
// id MUST be that space: a length disagreeing with its digits would otherwise
// slice the next field in half, and this check is what makes such a line a
// refusal rather than a wrong read (R-2).
//
// It is the one reader of the id, and appendID is the one writer, so no
// consumer keeps its own copy of the rule.
func cutID(line string) (digits, rest string, err error) {
	if line == "" {
		return "", "", errors.New("empty line carries no id")
	}
	if line[0] != '#' {
		return "", "", fmt.Errorf("line missing # prefix: %q", truncate(line, 80))
	}
	if len(line) < 3 {
		return "", "", fmt.Errorf("line states no id length: %q", truncate(line, 80))
	}
	count, known := idLengthValue(line[1])
	if !known {
		return "", "", fmt.Errorf("id length %q is not one base-36 character, 0 to 9 then A to Z", line[1:2])
	}
	if count == 0 {
		return "", "", fmt.Errorf("id states a length of zero digits: %q", truncate(line, 80))
	}
	if count > idDigitsMax {
		return "", "", fmt.Errorf("id states %d digits, past the %d a uint64 occupies", count, idDigitsMax)
	}
	if line[2] != ':' {
		return "", "", fmt.Errorf("id length is not followed by ':': %q", truncate(line, 80))
	}

	end := 3 + count
	if len(line) <= end {
		return "", "", fmt.Errorf("id of %d digits runs past the end of the line: %q", count, truncate(line, 80))
	}
	if line[end] != ' ' {
		return "", "", fmt.Errorf("id of %d digits does not end at a space: %q", count, truncate(line, 80))
	}
	return line[3:end], line[end+1:], nil
}

// ParseLine parses a wire line into id, verb, and payload.
// Format: #<len>:<id> <verb> [<payload>].
func ParseLine(line []byte) (id uint64, verb string, payload []byte, err error) {
	digits, rest, err := cutID(string(line))
	if err != nil {
		return 0, "", nil, err
	}

	id, err = strconv.ParseUint(digits, 10, 64)
	if err != nil {
		return 0, "", nil, fmt.Errorf("invalid id %q: %w", digits, err)
	}

	if rest == "" {
		return 0, "", nil, fmt.Errorf("line has no verb after id %d", id)
	}

	// Extract verb and optional payload
	verb, payloadStr, _ := strings.Cut(rest, " ")
	if payloadStr != "" {
		payload = []byte(payloadStr)
	}

	return id, verb, payload, nil
}

// AppendRequest appends a request line (#<len>:<id> <method> [<json>]) to buf
// and returns the extended slice. Newline is NOT appended. Callers on
// the hot path should supply a pool buffer; tests and one-shot callers
// can pass nil.
func AppendRequest(buf []byte, id uint64, method string, params json.RawMessage) []byte {
	buf = appendID(buf, id)
	buf = append(buf, method...)
	if len(params) == 0 || string(params) == "null" {
		return buf
	}
	buf = append(buf, ' ')
	buf = append(buf, params...)
	return buf
}

// AppendResult appends a success response line (#<len>:<id> ok [<json>]) to
// buf. Newline is NOT appended.
func AppendResult(buf []byte, id uint64, result json.RawMessage) []byte {
	buf = appendID(buf, id)
	if len(result) == 0 || string(result) == "null" {
		return append(buf, 'o', 'k')
	}
	buf = append(buf, 'o', 'k', ' ')
	buf = append(buf, result...)
	return buf
}

// AppendOK appends an empty success response line (#<len>:<id> ok) to buf.
// Newline is NOT appended.
func AppendOK(buf []byte, id uint64) []byte {
	return append(appendID(buf, id), 'o', 'k')
}

// AppendError appends an error response line (#<len>:<id> error [<json>]) to
// buf. Newline is NOT appended.
func AppendError(buf []byte, id uint64, errPayload json.RawMessage) []byte {
	buf = appendID(buf, id)
	if len(errPayload) == 0 {
		return append(buf, "error"...)
	}
	buf = append(buf, "error "...)
	buf = append(buf, errPayload...)
	return buf
}

// Answer lines. Every answer is a head, zero or more records, and a
// terminator, whatever its record count, so a reader follows one path and
// nothing declares a shape the payload can contradict. Each line is
// `#<len>:<id> <kind> <tail>`, where the kind states what the line IS and the
// tail is bare key=value pairs, so a reader decides how to read the answer
// without a JSON decoder.

// Answer line kinds. The token after the id field states what the line is. A
// reader knows that before it reads one byte of the tail, and derives it from
// nothing.
//
// Every kind is a whole word rather than a stump, because a person reads a
// captured session by eye. Byte 0 is distinct inside the set, so a machine
// switches on one load. The two are not traded against each other.
const (
	// AnswerKindHead opens the answer and states how its records are read.
	AnswerKindHead = "top"
	// AnswerKindRecord carries one row the command produced.
	AnswerKindRecord = "row"
	// AnswerKindFault carries one row the command rejected. The walk goes on,
	// which is what lets one answer report 97 rows applied and 3 rejected.
	AnswerKindFault = "bad"
	// AnswerKindTerminator ends the answer and carries its counts.
	AnswerKindTerminator = "end"
	// AnswerKindNotUnderstood is the whole answer to a command text naming no
	// command: one line, no head and no terminator, because nothing ran.
	AnswerKindNotUnderstood = "nay"
)

// answerKindWidth is the width of every kind token, and it is what makes the
// field arithmetic. The id field ends at the space that closes it. The kind is
// the three bytes after that space, and the tail starts one byte past the kind.
// The exec channel writes no id, so the kind sits at offset zero there.
const answerKindWidth = 3

// answerKinds is every kind a line can state, in the order the line table
// publishes them. answerKindAt reads it and a refusal names it, so the
// vocabulary a reader accepts and the vocabulary an error reports are one list.
var answerKinds = []string{
	AnswerKindHead,
	AnswerKindRecord,
	AnswerKindFault,
	AnswerKindTerminator,
	AnswerKindNotUnderstood,
}

// answerKindWords spells the kinds for a refusal, derived from the list a
// reader accepts rather than written out a second time beside it.
var answerKindWords = strings.Join(answerKinds, ", ")

// answerKindAt reads the kind token at the front of body and reports whether
// body opens with one.
//
// The token is a fixed width, so it is taken by arithmetic. No line is searched
// for a separator. The byte after the token MUST be the space that starts the
// tail, or the token MUST be the whole body. Without that check a method name
// opening with three kind bytes would be read as an answer line.
//
// It is the one reader of the token. The mux channel carries answers, plain
// responses and inbound requests on one wire. A body this refuses is one of the
// other two, so the caller falls through to the verb its family cuts (A-3). The
// exec channel carries answers alone, so a refusal there ends the line rather
// than routing it (R-5).
func answerKindAt(body string) (string, bool) {
	if len(body) < answerKindWidth {
		return "", false
	}
	if len(body) > answerKindWidth && body[answerKindWidth] != ' ' {
		return "", false
	}
	kind := body[:answerKindWidth]
	if !slices.Contains(answerKinds, kind) {
		return "", false
	}
	return kind, true
}

// Answer tail key names. The appenders below write these and ParseAnswerTail
// reads them, so a key cannot be spelled one way on the wire and another way in
// the reader. answerKeyEnvelope is spelled `key` on the wire and names the
// envelope the records belong under.
const (
	answerKeyStatus   = "status"
	answerKeyType     = "type"
	answerKeyEnvelope = "key"
	answerKeyFields   = "fields"
	answerKeyItem     = "item"
	answerKeyFault    = "fault"
	answerKeyCount    = "count"
	answerKeyFaults   = "faults"
	answerKeyMessage  = "message"
	answerKeyCode     = "code"
)

// Answer types. The head's type= states how a reader takes every item= that
// follows, so a consumer needs no first-byte test and no shape heuristic.
//
// AnswerTypeJSON carries the whole answer as ONE JSON document in one item,
// which is the answer a bounded command has always produced. AnswerTypeNDJSON
// carries one self-describing object per item. AnswerTypeStream carries one
// positional array per item, read against the head's fields=, which is what
// takes the repeated keys off a long answer with a fixed schema.
//
// The type is decided from the OUTPUT as the answer is written, never by the
// command: a handler produces records and states none of this.
const (
	AnswerTypeJSON   = "json"
	AnswerTypeNDJSON = "ndjson"
	AnswerTypeStream = "stream"
)

// AnswerBufferThreshold is how many records a producer holds while it decides
// which type an answer is. A walk that ends at or under it is answered as one
// AnswerTypeJSON document, which is the JSON a command answered with before it
// produced records at all; a walk that passes it is streamed, and the records
// already held go out in walk order ahead of the rest.
//
// It bounds what a producer holds for an answer nobody wants whole, which is
// the memory this protocol exists to stop growing, and it is of the same order
// as the queue a consumer reads an answer through (answerQueueDepth, mux.go):
// both bound one answer in flight, so one number covers the pair.
//
// It lives here rather than beside either producer because two producers decide
// it: the plugin connection's encoder and the SSH exec channel's renderer. One
// number is what makes their heads agree about what a bounded answer is.
//
// It is a constant and not a config knob. An operator who tuned it would be
// choosing the wire shape of every command's answer for every consumer at once,
// and the two shapes carry the same data, so there is nothing for a deployment
// to prefer.
const AnswerBufferThreshold = 256

// AnswerNoID is the id of an answer on a channel that carries exactly one
// answer, which is the SSH exec channel: one command owns the channel, so
// nothing needs to be told apart and no #<len>:<id> is written. Every other answer
// travels on the multiplexed plugin connection, whose ids start at 1
// (Conn.idSeq), so no real id collides with it.
const AnswerNoID uint64 = 0

// AppendAnswerHead appends an answer head line
// (#<len>:<id> top status=<status> type=<answerType> [key=<key>] [fields=<fields>]) to
// buf and returns the extended slice. Newline is NOT appended.
//
// status is StatusDone or StatusError, and it states what the daemon knows when
// the answer opens, so a consumer that renders a failure differently commits to
// a rendering on the first line rather than buffering the whole answer.
//
// answerType is AnswerTypeJSON, AnswerTypeNDJSON or AnswerTypeStream, and every
// head carries one: a reader that meets a head without it refuses the answer
// rather than guessing how to read the items.
//
// key names the envelope the records belong under, and an empty key writes no
// key= at all. fields is the JSON array of column names an AnswerTypeStream
// answer's positional rows are read against, already encoded by the caller. It
// is written last, because it runs to the end of the line, and the caller
// leaves it empty for every other type.
func AppendAnswerHead(buf []byte, id uint64, status, answerType, key string, fields json.RawMessage) []byte {
	buf = appendAnswerPrefix(buf, id, AnswerKindHead)
	buf = appendAnswerKey(buf, answerKeyStatus)
	buf = append(buf, status...)
	buf = appendAnswerKey(buf, answerKeyType)
	buf = append(buf, answerType...)
	if key != "" {
		buf = appendAnswerKey(buf, answerKeyEnvelope)
		buf = append(buf, key...)
	}
	if len(fields) == 0 {
		return buf
	}
	buf = appendAnswerKey(buf, answerKeyFields)
	start := len(buf)
	buf = append(buf, fields...)
	return replaceNewlines(buf, start)
}

// AppendAnswerItem appends a result record line (#<len>:<id> row item=<json>)
// and returns the extended slice. Newline is NOT appended. item takes the rest
// of the line verbatim, so a JSON value holding `=` or a space needs no
// quoting and no escaping.
func AppendAnswerItem(buf []byte, id uint64, item json.RawMessage) []byte {
	buf = appendAnswerPrefix(buf, id, AnswerKindRecord)
	buf = appendAnswerKey(buf, answerKeyItem)
	start := len(buf)
	buf = append(buf, item...)
	return replaceNewlines(buf, start)
}

// AppendAnswerFault appends an error record line (#<len>:<id> bad fault=<json>)
// and returns the extended slice. Newline is NOT appended. A fault records one
// rejected row and does not end the walk, which is what lets an answer report
// 97 rows applied and 3 rejected.
func AppendAnswerFault(buf []byte, id uint64, fault json.RawMessage) []byte {
	buf = appendAnswerPrefix(buf, id, AnswerKindFault)
	buf = appendAnswerKey(buf, answerKeyFault)
	start := len(buf)
	buf = append(buf, fault...)
	return replaceNewlines(buf, start)
}

// answerRecordPrefixMax is the capacity the record-line prefix is measured in:
// the id field at its widest (idPrefixMax, 24 bytes), the kind token
// (answerKindWidth), one space, the longest record key (answerKeyFault), and
// one `=`. That is 34 bytes, so the scratch never grows.
const answerRecordPrefixMax = idPrefixMax + answerKindWidth + 7

// AnswerRecordLineSize reports how many bytes the record line for record
// occupies under id, its newline terminator excluded. It measures the line
// AppendAnswerItem writes for a result record and the line AppendAnswerFault
// writes for a rejected one, so a producer can refuse a record wider than
// MaxMessageSize before it builds the line rather than after.
//
// The prefix is measured by the appenders that write it, and the value reaches
// the wire byte for byte (replaceNewlines overwrites in place and changes no
// length), so this size is the line's size rather than an estimate of it.
//
// A record carrying neither an item nor a fault measures as an empty item. Its
// producer refuses it on its own account (ErrEmptyAnswerRecord, collapse.go),
// and an empty record fits every line.
func AnswerRecordLineSize(id uint64, record Record) int {
	kind, key, value := AnswerKindRecord, answerKeyItem, record.Item
	if len(record.Fault) > 0 {
		kind, key, value = AnswerKindFault, answerKeyFault, record.Fault
	}
	var scratch [answerRecordPrefixMax]byte
	return len(appendAnswerKey(appendAnswerPrefix(scratch[:0], id, kind), key)) + len(value)
}

// AppendAnswerTerminator appends the terminator line
// (#<len>:<id> end count=<count> [faults=<faults>] [message=<message>]) to buf and
// returns the extended slice. Newline is NOT appended. The kind says the line
// ends the answer. Head and terminator are told apart by the token in front of
// the tail rather than by a key inside it. It states no status: the verdict is
// derived from the counts it carries, which is one source of truth instead of
// two that can disagree. count counts result records, faults counts rejected
// ones, and message states that the walk aborted after count records. A faults
// of 0 and an empty message write no key, so their absence is what states them.
func AppendAnswerTerminator(buf []byte, id, count, faults uint64, message string) []byte {
	buf = appendAnswerPrefix(buf, id, AnswerKindTerminator)
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
// understand (#<len>:<id> nay [code=<code>] message=<message>) to buf and returns
// the extended slice. Newline is NOT appended. It is the whole answer for its
// id: the kind says the conversation was valid and the command was not, which
// is what lets a client offer completion here and an operational message for a
// command that was understood and then failed.
func AppendAnswerNotUnderstood(buf []byte, id uint64, code, message string) []byte {
	buf = appendAnswerPrefix(buf, id, AnswerKindNotUnderstood)
	if code != "" {
		buf = appendAnswerKey(buf, answerKeyCode)
		buf = append(buf, code...)
	}
	buf = appendAnswerKey(buf, answerKeyMessage)
	start := len(buf)
	buf = append(buf, message...)
	return replaceNewlines(buf, start)
}

// appendAnswerPrefix appends the `#<len>:<id> <kind>` every answer line opens
// with. It is the one writer of the kind token, and answerKindAt the one
// reader, so no line spells a kind the reader does not know.
func appendAnswerPrefix(buf []byte, id uint64, kind string) []byte {
	buf = appendAnswerID(buf, id)
	return append(buf, kind...)
}

// appendAnswerID appends the `#<len>:<id> ` that names which answer a line
// belongs to, and appends nothing for AnswerNoID. A muxed connection
// interleaves the answers of many callers and needs the id on every line; the
// SSH exec channel carries one answer and would be stating a fact with one
// possible value.
//
// The field is the one appendID writes for a request, so both directions of the
// protocol spell an id the same way. Everything after it is the same grammar on
// both channels, so a reader parses one kind and one tail whichever channel it
// came from.
func appendAnswerID(buf []byte, id uint64) []byte {
	if id == AnswerNoID {
		return buf
	}
	return appendID(buf, id)
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

// AnswerTail is one answer line: the kind the line stated, and its decoded
// key=value tail. A consumer reads it without a JSON decoder and needs no
// state at all, because the kind says what the line is before the tail is
// touched.
//
// Item and Fault reference the payload they were parsed from rather than
// copying it, so a consumer that forwards a record parses and copies nothing.
type AnswerTail struct {
	// Kind is the token the line opened with: AnswerKindHead through
	// AnswerKindNotUnderstood. It is READ off the wire, never derived from the
	// keys below, so no consumer parses a tail to learn what its line is.
	Kind string
	// Status is the head's status=: StatusDone or StatusError. A terminator
	// states none.
	Status string
	// Type is the head's type=: AnswerTypeJSON, AnswerTypeNDJSON or
	// AnswerTypeStream. It states how every item= of this answer is read, and
	// every head carries it.
	Type string
	// Key is the head's key=, the envelope the records belong under. It is
	// empty when the head names none.
	Key string
	// Fields is the head's fields=, the column names an AnswerTypeStream
	// answer's positional rows are read against, in column order. It is empty
	// for every other type.
	Fields []string
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
}

// ParseAnswerTail decodes the tail of an answer line of kind into its keys.
// kind is the token the line opened with and payload is everything after it.
//
// An unknown kind is refused rather than guessed at, because the kind states
// which payload follows and a guess reads a fault as a result (R-5, AC-7).
//
// A key belonging to another kind is refused too, and so is an unknown key. A
// line this build cannot fully read is reported instead of half-read, and a
// line stating two things is the disagreement the kind exists to remove.
func ParseAnswerTail(kind string, payload []byte) (AnswerTail, error) {
	if _, known := answerKindAt(kind); !known {
		return AnswerTail{}, fmt.Errorf("answer line states kind %q, want one of %s", truncate(kind, answerKindWidth), answerKindWords)
	}
	tail := AnswerTail{Kind: kind}
	counted := false

	// Each pass consumes at least a key and its =, so the loop is bounded by
	// the payload length, which the frame reader has already bounded.
	rest := payload
	for len(rest) > 0 {
		name, value, remainder, err := answerTailToken(rest)
		if err != nil {
			return AnswerTail{}, err
		}
		rest = remainder

		if answerKeyMisplaced(name, kind) {
			return AnswerTail{}, fmt.Errorf("answer %s line carries %q, a key of another kind", kind, truncate(string(name), 40))
		}

		switch string(name) {
		case answerKeyStatus:
			tail.Status = string(value)
		case answerKeyType:
			tail.Type = string(value)
		case answerKeyEnvelope:
			tail.Key = string(value)
		case answerKeyFields:
			if fieldsErr := json.Unmarshal(value, &tail.Fields); fieldsErr != nil {
				return AnswerTail{}, fmt.Errorf("answer tail fields=%q: %w", truncate(string(value), 40), fieldsErr)
			}
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
			counted = true
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

	if kind == AnswerKindTerminator && !counted {
		return AnswerTail{}, errors.New("answer terminator states no count=, so nothing states how far the walk got")
	}
	if err := checkAnswerType(kind, &tail); err != nil {
		return AnswerTail{}, err
	}
	return tail, nil
}

// answerKeyMisplaced reports whether name is a key belonging to a kind other
// than this line's. The kind states what the line is. A key of another kind is
// one line stating two things, and the reader refuses it rather than reading
// one of them.
//
// message= is the one key two kinds carry: the operational text of an aborted
// walk and the reason a command was not understood are the same field.
//
// A name no kind claims is NOT misplaced. ParseAnswerTail refuses it as an
// unknown key, which is the distinct failure of a line this build cannot read.
func answerKeyMisplaced(name []byte, kind string) bool {
	switch string(name) {
	case answerKeyStatus, answerKeyType, answerKeyEnvelope, answerKeyFields:
		return kind != AnswerKindHead
	case answerKeyItem:
		return kind != AnswerKindRecord
	case answerKeyFault:
		return kind != AnswerKindFault
	case answerKeyCount, answerKeyFaults:
		return kind != AnswerKindTerminator
	case answerKeyCode:
		return kind != AnswerKindNotUnderstood
	case answerKeyMessage:
		return kind != AnswerKindTerminator && kind != AnswerKindNotUnderstood
	}
	return false
}

// ParseAnswerLine reads one whole answer line from a channel that carries no
// id (AnswerNoID), which is the SSH exec channel: the kind at offset zero, then
// the tail that kind carries. It is the reader for what appendAnswerID writes
// nothing in front of. The grammar of that channel therefore has one producer
// and one consumer, rather than a split rule each end keeps its own copy of.
//
// The kind is a fixed width, so it is taken by arithmetic and the line is
// searched for nothing (AC-4). A line opening with anything else is refused.
// Text that is not an answer at all is then named rather than read as an empty
// one.
func ParseAnswerLine(line []byte) (string, AnswerTail, error) {
	text := string(line)
	kind, known := answerKindAt(text)
	if !known {
		return "", AnswerTail{}, fmt.Errorf("answer line states no kind this build knows, want one of %s: %q", answerKindWords, truncate(text, 40))
	}
	if len(text) == answerKindWidth {
		return "", AnswerTail{}, fmt.Errorf("answer %s line carries no tail", kind)
	}
	tail, err := ParseAnswerTail(kind, line[answerKindWidth+1:])
	if err != nil {
		return "", AnswerTail{}, err
	}
	return kind, tail, nil
}

// checkAnswerType refuses a head whose type= and fields= disagree with each
// other. Each refusal is a line this build cannot read the BODY of, so it is
// named rather than read with a default filled in: a head with no type= would
// have its items guessed at, and a fields= a reader ignored would leave a
// positional row decoding as data it is not.
//
// Only a head states either key, and answerKeyMisplaced has already refused one
// anywhere else, so this reads the kind rather than deriving a head from the
// presence of status=.
func checkAnswerType(kind string, tail *AnswerTail) error {
	if kind != AnswerKindHead {
		return nil
	}
	if tail.Type == "" {
		return errors.New("answer head states no type=, so a consumer cannot read the items that follow it")
	}

	switch tail.Type {
	case AnswerTypeJSON, AnswerTypeNDJSON:
		if len(tail.Fields) > 0 {
			return fmt.Errorf("answer head states fields= with type=%q: only type=%s reads its rows against them", truncate(tail.Type, 40), AnswerTypeStream)
		}
		return nil
	case AnswerTypeStream:
		if len(tail.Fields) == 0 {
			return fmt.Errorf("answer head states type=%s and no fields=, so its positional rows have no schema", AnswerTypeStream)
		}
		return nil
	default:
		return fmt.Errorf("answer head states unknown type=%q", truncate(tail.Type, 40))
	}
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
// fields= joins the three because a column name can hold a space, and the head
// writes it last for that reason.
func isOpenEndedKey(name []byte) bool {
	switch string(name) {
	case answerKeyItem, answerKeyFault, answerKeyMessage, answerKeyFields:
		return true
	}
	return false
}

// Verdict derives an answer's outcome from its terminator, which is the only
// place that outcome is stated: a terminator carries no status=, so nothing can
// disagree with the counts. terminator is nil when none arrived, and a line of
// any other kind is read the same way, because both mean the answer stopped
// before it ended and a consumer must not read what it got as complete.
func Verdict(terminator *AnswerTail) string {
	if terminator == nil || terminator.Kind != AnswerKindTerminator {
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

// FormatRequest returns a request line (#<len>:<id> <method> [<json>]) in a
// freshly-allocated slice. Retained for tests and low-rate callers;
// hot-path senders should use AppendRequest with a pool buffer.
func FormatRequest(id uint64, method string, params json.RawMessage) []byte {
	return AppendRequest(make([]byte, 0, idPrefixMax+len(method)+1+len(params)), id, method, params)
}

// FormatOK returns an empty success response in a freshly-allocated
// slice. Retained for tests; hot-path senders should use AppendOK.
func FormatOK(id uint64) []byte {
	return AppendOK(make([]byte, 0, idPrefixMax+2), id)
}

// FormatError returns an error response in a freshly-allocated slice.
// Retained for tests; hot-path senders should use AppendError.
func FormatError(id uint64, errPayload json.RawMessage) []byte {
	return AppendError(make([]byte, 0, idPrefixMax+6+len(errPayload)), id, errPayload)
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
	if tail, tailErr := ParseAnswerTail(AnswerKindNotUnderstood, payload); tailErr == nil {
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
