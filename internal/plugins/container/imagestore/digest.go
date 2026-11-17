// Design: docs/architecture/container/image-store.md -- the store layout: a folder per digest
// Related: layout.go -- the folders the digest names

package imagestore

import (
	"encoding/hex"
	"errors"
	"strings"
)

// Digest forms. The text form is the one a registry and an operator write;
// the folder form replaces the colon, because a folder name is a field
// contract and a colon is awkward in a path on every tool that reads it.
const (
	digestTextPrefix   = "sha256:"
	digestFolderPrefix = "sha256-"
	// digestHexLength is the length of a SHA-256 digest in hex digits.
	digestHexLength = 64
)

// errDigestMalformed refuses a digest that is not SHA-256 in lower-case hex.
// The store names folders after digests, so only a digest that matches
// exactly one folder name is accepted.
var errDigestMalformed = errors.New("not a sha256 digest of 64 lower-case hex digits")

// imageDigest is a SHA-256 digest that was checked when it was built: only
// parseDigest and parseDigestFolder build one, so a value of this type always
// names exactly one folder of the store.
type imageDigest struct {
	hex string
}

// parseDigest reads the text form, sha256:<64 hex digits>.
func parseDigest(text string) (imageDigest, error) {
	hexDigits, ok := strings.CutPrefix(text, digestTextPrefix)
	if !ok {
		return imageDigest{}, errDigestMalformed
	}
	return digestFromHex(hexDigits)
}

// parseDigestFolder reads the folder form, sha256-<64 hex digits>.
func parseDigestFolder(name string) (imageDigest, error) {
	hexDigits, ok := strings.CutPrefix(name, digestFolderPrefix)
	if !ok {
		return imageDigest{}, errDigestMalformed
	}
	return digestFromHex(hexDigits)
}

// digestFromHex checks the hex digits of either form. Upper case is refused
// rather than folded: two spellings of one digest would name two folders.
func digestFromHex(hexDigits string) (imageDigest, error) {
	if len(hexDigits) != digestHexLength {
		return imageDigest{}, errDigestMalformed
	}
	if strings.ToLower(hexDigits) != hexDigits {
		return imageDigest{}, errDigestMalformed
	}
	if _, err := hex.DecodeString(hexDigits); err != nil {
		return imageDigest{}, errDigestMalformed
	}
	return imageDigest{hex: hexDigits}, nil
}

// String answers the text form, sha256:<hex>.
func (d imageDigest) String() string {
	return digestTextPrefix + d.hex
}

// folder answers the folder form, sha256-<hex>.
func (d imageDigest) folder() string {
	return digestFolderPrefix + d.hex
}
