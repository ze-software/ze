package schema

import (
	"strings"
	"testing"
)

// TestCommandMetaCmdSchemaOwnsMetaCommands is the owner half of the
// self-containment invariant: the central meta schema must NOT declare
// help, command, event, or plugin commands, and this package MUST.
// See ai/rules/plugin-self-containment.md.
func TestCommandMetaCmdSchemaOwnsMetaCommands(t *testing.T) {
	for _, want := range []string{
		`ze:command "ze-bgp:help"`,
		`ze:command "ze-bgp:command-list"`,
		`ze:command "ze-bgp:command-help"`,
		`ze:command "ze-bgp:command-complete"`,
		`ze:command "ze-bgp:event-list"`,
		`ze:command "ze-bgp:plugin-encoding"`,
		`ze:command "ze-bgp:plugin-format"`,
		`ze:command "ze-bgp:plugin-ack"`,
		"container help",
		"container command",
		"container event",
		"container plugin",
	} {
		if !strings.Contains(ZeCommandMetaCmdYANG, want) {
			t.Errorf("ze-command-meta-cmd.yang must declare %q so removing the command component removes the meta surface", want)
		}
	}
}

// TestCommandMonitorCmdSchemaOwnsEventMonitor is the owner half of the
// self-containment invariant for the event monitor command: the central
// monitor schema must NOT declare ze-event:monitor, and this package MUST.
// See ai/rules/plugin-self-containment.md.
func TestCommandMonitorCmdSchemaOwnsEventMonitor(t *testing.T) {
	for _, want := range []string{
		`ze:command "ze-event:monitor"`,
		"container monitor",
		"container event",
	} {
		if !strings.Contains(ZeCommandMonitorCmdYANG, want) {
			t.Errorf("ze-command-monitor-cmd.yang must declare %q so removing the command component removes the event monitor surface", want)
		}
	}
}
