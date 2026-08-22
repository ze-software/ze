// Design: docs/architecture/api/ipc_protocol.md — RPC wire message types
// Related: conn.go — Conn uses line format for RPC framing
// Related: framing.go — newline-delimited frame reader/writer
// Related: types.go — domain-specific RPC input/output types

package rpc

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
)

// Request represents a parsed incoming RPC request line: #<id> <method> [<json>].
type Request struct {
	ID     uint64          // Correlation ID from the #<id> prefix
	Method string          // module:rpc-name
	Params json.RawMessage // JSON payload (may be nil)
}

// RPCCallError represents an error returned by the remote side via
// #<id> error [<json>].
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

// The id field every line carrying an id opens with: `#<id> `, the decimal
// spelling of the id, closed by one space.
//
// A digit run that a space terminates is unambiguous: nothing inside it can be
// the byte that ends it. So the id needs no count in front of it, and one fused
// loop accumulates the value while it walks to that space, which is ONE pass
// where a search for the space and a separate parse of what it found would be
// two (owner measurement, 2026-08-22: 3.2 ns against 8.6 ns for a counted id,
// and two bytes less on every line).
//
// A COUNT belongs on a field whose value can hold the delimiter, which is the
// record payload and every other counted field below. It never belonged here.
//
// uint64DigitsMax is the widest decimal spelling of a uint64, and the id is
// one. idPrefixMax is the whole id field at that width: `#`, twenty digits, and
// the space that ends the field.
const (
	uint64DigitsMax = 20
	idPrefixMax     = 22
)

// A variable-width field states its own width, and the field's TYPE says how.
// There are two, and the protocol has no third variable-width shape:
//
//	counted number  decimal digits, closed by a space or by the end of the line
//	counted text    decimal digits, then `:`, then exactly that many BYTES
//
// The count on a text is a BYTE count and never a count of characters. A value
// MAY hold multi-byte utf-8, and a reader slices the bytes that arrived rather
// than the text they decode to.
//
// The colon is what tells the two fields apart, and it is ALWAYS there on a
// text, an empty one included, which is written `0:`. So a number never carries
// one and a text always does. A reader that meets the wrong byte refuses the
// line rather than reading one field as the other, which would take the bytes
// after it as a value and mis-slice every field that follows (cutCountedNumber,
// cutCountedText).
//
// NEITHER field states an outer length of its own. A digit run is closed by a
// byte no digit can be, so a count in front of it buys a reader nothing: the
// reader still has to check the terminator and still has to parse the digits,
// which IS the whole cost of the plain form (owner measurement, 2026-08-22: 3.2
// ns against 8.6 ns for a counted length, zero allocations either way, and two
// bytes less on every field). The count that stays is the text's, and it states
// a different fact: it counts bytes that MAY hold the delimiter, so nothing else
// can say where such a value ends.
//
// The id field is a number under a `#` sigil, closed by the space that
// separates it from the field after it (appendID, cutID).

// appendCountedNumber appends the decimal digits carrying value. The space that
// separates this field from the next one closes it, and so does the end of the
// line, so the digits are the whole field.
func appendCountedNumber(buf []byte, value uint64) []byte {
	return strconv.AppendUint(buf, value, 10)
}

// appendCountedText appends the `<n>:<bytes>` field carrying text, where `<n>`
// is the BYTE count of what follows the colon. An empty text writes `0:`, so
// the field is present and empty rather than omitted and the field count of a
// line never varies (AC-17).
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

// The two refusals the colon earns. A number and a text differ by that one byte,
// so each field names it when it meets the other field's spelling: a message
// saying only that the line is malformed would leave a reader hunting for which
// of the two it holds.
var (
	errCountedNumberColon  = errors.New("counted number is closed by ':', which is a counted text's byte count")
	errCountedTextUnclosed = errors.New("counted text byte count is not closed by ':', which every text carries, an empty one included")
)

// countedDigitsText reports how many decimal digits open line, and
// countedDigitsBytes is its twin for a line held as bytes. The wire reaches the
// frame reader as bytes and the multiplexer as a string, and neither pays a
// conversion to reach one counter.
//
// Both stop one byte past the widest run a uint64 can hold, so a payload of
// digits is never walked to its end. What is past that bound is refused by
// checkCountedDigits, whatever follows it.
func countedDigitsText(line string) int {
	end := min(len(line), uint64DigitsMax+1)
	for at := range end {
		if line[at] < '0' || line[at] > '9' {
			return at
		}
	}
	return end
}

func countedDigitsBytes(field []byte) int {
	end := min(len(field), uint64DigitsMax+1)
	for at := range end {
		if field[at] < '0' || field[at] > '9' {
			return at
		}
	}
	return end
}

// checkCountedDigits judges the digit run a counted field opens with. It is the
// one place both fields and both readers refuse a run no uint64 can hold. A
// malformed count therefore reads the same whichever of them met it.
//
// The caller quotes the field it read the run from, because only the caller
// knows how to bound that quotation.
func checkCountedDigits(digits int) error {
	if digits == 0 {
		return errors.New("counted field states no digits")
	}
	if digits > uint64DigitsMax {
		return fmt.Errorf("counted field states more than the %d digits a uint64 occupies", uint64DigitsMax)
	}
	return nil
}

// cutCountedNumber takes a counted number off the front of line and returns its
// value and everything after the field.
//
// A space or the end of the line closes the digits. What closes them is checked
// here, and what FOLLOWS them is the caller's business. cutFieldSeparator says
// another field follows; this says the field itself is a number.
func cutCountedNumber(line string) (value uint64, rest string, err error) {
	digits := countedDigitsText(line)
	if err := checkCountedDigits(digits); err != nil {
		return 0, "", fmt.Errorf("%w: %q", err, truncate(line, 80))
	}
	if digits < len(line) && line[digits] == ':' {
		return 0, "", fmt.Errorf("%w: %q", errCountedNumberColon, truncate(line, 80))
	}
	// The digits were counted above, so ParseUint refuses only a value past the
	// range of a uint64.
	value, err = strconv.ParseUint(line[:digits], 10, 64)
	if err != nil {
		return 0, "", fmt.Errorf("counted number %q: %w", line[:digits], err)
	}
	return value, line[digits:], nil
}

// cutCountedText takes a counted text off the front of line and returns its
// bytes and everything after the field.
//
// The colon MUST be there, whatever the count states. It is what separates a
// text from a number, and a text carries it even when it carries no bytes.
//
// The stated byte count is checked against what arrived BEFORE the slice, so a
// line claiming more bytes than it carries is refused rather than panicking the
// reader.
func cutCountedText(line string) (text, rest string, err error) {
	digits := countedDigitsText(line)
	if err := checkCountedDigits(digits); err != nil {
		return "", "", fmt.Errorf("%w: %q", err, truncate(line, 80))
	}
	if digits >= len(line) || line[digits] != ':' {
		return "", "", fmt.Errorf("%w: %q", errCountedTextUnclosed, truncate(line, 80))
	}
	size, err := strconv.ParseUint(line[:digits], 10, 64)
	if err != nil {
		return "", "", fmt.Errorf("counted text byte count %q: %w", line[:digits], err)
	}
	rest = line[digits+1:]
	if uint64(len(rest)) < size {
		return "", "", fmt.Errorf("counted text of %d bytes runs past the end of the line: %q", size, truncate(line, 80))
	}
	return rest[:size], rest[size:], nil
}

// cutCountedBytes takes a counted text off the front of field and returns its
// bytes and everything after it, both referencing field rather than copying it.
//
// It is cutCountedText for a value held as bytes, and it exists for the reason
// appendCountedBytes does: a record's payload reaches and leaves the wire
// without being copied into a string, on the line that repeats. The refusals
// are checkCountedDigits's and errCountedTextUnclosed's, so the two readers
// cannot disagree about what a malformed count is.
//
// The stated byte count is checked against what arrived BEFORE the slice, so a
// line claiming more bytes than it carries is refused rather than panicking the
// reader.
func cutCountedBytes(field []byte) (value, rest []byte, err error) {
	size, header, err := countedTextAt(field)
	if err != nil {
		return nil, nil, err
	}
	// The stated count is checked against what arrived BEFORE the slice.
	if size > uint64(len(field)-header) {
		return nil, nil, fmt.Errorf("counted text of %d bytes runs past the end of the line: %q", size, truncateBytes(field, 80))
	}
	end := header + int(size)
	return field[header:end], field[end:], nil
}

// countedTextAt reads the `<n>:` a counted text opens with and returns the byte
// count it states and how wide that header is. The bytes of the value follow the
// header and are never touched here.
//
// It is cutCountedText for a line held as bytes, minus the slice. A reader that
// needs to know where a line ENDS asks for widths and never for the values
// behind them, and that reader is the frame reader.
func countedTextAt(field []byte) (size uint64, header int, err error) {
	digits := countedDigitsBytes(field)
	if err := checkCountedDigits(digits); err != nil {
		return 0, 0, fmt.Errorf("%w: %q", err, truncateBytes(field, 80))
	}
	if digits >= len(field) || field[digits] != ':' {
		return 0, 0, fmt.Errorf("%w: %q", errCountedTextUnclosed, truncateBytes(field, 80))
	}
	// The digits were counted above, so ParseUint refuses only a value past the
	// range of a uint64.
	size, err = strconv.ParseUint(string(field[:digits]), 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("counted text byte count %q: %w", field[:digits], err)
	}
	return size, digits + 1, nil
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

// appendID appends the `#<id> ` field naming which conversation a line belongs
// to. Requests, responses and answer lines open with it alike, so the protocol
// carries ONE id encoding rather than one for each direction.
func appendID(buf []byte, id uint64) []byte {
	buf = append(buf, '#')
	buf = strconv.AppendUint(buf, id, 10)
	return append(buf, ' ')
}

// cutID takes the `#<id> ` field off the front of line and returns the id, the
// decimal digits it was spelled with, and everything after the space that ends
// the field.
//
// ONE loop accumulates the value while it walks to that space. The digits come
// back beside the value because a caller keys its pending calls by them and the
// substring costs nothing, where a search for the space and a ParseUint over
// what it found would walk the same bytes twice.
//
// The field MUST end at a space. A digit run that reaches the end of the line,
// a byte that is neither a digit nor that space, an id past the 20 digits a
// uint64 occupies, and one past the range of a uint64 are each refused: this is
// the field a reader takes before it knows anything else about the line, so a
// wrong read here mis-slices every field after it (R-2).
//
// It is the one reader of the id, and appendID is the one writer, so no
// consumer keeps its own copy of the rule.
func cutID(line string) (id uint64, digits, rest string, err error) {
	if line == "" {
		return 0, "", "", errors.New("empty line carries no id")
	}
	if line[0] != '#' {
		return 0, "", "", fmt.Errorf("line missing # prefix: %q", truncate(line, 80))
	}
	for i := 1; i < len(line); i++ {
		c := line[i]
		if c == ' ' {
			if i == 1 {
				return 0, "", "", fmt.Errorf("line states no id digits: %q", truncate(line, 80))
			}
			return id, line[1:i], line[i+1:], nil
		}
		if c < '0' || c > '9' {
			return 0, "", "", fmt.Errorf("line states an id holding %q, which is not a decimal digit: %q", string(rune(c)), truncate(line, 80))
		}
		if i > uint64DigitsMax {
			return 0, "", "", fmt.Errorf("line states an id past the %d digits a uint64 occupies: %q", uint64DigitsMax, truncate(line, 80))
		}
		if id > (math.MaxUint64-uint64(c-'0'))/10 {
			return 0, "", "", fmt.Errorf("line states an id past the range of a uint64: %q", truncate(line, 80))
		}
		id = id*10 + uint64(c-'0')
	}
	return 0, "", "", fmt.Errorf("id does not end at a space: %q", truncate(line, 80))
}

// ParseLine parses a wire line into id, verb, and payload.
// Format: #<id> <verb> [<payload>].
func ParseLine(line []byte) (id uint64, verb string, payload []byte, err error) {
	id, _, rest, err := cutID(string(line))
	if err != nil {
		return 0, "", nil, err
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

// AppendRequest appends a request line (#<id> <method> [<json>]) to buf
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

// AppendResult appends a success response line (#<id> ok [<json>]) to
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

// AppendOK appends an empty success response line (#<id> ok) to buf.
// Newline is NOT appended.
func AppendOK(buf []byte, id uint64) []byte {
	return append(appendID(buf, id), 'o', 'k')
}

// AppendError appends an error response line (#<id> error [<json>]) to
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
// `#<id> <kind> <tail>`, where the kind states what the line IS and the
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

// A three-letter word on an answer line MUST be followed by a space, and MUST
// NOT be followed by the line terminator. That holds for both of them: the kind
// token and the head's item type. It is a rule of the grammar rather than a
// property of the lines this build happens to write, and every writer here
// keeps it (TestEveryThreeLetterWordIsSpaceClosed).
//
// It is what makes the four bytes of a word ALWAYS present, so a reader loads
// them as one integer and compares once. That one compare proves three things
// at once: the token is one this build knows, it is closed by a space, and no
// longer word merely opening with those three letters is read as it. Without
// the rule a word could sit last on a line, and every load would need a bounds
// case of its own.
const answerWordWidth = answerKindWidth + 1

// answerWord is a three-letter word and the space that closes it, held as the
// one uint32 a reader loads to check both at once.
type answerWord uint32

// answerWordOf derives the word for a three-letter token. Every constant below
// comes from its own token string, so a token and the integer a reader compares
// it against cannot drift apart (TestAnswerWordsComeFromTheirTokens).
func answerWordOf(token string) answerWord {
	var spelled [answerWordWidth]byte
	copy(spelled[:], token)
	spelled[answerKindWidth] = ' '
	return answerWord(binary.LittleEndian.Uint32(spelled[:]))
}

// answerWordBytes and answerWordText load the same four bytes as one word. The
// wire reaches the frame reader as bytes and the multiplexer as a string, and
// neither pays a conversion to reach one loader.
func answerWordBytes(line []byte, at int) answerWord {
	return answerWord(binary.LittleEndian.Uint32(line[at : at+answerWordWidth]))
}

func answerWordText(line string, at int) answerWord {
	return answerWord(line[at]) |
		answerWord(line[at+1])<<8 |
		answerWord(line[at+2])<<16 |
		answerWord(line[at+3])<<24
}

// The word of every kind, and of every item type, derived from the token.
var (
	answerWordHead          = answerWordOf(AnswerKindHead)
	answerWordRecord        = answerWordOf(AnswerKindRecord)
	answerWordFault         = answerWordOf(AnswerKindFault)
	answerWordTerminator    = answerWordOf(AnswerKindTerminator)
	answerWordNotUnderstood = answerWordOf(AnswerKindNotUnderstood)
	answerWordTypeDocument  = answerWordOf(AnswerTypeDocument)
	answerWordTypeMap       = answerWordOf(AnswerTypeMap)
	answerWordTypeTable     = answerWordOf(AnswerTypeTable)
)

// answerKindOfWord names the kind a word states, and reports whether it states
// one. Five integer compares, no vocabulary lookup and no string compare.
func answerKindOfWord(word answerWord) (string, bool) {
	switch word {
	case answerWordHead:
		return AnswerKindHead, true
	case answerWordRecord:
		return AnswerKindRecord, true
	case answerWordFault:
		return AnswerKindFault, true
	case answerWordTerminator:
		return AnswerKindTerminator, true
	case answerWordNotUnderstood:
		return AnswerKindNotUnderstood, true
	}
	return "", false
}

// answerTypeOfWord names the item type a word states, and reports whether it
// states one. It is the same primitive answerKindOfWord uses, because the two
// tokens are the same shape and one reader serves both.
func answerTypeOfWord(word answerWord) (string, bool) {
	switch word {
	case answerWordTypeDocument:
		return AnswerTypeDocument, true
	case answerWordTypeMap:
		return AnswerTypeMap, true
	case answerWordTypeTable:
		return AnswerTypeTable, true
	}
	return "", false
}

// answerKindKnown reports whether kind is a kind this build knows, from the
// token alone. The frame reader and the tail reader take the word instead; this
// serves the one caller that holds a kind already cut off its line, which is
// the orphan-line guard in the multiplexer.
func answerKindKnown(kind string) bool {
	return slices.Contains(answerKinds, kind)
}

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
// It loads the token AND the space that closes it as one word and compares that
// word once. No line is searched for a separator, and a method name merely
// opening with three kind bytes is refused by the same compare rather than by a
// check of its own.
//
// It is the one reader of the token. The mux channel carries answers, plain
// responses and inbound requests on one wire. A body this refuses is one of the
// other two, so the caller falls through to the verb its family cuts (A-3). The
// exec channel carries answers alone, so a refusal there ends the line rather
// than routing it (R-5).
func answerKindAt(body string) (string, bool) {
	if len(body) < answerWordWidth {
		return "", false
	}
	return answerKindOfWord(answerWordText(body, 0))
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
// nothing needs to be told apart and no #<id> is written. Every other answer
// travels on the multiplexed plugin connection, whose ids start at 1
// (Conn.idSeq), so no real id collides with it.
const AnswerNoID uint64 = 0

// AppendAnswerHead appends an answer head line
// (#<id> top <type> <n>:<key> <n>:<columns>) to buf and returns the extended
// slice. Newline is NOT appended.
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
	buf = appendCountedText(buf, key)
	buf = append(buf, ' ')
	return appendCountedBytes(buf, columns)
}

// appendAnswerRecordPrefix appends everything a record line writes in FRONT of
// its payload bytes: the id field, the kind token, the space that closes it,
// and the counted number stating how many payload bytes follow.
//
// It is the one writer of that prefix, so AppendAnswerItem, AppendAnswerFault
// and AnswerRecordLineSize spell the record line once between them and a
// measured line cannot drift from a written one.
func appendAnswerRecordPrefix(buf []byte, id uint64, kind string, size uint64) []byte {
	buf = appendAnswerPrefix(buf, id, kind)
	buf = append(buf, ' ')
	buf = appendCountedNumber(buf, size)
	return append(buf, ':')
}

// AppendAnswerItem appends a result record line
// (#<id> row <n>:<json>) and returns the extended slice. Newline is
// NOT appended.
//
// The kind states that the payload is a produced row, so no key name sits in
// front of it, and the payload states its own byte count, so a reader slices it
// by arithmetic and never searches the line for where it stops.
//
// The payload is appended VERBATIM. It is the line that repeats, so no pass is
// made over its bytes, and no byte of it has to be rewritten. A raw newline or
// carriage return inside the payload is ordinary data: the frame is taken by the
// width the line states rather than by the first newline it meets
// (ScanAnswerLines, framing.go), so the terminator is the newline AFTER the
// counted payload and nothing else ends the line.
func AppendAnswerItem(buf []byte, id uint64, item json.RawMessage) []byte {
	buf = appendAnswerRecordPrefix(buf, id, AnswerKindRecord, uint64(len(item)))
	return append(buf, item...)
}

// AppendAnswerFault appends an error record line
// (#<id> bad <n>:<json>) and returns the extended slice. Newline is
// NOT appended. A fault records one rejected row and does not end the walk,
// which is what lets an answer report 97 rows applied and 3 rejected.
//
// Its payload is counted and appended verbatim, exactly as AppendAnswerItem's
// is, so one reader serves both record kinds.
func AppendAnswerFault(buf []byte, id uint64, fault json.RawMessage) []byte {
	buf = appendAnswerRecordPrefix(buf, id, AnswerKindFault, uint64(len(fault)))
	return append(buf, fault...)
}

// answerRecordPrefixWidth is the EXACT width of the widest record-line prefix,
// and it is the scratch AnswerRecordLineSize measures one in. Every part of it
// states its own bound, so it is arithmetic rather than a guess:
//
//	idPrefixMax        the id field at its widest, 22 bytes
//	answerKindWidth    the kind token, 3 bytes
//	1                  the space that closes the kind
//	uint64DigitsMax    the digits stating the payload's byte count, 20 bytes
//	1                  the colon that closes that count
//
// That is 47 bytes, and it is what appendAnswerRecordPrefix writes for the
// widest id and the widest count, neither more nor less
// (TestAnswerRecordLineSizeExact). A looser constant would cost nothing a test
// can see, which is why the test states equality rather than a bound.
const answerRecordPrefixWidth = idPrefixMax + answerKindWidth + 1 + uint64DigitsMax + 1

// AnswerRecordLineSize reports how many bytes the record line for record
// occupies under id, its newline terminator excluded. It measures the line
// AppendAnswerItem writes for a result record and the line AppendAnswerFault
// writes for a rejected one, so a producer can refuse a record wider than
// MaxMessageSize before it builds the line rather than after.
//
// The prefix is built by the appender that writes it and the payload reaches
// the wire byte for byte, so this size IS the line's size rather than an
// estimate of it.
//
// A record carrying neither an item nor a fault measures as an empty item. Its
// producer refuses it on its own account (ErrEmptyAnswerRecord, collapse.go),
// and an empty record fits every line.
func AnswerRecordLineSize(id uint64, record Record) int {
	kind, value := AnswerKindRecord, record.Item
	if len(record.Fault) > 0 {
		kind, value = AnswerKindFault, record.Fault
	}
	var scratch [answerRecordPrefixWidth]byte
	return len(appendAnswerRecordPrefix(scratch[:0], id, kind, uint64(len(value)))) + len(value)
}

// AppendAnswerTerminator appends the terminator line
// (#<id> end <count> <faults> <n>:<message>) to buf and returns the extended
// slice. Newline is NOT appended.
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
	return appendCountedText(buf, message)
}

// AppendAnswerNotUnderstood appends the answer to a command the daemon did not
// understand (#<id> nay <n>:<code> <n>:<message>) to buf and returns the
// extended slice. Newline is NOT appended.
//
// It is the whole answer for its id: the kind says the conversation was valid
// and the command was not, which is what lets a client offer completion here and
// an operational message for a command that was understood and then failed. An
// empty code is present and empty, so the line's field count never varies.
func AppendAnswerNotUnderstood(buf []byte, id uint64, code, message string) []byte {
	buf = appendAnswerPrefix(buf, id, AnswerKindNotUnderstood)
	buf = append(buf, ' ')
	buf = appendCountedText(buf, code)
	buf = append(buf, ' ')
	return appendCountedText(buf, message)
}

// appendAnswerPrefix appends the `#<id> <kind>` every answer line opens
// with. It is the one writer of the kind token, and answerKindAt the one
// reader, so no line spells a kind the reader does not know.
func appendAnswerPrefix(buf []byte, id uint64, kind string) []byte {
	buf = appendAnswerID(buf, id)
	return append(buf, kind...)
}

// appendAnswerID appends the `#<id> ` that names which answer a line
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
	case AnswerKindRecord, AnswerKindFault:
		return parseAnswerRecord(kind, payload)
	case AnswerKindTerminator:
		return parseAnswerTerminator(payload)
	case AnswerKindNotUnderstood:
		return parseAnswerNotUnderstood(payload)
	}
	return AnswerTail{}, fmt.Errorf("answer line states kind %q, want one of %s", truncate(kind, answerKindWidth), answerKindWords)
}

// parseAnswerRecord reads a record line's one field: the counted payload the
// kind states is a produced row (AnswerKindRecord) or a rejected one
// (AnswerKindFault).
//
// The payload is sliced out of the bytes it arrived in rather than copied, so a
// consumer that forwards a record copies nothing.
//
// The line MUST end where the payload ends. That check is what the stated count
// buys: a line carrying more is a line nobody can read whole, and a line
// carrying less is one the frame reader ended early, which is what a raw
// newline inside a value and a `\r\n` terminator each do. Both are refused here
// rather than half-read (AC-20).
func parseAnswerRecord(kind string, payload []byte) (AnswerTail, error) {
	value, rest, err := cutCountedBytes(payload)
	if err != nil {
		return AnswerTail{}, fmt.Errorf("answer %s payload: %w", kind, err)
	}
	if len(rest) > 0 {
		return AnswerTail{}, fmt.Errorf("answer %s line carries %d bytes past its payload: %q", kind, len(rest), truncateBytes(rest, 40))
	}
	if kind == AnswerKindFault {
		return AnswerTail{Kind: kind, Fault: value}, nil
	}
	return AnswerTail{Kind: kind, Item: value}, nil
}

// parseAnswerHead reads the head's three fields: the item type, the envelope
// name, and the column names. Each sits at an offset the field before it states,
// so the line is searched for nothing.
func parseAnswerHead(payload []byte) (AnswerTail, error) {
	if len(payload) < answerWordWidth {
		return AnswerTail{}, errors.New("answer head states no item type, so a consumer cannot read the records that follow it")
	}
	// One load and one compare prove the item type is one this build knows AND
	// that the space the grammar requires after a three-letter word is there.
	itemType, known := answerTypeOfWord(answerWordBytes(payload, 0))
	if !known {
		return AnswerTail{}, fmt.Errorf("answer head states item type %q, want one of %s", truncateBytes(payload, answerTypeWidth), answerTypeWords)
	}
	tail := AnswerTail{Kind: AnswerKindHead, Type: itemType}

	text := string(payload)
	rest := text[answerWordWidth:]
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

// FormatRequest returns a request line (#<id> <method> [<json>]) in a
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

// truncateBytes is truncate for a value held as bytes. It bounds the slice
// BEFORE it converts, so a refusal over a 16 MB line builds 80 bytes of string
// rather than 16 MB of it.
func truncateBytes(value []byte, n int) string {
	if len(value) > n {
		value = value[:n+1]
	}
	return truncate(string(value), n)
}

// answerFieldShape is how wide one field of an answer line is: a three-letter
// word, a counted number, or a counted text. Every kind is a fixed sequence of
// them, and that is what lets a reader compute a WHOLE line's width without
// decoding one byte of its values.
type answerFieldShape uint8

const (
	answerFieldWord answerFieldShape = iota
	answerFieldNumber
	answerFieldText
)

// answerLineShapes is the field order of every kind, in the order the published
// line table lists it. TestAnswerLineWidthMatchesTheWriters binds it to the
// appenders, so a kind whose fields move in one place and not the other is
// refused rather than mis-framed.
var answerLineShapes = map[string][]answerFieldShape{
	AnswerKindHead:          {answerFieldWord, answerFieldText, answerFieldText},
	AnswerKindRecord:        {answerFieldText},
	AnswerKindFault:         {answerFieldText},
	AnswerKindTerminator:    {answerFieldNumber, answerFieldNumber, answerFieldText},
	AnswerKindNotUnderstood: {answerFieldText, answerFieldText},
}

// answerLineState is what answerLineWidth could tell about the bytes it read.
type answerLineState uint8

const (
	// answerLineOther: the bytes open no answer line this build knows, so the
	// newline is what frames them. A request, a response, and an operator's
	// plain text all land here.
	answerLineOther answerLineState = iota
	// answerLinePartial: the bytes open an answer line and have not yet stated
	// how wide it is. More bytes decide it, and the newline MUST NOT frame it:
	// a counted field it has not reached may carry one as data.
	answerLinePartial
	// answerLineStated: the line states its own width.
	answerLineStated
)

// idFieldEnd reports the offset just past the space that closes the `#<id> `
// field, and -1 when data opens with no such field.
func idFieldEnd(data []byte) int {
	for at := 1; at < len(data) && at <= uint64DigitsMax+1; at++ {
		if data[at] == ' ' {
			if at == 1 {
				return -1
			}
			return at + 1
		}
		if data[at] < '0' || data[at] > '9' {
			return -1
		}
	}
	return -1
}

// answerLineWidth reports how many bytes the answer line opening data occupies,
// its newline terminator excluded.
//
// Every variable-width field of every kind states its own width: a number by the
// digits a space closes, a text by its byte count. The width of a whole line is
// the sum of what its fields state, and NOTHING is searched for. That is what
// lets a value hold a raw newline: it sits inside a counted field, and no reader
// frames on it.
//
// A prefix that has not fully arrived reports answerLineOther rather than
// answerLinePartial, and that is safe: a well-formed prefix holds no newline,
// so a caller that falls back to the newline finds none inside one that is
// still arriving and asks for more bytes instead of framing early. Once the
// kind is known the answer becomes answerLinePartial, because everything after
// it MAY hold one.
func answerLineWidth(data []byte) (uint64, answerLineState) {
	at := 0
	if len(data) > 0 && data[0] == '#' {
		end := idFieldEnd(data)
		if end < 0 {
			return 0, answerLineOther
		}
		at = end
	}

	if len(data)-at < answerWordWidth {
		return 0, answerLineOther
	}
	kind, known := answerKindOfWord(answerWordBytes(data, at))
	if !known {
		return 0, answerLineOther
	}
	at += answerWordWidth

	for index, shape := range answerLineShapes[kind] {
		if index > 0 {
			// One space separates two fields. A three-letter word carries its
			// own, so the shapes count three bytes for one and the space is
			// counted here for every field alike.
			if at >= len(data) {
				return 0, answerLinePartial
			}
			if data[at] != ' ' {
				return 0, answerLineOther
			}
			at++
		}
		width, state := answerFieldWidth(shape, data[at:])
		if state != answerLineStated {
			return 0, state
		}
		if width > uint64(MaxMessageSize) {
			// The line is past the maximum whatever follows this field, so the
			// width is stated NOW and the caller refuses it before anything
			// grows a buffer a peer chose the size of.
			return uint64(at) + width, answerLineStated
		}
		at += int(width)
	}
	return uint64(at), answerLineStated
}

// answerFieldWidth reports how many bytes one field of shape occupies at the
// front of field. A counted field states it; a word is three bytes by
// construction.
func answerFieldWidth(shape answerFieldShape, field []byte) (uint64, answerLineState) {
	switch shape {
	case answerFieldWord:
		if len(field) < answerTypeWidth {
			return 0, answerLinePartial
		}
		return answerTypeWidth, answerLineStated
	case answerFieldNumber:
		digits := countedDigitsBytes(field)
		if digits >= len(field) {
			// The run reached the end of what arrived, so the next read MAY
			// extend it and nothing has said yet where this number ends. The
			// byte that closes it is a space on every kind written today, and
			// the line terminator would do as well: either way it has to
			// arrive before the width is known.
			return 0, answerLinePartial
		}
		if checkCountedDigits(digits) != nil {
			return 0, answerLineOther
		}
		return uint64(digits), answerLineStated
	case answerFieldText:
		size, header, err := countedTextAt(field)
		if err != nil {
			return 0, countedTextState(field)
		}
		return uint64(header) + size, answerLineStated
	}
	// A shape nobody thought of is a line this build cannot frame, so it is
	// refused rather than measured with a default filled in.
	return 0, answerLineOther
}

// countedTextState says whether a counted text this build could not read is
// malformed or merely still arriving. Its header is at most the twenty decimal
// digits a uint64 occupies and the colon that closes them, so anything shorter
// than that may still be completed by the next read.
func countedTextState(field []byte) answerLineState {
	if len(field) <= uint64DigitsMax {
		return answerLinePartial
	}
	return answerLineOther
}
