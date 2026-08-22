// Design: docs/architecture/api/ipc_protocol.md — RPC wire message types
// Related: conn.go — Conn uses line format for RPC framing
// Related: framing.go — newline-delimited frame reader/writer
// Related: types.go — domain-specific RPC input/output types

package rpc

import (
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

// A counted field states its own width: one base-36 length character, a colon,
// and then exactly what the length names. A reader takes the length byte and
// reaches the field after it by addition, so no line is searched for a
// separator. Two fields are built from it, and the protocol has no third
// variable-width shape:
//
//	counted number  `<len>:<digits>`     the decimal spelling of a uint64
//	counted text    `<len>:<n>:<bytes>`  a counted number stating the byte count
//	                                     of the bytes that follow it
//
// A uint64 occupies 20 decimal digits at most, and a byte count is a uint64, so
// one length character expresses every counted number this protocol writes,
// with room left. A counted text therefore carries a value of any length, and no
// field is capped by its own encoding.
//
// The id field is a counted number under a `#` sigil, closed by the space that
// separates it from the field after it (appendID, cutID).

// appendCountedNumber appends the `<len>:<digits>` field carrying value.
//
// The digits are formatted once and the length is read off the very bytes that
// reach the wire, so the two halves of the field cannot disagree (R-2).
func appendCountedNumber(buf []byte, value uint64) []byte {
	var scratch [idDigitsMax]byte
	digits := strconv.AppendUint(scratch[:0], value, 10)
	buf = append(buf, idLengthAlphabet[len(digits)], ':')
	return append(buf, digits...)
}

// appendCountedText appends the `<len>:<n>:<bytes>` field carrying text. An
// empty text writes `1:0:`, so the field is present and empty rather than
// omitted and the field count of a line never varies (AC-17).
func appendCountedText(buf []byte, text string) []byte {
	buf = appendCountedNumber(buf, uint64(len(text)))
	buf = append(buf, ':')
	return append(buf, text...)
}

// appendCountedBytes is appendCountedText for a value already held as bytes,
// so a JSON payload reaches the line without being copied into a string first.
func appendCountedBytes(buf, value []byte) []byte {
	buf = appendCountedNumber(buf, uint64(len(value)))
	buf = append(buf, ':')
	return append(buf, value...)
}

// cutCountedDigits takes a counted number off the front of line and returns its
// decimal digits and everything after the field.
//
// It is the one reader of the shape appendCountedNumber writes, and every
// refusal a malformed length earns is stated here rather than at each field.
func cutCountedDigits(line string) (digits, rest string, err error) {
	if len(line) < 2 {
		return "", "", fmt.Errorf("counted field states no length: %q", truncate(line, 80))
	}
	count, known := idLengthValue(line[0])
	if !known {
		return "", "", fmt.Errorf("counted field length %q is not one base-36 character, 0 to 9 then A to Z", line[0:1])
	}
	if count == 0 {
		return "", "", fmt.Errorf("counted field states a length of zero digits: %q", truncate(line, 80))
	}
	if count > idDigitsMax {
		return "", "", fmt.Errorf("counted field states %d digits, past the %d a uint64 occupies", count, idDigitsMax)
	}
	if line[1] != ':' {
		return "", "", fmt.Errorf("counted field length is not followed by ':': %q", truncate(line, 80))
	}
	end := 2 + count
	if len(line) < end {
		return "", "", fmt.Errorf("counted field of %d digits runs past the end of the line: %q", count, truncate(line, 80))
	}
	return line[2:end], line[end:], nil
}

// cutCountedNumber takes a counted number off the front of line and returns its
// value and everything after the field.
func cutCountedNumber(line string) (value uint64, rest string, err error) {
	digits, rest, err := cutCountedDigits(line)
	if err != nil {
		return 0, "", err
	}
	value, err = strconv.ParseUint(digits, 10, 64)
	if err != nil {
		return 0, "", fmt.Errorf("counted number %q: %w", digits, err)
	}
	return value, rest, nil
}

// cutCountedText takes a counted text off the front of line and returns its
// bytes and everything after the field.
//
// The stated byte count is checked against what arrived BEFORE the slice, so a
// line claiming more bytes than it carries is refused rather than panicking the
// reader.
func cutCountedText(line string) (text, rest string, err error) {
	size, rest, err := cutCountedNumber(line)
	if err != nil {
		return "", "", err
	}
	if rest == "" || rest[0] != ':' {
		return "", "", fmt.Errorf("counted text length is not followed by ':': %q", truncate(line, 80))
	}
	rest = rest[1:]
	if uint64(len(rest)) < size {
		return "", "", fmt.Errorf("counted text of %d bytes runs past the end of the line: %q", size, truncate(line, 80))
	}
	return rest[:size], rest[size:], nil
}

// cutFieldSeparator takes the one space that separates two fields off the front
// of rest. A field whose width is stated needs no terminator of its own, so the
// space is what says another field follows rather than what says this one
// ended.
func cutFieldSeparator(rest string) (string, error) {
	if rest == "" {
		return "", errors.New("answer line ends where another field belongs")
	}
	if rest[0] != ' ' {
		return "", fmt.Errorf("answer line field is not followed by a space: %q", truncate(rest, 40))
	}
	return rest[1:], nil
}

// appendID appends the `#<len>:<id> ` field naming which conversation a line
// belongs to. Requests, responses and answer lines open with it alike, so the
// protocol carries ONE id encoding rather than one for each direction.
func appendID(buf []byte, id uint64) []byte {
	buf = append(buf, '#')
	buf = appendCountedNumber(buf, id)
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
	digits, rest, err = cutCountedDigits(line[1:])
	if err != nil {
		return "", "", fmt.Errorf("line states no readable id: %w", err)
	}
	if rest == "" || rest[0] != ' ' {
		return "", "", fmt.Errorf("id of %d digits does not end at a space: %q", len(digits), truncate(line, 80))
	}
	return digits, rest[1:], nil
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
// tail is the positional fields that kind carries, so a reader decides how to
// read the answer without a JSON decoder and without reading a key name.

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

// Answer item types. The head's item type states what each record of this
// answer IS, so a consumer needs no first-byte test and no shape heuristic.
//
// AnswerTypeDocument carries the whole answer as ONE JSON document in one
// record, which is the answer a bounded command has always produced.
// AnswerTypeMap carries one map of names to values per record. AnswerTypeTable
// carries one tabular row per record, read against the head's column names,
// which is what takes the repeated names off a long answer with a fixed schema.
//
// Each token is a whole word rather than a stump, and each says what a record IS
// rather than naming a serialization. That is also what ends the collision
// between a wire type and the `| json` pipe operator, which are unrelated and
// used to share a word.
//
// The type is decided from the OUTPUT as the answer is written, never by the
// command: a handler produces records and states none of this.
const (
	AnswerTypeDocument = "doc"
	AnswerTypeMap      = "map"
	AnswerTypeTable    = "tab"
)

// answerTypeWidth is the width of every item type token, and it is what puts
// the field after it at a computed offset. It is the kind token's width for the
// same reason: one word, one load, no search.
const answerTypeWidth = 3

// answerTypes is every item type a head can state, in the order the line table
// publishes them. The head reader and a refusal both read this list, so the
// vocabulary a reader accepts and the vocabulary an error reports are one list.
var answerTypes = []string{
	AnswerTypeDocument,
	AnswerTypeMap,
	AnswerTypeTable,
}

// answerTypeWords spells the item types for a refusal, derived from the list a
// reader accepts rather than written out a second time beside it.
var answerTypeWords = strings.Join(answerTypes, ", ")

// AnswerBufferThreshold is how many records a producer holds while it decides
// which type an answer is. A walk that ends at or under it is answered as one
// AnswerTypeDocument document, which is the JSON a command answered with before it
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
// (#<len>:<id> top <type> <len>:<n>:<key> <len>:<n>:<columns>) to buf and
// returns the extended slice. Newline is NOT appended.
//
// Its fields are positional: the head states no key name, so a reader reaches
// each one by arithmetic and nothing is spelled twice on the wire. It states no
// status either. The terminator carries the whole outcome, and two lines
// stating one outcome can disagree.
//
// answerType is AnswerTypeDocument, AnswerTypeMap or AnswerTypeTable, and every
// head carries one: a reader that meets a head without it refuses the answer
// rather than guessing how to read the records.
//
// key names the envelope the records belong under, and columns is the JSON array
// of column names an AnswerTypeTable answer's positional rows are read against,
// already encoded by the caller. Each is a counted field, so an empty one is
// present and empty rather than omitted and the head's field count never varies.
func AppendAnswerHead(buf []byte, id uint64, answerType, key string, columns json.RawMessage) []byte {
	buf = appendAnswerPrefix(buf, id, AnswerKindHead)
	buf = append(buf, ' ')
	buf = append(buf, answerType...)
	buf = append(buf, ' ')
	start := len(buf)
	buf = appendCountedText(buf, key)
	buf = append(buf, ' ')
	buf = appendCountedBytes(buf, columns)
	return replaceNewlines(buf, start)
}

// AppendAnswerItem appends a result record line (#<len>:<id> row <json>) and
// returns the extended slice. Newline is NOT appended. The kind states that the
// payload is a produced row, so no key name sits in front of it, and the payload
// takes the rest of the line verbatim.
func AppendAnswerItem(buf []byte, id uint64, item json.RawMessage) []byte {
	buf = appendAnswerPrefix(buf, id, AnswerKindRecord)
	buf = append(buf, ' ')
	start := len(buf)
	buf = append(buf, item...)
	return replaceNewlines(buf, start)
}

// AppendAnswerFault appends an error record line (#<len>:<id> bad <json>) and
// returns the extended slice. Newline is NOT appended. A fault records one
// rejected row and does not end the walk, which is what lets an answer report
// 97 rows applied and 3 rejected.
func AppendAnswerFault(buf []byte, id uint64, fault json.RawMessage) []byte {
	buf = appendAnswerPrefix(buf, id, AnswerKindFault)
	buf = append(buf, ' ')
	start := len(buf)
	buf = append(buf, fault...)
	return replaceNewlines(buf, start)
}

// answerRecordPrefixMax is the capacity the record-line prefix is measured in:
// the id field at its widest (idPrefixMax, 24 bytes), the kind token
// (answerKindWidth), and the one space that separates the kind from the
// payload. That is 28 bytes, so the scratch never grows.
const answerRecordPrefixMax = idPrefixMax + answerKindWidth + 1

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
	kind, value := AnswerKindRecord, record.Item
	if len(record.Fault) > 0 {
		kind, value = AnswerKindFault, record.Fault
	}
	var scratch [answerRecordPrefixMax]byte
	// The one space is what appendAnswerPrefix leaves to the payload's own
	// appender, so it is counted here rather than measured a second time.
	return len(appendAnswerPrefix(scratch[:0], id, kind)) + 1 + len(value)
}

// AppendAnswerTerminator appends the terminator line
// (#<len>:<id> end <len>:<count> <len>:<faults> <len>:<n>:<message>) to buf and
// returns the extended slice. Newline is NOT appended.
//
// The kind says the line ends the answer, and its fields are positional. It
// states no outcome of its own: the verdict is DERIVED from the counts it
// carries, which is one source of truth instead of two that can disagree.
//
// count counts result records, faults counts rejected ones, and message states
// why the walk produced fewer than it set out to. Each is a counted field, so a
// zero count and an empty message are present and empty rather than omitted, and
// the terminator's field count never varies.
func AppendAnswerTerminator(buf []byte, id, count, faults uint64, message string) []byte {
	buf = appendAnswerPrefix(buf, id, AnswerKindTerminator)
	buf = append(buf, ' ')
	buf = appendCountedNumber(buf, count)
	buf = append(buf, ' ')
	buf = appendCountedNumber(buf, faults)
	buf = append(buf, ' ')
	start := len(buf)
	buf = appendCountedText(buf, message)
	return replaceNewlines(buf, start)
}

// AppendAnswerNotUnderstood appends the answer to a command the daemon did not
// understand (#<len>:<id> nay <len>:<n>:<code> <len>:<n>:<message>) to buf and
// returns the extended slice. Newline is NOT appended.
//
// It is the whole answer for its id: the kind says the conversation was valid
// and the command was not, which is what lets a client offer completion here and
// an operational message for a command that was understood and then failed. An
// empty code is present and empty, so the line's field count never varies.
func AppendAnswerNotUnderstood(buf []byte, id uint64, code, message string) []byte {
	buf = appendAnswerPrefix(buf, id, AnswerKindNotUnderstood)
	buf = append(buf, ' ')
	start := len(buf)
	buf = appendCountedText(buf, code)
	buf = append(buf, ' ')
	buf = appendCountedText(buf, message)
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

// replaceNewlines overwrites every newline in buf from start onwards with a
// space. A line ends at the first newline the frame reader meets, so a newline
// inside a value would split one answer line into two and leave the reader
// taking the second half as a line of its own. A newline is insignificant
// whitespace between JSON tokens and can never appear inside a JSON string, so
// this cannot change what a record decodes to.
//
// It overwrites in place and changes no length, so a counted field's stated
// width still describes the bytes that reach the wire.
func replaceNewlines(buf []byte, start int) []byte {
	for i := start; i < len(buf); i++ {
		if buf[i] == '\n' || buf[i] == '\r' {
			buf[i] = ' '
		}
	}
	return buf
}

// AnswerFailureUnstated is the message a terminator carries for a producer that
// reported a failure and gave no reason for it.
//
// The terminator is the one line that states an outcome, and Verdict reads an
// empty message with zero counts as a completed answer. A failure that said
// nothing would therefore reach a consumer as a success, which is a zero value
// wearing a valid answer's clothes (ai/rules/evidence.md). Stating this instead
// fails closed: the consumer learns the command failed, and the producer that
// named no reason is what reads badly rather than the frame.
const AnswerFailureUnstated = "unknown error"

// Verdicts an answer derives from its terminator. VerdictDone and VerdictError
// are the status vocabulary a response envelope uses, so a consumer that
// rebuilds one from the answer says the same word for the same outcome.
const (
	VerdictDone      = StatusDone
	VerdictPartial   = "partial"
	VerdictError     = StatusError
	VerdictAborted   = "aborted"
	VerdictTruncated = "truncated"
)

// AnswerTail is one answer line: the kind the line stated, and the fields that
// kind carries. A consumer reads it without a JSON decoder and needs no state at
// all, because the kind says what the line is before the tail is touched.
//
// The fields are POSITIONAL on the wire. No key name reaches it, so each field
// is reached by arithmetic and nothing is spelled twice.
//
// Item and Fault reference the payload they were parsed from rather than
// copying it, so a consumer that forwards a record parses and copies nothing.
type AnswerTail struct {
	// Kind is the token the line opened with: AnswerKindHead through
	// AnswerKindNotUnderstood. It is READ off the wire, never derived from the
	// fields below, so no consumer parses a tail to learn what its line is.
	Kind string
	// Type is the head's item type: AnswerTypeDocument, AnswerTypeMap or
	// AnswerTypeTable. It states what every record of this answer IS, and every
	// head carries one.
	Type string
	// Key is the head's envelope name, which the records belong under. It is
	// empty when the head names none.
	Key string
	// Fields is the head's column names, which an AnswerTypeTable answer's
	// positional rows are read against, in column order. It is empty for every
	// other type.
	Fields []string
	// Item is a result record's payload, and Fault an error record's. One line
	// carries at most one of them, and the kind says which.
	Item  json.RawMessage
	Fault json.RawMessage
	// Count and Faults are the terminator's counts: result records produced,
	// and rows the walk rejected.
	Count  uint64
	Faults uint64
	// Message is the terminator's operational text, which states why a walk
	// produced fewer records than it set out to, or the not-understood answer's
	// reason. It is empty for a walk that ran to its end with nothing to say.
	Message string
	// Code is the not-understood answer's error code.
	Code string
}

// ParseAnswerTail decodes the tail of an answer line of kind into the fields
// that kind carries. kind is the token the line opened with and payload is
// everything after it.
//
// The fields are POSITIONAL: the kind states which fields follow and in what
// order, so each is reached by arithmetic and no key name is read. An unknown
// kind is refused rather than guessed at, because the kind is what states which
// payload follows and a guess reads a fault as a result (R-5, AC-7).
//
// A line carrying bytes past its last field is refused too. A line this build
// cannot fully read is reported instead of half-read.
func ParseAnswerTail(kind string, payload []byte) (AnswerTail, error) {
	switch kind {
	case AnswerKindHead:
		return parseAnswerHead(payload)
	case AnswerKindRecord:
		return AnswerTail{Kind: kind, Item: payload}, nil
	case AnswerKindFault:
		return AnswerTail{Kind: kind, Fault: payload}, nil
	case AnswerKindTerminator:
		return parseAnswerTerminator(payload)
	case AnswerKindNotUnderstood:
		return parseAnswerNotUnderstood(payload)
	}
	return AnswerTail{}, fmt.Errorf("answer line states kind %q, want one of %s", truncate(kind, answerKindWidth), answerKindWords)
}

// parseAnswerHead reads the head's three fields: the item type, the envelope
// name, and the column names. Each sits at an offset the field before it states,
// so the line is searched for nothing.
func parseAnswerHead(payload []byte) (AnswerTail, error) {
	text := string(payload)
	if len(text) < answerTypeWidth {
		return AnswerTail{}, errors.New("answer head states no item type, so a consumer cannot read the records that follow it")
	}
	tail := AnswerTail{Kind: AnswerKindHead, Type: text[:answerTypeWidth]}
	if !slices.Contains(answerTypes, tail.Type) {
		return AnswerTail{}, fmt.Errorf("answer head states item type %q, want one of %s", truncate(tail.Type, answerTypeWidth), answerTypeWords)
	}

	rest, err := cutFieldSeparator(text[answerTypeWidth:])
	if err != nil {
		return AnswerTail{}, fmt.Errorf("answer head item type: %w", err)
	}
	key, rest, err := cutCountedText(rest)
	if err != nil {
		return AnswerTail{}, fmt.Errorf("answer head envelope name: %w", err)
	}
	tail.Key = key

	rest, err = cutFieldSeparator(rest)
	if err != nil {
		return AnswerTail{}, fmt.Errorf("answer head envelope name: %w", err)
	}
	columns, rest, err := cutCountedText(rest)
	if err != nil {
		return AnswerTail{}, fmt.Errorf("answer head column names: %w", err)
	}
	if rest != "" {
		return AnswerTail{}, fmt.Errorf("answer head carries %d bytes past its last field: %q", len(rest), truncate(rest, 40))
	}
	if columns != "" {
		if err := json.Unmarshal([]byte(columns), &tail.Fields); err != nil {
			return AnswerTail{}, fmt.Errorf("answer head column names %q: %w", truncate(columns, 40), err)
		}
	}
	if err := checkAnswerType(&tail); err != nil {
		return AnswerTail{}, err
	}
	return tail, nil
}

// parseAnswerTerminator reads the terminator's three fields: the records the
// walk produced, the rows it rejected, and why it produced fewer than it set out
// to.
func parseAnswerTerminator(payload []byte) (AnswerTail, error) {
	tail := AnswerTail{Kind: AnswerKindTerminator}

	count, rest, err := cutCountedNumber(string(payload))
	if err != nil {
		return AnswerTail{}, fmt.Errorf("answer terminator count: %w", err)
	}
	tail.Count = count

	rest, err = cutFieldSeparator(rest)
	if err != nil {
		return AnswerTail{}, fmt.Errorf("answer terminator count: %w", err)
	}
	faults, rest, err := cutCountedNumber(rest)
	if err != nil {
		return AnswerTail{}, fmt.Errorf("answer terminator fault count: %w", err)
	}
	tail.Faults = faults

	rest, err = cutFieldSeparator(rest)
	if err != nil {
		return AnswerTail{}, fmt.Errorf("answer terminator fault count: %w", err)
	}
	message, rest, err := cutCountedText(rest)
	if err != nil {
		return AnswerTail{}, fmt.Errorf("answer terminator message: %w", err)
	}
	if rest != "" {
		return AnswerTail{}, fmt.Errorf("answer terminator carries %d bytes past its last field: %q", len(rest), truncate(rest, 40))
	}
	tail.Message = message
	return tail, nil
}

// parseAnswerNotUnderstood reads the not-understood answer's two fields: the
// error code and the reason.
func parseAnswerNotUnderstood(payload []byte) (AnswerTail, error) {
	tail := AnswerTail{Kind: AnswerKindNotUnderstood}

	code, rest, err := cutCountedText(string(payload))
	if err != nil {
		return AnswerTail{}, fmt.Errorf("answer not-understood code: %w", err)
	}
	tail.Code = code

	rest, err = cutFieldSeparator(rest)
	if err != nil {
		return AnswerTail{}, fmt.Errorf("answer not-understood code: %w", err)
	}
	message, rest, err := cutCountedText(rest)
	if err != nil {
		return AnswerTail{}, fmt.Errorf("answer not-understood message: %w", err)
	}
	if rest != "" {
		return AnswerTail{}, fmt.Errorf("answer not-understood answer carries %d bytes past its last field: %q", len(rest), truncate(rest, 40))
	}
	tail.Message = message
	return tail, nil
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

// checkAnswerType refuses a head whose item type and column names disagree with
// each other. Each refusal is a line this build cannot read the BODY of, so it
// is named rather than read with a default filled in: column names a reader
// ignored would leave a positional row decoding as data it is not, and a
// tabular answer with no schema has nothing to read its rows against.
//
// The type token itself is checked against the vocabulary before this runs, so
// this reads only the pair.
func checkAnswerType(tail *AnswerTail) error {
	switch tail.Type {
	case AnswerTypeDocument, AnswerTypeMap:
		if len(tail.Fields) > 0 {
			return fmt.Errorf("answer head names columns with item type %s: only %s reads its rows against them", tail.Type, AnswerTypeTable)
		}
	case AnswerTypeTable:
		if len(tail.Fields) == 0 {
			return fmt.Errorf("answer head states item type %s and names no columns, so its positional rows have no schema", AnswerTypeTable)
		}
	}
	return nil
}

// Verdict derives an answer's outcome from its terminator, which is the only
// place that outcome is stated: a terminator states no outcome of its own, so
// nothing can disagree with the counts. terminator is nil when none arrived,
// and a line of any other kind is read the same way, because both mean the
// answer stopped before it ended and a consumer must not read what it got as
// complete.
//
// A stated message and a count of zero is a command that failed before it
// produced anything, and the same message over records the walk did produce is
// a walk that stopped part way. The two are different outcomes to an operator:
// the first ran nothing, the second ran and the rows it wrote are real. The
// count is what tells them apart, so this reads it rather than reporting both
// as aborted (AC-11).
func Verdict(terminator *AnswerTail) string {
	if terminator == nil || terminator.Kind != AnswerKindTerminator {
		return VerdictTruncated
	}
	if terminator.Message != "" {
		if terminator.Count == 0 && terminator.Faults == 0 {
			return VerdictError
		}
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
	// A not-understood answer states its text as a positional tail rather than
	// as JSON, so the tail is read before the fallback below: without this the
	// operator reads the whole tail, its counted lengths included, as the
	// message.
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
