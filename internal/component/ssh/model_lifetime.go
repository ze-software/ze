// Design: docs/guide/config-editor.md -- SSH model drain before borrowed store release.
// Related: ssh.go -- Start captures a fresh modelSessions; Stop seals and joins it.

package ssh

import (
	"io"
	"sync"
	"time"

	"charm.land/ssh"
)

// modelSessions joins only TUI/model owners, not exec handlers: an exec lifecycle
// callback may synchronously call Server.Stop and MUST NOT join itself. The zero
// value is an empty open generation. Start creates a fresh one for each run;
// middleware captures that pointer so a late old handler cannot enter a new run.
// Existing maxSessionsMiddleware bounds admission; this is not another limit or
// statistics counter. Safe for concurrent use; MUST NOT be copied after use.
type modelSessions struct {
	mu      sync.Mutex
	active  int
	sealed  bool
	drained chan struct{} // made by seal, closed once sealed with no active owner
}

// admit reserves ownership before invoking a model factory. A successful caller
// MUST defer finish until its model closer returns, including factory refusal or
// panic. The count and seal share mu, so an admission cannot race the drain.
func (m *modelSessions) admit() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.sealed {
		return false
	}
	m.active++
	return true
}

// finish MUST follow one successful admit, after the actual model was closed.
func (m *modelSessions) finish() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.active--
	if m.sealed && m.active == 0 {
		close(m.drained)
	}
}

// running answers whether an admitted owner has not finished. A generation whose
// wait answered false stays running until its stuck owner returns, and Start
// reads this so that no new generation writes beside it.
func (m *modelSessions) running() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.active > 0
}

// seal refuses later admissions. The owner MUST call wait afterward before
// releasing borrowed resources. An owner still running when wait answers false
// keeps using them, so a resource it borrows MUST refuse its late calls once
// released. seal MUST be called once per generation.
func (m *modelSessions) seal() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sealed = true
	m.drained = make(chan struct{})
	if m.active == 0 {
		close(m.drained)
	}
}

// wait MUST be called after seal. It joins every admitted factory/model cleanup
// without a network deadline or dispatch/result-delivery dependency, for at most
// limit, because a storage call that never returns would otherwise hold the
// daemon's whole stop. It answers false when an owner was still running at limit.
func (m *modelSessions) wait(limit time.Duration) bool {
	m.mu.Lock()
	drained := m.drained
	m.mu.Unlock()
	timer := time.NewTimer(limit)
	defer timer.Stop()
	select {
	case <-drained:
		return true
	case <-timer.C:
		return false
	}
}

// modelSession owns the closer of one admitted SSH channel. Session methods,
// including Context/authz metadata and PTY/transport methods, delegate unchanged.
// The connection Context is shared by channels and MUST NOT hold this closer.
// A nil closer means the factory refused or its model has no io.Closer contract.
// The handler alone binds and closes it; it is not safe for concurrent mutation.
// sessionCloseMiddleware MUST close it before finishing its modelSessions slot.
type modelSession struct {
	ssh.Session
	closer io.Closer
}
