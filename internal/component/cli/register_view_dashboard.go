// Design: ai/rules/plugins.md -- registration over hardcoding
// The bgp-monitor dashboard live view's registry glue: the activeView instance
// and the init() registration (view_registry.go). Render/update/state stay in
// model_dashboard.go (Design 1). Delete this plus model_dashboard.go and the
// dashboard view is gone with no edit to Model.

package cli

import tea "charm.land/bubbletea/v2"

// dashboardView is the activeView instance for the bgp-monitor dashboard. Its
// pull-poll factory (no target args) proves the lifecycle interface spans the
// heterogeneous factory signatures (A-1).
type dashboardView struct{ st *dashboardState }

func (v *dashboardView) update(m *Model, msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case dashboardTickMsg:
		// A scheduled tick fires the next data poll (unless the view stopped).
		if m.activeDashboard() != nil {
			return *m, m.dashboardPollCmd()
		}
		return *m, nil
	case dashboardDataMsg:
		return m.handleDashboardData(msg)
	}
	return *m, nil
}
func (v *dashboardView) render(m *Model) string      { return m.renderDashboard() }
func (v *dashboardView) key(m *Model, k string) bool { return m.handleDashboardKey(k) }

// release is a no-op: the dashboard drives one-shot pull pollers via tea.Cmd and
// holds no context/goroutine, so a view-switch has nothing to cancel. It still
// satisfies the activeView interface so the dashboard can be released uniformly.
func (v *dashboardView) release() {}

func init() {
	RegisterView(viewSpec{
		key:     ViewKeyDashboard,
		prefix:  "monitor bgp",
		matches: isDashboardCommand, // "monitor bgp" [args]; no piped variant.
		start: func(m *Model, _ string) tea.Cmd {
			return m.startDashboard()
		},
	})
}
