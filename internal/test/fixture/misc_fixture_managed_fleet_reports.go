// Design: docs/architecture/fleet-config.md -- managed inventory and health reporting
// Related: misc_fixture_managed_fleet_registry.go -- registryHubConfig and registryTime
// Related: misc_fixture_managed_fleet_hold.go -- fleetHoldRunWithHubConfig provisions the hub and device
// Related: register_managed_fleet_reports.go -- registers the inventory and health report checks

package fixture

import (
	"context"
	"fmt"
	"strings"

	"github.com/ze-software/ze/pkg/fleet"
)

const (
	fleetReportPeerName          = "edge-par"
	fleetReportStatusReported    = "reported"
	fleetReportStatusNotReported = "not-reported"
	fleetReportHealthHealthy     = "healthy"
	fleetReportHealthDegraded    = "degraded"
	fleetReportHealthDown        = "down"
	fleetReportFixListDevices    = "show fleet devices"
	fleetInventoryQueryPrefix    = "show fleet device inventory name "
	fleetHealthQueryPrefix       = "show fleet device health name "
	fleetSummaryQueryCmd         = "show fleet health | json compact"
	fleetJSONCompactSuffix       = " | json compact"
)

type fleetInventoryAnswer struct {
	Name            string                  `json:"name"`
	Connected       *bool                   `json:"connected"`
	Reported        *bool                   `json:"reported"`
	Status          string                  `json:"status"`
	ReceivedAt      string                  `json:"received-at"`
	CollectedAt     string                  `json:"collected-at"`
	SoftwareVersion string                  `json:"software-version"`
	Model           string                  `json:"model"`
	Serial          string                  `json:"serial"`
	CPU             *fleet.InventoryCPU     `json:"cpu"`
	NIC             *fleet.InventoryNIC     `json:"nic"`
	Memory          *fleet.InventoryMemory  `json:"memory"`
	Storage         *fleet.InventoryStorage `json:"storage"`
	DMI             *fleet.InventoryDMI     `json:"dmi"`
}

type fleetHealthAnswer struct {
	Name       string                  `json:"name"`
	Connected  *bool                   `json:"connected"`
	Reported   *bool                   `json:"reported"`
	Status     string                  `json:"status"`
	ReceivedAt string                  `json:"received-at"`
	CheckedAt  string                  `json:"checked-at"`
	Components []fleet.HealthComponent `json:"components"`
}

type fleetHealthSummaryDeviceAnswer struct {
	Name       string                  `json:"name"`
	Status     string                  `json:"status"`
	Connected  *bool                   `json:"connected"`
	ReceivedAt string                  `json:"received-at"`
	CheckedAt  string                  `json:"checked-at"`
	Components []fleet.HealthComponent `json:"components"`
}

type fleetHealthSummaryAnswer struct {
	Total      int                              `json:"total"`
	Healthy    int                              `json:"healthy"`
	Degraded   int                              `json:"degraded"`
	Down       int                              `json:"down"`
	Unreported int                              `json:"unreported"`
	Devices    []fleetHealthSummaryDeviceAnswer `json:"devices"`
}

func isKnownFleetHealthStatus(status string) bool {
	switch status {
	case fleetReportHealthHealthy, fleetReportHealthDegraded, fleetReportHealthDown:
		return true
	default:
		return false
	}
}

func (s *fleetHoldScenario) queryInventory(ctx context.Context, name string) (fleetInventoryAnswer, string, error) {
	cmd := fleetInventoryQueryPrefix + name + fleetJSONCompactSuffix
	raw, err := storageLiveRequest(ctx, s.hubOperator, cmd)
	if err != nil {
		return fleetInventoryAnswer{}, raw, fmt.Errorf("%s: %w: %s", cmd, err, raw)
	}
	var doc fleetInventoryAnswer
	if err := fleetDivergenceDecode(raw, &doc); err != nil {
		return fleetInventoryAnswer{}, raw, fmt.Errorf("%s: %w: %s", cmd, err, raw)
	}
	return doc, raw, nil
}

func (s *fleetHoldScenario) queryHealth(ctx context.Context, name string) (fleetHealthAnswer, string, error) {
	cmd := fleetHealthQueryPrefix + name + fleetJSONCompactSuffix
	raw, err := storageLiveRequest(ctx, s.hubOperator, cmd)
	if err != nil {
		return fleetHealthAnswer{}, raw, fmt.Errorf("%s: %w: %s", cmd, err, raw)
	}
	var doc fleetHealthAnswer
	if err := fleetDivergenceDecode(raw, &doc); err != nil {
		return fleetHealthAnswer{}, raw, fmt.Errorf("%s: %w: %s", cmd, err, raw)
	}
	return doc, raw, nil
}

func (s *fleetHoldScenario) queryFleetHealth(ctx context.Context) (fleetHealthSummaryAnswer, string, error) {
	raw, err := storageLiveRequest(ctx, s.hubOperator, fleetSummaryQueryCmd)
	if err != nil {
		return fleetHealthSummaryAnswer{}, raw, fmt.Errorf("%s: %w: %s", fleetSummaryQueryCmd, err, raw)
	}
	var doc fleetHealthSummaryAnswer
	if err := fleetDivergenceDecode(raw, &doc); err != nil {
		return fleetHealthSummaryAnswer{}, raw, fmt.Errorf("%s: %w: %s", fleetSummaryQueryCmd, err, raw)
	}
	return doc, raw, nil
}

func verifyFleetHealthSummary(summary fleetHealthSummaryAnswer, status string, connected bool, receivedAt string) error {
	if summary.Total != 2 {
		return fmt.Errorf("fleet health total is %d, want 2", summary.Total)
	}
	if summary.Unreported != 1 {
		return fmt.Errorf("fleet health unreported is %d, want 1", summary.Unreported)
	}
	reportedCount := summary.Healthy + summary.Degraded + summary.Down
	if reportedCount != 1 {
		return fmt.Errorf("fleet health reported count is %d (healthy=%d degraded=%d down=%d), want 1",
			reportedCount, summary.Healthy, summary.Degraded, summary.Down)
	}
	if summary.Devices == nil {
		return fmt.Errorf("fleet health devices slice is nil")
	}
	switch status {
	case fleetReportHealthHealthy:
		if summary.Healthy != 1 {
			return fmt.Errorf("healthy device summary mismatch: healthy=%d, want 1", summary.Healthy)
		}
		if len(summary.Devices) != 0 {
			return fmt.Errorf("healthy device summary mismatch: devices=%d, want 0", len(summary.Devices))
		}
	case fleetReportHealthDegraded:
		if summary.Degraded != 1 {
			return fmt.Errorf("degraded device summary mismatch: degraded=%d, want 1", summary.Degraded)
		}
		return verifySummaryDeviceEntry(summary.Devices, status, connected, receivedAt)
	case fleetReportHealthDown:
		if summary.Down != 1 {
			return fmt.Errorf("down device summary mismatch: down=%d, want 1", summary.Down)
		}
		return verifySummaryDeviceEntry(summary.Devices, status, connected, receivedAt)
	default:
		return fmt.Errorf("unexpected health status %q", status)
	}
	return nil
}

func verifySummaryDeviceEntry(devices []fleetHealthSummaryDeviceAnswer, status string, connected bool, receivedAt string) error {
	if len(devices) != 1 {
		return fmt.Errorf("fleet health degraded/down devices count is %d, want 1", len(devices))
	}
	dev := devices[0]
	if dev.Name != caTrustClientName {
		return fmt.Errorf("fleet health device name is %q, want %q", dev.Name, caTrustClientName)
	}
	if dev.Status != status {
		return fmt.Errorf("fleet health device status is %q, want %q", dev.Status, status)
	}
	if dev.Connected == nil {
		return fmt.Errorf("fleet health device connected is nil, want %t", connected)
	}
	if *dev.Connected != connected {
		return fmt.Errorf("fleet health device connected is %t, want %t", *dev.Connected, connected)
	}
	if dev.ReceivedAt != receivedAt {
		return fmt.Errorf("fleet health device received-at is %q, want %q", dev.ReceivedAt, receivedAt)
	}
	return nil
}

func inventoryOnlineReady(doc fleetInventoryAnswer) bool {
	if doc.Connected == nil {
		return false
	}
	if doc.Reported == nil {
		return false
	}
	return *doc.Connected && *doc.Reported && doc.Name == caTrustClientName &&
		doc.Status == fleetReportStatusReported && doc.SoftwareVersion != "" &&
		registryTime(doc.CollectedAt) && registryTime(doc.ReceivedAt) &&
		doc.CPU != nil && doc.NIC != nil && doc.Memory != nil && doc.Storage != nil && doc.DMI != nil
}

func healthOnlineReady(doc fleetHealthAnswer) bool {
	if doc.Connected == nil {
		return false
	}
	if doc.Reported == nil {
		return false
	}
	return *doc.Connected && *doc.Reported && doc.Name == caTrustClientName &&
		isKnownFleetHealthStatus(doc.Status) && registryTime(doc.CheckedAt) &&
		registryTime(doc.ReceivedAt) && doc.Components != nil
}

func managedFleetInventoryReportDriver(ctx context.Context, args []string) error {
	return fleetHoldRunWithHubConfig(ctx, "managed/fleet-inventory-report", args, false, registryHubConfig,
		func(ctx context.Context, s *fleetHoldScenario) error {
			unreported, raw, err := s.queryInventory(ctx, fleetReportPeerName)
			if err != nil {
				return err
			}
			switch {
			case unreported.Name != fleetReportPeerName:
				return fmt.Errorf("unreported inventory name is %q, want %q", unreported.Name, fleetReportPeerName)
			case unreported.Connected == nil || *unreported.Connected:
				return fmt.Errorf("unreported device %s reports connected=%v: %s", fleetReportPeerName, unreported.Connected, raw)
			case unreported.Reported == nil || *unreported.Reported:
				return fmt.Errorf("unreported device %s reports reported=%v: %s", fleetReportPeerName, unreported.Reported, raw)
			case unreported.Status != fleetReportStatusNotReported:
				return fmt.Errorf("unreported device %s status is %q, want %q", fleetReportPeerName, unreported.Status, fleetReportStatusNotReported)
			case unreported.ReceivedAt != "":
				return fmt.Errorf("unreported device %s received-at is %q, want empty", fleetReportPeerName, unreported.ReceivedAt)
			}

			var online fleetInventoryAnswer
			var onlineRaw string
			err = s.withDevice(ctx, func(daemon *cachedBootDaemon) error {
				found := cachedBootWait(ctx, daemon, fleetHoldDoctorAttempts, func() bool {
					doc, answer, queryErr := s.queryInventory(ctx, caTrustClientName)
					onlineRaw = answer
					if queryErr != nil {
						return false
					}
					if !inventoryOnlineReady(doc) {
						return false
					}
					online = doc
					return true
				})
				if !found {
					return fmt.Errorf("inventory report for %s did not arrive: %s", caTrustClientName, onlineRaw)
				}
				lowerRaw := strings.ToLower(onlineRaw)
				if strings.Contains(lowerRaw, "smart") {
					return fmt.Errorf("inventory answer unexpectedly contains smart data: %s", onlineRaw)
				}
				if strings.Contains(lowerRaw, "drive-serial") {
					return fmt.Errorf("inventory answer unexpectedly contains drive-serial data: %s", onlineRaw)
				}
				return nil
			})
			if err != nil {
				return err
			}

			var offline fleetInventoryAnswer
			var offlineRaw string
			disconnected := Poll(ctx, fleetHoldDoctorAttempts, fleetStatusPollDelay, func() bool {
				doc, answer, queryErr := s.queryInventory(ctx, caTrustClientName)
				offlineRaw = answer
				if queryErr != nil {
					return false
				}
				if doc.Connected == nil {
					return false
				}
				if *doc.Connected {
					return false
				}
				offline = doc
				return true
			})
			if !disconnected {
				return fmt.Errorf("device %s did not transition to disconnected inventory state: %s", caTrustClientName, offlineRaw)
			}
			switch {
			case offline.Reported == nil || !*offline.Reported:
				return fmt.Errorf("offline device %s lost reported flag: %s", caTrustClientName, offlineRaw)
			case offline.Status != fleetReportStatusReported:
				return fmt.Errorf("offline device %s status is %q, want %q", caTrustClientName, offline.Status, fleetReportStatusReported)
			case offline.ReceivedAt != online.ReceivedAt:
				return fmt.Errorf("offline device %s received-at changed from %q to %q", caTrustClientName, online.ReceivedAt, offline.ReceivedAt)
			case offline.SoftwareVersion != online.SoftwareVersion:
				return fmt.Errorf("offline device %s software-version changed from %q to %q", caTrustClientName, online.SoftwareVersion, offline.SoftwareVersion)
			}

			if err := storageLiveRefused(ctx, s.hubOperator, fleetInventoryQueryPrefix+"nobody", fleetReportFixListDevices); err != nil {
				return err
			}
			return storageLiveOK("fleet inventory report stored, preserved offline, and refused unknown device")
		})
}

func managedFleetHealthReportDriver(ctx context.Context, args []string) error {
	return fleetHoldRunWithHubConfig(ctx, "managed/fleet-health-report", args, false, registryHubConfig,
		func(ctx context.Context, s *fleetHoldScenario) error {
			var online fleetHealthAnswer
			var onlineRaw string
			err := s.withDevice(ctx, func(daemon *cachedBootDaemon) error {
				found := cachedBootWait(ctx, daemon, fleetHoldDoctorAttempts, func() bool {
					doc, answer, queryErr := s.queryHealth(ctx, caTrustClientName)
					onlineRaw = answer
					if queryErr != nil {
						return false
					}
					if !healthOnlineReady(doc) {
						return false
					}
					online = doc
					return true
				})
				if !found {
					return fmt.Errorf("health report for %s did not arrive: %s", caTrustClientName, onlineRaw)
				}

				unreported, rawUnreported, queryErr := s.queryHealth(ctx, fleetReportPeerName)
				if queryErr != nil {
					return queryErr
				}
				switch {
				case unreported.Name != fleetReportPeerName:
					return fmt.Errorf("unreported health name is %q, want %q", unreported.Name, fleetReportPeerName)
				case unreported.Connected == nil || *unreported.Connected:
					return fmt.Errorf("unreported device %s reports connected=%v: %s", fleetReportPeerName, unreported.Connected, rawUnreported)
				case unreported.Reported == nil || *unreported.Reported:
					return fmt.Errorf("unreported device %s reports reported=%v: %s", fleetReportPeerName, unreported.Reported, rawUnreported)
				case unreported.Status != fleetReportStatusNotReported:
					return fmt.Errorf("unreported device %s status is %q, want %q", fleetReportPeerName, unreported.Status, fleetReportStatusNotReported)
				case unreported.Components == nil || len(unreported.Components) != 0:
					return fmt.Errorf("unreported device %s components are %v, want empty slice", fleetReportPeerName, unreported.Components)
				}

				summary, rawSummary, summaryErr := s.queryFleetHealth(ctx)
				if summaryErr != nil {
					return summaryErr
				}
				if verifyErr := verifyFleetHealthSummary(summary, online.Status, true, online.ReceivedAt); verifyErr != nil {
					return fmt.Errorf("%w: %s", verifyErr, rawSummary)
				}
				return nil
			})
			if err != nil {
				return err
			}

			var offline fleetHealthAnswer
			var offlineRaw string
			disconnected := Poll(ctx, fleetHoldDoctorAttempts, fleetStatusPollDelay, func() bool {
				doc, answer, queryErr := s.queryHealth(ctx, caTrustClientName)
				offlineRaw = answer
				if queryErr != nil {
					return false
				}
				if doc.Connected == nil {
					return false
				}
				if *doc.Connected {
					return false
				}
				offline = doc
				return true
			})
			if !disconnected {
				return fmt.Errorf("device %s did not transition to disconnected health state: %s", caTrustClientName, offlineRaw)
			}
			switch {
			case offline.Reported == nil || !*offline.Reported:
				return fmt.Errorf("offline device %s lost reported flag: %s", caTrustClientName, offlineRaw)
			case offline.Status != online.Status:
				return fmt.Errorf("offline device %s status changed from %q to %q", caTrustClientName, online.Status, offline.Status)
			case offline.ReceivedAt != online.ReceivedAt:
				return fmt.Errorf("offline device %s received-at changed from %q to %q", caTrustClientName, online.ReceivedAt, offline.ReceivedAt)
			case offline.CheckedAt != online.CheckedAt:
				return fmt.Errorf("offline device %s checked-at changed from %q to %q", caTrustClientName, online.CheckedAt, offline.CheckedAt)
			}

			summaryOffline, rawSummaryOffline, err := s.queryFleetHealth(ctx)
			if err != nil {
				return err
			}
			if verifyErr := verifyFleetHealthSummary(summaryOffline, online.Status, false, online.ReceivedAt); verifyErr != nil {
				return fmt.Errorf("after disconnect: %w: %s", verifyErr, rawSummaryOffline)
			}

			if err := storageLiveRefused(ctx, s.hubOperator, fleetHealthQueryPrefix+"nobody", fleetReportFixListDevices); err != nil {
				return err
			}
			return storageLiveOK("fleet health report stored, aggregated, preserved offline, and refused unknown device")
		})
}
