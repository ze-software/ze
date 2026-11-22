// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/component/bgp/plugins/role.

package role

import "github.com/ze-software/ze/internal/core/diagnostic"

// doctorDiagnosticCodes is the explanation metadata for the doctor codes this package owns.
var doctorDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:  doctorPeerNoRoleCode,
		Title: "peers and dynamic groups declare no RFC 9234 role",
		Description: "One or more peers, or dynamic groups, carry no `role { import ... }` block. A declared role states the " +
			"relationship the session carries, and Ze then inserts the RFC 7454 Section 9 upstream check " +
			"(bgp-role:upstream-asn), which rejects a route whose AS path carries one of your transit " +
			"providers' ASNs, and the RFC 9234 Only-To-Customer leak protection. A peer with no role declares " +
			"no relationship, so neither runs on it: the session is accepted and runs with whatever filters " +
			"it names. This is a warning so the omission is visible rather than silent. Remedy: declare " +
			"`role { import provider|customer|peer|rs|rs-client }` on each peer, or leave it out deliberately.",
		Examples: []string{"ze doctor --json", "ze show diagnostic doctor-bgp-peer-no-role"},
	},
}

func init() {
	for _, meta := range doctorDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: role: doctor code registration refused: " + err.Error())
		}
	}
}
