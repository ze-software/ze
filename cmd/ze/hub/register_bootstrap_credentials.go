// Design: docs/guide/authentication.md -- local recovery credential readiness.
// Related: bootstrap_credentials.go -- canonical reader and report-bus producer.

package hub

import "github.com/ze-software/ze/internal/core/diagnostic"

func init() {
	if err := diagnostic.RegisterDoctorCheck(diagnostic.DoctorCheck{
		Name:         "hub-local-admin-credentials",
		Phase:        diagnostic.DoctorPhasePreConfig,
		Order:        140,
		Component:    localAdminCredentialSource,
		Dependencies: []string{"storage"},
		Platforms:    []string{diagnostic.DoctorPlatformAny},
		Codes:        []string{diagnostic.CodeDoctorLocalAdminCredentials},
		Check:        checkLocalAdminCredentials,
	}); err != nil {
		panic("BUG: local credential doctor registration refused: " + err.Error())
	}
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: diagnostic.CodeDoctorLocalAdminCredentials}); err != nil {
		panic("BUG: local credential event registration refused: " + err.Error())
	}
}
