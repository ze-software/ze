// Design: docs/architecture/storage-backends.md -- content-addressed config history
// Related: pointer.go -- the pointers that name a version, and its removal
// Related: store.go -- the guard's raw-key methods every function here uses

package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"time"

	"github.com/ze-software/ze/pkg/zefs"
)

// ErrHistoryObject reports a history entry or object that cannot be trusted:
// an entry whose value is not `sha256:<64 lowercase hex>`, or an object whose
// bytes do not hash to its name. It is never absence: a missing entry or
// object answers fs.ErrNotExist instead.
var ErrHistoryObject = errors.New("config history object mismatch")

// entryDigestPrefix spells a digest the way the repository already does
// (writeConfigActiveHash, meta/config/last-known-good).
const entryDigestPrefix = "sha256:"

// digestHexLength is the length of a SHA-256 digest in hex characters.
const digestHexLength = 2 * sha256.Size

// historyEntry reports whether key is a dated history entry
// `file/<stamp>/<name>`, and returns its stamp and name. It is the one
// classification ListVersions and the removal sweep share: three components
// and a middle one parseVersionStamp accepts. The mutable file/active,
// file/draft and file/template keys fail the stamp parse.
func historyEntry(key string) (stamp, name string, ok bool) {
	parts := strings.Split(key, "/")
	if len(parts) != 3 {
		return "", "", false
	}
	if parts[0] != "file" {
		return "", "", false
	}
	if _, err := parseVersionStamp(parts[1]); err != nil {
		return "", "", false
	}
	return parts[1], parts[2], true
}

// objectKey names the object that holds the bytes whose SHA-256 is digest.
func objectKey(digest string) string {
	return zefs.KeyObject.Key(digest)
}

// contentDigest returns the lowercase hex SHA-256 of data.
func contentDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// entryDigest returns the digest an entry value names. The value MUST be
// exactly `sha256:` plus 64 lowercase hex characters; anything else is an
// ErrHistoryObject naming the entry, so a hand-written value is never read
// as a reference.
func entryDigest(entryKey string, value []byte) (string, error) {
	digest, ok := strings.CutPrefix(string(value), entryDigestPrefix)
	if !ok {
		return "", fmt.Errorf("%w: history entry %s does not hold a sha256 digest", ErrHistoryObject, entryKey)
	}
	if len(digest) != digestHexLength {
		return "", fmt.Errorf("%w: history entry %s holds a digest of %d characters, not %d", ErrHistoryObject, entryKey, len(digest), digestHexLength)
	}
	for i := range len(digest) {
		if !isLowerHex(digest[i]) {
			return "", fmt.Errorf("%w: history entry %s holds a digest that is not lowercase hex", ErrHistoryObject, entryKey)
		}
	}
	return digest, nil
}

func isLowerHex(c byte) bool {
	if c >= '0' && c <= '9' {
		return true
	}
	return c >= 'a' && c <= 'f'
}

// verifyObject refuses an object whose bytes do not hash to digest. The frame
// CRC proves the bytes are the ones written; only the hash proves they are the
// content the name promises.
func verifyObject(entryKey, digest string, data []byte) error {
	stored := contentDigest(data)
	if stored == digest {
		return nil
	}
	return fmt.Errorf("%w: %s expects %s%s, %s holds %s%s", ErrHistoryObject,
		entryKey, entryDigestPrefix, digest, objectKey(digest), entryDigestPrefix, stored)
}

// writeVersionObject stores data under object/<hex> and points entryKey at it,
// through the held guard. The object is written before the entry, so a crash
// between them leaves an unreferenced object, never a dangling entry. An
// existing object is reused only when its stored bytes hash to its name:
// `ze data write` and a repaired frame can both put wrong bytes under a right
// name, and existence alone never proves identity.
func writeVersionObject(g *guard, entryKey string, data []byte, stamp time.Time) error {
	digest := contentDigest(data)
	key := objectKey(digest)
	stored, err := g.ReadKey(key)
	if err != nil {
		if !isNotExist(err) {
			return fmt.Errorf("read %s: %w", key, err)
		}
		if err := g.write(key, data, stamp); err != nil {
			return fmt.Errorf("write %s: %w", key, err)
		}
		return g.write(entryKey, []byte(entryDigestPrefix+digest), stamp)
	}
	if err := verifyObject(entryKey, digest, stored); err != nil {
		return err
	}
	return g.write(entryKey, []byte(entryDigestPrefix+digest), stamp)
}

// readVersionEntry resolves a history entry to its bytes: entry, then object,
// then the hash verified over the bytes. read is the raw-key reader of the
// caller's context, Storage.ReadKey outside a guard and WriteGuard.ReadKey
// under one, because Storage called under a held guard never returns. A
// missing entry or object keeps fs.ErrNotExist in the chain; a malformed
// entry or a wrong-hash object is ErrHistoryObject.
func readVersionEntry(read func(string) ([]byte, error), entryKey string) ([]byte, error) {
	value, err := read(entryKey)
	if err != nil {
		return nil, fmt.Errorf("read history entry %s: %w", entryKey, err)
	}
	digest, err := entryDigest(entryKey, value)
	if err != nil {
		return nil, err
	}
	data, err := read(objectKey(digest))
	if err != nil {
		return nil, fmt.Errorf("history entry %s names %s%s: read %s: %w", entryKey, entryDigestPrefix, digest, objectKey(digest), err)
	}
	if err := verifyObject(entryKey, digest, data); err != nil {
		return nil, err
	}
	return data, nil
}

// sweepObject deletes object/<digest> when no dated history entry of any
// config name still names it. It MUST run under the guard that deleted the
// entry, and only after that entry was really deleted. It enumerates through
// the guard's recursive raw walk; a listing or a read that fails refuses the
// deletion rather than guessing. A malformed entry references nothing, so it
// cannot retain an object.
func sweepObject(guard WriteGuard, digest string) error {
	keys, err := guard.ListKeys("file/")
	if err != nil {
		return fmt.Errorf("sweep %s: list history: %w", objectKey(digest), err)
	}
	for _, key := range keys {
		if _, _, ok := historyEntry(key); !ok {
			continue
		}
		value, err := guard.ReadKey(key)
		if err != nil {
			return fmt.Errorf("sweep %s: read %s: %w", objectKey(digest), key, err)
		}
		referenced, err := entryDigest(key, value)
		if err != nil {
			continue
		}
		if referenced == digest {
			return nil
		}
	}
	if err := guard.RemoveKey(objectKey(digest)); err != nil {
		if isNotExist(err) {
			return nil
		}
		return fmt.Errorf("sweep %s: %w", objectKey(digest), err)
	}
	return nil
}

// objectsFirst orders a key walk so every object/* key comes before every
// other key, each group keeping its order. An import or restore interrupted
// part way then leaves unreferenced objects, never an entry naming an object
// the target does not hold.
func objectsFirst(keys []string) []string {
	ordered := make([]string, 0, len(keys))
	for _, key := range keys {
		if strings.HasPrefix(key, zefs.KeyObject.Prefix()) {
			ordered = append(ordered, key)
		}
	}
	for _, key := range keys {
		if !strings.HasPrefix(key, zefs.KeyObject.Prefix()) {
			ordered = append(ordered, key)
		}
	}
	return ordered
}

// ReadVersionEntry reads the version ListVersions(configPath) published under
// entryKey in VersionInfo.Path. Callers that carry the entry key rather than
// the stamp (the editor's rollback, `config diff` by revision) resolve it here,
// so a key that is not a version of configPath is refused rather than read.
func ReadVersionEntry(store Storage, configPath, entryKey string) ([]byte, error) {
	versions, err := store.ListVersions(configPath)
	if err != nil {
		return nil, err
	}
	for _, version := range versions {
		if version.Path == entryKey {
			return store.ReadVersion(configPath, version.Stamp)
		}
	}
	return nil, fmt.Errorf("config %s has no history entry %s: %w", configPath, entryKey, fs.ErrNotExist)
}
