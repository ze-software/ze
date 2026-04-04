package cmd

import (
	"testing"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/iface"

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

func TestHandleInterfaceShow(t *testing.T) {
	// List all interfaces -- should succeed on any platform.
	resp, err := handleInterfaceShow(nil, nil)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)
	assert.Contains(t, resp.Data, "lo") // loopback always exists

	// Show specific interface -- loopback always exists.
	resp, err = handleInterfaceShow(nil, []string{"lo"})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)
	assert.Contains(t, resp.Data, "lo")

	// Show nonexistent interface -- should return error response.
	resp, err = handleInterfaceShow(nil, []string{"nonexistent_iface99"})
	require.NoError(t, err) // Go error nil, operational error in Response
	require.NotNil(t, resp)
	assert.Equal(t, "error", resp.Status)
}

func TestHandleInterfaceMigrateNoBus(t *testing.T) {
	// With no bus set, should return error response.
	resp, err := handleInterfaceMigrate(nil, []string{
		"--from", "eth0.0", "--to", "lo1.0", "--address", "10.0.0.1/24",
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "error", resp.Status)
	assert.Contains(t, resp.Data, "bus not available")
}
