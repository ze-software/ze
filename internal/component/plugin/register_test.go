// Design: docs/guide/command-reference.md — show plugins
// Related: register.go — the registration these tests exercise
//
// register_test.go proves three things about `show plugins`: it answers, it
// answers with DATA rather than with text a renderer already formatted, and it
// refuses by name the operators its answer's shape cannot carry.

package plugin

import (
	"encoding/json"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// probePluginName is the plugin these tests put in the registry so each one
// reads an answer whose content it knows. Which real plugins a test binary
// carries depends on its build tags, and an assertion over an empty set proves
// nothing.
const probePluginName = "show-plugins-probe"

// commandShowPlugins is the path an operator types, as register.go publishes it.
const commandShowPlugins = "show plugins"

// probeOnce registers the probe exactly once. The plugin registry is
// process-wide and refuses a duplicate name, and every test in this package
// shares one binary.
var probeOnce sync.Once

// registerProbePlugin puts one plugin with every optional field filled into the
// registry. The registry has no removal, which is what the other registration
// tests in this package already rely on.
func registerProbePlugin(t *testing.T) {
	t.Helper()
	var err error
	probeOnce.Do(func() {
		err = registry.Register(registry.Registration{
			Name:            probePluginName,
			Description:     "Probe plugin for the show plugins command tests",
			RunEngine:       func(net.Conn) int { return 0 },
			CLIHandler:      func([]string) int { return 0 },
			RFCs:            []string{"9999"},
			Families:        []string{"ipv4/probe"},
			CapabilityCodes: []uint8{200},
		})
	})
	if err != nil {
		t.Fatalf("register probe plugin: %v", err)
	}
}

// TestShowPluginsAnswersEveryRegisteredPlugin proves the handler answers the
// registry rather than a fixed list, and that it carries the optional metadata a
// plugin declares.
func TestShowPluginsAnswersEveryRegisteredPlugin(t *testing.T) {
	registerProbePlugin(t)

	payload, code := dataPlugins(nil)
	if code != 0 {
		t.Fatalf("show plugins exit code = %d, want 0", code)
	}

	// The payload MUST satisfy ResponseData, which is what keeps text a
	// renderer already formatted out of a command's answer (ai/rules/cli.md).
	if _, ok := payload.(ResponseData); !ok {
		t.Fatalf("show plugins answered %T, which is not ResponseData", payload)
	}

	answer, ok := payload.(Map)
	if !ok {
		t.Fatalf("show plugins answered %T, want plugin.Map", payload)
	}
	rows, ok := answer[keyPlugins].([]PluginInfo)
	if !ok {
		t.Fatalf("answer key %q holds %T, want []PluginInfo", keyPlugins, answer[keyPlugins])
	}
	if len(rows) != len(registry.All()) {
		t.Errorf("answer has %d rows, registry has %d plugins", len(rows), len(registry.All()))
	}

	var probe *PluginInfo
	for i := range rows {
		if rows[i].Name == probePluginName {
			probe = &rows[i]
		}
	}
	if probe == nil {
		t.Fatalf("the registered probe plugin %q is missing from the answer", probePluginName)
	}
	if probe.Description == "" {
		t.Error("probe row carries no description")
	}
	if strings.Join(probe.Families, ",") != "ipv4/probe" {
		t.Errorf("probe families = %v, want [ipv4/probe]", probe.Families)
	}
	if strings.Join(probe.RFCs, ",") != "9999" {
		t.Errorf("probe RFCs = %v, want [9999]", probe.RFCs)
	}
	if len(probe.Capabilities) != 1 || probe.Capabilities[0] != 200 {
		t.Errorf("probe capabilities = %v, want [200]", probe.Capabilities)
	}
}

// TestShowPluginsRendersAsJSON proves the answer reaches the pipe layer, which
// is what a structured payload buys: `| json` renders the same payload
// `| yaml` and `| table` render.
func TestShowPluginsRendersAsJSON(t *testing.T) {
	registerProbePlugin(t)

	answer, code, served := command.ServeLocal(commandShowPlugins+" | json", "")
	if !served {
		t.Fatal("show plugins was not served in this process")
	}
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (answer: %q)", code, answer)
	}

	var rows []struct {
		Name        string   `json:"name"`
		Description string   `json:"description"`
		Families    []string `json:"families"`
	}
	if err := json.Unmarshal([]byte(answer), &rows); err != nil {
		t.Fatalf("| json answered something no JSON decoder takes: %v (answer: %q)", err, answer)
	}
	if len(rows) == 0 {
		t.Fatal("| json answered zero rows while the registry holds the probe plugin")
	}
	for _, row := range rows {
		if row.Name != probePluginName {
			continue
		}
		if row.Description == "" || strings.Join(row.Families, ",") != "ipv4/probe" {
			t.Errorf("probe row lost its fields through | json: %+v", row)
		}
		return
	}
	t.Errorf("the probe plugin is missing from the | json answer: %q", answer)
}

// TestShowPluginsCountsItsRows proves the declared shape admits the row
// operators, so the count is over plugins rather than over the keys of an
// envelope.
func TestShowPluginsCountsItsRows(t *testing.T) {
	registerProbePlugin(t)

	answer, code, served := command.ServeLocal(commandShowPlugins+" | count", "")
	if !served || code != 0 {
		t.Fatalf("show plugins | count: served=%v code=%d answer=%q", served, code, answer)
	}
	counted, err := strconv.Atoi(strings.TrimSpace(answer))
	if err != nil {
		t.Fatalf("| count answered %q, which is not a number: %v", answer, err)
	}
	if counted != len(registry.All()) {
		t.Errorf("| count = %d, registry holds %d plugins", counted, len(registry.All()))
	}
}

// TestShowPluginsRefusesAddressOperatorsByName proves the answer's shape is
// declared: no field holds an IP address, so `| resolve` is refused before the
// command runs rather than answered with something plausible.
func TestShowPluginsRefusesAddressOperatorsByName(t *testing.T) {
	answer, code, served := command.ServeLocal(commandShowPlugins+" | resolve", "")
	if !served {
		t.Fatal("show plugins was not served in this process")
	}
	if code == 0 {
		t.Fatalf("| resolve was accepted over an answer holding no address (first line: %q)",
			strings.SplitN(answer, "\n", 2)[0])
	}
	if !strings.Contains(answer, "resolve") {
		t.Errorf("refusal does not name the operator: %q", answer)
	}
}

// TestShowPluginsDeclaresItsShapeAndColumns proves the published catalog can say
// what the command supports before the command runs.
func TestShowPluginsDeclaresItsShapeAndColumns(t *testing.T) {
	shape, declared := command.ShapeForCommand(commandShowPlugins)
	if !declared {
		t.Fatal("show plugins declares no answer shape")
	}
	if shape != command.ShapeTab {
		t.Errorf("declared shape = %v, want tab", shape)
	}
	orders := command.ColumnsForCommand(commandShowPlugins)
	if len(orders) != 1 {
		t.Fatalf("show plugins declares %d column orders, want 1", len(orders))
	}
	if strings.Join(orders[0], ",") != "name,description,families,rfcs,capabilities" {
		t.Errorf("declared columns = %v", orders[0])
	}
}
