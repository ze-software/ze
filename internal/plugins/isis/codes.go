// Design: docs/architecture/isis/isis-13-cli-diag-interop.md -- IS-IS diagnostic code ownership.
// Related: doctor.go -- the config-sanity check that emits these codes.
// Related: register.go -- registerISISDiagnosticCodes() registers them at init
// Related: transport/doctor.go -- the SEPARATE raw-socket check (isis-3 owns its code)
//
// The IS-IS component OWNS its config-sanity diagnostic codes
// (NET, system-ID and interface-level checks). Their explanation
// metadata lives here as data; the registration call lives in register.go so the
// side effect stays in the component's registration file (ai/patterns/registration.md).
// Registering here instead of in the central diagnostic.builtinCodes slice means
// deleting the IS-IS component removes the codes with it (ai/rules/
// plugins.md). One code, one owner: the raw-socket code
// (doctor-isis-raw-socket) is owned and listed by the transport, never here.

package isis

import "github.com/ze-software/ze/internal/core/diagnostic"

// codeHelloSendFailed is the event code of a circuit whose Hello did not go out
// (circuits.go, helloSendWarn). It is an event, not a config finding: no doctor
// check can see it before the send, so registerISISDiagnosticCodes registers it
// as an event code and the recent-events doctor check reports it.
const codeHelloSendFailed = "doctor-isis-hello-send-failed"

// isisDiagnosticCodes is the explanation metadata for the IS-IS config-sanity
// codes this component owns, so `ze show diagnostic <code>` can describe them.
var isisDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:        codeNETMissing,
		Title:       "IS-IS configured without a NET",
		Description: "The IS-IS block is present but no net (Network Entity Title) is set. IS-IS derives its System ID from the NET and cannot originate LSPs without one (ISO/IEC 10589 section 6.2). Add at least one net, e.g. net 49.0001.0000.0000.0001.00.",
		Examples:    []string{"ze doctor --json", "ze show diagnostic doctor-isis-net-missing"},
	},
	{
		Code:        codeSystemIDMismatch,
		Title:       "IS-IS system-id does not match the NET",
		Description: "An explicit IS-IS system-id was configured that disagrees with the 6-octet System ID embedded in the first NET (the octets before the NSEL). Remove the system-id leaf to derive it from the NET, or align the two so the IS uses a single consistent identity.",
		Examples:    []string{"ze doctor --json", "ze show diagnostic doctor-isis-system-id-mismatch"},
	},
	{
		Code:        codeInterfaceLevelMismatch,
		Title:       "IS-IS interface and instance levels do not overlap",
		Description: "An enabled IS-IS interface requests a level that the instance disables. It cannot form an adjacency or advertise its prefixes at that level. Set the interface level to l1-l2 to use the instance's levels.",
		Examples:    []string{"ze doctor", "ze show diagnostic doctor-isis-interface-level-mismatch"},
	},
	{
		Code:        codeHelloSendFailed,
		Title:       "IS-IS cannot send Hellos on an interface",
		Description: "IS-IS could not send a Hello on an interface at one level: the link is down, the kernel refused the frame, or the Hello is larger than the interface MTU. Without Hellos every neighbor on that interface drops its adjacency when the hold time expires. The log line names the interface, the level and the error. ze_isis_frames_dropped_total{reason=\"send-error\"} counts the frames the kernel refused; a Hello larger than the MTU is refused before the send and is not counted there. ze show warnings lists the warning until a Hello goes out again at that level or the interface is removed. Check the interface with ze show isis interface, and fix the link or raise its MTU.",
		Examples:    []string{"ze show warnings", "ze show isis interface", "ze show diagnostic doctor-isis-hello-send-failed"},
	},
}
