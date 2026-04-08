// Design: docs/architecture/core-design.md -- ExaBGP log topic to Ze subsystem mapping

// Package topics provides the canonical mapping from ExaBGP log topic names
// to Ze subsystem paths. Both config-tree migration and env-file migration
// use this mapping to convert ExaBGP boolean topic toggles (packets, rib, etc.)
// to Ze per-subsystem log levels.
//
// Reference: https://github.com/Exa-Networks/exabgp/blob/main/lib/exabgp/environment/setup.py
package topics

// TopicToSubsystem maps ExaBGP boolean topic names to Ze subsystem paths.
// Multiple ExaBGP topics may map to the same Ze subsystem (e.g., packets,
// network, and message all map to bgp.wire).
var TopicToSubsystem = map[string]string{
	"packets":       "bgp.wire",
	"rib":           "plugin.bgp-rib",
	"configuration": "config",
	"reactor":       "bgp.reactor",
	"daemon":        "daemon",
	"processes":     "plugin",
	"network":       "bgp.wire",
	"statistics":    "bgp.metrics",
	"message":       "bgp.wire",
	"timers":        "bgp.reactor",
	"routes":        "plugin.bgp-rib",
	"parser":        "config",
}
