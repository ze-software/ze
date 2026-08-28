// Design: docs/architecture/core-design.md -- ExaBGP log topic to Ze subsystem mapping

// Package topics provides the canonical mapping from ExaBGP log topic names
// to Ze subsystem paths. Both config-tree migration and env-file migration
// use this mapping to convert ExaBGP boolean topic toggles (packets, rib, etc.)
// to Ze per-subsystem log levels.
//
// Reference: https://github.com/Exa-Networks/exabgp/blob/main/lib/exabgp/environment/setup.py
package topics

// Ze subsystem paths this table maps onto. They are the operator-facing
// names: `request log level <subsystem> <level>` takes these spellings, and
// slogutil registers a description under each one. Naming them here lets a
// reader see which ExaBGP topics collapse onto one subsystem without
// comparing three strings that all start with "bgp.".
const (
	subsystemBGPWire    = "bgp.wire"
	subsystemBGPReactor = "bgp.reactor"
	subsystemBGPMetrics = "bgp.metrics"
	subsystemBGPRIB     = "plugin.bgp-rib"
	subsystemConfig     = "config"
	subsystemDaemon     = "daemon"
	subsystemPlugin     = "plugin"
)

// TopicToSubsystem maps ExaBGP boolean topic names to Ze subsystem paths.
// Multiple ExaBGP topics may map to the same Ze subsystem (e.g., packets,
// network, and message all map to bgp.wire).
var TopicToSubsystem = map[string]string{
	"packets":       subsystemBGPWire,
	"rib":           subsystemBGPRIB,
	"configuration": subsystemConfig,
	"reactor":       subsystemBGPReactor,
	"daemon":        subsystemDaemon,
	"processes":     subsystemPlugin,
	"network":       subsystemBGPWire,
	"statistics":    subsystemBGPMetrics,
	"message":       subsystemBGPWire,
	"timers":        subsystemBGPReactor,
	"routes":        subsystemBGPRIB,
	"parser":        subsystemConfig,
}
