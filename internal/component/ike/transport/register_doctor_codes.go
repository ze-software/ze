// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/component/ike/transport.

package transport

import "github.com/ze-software/ze/internal/core/diagnostic"

const (
	// CodeDoctorIPsecIKESocketBypass is the doctor diagnostic code "doctor-ipsec-ike-socket-bypass".
	CodeDoctorIPsecIKESocketBypass = "doctor-ipsec-ike-socket-bypass"
	// CodeDoctorIPsecNATKeepaliveFailed is the doctor diagnostic code "doctor-ipsec-nat-keepalive-failed".
	CodeDoctorIPsecNATKeepaliveFailed = "doctor-ipsec-nat-keepalive-failed"
)

// doctorDiagnosticCodes is the explanation metadata for the doctor codes this package owns.
var doctorDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:  CodeDoctorIPsecIKESocketBypass,
		Title: "An IKE socket is not exempt from IPsec",
		Description: "Ze sets a per-socket IPsec bypass on each IKE socket it opens (RFC 4301 Section 5.2), " +
			"so the SPD never captures its own IKE traffic. The kernel refused it on this socket. Ze kept " +
			"the socket: IKE still flows wherever no policy captures it, but under `vpn ipsec " +
			"unmatched discard`, or a traffic selector covering the peer's IKE address, the SPD " +
			"captures Ze's own IKE and the tunnel cannot be negotiated, rekeyed or torn down. The " +
			"doctor finding names the socket, the kernel's answer and the cause that answer shows, " +
			"and its fix is for that cause. EPERM: ze lacks CAP_NET_ADMIN; under ze.service the fix " +
			"reinstalls the unit and starts ze again, otherwise it restarts ze holding the capability. " +
			"ENOPROTOOPT: the kernel is built without CONFIG_XFRM; boot a kernel built with it (Ze's own " +
			"kernel carries it), then restart ze. EACCES: a security module (SELinux or AppArmor) refused " +
			"the socket option; allow it in that policy, then restart ze. Any other answer: Ze cannot " +
			"tell the cause, and the fix names both the capability and the kernel. Each restart is " +
			"worded for this process: a systemd command only when ze runs under ze.service, otherwise " +
			"the steps for a ze started another way.",
		Examples: []string{"ze doctor --json", "ze show diagnostic doctor-ipsec-ike-socket-bypass"},
	},
	{
		Code:  CodeDoctorIPsecNATKeepaliveFailed,
		Title: "IPsec NAT-T keepalive packet could not be sent",
		Description: "Ze could not transmit a periodic RFC 3948 NAT-T keepalive packet to an IPsec peer. " +
			"While keepalives fail to leave the socket, intermediate NAT mappings can expire and stop " +
			"inbound IKE and ESP traffic until the peer renegotiates. Inspect the tunnel and local " +
			"uplink with `ze show vpn ipsec sa` and verify the NAT-T socket and route to the peer.",
		Examples: []string{"ze doctor --json", "ze show vpn ipsec sa", "ze show diagnostic doctor-ipsec-nat-keepalive-failed"},
	},
}

func init() {
	for _, meta := range doctorDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: transport: doctor code registration refused: " + err.Error())
		}
	}
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: CodeDoctorIPsecNATKeepaliveFailed}); err != nil {
		panic("BUG: transport: NAT keepalive event code registration refused: " + err.Error())
	}
}
