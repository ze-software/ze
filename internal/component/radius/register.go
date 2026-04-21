// Design: docs/research/l2tpv2-ze-integration.md -- RADIUS AAA registration
// Related: aaa.go -- RADIUS AAA backend implementation

package radius

import (
	"fmt"
	"os"

	"codeberg.org/thomas-mangin/ze/internal/component/aaa"
)

func init() {
	if err := aaa.Default.Register(radiusBackend{}); err != nil {
		fmt.Fprintf(os.Stderr, "radius aaa: registration failed: %v\n", err)
		os.Exit(1)
	}
}
