// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/component/ike/transport.

package transport

import "github.com/ze-software/ze/internal/core/diagnostic"

const (
	// CodeDoctorIPsecIKESocketBypass is the doctor diagnostic code "doctor-ipsec-ike-socket-bypass".
	CodeDoctorIPsecIKESocketBypass = "doctor-ipsec-ike-socket-bypass"
)

// doctorDiagnosticCodes is the explanation metadata for the doctor codes this package owns.
var doctorDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:  CodeDoctorIPsecIKESocketBypass,
		Title: "An IKE socket is not exempt from IPsec",
		Description: "Ze sets a per-socket IPsec bypass on each IKE socket it opens (RFC 4301 Section 5.2), " +
			"so the SPD never captures its own IKE traffic. The kernel refused it on this socket, " +
			"usually because Ze runs without CAP_NET_ADMIN or the kernel lacks CONFIG_XFRM. Ze kept " +
			"the socket: IKE still flows wherever no policy captures it, but under `vpn ipsec " +
			"unmatched discard`, or a traffic selector covering the peer's IKE address, the SPD " +
			"captures Ze's own IKE and the tunnel cannot be negotiated, rekeyed or torn down. The " +
			"doctor finding names the socket and the kernel's answer. Run ze with CAP_NET_ADMIN on a " +
			"kernel with CONFIG_XFRM, then restart it: the finding's fix gives the restart for this " +
			"process, a systemd command only when ze runs under ze.service, otherwise the steps for a ze " +
			"started another way.",
		Examples: []string{"ze doctor --json", "ze show diagnostic doctor-ipsec-ike-socket-bypass"},
	},
}

func init() {
	for _, meta := range doctorDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: transport: doctor code registration refused: " + err.Error())
		}
	}
}
