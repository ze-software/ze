// Design: docs/research/l2tpv2-ze-integration.md -- RADIUS AAA registration
// Related: aaa.go -- RADIUS AAA backend implementation
// Related: doctor.go -- system/authentication/radius reachability check

package radius

import (
	"fmt"
	"os"

	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

func init() {
	if err := aaa.Default.Register(radiusBackend{}); err != nil {
		fmt.Fprintf(os.Stderr, "radius aaa: registration failed: %v\n", err)
		os.Exit(1)
	}
	if err := diagnostic.RegisterDoctorCheck(radiusAdminDoctorCheck); err != nil {
		fmt.Fprintf(os.Stderr, "radius aaa: doctor check registration failed: %v\n", err)
		os.Exit(1)
	}
}
