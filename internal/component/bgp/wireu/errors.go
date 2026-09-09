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

	// ErrASPathSourceNotASN4 reports a payload offered for narrowing whose
	// encoding context says two-octet. Every payload on the forward path is
	// four-octet truth once the ingest collapse has run (aspath_collapse.go), so
	// this is a mislabeled payload rather than a request to widen one.
	ErrASPathSourceNotASN4 = errors.New("AS_PATH source encoding is not four-octet")
)
