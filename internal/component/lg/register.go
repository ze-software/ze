// Design: docs/architecture/pki/tls-listeners.md -- doctor check registration for the looking glass
// Related: doctor.go -- checkLGTLSCertificate, the check registered here
//
// The looking glass is a component rather than a plugin, so it registers on the
// direct path (diagnostic.RegisterDoctorCheck) instead of through a plugin
// Registration's DoctorChecks field (ai/rules/repo-maintenance.md, "Components
// that are not plugins").
//
// This file deliberately carries NO build tag, and it needs none. The lg package
// is linked only through cmd/ze/hub/register_lg.go, which carries
// //go:build ze_lg, so a binary built without that tag never runs this init and
// `ze doctor` never names a component that binary does not carry.

package lg

import (
	"os"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
)

func init() {
	check := diagnostic.DoctorCheck{
		Name:         "lg-tls-certificate",
		Phase:        diagnostic.DoctorPhasePostConfig,
		Order:        727,
		Component:    "lg",
		Dependencies: []string{"external-binary"},
		Platforms:    []string{diagnostic.DoctorPlatformAny},
		Codes:        []string{"doctor-tls-reference", "doctor-tls-expired"},
		Check:        checkLGTLSCertificate,
	}
	err := diagnostic.RegisterDoctorCheck(check)
	if err == nil {
		return
	}

	// A registration failure means `ze doctor` would silently stop reporting a
	// broken certificate reference on the listener Ze publishes to strangers.
	// Refuse the process rather than start one whose diagnostics have a hole.
	var tb textbuf.Buffer
	tb.Str("lg: doctor check registration: ").Err(err).Byte('\n')
	tb.StdErr() //nolint:errcheck // the process exits next; a failed stderr write changes nothing
	os.Exit(2)
}
