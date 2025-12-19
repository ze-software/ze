package config

import (
	"os"
	"path/filepath"
	"testing"

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

neighbor 192.0.2.1 {
    peer-as 65001;
    hold-time 90;
}

neighbor 192.0.2.2 {
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

neighbor 192.0.2.1 {
    peer-as 65001;
}
`

	r, err := LoadReactor(input)
	require.NoError(t, err)

	peers := r.Peers()
	require.Len(t, peers, 1)

	// Neighbor should inherit local-as from global
	n := peers[0].Neighbor()
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

neighbor 192.0.2.1 {
    peer-as 65001;
    passive true;
}
`

	r, err := LoadReactor(input)
	require.NoError(t, err)

	peers := r.Peers()
	require.Len(t, peers, 1)

	n := peers[0].Neighbor()
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

neighbor 192.0.2.1 {
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
neighbor 192.0.2.1 {
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
			data, err := os.ReadFile(file)
			require.NoError(t, err)

			_, err = p.Parse(string(data))
			require.NoError(t, err, "failed to parse %s", file)
		})
	}
}
