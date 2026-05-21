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
}
