// Design: plan/spec-pki-full-chain.md -- doctor check registration for the web component
// Related: doctor.go -- checkWebTLSCertificate, the check registered here
//
// The web component is not a plugin, so it registers through the component-style
// path (diagnostic.RegisterDoctorCheck) rather than a plugin Registration's
// DoctorChecks field (ai/rules/repo-maintenance.md, "Components that are not
// plugins"). Keeping the check and its registration under internal/component/web
// means removing the web component removes its diagnostic with it.

package web

import (
	"fmt"
	"os"

	"github.com/ze-software/ze/internal/core/diagnostic"
)

func init() {
	check := diagnostic.DoctorCheck{
		Name:         "web-tls-certificate",
		Phase:        diagnostic.DoctorPhasePostConfig,
		Order:        725,
		Component:    "web",
		Dependencies: []string{"external-binary"},
		Platforms:    []string{diagnostic.DoctorPlatformAny},
		Codes:        []string{"doctor-tls-reference", "doctor-tls-expired"},
		Check:        checkWebTLSCertificate,
	}
	if err := diagnostic.RegisterDoctorCheck(check); err != nil {
		fmt.Fprintf(os.Stderr, "web: doctor check registration: %v\n", err)
		os.Exit(2)
	}
}
