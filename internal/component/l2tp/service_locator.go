// Design: docs/architecture/l2tp.md -- CLI service locator
// Related: subsystem.go -- sole publisher
// Related: subsystem_snapshot.go -- the published value's methods

package l2tp

import "sync/atomic"

// Service is the narrow read/write contract the L2TP CLI handlers
// reach the running subsystem through. The Subsystem type satisfies
// this interface via its existing Snapshot / Lookup* / Listeners /
// EffectiveConfig / Teardown* methods.
//
// Placing the interface and the globalService atomic.Pointer in the
// l2tp package (rather than a sub-package) avoids the `l2tp ->
// l2tp/api -> l2tp` import cycle that would form if the interface
// referenced l2tp snapshot types from outside.
//
// Thread safety: all methods are safe for concurrent use.
type Service interface {
	Snapshot() Snapshot
	LookupTunnel(localTID uint16) (TunnelSnapshot, bool)
	LookupSession(localSID uint16) (SessionSnapshot, bool)
	Listeners() []ListenerSnapshot
	EffectiveConfig() ConfigSnapshot
	TeardownTunnel(localTID uint16) error
	TeardownSession(localSID uint16) error
	TeardownAllTunnels() int
	TeardownAllSessions() int
	SessionEvents(sessionID uint16) []ObserverEvent
	LoginSamples(login string) []CQMBucket
	SessionSummaries() []SessionSummary
	LoginSummaries() []LoginSummary
	EchoState(login string) *LoginEchoState
	ReliableStats(localTID uint16) *ReliableStats
	TunnelFSMHistory(localTID uint16) []FSMTransition
	SessionFSMHistory(localSID uint16) []FSMTransition
	RecordDisconnect(sessionID uint16, actor, reason string, cause uint32)
}

// Compile-time guarantee that *Subsystem implements Service.
var _ Service = (*Subsystem)(nil)

// globalService holds the in-process L2TP Service. nil when the
// subsystem has not yet started (or has already stopped).
var globalService atomic.Pointer[Service]

// PublishService stores svc as the in-process L2TP service handle.
// The Subsystem calls this from Start and clears it from Stop; no
// other writer exists.
//
// Safe for concurrent use.
func PublishService(svc Service) {
	if svc == nil {
		globalService.Store(nil)
		return
	}
	globalService.Store(&svc)
}

// LookupService returns the currently-published L2TP service, or nil
// when the subsystem has not yet started (or has already stopped).
// CLI handlers MUST handle nil by returning a clear error.
//
// Safe for concurrent use.
func LookupService() Service {
	p := globalService.Load()
	if p == nil {
		return nil
	}
	return *p
}
