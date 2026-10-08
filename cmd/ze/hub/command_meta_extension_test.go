package hub

import (
	"errors"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/api"
	"github.com/ze-software/ze/internal/component/config/yang"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// probeHeplModule names `ze:hepl`, an extension ze-extensions does not
// declare, so yang.DefaultLoader refuses the module set while it is
// registered.
const probeHeplModule = `module ze-probe {
    namespace "urn:ze:probe";
    prefix probe;
    import ze-extensions { prefix ze; }
    leaf probe {
        type string;
        ze:hepl "Probe leaf.";
    }
}`

// TestCommandMetaSourceSurfacesLoaderError: when yang.DefaultLoader refuses
// the module set, the command metadata source returns that error on every
// call, and the API engine reports it from ListCommands, DescribeCommand and
// Execute, instead of answering with metadata built from no schema. Method:
// register a probe module whose `ze:hepl` names no declared extension, for
// this test only, after the server is built. Each source is built fresh, so
// its cache holds only this test's answer.
//
// VALIDATES: API command metadata reports the loader error with its cause.
// PREVENTS: an API or MCP command list with no YANG parameters, task support
// or UI resource, and no word of why.
func TestCommandMetaSourceSurfacesLoaderError(t *testing.T) {
	// The server is built before the probe is registered: NewServer loads the
	// schema itself, and the question here is what the metadata source answers
	// once the schema is refused.
	server, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(yang.RegisterModuleForTest("ze-probe.yang", probeHeplModule))

	source := commandMetaSource(server)
	for attempt := range 2 {
		metas, err := source()
		if !errors.Is(err, yang.ErrUndeclaredExtension) {
			t.Fatalf("attempt %d: source answered (%d commands, %v), want ErrUndeclaredExtension", attempt, len(metas), err)
		}
		if !strings.Contains(err.Error(), "ze:hepl") {
			t.Errorf("attempt %d: error %q does not name ze:hepl", attempt, err)
		}
	}

	engine := buildAPIEngine(server)
	if _, err := engine.ListCommands(&api.ListCommandsRequest{}); !errors.Is(err, yang.ErrUndeclaredExtension) {
		t.Errorf("ListCommands error = %v, want ErrUndeclaredExtension", err)
	}
	if _, err := engine.DescribeCommand(&api.DescribeCommandRequest{Path: "help"}); !errors.Is(err, yang.ErrUndeclaredExtension) {
		t.Errorf("DescribeCommand error = %v, want ErrUndeclaredExtension", err)
	}
	if _, err := engine.Execute(t.Context(), &api.ExecuteRequest{Command: "help"}); !errors.Is(err, yang.ErrUndeclaredExtension) {
		t.Errorf("Execute error = %v, want ErrUndeclaredExtension", err)
	}
}
