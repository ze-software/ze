// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/component/bgp/config.

package bgpconfig

import "github.com/ze-software/ze/internal/core/diagnostic"

const (
	exampleDoctorJSON = "ze doctor --json"
)

// doctorDiagnosticCodes is the explanation metadata for the doctor codes this package owns.
var doctorDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:        codeFilterReferenceUndefined,
		Title:       "Dangling config reference",
		Description: "A filter chain in BGP config references a policy name that is not defined under the top-level policy root.",
		Examples:    []string{exampleDoctorJSON},
	},
	{
		Code:         diagnosticBGPPeerConfig,
		Title:        "BGP peer configuration rejected",
		Description:  "The BGP engine's own peer resolution refuses this configuration, so the daemon will fail to start on it -- an unknown address family, a missing mandatory setting (prefix maximum, connection local ip), or an unresolvable cross-reference. Doctor runs the same gate `ze config validate` applies; before this check existed it reported such a config as ready and exited 0, which is the operator trap it closes. Severity is error: the report is not ready and `ze doctor` exits 1. Remedy: run `ze config validate <file>` for the full error list and correct the named peer.",
		Examples:     []string{exampleDoctorJSON, "ze config validate ze.conf", "ze show diagnostic doctor-config-bgp-peer"},
		RelatedCodes: []string{diagnostic.CodeConfigBGPPeer, diagnostic.CodeConfigBGPResolve},
	},
	{
		Code:        diagnosticBGPMD5,
		Title:       "BGP TCP MD5 not supported",
		Description: "A BGP peer has TCP MD5 authentication configured but the platform does not support TCP_MD5SIG.",
		Examples:    []string{exampleDoctorJSON, "ze show diagnostic doctor-bgp-md5"},
	},
}

func init() {
	for _, meta := range doctorDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: bgpconfig: doctor code registration refused: " + err.Error())
		}
	}
}
