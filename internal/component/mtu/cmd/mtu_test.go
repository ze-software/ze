// Design: docs/architecture/diagnostics/path-mtu.md -- the wiring tests of show mtu
//
// Goal: prove `show mtu` reaches this module's handler through the registered
// RPC, that the payload is one document the pipe layer renders, and that the
// grammar refuses a host value that is not an address. Method: the handler is
// looked up by wire method in the registry init() filled, driven as the
// dispatcher drives it, and the host leaf's definition is read out of the
// command tree the loaded YANG builds.

package cmd

import (
	"encoding/json"
	"net/netip"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
	configyang "github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/env"
)

// registeredShowMTU answers the handler init() registered for ze-show:mtu,
// so every test below drives the same function the dispatcher does.
func registeredShowMTU(t *testing.T) pluginserver.Handler {
	t.Helper()
	for _, r := range pluginserver.AllBuiltinRPCs() {
		if r.WireMethod == wireMethodShowMTU {
			if r.Handler == nil {
				t.Fatalf("%s is registered with no handler", wireMethodShowMTU)
			}
			return r.Handler
		}
	}
	t.Fatalf("%s is not registered: show mtu reaches no handler", wireMethodShowMTU)
	return nil
}

// payloadOf answers the handler's data as the generic document a pipe reads.
func payloadOf(t *testing.T, resp *plugin.Response) map[string]any {
	t.Helper()
	encoded, err := json.Marshal(resp.Data)
	if err != nil {
		t.Fatalf("encode the payload: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(encoded, &doc); err != nil {
		t.Fatalf("decode the payload %s: %v", encoded, err)
	}
	return doc
}

// TestShowMTUReachesTheHandler is the first Wiring Test row: `show mtu` with
// no argument reaches handleShowMTU through the registered RPC and answers a
// document whose status names one of the run outcomes.
func TestShowMTUReachesTheHandler(t *testing.T) {
	newFakeDeps().install(t)
	handler := registeredShowMTU(t)
	resp, err := handler(nil, nil)
	if err != nil {
		t.Fatalf("show mtu: %v", err)
	}
	if resp.Status != plugin.StatusDone {
		t.Fatalf("show mtu answered status %q, error %q; want %q", resp.Status, resp.Error, plugin.StatusDone)
	}
	doc := payloadOf(t, resp)
	status, ok := doc[fieldStatus].(string)
	if !ok {
		t.Fatalf("the payload carries no %s string: %v", fieldStatus, doc)
	}
	switch status {
	case runStatusOK.String(), runStatusNothingMeasured.String(), runStatusDFGateFailed.String():
	default:
		t.Fatalf("status %q is none of the run outcomes", status)
	}
	for _, key := range []string{fieldMeasurements, fieldTunnels, fieldCommands, fieldNotes, fieldCaveats} {
		if _, present := doc[key]; !present {
			t.Errorf("the payload carries no %s key", key)
		}
	}
}

// TestShowMTUHostMeasuresOneAddress is the second Wiring Test row: `show mtu
// host <address>` reaches the same handler with one target, and the first
// measurement names that address and carries the path MTU the wire answered
// (AC-1). The wire is a fake clamped at 1400.
func TestShowMTUHostMeasuresOneAddress(t *testing.T) {
	f := newFakeDeps()
	f.paths[netip.MustParseAddr("192.0.2.1")] = clampedAt(1400)
	f.install(t)
	handler := registeredShowMTU(t)
	resp, err := handler(nil, []string{argHost, "192.0.2.1"})
	if err != nil {
		t.Fatalf("show mtu host: %v", err)
	}
	if resp.Status != plugin.StatusDone {
		t.Fatalf("show mtu host answered status %q, error %q; want %q", resp.Status, resp.Error, plugin.StatusDone)
	}
	doc := payloadOf(t, resp)
	measurements, ok := doc[fieldMeasurements].([]any)
	if !ok {
		t.Fatalf("the payload carries no %s list: %v", fieldMeasurements, doc)
	}
	if len(measurements) == 0 {
		t.Fatalf("show mtu host 192.0.2.1 measured nothing: %s is empty", fieldMeasurements)
	}
	first, ok := measurements[0].(map[string]any)
	if !ok {
		t.Fatalf("the first measurement is not a row: %v", measurements[0])
	}
	if got := first[fieldTarget]; got != "192.0.2.1" {
		t.Fatalf("the first measurement targets %v; want 192.0.2.1", got)
	}
	if got := first[fieldPathMTU]; got != float64(1400) {
		t.Fatalf("the measurement's path-mtu is %v; want 1400", got)
	}
}

// TestShowMTUPayloadRendersAsJSON is the fourth Wiring Test row: the payload
// goes through `| json` unrefused and comes out as the same document.
func TestShowMTUPayloadRendersAsJSON(t *testing.T) {
	newFakeDeps().install(t)
	handler := registeredShowMTU(t)
	resp, err := handler(nil, nil)
	if err != nil {
		t.Fatalf("show mtu: %v", err)
	}
	encoded, err := json.Marshal(resp.Data)
	if err != nil {
		t.Fatalf("encode the payload: %v", err)
	}
	_, ops := command.ParsePipe("show mtu | json")
	rendered, refusal := command.ApplyPipes(string(encoded), ops, nil, nil)
	if refusal != "" {
		t.Fatalf("`| json` refused the payload: %s", refusal)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(rendered), &doc); err != nil {
		t.Fatalf("`| json` rendered something that is not JSON: %v\n%s", err, rendered)
	}
	if _, ok := doc[fieldStatus]; !ok {
		t.Fatalf("the rendered document lost the %s key: %s", fieldStatus, rendered)
	}
	for key := range doc {
		if strings.ToLower(key) != key || strings.ContainsAny(key, "_ ") {
			t.Errorf("payload key %q is not kebab-case", key)
		}
	}
}

// TestShowMTUHostRefusedByTheGrammar proves the host leaf is typed as an
// address: a value outside the ip-address type is refused by the definition
// the dispatcher validates against, before the handler runs, and both address
// families pass it.
func TestShowMTUHostRefusedByTheGrammar(t *testing.T) {
	loader, err := configyang.DefaultLoader()
	if err != nil {
		t.Fatalf("load the YANG modules: %v", err)
	}
	tree := configyang.BuildCommandTree(loader)
	show := tree.Children["show"]
	if show == nil {
		t.Fatal("the command tree has no show verb")
	}
	node := show.Children["mtu"]
	if node == nil {
		t.Fatal("the command tree has no show mtu node: ze-mtu-cmd.yang did not merge")
	}
	var host *command.ArgDef
	for i := range node.ArgDefs {
		if node.ArgDefs[i].Name == argHost {
			host = &node.ArgDefs[i]
		}
	}
	if host == nil {
		t.Fatalf("show mtu declares no %s argument; it declares %d", argHost, len(node.ArgDefs))
	}
	if err := command.ValidateArgString("300.1.1.1", host); err == nil {
		t.Error("show mtu host 300.1.1.1 was accepted by the grammar; an octet above 255 is not an address")
	}
	if err := command.ValidateArgString("not-an-address", host); err == nil {
		t.Error("show mtu host not-an-address was accepted by the grammar")
	}
	for _, valid := range []string{"192.0.2.1", "2001:db8::1"} {
		if err := command.ValidateArgString(valid, host); err != nil {
			t.Errorf("show mtu host %s was refused by the grammar: %v", valid, err)
		}
	}
}

// TestParseMTUArgs pins the request the three keywords build, and the
// refusals: an unknown word, a duplicate, a missing or dash-leading host
// value, and a host that is not an address.
func TestParseMTUArgs(t *testing.T) {
	req, err := parseMTUArgs([]string{argHost, "2001:db8::1", argExhaustive, argDetail})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if req.host.String() != "2001:db8::1" {
		t.Errorf("host is %s; want 2001:db8::1", req.host)
	}
	if !req.exhaustive {
		t.Error("exhaustive was not read")
	}
	if !req.detail {
		t.Error("detail was not read")
	}
	req, err = parseMTUArgs(nil)
	if err != nil {
		t.Fatalf("parse with no argument: %v", err)
	}
	if req.host.IsValid() {
		t.Errorf("no host was given and yet host is %s", req.host)
	}
	for _, refused := range [][]string{
		{"peers"},
		{argHost},
		{argHost, "-h"},
		{argHost, "300.1.1.1"},
		{argHost, "192.0.2.1", argHost, "192.0.2.2"},
		{argExhaustive, argExhaustive},
		{argDetail, argDetail},
	} {
		if _, err := parseMTUArgs(refused); err == nil {
			t.Errorf("parse %v was accepted; want a refusal by name", refused)
		}
	}
}

// TestReferenceAddressReadsTheDefaultAndRefusesAMalformedOverride proves the
// leaf's default reaches the handler through the env layer, and that an
// override which is not an address is refused by name rather than read as no
// reference.
func TestReferenceAddressReadsTheDefaultAndRefusesAMalformedOverride(t *testing.T) {
	env.ResetCache()
	addr, err := referenceAddress()
	if err != nil {
		t.Fatalf("the default reference address: %v", err)
	}
	if addr.String() != referenceAddressEntry.Default {
		t.Errorf("the default reference address is %s; want %s", addr, referenceAddressEntry.Default)
	}

	// The canonical dot spelling is the one env.Set writes and env.Get reads.
	t.Setenv(envKeyReferenceAddress, "resolver.example")
	env.ResetCache()
	if _, err := referenceAddress(); err == nil {
		t.Error("a reference address that is a name was accepted; want a refusal")
	}
	newFakeDeps().install(t)
	resp, err := handleShowMTU(nil, nil)
	if err != nil {
		t.Fatalf("show mtu: %v", err)
	}
	if resp.Status != plugin.StatusError {
		t.Errorf("show mtu ran with a malformed reference address: status %q", resp.Status)
	}
	env.ResetCache()
}
