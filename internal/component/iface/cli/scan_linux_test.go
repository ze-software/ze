// Design: docs/features/interfaces.md -- interface discovery CLI
//
// VALIDATES: the scan's rows reach the pipe layer with no daemon running, so
//   `| json` and `| yaml` are two renderings of ONE payload rather than the
//   two hand-written renderers the deleted `--json` and `--yaml` flags were
//   (ai/rules/cli.md).
// PREVENTS: the local-data handler answering nothing, or answering a shape the
//   row operators cannot read, on the platform where a scan can answer at all.
//
// Linux only, and not for want of trying elsewhere: the scan reads link types
// through the netlink backend, and the backend of every other platform refuses
// ListInterfaces (internal/plugins/iface/netlink/backend_other.go, stubBackend).

package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
)

// The loopback interface is the one row every Linux host has, container and
// namespace included, so it is what this asserts on.
const loopbackName = "lo"

func TestTheScanAnswersRowsThroughThePipeLayer(t *testing.T) {
	rendered, code, served := command.ServeLocal(cmdPathShowInterfaceScan+" | json compact", "")
	if !served {
		t.Fatalf("%q is not served in this process", cmdPathShowInterfaceScan)
	}
	if code != 0 {
		t.Fatalf("json code = %d, want 0: %s", code, rendered)
	}

	var rows []map[string]any
	if err := json.Unmarshal([]byte(rendered), &rows); err != nil {
		t.Fatalf("the answer is not a JSON row list: %v: %s", err, rendered)
	}
	if len(rows) == 0 {
		t.Fatal("the scan answered no rows on a host that has at least a loopback")
	}
	found := false
	for _, row := range rows {
		if row["name"] == loopbackName {
			found = true
			if row["type"] != "loopback" {
				t.Errorf("loopback row = %#v, want type loopback", row)
			}
		}
	}
	if !found {
		t.Errorf("the answer has no %q row: %s", loopbackName, rendered)
	}
}

func TestTheYAMLRenderingComesFromTheSamePayload(t *testing.T) {
	jsonOut, code, served := command.ServeLocal(cmdPathShowInterfaceScan+" | json compact", "")
	if !served || code != 0 {
		t.Fatalf("json served=%t code=%d: %s", served, code, jsonOut)
	}
	yamlOut, code, served := command.ServeLocal(cmdPathShowInterfaceScan+" | yaml", "")
	if !served || code != 0 {
		t.Fatalf("yaml served=%t code=%d: %s", served, code, yamlOut)
	}

	var rows []map[string]any
	if err := json.Unmarshal([]byte(jsonOut), &rows); err != nil {
		t.Fatalf("the json answer is not a row list: %v: %s", err, jsonOut)
	}
	// Every name the json rendering carries is in the yaml rendering: two
	// renderings, one answer.
	for _, row := range rows {
		name, isText := row["name"].(string)
		if !isText {
			t.Fatalf("row has no name: %#v", row)
		}
		if !strings.Contains(yamlOut, "name: "+name) {
			t.Errorf("the yaml rendering lost the %q row: %s", name, yamlOut)
		}
	}
}
