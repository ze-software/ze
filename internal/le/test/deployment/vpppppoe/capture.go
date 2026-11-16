// Design: docs/labs/pppoe-interop.md -- discovery evidence at the VPP peer.
// Related: check.go -- capture activates before malformed traffic and pppd.
package testvpppppoe

import (
	"context"
	"fmt"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// Each topology shares the scratch mount, so remove the previous capture
// synchronously before waiting for this tcpdump's activation message. The
// packet and time bounds keep diagnostics below the control peer's lifetime;
// Suite owns the container and stops the capture on every cleanup path.
func startDiscoveryCapture(ctx context.Context, lab interoplab.CheckerLab) error {
	if _, err := query(ctx, lab, vppPeer, "rm", "-f", "/evidence/discovery.log"); err != nil {
		return err
	}
	command := []string{"sh", "-c", "exec timeout --signal=TERM 60s tcpdump -i any -nn -e -v -l -c 32 'ether proto 0x8863 or ether proto 0x8100' > /evidence/discovery.log 2>&1"}
	if err := lab.ExecDetached(ctx, vppPeer, command, nil); err != nil {
		return err
	}
	if _, err := waitText(ctx, lab, []string{readFileCommand, "/evidence/discovery.log"}, "listening on any,"); err != nil {
		return fmt.Errorf("discovery capture did not activate; inspect /evidence/discovery.log: %w", err)
	}
	return nil
}
