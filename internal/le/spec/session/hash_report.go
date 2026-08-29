// Design: docs/architecture/core-design.md -- native spec lifecycle support
// Related: review.go -- review content hashes

package specsession

import "github.com/ze-software/ze/internal/core/textbuf"

// reviewHashes is the structured hash action answer.
type reviewHashes struct {
	Files []ReviewedFile `json:"files"`
}

// Text renders the hash lines the prior review gate produced.
func (r reviewHashes) Text() string {
	var tb textbuf.Buffer
	for _, file := range r.Files {
		tb.Str("  ").Str(file.Hash).Str("  ").Str(file.Path).Byte('\n')
	}
	return tb.String()
}
