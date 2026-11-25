// Design: docs/architecture/fleet-config.md -- managed device registry on the hub
// Design: docs/architecture/api/commands.md -- show fleet devices and show fleet device name
// Related: register.go -- RPC registration, response shapes and device-name completion

package managed

import (
	"errors"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/opererr"
)

const (
	fleetDevicesPath = "show fleet devices"
	fleetDevicePath  = "show fleet device name"
)

// fleetDeviceRow is the public table contract. Absent times and addresses are
// empty strings, so a never-connected row has the same columns as a live one.
type fleetDeviceRow struct {
	Name            string            `json:"name"`
	Connected       bool              `json:"connected"`
	ConfigState     string            `json:"config-state"`
	Labels          map[string]string `json:"labels"`
	HubVersion      string            `json:"hub-version"`
	DeviceVersion   string            `json:"device-version"`
	LastFetch       string            `json:"last-fetch"`
	LastFetchResult string            `json:"last-fetch-result"`
	ConnectedSince  string            `json:"connected-since"`
	LastSeen        string            `json:"last-seen"`
	SourceAddress   string            `json:"source-address"`
	EverConnected   bool              `json:"ever-connected"`
	Diverged        bool              `json:"diverged"`
	Removed         bool              `json:"removed"`
}

type fleetDeviceDetail struct {
	plugin.DataMarker
	fleetDeviceRow
	FirstConnected string `json:"first-connected"`
	LastAck        string `json:"last-ack"`
	AckVersion     string `json:"ack-version"`
	AckResult      string `json:"ack-result"`
	AckError       string `json:"ack-error"`
	LastFetchError string `json:"last-fetch-error"`
	Persisted      bool   `json:"persisted"`
	Unreadable     bool   `json:"unreadable"`
	RemovedAt      string `json:"removed-at"`
}

func deviceTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
}

func deviceConfigState(device pluginserver.ManagedDevice) string {
	switch {
	case device.Diverged:
		return "diverged"
	case device.LastFetch.IsZero():
		return "never-fetched"
	case device.AckResult == pluginserver.ManagedAckRefused:
		return "rejected"
	case device.AckResult == pluginserver.ManagedAckApplied && device.AckVersion == device.HubVersion:
		return "in-sync"
	default:
		return "pending"
	}
}

func rowForDevice(device pluginserver.ManagedDevice) fleetDeviceRow {
	labels := device.Labels
	if !device.Declared || labels == nil {
		labels = map[string]string{}
	}
	var addr string
	if device.SourceAddress.IsValid() {
		addr = device.SourceAddress.String()
	}
	return fleetDeviceRow{
		Name: device.Name, Connected: device.Connected, ConfigState: deviceConfigState(device),
		Labels: labels, HubVersion: device.HubVersion, DeviceVersion: device.DeviceVersion,
		LastFetch: deviceTime(device.LastFetch), LastFetchResult: device.LastFetchResult.String(),
		ConnectedSince: deviceTime(device.ConnectedSince), LastSeen: deviceTime(device.LastSeen),
		SourceAddress: addr, EverConnected: device.Connected || !device.FirstConnected.IsZero(),
		Diverged: device.Diverged, Removed: !device.Declared,
	}
}

// splitFleetLabel accepts a nonempty key and value separated by the first '='.
// Cut leaves value empty when the separator is missing.
func splitFleetLabel(pair string) (key, value string, valid bool) {
	key, value, _ = strings.Cut(pair, "=")
	return key, value, key != "" && value != ""
}

func handleFleetDevices(_ *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	devices, err := pluginserver.ManagedRegistrySnapshot()
	if err != nil {
		return registryReadRefusal(fleetDevicesPath, err), nil
	}
	rows := make(plugin.Slice[fleetDeviceRow], 0, len(devices))
	for _, device := range devices {
		rows = append(rows, rowForDevice(device))
	}
	// The only command-owned operation is label. Successive filters narrow
	// the same sorted rows, so their order is preserved and an empty answer is [].
	for _, op := range validated.PipeOperations() {
		pair := strings.Join(op.Args, " ")
		key, value, valid := splitFleetLabel(pair)
		if !valid {
			return plugin.OperatorErrorResponse(opererr.New("the fleet label filter requires a key=value pair",
				"each managed device label has a key and a value", "show fleet devices | label site=lon")), nil
		}
		matched := rows[:0]
		for _, row := range rows {
			if actual, ok := row.Labels[key]; ok && actual == value {
				matched = append(matched, row)
			}
		}
		rows = matched
	}
	return plugin.NewResponse(plugin.StatusDone, rows), nil
}

func handleFleetDevice(_ *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	name, ok := validated.Value("name")
	if !ok {
		return plugin.OperatorErrorResponse(opererr.New(fleetDevicePath+": a device name is required",
			"the command reads the history of one managed device", fleetDevicesPath)), nil
	}
	devices, err := pluginserver.ManagedRegistrySnapshot()
	if err != nil {
		return registryReadRefusal(fleetDevicePath+" "+name, err), nil
	}
	for _, device := range devices {
		if device.Name == name {
			detail := fleetDeviceDetail{
				fleetDeviceRow: rowForDevice(device), FirstConnected: deviceTime(device.FirstConnected),
				LastAck: deviceTime(device.LastAck), AckResult: device.AckResult.String(),
				AckVersion: device.AckVersion, AckError: device.AckError, LastFetchError: device.LastFetchError,
				Persisted: device.Persisted, Unreadable: device.Unreadable, RemovedAt: deviceTime(device.RemovedAt),
			}
			return plugin.NewResponse(plugin.StatusDone, detail), nil
		}
	}
	return plugin.OperatorErrorResponse(opererr.New(fleetDevicePath+" "+name+": unknown device",
		"the hub has no declaration or record for this device", fleetDevicesPath)), nil
}

func registryReadRefusal(path string, cause error) *plugin.Response {
	return plugin.OperatorErrorResponse(opererr.Wrap(cause, path+": unable to read the device registry",
		"the hub could not supply its managed device records", "ze show doctor"))
}

func completeFleetDevices(_ *pluginserver.Server, completed []string) []pluginserver.Completion {
	if len(completed) != 1 {
		return nil
	}
	if completed[0] != "name" {
		return nil
	}
	devices, err := pluginserver.ManagedRegistrySnapshot()
	if errors.Is(err, pluginserver.ErrManagedNotServing) {
		return nil // A daemon that serves no managed devices has no names to complete.
	}
	// refuse-by-default: guards handleFleetDevice -- completion can return only candidates; executing the command reports the registry error.
	if err != nil {
		logger().Warn("the fleet registry could not be read for device name completion",
			"code", diagnostic.CodeDoctorStoreIntegrity, "error", err, "fix", "ze show doctor")
		return nil
	}
	names := make([]pluginserver.Completion, 0, len(devices))
	for _, device := range devices {
		names = append(names, pluginserver.Completion{Value: device.Name, ShortHelp: "Managed device"})
	}
	return names
}
