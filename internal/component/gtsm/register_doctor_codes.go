// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/component/gtsm.

package gtsm

import "github.com/ze-software/ze/internal/core/diagnostic"

// doctorDiagnosticCodes is the explanation metadata for the doctor codes this package owns.
var doctorDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:        doctorKernelStateCode,
		Title:       "GTSM peer kernel state missing",
		Description: "A BGP peer or dynamic group range configured with `connection ttl` (RFC 5082 GTSM) is short of the kernel state that protects it: the host route to the peer carrying hop limit 255, or a term of the ze_gtsm nftables table, either the one that drops TCP from the peer to Ze's listen port below its TTL or hop-limit floor, or one that drops an ICMP error arriving below the floor. Inside the daemon (show doctor) the check reads what the kernel holds against what the daemon published and names each missing term; the usual cause is a peer whose interface was down at config apply, or a table removed by hand. Restart Ze to republish the state, because a reload leaves an unchanged peer set alone: the finding gives the restart for this process, a systemd command only when ze runs under ze.service, otherwise the steps for a ze started another way. In `ze doctor`, before a start, the check reports a peer the kernel resolves no route to, because that route install will fail at apply.",
		Examples:    []string{"ze doctor --json", "ze show diagnostic doctor-gtsm-kernel-state"},
	},
}

func init() {
	for _, meta := range doctorDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: gtsm: doctor code registration refused: " + err.Error())
		}
	}
}
