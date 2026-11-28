// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/component/ike/engine.

package engine

import "github.com/ze-software/ze/internal/core/diagnostic"

const (
	codeDoctorIPsecIface                    = "doctor-ipsec-iface"
	codeDoctorIPsecUDPEncap                 = "doctor-ipsec-udp-encap"
	codeDoctorIPsecCertURL                  = "doctor-ipsec-cert-url"
	codeDoctorIPsecCertURLDenied            = "doctor-ipsec-cert-url-denied"
	codeDoctorIPsecCookieThreshold          = "doctor-ipsec-cookie-threshold"
	codeDoctorIPsecImplicitDiscard          = "doctor-ipsec-implicit-discard"
	codeDoctorIPsecNoPMTUDisc               = "doctor-ipsec-no-pmtu-disc"
	codeDoctorIPsecOverrideGuard            = "doctor-ipsec-override-guard"
	codeDoctorIPsecSocketPolicy             = "doctor-ipsec-socket-policy"
	codeDoctorIPsecESPFormOffload           = "doctor-ipsec-esp-form-offload"
	codeDoctorIPsecESPFormOffloadUnknown    = "doctor-ipsec-esp-form-offload-unknown"
	codeDoctorIPsecForwardDefault           = "doctor-ipsec-forward-default"
	codeDoctorIPsecStateScope               = "doctor-ipsec-state-scope"
	codeDoctorIPsecRedistributionSource     = "doctor-ipsec-redistribution-source"
	codeDoctorIPsecChildInstallRefused      = "doctor-ipsec-child-install-refused"
	codeDoctorIPsecSPDInstallRefused        = "doctor-ipsec-spd-install-refused"
	codeDoctorIPsecSPDCatchAllMissing       = "doctor-ipsec-spd-catch-all-missing"
	codeDoctorIPsecSPDExemptionMissing      = "doctor-ipsec-spd-exemption-missing"
	codeDoctorIPsecSourceGuardMissing       = "doctor-ipsec-source-guard-missing"
	codeDoctorIPsecSPDForwardBypass         = "doctor-ipsec-spd-forward-bypass" //nolint:gosec // G101: a doctor diagnostic code, not a credential
	codeDoctorIPsecSPDUndoIncomplete        = "doctor-ipsec-spd-undo-incomplete"
	codeDoctorIPsecSPDWriter                = "doctor-ipsec-spd-writer"
	codeDoctorIPsecSPDWriterUnknown         = "doctor-ipsec-spd-writer-unknown"
	codeDoctorIPsecVirtualIPPool            = "doctor-ipsec-virtual-ip-pool"
	codeDoctorIPsecUnauthenticatedInput     = "doctor-ipsec-unauthenticated-input"
	codeDoctorIPsecPeerMalformedMessage     = "doctor-ipsec-peer-malformed-message"
	codeDoctorIPsecInterfaceAddress         = "doctor-ipsec-interface-address"
	codeDoctorIPsecXFRMUnavailable          = "doctor-ipsec-xfrm-unavailable"
	codeDoctorIPsecXFRMUnknown              = "doctor-ipsec-xfrm-unknown"
	codeDoctorIPsecMOBIKESocketNotShared    = "doctor-ipsec-mobike-socket-not-shared"
	codeDoctorIPsecEAPTLSIdentity           = "doctor-ipsec-eap-tls-identity"
	codeDoctorIPsecCertURLFetchFailed       = "doctor-ipsec-cert-url-fetch-failed"
	codeDoctorIPsecChildRemoveFailed        = "doctor-ipsec-child-remove-failed"
	codeDoctorIPsecMessageBuildFailed       = "doctor-ipsec-message-build-failed"
	codeDoctorIPsecSendFailed               = "doctor-ipsec-send-failed"
	codeDoctorIPsecEventEmitFailed          = "doctor-ipsec-event-emit-failed"
	codeDoctorIPsecVolumeCounterReadFailed  = "doctor-ipsec-volume-counter-read-failed"
	codeDoctorIPsecChildHardLifetimeExpired = "doctor-ipsec-child-hard-lifetime-expired"
	exampleDoctorJSON                       = "ze doctor --json"
)

// doctorDiagnosticCodes is the explanation metadata for the doctor codes this package owns.
var doctorDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:  codeDoctorIPsecIface,
		Title: "IPsec interface missing",
		Description: "vpn ipsec interface names an interface that does not exist on the system, " +
			"or whose name is malformed. IPsec resolves the local address of every peer that has " +
			"no explicit local-address from this interface, so those peers will not establish.",
		Examples: []string{exampleDoctorJSON, "ze show diagnostic doctor-ipsec-iface"},
	},
	{
		Code:  codeDoctorIPsecUDPEncap,
		Title: "IPsec UDP encapsulation not available",
		Description: "ze cannot receive UDP-encapsulated ESP on the IKE NAT-T port 4500. Either the " +
			"socket would not bind, so ze does not hold that port, or it bound and does not carry " +
			"the UDP_ENCAP option, so the kernel will not decapsulate ESP that arrives inside UDP. " +
			"RFC 7296 Section 2.23 requires an implementation that supports NAT traversal to process " +
			"UDP-encapsulated ESP at any time. A tunnel through a NAT will establish and then carry " +
			"no traffic. A bind failure usually means another process holds port 4500. On Linux a " +
			"UDP_ENCAP failure usually means the setsockopt call was refused. On another platform it " +
			"means the host has no IPsec dataplane at all.",
		Examples: []string{exampleDoctorJSON, "ze show diagnostic doctor-ipsec-udp-encap"},
	},
	{
		Code:  codeDoctorIPsecCertURL,
		Title: "IPsec certificate URL unusable",
		Description: "A peer has hash-and-url set, and its certificate-url is not a URL ze can " +
			"publish its certificate at. RFC 7296 Section 3.6 requires support for the http scheme " +
			"for hash-and-url lookup, and ze refuses every other scheme before any name resolution. " +
			"A peer that asked for hash-and-url and cannot name a reachable http URL will fall back " +
			"to nothing: the certificate never reaches the remote end and IKE_AUTH fails there.",
		Examples: []string{exampleDoctorJSON, "ze show diagnostic doctor-ipsec-cert-url"},
	},
	{
		Code:  codeDoctorIPsecCertURLDenied,
		Title: "IPsec certificate URL denied by the fetcher",
		Description: "A peer's certificate-url names a destination the hash-and-url fetcher refuses: " +
			"a loopback, private, link-local or multicast address, or the cloud metadata address. " +
			"That deny list exists because the fetch is made on behalf of a peer that is NOT yet " +
			"authenticated, and the daemon runs on a router holding routes an internet host does " +
			"not. The URL ze publishes must be reachable by the PEER, so a private address is " +
			"usually a configuration mistake. Name the prefix in certificate-url-allow when it is " +
			"deliberate.",
		Examples: []string{exampleDoctorJSON, "ze show diagnostic doctor-ipsec-cert-url-denied"},
	},
	{
		Code:  codeDoctorIPsecCookieThreshold,
		Title: "IPsec cookie-threshold can never be reached",
		Description: "cookie-threshold is higher than the number of peers configured to accept an " +
			"inbound initiation. The responder challenges an IKE_SA_INIT once its count of " +
			"half-open handshakes meets the threshold, and each responding peer holds at most " +
			"one half-open slot, so the count cannot climb above the number of those peers. The " +
			"threshold is therefore never met, no initiation is ever challenged, and the COOKIE " +
			"defense RFC 7296 Section 2.6 offers against state and CPU exhaustion from forged " +
			"source addresses is off. Lower cookie-threshold to the responding-peer count or " +
			"below, or leave it at the default of 0 to challenge every inbound initiation.",
		Examples: []string{exampleDoctorJSON, "ze show diagnostic doctor-ipsec-cookie-threshold"},
	},
	{
		Code:  codeDoctorIPsecImplicitDiscard,
		Title: "The IPsec SPD catch-all discards because vpn ipsec unmatched is not set",
		Description: "The running configuration leaves `vpn ipsec unmatched` unset, and its default is " +
			"discard (RFC 4301 Section 4.4.1), so every packet this router sends, receives or forwards " +
			"that no SPD entry names is dropped: its own BGP, SSH, DNS and NTP traffic, its ICMP and " +
			"its IPv6 neighbor discovery outside a tunnel. A commit that turns this on without saying " +
			"so is refused, but a configuration the daemon starts with is not verified. Add a " +
			"`vpn ipsec policy <name>` bypass entry for each flow that must stay in the clear, then " +
			"`set vpn ipsec unmatched discard` to confirm, or `set vpn ipsec unmatched bypass` to pass " +
			"that traffic.",
		Examples: []string{exampleDoctorJSON, "ze show diagnostic doctor-ipsec-implicit-discard"},
	},
	{
		Code:  codeDoctorIPsecNoPMTUDisc,
		Title: "The kernel clears DF on df-bit copy IPsec peers",
		Description: "net.ipv4.ip_no_pmtu_disc is not 0, and while it is set the Linux kernel " +
			"(__xfrm_init_state) marks every IPv4 IPsec state Ze installs XFRM_STATE_NOPMTUDISC, " +
			"so the tunnel header carries DF clear. A peer configured with df-bit copy, the " +
			"default, then gets clear without any error, and the DF treatment RFC 4301 Section " +
			"8.1 lets the operator choose per SA is not the one configured; `show vpn ipsec " +
			"dataplane sa` reads df-bit clear. The same warning names a host where the sysctl " +
			"cannot be read. Fix: `set sysctl setting net.ipv4.ip_no_pmtu_disc value 0` and " +
			"commit, or set the peers to df-bit clear when clear is wanted, then rekey or restart " +
			"the tunnels already up.",
		Examples: []string{exampleDoctorJSON, "ze show diagnostic doctor-ipsec-no-pmtu-disc"},
	},
	{
		Code:  codeDoctorIPsecOverrideGuard,
		Title: "IPsec application-override guard not attached",
		Description: "vpn ipsec application-override is deny, but the cgroup setsockopt guard that " +
			"refuses per-socket IPsec policies to every process but Ze is not attached. Any process " +
			"holding CAP_NET_ADMIN in Ze's network namespace can then set IP_XFRM_POLICY or " +
			"IPV6_XFRM_POLICY on its socket and take that socket out of every SPD entry, the " +
			"DISCARD catch-all included (RFC 4301 Section 4.4.1). A process holding CAP_BPF or " +
			"CAP_SYS_ADMIN can detach the guard. It is also raised, carrying the apply's refusal, when " +
			"an apply asked for deny and the guard could not attach (no cgroup2 mounted, no CAP_BPF): " +
			"follow the fix that refusal names. Otherwise restart ze; if this persists, run " +
			"ze doctor --json and read the attach error.",
		Examples: []string{exampleDoctorJSON, "ze show diagnostic doctor-ipsec-override-guard"},
	},
	{
		Code:  codeDoctorIPsecSocketPolicy,
		Title: "A per-socket IPsec policy overrides the SPD",
		Description: "Under vpn ipsec application-override deny, the kernel holds a per-socket IPsec " +
			"policy that is not Ze's own. It was set before the guard attached, or while allow " +
			"held, and it overrides the SPD for its socket until the socket closes: no netlink request " +
			"removes it, and the kernel does not name the owning process. Restart the " +
			"applications started before Ze. show vpn ipsec dataplane policy lists it with the " +
			"role socket-override.",
		Examples: []string{exampleDoctorJSON, "show vpn ipsec dataplane policy"},
	},
	{
		Code:  codeDoctorIPsecESPFormOffload,
		Title: "A loaded ESP GRO module can consume bare ESP before Ze receives it",
		Description: "The IPv4 dual-form receiver has a watched SPI and esp4_offload is loaded. Its GRO callback " +
			"can send bare ESP into XFRM before tc ingress or raw sockets, so an ESP-in-UDP state can refuse it " +
			"before either Ze receive path sees it. The finding's fix stops ze, unloads esp4_offload with " +
			"sudo modprobe -r esp4_offload, then starts ze again, which removes Ze's SAs before unloading their " +
			"referenced module; it names the systemd commands only when ze runs under ze.service. " +
			"Other IPsec owners may still hold or load the module; stop them too or disable GRO on receiving " +
			"links through the existing interface offload configuration. This check only detects the loaded " +
			"module, never an ESP GRO implementation built into the kernel.",
		Examples: []string{exampleDoctorJSON, "ze show diagnostic doctor-ipsec-esp-form-offload"},
	},
	{
		Code:  codeDoctorIPsecESPFormOffloadUnknown,
		Title: "Ze cannot determine whether a loaded ESP GRO module affects bare ESP",
		Description: "The engine cannot read the dual-form receiver's state or its loaded-module list. " +
			"It reports the observation as unknown rather than claiming esp4_offload is absent. The finding " +
			"names the failed observation and its command: the dataplane warning log for receiver state, or " +
			"sudo mount -t proc proc /proc to restore missing procfs. Where procfs is already mounted, make " +
			"its module list readable in the engine's namespace. A kernel without module support and ESP GRO " +
			"built into the kernel require a separate check of the kernel's build configuration.",
		Examples: []string{exampleDoctorJSON, "ze show diagnostic doctor-ipsec-esp-form-offload-unknown"},
	},
	{
		Code:        codeDoctorIPsecForwardDefault,
		Title:       "IPsec forwarding-default dependency or restoration failed",
		Description: "The IKE process cannot read or restore its namespace's final forwarding disposition, or another policy manager changed the value it holds.",
		Examples:    []string{exampleDoctorJSON, "ze show diagnostic doctor-ipsec-forward-default"},
	},
	{
		Code:        codeDoctorIPsecStateScope,
		Title:       "IPsec complete state-scope support cannot be verified",
		Description: "Configured IPsec peers need complete per-SA traffic selector, ICMP quote, ICMP reverse and fragment checks, the mapping of a forwarded ICMP error an out discard covers into its return Child SA, and the payload check of a forwarded ICMP error that meets an operator bypass's transit gate. The running kernel did not provide a complete, readable capability receipt. Child SA installation is refused until Ze's native runtime kernel is rebuilt and booted; the runtime doctor finding preserves the exact cause and native repair commands.",
		Examples:    []string{exampleDoctorJSON, "ze show diagnostic doctor-ipsec-state-scope"},
	},
	{
		Code:        codeDoctorIPsecRedistributionSource,
		Title:       "IPsec redistribution lost a source publication",
		Description: "A Child SA source identity could not be allocated or safely inherited at rekey. Ze refused the publication instead of aliasing another Child SA. The forwarding installation remains independent, but redistribution may retain or miss a route. The runtime finding retains the first refusal until Ze restarts, because the IPsec producer has no replay that can reconstruct its complete live set.",
		Examples:    []string{"ze show doctor", "ze show diagnostic doctor-ipsec-redistribution-source"},
	},
	{
		Code:  codeDoctorIPsecChildInstallRefused,
		Title: "IPsec peer has no Child SA: this node refused the one the peer accepted",
		Description: "The peer accepted a Child SA, and this node could not build, install or bind its " +
			"own half: the esp-group has no proposal ze can use, the keys could not be derived, the " +
			"kernel refused the install, the vti bind names no XFRM interface, the ze-espform " +
			"device cannot be created, another peer holds an overlapping tunnel, or the peer's answer is " +
			"one this node cannot use. Ze deleted the peer's half and kept the IKE SA (RFC 7296 Section " +
			"2.21.2), so no traffic of that peer is protected until a Child SA exists. Ze asks again " +
			"after a wait that doubles after each refusal, up to five minutes, and the first attempt " +
			"after the cause is removed installs the Child SA with no restart. The doctor finding names " +
			"the peer, the error, the next attempt and the command that fixes the cause.",
		Examples: []string{exampleDoctorJSON, "ze show diagnostic doctor-ipsec-child-install-refused"},
	},
	{
		Code:  codeDoctorIPsecSPDInstallRefused,
		Title: "An operator SPD entry is not installed: the dataplane refused it at start",
		Description: "The configuration the daemon started with holds a `vpn ipsec policy` entry the " +
			"dataplane refused to install. A start never refuses the IPsec block, so Ze installed every " +
			"other entry, left this one out whole and logged the refusal at Error. The traffic the entry " +
			"names meets the next entry of the SPD instead (RFC 4301 Section 4.4.1). A commit that " +
			"changes `vpn ipsec` installs the entry again, and so does a restart; a commit whose entry the " +
			"dataplane refuses is refused itself and keeps the running entries. The doctor finding names " +
			"the entry, the dataplane's answer and the command that retries.",
		Examples: []string{exampleDoctorJSON, "ze show diagnostic doctor-ipsec-spd-install-refused"},
	},
	{
		Code:  codeDoctorIPsecSPDCatchAllMissing,
		Title: "The SPD catch-all is not in force: the dataplane refused it at start",
		Description: "The daemon started with a `vpn ipsec` block whose SPD catch-all (`vpn ipsec unmatched`, " +
			"discard unless set) the dataplane refused to install, wholly or in part. A start never refuses the " +
			"IPsec block, so Ze installed every catch-all entry the dataplane accepted and the rest of the " +
			"configuration, and logged the refusal at Error. Under `discard` the default discard is NOT in " +
			"force: traffic no SPD entry matches is passed in the clear by the kernel instead of being " +
			"discarded (RFC 4301 Section 4.4.1). The finding names the cause the kernel's answer gives: EPERM " +
			"is a daemon without CAP_NET_ADMIN, EPROTONOSUPPORT a kernel without XFRM (CONFIG_XFRM_USER), " +
			"EACCES a security module's denial, and any other answer is named as one Ze does not classify. " +
			"Its fix is that cause's, and ends in the restart for this process, a systemd command only when " +
			"ze runs under ze.service, otherwise the steps for a ze started another way. A commit that " +
			"changes `vpn ipsec` installs the catch-all again too, and is refused if the dataplane still refuses " +
			"it. The same holds for the final entry of each XFRM interface a peer binds with `vti bind`, " +
			"which is always a discard: the finding then names the interface whose default discard is not in " +
			"force, and Ze hands that interface no traffic from the node-wide SPD. The doctor finding names the " +
			"disposition or the interface and the dataplane's answer for each refused entry.",
		Examples: []string{exampleDoctorJSON, "ze show diagnostic doctor-ipsec-spd-catch-all-missing"},
	},
	{
		Code:  codeDoctorIPsecSPDExemptionMissing,
		Title: "A bound XFRM interface's SPD exemption is not installed",
		Description: "A peer binds an XFRM interface with `vti bind`, and under `unmatched discard` Ze installs " +
			"if_id 0 exemptions that hand the traffic routed into that interface on to the interface's own SPD. " +
			"The dataplane refused one, so that traffic meets the node-wide catch-all and is dropped: the tunnel " +
			"carries none of it. The entry fails closed, so no commit is refused for it. The next commit or " +
			"address change retries it, and the finding clears when it goes in. The finding names the cause " +
			"the kernel's answer gives (EPERM a daemon without CAP_NET_ADMIN, EPROTONOSUPPORT a kernel without " +
			"CONFIG_XFRM_USER, EACCES a security module's denial) and that cause's fix. The doctor finding names the interface, " +
			"the direction and the dataplane's answer, and its fix ends in the restart for this process: a " +
			"systemd command only when ze runs under ze.service, otherwise the steps for a ze started another way.",
		Examples: []string{exampleDoctorJSON, "ze show diagnostic doctor-ipsec-spd-exemption-missing"},
	},
	{
		Code:  codeDoctorIPsecSourceGuardMissing,
		Title: "A tunnel-source guard is not installed, so its IPv6 source exemption is not either",
		Description: "Under `unmatched discard` Ze gives each IPv6 address of an XFRM interface a peer binds with " +
			"`vti bind` an if_id 0 source exemption, which hands IPv6 host traffic from that address on to the " +
			"interface's own SPD. Ze installs that exemption only while the address's tunnel-source guard holds: " +
			"a rule `from <address>/128` into a routing table Ze owns for the interface, and the address's terms " +
			"in the nft table ze_ipsec_source_guard. The kernel refused one of the two, so the exemption stays " +
			"uninstalled, and IPv6 host traffic from that address routed into the interface meets the node-wide " +
			"DISCARD catch-all: the tunnel carries none of it. The entry fails closed, so no commit is refused for " +
			"it. The next commit or address change retries the refused step, and the finding clears when it goes " +
			"in. The finding names the interface, the address, the refused step and the cause the kernel's answer " +
			"gives (EPERM a daemon without CAP_NET_ADMIN; EAFNOSUPPORT for the rule a kernel without " +
			"CONFIG_IPV6_MULTIPLE_TABLES; EPROTONOSUPPORT or EOPNOTSUPP for the nft table a kernel without " +
			"CONFIG_NF_TABLES; EACCES a security module's denial), and that cause's fix, which ends in the " +
			"restart for this process: a systemd command only when ze runs under ze.service, otherwise the steps " +
			"for a ze started another way.",
		Examples:     []string{exampleDoctorJSON, "ze show diagnostic doctor-ipsec-source-guard-missing"},
		RelatedCodes: []string{codeDoctorIPsecSPDExemptionMissing},
	},
	{
		Code:  codeDoctorIPsecSPDForwardBypass,
		Title: "An operator SPD bypass with a port does not decide forwarded traffic",
		Description: "The running configuration holds a `vpn ipsec policy` bypass with a local or remote " +
			"port whose local prefix is not one address of this host, on a dataplane that checks " +
			"forwarded traffic in its own lookup (Linux XFRM). Linux checks a forwarded packet before it " +
			"reassembles it and reads port 0 from every fragment but the first, and RFC 4301 Section 7.4 " +
			"requires stateful fragment checking for such a bypass, which Ze does not perform for " +
			"forwarded traffic yet. Ze therefore installs the entry for traffic to and from Ze only: " +
			"forwarded traffic on its ports meets the next entry, and under `unmatched discard` it is " +
			"dropped. A commit refuses such an entry; a start installs it and logs it at Error, and this " +
			"check also reports it when an address the entry named has left the host. The doctor finding " +
			"names the entry and the command that sets its port to any; a discard with a port, or a bypass " +
			"whose local prefix is one address of this host, is accepted too.",
		Examples: []string{exampleDoctorJSON, "ze show diagnostic doctor-ipsec-spd-forward-bypass"},
	},
	{
		Code:  codeDoctorIPsecSPDUndoIncomplete,
		Title: "The IPsec SPD does not match the running configuration",
		Description: "The dataplane holds a `vpn ipsec policy` entry in a form the running configuration " +
			"does not. Either the dataplane refused an entry, Ze took back what that commit or start had " +
			"written, and one step of that undo failed, so the dataplane lacks the running form of an " +
			"entry or still holds one that was never accepted; or an accepted commit changed or removed " +
			"an entry and the dataplane refused to remove its previous form, which still acts on its " +
			"traffic. Traffic that entry names meets the wrong disposition until the SPD is rebuilt. " +
			"The doctor finding names the direction, the selector, the priority and the dataplane's " +
			"answer. Restart the daemon so it installs the running configuration whole: the finding's fix " +
			"gives the restart for this process, a systemd command only when ze runs under ze.service, " +
			"otherwise the steps for a ze started another way.",
		Examples: []string{exampleDoctorJSON, "ze show diagnostic doctor-ipsec-spd-undo-incomplete"},
	},
	{
		Code:  codeDoctorIPsecSPDWriter,
		Title: "Other processes can write the IPsec SPD",
		Description: "Processes other than Ze hold CAP_NET_ADMIN in Ze's network namespace. Linux " +
			"admits an XFRM netlink write from any such process, and no hook separates it from Ze, " +
			"so each can add or remove SPD entries directly. vpn ipsec application-override " +
			"governs per-socket policies only. Remove CAP_NET_ADMIN from those processes or stop them.",
		Examples: []string{exampleDoctorJSON, "ze show diagnostic doctor-ipsec-spd-writer"},
	},
	{
		Code:  codeDoctorIPsecSPDWriterUnknown,
		Title: "Ze cannot tell which processes can write the IPsec SPD",
		Description: "Ze could not read /proc, or Linux refused to show some processes' network " +
			"namespace or capabilities (procfs not mounted, hidepid, or no ptrace access to them). " +
			"Ze then cannot tell which other processes hold CAP_NET_ADMIN in its network namespace, " +
			"and each such process can add or remove SPD entries directly. Mount /proc without " +
			"hidepid and run ze as root with CAP_SYS_PTRACE, then run ze doctor again.",
		Examples: []string{exampleDoctorJSON, "ze show diagnostic doctor-ipsec-spd-writer-unknown"},
	},
	{
		Code:  codeDoctorIPsecVirtualIPPool,
		Title: "IPsec virtual-IP pool nearly exhausted",
		Description: "At least 90% of the addresses one family of a remote-access pool can lease " +
			"are held by live IKE SAs. A lease lives as long as its IKE SA, so once the range is " +
			"full every further remote-access client bound to the pool is refused with " +
			"INTERNAL_ADDRESS_FAILURE (RFC 7296 Section 3.15.4) and gets no tunnel. Widen range " +
			"or range6 under vpn ipsec remote-access pool <name>, or bind some peers to another " +
			"pool with virtual-ip-pool.",
		Examples: []string{exampleDoctorJSON, "ze show diagnostic doctor-ipsec-virtual-ip-pool"},
	},
	{
		Code:  codeDoctorIPsecUnauthenticatedInput,
		Title: "IPsec handshake receiving messages that fail their integrity check",
		Description: "The IKE handshake of a configured peer that has not established dropped at " +
			"least one message that failed its integrity check. Such a message carries no proof " +
			"that it came from the peer: the SPIs and the Message ID travel in the clear, so anyone " +
			"who saw one datagram can build it. RFC 7296 Section 2.4 forbids ending the handshake " +
			"on it, so Ze drops it, keeps the handshake and waits for the peer's genuine answer. " +
			"The tunnel comes up late, or not at all if forgeries keep arriving until the " +
			"retransmissions run out. Someone is sending datagrams under the SPIs of this " +
			"handshake, or the path to the peer damages them. Each drop is counted in the " +
			"handshake-unauthenticated series; read it with show metrics values | match " +
			"handshake-unauthenticated, and check the path to the peer for a device that rewrites " +
			"or corrupts IKE datagrams on UDP 500 and 4500.",
		Examples: []string{exampleDoctorJSON, "ze show diagnostic doctor-ipsec-unauthenticated-input"},
	},
	{
		Code:  codeDoctorIPsecPeerMalformedMessage,
		Title: "IPsec peer sends authenticated IKE messages that do not parse",
		Description: "The peer of an IKE SA, established or in its RFC 4739 second " +
			"authentication exchange, sent messages whose integrity check " +
			"verified and whose payloads do not parse. Only the peer holds the SA's keys, and the " +
			"check covers every octet of the message, so a path that damaged the datagram would " +
			"have failed that check: the defect is in the peer's IKE implementation. Ze drops " +
			"such a response, or a retransmission it would otherwise answer from its cache, and " +
			"keeps the SA; a malformed request is answered with INVALID_SYNTAX and ends the SA " +
			"(RFC 7296 Section 2.21.3). An IKE_AUTH response whose Padding or Pad Length is wrong " +
			"keeps the SA in the RFC 4739 second exchange, but in the first authentication exchange " +
			"it ends the handshake, so ze doctor never reports it: the WARN line that carries this " +
			"code is its only record, and the next cycle starts a new handshake. " +
			"The exchange the peer meant is lost, so a rekey, a " +
			"liveness check, an address update or the second authentication exchange can " +
			"fail. Run show vpn ipsec sa to see the SA, " +
			"collect the peer's IKE log for the same time, and report the defect to the peer's " +
			"vendor.",
		Examples: []string{exampleDoctorJSON, "ze show diagnostic doctor-ipsec-peer-malformed-message"},
	},
	{
		Code:  codeDoctorIPsecInterfaceAddress,
		Title: "IPsec peer waiting for its interface address",
		Description: "vpn ipsec interface names an interface that has no IPv4 address, so every " +
			"site-to-site peer without a local-address of its own is not started: it has no " +
			"address to send from. The peer starts by itself when an address appears on the " +
			"interface (a PPPoE or DHCP WAN that is not up yet). Bring the interface up, or give " +
			"the peer an address with set vpn ipsec site-to-site peer <name> local-address <ipv4>.",
		Examples: []string{exampleDoctorJSON, "ze show diagnostic doctor-ipsec-interface-address"},
	},
	{
		Code:  codeDoctorIPsecXFRMUnavailable,
		Title: "IPsec XFRM dataplane unavailable",
		Description: "The configuration installs an IPsec Security Association and the kernel holds " +
			"no XFRM dataplane. Ze installs every Child SA and every IPsec policy through XFRM, so " +
			"a tunnel would negotiate to the point of success and then carry no traffic. This is an " +
			"ERROR: `ze` refuses to start on it and `ze config validate` fails on it. The probe " +
			"opens the XFRM netlink socket, which needs no CAP_NET_ADMIN, and it reports absence " +
			"only for the errno that means the kernel carries no XFRM protocol; a probe that could " +
			"not ask reports doctor-ipsec-xfrm-unknown instead and does not refuse the start. " +
			"Remedy: run a kernel built with CONFIG_XFRM_USER and CONFIG_INET_ESP. " +
			"CONFIG_XFRM_USER alone gives the netlink socket interface and accepts every SA install while " +
			"CONFIG_INET_ESP carries the packets, so a kernel with the first and not the second " +
			"passes this probe and still encrypts nothing; only the appliance build requirement " +
			"catches that (internal/appliance/kernelreq.go). An empty `vpn { ipsec { } }` block " +
			"installs no SA and is never reported here.",
		Examples: []string{exampleDoctorJSON, "ze show diagnostic doctor-ipsec-xfrm-unavailable"},
	},
	{
		Code:  codeDoctorIPsecXFRMUnknown,
		Title: "IPsec XFRM dataplane undetermined",
		Description: "The configuration installs an IPsec Security Association and ze could not " +
			"establish whether the kernel holds an XFRM dataplane: the probe's socket open failed " +
			"for a reason other than the kernel carrying no XFRM protocol. This is a WARNING and it " +
			"does not refuse a start, because refusing on a question ze could not ask would stop a " +
			"router whose kernel is fine. Off Linux it is the standing answer: ze installs no SA " +
			"there, so there is no dataplane to find. Remedy: read the reason the message carries. " +
			"A permission denial is a privilege fault rather than a kernel one, and rebuilding a " +
			"kernel for it changes nothing.",
		Examples: []string{exampleDoctorJSON, "ze show diagnostic doctor-ipsec-xfrm-unknown"},
	},
	{
		Code:  codeDoctorIPsecMOBIKESocketNotShared,
		Title: "IPsec MOBIKE peer restarted: the reopened sockets lack the shared NAT-T socket",
		Description: "The interface address a MOBIKE peer uses moved, and the socket pair Ze reopened at the new " +
			"address did not take the one port-4500 socket Ze shares between addresses. The peer's IKE SA sends " +
			"from that shared socket alone, so MOBIKE cannot move it, and Ze restarts the peer from the new " +
			"address instead: its tunnel carries no traffic until it renegotiates. Ze shares the socket only while " +
			"the dataplane can migrate a tunnel and the ze.test.ike.port test override is unset, and that answer " +
			"changed after the peer's session opened. This is an ERROR event, raised once per peer and move. " +
			"Remedy: unset ze.test.ike.port if it is set, then restart Ze so every socket follows one " +
			"decision. The event's fix gives the restart for this process: a systemd command only when ze " +
			"runs under ze.service, otherwise the steps for a ze started another way.",
		Examples:     []string{exampleDoctorJSON, "ze show diagnostic doctor-ipsec-mobike-socket-not-shared"},
		RelatedCodes: []string{"doctor-ipsec-mobike-unavailable"},
	},
	{
		Code:  codeDoctorIPsecEAPTLSIdentity,
		Title: "IPsec EAP-TLS peer has no certificate identity constraint",
		Description: "A site-to-site peer using mode eap-tls has no certificate identity constraint " +
			"(allowed-identities on a respond peer, or server-identity / an unambiguous FQDN " +
			"remote-address on an initiate peer). RFC 5216 Section 5.3 requires both EAP-TLS roles " +
			"to authorize certificate identities rather than accepting any certificate signed by " +
			"ca-certificate. Validation, commit, and reload refuse this configuration fail-closed, " +
			"and a daemon started from an unvalidated file refuses the handshake until an identity " +
			"constraint is configured.",
		Examples: []string{exampleDoctorJSON, "ze show diagnostic doctor-ipsec-eap-tls-identity"},
	},
	{
		Code:  codeDoctorIPsecCertURLFetchFailed,
		Title: "IPsec hash-and-URL peer certificate fetch failed",
		Description: "Ze could not fetch a peer's certificate from the HTTP URL announced in a hash-and-URL " +
			"CERT payload (RFC 7296 Section 3.6). Until the certificate can be fetched, IKE_AUTH for " +
			"that peer cannot verify the peer's certificate chain and handshake retries stay blocked. " +
			"Verify that the peer's certificate URL is reachable from this router and permitted by " +
			"`vpn ipsec certificate-url-allow`, then inspect `ze show vpn ipsec sa`.",
		Examples:     []string{exampleDoctorJSON, fixShowIPsecSA, "ze show diagnostic doctor-ipsec-cert-url-fetch-failed"},
		RelatedCodes: []string{codeDoctorIPsecCertURL, codeDoctorIPsecCertURLDenied},
	},
	{
		Code:  codeDoctorIPsecChildRemoveFailed,
		Title: "IPsec Child SA removal failed in the kernel dataplane",
		Description: "The kernel refused to remove an inbound or outbound Child SA state during teardown or " +
			"rekey for a reason other than the state already being absent (ENOENT/ESRCH). A stranded " +
			"XFRM state remains in the kernel SAD and can interfere with subsequent installs or traffic " +
			"accounting. Inspect the kernel SAD with `ze show vpn ipsec dataplane sa` and verify Ze holds " +
			"CAP_NET_ADMIN.",
		Examples: []string{exampleDoctorJSON, fixShowIPsecDataplaneSA, "ze show diagnostic doctor-ipsec-child-remove-failed"},
	},
	{
		Code:  codeDoctorIPsecMessageBuildFailed,
		Title: "IPsec IKE encrypted message could not be built",
		Description: "Ze could not encrypt or encode an IKEv2 Delete, INFORMATIONAL response, or error " +
			"notification for a configured peer's SA. Because the message was not sent, the peer may " +
			"keep a stale SA until DPD liveness timeout or miss an error notification. Inspect the " +
			"peer's SA state with `ze show vpn ipsec sa` and check recent IKE warnings.",
		Examples: []string{exampleDoctorJSON, fixShowIPsecSA, "ze show diagnostic doctor-ipsec-message-build-failed"},
	},
	{
		Code:  codeDoctorIPsecSendFailed,
		Title: "IPsec IKE datagram could not be sent to peer",
		Description: "The IKE engine could not transmit an IKEv2 datagram or DF-clear path probe on the " +
			"peer's UDP transport socket. While sends fail, DPD liveness checks, rekeys, Deletes, and " +
			"path MTU probes cannot reach the peer and the tunnel may time out or stall. Inspect tunnel " +
			"status with `ze show vpn ipsec sa` and verify the route and local interface to the peer.",
		Examples: []string{exampleDoctorJSON, fixShowIPsecSA, "ze show diagnostic doctor-ipsec-send-failed"},
	},
	{
		Code:  codeDoctorIPsecEventEmitFailed,
		Title: "IPsec Child SA lifecycle event could not be published",
		Description: "The IKE engine could not emit a Child SA lifecycle event (child-up, child-down, or " +
			"child-rekey) on the plugin event bus. Downstream subscribers watching IPsec Child SA state " +
			"changes missed that transition, though the dataplane SA itself follows its own install " +
			"outcome. Inspect tunnel status with `ze show vpn ipsec sa` and check bus health with " +
			"`ze show doctor`.",
		Examples: []string{exampleDoctorJSON, fixShowIPsecSA, "ze show diagnostic doctor-ipsec-event-emit-failed"},
	},
	{
		Code:  codeDoctorIPsecVolumeCounterReadFailed,
		Title: "IPsec Child SA volume counters could not be read from the dataplane",
		Description: "The IKE engine could not read per-SA byte and packet counters from the dataplane " +
			"while enforcing a configured `life-bytes` or `life-packets` Child SA volume lifetime. " +
			"Until counter reads succeed, volume-based soft rekey and hard teardown cannot advance " +
			"for that Child SA, leaving only the time-based `lifetime` active. Inspect the dataplane " +
			"SAD with `ze show vpn ipsec dataplane sa` and verify the active dataplane exposes SA counters.",
		Examples: []string{exampleDoctorJSON, fixShowIPsecDataplaneSA, "ze show diagnostic doctor-ipsec-volume-counter-read-failed"},
	},
	{
		Code:  codeDoctorIPsecChildHardLifetimeExpired,
		Title: "IPsec Child SA hard lifetime expired before rekey completed",
		Description: "An established IPsec Child SA reached its configured hard lifetime (time, bytes, " +
			"or packets) before a replacement SA completed rekeying. Per RFC 7296 Section 2.8, the " +
			"engine tore down the expired Child SA and restarted negotiation with the peer. Check " +
			"tunnel status with `ze show vpn ipsec sa` and verify peer reachability and lifetime " +
			"margins.",
		Examples: []string{exampleDoctorJSON, fixShowIPsecSA, "ze show diagnostic doctor-ipsec-child-hard-lifetime-expired"},
	},
}

func init() {
	for _, meta := range doctorDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: engine: doctor code registration refused: " + err.Error())
		}
	}
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: codeDoctorIPsecSPDUndoIncomplete}); err != nil {
		panic("BUG: engine: SPD undo event code registration refused: " + err.Error())
	}
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: codeDoctorIPsecCertURLFetchFailed}); err != nil {
		panic("BUG: engine: cert URL fetch event code registration refused: " + err.Error())
	}
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: codeDoctorIPsecChildRemoveFailed}); err != nil {
		panic("BUG: engine: child remove event code registration refused: " + err.Error())
	}
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: codeDoctorIPsecMessageBuildFailed}); err != nil {
		panic("BUG: engine: message build event code registration refused: " + err.Error())
	}
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: codeDoctorIPsecSendFailed}); err != nil {
		panic("BUG: engine: send failed event code registration refused: " + err.Error())
	}
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: codeDoctorIPsecEventEmitFailed}); err != nil {
		panic("BUG: engine: event emit failed event code registration refused: " + err.Error())
	}
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: codeDoctorIPsecVolumeCounterReadFailed}); err != nil {
		panic("BUG: engine: volume counter read failed event code registration refused: " + err.Error())
	}
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: codeDoctorIPsecChildHardLifetimeExpired}); err != nil {
		panic("BUG: engine: child hard lifetime expired event code registration refused: " + err.Error())
	}
}
