// VALIDATES: fleet-4 AC-4, AC-5, AC-6, AC-7, AC-8, AC-14, AC-16, AC-17. The
// real dispatcher serves show fleet device inventory name, show fleet device
// health name, and show fleet health over a live ManagedServer, preserving
// snapshots across disconnect, distinguishing unreported devices, omitting raw
// SMART/drive serials, and refusing unknown or missing names.
// PREVENTS: missing RPC registration, unreported devices masquerading as
// healthy/zero hardware, null component slices, loss of last report on
// disconnect, or missing operator refusals.

package managed

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	configyang "github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginipc "github.com/ze-software/ze/internal/component/plugin/ipc"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	_ "github.com/ze-software/ze/internal/core/ipc/yang" // Link system command complete schema for live argument completion.
	"github.com/ze-software/ze/internal/core/network"
	"github.com/ze-software/ze/pkg/fleet"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func startReportCommandsServer(t *testing.T, clients ...string) string {
	t.Helper()
	root := testRoot(t)
	installCA(t, root)
	clientSecrets := make(map[string]string, len(clients))
	for _, name := range clients {
		clientSecrets[name] = testHubToken
	}
	srv, err := pluginserver.NewManagedServer(pluginserver.ManagedServerConfig{
		Authority: root,
		Listeners: []pluginserver.ManagedListener{{
			Addr:    "127.0.0.1:0",
			Clients: clientSecrets,
		}},
		ReadConfig: func(name string) ([]byte, error) { return []byte(name), nil },
	})
	require.NoError(t, err)
	require.NoError(t, srv.Start(t.Context()))
	t.Cleanup(srv.Stop)
	return srv.Addrs()[0].String()
}

func connectReportCommandClient(t *testing.T, addr, name string) *rpc.MuxConn {
	t.Helper()
	cfg := &ClientConfig{
		Server: addr,
		Name:   name,
		Token:  testHubToken,
		CA:     "fleet-hub",
	}
	tlsConf, err := clientTLSConfig(cfg)
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	dialer := &network.RealDialer{}
	rawConn, err := dialer.DialContext(ctx, "tcp", addr)
	require.NoError(t, err)
	tlsConn := tls.Client(rawConn, tlsConf)
	require.NoError(t, tlsConn.HandshakeContext(ctx))
	require.NoError(t, pluginipc.SendAuth(ctx, tlsConn, cfg.Token, cfg.Name))
	line, err := readLine(tlsConn, maxAuthResponse)
	require.NoError(t, err)
	_, verb, _, parseErr := rpc.ParseLine(line)
	require.NoError(t, parseErr)
	require.Equal(t, "ok", verb)
	mc := rpc.NewMuxConn(rpc.NewConn(tlsConn, tlsConn))
	t.Cleanup(func() {
		_ = mc.Close() //nolint:errcheck // test teardown
	})
	return mc
}

func sendTestInventory(t *testing.T, mc *rpc.MuxConn, report fleet.InventoryReport) {
	t.Helper()
	payload, err := fleet.EncodeInventoryReport(report)
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	_, err = mc.CallRPC(ctx, fleet.VerbInventoryReport, json.RawMessage(payload))
	require.NoError(t, err)
}

func sendTestHealth(t *testing.T, mc *rpc.MuxConn, report fleet.HealthReport) {
	t.Helper()
	payload, err := fleet.EncodeHealthReport(report)
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	_, err = mc.CallRPC(ctx, fleet.VerbHealthReport, json.RawMessage(payload))
	require.NoError(t, err)
}

func waitDeviceDisconnected(t *testing.T, name string) {
	t.Helper()
	require.Eventually(t, func() bool {
		dev, err := pluginserver.ManagedDeviceByName(name)
		return err == nil && !dev.Connected
	}, 5*time.Second, 10*time.Millisecond, "device %s did not disconnect", name)
}

func TestShowFleetReportsUnreportedAndReportedAndDisconnected(t *testing.T) {
	addr := startReportCommandsServer(t, "edge-01", "edge-02", "edge-03", "edge-04")
	server := registryDispatcher(t)

	// Unreported device: explicit not-reported state with empty components array.
	var invUnreported map[string]any
	registryAnswer(t, server, fleetDeviceInventoryPath+" edge-01", &invUnreported)
	require.Equal(t, map[string]any{
		"name":        "edge-01",
		"connected":   false,
		"reported":    false,
		"status":      fleetReportStatusNotReported,
		"received-at": "",
	}, invUnreported)

	var healthUnreported map[string]any
	registryAnswer(t, server, fleetDeviceHealthPath+" edge-01", &healthUnreported)
	require.Equal(t, map[string]any{
		"name":        "edge-01",
		"connected":   false,
		"reported":    false,
		"status":      fleetReportStatusNotReported,
		"received-at": "",
		"components":  []any{},
	}, healthUnreported)

	var emptySummary fleetHealthSummaryDoc
	registryAnswer(t, server, fleetHealthPath, &emptySummary)
	require.Equal(t, 4, emptySummary.Total)
	require.Equal(t, 0, emptySummary.Healthy)
	require.Equal(t, 0, emptySummary.Degraded)
	require.Equal(t, 0, emptySummary.Down)
	require.Equal(t, 4, emptySummary.Unreported)
	require.NotNil(t, emptySummary.Devices)
	require.Empty(t, emptySummary.Devices)

	// Connect edge-01 (full inventory + healthy), edge-02 (partial inventory + degraded),
	// and edge-03 (down); leave edge-04 unreported.
	mc1 := connectReportCommandClient(t, addr, "edge-01")
	sendTestInventory(t, mc1, fleet.InventoryReport{
		CollectedAt:     "2026-11-26T10:00:00Z",
		SoftwareVersion: "0.9.1-rc1",
		CPU:             fleet.InventoryCPU{Available: true, LogicalCPUs: 16, Model: "AMD EPYC 7313P"},
		NIC:             fleet.InventoryNIC{Available: true, Count: 2, Drivers: []string{"ice", "ixgbe"}},
		Memory: fleet.InventoryMemory{
			Available:  true,
			TotalBytes: 64 << 30,
			ECC:        &fleet.InventoryECC{CorrectableErrors: 3, UncorrectableErrors: 0},
		},
		Storage: fleet.InventoryStorage{Available: true, DeviceCount: 2, TotalBytes: 1 << 40},
		DMI:     fleet.InventoryDMI{Available: true, Model: "Dell PowerEdge R650", Serial: "SN-EDGE-01"},
	})
	sendTestHealth(t, mc1, fleet.HealthReport{
		Status:    fleet.HealthStatusHealthy,
		CheckedAt: "2026-11-26T10:01:00Z",
		Components: []fleet.HealthComponent{
			{Name: "bgp", Status: fleet.HealthStatusHealthy, CheckedAt: "2026-11-26T10:01:00Z"},
		},
	})

	mc2 := connectReportCommandClient(t, addr, "edge-02")
	sendTestInventory(t, mc2, fleet.InventoryReport{
		CollectedAt:     "2026-11-26T10:02:00Z",
		SoftwareVersion: "0.9.1-rc1",
		CPU:             fleet.InventoryCPU{Available: true, LogicalCPUs: 4, Model: "Intel Xeon"},
		NIC:             fleet.InventoryNIC{Available: true, Count: 0},
		Memory:          fleet.InventoryMemory{Available: true, TotalBytes: 16 << 30},
		Storage:         fleet.InventoryStorage{Available: false},
		DMI:             fleet.InventoryDMI{Available: false},
	})
	sendTestHealth(t, mc2, fleet.HealthReport{
		Status:    fleet.HealthStatusDegraded,
		CheckedAt: "2026-11-26T10:02:30Z",
		Components: []fleet.HealthComponent{
			{Name: "bgp", Status: fleet.HealthStatusDegraded, Reason: "2 of 4 peers down", CheckedAt: "2026-11-26T10:02:30Z"},
		},
	})

	mc3 := connectReportCommandClient(t, addr, "edge-03")
	sendTestHealth(t, mc3, fleet.HealthReport{
		Status:    fleet.HealthStatusDown,
		CheckedAt: "2026-11-26T10:03:00Z",
		Components: []fleet.HealthComponent{
			{Name: "fib", Status: fleet.HealthStatusDown, Reason: "netlink sync failed", CheckedAt: "2026-11-26T10:03:00Z"},
		},
	})

	// Check full inventory answer including ECC, DMI, software version, and absence of SMART/drive serials.
	invResp := dispatchRegistry(t, server, fleetDeviceInventoryPath+" edge-01")
	require.Equal(t, plugin.StatusDone, invResp.Status, "%s", invResp.Error)
	rawInvJSON, err := plugin.ResponseJSON(invResp, nil)
	require.NoError(t, err)
	lowerInvJSON := strings.ToLower(rawInvJSON)
	require.NotContains(t, lowerInvJSON, "smart")
	require.NotContains(t, lowerInvJSON, "drive-serial")

	var invDoc fleetDeviceInventoryDoc
	require.NoError(t, json.Unmarshal([]byte(rawInvJSON), &invDoc))
	require.Equal(t, "edge-01", invDoc.Name)
	require.True(t, invDoc.Connected)
	require.True(t, invDoc.Reported)
	require.Equal(t, fleetReportStatusReported, invDoc.Status)
	require.NotEmpty(t, invDoc.ReceivedAt)
	_, err = time.Parse(time.RFC3339Nano, invDoc.ReceivedAt)
	require.NoError(t, err)
	require.Equal(t, "2026-11-26T10:00:00Z", invDoc.CollectedAt)
	require.Equal(t, "0.9.1-rc1", invDoc.SoftwareVersion)
	require.Equal(t, "Dell PowerEdge R650", invDoc.Model)
	require.Equal(t, "SN-EDGE-01", invDoc.Serial)
	require.NotNil(t, invDoc.CPU)
	require.Equal(t, 16, invDoc.CPU.LogicalCPUs)
	require.NotNil(t, invDoc.NIC)
	require.Equal(t, []string{"ice", "ixgbe"}, invDoc.NIC.Drivers)
	require.NotNil(t, invDoc.Memory)
	require.NotNil(t, invDoc.Memory.ECC)
	require.Equal(t, uint64(3), invDoc.Memory.ECC.CorrectableErrors)
	require.NotNil(t, invDoc.Storage)
	require.Equal(t, 2, invDoc.Storage.DeviceCount)
	require.NotNil(t, invDoc.DMI)
	require.True(t, invDoc.DMI.Available)

	// Structured selector resolution against the YANG command schema resolves the
	// same inventory and health JSON as text dispatch, and refuses an omitted name.
	healthResp := dispatchRegistry(t, server, fleetDeviceHealthPath+" edge-01")
	require.Equal(t, plugin.StatusDone, healthResp.Status, "%s", healthResp.Error)
	rawHealthJSON, err := plugin.ResponseJSON(healthResp, nil)
	require.NoError(t, err)
	var healthPos fleetDeviceHealthDoc
	require.NoError(t, json.Unmarshal([]byte(rawHealthJSON), &healthPos))
	require.Equal(t, "edge-01", healthPos.Name)
	require.Equal(t, "healthy", healthPos.Status)

	for _, cmd := range []struct {
		path    string
		handler func(*pluginserver.CommandContext, command.ValidatedArgs) (*plugin.Response, error)
		want    string
	}{
		{fleetDeviceInventoryPath, handleFleetDeviceInventory, rawInvJSON},
		{fleetDeviceHealthPath, handleFleetDeviceHealth, rawHealthJSON},
	} {
		for _, sel := range []struct {
			label      string
			args       []string
			preMatched map[string]string
		}{
			{"positional", []string{"edge-01"}, nil},
			{"keyed", []string{fleetDeviceNameLeaf, "edge-01"}, nil},
			{"prematched", nil, map[string]string{fleetDeviceNameLeaf: "edge-01"}},
		} {
			validated, validateErr := command.ValidateModelArgs(cmd.path, sel.args, sel.preMatched)
			require.NoError(t, validateErr, "%s (%s)", cmd.path, sel.label)
			resp, handleErr := cmd.handler(&pluginserver.CommandContext{}, validated)
			require.NoError(t, handleErr, "%s (%s)", cmd.path, sel.label)
			require.Equal(t, plugin.StatusDone, resp.Status, "%s (%s): %s", cmd.path, sel.label, resp.Error)
			gotJSON, jsonErr := plugin.ResponseJSON(resp, nil)
			require.NoError(t, jsonErr, "%s (%s)", cmd.path, sel.label)
			require.Equal(t, cmd.want, gotJSON, "%s (%s)", cmd.path, sel.label)
		}
		_, missingErr := command.ValidateModelArgs(cmd.path, nil, nil)
		require.Error(t, missingErr, cmd.path)
		require.ErrorContains(t, missingErr, "required argument missing: name", cmd.path)
	}

	// Check partial inventory answer: unavailable DMI omits top-level model/serial and marks available=false.
	var partialInv fleetDeviceInventoryDoc
	registryAnswer(t, server, fleetDeviceInventoryPath+" edge-02", &partialInv)
	require.True(t, partialInv.Reported)
	require.Empty(t, partialInv.Model)
	require.Empty(t, partialInv.Serial)
	require.NotNil(t, partialInv.Storage)
	require.False(t, partialInv.Storage.Available)
	require.NotNil(t, partialInv.DMI)
	require.False(t, partialInv.DMI.Available)
	require.NotNil(t, partialInv.Memory)
	require.Nil(t, partialInv.Memory.ECC)

	// Check degraded health detail.
	var healthDoc fleetDeviceHealthDoc
	registryAnswer(t, server, fleetDeviceHealthPath+" edge-02", &healthDoc)
	require.Equal(t, "edge-02", healthDoc.Name)
	require.True(t, healthDoc.Connected)
	require.True(t, healthDoc.Reported)
	require.Equal(t, "degraded", healthDoc.Status)
	require.NotEmpty(t, healthDoc.ReceivedAt)
	require.Equal(t, "2026-11-26T10:02:30Z", healthDoc.CheckedAt)
	require.Len(t, healthDoc.Components, 1)
	require.Equal(t, "bgp", healthDoc.Components[0].Name)
	require.Equal(t, "2 of 4 peers down", healthDoc.Components[0].Reason)

	// Check fleet health summary.
	var summary fleetHealthSummaryDoc
	registryAnswer(t, server, fleetHealthPath, &summary)
	require.Equal(t, 4, summary.Total)
	require.Equal(t, 1, summary.Healthy)
	require.Equal(t, 1, summary.Degraded)
	require.Equal(t, 1, summary.Down)
	require.Equal(t, 1, summary.Unreported)
	require.Len(t, summary.Devices, 2)
	require.Equal(t, "edge-02", summary.Devices[0].Name)
	require.Equal(t, "degraded", summary.Devices[0].Status)
	require.True(t, summary.Devices[0].Connected)
	require.Equal(t, healthDoc.ReceivedAt, summary.Devices[0].ReceivedAt)
	require.Equal(t, "edge-03", summary.Devices[1].Name)
	require.Equal(t, "down", summary.Devices[1].Status)
	require.True(t, summary.Devices[1].Connected)

	// Disconnect edge-01 and edge-02; snapshots and receipt timestamps must survive.
	require.NoError(t, mc1.Close())
	require.NoError(t, mc2.Close())
	waitDeviceDisconnected(t, "edge-01")
	waitDeviceDisconnected(t, "edge-02")

	var invOffline fleetDeviceInventoryDoc
	registryAnswer(t, server, fleetDeviceInventoryPath+" edge-01", &invOffline)
	require.False(t, invOffline.Connected)
	require.True(t, invOffline.Reported)
	require.Equal(t, fleetReportStatusReported, invOffline.Status)
	require.Equal(t, invDoc.ReceivedAt, invOffline.ReceivedAt)
	require.Equal(t, invDoc.Model, invOffline.Model)
	require.Equal(t, invDoc.Serial, invOffline.Serial)

	var healthOffline fleetDeviceHealthDoc
	registryAnswer(t, server, fleetDeviceHealthPath+" edge-02", &healthOffline)
	require.False(t, healthOffline.Connected)
	require.True(t, healthOffline.Reported)
	require.Equal(t, "degraded", healthOffline.Status)
	require.Equal(t, healthDoc.ReceivedAt, healthOffline.ReceivedAt)
	require.Len(t, healthOffline.Components, 1)

	var summaryOffline fleetHealthSummaryDoc
	registryAnswer(t, server, fleetHealthPath, &summaryOffline)
	require.Equal(t, 4, summaryOffline.Total)
	require.Equal(t, 1, summaryOffline.Healthy)
	require.Equal(t, 1, summaryOffline.Degraded)
	require.Equal(t, 1, summaryOffline.Down)
	require.Equal(t, 1, summaryOffline.Unreported)
	require.Len(t, summaryOffline.Devices, 2)
	require.Equal(t, "edge-02", summaryOffline.Devices[0].Name)
	require.False(t, summaryOffline.Devices[0].Connected)
	require.Equal(t, healthDoc.ReceivedAt, summaryOffline.Devices[0].ReceivedAt)
}

func TestShowFleetReportsRefusalsAndCompletion(t *testing.T) {
	// Not serving: all three commands refuse with registryReadRefusal.
	server := registryDispatcher(t)
	for _, cmd := range []string{
		fleetDeviceInventoryPath + " edge-01",
		fleetDeviceHealthPath + " edge-01",
		fleetHealthPath,
	} {
		resp := dispatchRegistry(t, server, cmd)
		require.Equal(t, plugin.StatusError, resp.Status, cmd)
		require.Contains(t, resp.Error, "unable to read the device registry", cmd)
		require.Contains(t, resp.Error, "ze show doctor", cmd)
	}

	// Direct handler call with missing name refuses with show fleet devices fix.
	for _, tc := range []struct {
		label   string
		handler func(*pluginserver.CommandContext, command.ValidatedArgs) (*plugin.Response, error)
	}{
		{"inventory", handleFleetDeviceInventory},
		{"health", handleFleetDeviceHealth},
	} {
		resp, err := tc.handler(&pluginserver.CommandContext{}, command.ValidatedArgs{})
		require.NoError(t, err, tc.label)
		require.Equal(t, plugin.StatusError, resp.Status, tc.label)
		require.Contains(t, resp.Error, "a device name is required", tc.label)
		require.Contains(t, resp.Error, fleetDevicesPath, tc.label)
	}

	// Start serving and check unknown device refusals and omitted mandatory leaf refusal.
	startReportCommandsServer(t, "edge-01", "edge-02")
	for _, cmd := range []string{
		fleetDeviceInventoryPath + " absent",
		fleetDeviceHealthPath + " absent",
	} {
		resp := dispatchRegistry(t, server, cmd)
		require.Equal(t, plugin.StatusError, resp.Status, cmd)
		require.Contains(t, resp.Error, "absent: unknown device", cmd)
		require.Contains(t, resp.Error, fleetDevicesPath, cmd)
	}
	for _, cmd := range []string{fleetDeviceInventoryPath, fleetDeviceHealthPath} {
		_, err := server.Dispatcher().Dispatch(&pluginserver.CommandContext{Server: server}, cmd)
		require.Error(t, err, cmd)
		require.ErrorContains(t, err, "required argument missing: name", cmd)
	}

	// Check column and live device-name Tab completion for all report commands.
	schema, err := configyang.DefaultLoader()
	require.NoError(t, err)
	tree := configyang.BuildCommandTree(schema)
	server.WireArgCompletion(tree, func(line string) (string, error) {
		resp, dispatchErr := server.Dispatcher().Dispatch(&pluginserver.CommandContext{Server: server}, line)
		return plugin.ResponseJSON(resp, dispatchErr)
	})
	completer := command.NewTreeCompleter(tree)
	for _, tc := range []struct {
		line string
		want []string
	}{
		{fleetDeviceInventoryPath + " edge-01 | display software-", []string{"software-version"}},
		{fleetDeviceHealthPath + " edge-01 | display check", []string{"checked-at"}},
		{fleetHealthPath + " | display unrep", []string{"unreported"}},
		{fleetDeviceInventoryPath + " ", []string{"name", "edge-01", "edge-02"}},
		{fleetDeviceInventoryPath + " ed", []string{"edge-01", "edge-02"}},
		{fleetDeviceInventoryPath + " name ", []string{"edge-01", "edge-02"}},
		{fleetDeviceInventoryPath + " edge-01 ", nil},
		{fleetDeviceHealthPath + " ", []string{"name", "edge-01", "edge-02"}},
		{fleetDevicePath + " ", []string{"name", "edge-01", "edge-02"}},
	} {
		suggestions, completeErr := completer.CompleteChecked(tc.line)
		require.NoError(t, completeErr, tc.line)
		var fields []string
		for _, suggestion := range suggestions {
			fields = append(fields, suggestion.Text)
		}
		require.Equal(t, tc.want, fields, tc.line)
	}
}
