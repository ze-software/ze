// Design: docs/architecture/plugin/plugin-system.md -- session-ready reports.
// Related: plugin_fixture_10.go, register_initial_sync_barrier.go -- replay reporters.
package fixture

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// fixtureObserveInitialReplay MUST install capture before startup; its scenario
// MUST report completion with this receipt, never a later session's token.
func fixtureObserveInitialReplay(ctx context.Context, name string, registration sdk.Registration, scenario func(context.Context, *sdk.Plugin, uint64) error) error {
	receipt := make(chan uint64, 1)
	setup := func(plugin *sdk.Plugin) error {
		plugin.SetStartupSubscriptions([]string{eventState}, nil, "")
		plugin.OnEvent(func(event string) error {
			var envelope struct {
				BGP struct {
					Message struct {
						Type string `json:"type"`
					} `json:"message"`
					Peer struct {
						Remote struct {
							Address netip.Addr `json:"address"`
						} `json:"remote"`
					} `json:"peer"`
					State         string `json:"state"`
					InitialReplay uint64 `json:"initial-replay,string"`
				} `json:"bgp"`
			}
			if err := json.Unmarshal([]byte(event), &envelope); err != nil {
				return fmt.Errorf("decode replay state event: %w", err)
			}
			if envelope.BGP.Message.Type != eventState {
				return nil
			}
			if envelope.BGP.State != "up" {
				return nil
			}
			if envelope.BGP.Peer.Remote.Address != netip.MustParseAddr("127.0.0.1") {
				return nil
			}
			if envelope.BGP.InitialReplay == 0 {
				return errors.New("peer UP event lacks a nonzero initial-replay token")
			}
			// Keep the first pending receipt without blocking event delivery.
			// A later UP cannot change the scenario's captured local value.
			select {
			case receipt <- envelope.BGP.InitialReplay:
			default:
			}
			return nil
		})
		return nil
	}
	return observeConfigured(ctx, name, registration, setup, func(ctx context.Context, plugin *sdk.Plugin) error {
		timer := time.NewTimer(5 * time.Second)
		defer timer.Stop()
		select {
		case initialReplay := <-receipt:
			return scenario(ctx, plugin, initialReplay)
		case <-timer.C:
			return errors.New("peer 127.0.0.1 UP event with initial-replay token not received")
		case <-ctx.Done():
			return ctx.Err()
		}
	})
}
