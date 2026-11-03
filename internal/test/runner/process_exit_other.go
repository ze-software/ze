// Design: docs/architecture/testing/runner-architecture.md -- portable process waits.
// Related: internal/component/plugin/process/exitwait_other.go -- platform limitation.

//go:build !linux && !darwin && !freebsd && !openbsd

package runner

// beforeReap leaves the existing context-driven group cancellation in place on
// platforms without waitid(WNOWAIT). Retirement follows Cmd.Wait there; this
// cannot exclude a signal racing the reap, but suppresses later deferred kills.
func (p *ownedProcess) beforeReap() error { return nil }
