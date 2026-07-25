package cmd

import (
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/iface"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseMigrateArgs(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantCfg   iface.MigrateConfig
		wantTime  time.Duration
		wantError string
	}{
		{
			name: "valid minimal",
			args: []string{"--from", "eth0.0", "--to", "lo1.0", "--address", "10.0.0.1/24"},
			wantCfg: iface.MigrateConfig{
				OldIface: "eth0", OldUnit: 0,
				NewIface: "lo1", NewUnit: 0,
				Address: "10.0.0.1/24",
			},
			wantTime: 30 * time.Second,
		},
		{
			name: "with create and timeout",
			args: []string{"--from", "eth0.0", "--to", "lo1.0", "--address", "10.0.0.1/24",
				"--create", "dummy", "--timeout", "60s"},
			wantCfg: iface.MigrateConfig{
				OldIface: "eth0", OldUnit: 0,
				NewIface: "lo1", NewUnit: 0,
				Address: "10.0.0.1/24", NewIfaceType: "dummy",
			},
			wantTime: 60 * time.Second,
		},
		{
			name:      "missing from",
			args:      []string{"--to", "lo1.0", "--address", "10.0.0.1/24"},
			wantError: "--from, --to, and --address are required",
		},
		{
			name:      "missing to",
			args:      []string{"--from", "eth0.0", "--address", "10.0.0.1/24"},
			wantError: "--from, --to, and --address are required",
		},
		{
			name:      "missing address",
			args:      []string{"--from", "eth0.0", "--to", "lo1.0"},
			wantError: "--from, --to, and --address are required",
		},
		{
			name:      "invalid from format",
			args:      []string{"--from", "noDot", "--to", "lo1.0", "--address", "10.0.0.1/24"},
			wantError: "invalid --from",
		},
		{
			name:      "invalid to format",
			args:      []string{"--from", "eth0.0", "--to", "noDot", "--address", "10.0.0.1/24"},
			wantError: "invalid --to",
		},
		{
			name:      "invalid timeout",
			args:      []string{"--from", "eth0.0", "--to", "lo1.0", "--address", "10.0.0.1/24", "--timeout", "bad"},
			wantError: "invalid --timeout",
		},
		{
			name:      "unknown flag rejected",
			args:      []string{"--from", "eth0.0", "--bogus", "val"},
			wantError: "unknown argument",
		},
		{
			name:      "from missing value",
			args:      []string{"--from"},
			wantError: "--from requires a value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, dur, err := parseMigrateArgs(tt.args)
			if tt.wantError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantError)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantCfg, cfg)
			assert.Equal(t, tt.wantTime, dur)
		})
	}
}

func TestParseIfaceUnit(t *testing.T) {
	tests := []struct {
		input    string
		wantName string
		wantUnit int
		wantOK   bool
	}{
		{"eth0.0", "eth0", 0, true},
		{"lo1.5", "lo1", 5, true},
		{"eth0.100", "eth0", 100, true},
		{"br0.bridge.42", "br0.bridge", 42, true},
		{"noDot", "", 0, false},
		{".", "", 0, false},
		{".5", "", 0, false},
		{"eth0.", "", 0, false},
		{"eth0.abc", "", 0, false},
		{"eth0.-1", "", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			name, unit, ok := parseIfaceUnit(tt.input)
			assert.Equal(t, tt.wantOK, ok)
			if ok {
				assert.Equal(t, tt.wantName, name)
				assert.Equal(t, tt.wantUnit, unit)
			}
		})
	}
}

// TestHandleShowInterface is in internal/component/cmd/show/show_test.go
// because the handler moved to the show package (ze show interface verb syntax).

func TestHandleInterfaceMigrateNoBus(t *testing.T) {
	// With no bus set, should return error response.
	resp, err := handleInterfaceMigrate(nil, []string{
		"--from", "eth0.0", "--to", "lo1.0", "--address", "10.0.0.1/24",
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "error", resp.Status)
	assert.Contains(t, resp.Error, "bus not available")
}

// TestHandleInterfaceMTU_Validation verifies the MTU handler rejects
// malformed / out-of-range input BEFORE calling the backend.
// VALIDATES: MTU bound (68..65535) is enforced at the handler per
// rules/exact-or-reject.md.
// PREVENTS: regressions where a non-numeric or out-of-range MTU is
// passed to the backend and fails with a generic EINVAL.
func TestHandleInterfaceMTU_Validation(t *testing.T) {
	tests := []struct {
		name      string
		selectors map[string]string
		args      []string
		wantErr   string
	}{
		{"missing all", nil, nil, "usage: request interface"},
		{"missing bytes", map[string]string{"name": "eth0"}, nil, "usage: request interface"},
		{"non-numeric", map[string]string{"name": "eth0"}, []string{"abc"}, "invalid MTU"},
		{"below min", map[string]string{"name": "eth0"}, []string{"67"}, "out of range"},
		{"above max", map[string]string{"name": "eth0"}, []string{"65536"}, "out of range"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &pluginserver.CommandContext{Selectors: tt.selectors}
			resp, err := handleInterfaceMTU(ctx, tt.args)
			require.NoError(t, err)
			require.NotNil(t, resp)
			assert.Equal(t, "error", resp.Status)
			assert.Contains(t, resp.Error, tt.wantErr)
		})
	}
}

// TestHandleInterfaceMAC_Validation verifies the MAC handler rejects
// malformed MAC addresses BEFORE calling the backend.
// VALIDATES: MAC format (xx:xx:xx:xx:xx:xx) is enforced at the
// handler per rules/exact-or-reject.md.
// PREVENTS: regressions where a malformed MAC is passed to the
// backend and fails with a less specific kernel error.
func TestHandleInterfaceMAC_Validation(t *testing.T) {
	tests := []struct {
		name      string
		selectors map[string]string
		args      []string
		wantErr   string
	}{
		{"missing all", nil, nil, "usage: request interface"},
		{"missing address", map[string]string{"name": "eth0"}, nil, "usage: request interface"},
		{"too short", map[string]string{"name": "eth0"}, []string{"02:00:00:00:00"}, "invalid MAC"},
		{"non-hex", map[string]string{"name": "eth0"}, []string{"zz:zz:zz:zz:zz:zz"}, "invalid MAC"},
		{"wrong separator", map[string]string{"name": "eth0"}, []string{"02-00-00-00-00-01"}, "invalid MAC"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &pluginserver.CommandContext{Selectors: tt.selectors}
			resp, err := handleInterfaceMAC(ctx, tt.args)
			require.NoError(t, err)
			require.NotNil(t, resp)
			assert.Equal(t, "error", resp.Status)
			assert.Contains(t, resp.Error, tt.wantErr)
		})
	}
}

// TestIsValidMACAddress verifies the exported MAC validator accepts
// canonical forms and rejects malformed input.
// VALIDATES: IsValidMACAddress -- same regex as
// validators.MACAddressValidator, exposed so the offline CLI can
// reject identically.
func TestIsValidMACAddress(t *testing.T) {
	good := []string{
		"00:00:00:00:00:00",
		"02:00:00:00:00:01",
		"AB:CD:EF:12:34:56",
		"ab:cd:ef:12:34:56",
	}
	for _, m := range good {
		assert.True(t, IsValidMACAddress(m), "expected %q valid", m)
	}
	bad := []string{
		"",
		"not-a-mac",
		"02:00:00:00:00",       // too short
		"02:00:00:00:00:01:02", // too long
		"02-00-00-00-00-01",    // wrong sep
		"gg:00:00:00:00:01",    // non-hex
		"02:00:00:00:00:0x",    // non-hex
		"0200.0000.0001",       // cisco style rejected
	}
	for _, m := range bad {
		assert.False(t, IsValidMACAddress(m), "expected %q invalid", m)
	}
}

// TestHandleInterfaceUpDown_UsageGate verifies up/down handlers reject
// empty arg lists with the usage line.
// VALIDATES: admin state handlers reject missing arguments.
// PREVENTS: regressions where `interface up` with no name reaches the
// backend and is rejected by it with a less helpful error.
func TestHandleInterfaceUpDown_UsageGate(t *testing.T) {
	ctx := &pluginserver.CommandContext{}
	resp, err := handleInterfaceUp(ctx, nil)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "error", resp.Status)
	assert.Contains(t, resp.Error, "usage: request interface")

	resp, err = handleInterfaceDown(ctx, nil)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "error", resp.Status)
	assert.Contains(t, resp.Error, "usage: request interface")
}

// TestHandleUnitAdd_Validation verifies the unit-add handler rejects
// malformed input BEFORE calling the backend.
func TestHandleUnitAdd_Validation(t *testing.T) {
	tests := []struct {
		name      string
		selectors map[string]string
		args      []string
		wantErr   string
	}{
		{"missing all", nil, nil, "usage: create interface"},
		{"missing vid", map[string]string{"name": "eth0"}, nil, "usage: create interface"},
		{"non-numeric", map[string]string{"name": "eth0"}, []string{"abc"}, "invalid VLAN ID"},
		{"zero", map[string]string{"name": "eth0"}, []string{"0"}, "invalid VLAN ID"},
		{"above max", map[string]string{"name": "eth0"}, []string{"4095"}, "invalid VLAN ID"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &pluginserver.CommandContext{Selectors: tt.selectors}
			resp, err := handleUnitAdd(ctx, tt.args)
			require.NoError(t, err)
			require.NotNil(t, resp)
			assert.Equal(t, "error", resp.Status)
			assert.Contains(t, resp.Error, tt.wantErr)
		})
	}
}

// TestHandleUnitDel_Validation verifies the unit-del handler rejects
// malformed input and constructs the correct sub-interface name.
// PREVENTS: regression where handleUnitDel deleted the parent interface
// instead of the VLAN sub-interface (parent.vid).
func TestHandleUnitDel_Validation(t *testing.T) {
	tests := []struct {
		name      string
		selectors map[string]string
		args      []string
		wantErr   string
	}{
		{"missing all", nil, nil, "usage: delete interface"},
		{"missing vid", map[string]string{"name": "eth0"}, nil, "usage: delete interface"},
		{"non-numeric", map[string]string{"name": "eth0"}, []string{"abc"}, "invalid VLAN ID"},
		{"zero", map[string]string{"name": "eth0"}, []string{"0"}, "invalid VLAN ID"},
		{"above max", map[string]string{"name": "eth0"}, []string{"4095"}, "invalid VLAN ID"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &pluginserver.CommandContext{Selectors: tt.selectors}
			resp, err := handleUnitDel(ctx, tt.args)
			require.NoError(t, err)
			require.NotNil(t, resp)
			assert.Equal(t, "error", resp.Status)
			assert.Contains(t, resp.Error, tt.wantErr)
		})
	}
}

// TestHandleCreateBridge_UsageGate verifies the create-bridge handler
// rejects an empty arg list with the usage line.
func TestHandleCreateBridge_UsageGate(t *testing.T) {
	ctx := &pluginserver.CommandContext{}
	resp, err := handleCreateBridge(ctx, nil)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "error", resp.Status)
	assert.Contains(t, resp.Error, "usage: create interface bridge")
}
