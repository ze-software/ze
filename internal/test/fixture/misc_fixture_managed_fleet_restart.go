// Design: docs/architecture/fleet-config.md -- registry history survives a hub restart
// Related: misc_fixture_managed_fleet_registry.go -- registryRow reads the live hub command

package fixture

import (
	"context"
	"fmt"
)

func managedFleetDevicesRestartDriver(ctx context.Context, args []string) error {
	return fleetHoldRun(ctx, "managed/fleet-devices-restart", args, false, func(ctx context.Context, s *fleetHoldScenario) error {
		if err := s.withDevice(ctx, func(_ *cachedBootDaemon) error {
			_, err := s.registryRow(ctx, true)
			return err
		}); err != nil {
			return err
		}
		offline, err := s.registryRow(ctx, false)
		if err != nil {
			return err
		}
		if err := s.restartHub(ctx); err != nil {
			return err
		}
		restored, err := s.registryRow(ctx, false)
		if err != nil {
			return err
		}
		sameHistory := restored.LastSeen == offline.LastSeen && restored.ConnectedSince == offline.ConnectedSince &&
			restored.LastFetch == offline.LastFetch && restored.SourceAddress == offline.SourceAddress
		if !sameHistory {
			return fmt.Errorf("hub restart changed device history: before %+v, after %+v", offline, restored)
		}
		return storageLiveOK("hub restart restored the offline device and its recorded history")
	})
}
