// Design: ai/rules/plugins.md -- registration over hardcoding
// The monitor-traceroute live view's registry glue: the activeView instances,
// the active-state accessors, and the init() registration (view_registry.go).
// Kept here (not in model_traceroute.go) so that file stays under the size cap;
// delete this plus model_traceroute.go and the view is gone with no Model edit.

package cli

import tea "charm.land/bubbletea/v2"

// tracerouteView / traceroutePipedView are the activeView instances for monitor
// traceroute. Render/update/state stay in model_traceroute.go (Design 1).
type tracerouteView struct{ st *tracerouteState }
type traceroutePipedView struct{ st *traceroutePipedState }

func (v *tracerouteView) update(m *Model, msg tea.Msg) (tea.Model, tea.Cmd) {
	if _, ok := msg.(traceroutePollMsg); ok {
		return m.handleTraceroutePoll()
	}
	return *m, nil
}
func (v *tracerouteView) render(m *Model) string      { return m.renderTraceroute() }
func (v *tracerouteView) key(m *Model, k string) bool { return m.handleTracerouteKey(k) }
func (v *tracerouteView) release() {
	if v.st != nil && v.st.cancelRound != nil {
		v.st.cancelRound()
	}
}

func (v *traceroutePipedView) update(m *Model, msg tea.Msg) (tea.Model, tea.Cmd) {
	if _, ok := msg.(traceroutePipedPollMsg); ok {
		return m.handleTraceroutePipedPoll()
	}
	return *m, nil
}

func (v *traceroutePipedView) render(m *Model) string {
	// | log mode appends to the scrollback viewport; "" falls through to the
	// normal render instead of taking the alt screen.
	if v.st.logMode {
		return ""
	}
	return m.renderTraceroutePiped()
}

func (v *traceroutePipedView) key(m *Model, keyStr string) bool {
	if keyStr == "q" || keyStr == keyCtrlC || keyStr == keyEsc {
		m.stopTraceroutePiped()
		return true
	}
	// Replace mode (alt screen) absorbs all keys; log mode lets others through.
	return !v.st.logMode
}
func (v *traceroutePipedView) release() {
	if v.st != nil && v.st.cancelRound != nil {
		v.st.cancelRound()
	}
}

// activeTraceroute / activeTraceroutePiped return the active traceroute session
// state, or nil when the active view is not a traceroute view.
func (m *Model) activeTraceroute() *tracerouteState {
	if v, ok := m.activeView.(*tracerouteView); ok {
		return v.st
	}
	return nil
}

func (m *Model) activeTraceroutePiped() *traceroutePipedState {
	if v, ok := m.activeView.(*traceroutePipedView); ok {
		return v.st
	}
	return nil
}

// SetTracerouteFactory sets the factory used to run traceroute probes.
// Thin wrapper over the generic keyed factory store (view_registry.go).
func (m *Model) SetTracerouteFactory(f TracerouteFactory) {
	m.SetViewFactory(ViewKeyTraceroute, f)
}

// tracerouteFactory returns the injected TracerouteFactory, or nil when none is
// registered or the stored value is the wrong type (fail-closed).
func (m *Model) tracerouteFactory() TracerouteFactory {
	raw, present := m.viewFactoryRaw(ViewKeyTraceroute)
	if !present {
		return nil
	}
	f, ok := raw.(TracerouteFactory)
	if !ok {
		return nil
	}
	return f
}

func init() {
	RegisterView(viewSpec{
		key:    ViewKeyTraceroute,
		prefix: "monitor traceroute",
		matches: func(input string) bool {
			return isTracerouteMonitorCommand(input) || isPipedTracerouteMonitorCommand(input)
		},
		start: func(m *Model, input string) tea.Cmd {
			if isPipedTracerouteMonitorCommand(input) {
				return m.startTraceroutePiped(input)
			}
			return m.startTraceroute(input)
		},
	})
}
