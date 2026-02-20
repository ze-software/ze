// Design: docs/architecture/wire/messages.md — wire UPDATE lazy parsing

package wireu

import "errors"

// UPDATE message parsing errors.
// Use with fmt.Errorf for context: fmt.Errorf("withdrawn: %w", ErrUpdateTruncated).
var (
	// ErrUpdateTruncated indicates the UPDATE payload is shorter than declared lengths.
	ErrUpdateTruncated = errors.New("UPDATE payload truncated")

	// ErrUpdateMalformed indicates a structural error in the UPDATE message.
	ErrUpdateMalformed = errors.New("UPDATE malformed")
)
