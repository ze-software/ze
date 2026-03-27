// Design: docs/architecture/fleet-config.md — config version hashing
// Related: envelope.go — RPC payload types that carry version hashes

package fleet

import (
	"crypto/sha256"
	"encoding/hex"
)

// VersionHash computes a truncated SHA-256 hash of config content.
// Returns 16 lowercase hex characters. Deterministic: same content
// always produces the same hash.
func VersionHash(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:8])
}
