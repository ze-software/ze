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

func lookupManagedDevice(
	ctx *pluginserver.CommandContext,
	path, purpose string,
	validated command.ValidatedArgs,
) (pluginserver.ManagedDevice, *plugin.Response) {
	name, ok := validated.Value(fleetDeviceNameLeaf)
	if !ok {
		return pluginserver.ManagedDevice{}, plugin.OperatorErrorResponse(opererr.New(
			path+": a device name is required", purpose, fleetDevicesPath))
	}
	if !pluginserver.ResolveFleetDeviceScope(ctx).AllowsDevice(name) {
		return pluginserver.ManagedDevice{}, unknownFleetDeviceRefusal(path, name)
	}
	device, err := pluginserver.ManagedDeviceByName(name)
	if errors.Is(err, pluginserver.ErrManagedDeviceUnknown) {
		return pluginserver.ManagedDevice{}, unknownFleetDeviceRefusal(path, name)
	}
	if err != nil {
		return pluginserver.ManagedDevice{}, registryReadRefusal(path+" "+name, err)
	}
	return device, nil
}

func handleFleetDeviceInventory(ctx *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	device, refusal := lookupManagedDevice(
		ctx,
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

func handleFleetDeviceHealth(ctx *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	device, refusal := lookupManagedDevice(
		ctx,
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

func fleetHealthRowFromDevice(device *pluginserver.ManagedDevice) fleetHealthDeviceRow {
	report := device.Health.Report.Clone()
	components := report.Components
	if components == nil {
		components = []fleet.HealthComponent{}
	}
	return fleetHealthDeviceRow{
		Name:       device.Name,
		Status:     report.Status.String(),
		Connected:  device.Connected,
		ReceivedAt: deviceTime(device.Health.ReceivedAt),
		CheckedAt:  report.CheckedAt,
		Components: components,
	}
}

func handleFleetHealth(ctx *pluginserver.CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
	scope := pluginserver.ResolveFleetDeviceScope(ctx)
	devices, err := pluginserver.ManagedRegistrySnapshot()
	if err != nil {
		return registryReadRefusal(fleetHealthPath, err), nil
	}
	doc := fleetHealthSummaryDoc{
		Devices: []fleetHealthDeviceRow{},
	}
	for i := range devices {
		device := &devices[i]
		if !device.Declared {
			continue
		}
		if !scope.AllowsDevice(device.Name) {
			continue
		}
		doc.Total++
		if device.Health == nil {
			doc.Unreported++
			continue
		}
		switch device.Health.Report.Status {
		case fleet.HealthStatusHealthy:
			doc.Healthy++
		case fleet.HealthStatusDegraded:
			doc.Degraded++
			doc.Devices = append(doc.Devices, fleetHealthRowFromDevice(device))
		case fleet.HealthStatusDown:
			doc.Down++
			doc.Devices = append(doc.Devices, fleetHealthRowFromDevice(device))
		case fleet.HealthStatusUnspecified:
			doc.Unreported++
		default:
			// Open enum from pkg/fleet: treat an unrecognized status as unreported.
			doc.Unreported++
		}
	}
	return plugin.NewResponse(plugin.StatusDone, doc), nil
}
