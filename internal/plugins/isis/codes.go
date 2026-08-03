// Design: plan/learned/937-isis-13-cli-diag-interop.md -- IS-IS diagnostic code ownership.
// Related: doctor.go -- the config-sanity check that emits these two codes
// Related: register.go -- registerISISDiagnosticCodes() registers them at init
// Related: transport/doctor.go -- the SEPARATE raw-socket check (isis-3 owns its code)
//
// The IS-IS component OWNS its config-sanity diagnostic codes
// (doctor-isis-net-missing, doctor-isis-system-id-mismatch). Their explanation
// metadata lives here as data; the registration call lives in register.go so the
// side effect stays in the component's registration file (ai/patterns/registration.md).
// Registering here instead of in the central diagnostic.builtinCodes slice means
// deleting the IS-IS component removes the codes with it (ai/rules/
// plugins.md). One code, one owner: the raw-socket code
// (doctor-isis-raw-socket) is owned and listed by the transport, never here.

package isis

import "github.com/ze-software/ze/internal/core/diagnostic"

// isisDiagnosticCodes is the explanation metadata for the IS-IS config-sanity
// codes this component owns, so `ze explain <code>` can describe them.
var isisDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:        codeNETMissing,
		Title:       "IS-IS configured without a NET",
		Description: "The IS-IS block is present but no net (Network Entity Title) is set. IS-IS derives its System ID from the NET and cannot originate LSPs without one (ISO/IEC 10589 section 6.2). Add at least one net, e.g. net 49.0001.0000.0000.0001.00.",
		Examples:    []string{"ze doctor --json", "ze explain doctor-isis-net-missing"},
	},
	{
		Code:        codeSystemIDMismatch,
		Title:       "IS-IS system-id does not match the NET",
		Description: "An explicit IS-IS system-id was configured that disagrees with the 6-octet System ID embedded in the first NET (the octets before the NSEL). Remove the system-id leaf to derive it from the NET, or align the two so the IS uses a single consistent identity.",
		Examples:    []string{"ze doctor --json", "ze explain doctor-isis-system-id-mismatch"},
	},
}
