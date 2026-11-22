// Design: docs/research/l2tpv2-ze-integration.md -- RADIUS AAA registration
// Related: aaa.go -- RADIUS AAA backend implementation
// Related: doctor.go -- system/authentication/radius reachability check

package radius

import (
	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

func init() {
	if err := aaa.Default.Register(radiusBackend{}); err != nil {
		panic("BUG: radius aaa: registration failed: " + err.Error())
	}
	if err := diagnostic.RegisterDoctorCheck(radiusAdminDoctorCheck); err != nil {
		panic("BUG: radius aaa: doctor check registration failed: " + err.Error())
	}
}
