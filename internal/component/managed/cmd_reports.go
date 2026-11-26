// Design: docs/architecture/fleet-config.md -- managed device inventory and health reporting
// Design: docs/architecture/api/commands.md -- show fleet device inventory, health, and estate summary
// Related: register.go -- RPC registration, response shapes, and device-name completion
// Related: cmd_devices.go -- shared deviceTime, registryReadRefusal, and completeFleetDevices helpers

package managed

import (
	"errors"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/pkg/fleet"
)

const (
	fleetDeviceInventoryPath     = "show fleet device inventory name"
	fleetDeviceHealthPath        = "show fleet device health name"
	fleetHealthPath              = "show fleet health"
	fleetDeviceNameLeaf          = "name"
	fleetReportStatusReported    = "reported"
	fleetReportStatusNotReported = "not-reported"
)

// fleetDeviceInventoryDoc is the structured document answered by
// `show fleet device inventory name <name>`.
type fleetDeviceInventoryDoc struct {
	plugin.DataMarker
	Name            string                  `json:"name"`
	Connected       bool                    `json:"connected"`
	Reported        bool                    `json:"reported"`
	Status          string                  `json:"status"`
	ReceivedAt      string                  `json:"received-at"`
	CollectedAt     string                  `json:"collected-at,omitempty"`
	SoftwareVersion string                  `json:"software-version,omitempty"`
	Model           string                  `json:"model,omitempty"`
	Serial          string                  `json:"serial,omitempty"`
	CPU             *fleet.InventoryCPU     `json:"cpu,omitempty"`
	NIC             *fleet.InventoryNIC     `json:"nic,omitempty"`
	Memory          *fleet.InventoryMemory  `json:"memory,omitempty"`
	Storage         *fleet.InventoryStorage `json:"storage,omitempty"`
	DMI             *fleet.InventoryDMI     `json:"dmi,omitempty"`
}

// fleetDeviceHealthDoc is the structured document answered by
// `show fleet device health name <name>`.
type fleetDeviceHealthDoc struct {
	plugin.DataMarker
	Name       string                  `json:"name"`
	Connected  bool                    `json:"connected"`
	Reported   bool                    `json:"reported"`
	Status     string                  `json:"status"`
	ReceivedAt string                  `json:"received-at"`
	CheckedAt  string                  `json:"checked-at,omitempty"`
	Components []fleet.HealthComponent `json:"components"`
}

// fleetHealthDeviceRow describes one degraded or down declared device inside
// fleetHealthSummaryDoc.
type fleetHealthDeviceRow struct {
	Name       string                  `json:"name"`
	Status     string                  `json:"status"`
	Connected  bool                    `json:"connected"`
	ReceivedAt string                  `json:"received-at"`
	CheckedAt  string                  `json:"checked-at,omitempty"`
	Components []fleet.HealthComponent `json:"components"`
}

// fleetHealthSummaryDoc is the structured document answered by
// `show fleet health`.
type fleetHealthSummaryDoc struct {
	plugin.DataMarker
	Total      int                    `json:"total"`
	Healthy    int                    `json:"healthy"`
	Degraded   int                    `json:"degraded"`
	Down       int                    `json:"down"`
	Unreported int                    `json:"unreported"`
	Devices    []fleetHealthDeviceRow `json:"devices"`
}

func lookupManagedDevice(path, purpose string, validated command.ValidatedArgs) (pluginserver.ManagedDevice, *plugin.Response) {
	name, ok := validated.Value(fleetDeviceNameLeaf)
	if !ok {
		return pluginserver.ManagedDevice{}, plugin.OperatorErrorResponse(opererr.New(
			path+": a device name is required", purpose, fleetDevicesPath))
	}
	device, err := pluginserver.ManagedDeviceByName(name)
	if errors.Is(err, pluginserver.ErrManagedDeviceUnknown) {
		return pluginserver.ManagedDevice{}, plugin.OperatorErrorResponse(opererr.New(
			path+" "+name+": unknown device",
			"the hub has no declaration or record for this device", fleetDevicesPath))
	}
	if err != nil {
		return pluginserver.ManagedDevice{}, registryReadRefusal(path+" "+name, err)
	}
	return device, nil
}

func handleFleetDeviceInventory(_ *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	device, refusal := lookupManagedDevice(
		fleetDeviceInventoryPath,
		"the command reads the reported inventory of one managed device",
		validated,
	)
	if refusal != nil {
		return refusal, nil
	}
	doc := fleetDeviceInventoryDoc{
		Name:      device.Name,
		Connected: device.Connected,
		Status:    fleetReportStatusNotReported,
	}
	if device.Inventory != nil {
		report := device.Inventory.Report.Clone()
		cpu := report.CPU
		nic := report.NIC
		mem := report.Memory
		storage := report.Storage
		dmi := report.DMI
		doc.Reported = true
		doc.Status = fleetReportStatusReported
		doc.ReceivedAt = deviceTime(device.Inventory.ReceivedAt)
		doc.CollectedAt = report.CollectedAt
		doc.SoftwareVersion = report.SoftwareVersion
		if dmi.Available {
			doc.Model = dmi.Model
			doc.Serial = dmi.Serial
		}
		doc.CPU = &cpu
		doc.NIC = &nic
		doc.Memory = &mem
		doc.Storage = &storage
		doc.DMI = &dmi
	}
	return plugin.NewResponse(plugin.StatusDone, doc), nil
}

func handleFleetDeviceHealth(_ *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	device, refusal := lookupManagedDevice(
		fleetDeviceHealthPath,
		"the command reads the reported component health of one managed device",
		validated,
	)
	if refusal != nil {
		return refusal, nil
	}
	doc := fleetDeviceHealthDoc{
		Name:       device.Name,
		Connected:  device.Connected,
		Status:     fleetReportStatusNotReported,
		Components: []fleet.HealthComponent{},
	}
	if device.Health != nil {
		report := device.Health.Report.Clone()
		components := report.Components
		if components == nil {
			components = []fleet.HealthComponent{}
		}
		doc.Reported = true
		doc.Status = report.Status.String()
		doc.ReceivedAt = deviceTime(device.Health.ReceivedAt)
		doc.CheckedAt = report.CheckedAt
		doc.Components = components
	}
	return plugin.NewResponse(plugin.StatusDone, doc), nil
}

func handleFleetHealth(_ *pluginserver.CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
	summary, err := pluginserver.ManagedFleetHealth()
	if err != nil {
		return registryReadRefusal(fleetHealthPath, err), nil
	}
	devices := make([]fleetHealthDeviceRow, 0, len(summary.Devices))
	for _, device := range summary.Devices {
		components := device.Components
		if components == nil {
			components = []fleet.HealthComponent{}
		}
		devices = append(devices, fleetHealthDeviceRow{
			Name:       device.Name,
			Status:     device.Status.String(),
			Connected:  device.Connected,
			ReceivedAt: deviceTime(device.ReceivedAt),
			CheckedAt:  device.CheckedAt,
			Components: components,
		})
	}
	doc := fleetHealthSummaryDoc{
		Total:      summary.Total,
		Healthy:    summary.Healthy,
		Degraded:   summary.Degraded,
		Down:       summary.Down,
		Unreported: summary.Unreported,
		Devices:    devices,
	}
	return plugin.NewResponse(plugin.StatusDone, doc), nil
}
