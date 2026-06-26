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
}
