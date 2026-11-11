// Design: docs/architecture/api/process-protocol.md -- refuse auto-load without a forwarding provider.

package server

import (
	"strings"
	"testing"

	plugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// TestConfigPathAutoLoadRefusesMissingDataPlaneWriter exercises startup
// selection, so a missing writer cannot be hidden by returning the producer.
func TestConfigPathAutoLoadRefusesMissingDataPlaneWriter(t *testing.T) {
	if _, ok := registry.PluginForDataPlane("p4-runtime"); ok {
		t.Fatal("fixture data plane unexpectedly has a writer")
	}
	s := &Server{
		config: &ServerConfig{
			ConfigNeedHolds: holdsForEmptyConfig,
			ConfiguredPaths: []string{"static"},
			DataPlane:       "p4-runtime",
		},
		registry:      plugin.NewPluginRegistry(),
		loadedPlugins: make(map[string]bool),
	}
	got, err := s.getConfigPathPlugins()
	if err == nil {
		t.Fatalf("missing writer accepted: %+v", got)
	}
	if !strings.Contains(err.Error(), "p4-runtime") {
		t.Fatalf("refusal does not name the selected data plane: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("refusal retained plugins to start: %+v", got)
	}
}

// TestReloadAutoLoadRefusesMissingDataPlaneWriter proves the reload selector
// propagates the same refusal before it returns any plugin to start.
func TestReloadAutoLoadRefusesMissingDataPlaneWriter(t *testing.T) {
	if _, ok := registry.PluginForDataPlane("p4-runtime"); ok {
		t.Fatal("fixture data plane unexpectedly has a writer")
	}
	s := &Server{
		config:        &ServerConfig{},
		registry:      plugin.NewPluginRegistry(),
		loadedPlugins: make(map[string]bool),
	}
	tree := map[string]any{
		"interface": map[string]any{"backend": "p4-runtime"},
		"static":    map[string]any{},
	}
	got, err := s.selectAutoLoad(tree, []string{"static"})
	if err == nil {
		t.Fatalf("reload accepted a producer without its writer: %+v", got)
	}
	if !strings.Contains(err.Error(), "p4-runtime") {
		t.Fatalf("refusal does not name the selected data plane: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("reload refusal retained plugins to start: %+v", got)
	}
}
