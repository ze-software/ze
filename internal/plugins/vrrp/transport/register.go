// Design: docs/architecture/vrrp/vrrp-first-hop-redundancy.md -- doctor-check registration
//
// The transport registers its raw-socket readiness check via
// diagnostic.RegisterDoctorCheck from init(). The transport is a
// platform-specific raw-socket backend, not its own plugin Registration (the VRRP
// plugin Registration is owned by spec-vrrp-5), so it uses the component-style
// registration path (ai/rules/repo-maintenance.md "Components that are not plugins").
// This keeps the entire check self-contained under transport/.

package transport

import (
	"fmt"
	"os"

	"github.com/ze-software/ze/internal/core/diagnostic"
)

func init() {
	check := diagnostic.DoctorCheck{
		Name:         "vrrp-raw-socket",
		Phase:        diagnostic.DoctorPhasePostConfig,
		Order:        740,
		Component:    "vrrp",
		Dependencies: []string{"external-binary"},
		Platforms:    []string{diagnostic.DoctorPlatformAny},
		Codes:        []string{"doctor-vrrp-raw-socket"},
		Check:        checkVRRPRawSocket,
	}
	if err := diagnostic.RegisterDoctorCheck(check); err != nil {
		fmt.Fprintf(os.Stderr, "vrrp/transport: doctor check registration: %v\n", err)
		os.Exit(2)
	}
}
