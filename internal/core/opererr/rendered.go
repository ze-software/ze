// Design: docs/architecture/cli/error-surface.md -- checked operator-error wire text.
package opererr

import (
	"errors"
	"strings"
)

// FromRendered checks an operator error received as Error() text. Unlike New
// and Wrap, it accepts external input and returns an error for incomplete text.
// Valid text renders byte for byte as received, without adding another fix.
//
// The first line is Problem; the text up to the final "fix: " block is Why.
// Embedded line breaks and rendered causes stay in that text. The flat wire
// format cannot recover the producer's original part boundaries or Go cause
// identities: Unwrap answers nil. Use native errors directly within a process.
func FromRendered(text string) (*Error, error) {
	correction := strings.LastIndex(text, "\n"+fixLabel)
	if correction < 0 {
		return nil, errors.New("operator error text has no final fix block")
	}
	problem, why, separated := strings.Cut(text[:correction], "\n")
	if !separated {
		return nil, errors.New("operator error text has no separate explanation")
	}
	fix := text[correction+1+len(fixLabel):]
	if strings.TrimSpace(problem) == "" {
		return nil, errors.New("operator error text has no problem")
	}
	if strings.TrimSpace(why) == "" {
		return nil, errors.New("operator error text has no explanation")
	}
	if strings.TrimSpace(fix) == "" {
		return nil, errors.New("operator error text has no correction")
	}
	return &Error{problem: problem, why: why, fix: fix}, nil
}
