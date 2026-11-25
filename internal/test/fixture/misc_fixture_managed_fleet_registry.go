// Design: docs/architecture/fleet-config.md -- connection and configuration facts of a managed device
// Related: misc_fixture_managed_fleet_hold.go -- fleetHoldRun provisions the hub and one device
// Related: register_managed_fleet_registry.go -- registers the registry checks

package fixture

import (
	"context"
	"fmt"
	"slices"
	"time"
)

// fleetRegistryRow contains the fields that connect and disconnect must publish.
type fleetRegistryRow struct {
	Name           string            `json:"name"`
	Connected      *bool             `json:"connected"`
	ConnectedSince string            `json:"connected-since"`
	EverConnected  bool              `json:"ever-connected"`
	SourceAddress  string            `json:"source-address"`
	LastSeen       string            `json:"last-seen"`
	LastFetch      string            `json:"last-fetch"`
	ConfigState    string            `json:"config-state"`
	HubVersion     string            `json:"hub-version"`
	Diverged       *bool             `json:"diverged"`
	Removed        *bool             `json:"removed"`
	Labels         map[string]string `json:"labels"`
}

func (s *fleetHoldScenario) registryRow(ctx context.Context, connected bool) (fleetRegistryRow, error) {
	const command = "show fleet devices | json compact"
	var row fleetRegistryRow
	var answer string
	wantVersion := s.version(restoreClientServedRouterID)
	found := Poll(ctx, fleetHoldDoctorAttempts, fleetStatusPollDelay, func() bool {
		var err error
		answer, err = storageLiveRequest(ctx, s.hubOperator, command)
		if err != nil {
			return false
		}
		var rows []fleetRegistryRow
		if fleetDivergenceDecode(answer, &rows) != nil {
			return false
		}
		if len(rows) != 1 {
			return false
		}
		row = rows[0]
		if row.Connected == nil {
			return false
		}
		if !connected && !registryTime(row.LastSeen) {
			return false
		}
		return row.Name == caTrustClientName && *row.Connected == connected && row.EverConnected &&
			row.SourceAddress != "" && row.ConfigState == "in-sync" && row.HubVersion == wantVersion &&
			registryTime(row.ConnectedSince) && registryTime(row.LastFetch)
	})
	if !found {
		return row, fmt.Errorf("%s did not show %s connected=%t with applied version %s: %s",
			command, caTrustClientName, connected, wantVersion, answer)
	}
	return row, nil
}

func registryTime(value string) bool {
	stamp, err := time.Parse(time.RFC3339Nano, value)
	return err == nil && !stamp.IsZero()
}

func managedFleetDevicesConnectDriver(ctx context.Context, args []string) error {
	return fleetHoldRun(ctx, "managed/fleet-devices-connect", args, false, func(ctx context.Context, s *fleetHoldScenario) error {
		var online fleetRegistryRow
		err := s.withDevice(ctx, func(_ *cachedBootDaemon) error {
			var err error
			online, err = s.registryRow(ctx, true)
			return err
		})
		if err != nil {
			return err
		}
		offline, err := s.registryRow(ctx, false)
		if err != nil {
			return err
		}
		if offline.ConnectedSince != online.ConnectedSince {
			return fmt.Errorf("session start changed after disconnect from %s to %s", online.ConnectedSince, offline.ConnectedSince)
		}
		return storageLiveOK("fleet device listed connected and offline")
	})
}

func managedFleetDeviceDetailDriver(ctx context.Context, args []string) error {
	return fleetHoldRun(ctx, "managed/fleet-device-detail", args, false, func(ctx context.Context, s *fleetHoldScenario) error {
		return s.withDevice(ctx, func(daemon *cachedBootDaemon) error {
			const cmd = "show fleet device name " + caTrustClientName + " | json compact"
			var answer string
			found := cachedBootWait(ctx, daemon, fleetHoldDoctorAttempts, func() bool {
				var err error
				answer, err = storageLiveRequest(ctx, s.hubOperator, cmd)
				if err != nil {
					return false
				}
				var detail struct {
					fleetRegistryRow
					FirstConnected string `json:"first-connected"`
					LastAck        string `json:"last-ack"`
					AckVersion     string `json:"ack-version"`
					AckResult      string `json:"ack-result"`
					Persisted      bool   `json:"persisted"`
				}
				if fleetDivergenceDecode(answer, &detail) != nil {
					return false
				}
				if detail.Connected == nil {
					return false
				}
				return detail.Name == caTrustClientName && *detail.Connected && detail.Persisted &&
					detail.AckResult == "applied" && detail.AckVersion == s.version(restoreClientServedRouterID) &&
					registryTime(detail.FirstConnected) && registryTime(detail.LastFetch) && registryTime(detail.LastAck)
			})
			if !found {
				return fmt.Errorf("%s did not show the connected device's first session and applied configuration: %s", cmd, answer)
			}
			if err := storageLiveRefused(ctx, s.hubOperator, "show fleet device name nobody", "show fleet devices"); err != nil {
				return err
			}
			return storageLiveOK("fleet device detail reported the named session")
		})
	})
}

// registryHubConfig declares a London and a Paris device. The first device has a
// config in the store; the second has only its declaration.
func registryHubConfig(sshPort, pluginPort, managedPort string) string {
	clients := "            client " + caTrustClientName + " { secret \"" + caTrustClientSecret +
		"\"; label site { value lon; } }\n            client edge-par { secret \"" + caTrustClientSecret +
		"\"; label site { value par; } }\n"
	return restoreClientHubConfigWithClients(sshPort, pluginPort, managedPort, clients)
}

func (s *fleetHoldScenario) registryRows(ctx context.Context, command string) ([]fleetRegistryRow, error) {
	// Raw leaves the table as an array even when a label selected the rows.
	// The dispatcher test also asserts the metadata carried by ordinary JSON.
	answer, err := storageLiveRequest(ctx, s.hubOperator, command+" | raw")
	if err != nil {
		return nil, fmt.Errorf("%s: %w: %s", command, err, answer)
	}
	var rows []fleetRegistryRow
	if err := fleetDivergenceDecode(answer, &rows); err != nil {
		return nil, fmt.Errorf("%s: %w: %s", command, err, answer)
	}
	return rows, nil
}

func managedFleetDevicesLabelDriver(ctx context.Context, args []string) error {
	return fleetHoldRunWithHubConfig(ctx, "managed/fleet-devices-label", args, false, registryHubConfig,
		func(ctx context.Context, s *fleetHoldScenario) error {
			for _, check := range []struct {
				pipe  string
				names []string
			}{
				{"", []string{caTrustClientName, "edge-par"}},
				{" | label site=lon", []string{caTrustClientName}},
				{" | label site=par", []string{"edge-par"}},
				{" | label site=absent", []string{}},
			} {
				rows, err := s.registryRows(ctx, "show fleet devices"+check.pipe)
				if err != nil {
					return err
				}
				names := make([]string, 0, len(rows))
				for _, row := range rows {
					names = append(names, row.Name)
				}
				if !slices.Equal(names, check.names) {
					return fmt.Errorf("show fleet devices%s listed %v; want %v", check.pipe, names, check.names)
				}
			}
			return storageLiveOK("fleet labels selected exactly the declared devices")
		})
}

func managedFleetDevicesDeclaredDriver(ctx context.Context, args []string) error {
	return fleetHoldRunWithHubConfig(ctx, "managed/fleet-devices-declared", args, false, registryHubConfig,
		func(ctx context.Context, s *fleetHoldScenario) error {
			for _, routerID := range []string{restoreClientServedRouterID, fleetHoldChangedRouterID} {
				if routerID == fleetHoldChangedRouterID {
					if err := s.hubImport(ctx, routerID); err != nil {
						return err
					}
				}
				rows, err := s.registryRows(ctx, "show fleet devices | label site=lon")
				if err != nil {
					return err
				}
				if len(rows) != 1 {
					return fmt.Errorf("declared London device: got %d rows, want one", len(rows))
				}
				row := rows[0]
				wantVersion := s.version(routerID)
				switch {
				case row.Name != caTrustClientName:
					return fmt.Errorf("London device name is %q, want %q", row.Name, caTrustClientName)
				case row.ConfigState != "never-fetched":
					return fmt.Errorf("%s config-state is %q before first connection, want never-fetched", row.Name, row.ConfigState)
				case row.HubVersion != wantVersion:
					return fmt.Errorf("%s hub-version is %q for router ID %s, want %q", row.Name, row.HubVersion, routerID, wantVersion)
				case row.Labels["site"] != "lon":
					return fmt.Errorf("%s labels are %v, want site=lon", row.Name, row.Labels)
				case row.Connected == nil:
					return fmt.Errorf("%s row omits connected", row.Name)
				case *row.Connected:
					return fmt.Errorf("%s reports connected before its first connection", row.Name)
				case row.EverConnected:
					return fmt.Errorf("%s reports ever-connected before its first connection", row.Name)
				case row.Removed == nil:
					return fmt.Errorf("%s row omits removed", row.Name)
				case *row.Removed:
					return fmt.Errorf("%s reports removed while its declaration exists", row.Name)
				case row.LastFetch != "":
					return fmt.Errorf("%s reports last-fetch %q before its first connection", row.Name, row.LastFetch)
				}
			}
			return storageLiveOK("declared device listed its current config before its first connection")
		})
}

func managedFleetDevicesDivergedDriver(ctx context.Context, args []string) error {
	return fleetHoldRun(ctx, "managed/fleet-devices-diverged", args, false, func(ctx context.Context, s *fleetHoldScenario) error {
		if err := s.withDevice(ctx, func(daemon *cachedBootDaemon) error {
			return s.requireFollows(ctx, daemon, restoreClientServedRouterID, "before the local edit")
		}); err != nil {
			return err
		}
		if err := s.editStopped(ctx, fleetHoldLocalRouterID); err != nil {
			return err
		}
		return s.withDevice(ctx, func(daemon *cachedBootDaemon) error {
			var rows []fleetRegistryRow
			var err error
			found := cachedBootWait(ctx, daemon, fleetHoldDoctorAttempts, func() bool {
				rows, err = s.registryRows(ctx, "show fleet devices")
				if len(rows) != 1 {
					return false
				}
				row := rows[0]
				return row.Name == caTrustClientName && row.Diverged != nil && *row.Diverged && row.ConfigState == "diverged"
			})
			if !found {
				return fmt.Errorf("held device missing from the registry: %+v (read error %v)", rows, err)
			}
			if err := s.hubImport(ctx, fleetHoldLocalRouterID); err != nil {
				return err
			}
			found = cachedBootWait(ctx, daemon, fleetHoldDoctorAttempts, func() bool {
				rows, err = s.registryRows(ctx, "show fleet devices")
				if len(rows) != 1 {
					return false
				}
				row := rows[0]
				return row.Name == caTrustClientName && row.Diverged != nil && !*row.Diverged &&
					row.HubVersion == s.version(fleetHoldLocalRouterID) && row.ConfigState == "in-sync"
			})
			if !found {
				return fmt.Errorf("device still held after the hub served its exact configuration: %+v (read error %v)", rows, err)
			}
			return storageLiveOK("fleet device showed the hold until the configurations matched")
		})
	})
}
