// Design: ai/rules/doctor-checks.md -- managed hub reachability check registration
// Related: doctor.go -- the hub-unreachable reachability check registered here

package managed

import (
	"fmt"
	"os"

	"github.com/ze-software/ze/internal/core/diagnostic"
)

func init() {
	if err := diagnostic.RegisterDoctorCheck(hubDoctorCheck); err != nil {
		fmt.Fprintf(os.Stderr, "managed: doctor check registration failed: %v\n", err)
		os.Exit(1)
	}
}
