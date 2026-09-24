package bmp

import (
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// TestConfigRootsAutoLoadOnlyForBMPBlocks checks which configs start bgp-bmp.
//
// Startup auto-load matches the registered ConfigRoots against every container
// path of the config (config.CollectContainerPaths), and reload matches the
// same way. The test builds each config tree, collects its paths with that
// producer, and asks whether any path is a bgp-bmp root.
//
// VALIDATES: `environment bmp` and `bgp` start BMP; `system` alone and an
// environment block BMP does not own (api-server) do not.
// PREVENTS: an unrelated environment or system block starting bgp-bmp, whose
// startup then fails the whole reload.
func TestConfigRootsAutoLoadOnlyForBMPBlocks(t *testing.T) {
	reg := registry.Lookup("bgp-bmp")
	if reg == nil {
		t.Fatal("bgp-bmp is not registered")
	}

	tree := func(root, child string) *config.Tree {
		top := config.NewTree()
		container := config.NewTree()
		if child != "" {
			container.SetContainer(child, config.NewTree())
		}
		top.SetContainer(root, container)
		return top
	}

	cases := []struct {
		name  string
		tree  *config.Tree
		start bool
	}{
		{"environment bmp", tree(configRootEnvironment, protocolBMP), true},
		{"bgp", tree(configRootBGP, ""), true},
		{"environment api-server", tree(configRootEnvironment, "api-server"), false},
		{"system", tree(configRootSystem, "host"), false},
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
