// Design: docs/architecture/container/image-store.md -- the store layout: a folder per digest
// Related: layout.go -- the folders the digest names

package imagestore

import (
	"errors"
	"strings"

	"github.com/ze-software/ze/internal/plugins/container/registry"
)

// Digest forms. The text form is the one a registry and an operator write,
// and registry.ParseDigest is its one parser; the folder form replaces the
// colon, because a folder name is a field contract and a colon is awkward in
// a path on every tool that reads it.
const (
	digestTextPrefix   = "sha256:"
	digestFolderPrefix = "sha256-"
	// digestHexLength is the length of a SHA-256 digest in hex digits.
	digestHexLength = 64
)

// errDigestFolder refuses a folder name that is not the folder form of a
// digest. The listing passes over such a name: publication never writes one.
var errDigestFolder = errors.New("not a folder named sha256-<64 lower-case hex digits>")

// parseDigestFolder reads the folder form, sha256-<64 hex digits>, through
// the one parser of the text form, so a folder name and a digest the
// registry names are checked by the same rule.
func parseDigestFolder(name string) (registry.Digest, error) {
	hexDigits, ok := strings.CutPrefix(name, digestFolderPrefix)
	if !ok {
		return registry.Digest{}, errDigestFolder
	}
	digest, err := registry.ParseDigest(digestTextPrefix + hexDigits)
	if err != nil {
		return registry.Digest{}, errDigestFolder
	}
	return digest, nil
}

// digestFolder answers the folder form, sha256-<hex>, of digest. digest MUST
// not be the zero Digest: every caller holds one a parser built.
func digestFolder(digest registry.Digest) string {
	hexDigits, ok := strings.CutPrefix(digest.String(), digestTextPrefix)
	if !ok {
		panic("BUG: imagestore: a digest with no sha256 prefix names no folder")
	}
	return digestFolderPrefix + hexDigits
}
