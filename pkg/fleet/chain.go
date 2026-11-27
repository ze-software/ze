// Design: docs/architecture/fleet-config.md — SHA-1 configuration hash chain
// Related: version.go — raw config version hashing
// Related: diff.go — deterministic line diff/patch folded into the chain
// Related: envelope.go — managed RPC envelopes carrying chain hashes

package fleet

import (
	"crypto/sha1" //nolint:gosec // SHA-1 hash chain is a non-adversarial sync checksum over authenticated TLS 1.3 per spec-fleet-10
	"encoding/hex"
	"errors"
)

// ChainHashLen is the lowercase hexadecimal length of a 20-byte SHA-1 chain digest.
const ChainHashLen = 40

// ChainState ties a 40-hex SHA-1 chain head to the 16-hex VersionHash of the
// active configuration it represents.
type ChainState struct {
	Version string `json:"version"`
	Chain   string `json:"chain"`
}

// ValidChainHash reports whether s is exactly ChainHashLen lowercase
// hexadecimal characters ('0'-'9', 'a'-'f').
func ValidChainHash(s string) bool {
	if len(s) != ChainHashLen {
		return false
	}
	for i := range len(s) {
		c := s[i]
		if c >= '0' && c <= '9' {
			continue
		}
		if c >= 'a' && c <= 'f' {
			continue
		}
		return false
	}
	return true
}

// FoldFull anchors a configuration hash chain from 20 zero bytes over a full
// configuration snapshot: SHA-1(20 zero bytes || "full\n" || config).
func FoldFull(config []byte) string {
	var zero [sha1.Size]byte
	h := sha1.New() //nolint:gosec // SHA-1 hash chain is a non-adversarial sync checksum over authenticated TLS 1.3 per spec-fleet-10
	h.Write(zero[:])
	h.Write([]byte("full\n"))
	h.Write(config)
	var sum [sha1.Size]byte
	return hex.EncodeToString(h.Sum(sum[:0]))
}

// FoldDiff folds one applied delta onto prevHex:
// SHA-1(prev20 || "diff\n" || baseVer || "\n" || targetVer || "\n" || delta).
// It returns an error when prevHex is not a valid 40-char lowercase hex hash,
// or when baseVer or targetVer is empty.
func FoldDiff(prevHex, baseVer, targetVer string, delta []byte) (string, error) {
	if !ValidChainHash(prevHex) {
		return "", errors.New("fleet: invalid previous chain hash")
	}
	if baseVer == "" {
		return "", errors.New("fleet: base version is required")
	}
	if targetVer == "" {
		return "", errors.New("fleet: target version is required")
	}
	var prev [sha1.Size]byte
	if _, err := hex.Decode(prev[:], []byte(prevHex)); err != nil {
		return "", errors.New("fleet: invalid previous chain hash")
	}
	h := sha1.New() //nolint:gosec // SHA-1 hash chain is a non-adversarial sync checksum over authenticated TLS 1.3 per spec-fleet-10
	h.Write(prev[:])
	h.Write([]byte("diff\n"))
	h.Write([]byte(baseVer))
	h.Write([]byte("\n"))
	h.Write([]byte(targetVer))
	h.Write([]byte("\n"))
	h.Write(delta)
	var sum [sha1.Size]byte
	return hex.EncodeToString(h.Sum(sum[:0])), nil
}
