// Design: docs/architecture/config/syntax.md — the config tokens an operator types
// Related: internal/core/bgp/attribute/text.go — AS-path text parsing that calls Parse
// Related: internal/core/bgp/attribute/text_append.go — AS-path text rendering that calls Append
//
// Package asn is the textual representation of an AS number, in the three
// notations RFC 5396 names. The stored value and the wire value are always a
// uint32. A notation decides how that number is SPELLED, and nothing else.
//
// Parse accepts every notation, whatever the operator configured. A pasted
// asdot number is therefore never refused. Append writes the one notation the
// operator asked to read.
//
// The package holds no configuration and reads no global state. The notation
// arrives as a parameter, because the renderers that call Append sit in leaf
// packages that must not import config.
package asn

import (
	"fmt"
	"strconv"
	"strings"
	"sync/atomic"
)

// firstFourByteAS is the lowest AS number that does not fit in 16 bits.
// RFC 5396 Section 2 puts the asdot boundary here: "asdot refers to a syntax
// scheme of representing AS number values less than 65536 using asplain
// notation and representing AS number values equal to or greater than 65536
// using asdot+ notation".
const firstFourByteAS = 65536

// fieldMax is the largest value either side of the period can hold. RFC 5396
// Section 2 defines asdot+ as "<high order 16-bit value in decimal>.<low
// order 16-bit value in decimal>". Each field is therefore a 16-bit number.
const fieldMax = 65535

// maxASNumber is the largest AS number, which RFC 6793 makes a 4-octet value.
const maxASNumber = 4294967295

// Notation is how an AS number is spelled in text.
//
// The zero value is NotationPlain on purpose. It is the only zero this package
// treats as an answer, and RFC 5396 Section 3 recommends it: "The decimal value
// representation, or "asplain" is proposed as the textual notation to use for
// AS numbers". An operator who configures no notation therefore reads what the
// RFC asks every system to write. Ze output is unchanged from before the leaf
// existed.
type Notation uint8

const (
	// NotationPlain writes every AS number as a decimal integer.
	NotationPlain Notation = iota
	// NotationDot writes an AS number less than 65536 as a decimal integer.
	// It writes any other AS number as X.Y.
	NotationDot
	// NotationDotPlus writes every AS number as X.Y.
	NotationDotPlus
)

// The config tokens that name each notation. They are the enum values of the
// `as-notation` leaf in internal/component/bgp/yang/ze-bgp-conf.yang. They are
// also the names RFC 5396 Section 2 gives the three schemes.
const (
	TokenPlain   = "asplain"
	TokenDot     = "asdot"
	TokenDotPlus = "asdot+"
)

// TypedefName is the ze-types YANG typedef that marks a leaf as an AS number.
// PrefixedTypedefName is the same typedef, as a module that imports ze-types
// spells it. A schema reader tests a leaf type name with IsTypedef, and learns
// that the leaf takes the notations this package reads.
const (
	TypedefName         = "asn"
	PrefixedTypedefName = "zt:asn"
)

// IsTypedef reports whether a YANG type name names the AS number typedef.
func IsTypedef(name string) bool {
	return name == TypedefName || name == PrefixedTypedefName
}

// String returns the config token that names the notation.
func (n Notation) String() string {
	switch n {
	case NotationPlain:
		return TokenPlain
	case NotationDot:
		return TokenDot
	case NotationDotPlus:
		return TokenDotPlus
	}
	panic("BUG: unknown AS notation")
}

// parseNotation returns the notation a config token names.
func parseNotation(token string) (Notation, error) {
	switch token {
	case TokenPlain:
		return NotationPlain, nil
	case TokenDot:
		return NotationDot, nil
	case TokenDotPlus:
		return NotationDotPlus, nil
	}
	return NotationPlain, fmt.Errorf("invalid as-notation %q: expected %s, %s, or %s", token, TokenPlain, TokenDot, TokenDotPlus)
}

// Parse returns the AS number a textual representation names, in any of the
// three RFC 5396 notations. A token that holds a period is read as asdot or
// asdot+. RFC 5396 Section 2 spells those two the same way. Every other token
// is read as asplain.
//
// Parse is the only place in Ze that turns AS number text into a number. Every
// surface therefore accepts the same spellings.
func Parse(text string) (uint32, error) {
	high, low, dotted := strings.Cut(text, ".")
	if !dotted {
		number, err := strconv.ParseUint(text, 10, 32)
		if err != nil {
			return 0, fmt.Errorf("invalid AS number %q: expected a decimal number or the asdot form X.Y", text)
		}
		return uint32(number), nil
	}

	first, err := strconv.ParseUint(high, 10, 64)
	if err != nil || first > fieldMax {
		return 0, fmt.Errorf("invalid asdot AS number %q: the value before the period must be 0 to %d", text, fieldMax)
	}
	second, err := strconv.ParseUint(low, 10, 64)
	if err != nil || second > fieldMax {
		return 0, fmt.Errorf("invalid asdot AS number %q: the value after the period must be 0 to %d", text, fieldMax)
	}
	return uint32(first)<<16 | uint32(second), nil
}

// Append appends the AS number to buf in the given notation and returns the
// extended slice. It allocates nothing beyond the growth of buf.
func Append(buf []byte, number uint32, notation Notation) []byte {
	if notation.plain(number) {
		return strconv.AppendUint(buf, uint64(number), 10)
	}
	// RFC 5396 Section 2: "<high order 16-bit value in decimal>.<low order
	// 16-bit value in decimal>".
	buf = strconv.AppendUint(buf, uint64(number>>16), 10)
	buf = append(buf, '.')
	return strconv.AppendUint(buf, uint64(number&0xFFFF), 10)
}

// plain reports whether the notation writes this AS number as a decimal
// integer rather than as two fields joined by a period.
func (n Notation) plain(number uint32) bool {
	switch n {
	case NotationPlain:
		return true
	case NotationDot:
		// RFC 5396 Section 2: asdot represents "AS number values less than
		// 65536 using asplain notation".
		return number < firstFourByteAS
	case NotationDotPlus:
		return false
	}
	panic("BUG: unknown AS notation")
}

// Text returns the AS number written in the given notation. Callers on a wire
// or a render path append into a buffer they own instead: Text allocates the
// string it returns.
func Text(number uint32, notation Notation) string {
	// 11 bytes holds the longest form either notation produces: "4294967295"
	// and "65535.65535".
	var scratch [11]byte
	return string(Append(scratch[:0], number, notation))
}

// LeafName is the config leaf that selects the notation. It is the leaf in
// internal/component/bgp/yang/ze-bgp-conf.yang, named here so the readers of a
// config subtree do not each spell it.
const LeafName = "as-notation"

// configured is the notation this process writes an AS number in.
//
// A notation is a property of the RUNNING configuration. It is recorded where
// a candidate becomes that configuration, and nowhere else:
//
//   - the daemon's first config, at applyASNotation
//     (internal/component/bgp/config/asn_notation.go), which no refusal follows
//   - every later config, at SetConfigTree
//     (internal/component/bgp/reactor/reactor_api.go). The reload coordinator
//     calls it last
//   - a plugin in a process of its own, at its Stage 2 config callback
//
// A CLIENT of those processes records nothing. It renders the spelling it
// received, which is what Number carries.
//
// It is package level because the renderers are spread over the daemon, four
// plugins, the CLI and the looking glass. A parameter threaded to each of them
// would be one value under a dozen names. Append and Text still take the
// notation as a parameter, so the formatter itself reads no state.
//
//nolint:gochecknoglobals // Process-wide display setting, written at config load.
var configured atomic.Uint32

// Configured returns the notation this process writes an AS number in.
func Configured() Notation {
	return Notation(configured.Load())
}

// Configure records the notation a config token selects. An empty token
// selects asplain, which is what an absent leaf means.
func Configure(token string) error {
	if token == "" {
		configured.Store(uint32(NotationPlain))
		return nil
	}
	notation, err := parseNotation(token)
	if err != nil {
		return err
	}
	configured.Store(uint32(notation))
	return nil
}

// ConfigureFromBGP records the notation the as-notation leaf of a bgp config
// subtree selects. A leaf that is not a string is an error rather than an
// absence. A value the reader cannot read must not come back looking like a
// value nobody wrote.
func ConfigureFromBGP(bgp map[string]any) error {
	value, present := bgp[LeafName]
	if !present {
		return Configure("")
	}
	token, ok := value.(string)
	if !ok {
		return fmt.Errorf("%s must be a string, got %T", LeafName, value)
	}
	// A leaf that IS present goes through parseNotation, so an empty value is
	// refused rather than read as asplain. Absence is what selects asplain,
	// and a leaf holding nothing is not an absent leaf.
	notation, err := parseNotation(token)
	if err != nil {
		return err
	}
	configured.Store(uint32(notation))
	return nil
}

// Number is an AS number as it crosses a JSON payload.
//
// A producer builds one with Of, and it marshals in the notation Configured()
// names. A consumer decodes a JSON number, or any of the three notations
// written as a string, and the Number KEEPS the text it was given.
//
// Keeping the text is what lets a client render what the producer rendered.
// The CLI dashboard and shell completion decode a payload the daemon wrote and
// show it again. Neither reads the daemon's configuration. A second derivation
// of the notation there would be a second declaration of one fact, free to
// disagree with the first.
//
// Both halves are needed together. asdot cannot be a JSON number, so a payload
// written under a dotted notation carries a string. Every reader of that
// payload has to accept it. A consumer that declared plain uint32 would fail
// the whole decode and answer nothing.
type Number struct {
	value uint32
	// text is the spelling this Number was decoded from. It is empty for a
	// Number a producer built, which has no spelling until it is written.
	text string
}

// Of returns the Number a producer writes into a payload.
func Of(number uint32) Number { return Number{value: number} }

// Value returns the AS number itself, for a comparison, a sort or a map key.
func (n Number) Value() uint32 { return n.value }

// String returns the text to show. A decoded Number answers the spelling its
// producer wrote. A Number built here answers the configured rendering.
func (n Number) String() string {
	if n.text != "" {
		return n.text
	}
	return Text(n.value, Configured())
}

// MarshalJSON writes the number in the configured notation. asplain writes a
// JSON number, which is byte-for-byte what Ze wrote before the leaf existed.
//
// A Number that was decoded is re-encoded in THIS process's notation, not in
// the spelling it arrived with. A relay then answers one notation throughout,
// rather than one per hop.
func (n Number) MarshalJSON() ([]byte, error) {
	return AppendJSON(make([]byte, 0, 13), n.value), nil
}

// UnmarshalJSON reads a JSON number or a quoted AS number in any notation, and
// keeps the spelling for String to answer with.
//
// JSON null leaves the Number at its zero value, which is what the plain
// uint32 field this type replaced did with one. No producer writes null today.
// Refusing one would fail an entire dashboard or completion decode over a
// single field, which is the failure this type exists to remove.
func (n *Number) UnmarshalJSON(data []byte) error {
	text := string(data)
	if text == "null" {
		return nil
	}
	if len(text) >= 2 && text[0] == '"' && text[len(text)-1] == '"' {
		text = text[1 : len(text)-1]
	}
	number, err := Parse(text)
	if err != nil {
		return err
	}
	n.value = number
	n.text = text
	return nil
}

// FromJSON reads an AS number out of a value decoded from a JSON payload,
// whichever notation wrote it. It reports false for a value that names no AS
// number, so a caller cannot read a failure as AS 0.
//
// json.Unmarshal into an `any` decodes a number as float64. A config tree
// delivered in process carries int or uint32. All four shapes therefore
// arrive.
func FromJSON(value any) (uint32, bool) {
	switch v := value.(type) {
	case string:
		number, err := Parse(v)
		return number, err == nil
	case float64:
		if v < 0 || v > maxASNumber {
			return 0, false
		}
		return uint32(v), true
	case int:
		if v < 0 || int64(v) > maxASNumber {
			return 0, false
		}
		return uint32(v), true
	case int64:
		if v < 0 || v > maxASNumber {
			return 0, false
		}
		return uint32(v), true
	case uint32:
		return v, true
	}
	return 0, false
}

// AppendJSON appends the AS number as a JSON value in the configured notation.
// asplain writes a JSON number, which is byte-for-byte what Ze wrote before
// the leaf existed. A dotted notation writes a quoted string, because X.Y is
// not a JSON number. Every reader of such a payload uses FromJSON or Number.
func AppendJSON(buf []byte, number uint32) []byte {
	notation := Configured()
	if notation == NotationPlain {
		return Append(buf, number, notation)
	}
	buf = append(buf, '"')
	buf = Append(buf, number, notation)
	return append(buf, '"')
}

// JSONValue returns the AS number as a JSON value, for a producer that builds
// its payload as text rather than through json.Marshal. AppendJSON states the
// rule. This form allocates the string it returns.
func JSONValue(number uint32) string {
	// 13 bytes holds "65535.65535" and the two quotes a dotted rendering adds.
	var scratch [13]byte
	return string(AppendJSON(scratch[:0], number))
}

// TextFromJSON returns the spelling a decoded payload value carries, so a
// client shows what the producer wrote. It reports false for a value that
// names no AS number.
//
// A dotted notation arrives as a string and is returned unchanged. asplain
// arrives as a JSON number, which json.Unmarshal decodes into a float64, and
// the %v verb prints a large one in exponent form. The decimal digits are
// written here instead.
func TextFromJSON(value any) (string, bool) {
	if text, ok := value.(string); ok {
		if _, err := Parse(text); err != nil {
			return "", false
		}
		return text, true
	}
	number, ok := FromJSON(value)
	if !ok {
		return "", false
	}
	return Text(number, NotationPlain), true
}
