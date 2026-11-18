// Design: docs/architecture/api/process-protocol.md -- exact selecting process lifetime.
// Related: process.go -- one Start and one runtime cleanup per Process.
package process

import (
	"context"
	"errors"

	"github.com/ze-software/ze/internal/component/plugin"
)

// SelectedOwner captures the exact live process and connection once. The first
// call MUST follow InitConns. Subsequent calls return the same comparable token
// and recheck its lifetime. Safe for concurrent use; no manager lock is taken.
func (p *Process) SelectedOwner() (plugin.SelectedOwner, error) {
	if err := p.RequirePeerDeclarations(); err != nil {
		return plugin.SelectedOwner{}, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.selectedOwner != nil {
		if err := p.selectedOwner.Current(); err != nil {
			return plugin.SelectedOwner{}, err
		}
		return *p.selectedOwner, nil
	}
	if p.ctx == nil {
		return plugin.SelectedOwner{}, errors.New("selecting process has not started; start its new Process before selecting routes")
	}
	if p.conn == nil {
		return plugin.SelectedOwner{}, errors.New("selecting connection is not ready; initialize the Process connection before selecting routes")
	}
	owner, err := plugin.NewSelectedOwner(plugin.ProcessSender(p.config.Name), p.ctx.Done(), p.engineDone)
	if err != nil {
		return plugin.SelectedOwner{}, err
	}
	lifetime, cancel := context.WithCancel(p.ctx)
	owner, err = owner.WithConnection(p.conn.SendCurrentSelection, lifetime)
	if err != nil {
		cancel()
		return plugin.SelectedOwner{}, err
	}
	registration := p.Registration()
	if registration != nil && registration.OriginIntake {
		owner, err = owner.WithOriginIntake(p.conn.SendOriginIntake)
		if err != nil {
			cancel()
			return plugin.SelectedOwner{}, err
		}
	}
	if registration != nil {
		if registration.OriginAdmissionLookup {
			owner, err = owner.WithOriginAdmissionLookup(p.conn.SendOriginAdmissionLookup)
			if err != nil {
				cancel()
				return plugin.SelectedOwner{}, err
			}
		}
	}
	p.selectedCancel = cancel
	p.selectedOwner = &owner
	return owner, nil
}

// SelectedOwnerForCleanup returns the original captured token even after Stop
// or engine exit. The generation cleanup MUST use this value without trying to
// construct a new live owner. False means no selected/source admission occurred.
// Safe for concurrent use; it neither grants authority nor takes a manager lock.
func (p *Process) SelectedOwnerForCleanup() (plugin.SelectedOwner, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.selectedOwner == nil {
		return plugin.SelectedOwner{}, false
	}
	return *p.selectedOwner, true
}

// endSelectedEngine is the engine goroutine's final publication. The same lock
// as SelectedOwner prevents a new selected context appearing between cancel
// and EngineDone. It MUST cancel selected work before a caller joins dispatch,
// but MUST NOT cancel p.ctx: engine exit and operator Stop are distinct causes.
func (p *Process) endSelectedEngine() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.selectedCancel != nil {
		p.selectedCancel()
	}
	close(p.engineDone)
}
