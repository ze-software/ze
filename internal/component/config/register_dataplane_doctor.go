// Design: docs/architecture/doctor-and-health-checks.md -- doctor code and parsed-config check registration
// Related: dataplane_verdict.go -- CodeDoctorDataPlaneMismatch and dataPlaneDoctorCheck

package config

import "github.com/ze-software/ze/internal/core/diagnostic"

func init() {
	if err := diagnostic.Register(diagnostic.CodeMeta{
		Code:  CodeDoctorDataPlaneMismatch,
		Title: "Data-plane compatibility mismatch",
		Description: "The configuration combines features, FIB writers, or backend selectors that do not match the selected data plane, uses a feature node whose ze:backend declaration excludes the active backend, or sets a selector leaf to a backend name this build does not register. " +
			"The config doors (start, reload, commit, and `ze config validate`) refuse the same configuration with the same problem, reason, and fix command.",
		Examples: []string{"ze doctor --json", "ze show diagnostic " + CodeDoctorDataPlaneMismatch},
	}); err != nil {
		panic("BUG: config: doctor code registration failed: " + err.Error())
	}
	if err := diagnostic.RegisterDoctorCheck(dataPlaneDoctorCheck()); err != nil {
		panic("BUG: config: doctor check registration failed: " + err.Error())
	}
}
