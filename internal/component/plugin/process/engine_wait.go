// Design: docs/architecture/api/process-protocol.md -- plugin shutdown ordering
// Overview: process.go -- Process struct, startInternal closes engineDone
// Related: manager.go -- ProcessManager.Stop, the only caller of WaitEngine

package process

import "context"

// WaitEngine blocks until an internal plugin's engine has returned, or ctx expires. The
// engine releases what it installed on its way out, so this is the wait that decides
// whether that release lands before the daemon exits (ProcessManager.Stop).
//
// An external plugin has no engine goroutine here: its release runs in its own process,
// which context cancellation ends, so there is nothing to wait for and this returns nil.
func (p *Process) WaitEngine(ctx context.Context) error {
	if p.engineDone == nil {
		return nil
	}
	select {
	case <-p.engineDone:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
