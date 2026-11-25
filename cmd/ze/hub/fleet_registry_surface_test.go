//go:build ze_ssh && ze_web

// Design: docs/architecture/api/commands.md -- daemon command discovery across operator surfaces
// Related: surface_parity_test.go -- surfaceArms reads each surface's own command catalog

package hub

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// TestFleetRegistryCommandsOnEverySurface checks that each daemon surface
// offers both registry commands, then dispatches their operator spelling.
// With no managed server the refusal from each handler names its subject.
//
// VALIDATES: fleet-1 AC-34, including the argument of the detail command.
// PREVENTS: a command listed in the schema that never reaches its handler.
func TestFleetRegistryCommandsOnEverySurface(t *testing.T) {
	server, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(server.Stop)
	arms := surfaceArms(t, server)
	for _, path := range []string{"show fleet devices", "show fleet device name"} {
		for _, surface := range []command.Surface{
			command.SurfaceCLI, command.SurfaceSSH, command.SurfaceMCP, command.SurfaceREST,
			command.SurfaceGRPC, command.SurfaceWebAdmin, command.SurfaceWebCompletion,
		} {
			arm := arms[surface]
			if !arm.listed(path) {
				t.Errorf("%s does not list %q", surface, path)
			}
			if !arm.served(path) {
				t.Errorf("%s cannot serve %q", surface, path)
			}
		}
	}
	for _, line := range []string{"show fleet devices", "show fleet device name edge-01"} {
		response, err := server.Dispatcher().Dispatch(&pluginserver.CommandContext{Server: server}, line)
		if err != nil {
			t.Fatalf("dispatch %q: %v", line, err)
		}
		if response == nil {
			t.Fatalf("dispatch %q returned no answer", line)
		}
		if response.Status != plugin.StatusError || !strings.Contains(response.Error, line+": unable to read the device registry") {
			t.Errorf("dispatch %q without a serving hub: got %+v", line, response)
		}
	}
}
