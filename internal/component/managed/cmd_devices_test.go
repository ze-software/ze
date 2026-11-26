// VALIDATES: fleet-1 AC-7, AC-10 to AC-13, AC-34. The real dispatcher selects
// the two registered RPCs, carries the label operation and decodes the detail name.
// PREVENTS: CLI-only filtering, leaking held details in the table, or claiming a
// device is in sync before its first fetch or while a held copy exists.

package managed

import (
	"encoding/json"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	configyang "github.com/ze-software/ze/internal/component/config/yang"
	_ "github.com/ze-software/ze/internal/component/managed/yang" // Link the same command schema the daemon composition root registers.
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/pkg/fleet"
)

func registryDispatcher(t *testing.T) *pluginserver.Server {
	t.Helper()
	server, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, nil)
	require.NoError(t, err)
	t.Cleanup(server.Stop)
	return server
}

func startRegistryForCommand(t *testing.T) {
	t.Helper()
	startRegistryWithReader(t, func(name string) ([]byte, error) { return []byte(name), nil })
}

func startRegistryWithReader(t *testing.T, read pluginserver.ConfigReader) {
	t.Helper()
	server, err := pluginserver.NewManagedServer(pluginserver.ManagedServerConfig{
		Authority: testRoot(t),
		Listeners: []pluginserver.ManagedListener{{Addr: "127.0.0.1:0",
			Clients: map[string]string{"edge-01": testHubToken, "edge-02": testHubToken, "edge-03": testHubToken},
			ClientLabels: map[string]map[string]string{
				"edge-01": {"site": "lon", "role": "core"}, "edge-02": {"site": "par"}, "edge-03": {"site": "lon"},
			},
		}},
		ReadConfig: read,
	})
	require.NoError(t, err)
	require.NoError(t, server.Start(t.Context()))
	t.Cleanup(server.Stop)
}

func dispatchRegistry(t *testing.T, server *pluginserver.Server, line string) *plugin.Response {
	t.Helper()
	resp, err := server.Dispatcher().Dispatch(&pluginserver.CommandContext{Server: server}, line)
	require.NoError(t, err)
	require.NotNil(t, resp)
	return resp
}

func registryAnswer(t *testing.T, server *pluginserver.Server, line string, answer any) {
	t.Helper()
	resp := dispatchRegistry(t, server, line)
	require.Equal(t, plugin.StatusDone, resp.Status, "%s", resp.Error)
	data, err := plugin.ResponseJSON(resp, nil)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal([]byte(data), answer), "%s returned %s", line, data)
}

func TestShowFleetDevicesAnswersRows(t *testing.T) {
	startRegistryForCommand(t)
	server := registryDispatcher(t)
	var rows []map[string]any
	registryAnswer(t, server, fleetDevicesPath, &rows)
	require.Len(t, rows, 3)
	require.Equal(t, map[string]any{
		"name": "edge-01", "connected": false, "config-state": "never-fetched",
		"labels": map[string]any{"site": "lon", "role": "core"}, "hub-version": fleet.VersionHash([]byte("edge-01")),
		"device-version": "", "last-fetch": "", "last-fetch-result": "", "connected-since": "", "last-seen": "",
		"source-address": "", "ever-connected": false, "diverged": false, "removed": false,
	}, rows[0])
}

func TestShowFleetDevicesLabelFilter(t *testing.T) {
	startRegistryForCommand(t)
	server := registryDispatcher(t)
	for _, tc := range []struct {
		pipe string
		want []string
	}{
		{" | label site=lon", []string{"edge-01", "edge-03"}},
		{" | label site=lon | label role=core", []string{"edge-01"}},
		{" | label site=par", []string{"edge-02"}},
		{" | label site=none", []string{}},
	} {
		var answer struct {
			Data []fleetDeviceRow `json:"data"`
			Pipe []struct {
				Op  string `json:"op"`
				Arg string `json:"arg"`
			} `json:"pipe"`
		}
		registryAnswer(t, server, fleetDevicesPath+tc.pipe, &answer)
		require.NotNil(t, answer.Data, "empty results must be []")
		var pipeline string
		for _, op := range answer.Pipe {
			pipeline += " | " + op.Op + " " + op.Arg
		}
		require.Equal(t, tc.pipe, pipeline, "dispatcher reports the row-selection operations it applied")
		var names []string
		for _, row := range answer.Data {
			names = append(names, row.Name)
		}
		require.True(t, slices.Equal(tc.want, names), "%s returned %v, want %v", tc.pipe, names, tc.want)
	}
	for _, bad := range []string{"site", "=lon", "site="} {
		resp := dispatchRegistry(t, server, fleetDevicesPath+" | label "+bad)
		require.Equal(t, plugin.StatusError, resp.Status)
		require.Contains(t, resp.Error, "key=value")
	}
	var args command.ValidatedArgs
	args = args.WithPipeOperations([]command.PipeOperation{{Name: "other", Args: []string{"site=lon"}}})
	resp, err := handleFleetDevices(&pluginserver.CommandContext{}, args)
	require.NoError(t, err)
	require.Equal(t, plugin.StatusError, resp.Status)
	require.Contains(t, resp.Error, "unknown fleet device pipe operation other")
}

func TestShowFleetDeviceAnswersDetail(t *testing.T) {
	startRegistryForCommand(t)
	server := registryDispatcher(t)
	var detail map[string]any
	registryAnswer(t, server, fleetDevicePath+" edge-02", &detail)
	var rows []map[string]any
	registryAnswer(t, server, fleetDevicesPath, &rows)
	for key, value := range rows[1] {
		require.Equal(t, value, detail[key], key)
		delete(detail, key)
	}
	require.Equal(t, map[string]any{"first-connected": "", "last-ack": "", "ack-version": "",
		"ack-result": "", "ack-error": "", "last-fetch-error": "", "persisted": false,
		"unreadable": false, "removed-at": ""}, detail)
}

func TestShowFleetDeviceUnknownName(t *testing.T) {
	startRegistryForCommand(t)
	resp := dispatchRegistry(t, registryDispatcher(t), fleetDevicePath+" absent")
	require.Equal(t, plugin.StatusError, resp.Status)
	require.Contains(t, resp.Error, "absent: unknown device")
	require.Contains(t, resp.Error, fleetDevicesPath)
}

func TestShowFleetDeviceReadsOnlyRequestedConfig(t *testing.T) {
	var reads []string
	startRegistryWithReader(t, func(name string) ([]byte, error) {
		reads = append(reads, name)
		if name == "edge-02" {
			return []byte("target config"), nil
		}
		return nil, errors.New("cannot read unrelated configuration")
	})
	server := registryDispatcher(t)
	require.Len(t, completeFleetDevices(server, []string{"name"}), 3)
	require.Empty(t, reads)
	resp := dispatchRegistry(t, server, fleetDevicePath+" absent")
	require.Contains(t, resp.Error, "absent: unknown device")
	require.Empty(t, reads)
	var detail fleetDeviceDetail
	registryAnswer(t, server, fleetDevicePath+" edge-02", &detail)
	require.Equal(t, fleet.VersionHash([]byte("target config")), detail.HubVersion)
	require.Equal(t, []string{"edge-02"}, reads)
	resp = dispatchRegistry(t, server, fleetDevicePath+" edge-01")
	require.Contains(t, resp.Error, "cannot read unrelated configuration")
}

func TestShowFleetDevicesNotServing(t *testing.T) {
	resp := dispatchRegistry(t, registryDispatcher(t), fleetDevicesPath)
	require.Equal(t, plugin.StatusError, resp.Status)
	require.Contains(t, resp.Error, "unable to read the device registry")
	require.Contains(t, resp.Error, "ze show doctor")
}

func TestShowFleetDevicesConfigState(t *testing.T) {
	device := pluginserver.ManagedDevice{ManagedDeviceRecord: pluginserver.ManagedDeviceRecord{
		AckResult: pluginserver.ManagedAckRefused, AckVersion: "v1",
	}, HubVersion: "v1", Diverged: true}
	require.Equal(t, "diverged", deviceConfigState(&device))
	device.Diverged = false
	require.Equal(t, "never-fetched", deviceConfigState(&device))
	device.LastFetch = time.Now()
	require.Equal(t, "rejected", deviceConfigState(&device))
	device.AckResult = pluginserver.ManagedAckHolding
	require.Equal(t, "pending", deviceConfigState(&device))
	device.AckResult = pluginserver.ManagedAckApplied
	require.Equal(t, "in-sync", deviceConfigState(&device))
	device.HubVersion = "v2"
	require.Equal(t, "pending", deviceConfigState(&device))
	device.AckVersion, device.HubVersion = "", ""
	require.Equal(t, "pending", deviceConfigState(&device))

	device.Labels = map[string]string{"site": "lon"}
	row := rowForDevice(&device)
	require.True(t, row.Removed)
	require.Empty(t, row.Labels)
}

func TestShowFleetDevicesEverConnectedAfterForgettingHistory(t *testing.T) {
	device := pluginserver.ManagedDevice{Persisted: true, ManagedDeviceRecord: pluginserver.ManagedDeviceRecord{
		ConnectedSince: time.Now().UTC(), LastSeen: time.Now().UTC(),
	}}
	require.True(t, rowForDevice(&device).EverConnected)
	device.ConnectedSince = time.Time{}
	require.True(t, rowForDevice(&device).EverConnected)
	device.LastSeen = time.Time{}
	require.False(t, rowForDevice(&device).EverConnected)
}

func TestShowFleetDeviceCompletion(t *testing.T) {
	startRegistryForCommand(t)
	names := completeFleetDevices(nil, []string{"name"})
	var got []string
	for _, name := range names {
		got = append(got, name.Value)
	}
	require.Equal(t, []string{"edge-01", "edge-02", "edge-03"}, got)

	schema, err := configyang.DefaultLoader()
	require.NoError(t, err)
	completer := command.NewTreeCompleter(configyang.BuildCommandTree(schema))
	for _, tc := range []struct {
		line string
		want []string
	}{
		{fleetDevicesPath + " | lab", []string{"label"}},
		{fleetDevicesPath + " | display hub-", []string{"hub-version"}},
		{fleetDevicePath + " edge-01 | display ack-v", []string{"ack-version"}},
	} {
		var fields []string
		for _, suggestion := range completer.Complete(tc.line) {
			fields = append(fields, suggestion.Text)
		}
		require.Equal(t, tc.want, fields, tc.line)
	}
}
