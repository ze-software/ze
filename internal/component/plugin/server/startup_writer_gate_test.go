// Design: docs/architecture/plugin/feature-gates.md -- a compiled-out forwarding provider names its tags.

package server

import (
	"errors"
	"net"
	"slices"
	"strings"
	"testing"

	plugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// TestMissingDataPlaneWriterNamesDeclaredBuildGate proves an absent VPP writer
// retains its declared plugin identity and gate through startup selection.
// The live registry deliberately holds only the requesting producer.
func TestMissingDataPlaneWriterNamesDeclaredBuildGate(t *testing.T) {
	held := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(held) })
	const producer = "writer-gate-test"
	if err := registry.Register(registry.Registration{
		Name:           producer,
		Description:    "test route producer",
		ConfigRoots:    []string{producer},
		NeedsDataPlane: true,
		RunEngine:      func(net.Conn) int { return 0 },
		CLIHandler:     func([]string) int { return 0 },
	}); err != nil {
		t.Fatal(err)
	}
	s := &Server{
		config: &ServerConfig{
			ConfigNeedHolds: holdsForEmptyConfig,
			ConfiguredPaths: []string{producer},
			DataPlane:       "vpp",
		},
		registry:      plugin.NewPluginRegistry(),
		loadedPlugins: make(map[string]bool),
	}
	got, err := s.getConfigPathPlugins()
	var missing *registry.MissingDependencyError
	if !errors.As(err, &missing) {
		t.Fatalf("missing writer did not preserve dependency provenance: %v", err)
	}
	if missing.Plugin != producer {
		t.Fatalf("refusal names the wrong requester: %q", missing.Plugin)
	}
	if missing.Dependency != "fib-vpp" {
		t.Fatalf("refusal names the wrong writer: %q", missing.Dependency)
	}
	if !slices.Contains(missing.Tags, "ze_vpp") {
		t.Fatalf("refusal lost writer build gate: %v", missing.Tags)
	}
	if !strings.Contains(err.Error(), "vpp") {
		t.Fatalf("refusal lost the selected data plane: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("refusal returned plugins to start: %+v", got)
	}
}
