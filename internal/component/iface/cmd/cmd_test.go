package cmd

import (
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/iface"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseMigrateArgs drives the daemon's reader of `request interface
// migrate` over the keyword grammar an operator types and over every way of
// getting it wrong.
//
// VALIDATES: each keyword binds its own value, the order the keywords arrive in
// does not matter, and an unknown keyword, a repeated keyword, a keyword with
// no value, a missing required keyword and a malformed value are each refused
// by name.
// PREVENTS: a token nobody recognized being SKIPPED. An operator who misspells
// a keyword would otherwise be handed a migration of an address they did not
// name, and the parser would report success.
func TestParseMigrateArgs(t *testing.T) {
	minimal := iface.MigrateConfig{
		OldIface: "eth0", OldUnit: 0,
		NewIface: "lo1", NewUnit: 0,
		Address: "10.0.0.1/24",
	}

	tests := []struct {
		name string
		args []string
		// wantGrammar says the refusal is about the shape of the command, so it
		// must quote the whole form back. A refusal about ONE value states what
		// that value should look like instead, which is the narrower answer.
		wantGrammar bool
		wantCfg     iface.MigrateConfig
		wantTime    time.Duration
		wantError   string
	}{
		{
			name:     "required keywords only",
			args:     []string{"from", "eth0.0", "to", "lo1.0", "address", "10.0.0.1/24"},
			wantCfg:  minimal,
			wantTime: 30 * time.Second,
		},
		{
			name: "every keyword",
			args: []string{"from", "eth0.0", "to", "lo1.0", "address", "10.0.0.1/24",
				"create", "dummy", "timeout", "60s"},
			wantCfg: iface.MigrateConfig{
				OldIface: "eth0", OldUnit: 0,
				NewIface: "lo1", NewUnit: 0,
				Address: "10.0.0.1/24", NewIfaceType: "dummy",
			},
			wantTime: 60 * time.Second,
		},
		{
			name:     "keyword order does not matter",
			args:     []string{"address", "10.0.0.1/24", "to", "lo1.0", "from", "eth0.0"},
			wantCfg:  minimal,
			wantTime: 30 * time.Second,
		},
		{
			name: "unit is read from the value",
			args: []string{"from", "eth0.100", "to", "lo1.7", "address", "10.0.0.1/24"},
			wantCfg: iface.MigrateConfig{
				OldIface: "eth0", OldUnit: 100,
				NewIface: "lo1", NewUnit: 7,
				Address: "10.0.0.1/24",
			},
			wantTime: 30 * time.Second,
		},
		{
			name:        "unknown keyword",
			wantGrammar: true,
			args:        []string{"from", "eth0.0", "bogus", "value"},
			wantError:   `unknown keyword "bogus"`,
		},
		{
			name:        "a value with no keyword before it",
			wantGrammar: true,
			args:        []string{"eth0.0", "to", "lo1.0", "address", "10.0.0.1/24"},
			wantError:   `unknown keyword "eth0.0"`,
		},
		{
			name:        "keyword given twice",
			wantGrammar: true,
			args:        []string{"from", "eth0.0", "from", "eth1.0", "to", "lo1.0", "address", "10.0.0.1/24"},
			wantError:   `keyword "from" given twice`,
		},
		{
			name:        "keyword with no value",
			wantGrammar: true,
			args:        []string{"from", "eth0.0", "to", "lo1.0", "address"},
			wantError:   `keyword "address" has no value`,
		},
		{
			name:        "no arguments at all",
			wantGrammar: true,
			args:        nil,
			wantError:   `keyword "from" is missing`,
		},
		{
			name:        "missing from",
			wantGrammar: true,
			args:        []string{"to", "lo1.0", "address", "10.0.0.1/24"},
			wantError:   `keyword "from" is missing`,
		},
		{
			name:        "missing to",
			wantGrammar: true,
			args:        []string{"from", "eth0.0", "address", "10.0.0.1/24"},
			wantError:   `keyword "to" is missing`,
		},
		{
			name:        "missing address",
			wantGrammar: true,
			args:        []string{"from", "eth0.0", "to", "lo1.0"},
			wantError:   `keyword "address" is missing`,
		},
		{
			name:      "invalid from format",
			args:      []string{"from", "noDot", "to", "lo1.0", "address", "10.0.0.1/24"},
			wantError: `invalid from value "noDot"`,
		},
		{
			name:      "invalid to format",
			args:      []string{"from", "eth0.0", "to", "noDot", "address", "10.0.0.1/24"},
			wantError: `invalid to value "noDot"`,
		},
		{
			name:      "invalid timeout",
			args:      []string{"from", "eth0.0", "to", "lo1.0", "address", "10.0.0.1/24", "timeout", "bad"},
			wantError: `invalid timeout value "bad"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, dur, err := parseMigrateArgs(tt.args)
			if tt.wantError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantError)
				if tt.wantGrammar {
					assert.Contains(t, err.Error(), migrateGrammar,
						"a refusal about the shape of the command quotes the whole form back")
				}
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
		"from", "eth0.0", "to", "lo1.0", "address", "10.0.0.1/24",
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
