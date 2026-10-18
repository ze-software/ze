// Design: docs/architecture/config/yang-config-design.md — YANG schema handling
// Related: validator.go — every check returns a ValidationError

package yang

import "github.com/ze-software/ze/internal/core/textbuf"

// ErrorType represents the type of validation error.
type ErrorType int

const (
	ErrTypeUnknown     ErrorType = iota
	ErrTypeMissing               // Missing mandatory field
	ErrTypeType                  // Wrong type
	ErrTypeRange                 // Value outside allowed range
	ErrTypePattern               // String doesn't match pattern
	ErrTypeEnum                  // Invalid enum value
	ErrTypeLength                // String length outside allowed range
	ErrTypeCardinality           // List/leaf-list has too many or too few entries
)

func (e ErrorType) String() string {
	switch e {
	case ErrTypeMissing:
		return "missing"
	case ErrTypeType:
		return "type"
	case ErrTypeRange:
		return "range"
	case ErrTypePattern:
		return "pattern"
	case ErrTypeEnum:
		return "enum"
	case ErrTypeLength:
		return "length"
	case ErrTypeCardinality:
		return "cardinality"
	case ErrTypeUnknown:
		return "unknown"
	default:
		panic("BUG: unknown YANG validation error type")
	}
}

// ValidationError represents a YANG validation error.
type ValidationError struct {
	Path       string    // Path to the invalid value
	Type       ErrorType // Type of validation error
	Message    string    // Human-readable error message
	Expected   string    // What was expected
	Got        string    // What was provided
	LineNumber int       // Line number in config file (if available)
}

func (e *ValidationError) Error() string {
	var b textbuf.Buffer
	if e.LineNumber > 0 {
		return b.Str("line ").Int(int64(e.LineNumber)).Str(": ").Str(e.Type.String()).Str(" error at ").Str(e.Path).Str(": ").Str(e.Message).String()
	}
	return b.Str(e.Type.String()).Str(" error at ").Str(e.Path).Str(": ").Str(e.Message).String()
}
