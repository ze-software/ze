package bridgeplugin

import (
	"log/slog"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/exabgp/bridge"
	"github.com/ze-software/ze/internal/plugins/exabgp/bridgerun"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestExabgpBridgeDeclaresItCanRestart: the bridge tells ze it may be started
// again after a failure.
//
// VALIDATES: AC-12 -- the ExaBGP bridge declares restart, so a migrated ExaBGP
// configuration that wrote `respawn true` is asking for something ze can give,
// and its scripts come back after a crash.
//
// PREVENTS: the owner's rule refusing the very configurations it exists to
// support. A bridge that declared nothing would be a plugin that "can not
// restart", so `respawn true` against it would stop the daemon at startup --
// and respawn is on by default in ExaBGP, which is where those configurations
// come from (exabgpRespawn, internal/exabgp/migration/migrate.go).
func TestExabgpBridgeDeclaresItCanRestart(t *testing.T) {
	reg := bridgeRegistration()
	if reg.FailurePolicy != sdk.FailureRestart {
		t.Fatalf("failure policy = %q, want %q", reg.FailurePolicy, sdk.FailureRestart)
	}
	if !reg.FailurePolicy.AllowsRestart() {
		t.Fatal("the bridge must permit ze to start it again")
	}
}

// VALIDATES: the internal exabgp-bridge parses every `process` block an ExaBGP
// config declares, and refuses one it cannot run.
// PREVENTS: a config naming two scripts running one of them in silence.

// TestParseConfigNested verifies the runner parses the nested
// `exabgp { bridge { ... } }` config root (spec-followup-subsystem AC-1, user
// directive 2026-07-09: nested shape, registry name stays exabgp-bridge).
func TestParseConfigNested(t *testing.T) {
	data := `{"exabgp":{"bridge":{"process":{"main":{"run":"./plugin.py arg"}},"family":["ipv4/unicast","ipv6/unicast"],"route-refresh":"true","add-path":"receive"}}}`
	cfg, err := parseConfig(data)
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	if !cfg.Present {
		t.Fatalf("expected Present=true")
	}
	if len(cfg.Scripts) != 1 {
		t.Fatalf("Scripts = %+v, want one", cfg.Scripts)
	}
	if got := cfg.Scripts[0]; got.Name != "main" || len(got.Argv) != 2 || got.Argv[0] != "./plugin.py" || got.Argv[1] != "arg" {
		t.Errorf("Scripts[0] = %+v, want main [./plugin.py arg]", got)
	}
	if len(cfg.Families) != 2 || cfg.Families[0] != "ipv4/unicast" || cfg.Families[1] != "ipv6/unicast" {
		t.Errorf("Families = %v", cfg.Families)
	}
	if !cfg.RouteRefresh {
		t.Errorf("RouteRefresh = false, want true")
	}
	if cfg.AddPath != "receive" {
		t.Errorf("AddPath = %q, want receive", cfg.AddPath)
	}
}

// TestParseConfigDefaults verifies an exabgp.bridge with only one process gets
// the default family, add-path=none, and ExaBGP's respawn default of true.
func TestParseConfigDefaults(t *testing.T) {
	cfg, err := parseConfig(`{"exabgp":{"bridge":{"process":{"main":{"run":"./plugin.py"}}}}}`)
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	if !cfg.Present {
		t.Fatalf("expected Present=true")
	}
	if len(cfg.Families) != 1 || cfg.Families[0] != defaultFamily {
		t.Errorf("Families = %v, want [%s]", cfg.Families, defaultFamily)
	}
	if cfg.AddPath != addPathNone {
		t.Errorf("AddPath = %q, want none", cfg.AddPath)
	}
	if cfg.RouteRefresh {
		t.Errorf("RouteRefresh = true, want false")
	}
	if len(cfg.Scripts) != 1 || !cfg.Scripts[0].Respawn {
		t.Errorf("Scripts = %+v, want one with Respawn=true", cfg.Scripts)
	}
}

// TestParseConfigAbsent verifies a section without the exabgp.bridge container
// yields Present=false with no error.
func TestParseConfigAbsent(t *testing.T) {
	cfg, err := parseConfig(`{"exabgp":{}}`)
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	if cfg.Present {
		t.Errorf("expected Present=false for empty exabgp root")
	}

	cfg, err = parseConfig(`{"other":{"x":"y"}}`)
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	if cfg.Present {
		t.Errorf("expected Present=false for unrelated root")
	}
}

// TestParseConfigInvalidFamily rejects an unregistered address family.
func TestParseConfigInvalidFamily(t *testing.T) {
	_, err := parseConfig(`{"exabgp":{"bridge":{"process":{"main":{"run":"./p.py"}},"family":["bogus/family"]}}}`)
	if err == nil {
		t.Fatalf("expected error for invalid family")
	}
}

// TestParseConfigInvalidAddPath rejects an out-of-range add-path mode.
func TestParseConfigInvalidAddPath(t *testing.T) {
	_, err := parseConfig(`{"exabgp":{"bridge":{"process":{"main":{"run":"./p.py"}},"add-path":"sideways"}}}`)
	if err == nil {
		t.Fatalf("expected error for invalid add-path")
	}
}

// TestCapabilityDecls maps route-refresh and add-path config to BGP capability
// declarations (RFC 2918 code 2, RFC 7911 code 69).
func TestCapabilityDecls(t *testing.T) {
	caps := capabilityDecls(bridgeConfig{RouteRefresh: true, AddPath: addPathNone, Families: []string{defaultFamily}})
	if len(caps) != 1 || caps[0].Code != 2 {
		t.Fatalf("route-refresh caps = %+v, want one code=2", caps)
	}

	caps = capabilityDecls(bridgeConfig{AddPath: "receive", Families: []string{defaultFamily}})
	if len(caps) != 1 || caps[0].Code != 69 {
		t.Fatalf("add-path caps = %+v, want one code=69", caps)
	}

	caps = capabilityDecls(bridgeConfig{AddPath: addPathNone, Families: []string{defaultFamily}})
	if len(caps) != 0 {
		t.Fatalf("no-cap config = %+v, want empty", caps)
	}
}

// TestFamilyDeclsDefault verifies familyDecls falls back to the default family
// when given none, and resolves configured families.
func TestFamilyDeclsDefault(t *testing.T) {
	decls := familyDecls(nil)
	if len(decls) != 1 || decls[0].Name != defaultFamily {
		t.Fatalf("default familyDecls = %+v, want [%s]", decls, defaultFamily)
	}

	decls = familyDecls([]string{"ipv4/unicast"})
	if len(decls) != 1 || decls[0].Name != "ipv4/unicast" {
		t.Fatalf("familyDecls = %+v", decls)
	}
}

// TestSplitCommand verifies whitespace argv splitting.
func TestSplitCommand(t *testing.T) {
	got := splitCommand("  python3   /opt/plugin.py  --flag ")
	want := []string{"python3", "/opt/plugin.py", "--flag"}
	if len(got) != len(want) {
		t.Fatalf("splitCommand = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("splitCommand[%d] = %q, want %q", i, got[i], want[i])
		}
	}
	if len(splitCommand("   ")) != 0 {
		t.Errorf("splitCommand(blank) should be empty")
	}
}

// TestParseConfigTwoProcesses is the defect this list exists to remove: an
// ExaBGP config declaring two processes reaches the runner as two scripts,
// sorted by name, with the respawn setting each block asked for.
func TestParseConfigTwoProcesses(t *testing.T) {
	data := `{"exabgp":{"bridge":{"process":{` +
		`"one-shot":{"run":"./run/api-no-respawn-1.run","respawn":"false"},` +
		`"respawn":{"run":"./run/api-no-respawn-2.run"}}}}}`
	cfg, err := parseConfig(data)
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	if len(cfg.Scripts) != 2 {
		t.Fatalf("Scripts = %+v, want two", cfg.Scripts)
	}
	if cfg.Scripts[0].Name != "one-shot" || cfg.Scripts[1].Name != "respawn" {
		t.Fatalf("Scripts = %+v, want [one-shot respawn] sorted by name", cfg.Scripts)
	}
	if cfg.Scripts[0].Respawn {
		t.Errorf("one-shot Respawn = true, want false (the block says respawn false)")
	}
	if !cfg.Scripts[1].Respawn {
		t.Errorf("respawn Respawn = false, want true (no leaf means ExaBGP's default)")
	}
	if got := cfg.Scripts[1].Argv; len(got) != 1 || got[0] != "./run/api-no-respawn-2.run" {
		t.Errorf("respawn Argv = %v", got)
	}
}

// TestParseConfigProcessWithoutRun REFUSES a process block the bridge cannot
// run, and names it. Starting the other blocks and saying nothing about this
// one is the silently-wrong answer ai/rules/principles.md bans.
func TestParseConfigProcessWithoutRun(t *testing.T) {
	_, err := parseConfig(`{"exabgp":{"bridge":{"process":{"good":{"run":"./a.py"},"bad":{}}}}}`)
	if err == nil {
		t.Fatalf("expected a refusal for a process with no run command")
	}
	if !strings.Contains(err.Error(), `"bad"`) {
		t.Errorf("error = %v, want it to name the process bad", err)
	}
}

// TestParseConfigRespawnInvalid fails closed on a respawn value it cannot read,
// rather than reading it as ExaBGP's default of true.
func TestParseConfigRespawnInvalid(t *testing.T) {
	_, err := parseConfig(`{"exabgp":{"bridge":{"process":{"main":{"run":"./a.py","respawn":"sometimes"}}}}}`)
	if err == nil {
		t.Fatalf("expected a refusal for respawn=sometimes")
	}
}

// TestParseConfigReadsTheEncoderAndTheFeeds checks that everything an ExaBGP
// process block carries beyond `run` and `respawn` reaches the runner. The
// `encoder` leaf was read by NOTHING until 2026-09-06: a config asking for the
// text event format was parsed, accepted, and answered in JSON.
func TestParseConfigReadsTheEncoderAndTheFeeds(t *testing.T) {
	data := `{"exabgp":{"bridge":{"process":{
		"public":{"run":"./public.run","encoder":"text","feed":{
			"127.0.0.1":{"event":["receive-update","send-update"]},
			"10.0.0.1":{"event":[]}}},
		"quiet":{"run":"./quiet.run"}}}}}`
	cfg, err := parseConfig(data)
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	if len(cfg.Scripts) != 2 {
		t.Fatalf("Scripts = %+v, want two", cfg.Scripts)
	}

	// parseScripts sorts by name, so `public` is first and `quiet` second.
	if got := cfg.Scripts[0]; got.Encoder != bridge.EncoderText {
		t.Errorf("public encoder = %v, want text", got.Encoder)
	}
	// parseFeeds sorts by address, so 10.0.0.1 comes before 127.0.0.1.
	feeds := cfg.Scripts[0].Feeds
	if len(feeds) != 2 || feeds[0].Peer != "10.0.0.1" || feeds[1].Peer != "127.0.0.1" {
		t.Fatalf("public feeds = %+v, want 10.0.0.1 then 127.0.0.1", feeds)
	}
	if got := feeds[1].Events; len(got) != 2 || got[0] != "receive-update" || got[1] != "send-update" {
		t.Errorf("127.0.0.1 events = %v, want [receive-update send-update]", got)
	}
	if got := feeds[0].Events; len(got) != 0 {
		t.Errorf("10.0.0.1 events = %v, want none: a neighbor can grant a script nothing", got)
	}

	// An absent encoder is JSON, which is what the 6.0.0 envelope this bridge
	// declares names, and an absent peer list is every peer.
	if got := cfg.Scripts[1]; got.Encoder != bridge.EncoderJSON {
		t.Errorf("quiet encoder = %v, want json", got.Encoder)
	}
	if got := cfg.Scripts[1].Feeds; len(got) != 0 {
		t.Errorf("quiet feeds = %v, want none: no feed block is every peer and every event", got)
	}
}

// TestParseConfigRefusesAnUnknownEncoder proves the leaf fails closed and names
// the process. Accepting the word and answering JSON is the silently-wrong
// value this leaf was added to remove.
func TestParseConfigRefusesAnUnknownEncoder(t *testing.T) {
	_, err := parseConfig(`{"exabgp":{"bridge":{"process":{"main":{"run":"./p.run","encoder":"yaml"}}}}}`)
	if err == nil {
		t.Fatalf("parseConfig accepted encoder yaml")
	}
	if !strings.Contains(err.Error(), "main") || !strings.Contains(err.Error(), "yaml") {
		t.Errorf("error = %v, want it to name the process and the word", err)
	}
}

// TestBridgeQueuesEventsFromConfigureNotFromStart pins the moment the bridge
// begins accepting events.
//
// VALIDATES: applyConfig builds the fleet, so onEvent has somewhere to put an
// event from OnConfigure onward, and an event delivered before the scripts start
// is queued rather than discarded.
// PREVENTS: the silent hole this closed. The fleet used to be built in
// startWhenReady, which runs at OnAllPluginsReady, and onEvent's `if fleet != nil`
// dropped everything before it without a word. Measured on
// test/exabgp-compat api-api: the sent OPEN, the received OPEN, both KEEPALIVEs,
// the state-up, the negotiated event, the received default route and BOTH
// End-of-RIB markers were discarded inside 16ms, and the fleet started 0.2ms
// after the last of them. A script whose first line must be `open` or the
// received `0.0.0.0/32` then waited out the mock peer's 60s deadline, which is
// why `api-open` and `api-api` failed by the clock and passed when run alone.
func TestBridgeQueuesEventsFromConfigureNotFromStart(t *testing.T) {
	runner := &bridgeRunner{log: slog.New(slog.DiscardHandler)}

	if runner.scripts() != nil {
		t.Fatal("a runner that has not been configured must hold no fleet")
	}

	cfg := bridgeConfig{
		Present:  true,
		Families: []string{"ipv4/unicast"},
		Scripts:  []bridgerun.Script{{Name: "watcher", Argv: []string{"/bin/cat"}, Encoder: bridge.EncoderJSON}},
	}
	if err := runner.applyConfig(nil, cfg); err != nil {
		t.Fatalf("applyConfig: %v", err)
	}

	fleet := runner.scripts()
	if fleet == nil {
		t.Fatal("the fleet must exist after OnConfigure, or every event until " +
			"OnAllPluginsReady reaches no script and is dropped in silence")
	}
	if fleet.Count() != 1 {
		t.Fatalf("the fleet carries %d script(s), want the 1 the config named", fleet.Count())
	}

	// Not started: the scripts are not forked, and that is the point. The fleet
	// accepts an event anyway, because bridgerun.New allocates each script's
	// queue at construction.
	runner.mu.Lock()
	started := runner.started
	runner.mu.Unlock()
	if started {
		t.Error("applyConfig must not START the scripts: a script's first line is a " +
			"command, and dispatching one during the bridge's own handshake aborts " +
			"the startup barrier")
	}

	if err := runner.onEvent(`{"type":"bgp","bgp":{"message":{"type":"open"}}}`); err != nil {
		t.Fatalf("onEvent before start: %v", err)
	}
}
