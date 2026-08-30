// Design: ai/rules/plugins.md -- registration over hardcoding
// Registers the monitor-ping live view (model_ping.go) with the client-side
// view registry (view_registry.go). Kept in a register_*.go file per
// ai/patterns/registration.md; delete this plus model_ping.go and the view is
// gone with no edit to Model.

package cli

import (
	tea "charm.land/bubbletea/v2"

	"github.com/ze-software/ze/internal/component/command"
)

// commandMonitorPing is the command prefix the ping view answers to.
const commandMonitorPing = "monitor ping"

func init() {
	command.RegisterShape([]string{commandMonitorPing}, command.ShapeTab)
	command.RegisterAddressFields([]string{commandMonitorPing}, "target")

	RegisterView(viewSpec{
		key:    ViewKeyPing,
		prefix: commandMonitorPing,
		// Plain and piped monitor-ping share one spec; the "| ..." split is a
		// render-mode detail resolved inside start.
		matches: func(input string) bool {
			return isPingMonitorCommand(input) || isPipedPingMonitorCommand(input)
		},
		start: func(m *Model, input string) tea.Cmd {
			if isPipedPingMonitorCommand(input) {
				return m.startPingMonitorPiped(input)
			}
			return m.startPingMonitor(input)
		},
	})
}
