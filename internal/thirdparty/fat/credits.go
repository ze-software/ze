// Design: docs/architecture/third-party-credits.md -- what a carrying package declares
//
// credits.go is Ze's own file in this copied package. It is kept apart from
// the copied sources so the copy stays as gokrazy wrote it, under gokrazy's
// licence (LICENSE beside it).

package fat

import "github.com/ze-software/ze/internal/core/credits"

// This package is a copy of gokrazy's FAT writer, so a build that links it
// credits that work.
func init() {
	credits.DeclarePresent("gokrazy FAT writer")
}
