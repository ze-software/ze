//go:build linux

// Design: docs/architecture/bgp/interface-event-reactions.md -- transport zones survive refresh.
// Related: plugin_fixture_scoped_peer_prefix_process_linux.go -- phase completion files.
package fixture

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// scopedPrefixStatusDriver reads the daemon's public status after each exact
// recipient wire fence. A changed local endpoint, lost zone or reconnect fails
// even when the later wire form happens to be correct.
func scopedPrefixStatusDriver(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("scoped-prefix status observer takes no arguments")
	}
	return Observe(ctx, "scoped-peer-prefix-status", &sdk.Registration{}, func(ctx context.Context, plugin *sdk.Plugin) error {
		var localPort, remotePort any
		for phase := range 3 {
			wireFile := scopedPrefixWireFile + strconv.Itoa(phase+1)
			var fileErr error
			if !Poll(ctx, 1000, clampedPeerPoll, func() bool {
				present, err := peerFileExists(wireFile)
				if err != nil {
					fileErr = err
					return true
				}
				return present
			}) {
				return fmt.Errorf("scoped-prefix missing recipient wire fence %s", wireFile)
			}
			if fileErr != nil {
				return fileErr
			}
			status, data, err := dispatchMap(ctx, plugin, "show bgp peer receive detail")
			if err != nil {
				return err
			}
			if status != statusDone {
				return fmt.Errorf("scoped-prefix peer detail status=%s", status)
			}
			peers, ok := data["peers"].(map[string]any)
			if !ok {
				return fmt.Errorf("scoped-prefix detail has no peer map")
			}
			row, ok := peers[scopedPrefixPeer].(map[string]any)
			if !ok {
				return fmt.Errorf("scoped-prefix detail lost scoped recipient key %s", scopedPrefixPeer)
			}
			for key, want := range map[string]any{
				"local-ip":                  scopedPrefixLocal,
				"fsm-state":                 bgpStateEstablished,
				fieldConnectionsEstablished: float64(1),
				"connections-dropped":       float64(0),
			} {
				if row[key] != want {
					return fmt.Errorf("scoped-prefix phase %d %s=%v, want %v", phase+1, key, row[key], want)
				}
			}
			if phase == 0 {
				localPort, remotePort = row["local-port"], row["remote-port"]
				if localPort == nil {
					return fmt.Errorf("scoped-prefix actual local TCP port missing")
				}
				if remotePort == nil {
					return fmt.Errorf("scoped-prefix actual remote TCP port missing")
				}
			} else {
				if row["local-port"] != localPort {
					return fmt.Errorf("scoped-prefix actual local TCP port changed")
				}
				if row["remote-port"] != remotePort {
					return fmt.Errorf("scoped-prefix actual remote TCP port changed")
				}
			}
			if err := os.WriteFile(scopedPrefixStatusFile+strconv.Itoa(phase+1), nil, 0o600); err != nil {
				return err
			}
		}
		// The outer fixture owns completion and must join all four held peers.
		// Returning before it does would ask Observe to stop the daemon early.
		<-ctx.Done()
		return nil
	})
}
