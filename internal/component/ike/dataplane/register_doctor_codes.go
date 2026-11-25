// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/component/ike/dataplane.

package dataplane

import "github.com/ze-software/ze/internal/core/diagnostic"

const (
	// CodeDoctorIPsecMOBIKEUnavailable is the doctor diagnostic code "doctor-ipsec-mobike-unavailable".
	CodeDoctorIPsecMOBIKEUnavailable    = "doctor-ipsec-mobike-unavailable"
	codeDoctorIPsecMOBIKEUnknown        = "doctor-ipsec-mobike-unknown"
	codeDoctorIPsecESPUnavailable       = "doctor-ipsec-esp-unavailable"
	codeDoctorIPsecESPUnknown           = "doctor-ipsec-esp-unknown"
	codeDoctorIPsecTransformUnavailable = "doctor-ipsec-transform-unavailable"
	codeDoctorIPsecTransformUnknown     = "doctor-ipsec-transform-unknown"
	// CodeDoctorIPsecESPFormFastPath is the doctor diagnostic code "doctor-ipsec-esp-form-fast-path".
	CodeDoctorIPsecESPFormFastPath = "doctor-ipsec-esp-form-fast-path"
	// CodeDoctorIPsecESPFormLink is the doctor diagnostic code "doctor-ipsec-esp-form-link".
	CodeDoctorIPsecESPFormLink = "doctor-ipsec-esp-form-link"
	// CodeDoctorIPsecESPFormTUN is the doctor diagnostic code "doctor-ipsec-esp-form-tun".
	CodeDoctorIPsecESPFormTUN = "doctor-ipsec-esp-form-tun"
	// CodeDoctorIPsecESPFormReaderStopped is the doctor diagnostic code "doctor-ipsec-esp-form-reader-stopped".
	CodeDoctorIPsecESPFormReaderStopped = "doctor-ipsec-esp-form-reader-stopped"
	// CodeDoctorIPsecESPFormFirewall is the doctor diagnostic code "doctor-ipsec-esp-form-firewall".
	CodeDoctorIPsecESPFormFirewall = "doctor-ipsec-esp-form-firewall"
	exampleDoctorJSON              = "ze doctor --json"
)

// doctorDiagnosticCodes is the explanation metadata for the doctor codes this package owns.
var doctorDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:  CodeDoctorIPsecMOBIKEUnavailable,
		Title: "IPsec MOBIKE migration unavailable",
		Description: "The configuration installs an IPsec Security Association and the kernel does not " +
			"handle XFRM_MSG_MIGRATE_STATE, which first appears in Linux 7.2 and needs " +
			"CONFIG_XFRM_MIGRATE. Ze moves a live Child SA to a new address pair with that message " +
			"alone, so without it Ze does not offer MOBIKE (RFC 4555) and a peer whose address " +
			"changes must renegotiate. This is a WARNING and it does not refuse a start. A Docker " +
			"host that runs Ze's labs is refused on it, because the MOBIKE scenarios cannot pass " +
			"there. The probe sends a migration naming no SA, so it changes nothing. Remedy: run a " +
			"kernel built with CONFIG_XFRM_MIGRATE from Linux 7.2 on, such as Ze's own runtime kernel.",
		Examples:     []string{exampleDoctorJSON, "ze show diagnostic doctor-ipsec-mobike-unavailable"},
		RelatedCodes: []string{codeDoctorIPsecMOBIKEUnknown},
	},
	{
		Code:  codeDoctorIPsecMOBIKEUnknown,
		Title: "IPsec MOBIKE migration undetermined",
		Description: "Ze could not establish whether the kernel handles XFRM_MSG_MIGRATE_STATE: the " +
			"probe needs CAP_NET_ADMIN, and the kernel answered with an errno that neither proves " +
			"nor rules out the handler. Ze does not offer MOBIKE until the answer is present. This " +
			"is a WARNING and it does not refuse a start. Remedy: run `ze doctor` as root.",
		Examples:     []string{exampleDoctorJSON, "ze show diagnostic doctor-ipsec-mobike-unknown"},
		RelatedCodes: []string{CodeDoctorIPsecMOBIKEUnavailable},
	},
	{
		Code:  codeDoctorIPsecESPUnavailable,
		Title: "IPsec ESP unavailable for an address family",
		Description: "The configuration installs an IPsec Security Association and the kernel holds " +
			"no ESP transform type for IPv4 (CONFIG_INET_ESP) or IPv6 (CONFIG_INET6_ESP), which the " +
			"message names. The XFRM netlink socket alone accepts the request and the kernel then has nothing " +
			"to encrypt with, so a Child SA of that family fails to install. This is a WARNING and " +
			"it does not refuse a start, because the other family may still work. The probe asks " +
			"the kernel to replace an SA on a reserved SPI that cannot exist, so it installs " +
			"nothing. Remedy: load esp4 or esp6, or run a kernel built with the symbol named.",
		Examples:     []string{exampleDoctorJSON, "ze show diagnostic doctor-ipsec-esp-unavailable"},
		RelatedCodes: []string{codeDoctorIPsecESPUnknown, "doctor-ipsec-xfrm-unavailable"},
	},
	{
		Code:  codeDoctorIPsecESPUnknown,
		Title: "IPsec ESP support undetermined",
		Description: "Ze could not establish whether the kernel holds ESP for the address family the " +
			"message names. The probe needs CAP_NET_ADMIN and builds an AES-GCM state, so a denied " +
			"request, or a kernel missing AES-GCM itself, leaves the question open. This is a " +
			"WARNING and it does not refuse a start. Remedy: run `ze doctor` as root and read the " +
			"reason the message carries.",
		Examples:     []string{exampleDoctorJSON, "ze show diagnostic doctor-ipsec-esp-unknown"},
		RelatedCodes: []string{codeDoctorIPsecESPUnavailable},
	},
	{
		Code:  codeDoctorIPsecTransformUnavailable,
		Title: "IPsec ESP transform unavailable",
		Description: "The kernel cannot run a cryptographic transform Ze's XFRM backend installs for " +
			"the algorithm words the message names. A Child SA negotiated with one of those words " +
			"fails to install. The transforms are derived from the backend's own algorithm tables, " +
			"so every word Ze can install is asked about. This is a WARNING and it does not refuse " +
			"a start, because a proposal naming another algorithm still installs. A Docker host " +
			"that runs Ze's labs is refused on it. Remedy: run a kernel built with the symbol the " +
			"message names, or load its module.",
		Examples:     []string{exampleDoctorJSON, "ze show diagnostic doctor-ipsec-transform-unavailable"},
		RelatedCodes: []string{codeDoctorIPsecTransformUnknown},
	},
	{
		Code:  codeDoctorIPsecTransformUnknown,
		Title: "IPsec ESP transform undetermined",
		Description: "Ze could not establish whether the kernel runs the transform the message names. " +
			"The probe needs CAP_NET_ADMIN and an ESP transform type, so a denied request, or a " +
			"kernel without ESP, leaves the question open. This is a WARNING and it does not " +
			"refuse a start. Remedy: run `ze doctor` as root and read the reason the message carries.",
		Examples:     []string{exampleDoctorJSON, "ze show diagnostic doctor-ipsec-transform-unknown"},
		RelatedCodes: []string{codeDoctorIPsecTransformUnavailable},
	},
	{
		Code:  CodeDoctorIPsecESPFormFastPath,
		Title: "IPsec bare ESP is not rewritten in the kernel",
		Description: "A Child SA on UDP port 4500 receives bare ESP as well as ESP-in-UDP (RFC 7296 " +
			"Section 2.23). Ze normally turns bare ESP into the form the SA's kernel state carries with " +
			"a tc ingress BPF program on each link, the fast path. The fast path did not load, or it " +
			"stopped following new links, so that bare ESP goes through ze-espform instead, the " +
			"complete path: at a bounded rate, above which every datagram is dropped, and a " +
			"firewall rule keyed on the input " +
			"interface sees ze-espform, not the link the packet arrived on. Every packet is still " +
			"checked against the SA's policy. The finding says why the program did not load. A " +
			"capability fault is a load the kernel refused with EPERM: ze lacks CAP_BPF or " +
			"CAP_NET_ADMIN. A kernel fault is a kernel before Linux 6.6, which has no TCX, so no " +
			"capability makes the program load: the fix builds Ze's runtime kernel, which has it, " +
			"to install and boot. Any other cause is one Ze cannot classify, and the fix names the " +
			"warnings that carry the kernel's answer. A fast path that detached itself, or stopped " +
			"following new links, loads again when ze restarts. For a missing capability and for a " +
			"restart, the finding's fix gives the remedy for this process: a systemd command only " +
			"when ze runs under ze.service, otherwise the steps for a ze started another way.",
		Examples: []string{exampleDoctorJSON, "ze show diagnostic doctor-ipsec-esp-form-fast-path"},
	},
	{
		Code:  CodeDoctorIPsecESPFormLink,
		Title: "IPsec bare ESP on one link is not rewritten in the kernel",
		Description: "The IPsec ESP form fast path is not attached to one link: the link's type has " +
			"no program variant (only Ethernet and links that carry IP with no link header have " +
			"one), or the kernel refused the attach. Bare ESP that arrives on that link for a Child " +
			"SA on UDP port 4500 goes through ze-espform, the complete path: at a bounded rate, " +
			"above which every datagram is dropped, " +
			"and a firewall rule keyed on the input interface sees ze-espform, not that " +
			"link. Every packet is still checked against the SA's policy. To keep that traffic off " +
			"the complete path, set the peer to always encapsulate ESP in UDP (strongSwan: encap = " +
			"yes), or route it over a link of a supported type.",
		Examples: []string{exampleDoctorJSON, "ze show diagnostic doctor-ipsec-esp-form-link"},
	},
	{
		Code:  CodeDoctorIPsecESPFormTUN,
		Title: "IPsec Child SAs on UDP port 4500 are refused: no usable ze-espform interface",
		Description: "A Child SA on UDP port 4500 must receive bare ESP as well as ESP-in-UDP (RFC " +
			"7296 Section 2.23). The bare ESP the fast path cannot rewrite (fragments, IPv4 options, " +
			"a link it does not serve) goes through ze-espform, the complete path, a TUN interface " +
			"Ze creates from /dev/net/tun. Ze cannot create a usable one here: the TUN device does " +
			"not open, or /proc/sys is read-only and the interface would take from " +
			"net.ipv4.conf.default a disable_policy that skips the inbound policy check or a strict " +
			"rp_filter that drops what Ze writes into it. So Ze refuses every Child SA on UDP port " +
			"4500 rather than install one that receives a single ESP form, or one whose bare ESP " +
			"skips the policy check. Ze checks this on every configuration apply that configures a " +
			"peer. The doctor finding names the cause (a container started without the device, a " +
			"kernel with no TUN driver, a missing device node, no CAP_NET_ADMIN, or the sysctl " +
			"setting) and the command that fixes it. The complete path also reads bare ESP through " +
			"a raw ESP socket, and the receiver records its last failure to open the socket or the " +
			"interface, at a Child SA install, a MOBIKE move, or its own reopen after the socket " +
			"failed; the doctor reports that record under this code, with the cause (no CAP_NET_RAW " +
			"or CAP_NET_ADMIN, or a step of the interface's configuration) and its fix, until an " +
			"open succeeds.",
		Examples: []string{exampleDoctorJSON, "ze show diagnostic doctor-ipsec-esp-form-tun"},
	},
	{
		Code:  CodeDoctorIPsecESPFormReaderStopped,
		Title: "IPsec bare ESP went unreceived for a while: Ze's own receiver for bare ESP stopped",
		Description: "A Child SA on UDP port 4500 must receive bare ESP as well as ESP-in-UDP (RFC " +
			"7296 Section 2.23). The bare ESP the fast path cannot rewrite reaches the SA only " +
			"through Ze's own receiver, which reads it from a raw ESP socket and writes it into " +
			"ze-espform. That socket failed, so until Ze opened the receiver again, such bare ESP " +
			"was not received. Ze opens it again by itself, at once and then further apart if it " +
			"stops again soon after each open. The event is raised on the report bus each time the " +
			"receiver stops, even when it opened again at once; when it cannot open, the doctor also " +
			"reports a doctor-ipsec-esp-form-tun error with the cause. The receiver's warnings name " +
			"the error the socket answered and each attempt to open it again.",
		Examples: []string{exampleDoctorJSON, "ze show errors", "ze show diagnostic doctor-ipsec-esp-form-reader-stopped"},
	},
	{
		Code:  CodeDoctorIPsecESPFormFirewall,
		Title: "IPsec bare ESP is not rewritten in the kernel: the firewall could drop ESP",
		Description: "The IPsec ESP form fast path rewrites bare ESP for a Child SA on UDP port 4500 " +
			"at tc ingress, before netfilter, so a firewall rule on the input path would not see " +
			"the ESP it was written for. Ze reads the nftables ruleset and keeps the fast path off " +
			"while an ingress, prerouting or input chain holds a rule or a drop policy that could " +
			"drop ESP, and while it cannot read the ruleset or follow its changes. Bare ESP then " +
			"goes through ze-espform, the complete path, after the input path, so the firewall " +
			"still decides on it. The complete path carries a bounded number of datagrams per " +
			"second, which the doctor finding names, and drops every datagram above it before " +
			"XFRM sees it; a firewall rule keyed on the input interface sees ze-espform, not the " +
			"link the packet arrived on. The finding names the first blocking site. To keep the " +
			"fast path, and only if that ESP should be accepted, put a term that accepts protocol " +
			"esp first in that chain; otherwise keep the firewall as it is and accept the bound. " +
			"When the ruleset could not be read, or the monitor that follows its changes was " +
			"closed, restart ze. When only following its changes failed, no restart is needed: " +
			"Ze follows them again by itself, trying at most 30 seconds apart, and while the " +
			"finding stays, ze show log recent level warn component ike.dataplane names why it " +
			"keeps failing.",
		Examples: []string{exampleDoctorJSON, "ze show diagnostic doctor-ipsec-esp-form-firewall"},
	},
}

func init() {
	for _, meta := range doctorDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: dataplane: doctor code registration refused: " + err.Error())
		}
	}
}
