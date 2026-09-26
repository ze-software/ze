// Design: docs/architecture/appliance/on-device-installer.md -- download with retry and integrity check
// Related: fetch.go -- ToFile, the fetcher every scheme below names

package fetch

import (
	"fmt"
	"maps"
	"slices"
	"strings"
)

// Fetcher downloads source to dest, then checks the SHA-256 of the bytes it
// wrote when expectedSHA is not empty. The caller owns dest and MUST remove
// it when the Fetcher fails.
type Fetcher func(source, dest, expectedSHA string) error

// schemes is the one table of the URL schemes a remote source can name. A
// later scheme (tftp, ssh) is one entry here, and every caller, its refusal
// and its help derive from this table.
var schemes = map[string]Fetcher{
	"http":  ToFile,
	"https": ToFile,
}

const schemeSeparator = "://"

// IsRemote reports whether source names a URL rather than a local path. A URL
// carries "<scheme>://" with no path separator in the scheme, so
// "./a://b" stays a local path.
func IsRemote(source string) bool {
	scheme, _, found := strings.Cut(source, schemeSeparator)
	if !found {
		return false
	}
	if scheme == "" {
		return false
	}
	return !strings.ContainsRune(scheme, '/')
}

// Resolve returns the Fetcher for a remote source. A scheme the table does
// not hold is refused, naming the scheme and the supported list.
func Resolve(source string) (Fetcher, error) {
	scheme, _, _ := strings.Cut(source, schemeSeparator)
	fetcher, known := schemes[strings.ToLower(scheme)]
	if !known {
		return nil, fmt.Errorf("unsupported scheme %q: supported %s", scheme, strings.Join(Schemes(), ", "))
	}
	return fetcher, nil
}

// Schemes returns the supported scheme names, sorted.
func Schemes() []string {
	return slices.Sorted(maps.Keys(schemes))
}
