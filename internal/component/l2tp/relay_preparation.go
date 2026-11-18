// Design: docs/architecture/l2tp/followup-l2tp-call.md -- accepted Service-Name relay ownership
// Related: kernel_linux.go -- pppSetupReal; relay_sink.go -- matches the relay binding

package l2tp

import (
	"fmt"
	"os"

	pppkernel "github.com/ze-software/ze/internal/component/l2tp/ppp/kernel"
)

// prepareAndSetupPPP releases an accepted relay's unused local PPP ownership
// immediately before attaching the worker's own channel. The caller MUST close
// the original transport file if this returns an error. A nil preparation deliberately means this
// call owns no unused local lease; it does not bypass the normal PPP setup.
func prepareAndSetupPPP(ev kernelSetupEvent, transport *os.File,
	setup func(*os.File, string) (*pppkernel.CreatedChannel, *pppkernel.CreatedUnit, error),
) (*pppkernel.CreatedChannel, *pppkernel.CreatedUnit, error) {
	if ev.beforeBridge != nil {
		if err := ev.beforeBridge.PrepareBridge(); err != nil {
			return nil, nil, fmt.Errorf("l2tp: prepare relayed PPP channel: %w", err)
		}
	}
	return setup(transport, "")
}
