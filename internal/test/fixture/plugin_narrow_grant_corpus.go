// Design: docs/architecture/api/architecture.md -- explicit grants and replay claims.
// Related: plugin_claim_permission.go -- the unchanged delivered-role observer.
// Related: plugin_fixture_12_redistribute_chain.go -- the original state/EOR checks.
package fixture

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func init() {
	Register("plugin/replay-permission-claimant", replayPermissionClaimant)
	Register("plugin/redistribute-explicit-source-grant", p12ObserveDriver("redist-explicit-attach-test", redistExplicitSourceGrantServes), StopsDaemon)
}

// replayPermissionClaimant advertises only the role whose per-peer permission
// the independent observer checks. It requests no ordinary dissemination role,
// emits no routes and stays in Run until the daemon removes or stops it.
func replayPermissionClaimant(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("plugin/replay-permission-claimant takes no arguments")
	}
	plugin, err := newObserver("replay-permission-claimant")
	if err != nil {
		return fmt.Errorf("connect replay permission claimant: %w", err)
	}
	defer plugin.Close() //nolint:errcheck // the Run result carries transport failures
	plugin.OnEvent(func(string) error { return nil })
	plugin.OnBye(func(string) error { return nil })
	return plugin.Run(ctx, &sdk.Registration{
		Claims:   []string{"bgp-peer-up-replay"},
		Receives: bgpReceiveDecls([]string{eventState}, nil),
	})
}

// These DTOs decode the existing operator command. Their zero values mean a
// missing response and fail the exact peer/process checks below.
type redistExplicitSourceDelivery struct {
	Peers []redistExplicitSourcePeer `json:"peers"`
}

type redistExplicitSourcePeer struct {
	Peer      string                        `json:"peer"`
	Processes []redistExplicitSourceProcess `json:"processes"`
}

type redistExplicitSourceProcess struct {
	Process    string   `json:"process"`
	Receive    []string `json:"receive"`
	Send       []string `json:"send"`
	Unresolved []string `json:"unresolved"`
	Implicit   []string `json:"implicit"`
}

// redistExplicitSourceGrantServes keeps the original real Loc-RIB peer count
// and EOR checks, then requires the exact narrower valid source edge. The wider
// derived update/state/refresh binding MUST NOT replace or supplement it.
func redistExplicitSourceGrantServes(ctx context.Context, plugin *sdk.Plugin) error {
	if err := redistChainOperatorStateGrantServes(ctx, plugin); err != nil {
		return err
	}
	var delivery redistExplicitSourceDelivery
	status, err := Dispatch(ctx, plugin, "show event delivery", &delivery)
	if err != nil {
		return fmt.Errorf("inspect explicit source grant: %w", err)
	}
	if status != statusDone {
		return fmt.Errorf("inspect explicit source grant: status %q", status)
	}
	if len(delivery.Peers) != 1 {
		return fmt.Errorf("explicit source grant: got %d delivery peers, want 1", len(delivery.Peers))
	}
	peer := &delivery.Peers[0]
	if peer.Peer != addrLoopback {
		return fmt.Errorf("explicit source grant: peer %q, want 127.0.0.1", peer.Peer)
	}
	bindings := 0
	for i := range peer.Processes {
		process := &peer.Processes[i]
		if process.Process != "bgp-rib" {
			continue
		}
		bindings++
		receive := slices.Clone(process.Receive)
		slices.Sort(receive)
		if !slices.Equal(receive, []string{"state", "update-received"}) {
			return fmt.Errorf("explicit source grant: receive %v, want exactly state and update-received", receive)
		}
		if len(process.Send) != 0 {
			return fmt.Errorf("explicit source grant: unexpected send grants %v", process.Send)
		}
		if len(process.Unresolved) != 0 {
			return fmt.Errorf("explicit source grant: unresolved grants %v", process.Unresolved)
		}
		if len(process.Implicit) != 0 {
			return fmt.Errorf("explicit source grant: unexpected implicit grants %v", process.Implicit)
		}
	}
	if bindings != 1 {
		return fmt.Errorf("explicit source grant: got %d bgp-rib bindings, want 1", bindings)
	}
	fmt.Fprintln(os.Stderr, "EXPLICIT-SOURCE-GRANT: bindings=1 receive=[state update-received] send=[] implicit=[] unresolved=[]")
	return nil
}
