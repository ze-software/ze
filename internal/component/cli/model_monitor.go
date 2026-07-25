// Design: docs/architecture/api/commands.md -- monitor streaming in BubbleTea
// Overview: model.go -- editor model and update loop
// Related: model_mode.go -- mode switching (monitor enters from command mode)

package cli

import (
	"context"

	"time"

	"github.com/ze-software/ze/internal/component/cli/contract"

	tea "charm.land/bubbletea/v2"

	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// MonitorSession represents an active monitor streaming session inside the TUI.
// The model holds at most one active session. Events arrive on EventChan;
// the model polls with a ticker and appends them to the viewport.
// MonitorSession represents an active monitor streaming session inside the TUI.
// Type alias of contract.MonitorSession so ssh, web, and hub use the same type.
type MonitorSession = contract.MonitorSession

// MonitorFactory creates a monitor session for the given arguments.
// The context controls the session lifetime; cancel stops delivery.
// args are the parsed monitor keywords (e.g., ["peer", "10.0.0.1"]).
// MonitorFactory creates a monitor session.
// Type alias of contract.MonitorFactory so ssh, web, and hub use the same type.
type MonitorFactory = contract.MonitorFactory

// monitorPollInterval is how often the model drains monitor events.
const monitorPollInterval = 50 * time.Millisecond

// monitorPollMsg triggers a poll of the monitor event channel.
type monitorPollMsg struct{}

// isMonitorCommand returns true if the input is a monitor streaming command.
// Uses the registry-based prefix matching from the plugin server.
func isMonitorCommand(input string) bool {
	return pluginserver.IsStreamingCommand(input)
}

// extractMonitorCmdArgs extracts the keyword arguments after the matched streaming prefix.
func extractMonitorCmdArgs(input string) []string {
	_, args := pluginserver.GetStreamingHandlerForCommand(input)
	return args
}

// startMonitorSessionFromInput creates a monitor session. It first checks
// for a registered MonitorProvider (full-screen TUI); if none matches, it
// falls back to the event-bus factory (log mode).
func (m *Model) startMonitorSessionFromInput(args []string, fullInput string) tea.Cmd {
	if provider, provArgs := pluginserver.GetMonitorProvider(fullInput); provider != nil {
		return m.startProviderMonitor(provider, provArgs)
	}

	if m.monitorFactory == nil {
		m.statusMessage = "monitor not available (no daemon connection)"
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	session, err := m.monitorFactory(ctx, args)
	if err != nil {
		cancel()
		m.err = err
		return nil
	}

	m.monitorSession = session
	origCancel := session.Cancel
	session.Cancel = func() {
		origCancel()
		cancel()
	}

	m.outputBuf.WriteString("--- monitor active (Esc to stop) ---\n")
	m.setViewportText(m.outputBuf.String())
	m.viewport.GotoBottom()
	m.statusMessage = "monitoring (Esc to stop)"

	return tea.Tick(monitorPollInterval, func(time.Time) tea.Msg { return monitorPollMsg{} })
}

func (m *Model) startProviderMonitor(provider *pluginserver.MonitorProvider, args []string) tea.Cmd {
	ctx, cancel := context.WithCancel(context.Background())
	eventCh, renderFn, provCancel, err := provider.CreateFn(ctx, args)
	if err != nil {
		cancel()
		m.err = err
		return nil
	}

	m.monitorSession = &MonitorSession{
		EventChan: eventCh,
		Cancel: func() {
			provCancel()
			cancel()
		},
		RenderFunc: renderFn,
	}
	m.statusMessage = "monitoring (Esc to stop)"

	return tea.Tick(monitorPollInterval, func(time.Time) tea.Msg { return monitorPollMsg{} })
}

// stopMonitorSession cancels the active monitor and cleans up.
func (m *Model) stopMonitorSession() {
	if m.monitorSession == nil {
		return
	}
	m.monitorSession.Cancel()
	m.monitorSession = nil
	m.statusMessage = "monitor stopped"
}

// drainMonitorEvents reads all immediately available events from the channel.
// Returns the events and whether the channel was closed.
func drainMonitorEvents(ch <-chan string) (events []string, closed bool) {
	for {
		select {
		case event, ok := <-ch:
			if !ok {
				return events, true
			}
			events = append(events, event)
		default: //nolint:staticcheck // non-blocking drain requires default
			return events, false
		}
	}
}

// handleMonitorPoll drains available events from the monitor channel
// and appends them to the viewport (log mode) or just drains them
// (full-screen mode where RenderFunc drives View). Reschedules the next poll.
func (m Model) handleMonitorPoll() (tea.Model, tea.Cmd) {
	if m.monitorSession == nil {
		return m, nil
	}

	fullScreen := m.monitorSession.RenderFunc != nil
	events, closed := drainMonitorEvents(m.monitorSession.EventChan)

	if !fullScreen {
		for _, event := range events {
			if m.outputBuf.Len() > 0 {
				m.outputBuf.WriteString("\n")
			}
			if m.monitorSession.FormatFunc != nil {
				event = m.monitorSession.FormatFunc(event)
			}
			m.outputBuf.WriteString(event)
		}
	}

	if closed {
		m.monitorSession.Cancel()
		m.monitorSession = nil
		m.statusMessage = "monitor ended"
		if !fullScreen && len(events) > 0 {
			m.setViewportText(m.outputBuf.String())
			m.viewport.GotoBottom()
		}
		return m, nil
	}

	if !fullScreen && len(events) > 0 {
		m.setViewportText(m.outputBuf.String())
		m.viewport.GotoBottom()
	}

	return m, tea.Tick(monitorPollInterval, func(time.Time) tea.Msg { return monitorPollMsg{} })
}

// SetMonitorFactory sets the factory used to create monitor sessions.
// When set, streaming commands (e.g., "monitor event") in the interactive TUI enter streaming mode.
// When nil, streaming commands fall through to the regular executor (non-streaming).
func (m *Model) SetMonitorFactory(f MonitorFactory) {
	m.monitorFactory = f
}

// IsMonitoring returns true if a monitor session is active.
func (m Model) IsMonitoring() bool {
	return m.monitorSession != nil
}
