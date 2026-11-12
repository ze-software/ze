// Design: docs/architecture/config/syntax.md -- how a parse refusal names its line
// Related: parser.go -- Parser.errorf, the hierarchical parser's refusals
// Related: setparser.go -- the set parser's refusals
// Related: loader.go -- parseTreeWithYANG, the one layer that adds "parse config"

package config

import (
	"errors"
	"fmt"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// LineError is a parse refusal tied to the line of config text that holds it.
// The parser that read the line is the one layer that owns "line N: ", so a
// surface that prints the line apart (the `ze config validate` diagnostic)
// asks SplitLine for the text without it instead of cutting the prefix out of
// the rendered string.
//
// Build it with lineErrorf or newLineError; the zero value names the defect.
type LineError struct {
	line  int
	cause error
}

// lineErrorf builds the refusal of line from a format, as fmt.Errorf does, so
// a %w in format keeps its cause reachable through errors.As.
func lineErrorf(line int, format string, args ...any) error {
	return &LineError{line: line, cause: fmt.Errorf(format, args...)}
}

// newLineError ties an existing refusal to line.
func newLineError(line int, cause error) error {
	return &LineError{line: line, cause: cause}
}

// unknownFieldLine is the refusal of name, a field this build's schema does
// not define, on line. The pre-migration set parse records one for each
// field it skips (SetParser.UnknownFields).
func unknownFieldLine(line int, name string) *LineError {
	return &LineError{line: line, cause: fmt.Errorf("unknown field: %s", name)}
}

// Error renders "line N: <refusal>".
func (e *LineError) Error() string {
	var b textbuf.Buffer
	return b.Str("line ").Int(int64(e.line)).Str(": ").Str(e.Message()).String()
}

// Line answers the line the refusal names.
func (e *LineError) Line() int { return e.line }

// Message answers the refusal without its "line N: " prefix.
func (e *LineError) Message() string {
	if e.cause == nil {
		return "BUG: config.LineError built without lineErrorf or newLineError"
	}
	return e.cause.Error()
}

// Unwrap answers the refusal the line carries.
func (e *LineError) Unwrap() error { return e.cause }

// parseConfigError is the loader's context on a parse refusal: "parse config"
// is added here and by no other layer, so a caller that holds a further fact
// (a reload) adds only that fact (docs/contributing/ze-go-style.md, "Every
// error is handled"). hint, when set, comes before the refusal, followed by a
// blank line, so a refusal's "fix:" line stays the last line printed.
type parseConfigError struct {
	cause error
	hint  string
}

const parseConfigSegment = "parse config: "

func (e *parseConfigError) Error() string {
	var b textbuf.Buffer
	b.Str(parseConfigSegment)
	writeHint(&b, e.hint)
	return b.Err(e.cause).String()
}

// writeHint writes hint and the blank line that parts it from the refusal
// after it, nothing for no hint.
func writeHint(b *textbuf.Buffer, hint string) {
	if hint == "" {
		return
	}
	b.Str(hint).Str("\n\n")
}

func (e *parseConfigError) Unwrap() error { return e.cause }

// unknownFieldsError names every field of a set-format config this build
// does not define, each line once. strict is the strict parse's refusal of
// the first one, which carries its hint (the closest key, or the build tag
// of the plugin compiled out of this build that owns it, KeywordHint);
// others are the unknown fields of every other line. The two parses find
// the first field each, so a field is left out of others by its line, a
// typed value, never by comparing the rendered text.
type unknownFieldsError struct {
	strict error
	others []*LineError
}

// newUnknownFieldsError builds the list from the strict parse's refusal and
// every field the pre-migration parse skipped.
func newUnknownFieldsError(strict error, fields []*LineError) *unknownFieldsError {
	strictLine, _ := SplitLine(strict)
	others := make([]*LineError, 0, len(fields))
	for _, field := range fields {
		if field.Line() == strictLine {
			continue
		}
		others = append(others, field)
	}
	return &unknownFieldsError{strict: strict, others: others}
}

// Error renders the strict refusal, then each other field on its own line.
func (e *unknownFieldsError) Error() string {
	var b textbuf.Buffer
	b.Err(e.strict)
	e.writeOthers(&b)
	return b.String()
}

// writeOthers appends each other unknown field, "line N: " kept, since only
// the strict refusal's line is the one a surface prints apart.
func (e *unknownFieldsError) writeOthers(b *textbuf.Buffer) {
	for _, field := range e.others {
		b.Str("\n").Str(field.Error())
	}
}

// Unwrap answers the strict parse's refusal, so errors.As still reaches its
// LineError.
func (e *unknownFieldsError) Unwrap() error { return e.strict }

// SplitLine answers the line a parse refusal names and the refusal's text
// with that line left out, every other segment kept once: the loader's
// "parse config", the syntax path, the refusal itself. line is 0 when no
// layer of the chain names a line, and message is then err's own text.
//
// It walks the parser's own error types, never the rendered text, so a
// refusal whose words happen to hold "line " is not misread. The recursion
// is bounded by the parser's own wrapping: one parseConfigError, one
// opererr.Error and one unknownFieldsError (the set-format unknown-field
// refusal), one syntaxRefusal, one LineError or NodeOfAnotherOSError.
func SplitLine(err error) (line int, message string) {
	switch e := err.(type) { //nolint:errorlint // each layer of the parser's own chain is read in turn
	case *LineError:
		return e.line, e.Message()
	case *NodeOfAnotherOSError:
		return e.Line, e.message()
	case *syntaxRefusal:
		line, message = SplitLine(e.cause)
		var b textbuf.Buffer
		return line, b.Str(e.path).Str(": ").Str(message).String()
	case *parseConfigError:
		line, message = SplitLine(e.cause)
		var b textbuf.Buffer
		b.Str(parseConfigSegment)
		writeHint(&b, e.hint)
		return line, b.Str(message).String()
	case *unknownFieldsError:
		line, message = SplitLine(e.strict)
		var b textbuf.Buffer
		b.Str(message)
		e.writeOthers(&b)
		return line, b.String()
	case *opererr.Error:
		// An operator error around a line refusal is rendered again with the
		// refusal's line left out, by opererr's own layout.
		cause := e.Unwrap()
		if cause == nil {
			return 0, e.Error()
		}
		line, message = SplitLine(cause)
		return line, opererr.Wrap(errors.New(message), e.Problem(), e.Why(), e.Fix()).Error()
	}
	// A chain no parser layer built names no line: its own text is the whole
	// message, and any "line N: " in it is printed once, by that text.
	return 0, err.Error()
}
