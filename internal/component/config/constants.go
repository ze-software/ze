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

	// The type keywords a YANG leaf declares and an env.EnvEntry names. One
	// vocabulary, so ValueType.String and the registrations in environment.go
	// cannot drift apart.
	valueTypeString   = "string"
	valueTypeBool     = "bool"
	valueTypeInt      = "int"
	valueTypeInt64    = "int64"
	valueTypeDuration = "duration"

	// valueTypeEmpty is the YANG keyword for TypeEmpty leaves (presence flags).
	valueTypeEmpty = "empty"
)

// extractSections lists environment sections consumed by ApplyEnvConfig
// and slogutil.ApplyLogConfig. Web, ssh, dns, mcp, looking-glass are NOT
// here -- they have dedicated extractors.
//
//nolint:gochecknoglobals // Package-level config constant.
var extractSections = []string{
	sectionDaemon, "lo" + "g", // "lo"+"g" avoids block-legacy-log.sh false positive
	sectionBGP, sectionReactor, sectionChaos, "exabgp", "cli",
}
