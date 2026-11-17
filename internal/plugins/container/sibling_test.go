// Design: docs/architecture/container/runtime.md -- the sibling seam: config, apply, engine calls, commands and doctor checks

package container

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// fakeNetworkConfig is a fake sibling's parsed config: the roots its parse
// was handed.
type fakeNetworkConfig struct {
	roots []string
}

// fakeConfigSibling is a config sibling that records its applies in events,
// with what the supervisor held at each, and refuses its parse with
// parseErr and its apply of a config that holds refusedRoot with applyErr.
type fakeConfigSibling struct {
	name        string
	events      *eventLog
	sup         *supervisor
	parseErr    error
	applyErr    error
	refusedRoot string
}

func (f *fakeConfigSibling) parse(sections []sdk.ConfigSection) (fakeNetworkConfig, error) {
	if f.parseErr != nil {
		return fakeNetworkConfig{}, f.parseErr
	}
	cfg := fakeNetworkConfig{}
	for _, section := range sections {
		cfg.roots = append(cfg.roots, section.Root)
	}
	return cfg, nil
}

func (f *fakeConfigSibling) apply(_ context.Context, cfg fakeNetworkConfig) error {
	f.sup.mu.Lock()
	started := f.sup.started
	desired := len(f.sup.desired)
	f.sup.mu.Unlock()
	event := "apply " + f.name + " roots=" + strings.Join(cfg.roots, ",")
	if started {
		event += " supervisor-started"
	}
	if desired > 0 {
		event += " supervisor-holds-containers"
	}
	f.events.add(event)
	if f.refusedRoot != "" && slices.Contains(cfg.roots, f.refusedRoot) {
		return f.applyErr
	}
	return nil
}

func (f *fakeConfigSibling) configSibling() configSibling {
	return newConfigSibling(f.name, f.parse, f.apply)
}

// sectionsOf answers a delivery holding the container root with one enabled
// container, web, and the other roots named.
func sectionsOf(roots ...string) []sdk.ConfigSection {
	sections := []sdk.ConfigSection{{Root: configRoot, Data: sectionDisabledWeb}}
	for _, root := range roots {
		sections = append(sections, sdk.ConfigSection{Root: root, Data: `{}`})
	}
	return sections
}

// TestSiblingParseRefusalRefusesTheConfig proves a sibling judges its own
// nodes in the plugin's one rule set: its parse is handed every section the
// plugin received, its value travels in the parsed config, and its refusal is
// the config's refusal, as the sibling's own operator error.
//
// Method: parse one delivery with a fake sibling that accepts, then with one
// that refuses with an operator error the runtime could not have written.
func TestSiblingParseRefusalRefusesTheConfig(t *testing.T) {
	accepting := &fakeConfigSibling{name: "network"}
	parsed, present, err := parseSectionsWith(sectionsOf("interface"), []configSibling{accepting.configSibling()})
	if err != nil {
		t.Fatalf("the accepting sibling's parse refused the config: %v", err)
	}
	if !present {
		t.Fatalf("a delivery with the container root parsed as absent")
	}
	if len(parsed.siblings) != 1 {
		t.Fatalf("the parsed config holds siblings %+v, want the network's alone", parsed.siblings)
	}
	if parsed.siblings[0].name != "network" {
		t.Fatalf("the parsed config holds the sibling %q, want network", parsed.siblings[0].name)
	}
	got, isNetwork := parsed.siblings[0].value.(fakeNetworkConfig)
	if !isNetwork {
		t.Fatalf("the parsed config holds %T for the network, want its parse's value", parsed.siblings[0].value)
	}
	if !slices.Equal(got.roots, []string{configRoot, "interface"}) {
		t.Fatalf("the sibling's parse was handed the roots %v, want the container root and interface", got.roots)
	}

	refusal := opererr.New("container network lan: the bridge is named twice", "a bridge has one name", "delete container network lan")
	refusing := &fakeConfigSibling{name: "network", parseErr: refusal}
	_, _, err = parseSectionsWith(sectionsOf(), []configSibling{refusing.configSibling()})
	if !errors.Is(err, refusal) {
		t.Fatalf("the refusing sibling's parse answered %v, want its own operator error", err)
	}
}

// TestSiblingWithoutContainerRootIsNotParsed proves a delivery that carries
// no container root reads as unchanged for the siblings too: no sibling's
// parse runs, so none can refuse a config the plugin does not hold.
//
// Method: a refusing sibling and a delivery with only the interface root.
func TestSiblingWithoutContainerRootIsNotParsed(t *testing.T) {
	refusing := &fakeConfigSibling{name: "network", parseErr: errors.New("parsed")}
	_, present, err := parseSectionsWith([]sdk.ConfigSection{{Root: "interface", Data: `{}`}}, []configSibling{refusing.configSibling()})
	if err != nil {
		t.Fatalf("a delivery without the container root ran the sibling's parse: %v", err)
	}
	if present {
		t.Fatalf("a delivery without the container root parsed as present")
	}
}

// installerFixture is an install over a test supervisor and the fake
// siblings, in that order of registration.
func installerFixture(t *testing.T, events *eventLog, siblings ...*fakeConfigSibling) (*configInstaller, *supervisor) {
	t.Helper()
	store := &fakeStore{}
	store.setAnswer(readyImage())
	sup := newSupervisor(slogutil.DiscardLogger(), &fakeLibrary{events: &eventLog{}}, storeFactoryOf(store), nil)
	t.Cleanup(sup.close)
	configs := make([]configSibling, 0, len(siblings))
	for _, sibling := range siblings {
		sibling.events, sibling.sup = events, sup
		configs = append(configs, sibling.configSibling())
	}
	return newConfigInstaller(context.Background(), sup, configs), sup
}

// TestSiblingApplyRunsBeforeTheSupervisorsApply proves the order of the
// install: the boot configure applies no sibling, since the engine answers
// none of its requests yet; the started notice applies each sibling, in
// registration order, before the supervisor starts any container; and a
// reload applies each sibling before the supervisor holds the new desired
// set.
//
// Method: fake siblings record, at each apply, whether the supervisor had
// started and whether it held a container.
func TestSiblingApplyRunsBeforeTheSupervisorsApply(t *testing.T) {
	events := &eventLog{}
	network := &fakeConfigSibling{name: "network"}
	shell := &fakeConfigSibling{name: "shell"}
	installer, sup := installerFixture(t, events, network, shell)
	siblings := installer.siblings

	boot, _, err := parseSectionsWith(sectionsOf(), siblings)
	if err != nil {
		t.Fatalf("the boot config was refused: %v", err)
	}
	if err := installer.install(boot); err != nil {
		t.Fatalf("the boot install failed: %v", err)
	}
	if got := events.all(); len(got) != 0 {
		t.Fatalf("the boot configure applied siblings before the plugin started: %v", got)
	}

	if err := installer.begin(); err != nil {
		t.Fatalf("the started notice failed: %v", err)
	}
	afterBegin := []string{
		"apply network roots=container supervisor-holds-containers",
		"apply shell roots=container supervisor-holds-containers",
	}
	if got := events.all(); !slices.Equal(got, afterBegin) {
		t.Fatalf("the started notice made:\n  %v\nwant:\n  %v", got, afterBegin)
	}
	sup.mu.Lock()
	started := sup.started
	sup.mu.Unlock()
	if !started {
		t.Fatalf("the started notice applied the siblings and never began the supervisor")
	}

	if err := installer.install(configOf()); err != nil {
		t.Fatalf("emptying the supervisor failed: %v", err)
	}
	reload, _, err := parseSectionsWith(sectionsOf("interface"), siblings)
	if err != nil {
		t.Fatalf("the reload config was refused: %v", err)
	}
	if err := installer.install(reload); err != nil {
		t.Fatalf("the reload install failed: %v", err)
	}
	reloaded := append(slices.Clone(afterBegin),
		// The emptying config holds no sibling part: each gets its zero config.
		"apply network roots= supervisor-started supervisor-holds-containers",
		"apply shell roots= supervisor-started supervisor-holds-containers",
		// The supervisor still holds nothing when the siblings apply: its own
		// apply comes after.
		"apply network roots=container,interface supervisor-started",
		"apply shell roots=container,interface supervisor-started",
	)
	if got := events.all(); !slices.Equal(got, reloaded) {
		t.Fatalf("the reload made:\n  %v\nwant:\n  %v", got, reloaded)
	}
	if _, held := sup.instance("web"); !held {
		t.Fatalf("the reload's install never reached the supervisor")
	}
}

// TestSiblingApplyFailureRefusesTheCommit proves a sibling's apply refusal
// leaves the running config as it was: the sibling that applied before it
// gets its previous config back, the supervisor's apply is not made, and the
// refusal is the commit's answer, unwrapped.
//
// Method: two siblings, the second refusing a config that reads the
// interface root; the supervisor's desired set is read after the refusal.
func TestSiblingApplyFailureRefusesTheCommit(t *testing.T) {
	events := &eventLog{}
	refusal := opererr.New("container network lan: the bridge could not be made", "the interface component refused it", "ze show doctor")
	network := &fakeConfigSibling{name: "network"}
	firewall := &fakeConfigSibling{name: "firewall", refusedRoot: "interface", applyErr: refusal}
	installer, sup := installerFixture(t, events, network, firewall)
	if err := installer.begin(); err != nil {
		t.Fatalf("the started notice failed: %v", err)
	}
	// The running config holds no container, and the candidate holds web.
	running, _, err := parseSectionsWith([]sdk.ConfigSection{{Root: configRoot, Data: sectionNoContainer}}, installer.siblings)
	if err != nil {
		t.Fatalf("the running config was refused: %v", err)
	}
	if err := installer.install(running); err != nil {
		t.Fatalf("installing the running config failed: %v", err)
	}
	candidate, _, err := parseSectionsWith(sectionsOf("interface"), installer.siblings)
	if err != nil {
		t.Fatalf("the candidate was refused at parse: %v", err)
	}
	before := len(events.all())

	err = installer.install(candidate)

	if !errors.Is(err, refusal) {
		t.Fatalf("the install answered %v, want the sibling's own refusal", err)
	}
	if errors.Is(err, sdk.ErrRunningConfigLost) {
		t.Fatalf("the install reported the running config lost, though the put-back succeeded: %v", err)
	}
	want := []string{
		"apply network roots=container,interface supervisor-started",
		"apply firewall roots=container,interface supervisor-started",
		"apply network roots=container supervisor-started",
	}
	if got := events.all()[before:]; !slices.Equal(got, want) {
		t.Fatalf("the refused install made:\n  %v\nwant:\n  %v", got, want)
	}
	if _, held := sup.instance("web"); held {
		t.Fatalf("the supervisor's apply was made although a sibling refused the config")
	}
}

// TestSiblingCommandIsDeclaredAndDispatched proves a sibling's command is in
// the plugin's declarations and is answered by the sibling's handler through
// the plugin's one command entry, and that a sibling cannot take a command
// the runtime answers.
//
// Method: a test registry with one command, handleCommand on a supervisor
// that holds it.
func TestSiblingCommandIsDeclaredAndDispatched(t *testing.T) {
	set := &siblingRegistry{}
	var handed []string
	set.addCommand(SiblingCommand{
		Decl:       sdk.CommandDecl{Name: "show container network list", ShortHelp: "Show the container networks."},
		WireMethod: "ze-container:show-network-list",
		Handle: func(_ context.Context, args []string) (any, error) {
			handed = args
			return map[string]any{"networks": []string{"lan"}}, nil
		},
	})
	if decls := set.commandDecls(); len(decls) != 1 || decls[0].Name != "show container network list" {
		t.Fatalf("the sibling's declarations are %+v, want its one command", decls)
	}
	sup := newSupervisor(slogutil.DiscardLogger(), nil, nil, nil)
	sup.siblings = set

	status, answer, err := handleCommand(context.Background(), sup, "show container network list", []string{"name", "lan"})

	if err != nil {
		t.Fatalf("the sibling's command was refused: %v", err)
	}
	if status != rpc.StatusDone {
		t.Fatalf("the sibling's command answered status %q, want %q", status, rpc.StatusDone)
	}
	if !slices.Equal(handed, []string{"name", "lan"}) {
		t.Fatalf("the sibling's handler was handed %v, want the command's arguments", handed)
	}
	document, isDocument := answer.(map[string]any)
	if !isDocument {
		t.Fatalf("the command answered %T, want the sibling's document", answer)
	}
	networks, isList := document["networks"].([]string)
	if !isList {
		t.Fatalf("the command answered %v, want the sibling's list of networks", answer)
	}
	if !slices.Equal(networks, []string{"lan"}) {
		t.Fatalf("the command answered %v, want the sibling's answer", answer)
	}

	assertPanics(t, "a sibling taking the restart command", func() {
		set.addCommand(SiblingCommand{Decl: sdk.CommandDecl{Name: cmdRequestRestart}, WireMethod: "w", Handle: func(context.Context, []string) (any, error) { return struct{}{}, nil }})
	})
	assertPanics(t, "a sibling taking an image command", func() {
		set.addCommand(SiblingCommand{Decl: sdk.CommandDecl{Name: cmdShowImageList}, WireMethod: "w", Handle: func(context.Context, []string) (any, error) { return struct{}{}, nil }})
	})
}

// TestSiblingDoctorCheckIsDeclaredAndAnswered proves a sibling's doctor check
// is in the plugin's declarations and its answer is the sibling's, through
// the plugin's one doctor callback, and that a sibling cannot take a
// runtime check's name.
//
// Method: a test registry with one check, answerDoctorCheck on a supervisor
// that holds it.
func TestSiblingDoctorCheckIsDeclaredAndAnswered(t *testing.T) {
	set := &siblingRegistry{}
	finding := rpc.DoctorCheckDiagnostic{Code: "doctor-container-network-test", Severity: "error", Message: "container web has no address", Fix: "ze show doctor"}
	set.addDoctorCheck(SiblingDoctorCheck{
		Decl:   rpc.DoctorCheckDecl{Name: "container-network-test", Phase: rpc.DoctorPhaseRuntime, Codes: []string{finding.Code}},
		Answer: func() ([]rpc.DoctorCheckDiagnostic, error) { return []rpc.DoctorCheckDiagnostic{finding}, nil },
	})
	if decls := set.doctorCheckDecls(); len(decls) != 1 || decls[0].Name != "container-network-test" {
		t.Fatalf("the sibling's check declarations are %+v, want its one check", decls)
	}
	sup := newSupervisor(slogutil.DiscardLogger(), nil, nil, nil)
	sup.siblings = set

	findings, err := sup.answerDoctorCheck("container-network-test")

	if err != nil {
		t.Fatalf("the sibling's check was refused: %v", err)
	}
	if !slices.Equal(findings, []rpc.DoctorCheckDiagnostic{finding}) {
		t.Fatalf("the check answered %+v, want the sibling's finding", findings)
	}
	assertPanics(t, "a sibling taking a runtime check's name", func() {
		set.addDoctorCheck(SiblingDoctorCheck{
			Decl:   rpc.DoctorCheckDecl{Name: runtimeDoctorChecks[0].decl.Name, Codes: []string{"doctor-x"}},
			Answer: func() ([]rpc.DoctorCheckDiagnostic, error) { return nil, nil },
		})
	})
}

// fakeEngine records each request it answers.
type fakeEngine struct {
	requests []string
}

func (f *fakeEngine) OwnedBridgesReplace(context.Context, *rpc.OwnedBridgesReplaceInput) (*rpc.OwnedBridgesReplaceOutput, error) {
	f.requests = append(f.requests, "bridges")
	return &rpc.OwnedBridgesReplaceOutput{}, nil
}

func (f *fakeEngine) OwnedLinkCreate(context.Context, *rpc.OwnedLinkCreateInput) (*rpc.OwnedLinkCreateOutput, error) {
	f.requests = append(f.requests, "link-create")
	return &rpc.OwnedLinkCreateOutput{}, nil
}

func (f *fakeEngine) OwnedLinkRemove(context.Context, *rpc.OwnedLinkRemoveInput) (*rpc.OwnedLinkRemoveOutput, error) {
	f.requests = append(f.requests, "link-remove")
	return &rpc.OwnedLinkRemoveOutput{}, nil
}

func (f *fakeEngine) FirewallTablesReplace(context.Context, *rpc.FirewallTablesReplaceInput) (*rpc.FirewallTablesReplaceOutput, error) {
	f.requests = append(f.requests, "tables")
	return &rpc.FirewallTablesReplaceOutput{}, nil
}

func (f *fakeEngine) StateGet(_ context.Context, key string) ([]byte, bool, error) {
	f.requests = append(f.requests, "get "+key)
	return nil, false, nil
}

func (f *fakeEngine) StatePut(_ context.Context, key string, _ []byte) error {
	f.requests = append(f.requests, "put "+key)
	return nil
}

func (f *fakeEngine) EmitEvent(_ context.Context, namespace, eventType, _, _, _ string) (int, error) {
	f.requests = append(f.requests, "emit "+namespace+" "+eventType)
	return 1, nil
}

// TestEngineClientIsHandedAndRefusesUntilStarted proves a sibling receives
// the engine client, that a request made before the engine started the
// plugin (an ended call of the start-up sweep) is refused at once and never
// sent, and that every request reaches the engine once it started.
//
// Method: a receiver stores the client handed over a recording engine; each
// of the seven requests is made before and after open.
func TestEngineClientIsHandedAndRefusesUntilStarted(t *testing.T) {
	set := &siblingRegistry{}
	var received EngineClient
	set.addReceiver(func(client EngineClient) { received = client })
	inner := &fakeEngine{}
	engine := &startedEngine{engine: inner}
	set.handEngine(engine)
	if received == nil {
		t.Fatalf("the receiver was handed no client")
	}

	ctx := context.Background()
	requests := func() []error {
		_, bridges := received.OwnedBridgesReplace(ctx, &rpc.OwnedBridgesReplaceInput{})
		_, create := received.OwnedLinkCreate(ctx, &rpc.OwnedLinkCreateInput{})
		_, remove := received.OwnedLinkRemove(ctx, &rpc.OwnedLinkRemoveInput{})
		_, tables := received.FirewallTablesReplace(ctx, &rpc.FirewallTablesReplaceInput{})
		_, _, get := received.StateGet(ctx, "meta/container-network/lan")
		put := received.StatePut(ctx, "meta/container-network/lan", []byte("x"))
		_, emit := received.EmitEvent(ctx, "sysctl", "default", "", "", `{"key":"k","value":"1","source":"s"}`)
		return []error{bridges, create, remove, tables, get, put, emit}
	}

	for i, err := range requests() {
		if err == nil {
			t.Errorf("request %d before the start was answered, want a refusal", i)
		}
	}
	if len(inner.requests) != 0 {
		t.Fatalf("requests before the start reached the engine: %v", inner.requests)
	}

	engine.open()
	for i, err := range requests() {
		if err != nil {
			t.Errorf("request %d after the start was refused: %v", i, err)
		}
	}
	want := []string{"bridges", "link-create", "link-remove", "tables", "get meta/container-network/lan", "put meta/container-network/lan", "emit sysctl default"}
	if !slices.Equal(inner.requests, want) {
		t.Fatalf("the engine received %v, want %v", inner.requests, want)
	}
}

// assertPanics fails the test when call returns without a panic.
func assertPanics(t *testing.T, what string, call func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Errorf("%s did not panic", what)
		}
	}()
	call()
}
