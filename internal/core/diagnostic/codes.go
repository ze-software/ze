// Design: docs/features/ai-first.md — built-in diagnostic codes

package diagnostic

// RegisterBuiltinCodes registers all built-in diagnostic codes.
// Called explicitly by the binary entry point, not from init().
func RegisterBuiltinCodes() {
	for _, m := range builtinCodes {
		_ = Register(m)
	}
}

var builtinCodes = []CodeMeta{
	{
		Code:        "config-parse",
		Title:       "Config syntax error",
		Description: "The config file contains a syntax error such as an unknown keyword, missing token, or invalid scalar value.",
		Examples:    []string{"ze config validate --json bad.conf", "ze explain config-parse"},
	},
	{
		Code:         "config-yang-missing",
		Title:        "Missing mandatory field",
		Description:  "A mandatory config field required by the YANG schema is not present.",
		RelatedCodes: []string{"config-yang-type", "config-yang-enum"},
	},
	{
		Code:         "config-yang-type",
		Title:        "Wrong value type",
		Description:  "The config value does not match the type expected by the YANG schema.",
		RelatedCodes: []string{"config-yang-missing", "config-yang-enum"},
	},
	{
		Code:        "config-yang-range",
		Title:       "Value outside allowed range",
		Description: "A numeric config value falls outside the range defined in the YANG schema.",
	},
	{
		Code:        "config-yang-pattern",
		Title:       "Value does not match pattern",
		Description: "A string config value does not match the regular expression pattern defined in the YANG schema.",
	},
	{
		Code:         "config-yang-enum",
		Title:        "Invalid enumeration value",
		Description:  "The config value is not one of the allowed enumeration values defined in the YANG schema.",
		RelatedCodes: []string{"config-yang-type"},
	},
	{
		Code:        "config-yang-length",
		Title:       "String length outside allowed range",
		Description: "A string config value has a length outside the range defined in the YANG schema.",
	},
	{
		Code:        "config-yang-cardinality",
		Title:       "List cardinality violation",
		Description: "A list or leaf-list in the config has too many or too few entries per the YANG schema.",
	},
	{
		Code:        "config-plugin-verify",
		Title:       "Plugin config verification failure",
		Description: "An in-process plugin config verifier rejected the configuration.",
	},
	{
		Code:        "config-mcp-invalid",
		Title:       "MCP config consistency failure",
		Description: "MCP auth-mode, bind-remote, OAuth, or TLS cross-leaf consistency check failed.",
	},
	{
		Code:        "config-bgp-resolve",
		Title:       "BGP config resolution failure",
		Description: "Template or BGP tree resolution failed during config validation.",
	},
	{
		Code:        "config-bgp-authz",
		Title:       "BGP authz profile reference failure",
		Description: "An authorization profile referenced in BGP config does not exist.",
	},
	{
		Code:        "config-bgp-peer",
		Title:       "BGP peer extraction failure",
		Description: "Peer settings, route extraction, or capability constraints failed during config validation.",
	},
	{
		Code:        "config-hub-invalid",
		Title:       "Hub config extraction failure",
		Description: "Plugin hub config extraction (secret length, client blocks) failed.",
	},
	{
		Code:        "config-listener-conflict",
		Title:       "Listener port conflict",
		Description: "Two listeners in the config conflict on the same address and port.",
	},
	{
		Code:        "config-warning",
		Title:       "Config warning",
		Description: "A warning from semantic validation without a more specific diagnostic code.",
	},

	// Doctor diagnostic codes -- platform.
	{
		Code:        "doctor-platform-detect",
		Title:       "Platform detection failed",
		Description: "Runtime platform detection encountered an error.",
		Examples:    []string{"ze doctor --json"},
	},
	{
		Code:        "doctor-platform-unknown",
		Title:       "Unknown runtime platform",
		Description: "The runtime platform could not be identified as gokrazy, systemd, container, or plain Linux.",
		Examples:    []string{"ze doctor --json"},
	},
	{
		Code:        "doctor-platform-perm",
		Title:       "Gokrazy /perm not writable",
		Description: "Running on gokrazy but /perm is not writable. Config persistence and state storage require a writable /perm partition.",
		Examples:    []string{"ze doctor --json"},
	},
	{
		Code:        "doctor-platform-container-ro",
		Title:       "Container with read-only root",
		Description: "Running in a container with a read-only root filesystem. Writable volumes must be mounted for config and state.",
		Examples:    []string{"ze doctor --json"},
	},

	// Doctor diagnostic codes -- storage and config.
	{
		Code:        "doctor-store-integrity",
		Title:       "Store integrity failure",
		Description: "The zefs database has corrupt entries or a container-level error.",
		Examples:    []string{"ze doctor --json"},
	},
	{
		Code:        "doctor-config-missing",
		Title:       "Config file not found",
		Description: "No config file could be resolved from storage. Ze cannot determine which services to check.",
		Examples:    []string{"ze doctor --json"},
	},
	{
		Code:        "doctor-config-parse",
		Title:       "Config parse failure",
		Description: "The config file was found but could not be parsed.",
		Examples:    []string{"ze doctor --json"},
	},
	{
		Code:        "doctor-vpp-unreachable",
		Title:       "VPP socket unreachable",
		Description: "The VPP API socket could not be reached. VPP may not be running.",
		Examples:    []string{"ze doctor --json"},
	},
	{
		Code:        "doctor-module-missing",
		Title:       "Kernel module not loaded",
		Description: "A required kernel module is not loaded.",
		Examples:    []string{"ze doctor --json"},
	},
	{
		Code:        "doctor-storage-unavailable",
		Title:       "Blob storage unavailable",
		Description: "The zefs database could not be opened.",
		Examples:    []string{"ze doctor --json"},
	},
	{
		Code:        "doctor-static-interface-nexthop-no-backend",
		Title:       "Static interface next-hop has no iface backend",
		Description: "A static route forwards over an interface-only next-hop (no gateway address) but the config declares no `interface { backend ... }` stanza. The next-hop interface name cannot be resolved to an ifindex at runtime, so the static section fails to load. Add an interface backend, or give the next-hop a gateway address.",
		Examples:    []string{"ze doctor --json", "ze explain doctor-static-interface-nexthop-no-backend"},
	},
	{
		Code:        "doctor-static-route-skipped",
		Title:       "Static route skipped (rest of section kept programmed)",
		Description: "The running static plugin could not program one or more routes (an absent device, an absent interface backend, or an unresolvable next-hop) and skipped them so the rest of the static section stayed programmed. A skipped prefix is unrouted until its dependency appears; it is re-attempted on the next config apply and clears once it programs. Run `static show` to see which prefixes are skipped and why, then fix the offending next-hop or create the missing device or interface backend.",
		Examples:    []string{"ze doctor --json", "ze explain doctor-static-route-skipped"},
	},
	{
		Code:        "doctor-ddos-detect-no-flow-source",
		Title:       "DDoS characterization has no flow source",
		Description: "ddos-detect is enabled with characterization on, but neither traffic-usage (track-ip) nor flow-export (conntrack) is configured. Detection still works, but mitigation degrades to generic-flood with no target prefix, so responders cannot install a surgical or targeted rule. Enable traffic-usage track-ip and/or flow-export conntrack.",
		Examples:    []string{"ze doctor --json", "ze explain doctor-ddos-detect-no-flow-source"},
	},
	{
		Code:        "doctor-flowexport-conntrack-unavailable",
		Title:       "flow-export conntrack table unavailable",
		Description: "flow-export conntrack export is enabled but nf_conntrack is not loaded (nothing -- no firewall/NAT rule, no modprobe -- pulled it in), so the ctnetlink dump reads an empty table. The recent-flow ring stays empty and DDoS characterization degrades to generic-flood with no discriminating vector. Load nf_conntrack (a firewall or NAT rule does this) so per-flow records are available.",
		Examples:    []string{"ze doctor --json", "ze explain doctor-flowexport-conntrack-unavailable"},
	},
	{
		Code:        "doctor-anomaly-detect-no-feature-source",
		Title:       "Anomaly detector has no feature source",
		Description: "anomaly-detect is enabled, but neither traffic-usage nor flow-export is configured. The trafficfeature layer is fed by the observation feed from these sources; with neither configured it produces no per-source features, so the behavioral detector observes nothing and emits no incidents. Enable traffic-usage and/or flow-export.",
		Examples:    []string{"ze doctor --json", "ze explain doctor-anomaly-detect-no-feature-source"},
	},
	{
		Code:        "doctor-anomaly-shape-armed-no-firewall",
		Title:       "Anomaly responder armed without a firewall",
		Description: "anomaly-shape is in armed mode but no firewall is configured. The responder installs live per-source rate-limit rules via the firewall component; with no firewall the armed actions cannot be applied to the kernel, so autonomous mitigation silently does nothing. Configure a firewall, or keep the responder in shadow mode.",
		Examples:    []string{"ze doctor --json", "ze explain doctor-anomaly-shape-armed-no-firewall"},
	},
	{
		Code:        "doctor-tls-missing",
		Title:       "TLS certificate or key not found",
		Description: "A TLS certificate or key file referenced in the config does not exist.",
		Examples:    []string{"ze doctor --json"},
	},
	{
		Code:        "doctor-tls-expired",
		Title:       "TLS certificate expired",
		Description: "A TLS certificate referenced in the config has expired or is not yet valid.",
		Examples:    []string{"ze doctor --json"},
	},
	{
		Code:        "doctor-tls-invalid",
		Title:       "TLS certificate cannot be parsed",
		Description: "A TLS certificate file is not valid PEM or the DER content cannot be parsed as an X.509 certificate.",
		Examples:    []string{"ze doctor --json"},
	},
	{
		Code:        "doctor-plugin-missing",
		Title:       "Plugin binary not found",
		Description: "An external plugin binary referenced in the config is not on PATH.",
		Examples:    []string{"ze doctor --json"},
	},
	{
		Code:        "doctor-service-unit",
		Title:       "Systemd service unit unreadable",
		Description: "The installed ze systemd unit exists but cannot be read by doctor.",
		Examples:    []string{"ze doctor --json", "ze explain doctor-service-unit"},
	},
	{
		Code:        "doctor-service-executable",
		Title:       "Systemd service executable invalid",
		Description: "The installed ze systemd unit ExecStart does not point to an existing executable file.",
		Examples:    []string{"ze doctor --json", "ze explain doctor-service-executable"},
	},
	{
		Code:        "doctor-service-user",
		Title:       "Systemd service user missing",
		Description: "The user configured in the ze systemd unit does not exist on this host.",
		Examples:    []string{"ze doctor --json", "ze explain doctor-service-user"},
	},
	{
		Code:        "doctor-service-group",
		Title:       "Systemd service group missing",
		Description: "The group configured in the ze systemd unit does not exist on this host.",
		Examples:    []string{"ze doctor --json", "ze explain doctor-service-group"},
	},
	{
		Code:        "doctor-ssh-hostkey-missing",
		Title:       "SSH host key not found",
		Description: "The SSH host key file could not be found at the expected path.",
		Examples:    []string{"ze doctor --json"},
	},
	{
		Code:        "doctor-listen-unavailable",
		Title:       "Listen address unavailable",
		Description: "A configured listen address/port could not be bound.",
		Examples:    []string{"ze doctor --json"},
	},
	{
		Code:        "doctor-iface-missing",
		Title:       "Configured interface not found",
		Description: "An ethernet interface named in the config does not exist on the system.",
		Examples:    []string{"ze doctor --json"},
	},
	{
		Code:        "doctor-config-reference",
		Title:       "Dangling config reference",
		Description: "A filter chain in BGP config references a policy name that is not defined under bgp/policy.",
		Examples:    []string{"ze doctor --json"},
	},
	{
		Code:        "doctor-disk-space",
		Title:       "Low disk space on config partition",
		Description: "The partition containing the config directory has less than 5% free space. The zefs database may fail to write.",
		Examples:    []string{"ze doctor --json"},
	},
	{
		Code:        "doctor-dns-resolver",
		Title:       "No DNS resolver responding",
		Description: "None of the name servers configured under system/name-server responded to a query.",
		Examples:    []string{"ze doctor --json"},
	},
	{
		Code:        "doctor-iface-down",
		Title:       "Configured interface is down",
		Description: "An ethernet interface named in the config exists but its link is not up.",
		Examples:    []string{"ze doctor --json"},
	},
	{
		Code:        "doctor-clock-skew",
		Title:       "System clock skewed",
		Description: "The system clock differs from NTP by more than 5 minutes. TLS validation, BGP timers, and log correlation may be affected.",
		Examples:    []string{"ze doctor --json"},
	},
	{
		Code:        "doctor-vpp-version",
		Title:       "VPP version check failed",
		Description: "The VPP version could not be determined or is not compatible with the expected API.",
		Examples:    []string{"ze doctor --json"},
	},
	{
		Code:        "doctor-vpp-wireguard",
		Title:       "VPP wireguard plugin not enabled",
		Description: "A wireguard interface is configured under the vpp backend, but the VPP wireguard plugin is not enabled (vpp.plugins.wireguard) or not loaded in the running VPP. VPP cannot program wireguard tunnels without it, so the interface will fail at apply.",
		Examples:    []string{"ze doctor --json", "ze explain doctor-vpp-wireguard"},
	},
	{
		Code:        "doctor-vpp-lcp-netns",
		Title:       "LCP netns unreachable by BGP",
		Description: "The VPP Linux Control Plane TAPs are created in a network namespace (vpp.lcp.netns) that ze's BGP listener does not run in, so BGP cannot bind on an LCP-shadowed interface. No value of the leaf avoids this: VPP resolves vpp.lcp.netns as a namespace name under /var/run/netns/, and ze also passes it as VPP's global default netns, so host and root name namespaces that normally do not exist and LCP pair creation fails. The only remedy today is to run ze in the same namespace as the TAPs; netns-aware BGP binding is specced but not implemented.",
		Examples:    []string{"ze doctor --json", "ze explain doctor-vpp-lcp-netns"},
	},
	{
		Code:        "doctor-kernel-nexthop",
		Title:       "Kernel nexthop objects unavailable",
		Description: "The kernel does not support nexthop objects (requires Linux 5.3+). ECMP will fall back to multipath routes.",
		Examples:    []string{"ze doctor --json"},
	},
	{
		Code:        "doctor-mpls-unavailable",
		Title:       "MPLS kernel support unavailable",
		Description: "MPLS kernel modules (mpls_router, mpls_iptunnel) are not loaded. BGP labeled routes cannot be installed in the kernel FIB.",
		Examples:    []string{"ze doctor --json"},
	},
	{
		Code:        "doctor-ldp-port-unavailable",
		Title:       "LDP port 646 cannot be bound",
		Description: "LDP is configured but the daemon cannot bind UDP/TCP port 646 (RFC 5036). The port is privileged (<1024) and may need CAP_NET_BIND_SERVICE or root, or it is already in use by another process.",
		Examples:    []string{"ze doctor --json", "ze explain doctor-ldp-port-unavailable"},
	},
	{
		Code:        "doctor-rsvpte-rawsock-unavailable",
		Title:       "RSVP-TE raw IP socket unavailable",
		Description: "RSVP-TE is configured but a raw IP socket for protocol 46 (RFC 2205) cannot be opened. This requires CAP_NET_RAW or root; without it RSVP-TE cannot send or receive PATH/RESV messages.",
		Examples:    []string{"ze doctor --json", "ze explain doctor-rsvpte-rawsock-unavailable"},
	},
	{
		Code:        "doctor-geodns-port-unavailable",
		Title:       "GeoDNS privileged listen-port cannot be bound",
		Description: "GeoDNS is configured to listen on a privileged port (<1024, e.g. 53) but the daemon cannot bind it for UDP/TCP. The port needs CAP_NET_BIND_SERVICE or root, or it is already in use. The default port 5300 is unprivileged and not affected.",
		Examples:    []string{"ze doctor --json", "ze explain doctor-geodns-port-unavailable"},
	},
	{
		Code:        "doctor-as112-port-unavailable",
		Title:       "AS112 port 53 cannot be bound",
		Description: "AS112 is enabled but the daemon cannot bind its fixed UDP/TCP port 53 (RFC 7534 Section 3.5). The port needs CAP_NET_BIND_SERVICE or root, or it is already in use by another service (e.g. GeoDNS on the same address).",
		Examples:    []string{"ze doctor --json", "ze explain doctor-as112-port-unavailable"},
	},
	{
		Code:        "doctor-as112-watchdog-missing-withdraw",
		Title:       "AS112 route missing watchdog withdraw marker",
		Description: "A BGP update block announces an AS112 covering prefix (192.175.48.0/24, 192.31.196.0/24, 2620:4f:8000::/48, or 2001:4:112::/48) without a watchdog{withdraw true} marker. The marker's absence defaults to already-announced, so the route is advertised at startup before AS112 health is confirmed (RFC 7534 Section 3.3).",
		Examples:    []string{"ze doctor --json", "ze explain doctor-as112-watchdog-missing-withdraw"},
	},
	{
		Code:        "doctor-as112-global-origin-uncoordinated",
		Title:       "AS112 global origin override to a public ASN",
		Description: "A BGP session sets asn.local 112 with the replace-as local-option while eBGP-peering a non-private-use remote ASN (RFC 6996 Section 4), making this node an uncoordinated global AS112 origin. RFC 7534 Section 3.2/Section 5 requires coordination before deploying outside a local-use mirror.",
		Examples:    []string{"ze doctor --json", "ze explain doctor-as112-global-origin-uncoordinated"},
	},
	{
		Code:        "doctor-as112-redistribute-origin-uncoordinated",
		Title:       "AS112 redistribute origin to a public ASN",
		Description: "The as112 service originates its covering prefixes as AS112 (asn 112, the default) via redistribute { destination bgp { import as112 } } while an eBGP session to a non-private-use remote ASN (RFC 6996 Section 4) exists, making this node an uncoordinated global AS112 origin. RFC 7534 Section 3.2/Section 5 requires coordination; restrict the route with an egress community/prefix filter or set an operator/private asn.",
		Examples:    []string{"ze doctor --json", "ze explain doctor-as112-redistribute-origin-uncoordinated"},
	},
	{
		Code:        "doctor-as112-redistribute-not-imported",
		Title:       "AS112 redistribute knob set but not imported into BGP",
		Description: "service as112 is enabled and sets a redistribute-only knob (an explicit asn or a community) but no redistribute { destination bgp { import as112 } } is configured. Those knobs only affect the BGP-originated covering prefixes and are ignored without the import, so the covering prefixes are never originated into BGP -- the common wiring mistake where an operator expects the routes but forgot the import. Add the import, or remove the knob if the node serves DNS only.",
		Examples:    []string{"ze doctor --json", "ze explain doctor-as112-redistribute-not-imported"},
	},
	{
		Code:        "doctor-iface-macvlan",
		Title:       "Kernel macvlan support unavailable",
		Description: "The kernel cannot create a bridge-mode macvlan device (CONFIG_MACVLAN). ze's owned-device mechanism -- plugin-requested macvlan devices on a parent interface -- needs it; without CONFIG_MACVLAN those devices fail at apply. Enable CONFIG_MACVLAN or load the macvlan module. If the probe instead reports a permission failure, it lacked CAP_NET_ADMIN.",
		Examples:    []string{"ze doctor --json", "ze explain doctor-iface-macvlan"},
	},
	{
		Code:        "doctor-isis-raw-socket",
		Title:       "IS-IS raw L2 socket unavailable",
		Description: "IS-IS is configured but a raw AF_PACKET/SOCK_RAW socket cannot be opened. IS-IS runs directly over IEEE 802.3 frames (ISO/IEC 10589), so it needs CAP_NET_RAW or root; without it IS-IS cannot send or receive IIH/LSP/CSNP/PSNP PDUs and forms no adjacencies.",
		Examples:    []string{"ze doctor --json", "ze explain doctor-isis-raw-socket"},
	},
	{
		Code:        "doctor-ospf-raw-socket",
		Title:       "OSPF raw IP socket unavailable",
		Description: "OSPF is configured but a raw IP socket for protocol 89 (RFC 2328) cannot be opened. This requires CAP_NET_RAW or root; without it OSPF cannot send or receive Hello, DD, LS Request, LS Update, or LS Ack packets.",
		Examples:    []string{"ze doctor --json", "ze explain doctor-ospf-raw-socket"},
	},
	{
		Code:        "doctor-ospfv3-raw-socket",
		Title:       "OSPFv3 raw IPv6 socket unavailable",
		Description: "OSPFv3 is configured but a raw IPv6 socket for protocol 89 (RFC 5340) cannot be opened. This requires CAP_NET_RAW or root; without it OSPFv3 cannot send or receive Hello, DD, LS Request, LS Update, or LS Ack packets.",
		Examples:    []string{"ze doctor --json", "ze explain doctor-ospfv3-raw-socket"},
	},
	{
		Code:        "doctor-vrrp-raw-socket",
		Title:       "VRRP raw IP socket unavailable",
		Description: "VRRP is configured but a raw IP socket for protocol 112 (RFC 9568 / RFC 3768) cannot be opened. VRRP advertisements are sent and received directly over IP protocol 112, so this requires CAP_NET_RAW or root; without it VRRP cannot send or receive advertisements and every group stays in Initialize (no failover). The transport also needs it for the gratuitous-ARP (AF_PACKET) and unsolicited-NA (raw ICMPv6) failover announcers.",
		Examples:    []string{"ze doctor --json", "ze explain doctor-vrrp-raw-socket"},
	},
	{
		Code:        "doctor-ospfv3-ipsec",
		Title:       "OSPFv3 IPsec (RFC 4552) kernel unavailable",
		Description: "An OSPFv3 (IPv6-family) interface configures RFC 4552 manual IPsec (AH/ESP), but the kernel XFRM dataplane cannot be reached to install the transport-mode Security Associations and policies. This needs CAP_NET_ADMIN and kernel IPsec support; without it the interface would form an UNPROTECTED adjacency, so Ze warns instead of silently claiming protection.",
		Examples:    []string{"ze doctor --json", "ze explain doctor-ospfv3-ipsec"},
	},
	{
		Code:        "doctor-ospf-graceful-restart-nvs",
		Title:       "OSPF Graceful Restart NVS path unwritable",
		Description: "OSPF Graceful Restart is enabled (restarter support planned or planned-and-unplanned, RFC 3623 / RFC 5187), which requires a writable non-volatile store to persist the restart fact (RFC 3623 Section 2.1) across a planned restart, but the ZeFS blob store directory cannot be resolved or written. Without it a planned restart cannot record its grace deadline, so the resumed engine boots normally instead of staying on the forwarding path, defeating non-stop forwarding. Ensure a persistent config directory is configured and writable.",
		Examples:    []string{"ze doctor --json", "ze explain doctor-ospf-graceful-restart-nvs"},
	},
	{
		Code:        "doctor-ospf-segment-routing-overlap",
		Title:       "OSPF Segment Routing label range overlap or unsound",
		Description: "OSPF Segment Routing is enabled (RFC 8665 / RFC 8666) but the configured SRGB (global) and SRLB (local) label ranges overlap each other, overlap another range, sit outside the 20-bit MPLS label space, carry a zero Range Size, or a Prefix-SID index falls outside the total SRGB size. A double-claimed label maps two SIDs onto one forwarding entry and blackholes traffic. Make the SRGB and SRLB disjoint contiguous ranges within 16..1048575, each with Range Size greater than zero, not overlapping the LDP or RSVP-TE label space, and keep every node Prefix-SID index within the SRGB.",
		Examples:    []string{"ze doctor --json", "ze explain doctor-ospf-segment-routing-overlap"},
	},
	// doctor-isis-net-missing and doctor-isis-system-id-mismatch are NOT listed
	// here: they are owned and registered by the IS-IS component
	// (internal/plugins/isis/codes.go init() via diagnostic.Register), so
	// deleting the IS-IS component removes them (ai/rules/plugin-self-containment.md).
	{
		Code:        "doctor-l2tp-module",
		Title:       "L2TP kernel module not loaded",
		Description: "The L2TP subsystem is configured but neither l2tp_ppp nor pppol2tp is loaded.",
		Examples:    []string{"ze doctor --json", "ze explain doctor-l2tp-module"},
	},
	{
		Code:        "doctor-pppoe-module",
		Title:       "PPPoE kernel module not loaded",
		Description: "The PPPoE subsystem is configured but the pppoe kernel module is not loaded.",
		Examples:    []string{"ze doctor --json", "ze explain doctor-pppoe-module"},
	},
	{
		Code:        "doctor-firewall-nftables",
		Title:       "nftables kernel support unavailable",
		Description: "The firewall is configured with the nft backend but nf_tables kernel support is not loaded.",
		Examples:    []string{"ze doctor --json", "ze explain doctor-firewall-nftables"},
	},
	{
		Code:        "doctor-dhcp-iface",
		Title:       "DHCP listen interface missing",
		Description: "The DHCP server is configured to listen on an interface that does not exist on the system.",
		Examples:    []string{"ze doctor --json", "ze explain doctor-dhcp-iface"},
	},
	{
		Code:  "doctor-ipsec-iface",
		Title: "IPsec interface missing",
		Description: "vpn ipsec interface names an interface that does not exist on the system, " +
			"or whose name is malformed. IPsec resolves the local address of every peer that has " +
			"no explicit local-address from this interface, so those peers will not establish.",
		Examples: []string{"ze doctor --json", "ze explain doctor-ipsec-iface"},
	},
	{
		Code:        "doctor-bgp-listen",
		Title:       "BGP listener unavailable",
		Description: "A configured BGP local address and port could not be bound before daemon startup.",
		Examples:    []string{"ze doctor --json", "ze explain doctor-bgp-listen"},
	},
	{
		Code:        "doctor-bgp-md5",
		Title:       "BGP TCP MD5 not supported",
		Description: "A BGP peer has TCP MD5 authentication configured but the platform does not support TCP_MD5SIG.",
		Examples:    []string{"ze doctor --json", "ze explain doctor-bgp-md5"},
	},
	{
		Code:        "doctor-tacacs-unreachable",
		Title:       "TACACS+ servers unreachable",
		Description: "No configured TACACS+ authentication server accepted a TCP connection within the probe timeout.",
		Examples:    []string{"ze doctor --json", "ze explain doctor-tacacs-unreachable"},
	},
	{
		Code:        "doctor-radius-unreachable",
		Title:       "RADIUS servers unreachable",
		Description: "No configured L2TP RADIUS server could be reached by the UDP readiness probe.",
		Examples:    []string{"ze doctor --json", "ze explain doctor-radius-unreachable"},
	},
	{
		Code:        "doctor-radius-admin-unreachable",
		Title:       "RADIUS admin servers unreachable",
		Description: "No configured system/authentication/radius server answered an Access-Request probe, so operator logins via RADIUS may fall through to local or fail.",
		Examples:    []string{"ze doctor --json", "ze explain doctor-radius-admin-unreachable"},
	},
	{
		Code:        "doctor-hub-unreachable",
		Title:       "Management hub unreachable",
		Description: "This node is configured as a managed client (plugin/hub/client) but none of its management hubs answered a TCP probe. The node still boots and runs its committed config; it will not receive hub-pushed config updates until a hub becomes reachable.",
		Examples:    []string{"ze doctor --json", "ze explain doctor-hub-unreachable"},
	},
	{
		Code:        "doctor-traffic-usage-ebpf",
		Title:       "traffic-usage eBPF unavailable",
		Description: "The traffic-usage plugin is enabled but the kernel cannot load or attach its eBPF TCX programs (missing CAP_BPF/CAP_NET_ADMIN, no TCX support, or a non-Linux build). Byte accounting will not run.",
		Examples:    []string{"ze doctor --json", "ze explain doctor-traffic-usage-ebpf"},
	},
	{
		Code:        "doctor-bfd-port",
		Title:       "BFD control port unavailable",
		Description: "BFD is configured but the UDP control port could not be bound.",
		Examples:    []string{"ze doctor --json", "ze explain doctor-bfd-port"},
	},
	{
		Code:        "doctor-pki-cert",
		Title:       "PKI certificate invalid",
		Description: "A PKI CA or device certificate is missing, malformed, expired, or not yet valid.",
		Examples:    []string{"ze doctor --json", "ze explain doctor-pki-cert"},
	},
	{
		Code:        "doctor-ipsec-listen",
		Title:       "IPsec IKE listener unavailable",
		Description: "IPsec is configured but an IKE UDP listener port could not be bound.",
		Examples:    []string{"ze doctor --json", "ze explain doctor-ipsec-listen"},
	},
	{
		Code:        "doctor-telemetry-procfs",
		Title:       "Telemetry procfs unavailable",
		Description: "Telemetry OS collectors are configured but required procfs files are not readable.",
		Examples:    []string{"ze doctor --json", "ze explain doctor-telemetry-procfs"},
	},
	{
		Code:        "doctor-tftp-listen",
		Title:       "TFTP listener unavailable",
		Description: "The TFTP server is configured but its UDP listener port could not be bound.",
		Examples:    []string{"ze doctor --json", "ze explain doctor-tftp-listen"},
	},
	{
		Code:        "doctor-image-listen",
		Title:       "Image server listener unavailable",
		Description: "The image server is configured but its HTTP listener port could not be bound.",
		Examples:    []string{"ze doctor --json", "ze explain doctor-image-listen"},
	},
	{
		Code:        "doctor-ntp-listen",
		Title:       "NTP listener unavailable",
		Description: "NTP is configured but UDP port 123 could not be bound.",
		Examples:    []string{"ze doctor --json", "ze explain doctor-ntp-listen"},
	},
	{
		Code:        "doctor-sysctl-procfs",
		Title:       "Sysctl procfs unavailable",
		Description: "Sysctl settings are configured but /proc/sys is not writable.",
		Examples:    []string{"ze doctor --json", "ze explain doctor-sysctl-procfs"},
	},
	{
		Code:        "doctor-conntrack-procfs",
		Title:       "Conntrack procfs unavailable",
		Description: "Conntrack tuning is configured but /proc/sys/net/netfilter/nf_conntrack_max is unavailable or not writable.",
		Examples:    []string{"ze doctor --json", "ze explain doctor-conntrack-procfs"},
	},
	{
		Code:        "doctor-policyroute-netlink",
		Title:       "Policy route netlink unavailable",
		Description: "Policy routing is configured but a route netlink handle could not be opened.",
		Examples:    []string{"ze doctor --json", "ze explain doctor-policyroute-netlink"},
	},
	{
		Code:        "doctor-ntp-server-unreachable",
		Title:       "NTP servers unreachable",
		Description: "None of the configured NTP servers responded to a reachability probe. Clock synchronization may fail.",
		Examples:    []string{"ze doctor --json", "ze explain doctor-ntp-server-unreachable"},
	},
	{
		Code:        "doctor-clock-no-sync",
		Title:       "No clock synchronization configured",
		Description: "Ze NTP is disabled on a platform where clock synchronization must be provided by Ze or explicitly verified externally.",
		Examples:    []string{"ze doctor --json", "ze explain doctor-clock-no-sync"},
	},
	{
		Code:        "doctor-config-platform-mismatch",
		Title:       "Config default mismatches platform",
		Description: "A configured or default path matches a different runtime platform, such as gokrazy /perm storage on systemd or /etc/resolv.conf on gokrazy.",
		Examples:    []string{"ze doctor --json", "ze explain doctor-config-platform-mismatch"},
	},
	{
		Code:        "doctor-machine-id-missing",
		Title:       "Machine ID missing",
		Description: "The platform expects /etc/machine-id to exist and contain a stable machine identifier, but it is missing or empty.",
		Examples:    []string{"ze doctor --json", "ze explain doctor-machine-id-missing"},
	},
	{
		Code:        "doctor-rpki-unreachable",
		Title:       "RPKI cache servers unreachable",
		Description: "None of the configured RPKI cache servers accepted a TCP connection within the probe timeout.",
		Examples:    []string{"ze doctor --json", "ze explain doctor-rpki-unreachable"},
	},
	{
		Code:        "doctor-bmp-unreachable",
		Title:       "BMP collectors unreachable",
		Description: "None of the configured BMP sender collectors accepted a TCP connection within the probe timeout.",
		Examples:    []string{"ze doctor --json", "ze explain doctor-bmp-unreachable"},
	},
	{
		Code:        "doctor-write-destination",
		Title:       "Write destination not writable",
		Description: "A configured file write destination (persist path, output directory) is not writable.",
		Examples:    []string{"ze doctor --json", "ze explain doctor-write-destination"},
	},
	{
		Code:        "doctor-ntp-clock-privilege",
		Title:       "NTP clock adjustment privilege missing",
		Description: "NTP is configured but the process lacks CAP_SYS_TIME. Clock adjustment will fail unless running as root or with the capability granted.",
		Examples:    []string{"ze doctor --json", "ze explain doctor-ntp-clock-privilege"},
	},
	{
		Code:        "doctor-vpp-dpdk",
		Title:       "VPP DPDK readiness failure",
		Description: "VPP DPDK is configured but a required VFIO kernel module is not loaded or a PCI device is not present in sysfs.",
		Examples:    []string{"ze doctor --json", "ze explain doctor-vpp-dpdk"},
	},
	{
		Code:        "doctor-vpp-hugepages",
		Title:       "VPP hugepage reservation problem",
		Description: "VPP is enabled but the host's boot-time hugepage reservation is missing, smaller than VPP needs, clamped by the kernel, or (for 1G pages) unsupported by the CPU. Reserve hugepages via image.hugepages or the boot cmdline.",
		Examples:    []string{"ze doctor --json", "ze explain doctor-vpp-hugepages"},
	},
	{
		Code:        "doctor-update-check-unreachable",
		Title:       "Update check URL unreachable",
		Description: "The configured firmware update-check URL did not respond to an HTTP HEAD probe within the timeout.",
		Examples:    []string{"ze doctor --json", "ze explain doctor-update-check-unreachable"},
	},
	{
		Code:        "doctor-archive-unreachable",
		Title:       "Archive destination unreachable",
		Description: "A configured HTTP/HTTPS config archive destination did not respond to an HTTP HEAD probe within the timeout.",
		Examples:    []string{"ze doctor --json", "ze explain doctor-archive-unreachable"},
	},
	{
		Code:        "doctor-random-seed",
		Title:       "No random seed persistence",
		Description: "The platform has no known random-seed persistence service. On gokrazy, randomd saves to /perm/random.seed; on systemd, systemd-random-seed.service saves to /var/lib/systemd/random-seed. Without seed persistence, early-boot entropy may be poor, weakening cryptographic operations like BGP TCP-AO or TLS key generation.",
		Examples:    []string{"ze doctor --json", "ze explain doctor-random-seed"},
	},
	{
		Code:        "doctor-copp-missing",
		Title:       "CoPP input chain not active",
		Description: "Control-plane policing (CoPP) for BGP is configured but the nftables input chain protecting TCP/179 may not be installed. Verify the firewall backend is running and the copp plugin started successfully.",
		Examples:    []string{"ze doctor --json", "ze explain doctor-copp-missing"},
	},
}
