// Design: docs/guide/command-reference.md — show plugins
// Related: register.go — the registration these tests exercise
//
// register_test.go proves four things about `show plugins`: it answers, it
// answers with DATA rather than with text a renderer already formatted, it
// refuses by name the operators its answer's shape cannot carry, and each row
// carries the setup outcome the plugin's own init() recorded rather than a
// list this package keeps.

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
	rows, ok := answer[keyPlugins].([]pluginRow)
	if !ok {
		t.Fatalf("answer key %q holds %T, want []pluginRow", keyPlugins, answer[keyPlugins])
	}
	if len(rows) != len(registry.All()) {
		t.Errorf("answer has %d rows, registry has %d plugins", len(rows), len(registry.All()))
	}

	var probe *pluginRow
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
	if strings.Join(orders[0], ",") != "name,description,outcome,families,rfcs,capabilities,reason" {
		t.Errorf("declared columns = %v", orders[0])
	}
}

// recordingPluginName is the plugin the setup-outcome tests record against. A
// name no real plugin uses keeps the assertion readable when it fails.
const recordingPluginName = "show-plugins-setup-probe"

// silentPluginName is a plugin that registers and records nothing, which is the
// case AC-4 exists for.
const silentPluginName = "show-plugins-setup-silent"

// isolateRegistry empties the plugin registry and the setup record for one
// test, and puts both back. Every test in this package shares one binary and
// one process-wide registry.
func isolateRegistry(t *testing.T) {
	t.Helper()
	saved := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(saved) })
	registry.Reset()
}

// registerPlugin puts one plugin in the registry under the given name.
func registerPlugin(t *testing.T, name string) {
	t.Helper()
	err := registry.Register(registry.Registration{
		Name:        name,
		Description: "Probe plugin for the show plugins tests",
		RunEngine:   func(net.Conn) int { return 0 },
		CLIHandler:  func([]string) int { return 0 },
	})
	if err != nil {
		t.Fatalf("register %q: %v", name, err)
	}
}

// TestShowPluginsCarriesTheRecordedSetupOutcome is the wiring test for the
// join: what a plugin records from its init() is what the row carries.
//
// VALIDATES: dataPlugins takes the outcome and the reason from
// registry.SetupResults, intact, beside the plugin's own registration fields.
//
// PREVENTS: a command that answers a list this package keeps for itself, which
// would stay green while the registry the daemon reads says something else.
func TestShowPluginsCarriesTheRecordedSetupOutcome(t *testing.T) {
	isolateRegistry(t)
	registerPlugin(t, recordingPluginName)
	registry.RecordSetup(recordingPluginName, registry.SetupFailedSoft, "RLIMIT_MEMLOCK is too small")

	payload, code := dataPlugins(nil)
	if code != 0 {
		t.Fatalf("show plugins exit code = %d, want 0", code)
	}
	answer, ok := payload.(Map)
	if !ok {
		t.Fatalf("show plugins answered %T, want plugin.Map", payload)
	}
	rows, ok := answer[keyPlugins].([]pluginRow)
	if !ok {
		t.Fatalf("answer key %q holds %T, want []pluginRow", keyPlugins, answer[keyPlugins])
	}
	if len(rows) != 1 {
		t.Fatalf("answer has %d rows, the registry holds one plugin: %+v", len(rows), rows)
	}
	if rows[0].Name != recordingPluginName {
		t.Errorf("row names %q, want %q", rows[0].Name, recordingPluginName)
	}
	if rows[0].Description == "" {
		t.Error("the row lost the registration's description")
	}
	if rows[0].Outcome != registry.SetupFailedSoft {
		t.Errorf("row outcome = %v, want soft-failure", rows[0].Outcome)
	}
	if rows[0].Reason != "RLIMIT_MEMLOCK is too small" {
		t.Errorf("row reason = %q, want the recorded text", rows[0].Reason)
	}
}

// TestShowPluginsNamesAPluginThatRecordedNothing proves AC-4 at the command.
//
// VALIDATES: a registered plugin that recorded nothing is listed with the
// unknown outcome and no reason.
//
// PREVENTS: the failure the setup record exists to remove. An absent row reads
// as "not built in", so the plugin that owes a record is the one nobody sees.
func TestShowPluginsNamesAPluginThatRecordedNothing(t *testing.T) {
	isolateRegistry(t)
	registerPlugin(t, silentPluginName)

	answer, code, served := command.ServeLocal(commandShowPlugins+" | json", "")
	if !served {
		t.Fatal("show plugins was not served in this process")
	}
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (answer: %q)", code, answer)
	}

	var rows []struct {
		Name    string `json:"name"`
		Outcome string `json:"outcome"`
		Reason  string `json:"reason"`
	}
	if err := json.Unmarshal([]byte(answer), &rows); err != nil {
		t.Fatalf("| json answered something no JSON decoder takes: %v (answer: %q)", err, answer)
	}
	if len(rows) != 1 {
		t.Fatalf("| json answered %d rows, the registry holds one plugin: %q", len(rows), answer)
	}
	if rows[0].Name != silentPluginName {
		t.Errorf("row names %q, want %q", rows[0].Name, silentPluginName)
	}
	if rows[0].Outcome != "unknown" {
		t.Errorf("a plugin that recorded nothing has outcome %q, want unknown", rows[0].Outcome)
	}
	if rows[0].Reason != "" {
		t.Errorf("a plugin that recorded nothing carries reason %q, want none", rows[0].Reason)
	}
}

// TestShowPluginsRendersTheOutcomeInEveryFormat proves AC-7.
//
// VALIDATES: `| json`, `| yaml` and `| table` each render the same rows,
// outcome and reason included.
//
// PREVENTS: a handler that returns finished text, which one renderer would
// hand back unchanged while the other two produce something a parser cannot
// read.
func TestShowPluginsRendersTheOutcomeInEveryFormat(t *testing.T) {
	isolateRegistry(t)
	registerPlugin(t, recordingPluginName)
	registry.RecordSetup(recordingPluginName, registry.SetupFailedSoft, "RLIMIT_MEMLOCK is too small")

	for _, format := range []string{"json", "yaml", "table"} {
		t.Run(format, func(t *testing.T) {
			answer, code, served := command.ServeLocal(commandShowPlugins+" | "+format, "")
			if !served {
				t.Fatalf("show plugins | %s was not served in this process", format)
			}
			if code != 0 {
				t.Fatalf("exit code = %d, want 0 (answer: %q)", code, answer)
			}
			for _, want := range []string{recordingPluginName, "soft-failure", "RLIMIT_MEMLOCK is too small"} {
				if !strings.Contains(answer, want) {
					t.Errorf("| %s lost %q: %s", format, want, answer)
				}
			}
		})
	}
}

// TestShowPluginsKeepsAPluginThatRecordedAndDidNotRegister proves the loudest
// case survives the join.
//
// VALIDATES: a name that recorded an outcome and never completed its Register
// call still gets a row, and the row says why its registration fields are
// absent.
//
// PREVENTS: the join dropping the one plugin whose setup failed hard enough to
// take its own registration with it, which is the absence-reads-as-fine defect
// the whole setup record exists to remove.
func TestShowPluginsKeepsAPluginThatRecordedAndDidNotRegister(t *testing.T) {
	isolateRegistry(t)
	registry.RecordSetup("never-registered", registry.SetupFailedHard, "the kernel does not support it")

	rows := pluginRows()
	if len(rows) != 1 {
		t.Fatalf("pluginRows = %+v, want the one recorded name", rows)
	}
	if rows[0].Name != "never-registered" {
		t.Fatalf("row names %q, want the recorded name", rows[0].Name)
	}
	if rows[0].Description != descriptionUnregistered {
		t.Errorf("row description = %q, want %q", rows[0].Description, descriptionUnregistered)
	}
	if rows[0].Outcome != registry.SetupFailedHard || rows[0].Reason != "the kernel does not support it" {
		t.Errorf("the row lost the recorded outcome: %+v", rows[0])
	}
}

// TestShowPluginsRowsAgreeWithInternalPluginInfo pins the ONE divergence the
// join is allowed to have.
//
// VALIDATES: every row either matches an InternalPluginInfo entry of the same
// name, field for field, or is a name that recorded without registering, in
// which case it says so; and every entry InternalPluginInfo returns has a row.
//
// PREVENTS: a future divergence between the two registry walks going unnoticed.
// registry.All and registry.SetupResults read the same map today, so a reader
// sees rows with the outcome filled in; a walk that stopped agreeing would
// render blank outcome cells, or silently drop plugins, and no other assertion
// in this package would go red.
func TestShowPluginsRowsAgreeWithInternalPluginInfo(t *testing.T) {
	t.Run("live registry", func(t *testing.T) {
		registerProbePlugin(t)
		assertRowsAgreeWithPluginInfo(t)
	})

	t.Run("with a name that recorded and did not register", func(t *testing.T) {
		isolateRegistry(t)
		registerPlugin(t, recordingPluginName)
		registry.RecordSetup(recordingPluginName, registry.SetupSucceeded, "")
		registry.RecordSetup("never-registered", registry.SetupFailedHard, "the kernel does not support it")
		assertRowsAgreeWithPluginInfo(t)
	})
}

// assertRowsAgreeWithPluginInfo checks the row set against InternalPluginInfo,
// allowing only the recorded-but-unregistered row.
func assertRowsAgreeWithPluginInfo(t *testing.T) {
	t.Helper()

	described := make(map[string]PluginInfo)
	for _, info := range InternalPluginInfo() {
		described[info.Name] = info
	}

	seen := make(map[string]bool, len(described))
	for _, row := range pluginRows() {
		seen[row.Name] = true
		info, registered := described[row.Name]
		if !registered {
			if row.Description != descriptionUnregistered {
				t.Errorf("row %q matches no registration and does not say so: %q", row.Name, row.Description)
			}
			if row.Outcome == registry.SetupUnknown {
				t.Errorf("row %q matches no registration and recorded nothing, so it has no reason to exist", row.Name)
			}
			continue
		}
		if row.Description != info.Description {
			t.Errorf("row %q description = %q, registration says %q", row.Name, row.Description, info.Description)
		}
		if strings.Join(row.Families, ",") != strings.Join(info.Families, ",") {
			t.Errorf("row %q families = %v, registration says %v", row.Name, row.Families, info.Families)
		}
		if strings.Join(row.RFCs, ",") != strings.Join(info.RFCs, ",") {
			t.Errorf("row %q RFCs = %v, registration says %v", row.Name, row.RFCs, info.RFCs)
		}
	}

	for name := range described {
		if !seen[name] {
			t.Errorf("the registered plugin %q has no row in show plugins", name)
		}
	}
}
