// Design: docs/architecture/core-design.md -- the fingerprint of the work a job does
package job

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// jobKey returns the fingerprint of the complete native command.
func jobKey(argv []string) string {
	var tb textbuf.Buffer
	tb.Str("CMD=").Join(argv, " ").Byte('\n')

	sum := sha256.Sum256([]byte(tb.String()))
	return hex.EncodeToString(sum[:])
}
