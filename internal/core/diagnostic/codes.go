// Design: docs/features/ai-first.md — built-in diagnostic codes

package diagnostic

// RegisterBuiltinCodes registers all built-in diagnostic codes.
// Called explicitly by the binary entry point, not from init().
func RegisterBuiltinCodes() {
	for _, m := range builtinCodes {
		_ = Register(m)
	}
}

// exampleDoctorJSON is the one command that shows every doctor finding as JSON.
// Most codes list it as their example, so it is written once.
const exampleDoctorJSON = "ze doctor --json"

// exampleConfigValidate is the command every config-time code tells the
// operator to run, so the three that name it name one string.
const exampleConfigValidate = "ze config validate ze.conf"

// exampleShowDoctor is the daemon's doctor answer, which the runtime-phase
// codes name because only the daemon route runs their checks.
const exampleShowDoctor = "ze show doctor"

// exampleShowWarnings lists the warnings the report bus holds now, the record
// behind every warning code that names it.
const exampleShowWarnings = "ze show warnings"

// The spelling of each built-in code, declared once so that no other package
// writes it out.
//
// Only config-* codes, core TLS helper codes emitted by this package, and
// cross-cutting doctor-* codes shared across unrelated packages belong here.
// Feature- and plugin-owned doctor-* codes live in their owning package and
// register themselves via diagnostic.Register from that package's init().
//
// A code is an operator-facing surface: `ze show diagnostic <code>` looks it up
// by this spelling, and the check that emits it, the doctor check registration
// that lists it, and the metadata entry that explains it must all agree
// (ai/rules/principles.md).
const (
	CodeDoctorBGPLearnedExportNoPolicy = "doctor-bgp-learned-export-no-policy"
	CodeConfigNodeOtherOS              = "config-node-other-os"
	CodeConfigHostBackend              = "config-host-backend"
	CodeConfigYANGCardinality          = "config-yang-cardinality"
	CodeConfigYANGEnum                 = "config-yang-enum"
	CodeConfigYANGLength               = "config-yang-length"
	CodeConfigYANGMissing              = "config-yang-missing"
	CodeConfigYANGPattern              = "config-yang-pattern"
	CodeConfigYANGReference            = "config-yang-reference"
	CodeConfigYANGRange                = "config-yang-range"
	CodeConfigYANGType                 = "config-yang-type"
	CodeDoctorRecentEvents             = "doctor-recent-events"
	CodeDoctorModuleMissing            = "doctor-module-missing"
	CodeDoctorStoreIntegrity           = "doctor-store-integrity"
	CodeDoctorTLSExpired               = "doctor-tls-expired"
	CodeDoctorTLSInvalid               = "doctor-tls-invalid"
	CodeDoctorStorageUnavailable       = "doctor-storage-unavailable"
	CodeDoctorCLIHistoryWrite          = "doctor-cli-history-write"
	CodeDoctorTLSMissing               = "doctor-tls-missing"
	CodeDoctorTLSReference             = "doctor-tls-reference"
	CodeConfigBGPPeer                  = "config-bgp-peer"
	CodeConfigBGPResolve               = "config-bgp-resolve"
	CodeConfigGNMIInvalid              = "config-gnmi-invalid"
	CodeConfigHubInvalid               = "config-hub-invalid"
	CodeConfigMCPInvalid               = "config-mcp-invalid"
	CodeConfigPluginVerify             = "config-plugin-verify"
	CodeDoctorBGPListen                = "doctor-bgp-listen"
)

// builtinCodes is every code RegisterBuiltinCodes registers: the table below,
// followed by the plugin infrastructure and core codes (codes_plugin_infra.go).
var builtinCodes = append([]CodeMeta{
	{
		Code:        CodeDoctorBGPLearnedExportNoPolicy,
		Title:       "An internal BGP destination has no learned-route export policy",
		Description: "An internal peer has an eligible ordinary selecting owner and no explicit export policy. Ze withholds routes learned from other peers while local announcements remain permitted. The finding names the peer and its editor command. Configure the intended export policy and commit it; an explicit rejecting policy also clears this absence. The config-phase doctor result describes its supplied saved configuration, not live OPEN or external declaration state.",
		Examples:    []string{exampleDoctorJSON, "ze show bgp peer list"},
	},
	{
		Code:        "config-parse",
		Title:       "Config syntax error",
		Description: "The config file contains a syntax error such as an unknown keyword, missing token, or invalid scalar value.",
		Examples:    []string{`ze cli -c "show config validation bad.conf | json"`, "ze show diagnostic config-parse"},
	},
	{
		Code:         CodeConfigYANGMissing,
		Title:        "Missing mandatory field",
		Description:  "A mandatory config field required by the YANG schema is not present.",
		RelatedCodes: []string{CodeConfigYANGType, CodeConfigYANGEnum},
	},
	{
		Code:         CodeConfigYANGType,
		Title:        "Wrong value type",
		Description:  "The config value does not match the type expected by the YANG schema.",
		RelatedCodes: []string{CodeConfigYANGMissing, CodeConfigYANGEnum},
	},
	{
		Code:        CodeConfigYANGRange,
		Title:       "Value outside allowed range",
		Description: "A numeric config value falls outside the range defined in the YANG schema.",
	},
	{
		Code:        CodeConfigYANGPattern,
		Title:       "Value does not match pattern",
		Description: "A string config value does not match the regular expression pattern defined in the YANG schema.",
	},
	{
		Code:         CodeConfigYANGEnum,
		Title:        "Invalid enumeration value",
		Description:  "The config value is not one of the allowed enumeration values defined in the YANG schema.",
		RelatedCodes: []string{CodeConfigYANGType},
	},
	{
		Code:        CodeConfigYANGLength,
		Title:       "String length outside allowed range",
		Description: "A string config value has a length outside the range defined in the YANG schema.",
	},
	{
		Code:        CodeConfigYANGCardinality,
		Title:       "List cardinality violation",
		Description: "A list or leaf-list in the config has too many or too few entries per the YANG schema.",
	},
	{
		Code:        CodeConfigYANGReference,
		Title:       "Reference names no instance",
		Description: "A leafref or instance-identifier value names a node the config does not hold, and the YANG type requires the instance to exist (RFC 7950 Section 9.9.3). Configure the node the message names, or correct the value.",
	},
	{
		Code:        CodeConfigPluginVerify,
		Title:       "Plugin config verification failure",
		Description: "An in-process plugin config verifier rejected the configuration.",
	},
	{
		Code:        CodeConfigMCPInvalid,
		Title:       "MCP config consistency failure",
		Description: "MCP auth-mode, bind-remote, OAuth, or TLS cross-leaf consistency check failed.",
	},
	{
		Code:        "config-kernel-capability",
		Title:       "Kernel capability missing for a configured subsystem",
		Description: "The configuration uses a subsystem this kernel cannot carry. Every failing subsystem is named, with the kernel feature it needs and the configuration that asked for it. `ze` refuses to start on the same verdict, so a config that fails here would produce a daemon that comes up and does not work. The verdict is about the host running the command: validating a config written for another machine answers about this one. Remedy: run the kernel feature the message names, or remove the configuration that needs it.",
		Examples:    []string{exampleConfigValidate, "ze show diagnostic config-kernel-capability"},
		HostVerdict: true,
	},
	{
		Code:        CodeConfigNodeOtherOS,
		Title:       "Config node exists only on another operating system",
		Description: "The configuration uses a node the schema carries only on another operating system (YANG ze:os), such as interface xfrm, interface pppoe-client or a mirror container, which exist only on Linux. The schema of this build has no such node, because the operating system is fixed for the binary. The verdict is about the host running the command, not about the config: the same file validates on the operating system the message names. Remedy: validate the config on a host of that operating system, or remove the node if the config is meant for this one.",
		Examples:    []string{exampleConfigValidate, "ze show diagnostic config-node-other-os"},
		HostVerdict: true,
	},
	{
		Code:        CodeConfigHostBackend,
		Title:       "No backend this host can provide",
		Description: "A plugin verifier refused the configuration for a reason that belongs to the host running the command: a subsystem that names no backend has no default on this operating system (interface, firewall and traffic-control default to netlink, nft and tc on Linux only), or a hardware inventory the verifier reads (the CPU list for VPP) is not present here. The same file validates on a host that has it. Remedy: validate the config on the host it is written for, or name a backend this host provides.",
		Examples:    []string{exampleConfigValidate, "ze show diagnostic config-host-backend"},
		HostVerdict: true,
	},
	{
		Code:        CodeConfigGNMIInvalid,
		Title:       "gNMI config exposure failure",
		Description: "A gNMI server listens on a non-loopback address with no token, so it accepts unauthenticated Get and Set requests.",
	},
	{
		Code:        CodeConfigBGPResolve,
		Title:       "BGP config resolution failure",
		Description: "Template or BGP tree resolution failed during config validation.",
	},
	{
		Code:        "config-bgp-authz",
		Title:       "BGP authz profile reference failure",
		Description: "An authorization profile referenced in BGP config does not exist.",
	},
	{
		Code:        CodeConfigBGPPeer,
		Title:       "BGP peer extraction failure",
		Description: "Peer settings, route extraction, or capability constraints failed during config validation.",
	},
	{
		Code:        CodeConfigHubInvalid,
		Title:       "Hub config extraction failure",
		Description: "Plugin hub config extraction (secret length, client blocks) failed.",
	},
	{
		Code:        "config-listener-conflict",
		Title:       "Listener port conflict",
		Description: "Two listeners in the config conflict on the same address and port.",
	},
	{
		Code:        "config-secret-masked",
		Title:       "Secret holds the display placeholder",
		Description: "A ze:sensitive or ze:bcrypt leaf holds the placeholder a display path writes over a secret. The value was masked for display and cannot be stored. Restore the real value, or set it through the plaintext-<name> sibling.",
		Examples:    []string{exampleConfigValidate, "ze show diagnostic config-secret-masked"},
	},
	{
		Code:        "config-warning",
		Title:       "Config warning",
		Description: "A warning from semantic validation without a more specific diagnostic code.",
	},

	// Cross-cutting and core-helper doctor diagnostic codes.
	{
		Code:  CodeDoctorStoreIntegrity,
		Title: "Store integrity failure",
		Description: "A managed store key failed its CRC32c check. The diagnostic names the damaged key. " +
			"ze data check and ze data repair --output <path> run while ze runs: the check lists every damaged entry, and the repair writes a salvage copy holding every entry that reads, without changing the source. " +
			"A remote operator runs the same through the daemon with show data check and request data repair path <file>, which writes the copy on the daemon's host. " +
			"Installing the repaired store in place of the damaged one needs ze stopped.",
		Examples: []string{exampleDoctorJSON},
	},
	{
		Code:        CodeDoctorModuleMissing,
		Title:       "Kernel module not loaded",
		Description: "A required kernel module is not loaded.",
		Examples:    []string{exampleDoctorJSON},
	},
	{
		Code:        CodeDoctorStorageUnavailable,
		Title:       "Config storage unavailable",
		Description: "The managed store could not be opened read-only. Run ze init when no store exists, or follow the named import or repair command. Doctor never creates a store.",
		Examples:    []string{exampleDoctorJSON},
	},
	{
		Code:        CodeDoctorCLIHistoryWrite,
		Title:       "Command recall could not be saved",
		Description: "A command-history publication failed, so that recall may be lost when the session ends. A later successful publication clears the current failure but retains this recent event. One event per minute covers all users and modes without recording commands, usernames or storage error values. Run ze show doctor to diagnose storage readiness, then reconnect after restoring storage access.",
		Examples:    []string{exampleShowDoctor},
	},
	{
		Code:        CodeDoctorTLSMissing,
		Title:       "TLS certificate or key not found",
		Description: "A TLS certificate or key file referenced in the config does not exist.",
		Examples:    []string{exampleDoctorJSON},
	},
	{
		Code:        CodeDoctorTLSExpired,
		Title:       "TLS certificate expired",
		Description: "A TLS certificate referenced in the config has expired or is not yet valid.",
		Examples:    []string{exampleDoctorJSON},
	},
	{
		Code:        CodeDoctorTLSInvalid,
		Title:       "TLS certificate or key cannot be used",
		Description: "A TLS certificate file is not valid PEM, or the DER content cannot be parsed as an X.509 certificate, or a stored certificate and key do not load as a pair. A listener whose material does not load does not start.",
		Examples:    []string{exampleDoctorJSON},
	},
	{
		Code:  CodeDoctorTLSReference,
		Title: "TLS certificate reference cannot serve",
		Description: "A listener names a certificate in the PKI store that cannot serve TLS. " +
			"The pki block defines no certificate with that name, or the entry has no private key, " +
			"or its stored intermediate does not build a chain to a configured ca certificate. " +
			"The listener does not start; ze never substitutes a self-signed certificate for a name the operator configured. " +
			"Fix the name, add private { key ... } to the pki entry, or correct the intermediate.",
		Examples: []string{exampleDoctorJSON, "ze show diagnostic doctor-tls-reference", "show pki certificate name <name>"},
	},
	{
		Code:        CodeDoctorRecentEvents,
		Title:       "A problem happened recently and may have recovered",
		Description: "A component raised an event on the report bus, such as a peer session reset or a failed retry, and the event is still in the recent-event ring. The row names the event's own code, its subject, how many times it happened and when it last happened. It does not make the box read not ready, because the component may have recovered. Run show errors and show warnings to see each event, and ze show diagnostic <code> for the event's own code.",
		Examples:    []string{exampleShowDoctor, exampleShowErrors, exampleShowWarnings},
	},
	{
		Code:        "doctor-dns-resolver",
		Title:       "No DNS resolver responding",
		Description: "None of the name servers configured under system/name-server responded to a query.",
		Examples:    []string{exampleDoctorJSON},
	},
	// doctor-isis-net-missing and doctor-isis-system-id-mismatch are NOT listed
	// here: they are owned and registered by the IS-IS component
	// (internal/plugins/isis/codes.go init() via diagnostic.Register), so
	// deleting the IS-IS component removes them (ai/rules/plugins.md).
	{
		Code:  CodeDoctorBGPListen,
		Title: "BGP listener unavailable",
		Description: "A configured BGP local address and port could not be bound: by the doctor's bind probe, or by the daemon when a reload added a dynamic group or the kernel reported the address added, which the daemon logs without refusing. " +
			"The message names the address, port and what configured them. A port below net.ipv4.ip_unprivileged_port_start (1024 by default) needs CAP_NET_BIND_SERVICE, and the fix names how to grant it; an address in use names the command that finds its holder.",
		Examples: []string{exampleDoctorJSON, "ze show diagnostic doctor-bgp-listen"},
	},
	{
		Code:        "doctor-config-platform-mismatch",
		Title:       "Config default mismatches platform",
		Description: "A configured or default path matches a different runtime platform, such as gokrazy /perm storage on systemd or /etc/resolv.conf on gokrazy.",
		Examples:    []string{exampleDoctorJSON, "ze show diagnostic doctor-config-platform-mismatch"},
	},
}, pluginInfraCodes...)
