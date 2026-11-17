// Design: docs/architecture/api/process-protocol.md -- exact replacement output admission.
package process

import "errors"

// HoldPeerDeclarations MUST run before a replacement's handshake publishes its
// bridge. Ordinary initial processes need no hold: peers start only after the
// whole startup declaration set has been reconciled. Safe for concurrent use.
func (p *Process) HoldPeerDeclarations() {
	p.peerDeclarationsPending.Store(true)
}

// RequirePeerDeclarations refuses output while this exact replacement awaits
// reconciliation. Setup-only calls remain available. Safe for concurrent use.
func (p *Process) RequirePeerDeclarations() error {
	if p == nil {
		return errors.New("output caller process is missing; use the authenticated process connection")
	}
	if p.peerDeclarationsPending.Load() {
		return errors.New("runtime peer declarations are awaiting reconciliation; wait for the configuration transaction to finish before sending routes")
	}
	return nil
}

// AdmitPeerDeclarations releases a held replacement exactly once. The server
// MUST reconcile and confirm its current manager identity first. True owns the
// one post-startup delivery; a duplicate admission does nothing. Safe for
// concurrent use.
func (p *Process) AdmitPeerDeclarations() bool {
	return p.peerDeclarationsPending.CompareAndSwap(true, false)
}

// BeginPostStartup reserves the one callback for this process lifetime. The
// server MUST check output admission before calling it; a held process does
// not spend its delivery. Safe for concurrent use.
func (p *Process) BeginPostStartup() bool {
	return p.postStartupStarted.CompareAndSwap(false, true)
}
