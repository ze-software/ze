// Design: docs/architecture/web-interface.md -- fault injection through the sole private owner.
package artifact

import "time"

// FixtureState is a detached observation for external native HTTP tests. No
// production build exposes private paths or mutable owner accounting.
type FixtureState struct {
	Root       string
	Parent     string
	Bytes      int64
	Debt       string
	DebtBytes  int64
	Workspaces map[string]string
	Uploads    int
	Sources    int
	Views      int
}

// FixtureState returns copied paths and counters under the same owner lock.
func (m *Owner) FixtureState() FixtureState {
	m.mu.Lock()
	defer m.mu.Unlock()
	state := FixtureState{Root: m.root, Parent: m.parentTemp, Bytes: m.bytes,
		Debt: m.cleanupDebt, DebtBytes: m.debtBytes, Workspaces: make(map[string]string, len(m.items)),
		Uploads: len(m.uploads), Sources: len(m.sources), Views: len(m.views)}
	for id, workspace := range m.items {
		state.Workspaces[id] = workspace.path
	}
	return state
}

// FixtureRetainedBytes injects the original one-byte quota boundary without
// allocating a second 256 MiB fixture. Tests MUST use it before admitting work.
func (m *Owner) FixtureRetainedBytes(bytes int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.bytes = bytes
}

// FixtureRemoval installs a deterministic cleanup failure. Tests MUST restore
// ordinary removal before the owner's successful cleanup is expected.
func (m *Owner) FixtureRemoval(remove func(string) error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.removeAll = remove
}

// FixtureClock supplies a fixed observation time without a sleep or a shared
// variable read outside the owner lock.
func (m *Owner) FixtureClock(now time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.now = func() time.Time { return now }
}

// FixtureParent chooses only this test's private root before first admission.
func (m *Owner) FixtureParent(path string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.parentTemp = path
}

// FixtureRoot admits a test-owned private root through the production producer.
func (m *Owner) FixtureRoot() (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.ensureRoot(); err != nil {
		return "", err
	}
	return m.root, nil
}
