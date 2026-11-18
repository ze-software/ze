//go:build linux

// Design: docs/architecture/doctor-and-health-checks.md -- dependency explanation.
// Related: pppoe/register_original_ppp_support_linux.go -- configured check enrollment.

package kernel

import "github.com/ze-software/ze/internal/core/diagnostic"

const originalAttachmentCheckName = "ppp-original-unit-attachment"
const originalAttachmentDoctorCode = "doctor-ppp-original-unit-attachment"

func init() {
	if err := diagnostic.Register(diagnostic.CodeMeta{
		Code: originalAttachmentDoctorCode, Title: "Required original-resource PPP support is unavailable or unknown",
		Description: "On configured native PPP roles, a fresh unbound /dev/ppp file must return a valid query42 ABI1 declaration with capture8+connect6, and bridge7 for a bridge role. ENOTTY or missing required bits is unavailable; permission, malformed replies, inspection and cleanup failures are Unknown. This is compiled ABI availability, never a live owner or operation permission receipt. Unit51/channel46/transport44 admission remains mandatory.",
		Examples:    []string{"ze doctor", "ze show diagnostic " + originalAttachmentDoctorCode}, HostVerdict: true,
	}); err != nil {
		panic("BUG: original PPP support diagnostic registration failed: " + err.Error())
	}
}
