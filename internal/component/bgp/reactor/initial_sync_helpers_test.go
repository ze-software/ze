// Design: docs/architecture/behavior/peer-lifecycle.md — initial sync owned by one session
// Related: peer_initial_sync.go — spawnInitialSync, runInitialSync, sendInitialRoutes

package reactor

import (
	"context"

	"github.com/ze-software/ze/internal/component/bgp/message"
)

// testInitialSync returns the session the peer's initial-sync state lives on,
// for tests only. The initial-sync gate and the End-of-RIB owed mark are
// fields of the Session, so a test that once wrote them on the Peer writes
// them on this session instead.
//
// It returns p.session when one is installed. When none is, it builds one
// with NewSession(p.settings), wires the peer's adj-RIB-Out into it the way
// runOnce does, and installs it as p.session. In both cases it publishes the
// session through p.establishedSession, as enterEstablished does, so
// forwardOrderHold and initialSyncInProgress read the same session.
// Safe for concurrent use: p.session is read and written under p.mu.
func (p *Peer) testInitialSync() *Session {
	p.mu.Lock()
	session := p.session
	if session == nil {
		session = NewSession(p.settings)
		session.adjOut = &p.adjOut
		p.session = session
	}
	p.mu.Unlock()

	p.establishedSession.Store(session)
	return session
}

// setTestInitialSync installs the test session (testInitialSync) and sets its
// initial-sync gate and End-of-RIB owed mark, for tests only. gate takes the
// Session.sendingInitialRoutes values: 0 open, 1 closed, 2 running.
func (p *Peer) setTestInitialSync(gate int32, owed bool) *Session {
	session := p.testInitialSync()
	session.sendingInitialRoutes.Store(gate)
	session.initialSyncEOROwed.Store(owed)
	return session
}

// runInitialSyncNow runs the initial sync of the test session synchronously
// on the calling goroutine, for tests only. It bypasses the once-per-session
// latch (spawnInitialSync) so a test can drive the sync body directly; the
// gate CAS inside sendInitialRoutes still lets a closed gate run only once.
func (p *Peer) runInitialSyncNow() {
	p.sendInitialRoutes(p.testInitialSync())
}

// sendUpdateWithSplit sends a built UPDATE on the peer's current session, for
// tests only: production callers hold their session and call
// Session.sendUpdateWithSplit on it, so that every chunk stays on one
// connection. A test that drives the split through the peer has one session
// and passes the ADD-PATH mode its UPDATE was built with.
func (p *Peer) sendUpdateWithSplit(ctx context.Context, update *message.Update, maxSize int, addPath bool) error { //nolint:unparam // tests name the ADD-PATH mode their UPDATE was built with, and every current one builds without it.
	return p.currentSession().sendUpdateWithSplit(ctx, update, maxSize, addPath, false)
}
