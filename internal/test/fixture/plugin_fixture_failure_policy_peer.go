// Design: docs/architecture/api/process-protocol.md -- fatal failure policy
// Overview: register_failure_policy_peer.go -- external fixture registration
// Related: plugin_fixture_failure_policy.go -- actual process exit driver

package fixture

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// failurePolicyFatalPeerDriver exits only after a real peer's initial-sync EOR
// reached the wire. It never asks for shutdown: ProcessManager must observe the
// fatal owner's process exit and drive hub teardown of that live BGP session.
func failurePolicyFatalPeerDriver(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("plugin/failure-policy-fatal-peer takes no arguments")
	}
	return failurePolicyRun(ctx, "failure-fatal-peer", sdk.FailureFatal,
		func(ctx context.Context, plugin *sdk.Plugin) error {
			if err := p12WaitPeersEOR(ctx, plugin, "peer1"); err != nil {
				return err
			}
			fmt.Fprintln(os.Stderr, "FIXTURE: fatal owner exiting after peer1 initial-sync EOR reached the wire")
			return nil
		})
}
