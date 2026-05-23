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

	// Doctor diagnostic codes.
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
}
