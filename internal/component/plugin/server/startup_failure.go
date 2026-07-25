// Design: docs/architecture/api/process-protocol.md — 5-stage plugin startup protocol
// Overview: startup.go — the startup phases that build and report this error

package server

import (
	"errors"
	"fmt"

	"github.com/ze-software/ze/internal/component/plugin/process"
)

// preferDiagnosedError chooses which failing plugin's error a phase reports.
//
// When one plugin fails its handshake, every sibling in the same tier aborts at
// the startup barrier with errStartupBarrierAborted, so a tier usually ends with
// many failures and exactly one real diagnosis. The tier is walked in
// alphabetical name order (registry.tierNames sorts it), so simply keeping the
// first failure reports whichever plugin sorts first -- almost never the one that
// actually failed. A config carrying both `bgp` and `interface`, the common
// appliance shape, reported
//
//	plugin bgp-filter-aspath failed during startup at stage Config: startup barrier aborted
//
// while the real "interface: no backend configured and no OS default available"
// sat in the log, and the barrier-abort suffix made the wrong line look
// diagnosed. That defeats the whole point of carrying the cause.
//
// So a barrier abort yields to any non-abort cause. Order is otherwise
// preserved: the first real diagnosis wins, and an all-aborts tier still reports
// something rather than nothing (ai/rules/fail-closed-guards.md).
func preferDiagnosedError(current, candidate error) error {
	if current == nil {
		return candidate
	}
	if errors.Is(current, errStartupBarrierAborted) && !errors.Is(candidate, errStartupBarrierAborted) {
		return candidate
	}
	return current
}

// startupFailureError builds the error reported for a plugin that did not reach
// StageRunning.
//
// It WRAPS the cause recorded by handleProcessStartupRPC, because the stage
// alone is not a diagnosis. A plugin that cannot apply its config stops at
// StageConfig whether the config was invalid, a file was missing, or the kernel
// refused the operation -- and the operator sees only this error on stderr
// before ze exits. Reporting the stage alone produced
//
//	error: config-path plugin startup failed: plugin interface failed during startup at stage Config
//
// which names no offending object, no reason, and no corrective action, while
// the real cause (`interface config: dummy zdiag0 create: iface: create dummy
// "zdiag0": operation not permitted (interface configuration needs
// CAP_NET_ADMIN: ...)`) was logged at Debug level and discarded. Wrapping keeps
// the chain intact for errors.Is/errors.As as well as for the text
// (ai/rules/error-messages.md: wrap the cause and add context; never drop it).
//
// When no cause was recorded the failure is still reported, with the stage
// alone: a missing cause is not evidence that startup succeeded
// (ai/rules/fail-closed-guards.md).
func startupFailureError(proc *process.Process) error {
	if cause := proc.StartupError(); cause != nil {
		return fmt.Errorf("plugin %s failed during startup at stage %s: %w", proc.Name(), proc.Stage(), cause)
	}
	return fmt.Errorf("plugin %s failed during startup at stage %s", proc.Name(), proc.Stage())
}
