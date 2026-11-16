// Design: docs/architecture/l2tp/followup-l2tp-call.md -- accepted Service-Name relay ownership
// Related: kernel_linux.go -- pppSetupReal; relay_sink.go -- matches the relay binding

package l2tp

import "fmt"

// prepareAndSetupPPP releases an accepted relay's unused local PPP ownership
// immediately before attaching the worker's own channel. The caller MUST close
// pppoxFD if this returns an error. A nil preparation deliberately means this
// call owns no unused local lease; it does not bypass the normal PPP setup.
func prepareAndSetupPPP(ev kernelSetupEvent, pppoxFD int,
	setup func(int, string) (int, int, int, error),
) (int, int, int, error) {
	if ev.beforeBridge != nil {
		if err := ev.beforeBridge.PrepareBridge(); err != nil {
			return -1, -1, -1, fmt.Errorf("l2tp: prepare relayed PPP channel: %w", err)
		}
	}
	return setup(pppoxFD, "")
}
