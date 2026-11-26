// Design: docs/architecture/fleet-config.md — host-owned fleet inventory report producer
// Related: inventory.go — Detect and hardware inventory types

package host

import (
	"context"
	"math"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/ze-software/ze/internal/core/version"
	"github.com/ze-software/ze/pkg/fleet"
)

// FleetInventoryInterval is the cadence at which a managed client pushes its
// hardware inventory summary after the initial post-connect report.
const FleetInventoryInterval = 24 * time.Hour

const fleetInventoryProducerName = "inventory"

func collectFleetInventory(ctx context.Context) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	inv, err := Detect()
	if err != nil {
		return nil, err
	}
	rep := ProjectFleetInventory(inv, version.Release(), time.Now().UTC())
	return fleet.EncodeInventoryReport(rep)
}

// ProjectFleetInventory projects a host Inventory into the bounded wire
// summary sent to a management hub. Unavailable sections carry Available: false
// with zero fields so a missing reading is never mistaken for a zero value.
// Raw SMART attributes and block-device serial numbers are intentionally
// excluded.
func ProjectFleetInventory(inv *Inventory, softwareVersion string, collectedAt time.Time) fleet.InventoryReport {
	sw := sanitizeFleetText(softwareVersion, fleet.ReportVersionMax)
	if sw == "" {
		sw = strUnknown
	}
	return fleet.InventoryReport{
		CollectedAt:     collectedAt.UTC().Format(time.RFC3339),
		SoftwareVersion: sw,
		CPU:             projectFleetCPU(inv),
		NIC:             projectFleetNIC(inv),
		Memory:          projectFleetMemory(inv),
		Storage:         projectFleetStorage(inv),
		DMI:             projectFleetDMI(inv),
	}
}

func projectFleetCPU(inv *Inventory) fleet.InventoryCPU {
	if inv == nil {
		return fleet.InventoryCPU{}
	}
	if inv.CPU == nil {
		return fleet.InventoryCPU{}
	}
	return fleet.InventoryCPU{
		Available:   true,
		LogicalCPUs: max(inv.CPU.LogicalCPUs, 0),
		Model:       sanitizeFleetText(inv.CPU.ModelName, fleet.ReportTextMax),
	}
}

func projectFleetNIC(inv *Inventory) fleet.InventoryNIC {
	if !nicSectionAvailable(inv) {
		return fleet.InventoryNIC{}
	}
	seen := make(map[string]struct{}, len(inv.NICs))
	drivers := make([]string, 0, len(inv.NICs))
	for i := range inv.NICs {
		driver := sanitizeFleetText(inv.NICs[i].Driver, fleet.ReportNameMax)
		if driver == "" {
			continue
		}
		if _, dup := seen[driver]; dup {
			continue
		}
		seen[driver] = struct{}{}
		drivers = append(drivers, driver)
	}
	slices.Sort(drivers)
	if len(drivers) > fleet.InventoryDriversMax {
		drivers = drivers[:fleet.InventoryDriversMax]
	}
	if len(drivers) == 0 {
		drivers = nil
	}
	return fleet.InventoryNIC{
		Available: true,
		Count:     len(inv.NICs),
		Drivers:   drivers,
	}
}

func nicSectionAvailable(inv *Inventory) bool {
	if inv == nil {
		return false
	}
	if inv.NICs == nil {
		return false
	}
	for _, detectErr := range inv.Errors {
		if detectErr.Path == "nics" {
			return false
		}
		if detectErr.Path == sectionNIC {
			return false
		}
	}
	return true
}

func projectFleetMemory(inv *Inventory) fleet.InventoryMemory {
	if inv == nil {
		return fleet.InventoryMemory{}
	}
	if inv.Memory == nil {
		return fleet.InventoryMemory{}
	}
	out := fleet.InventoryMemory{
		Available:  true,
		TotalBytes: inv.Memory.TotalBytes,
	}
	if inv.Memory.ECCPresent {
		out.ECC = &fleet.InventoryECC{
			CorrectableErrors:   inv.Memory.ECCCorrectableErrors,
			UncorrectableErrors: inv.Memory.ECCUncorrectableErrors,
		}
	}
	return out
}

func projectFleetStorage(inv *Inventory) fleet.InventoryStorage {
	if inv == nil {
		return fleet.InventoryStorage{}
	}
	if inv.Storage == nil {
		return fleet.InventoryStorage{}
	}
	var totalBytes uint64
	for _, dev := range inv.Storage.Devices {
		if math.MaxUint64-totalBytes < dev.SizeBytes {
			totalBytes = math.MaxUint64
			continue
		}
		totalBytes += dev.SizeBytes
	}
	return fleet.InventoryStorage{
		Available:   true,
		DeviceCount: len(inv.Storage.Devices),
		TotalBytes:  totalBytes,
	}
}

func projectFleetDMI(inv *Inventory) fleet.InventoryDMI {
	if inv == nil {
		return fleet.InventoryDMI{}
	}
	if inv.DMI == nil {
		return fleet.InventoryDMI{}
	}
	return fleet.InventoryDMI{
		Available: true,
		Model:     sanitizeFleetText(inv.DMI.SystemProduct, fleet.ReportTextMax),
		Serial:    sanitizeFleetText(inv.DMI.SystemSerial, fleet.ReportTextMax),
	}
}

// sanitizeFleetText removes invalid UTF-8 sequences and control characters from
// hardware-sourced strings and bounds the result to maxBytes on a UTF-8 rune
// boundary so unusual firmware or sysfs strings never fail wire validation.
func sanitizeFleetText(raw string, maxBytes int) string {
	valid := strings.ToValidUTF8(raw, "")
	var b strings.Builder
	b.Grow(min(len(valid), maxBytes))
	for _, r := range valid {
		if unicode.IsControl(r) {
			continue
		}
		b.WriteRune(r)
	}
	cleaned := strings.TrimSpace(b.String())
	return truncateFleetUTF8(cleaned, maxBytes)
}

func truncateFleetUTF8(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	cut := maxBytes
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut]
}
