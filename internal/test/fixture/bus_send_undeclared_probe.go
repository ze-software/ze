// Design: docs/architecture/api/architecture.md -- plugin command authorization through the bus send filter.
// Overview: register_bus_send_undeclared_probe.go -- the observer registered from this probe.
// Related: fixture.go -- Observe, ReportFailure.

package fixture

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const (
	busSendUndeclaredPlugin = "bus-send-undeclared-probe"
	busSendAllowedCommand   = "show bgp peer list"
	busSendUndeclaredCmd    = "clear bgp rib in *"
)

// busSendUndeclaredProbe connects as an external plugin, dispatches an allowed
// read command (show bgp peer list), and then dispatches an undeclared mutating
// command (clear bgp rib in *). It fails when the undeclared command succeeds
// (as today's aaa.ReservedInternalPrefix bypass in authz.Store.authorize allows)
// or when it fails for a reason other than refusal (such as unknown command or
// missing handler).
func busSendUndeclaredProbe(ctx context.Context, p *sdk.Plugin) error {
	status, raw, err := p.DispatchCommand(ctx, busSendAllowedCommand)
	if err != nil {
		return fmt.Errorf("dispatch %q: %w", busSendAllowedCommand, err)
	}
	if status != statusDone {
		return fmt.Errorf("dispatch %q returned status %q (data=%s), want %s",
			busSendAllowedCommand, status, string(raw), statusDone)
	}

	status, raw, err = p.DispatchCommand(ctx, busSendUndeclaredCmd)
	if err == nil {
		if status == statusDone {
			return fmt.Errorf("undeclared command %q succeeded (status=%s data=%s): bus must refuse undeclared plugin commands instead of waiving plugin identities through Store.authorize",
				busSendUndeclaredCmd, status, string(raw))
		}
	}

	refusal := string(raw)
	if err != nil {
		refusal = err.Error()
	}
	if strings.Contains(refusal, "unknown command") {
		return fmt.Errorf("undeclared command %q failed as unknown command rather than bus refusal: %s",
			busSendUndeclaredCmd, refusal)
	}
	if strings.Contains(refusal, "not registered") {
		return fmt.Errorf("undeclared command %q failed because handler was not registered rather than bus refusal: %s",
			busSendUndeclaredCmd, refusal)
	}

	_, err = fmt.Fprintln(os.Stderr, "OK: show bgp peer list succeeded and undeclared clear bgp rib in * was refused")
	return err
}
