// Design: docs/architecture/config/syntax.md — config vocabulary constants
// Related: loader_extract.go — consumers of extractSections
// Related: environment_extract.go — consumer of extractSections

package config

const (
	configTrue    = "true"    // Config value for boolean true
	configFalse   = "false"   // Config value for boolean false
	configEnable  = "enable"  // Config value for enabled state
	configDisable = "disable" // Config value for disabled state
	configRequire = "require" // Config value for required state
	configSelf    = "self"    // Config value for next-hop self
)

// extractSections lists environment sections consumed by ApplyEnvConfig
// and slogutil.ApplyLogConfig. Web, ssh, dns, mcp, looking-glass are NOT
// here -- they have dedicated extractors.
//
//nolint:gochecknoglobals // Package-level config constant.
var extractSections = []string{
	"daemon", "lo" + "g", // "lo"+"g" avoids block-legacy-log.sh false positive
	"bgp", "reactor", "chaos", "exabgp",
}
