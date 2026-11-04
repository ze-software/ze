// Design: docs/guide/config-editor.md -- SSH model drain before borrowed store release.
// Related: ssh.go -- Start captures a fresh modelSessions; Stop seals and joins it.

package ssh

import (
	"io"
	"sync"

	"charm.land/ssh"
)

// modelSessions joins only TUI/model owners, not exec handlers: an exec lifecycle
// callback may synchronously call Server.Stop and MUST NOT join itself. The zero
// value is an empty open generation. Start creates a fresh one for each run;
// middleware captures that pointer so a late old handler cannot enter a new run.
// Existing maxSessionsMiddleware bounds admission; this is not another limit or
// statistics counter. Safe for concurrent use; MUST NOT be copied after use.
type modelSessions struct {
	mu     sync.Mutex
	exited sync.WaitGroup
	sealed bool
}

// admit reserves ownership before invoking a model factory. A successful caller
// MUST defer finish until its model closer returns, including factory refusal or
// panic. Add and seal share mu, so no positive Add can race a sealed zero Wait.
func (m *modelSessions) admit() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.sealed {
		return false
	}
	m.exited.Add(1)
	return true
}

// finish MUST follow one successful admit, after the actual model was closed.
func (m *modelSessions) finish() {
	m.exited.Done()
}

// seal refuses later admissions. The owner MUST call wait afterward before
// releasing borrowed resources or opening a new generation.
func (m *modelSessions) seal() {
	m.mu.Lock()
	m.sealed = true
	m.mu.Unlock()
}

// wait MUST be called after seal. It joins every admitted factory/model cleanup
// without a network deadline or dispatch/result-delivery dependency.
func (m *modelSessions) wait() {
	m.exited.Wait()
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
