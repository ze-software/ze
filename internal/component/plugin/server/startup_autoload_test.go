package server

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
)

// TestGetUnclaimedEventTypePlugins verifies auto-loading plugins for custom event types.
//
// VALIDATES: Plugins producing custom event types are auto-loaded when not explicitly configured.
// PREVENTS: Custom event types silently ignored because producing plugin was not loaded.
func TestGetUnclaimedEventTypePlugins(t *testing.T) {
	tests := []struct {
		name              string
		customEvents      []string
		configuredPlugins []plugin.PluginConfig
		wantPluginNames   []string
		wantNil           bool
	}{
		{
			name:         "no_custom_events",
			customEvents: nil,
			wantNil:      true,
		},
		{
			name:         "unknown_event_type_returns_nil",
			customEvents: []string{"nonexistent-event"},
			wantNil:      true,
		},
		{
			name:         "known_event_type_auto_loads_plugin_and_deps",
			customEvents: []string{"update-rpki"},
			// bgp-rpki-decorator produces update-rpki, depends on bgp-rpki,
			// which depends on bgp-adj-rib-in. ResolveDependencies returns
			// all transitive dependencies in dependency-first order.
			wantPluginNames: []string{"bgp-rpki-decorator", "bgp", "bgp-rpki", "bgp-adj-rib-in"},
		},
		{
			name:         "already_configured_plugin_skipped",
			customEvents: []string{"update-rpki"},
			configuredPlugins: []plugin.PluginConfig{
				{Name: "bgp-rpki-decorator"},
			},
			// The producing plugin is already configured, so nothing to auto-load.
			// But bgp-rpki (dependency) is not configured -- however the producing
			// plugin itself is skipped, so no dependency resolution happens.
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Server{
				config: &ServerConfig{
					ConfiguredCustomEvents: tt.customEvents,
					Plugins:                tt.configuredPlugins,
				},
				registry: plugin.NewPluginRegistry(),
			}

			got := s.getUnclaimedEventTypePlugins()

			if tt.wantNil {
				assert.Nil(t, got)
				return
			}

			require.NotNil(t, got)

			var names []string
			for _, p := range got {
				names = append(names, p.Name)
				assert.Equal(t, "json", p.Encoder, "auto-loaded plugin should use json encoder")
				assert.True(t, p.Internal, "auto-loaded plugin should be internal")
			}

			assert.Equal(t, tt.wantPluginNames, names)
		})
	}
}

// TestBGPPluginAutoLoads verifies that ConfigRoots "bgp" triggers BGP plugin
// auto-loading when the config contains a bgp { } section.
//
// VALIDATES: AC-1 -- Config with bgp { } auto-loads BGP plugin via ConfigRoots.
// PREVENTS: BGP plugin not loaded when bgp section is present in config.
func TestBGPPluginAutoLoads(t *testing.T) {
	s := &Server{
		config: &ServerConfig{
			ConfiguredPaths: []string{"bgp"},
		},
		registry:      plugin.NewPluginRegistry(),
		loadedPlugins: make(map[string]bool),
	}

	got := s.getConfigPathPlugins()
	require.NotNil(t, got, "should auto-load plugins for bgp config path")

	var names []string
	for _, p := range got {
		names = append(names, p.Name)
	}
	assert.Contains(t, names, "bgp", "bgp plugin should be in the auto-load list")

	for _, p := range got {
		assert.True(t, p.Internal, "plugin %s should be internal", p.Name)
		assert.Equal(t, "json", p.Encoder, "plugin %s should use json encoder", p.Name)
	}
}

// TestEngineStartsWithoutBGP verifies that no BGP plugins are auto-loaded when
// the config has no bgp section.
//
// VALIDATES: AC-2/AC-5 -- Config without bgp section does not load BGP.
// PREVENTS: BGP plugin loading unconditionally regardless of config.
func TestEngineStartsWithoutBGP(t *testing.T) {
	tests := []struct {
		name  string
		paths []string
	}{
		{name: "empty_paths", paths: nil},
		{name: "interface_only", paths: []string{"interface"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Server{
				config: &ServerConfig{
					ConfiguredPaths: tt.paths,
				},
				registry:      plugin.NewPluginRegistry(),
				loadedPlugins: make(map[string]bool),
			}

			got := s.getConfigPathPlugins()

			for _, p := range got {
				assert.NotEqual(t, "bgp", p.Name,
					"bgp plugin should not auto-load without bgp config path")
			}
		})
	}
}
