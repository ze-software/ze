package ntp

import (
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// TestConfigRootsAutoLoadOnlyForNTPBlock checks which configs start ntp.
//
// Startup and reload auto-load match the registered ConfigRoots against every
// container path of the config (config.CollectContainerPaths). The test builds
// each config tree, collects its paths with that producer, and asks whether
// any path is an ntp root.
//
// VALIDATES: `environment ntp` starts ntp.
// PREVENTS: an environment block ntp does not own (api-server) starting ntp,
// whose startup then fails a reload compensation that has no spawner.
func TestConfigRootsAutoLoadOnlyForNTPBlock(t *testing.T) {
	reg := registry.Lookup("ntp")
	if reg == nil {
		t.Fatal("ntp is not registered")
	}

	environment := func(child string) *config.Tree {
		top := config.NewTree()
		container := config.NewTree()
		container.SetContainer(child, config.NewTree())
		top.SetContainer(configRootEnvironment, container)
		return top
	}

	cases := []struct {
		name  string
		tree  *config.Tree
		start bool
	}{
		{"environment ntp", environment("ntp"), true},
		{"environment api-server", environment("api-server"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			paths := config.CollectContainerPaths(tc.tree)
			start := false
			for _, root := range reg.ConfigRoots {
				if slices.Contains(paths, root) {
					start = true
				}
			}
			if start != tc.start {
				t.Errorf("paths %v against roots %v: auto-load %v, want %v", paths, reg.ConfigRoots, start, tc.start)
			}
		})
	}
}
