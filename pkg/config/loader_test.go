package config

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLoadReactor verifies loading config into a Reactor.
//
// VALIDATES: Config creates properly configured Reactor.
//
// PREVENTS: Broken config → reactor integration.
func TestLoadReactor(t *testing.T) {
	input := `
router-id 10.0.0.1;
local-as 65000;
listen 127.0.0.1:1179;

peer 192.0.2.1 {
    peer-as 65001;
    hold-time 90;
}

peer 192.0.2.2 {
    peer-as 65002;
    passive true;
}
`

	r, err := LoadReactor(input)
	require.NoError(t, err)
	require.NotNil(t, r)

	peers := r.Peers()
	require.Len(t, peers, 2)
}

// TestLoadReactorInheritance verifies local-as inheritance.
//
// VALIDATES: Neighbors inherit global local-as.
//
// PREVENTS: Zero AS numbers in neighbors.
func TestLoadReactorInheritance(t *testing.T) {
	input := `
router-id 10.0.0.1;
local-as 65000;

peer 192.0.2.1 {
    peer-as 65001;
}
`

	r, err := LoadReactor(input)
	require.NoError(t, err)

	peers := r.Peers()
	require.Len(t, peers, 1)

	// Neighbor should inherit local-as from global
	n := peers[0].Settings()
	require.Equal(t, uint32(65000), n.LocalAS)
	require.Equal(t, uint32(65001), n.PeerAS)
}

// TestLoadReactorPassive verifies passive neighbor handling.
//
// VALIDATES: Passive neighbors are configured correctly.
//
// PREVENTS: Active connections to passive peers.
func TestLoadReactorPassive(t *testing.T) {
	input := `
router-id 10.0.0.1;
local-as 65000;

peer 192.0.2.1 {
    peer-as 65001;
    passive true;
}
`

	r, err := LoadReactor(input)
	require.NoError(t, err)

	peers := r.Peers()
	require.Len(t, peers, 1)

	n := peers[0].Settings()
	require.True(t, n.Passive)
}

// TestLoadReactorConfig verifies reactor config settings.
//
// VALIDATES: Listen address and router-id are set.
//
// PREVENTS: Missing reactor configuration.
func TestLoadReactorConfig(t *testing.T) {
	input := `
router-id 10.0.0.1;
local-as 65000;
listen 0.0.0.0:179;

peer 192.0.2.1 {
    peer-as 65001;
}
`

	cfg, r, err := LoadReactorWithConfig(input)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.NotNil(t, r)

	require.Equal(t, uint32(0x0a000001), cfg.RouterID) // 10.0.0.1
	require.Equal(t, uint32(65000), cfg.LocalAS)
}

// TestLoadReactorError verifies error handling.
//
// VALIDATES: Invalid config returns error.
//
// PREVENTS: Silent config failures.
func TestLoadReactorError(t *testing.T) {
	input := `
peer 192.0.2.1 {
    peer-as not-a-number;
}
`

	_, err := LoadReactor(input)
	require.Error(t, err)
}

// TestCreateReactorFromConfig verifies direct Config to Reactor.
//
// VALIDATES: CreateReactor works with typed Config.
//
// PREVENTS: Only string-based loading working.
func TestCreateReactorFromConfig(t *testing.T) {
	cfg := &BGPConfig{
		RouterID: 0x0a000001,
		LocalAS:  65000,
		Listen:   "127.0.0.1:1179",
	}

	r, err := CreateReactor(cfg)
	require.NoError(t, err)
	require.NotNil(t, r)
}

// TestParseAllConfigFiles verifies all etc/zebgp/*.conf files parse.
//
// VALIDATES: All example configs are syntactically valid.
//
// PREVENTS: Broken example configs shipped with the project.
func TestParseAllConfigFiles(t *testing.T) {
	files, err := filepath.Glob("../../etc/zebgp/*.conf")
	require.NoError(t, err)
	require.NotEmpty(t, files, "no config files found")

	p := NewParser(BGPSchema())

	for _, file := range files {
		t.Run(filepath.Base(file), func(t *testing.T) {
			data, err := os.ReadFile(file) //nolint:gosec // Test file from known directory
			require.NoError(t, err)

			_, err = p.Parse(string(data))
			require.NoError(t, err, "failed to parse %s", file)
		})
	}
}

// TestV2SyntaxHint verifies that v2 syntax errors include migration hint.
//
// VALIDATES: Users get helpful error message with migration instructions.
//
// PREVENTS: Confusing "unknown keyword" errors without guidance.
func TestV2SyntaxHint(t *testing.T) {
	t.Run("neighbor keyword triggers hint", func(t *testing.T) {
		input := `neighbor 192.0.2.1 { local-as 65000; peer-as 65001; }`
		_, err := LoadReactor(input)
		require.Error(t, err)
		require.Contains(t, err.Error(), "unknown top-level keyword: neighbor")
		require.Contains(t, err.Error(), "zebgp config migrate")
	})

	t.Run("template.neighbor triggers hint", func(t *testing.T) {
		input := `template { neighbor test { local-as 65000; } }`
		_, err := LoadReactor(input)
		require.Error(t, err)
		require.Contains(t, err.Error(), "unknown field in template: neighbor")
		require.Contains(t, err.Error(), "zebgp config migrate")
	})

	t.Run("v3 syntax no hint", func(t *testing.T) {
		// Valid v3 config should parse without error (no hint needed)
		input := `peer 192.0.2.1 { local-as 65000; peer-as 65001; }`
		_, err := LoadReactor(input)
		require.NoError(t, err)
	})
}

// TestBuildMUPNLRI_T1ST_Source verifies T1ST source field encoding.
//
// VALIDATES: MUP T1ST routes correctly encode the optional source field
// with source_len (1 byte) + source_addr (4 or 16 bytes).
//
// PREVENTS: Silent failures when source address is invalid,
// missing source encoding in NLRI output.
func TestBuildMUPNLRI_T1ST_Source(t *testing.T) {
	tests := []struct {
		name        string
		config      MUPRouteConfig
		wantHex     string // expected hex substring in NLRI (source_len + source_addr)
		wantErr     bool
		wantErrText string
	}{
		{
			name: "IPv4 T1ST with source",
			config: MUPRouteConfig{
				RouteType: "mup-t1st",
				IsIPv6:    false,
				Prefix:    "192.168.0.2/32",
				RD:        "100:100",
				TEID:      "12345",
				QFI:       9,
				Endpoint:  "10.0.0.1",
				Source:    "10.0.1.1",
			},
			// source: len=32 (0x20), addr=10.0.1.1 (0x0a000101)
			wantHex: "200a000101",
		},
		{
			name: "IPv6 T1ST with source",
			config: MUPRouteConfig{
				RouteType: "mup-t1st",
				IsIPv6:    true,
				Prefix:    "2001:db8:1:1::2/128",
				RD:        "100:100",
				TEID:      "12345",
				QFI:       9,
				Endpoint:  "2001::1",
				Source:    "2002::2",
			},
			// source: len=128 (0x80), addr=2002::2
			wantHex: "8020020000000000000000000000000002",
		},
		{
			name: "T1ST without source (optional)",
			config: MUPRouteConfig{
				RouteType: "mup-t1st",
				IsIPv6:    false,
				Prefix:    "192.168.0.2/32",
				RD:        "100:100",
				TEID:      "12345",
				QFI:       9,
				Endpoint:  "10.0.0.1",
				Source:    "", // no source
			},
			// endpoint should be last: len=32 (0x20), addr=10.0.0.1 (0x0a000001)
			// no source bytes after
			wantHex: "200a000001",
		},
		{
			name: "T1ST with invalid source fails loudly",
			config: MUPRouteConfig{
				RouteType: "mup-t1st",
				IsIPv6:    false,
				Prefix:    "192.168.0.2/32",
				RD:        "100:100",
				TEID:      "12345",
				QFI:       9,
				Endpoint:  "10.0.0.1",
				Source:    "not-an-ip",
			},
			wantErr:     true,
			wantErrText: "invalid T1ST source",
		},
		{
			name: "T1ST with invalid endpoint fails loudly",
			config: MUPRouteConfig{
				RouteType: "mup-t1st",
				IsIPv6:    false,
				Prefix:    "192.168.0.2/32",
				RD:        "100:100",
				TEID:      "12345",
				QFI:       9,
				Endpoint:  "bad-endpoint",
				Source:    "10.0.1.1",
			},
			wantErr:     true,
			wantErrText: "invalid T1ST endpoint",
		},
		{
			name: "T1ST with invalid prefix fails loudly",
			config: MUPRouteConfig{
				RouteType: "mup-t1st",
				IsIPv6:    false,
				Prefix:    "not-a-prefix",
				RD:        "100:100",
			},
			wantErr:     true,
			wantErrText: "invalid T1ST prefix",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nlri, err := buildMUPNLRI(tt.config)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErrText)
				return
			}

			require.NoError(t, err)
			require.NotEmpty(t, nlri)

			// Check that the expected hex is present in the NLRI
			nlriHex := hex.EncodeToString(nlri)
			assert.Contains(t, nlriHex, tt.wantHex,
				"NLRI should contain %s, got %s", tt.wantHex, nlriHex)
		})
	}
}
