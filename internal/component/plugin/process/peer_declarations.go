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

// HoldRuntimeRestoration MUST run before a replacement exposes its runtime
// bridge. CompleteRuntimeRestoration MUST clear it only after the exact joined
// generation-ready work succeeds. A failed/stopped lifetime remains held.
// Initial startup owns its phase's join and needs no replacement hold.
// Safe for concurrent use.
func (p *Process) HoldRuntimeRestoration() {
	p.runtimeRestorationPending.Store(true)
}

// CompleteRuntimeRestoration MUST follow HoldRuntimeRestoration and the joined
// generation-ready success for this exact current replacement. Failure stops
// the lifetime instead; it MUST NOT clear this hold. Safe for concurrent use.
func (p *Process) CompleteRuntimeRestoration() {
	p.runtimeRestorationPending.Store(false)
}

// RequireRuntimeRestoration MUST be checked after the owner joined its work.
// A nil startup cause alone is not success: the hold must also be complete.
// The caller MUST independently confirm this exact lifetime is current,
// running and admitted. Safe for concurrent use.
func (p *Process) RequireRuntimeRestoration() error {
	if p == nil {
		return errors.New("runtime restoration has no replacement process")
	}
	if err := p.StartupError(); err != nil {
		return err
	}
	if p.runtimeRestorationPending.Load() {
		return errors.New("runtime restoration has not completed")
	}
	return nil
}

// BeginPostStartup reserves the one callback for this process lifetime. The
// server MUST check output admission before calling it; a held process does
// not spend its delivery. Safe for concurrent use.
func (p *Process) BeginPostStartup() bool {
	if p.runtimeRestorationPending.Load() {
		return false
	}
	return p.postStartupStarted.CompareAndSwap(false, true)
}
