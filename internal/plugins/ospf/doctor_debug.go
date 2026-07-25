// Design: plan/learned/1052-ospf-ext-14-debug-introspection.md -- the debug-enabled doctor Warning.
// RFC: rfc/short/rfc5250.md / rfc/short/rfc5340.md -- the inject surface this guards.
//
// AC-25: when debug LSA injection is left enabled, `ze doctor` emits a Warning so an
// operator does not leave a router able to originate crafted test LSAs into production. It
// is a DISTINCT ext-14 doctor code (doctor-ospf-debug-enabled); the config-sanity codes are
// untouched. One Warning covers the shared (both address families) enablement. The
// os.Exit-guarded registration lives in register.go (registerOSPFDebugDoctor).

package ospf

import (
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// ospfDebugDoctorCode is the explanation metadata for the ext-14 debug-enabled code.
var ospfDebugDoctorCode = diagnostic.CodeMeta{
	Code:        codeOSPFDebugEnabled,
	Title:       "OSPF debug LSA injection enabled",
	Description: "OSPF debug LSA injection is currently enabled (`debug ospf inject enable`). An authorized operator can originate crafted test LSAs into the local LSDB, which then flood normally. This is a testing/research facility that should be disabled on a production router; run `debug ospf inject disable`.",
	Examples:    []string{"ze doctor --json", "ze explain doctor-ospf-debug-enabled"},
}

// checkOSPFDebugEnabled emits a Warning while the shared debug-injection enablement is on.
func checkOSPFDebugEnabled(_ diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	if !debugInjectIsEnabled() {
		return nil
	}
	return []diagnostic.Diagnostic{{
		Code:     codeOSPFDebugEnabled,
		Severity: diagnostic.SeverityWarning,
		Message:  "ospf debug LSA injection is enabled; disable it on a production router with `debug ospf inject disable`",
	}}
}
