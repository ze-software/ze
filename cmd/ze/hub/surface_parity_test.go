//go:build ze_ssh && ze_web

package hub

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/cli"
	cliclient "github.com/ze-software/ze/internal/component/cli/client"
	"github.com/ze-software/ze/internal/component/command"
	zeconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/infra"
	yangloader "github.com/ze-software/ze/internal/component/config/yang"
	_ "github.com/ze-software/ze/internal/component/config/yang/cli" // show yang ...: local-data commands only cmd/ze links otherwise
	"github.com/ze-software/ze/internal/component/plugin"
	_ "github.com/ze-software/ze/internal/component/plugin/all"
	pluginmgr "github.com/ze-software/ze/internal/component/plugin/manager"
	"github.com/ze-software/ze/internal/component/plugin/process"
	pluginreg "github.com/ze-software/ze/internal/component/plugin/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/plugins/completion"
	"github.com/ze-software/ze/internal/test/userns"
)

// The fixture plugin: one visible command with no YANG node and one hidden
// command. These are the only command paths this file spells; every other
// command comes from a registry.
const (
	surfaceFixturePlugin  = "zz-surface-fixture"
	surfaceFixtureVisible = "show zz-surface-fixture visible"
	surfaceFixtureHidden  = "show zz-surface-fixture hidden"
)

// TestEveryCommandOnEverySurface walks every command every registry declares
// and asks each operator surface's own producer whether it offers it, then
// compares that with command.OffersCommand.
//
// Method: a daemon dispatcher is built by pluginserver.NewServer from the
// linked registrations, the way the daemon builds it, and every internal
// plugin's Stage 1 commands plus a fixture plugin are registered into its
// command registry. A surface offers a command when its producer lists it AND,
// for a surface that executes through the daemon, the dispatcher can serve it:
// a listed command the dispatcher refuses is an offer the surface cannot keep.
//
// VALIDATES: AC-1, AC-2, AC-3 and AC-11 of spec-command-surface-parity: every
// registration is classified from its registry, with no exception list.
// PREVENTS: a surface that omits a command it can serve, or lists one it cannot.
func TestEveryCommandOnEverySurface(t *testing.T) {
	server, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, nil)
	if err != nil {
		t.Fatalf("daemon dispatcher: %v", err)
	}
	t.Cleanup(server.Stop)
	d := server.Dispatcher()

	schema, err := yangloader.DefaultLoader()
	if err != nil {
		t.Fatalf("YANG schema: %v", err)
	}
	registerStage1Commands(t, d)
	regs := collectCommandRegistrations(t, schema, cli.RegisteredViews())
	arms := surfaceArms(t, server)
	for _, failure := range surfaceFailures(regs, arms) {
		t.Error(failure)
	}
}

// TestUndeclaredTUIOnlyViewFailsTheCheck proves AC-13's guard with a fixture
// view: a view whose prefix no other registry holds, and whose registration
// does not declare it TUI-only, fails the check on every surface, and the
// same view declared TUI-only passes on surfaces that answer as the predicate
// says.
//
// VALIDATES: AC-13 of spec-command-surface-parity: an undeclared TUI-only
// command fails the check.
// PREVENTS: a view-only command silently missing from the one-shot surfaces
// with nothing declaring why.
func TestUndeclaredTUIOnlyViewFailsTheCheck(t *testing.T) {
	const prefix = "monitor zz-surface-fixture view"
	never := func(string) bool { return false }
	always := func(string) bool { return true }
	arms := make(map[command.Surface]surfaceArm)
	for _, s := range command.Surfaces() {
		listed := never
		if s == command.SurfaceSSH || s == command.SurfaceCLI {
			listed = always
		}
		arms[s] = surfaceArm{listed: listed, served: always}
	}

	undeclared := []command.CommandRegistration{viewRegistration(cli.ViewInfo{Key: "zz", Prefix: prefix})}
	if got := surfaceFailures(undeclared, arms); len(got) != len(command.Surfaces()) {
		t.Errorf("an undeclared TUI-only view gave %d failures, want one per surface (%d): %q", len(got), len(command.Surfaces()), got)
	}
	declared := []command.CommandRegistration{viewRegistration(cli.ViewInfo{Key: "zz", Prefix: prefix, TUIOnly: true})}
	if got := surfaceFailures(declared, arms); len(got) != 0 {
		t.Errorf("a declared TUI-only view offered on SSH and ze cli only failed: %q", got)
	}
}

// surfaceFailures compares every surface's answer for every registration with
// command.AnswerSurface and returns one failure line per disagreement, naming
// the path, the surface, the registration kind and the expected answer.
func surfaceFailures(regs []command.CommandRegistration, arms map[command.Surface]surfaceArm) []string {
	var failures []string
	for _, reg := range regs {
		for _, s := range command.Surfaces() {
			answer, err := command.AnswerSurface(reg, s)
			if err != nil {
				failures = append(failures, err.Error())
				continue
			}
			arm, ok := arms[s]
			if !ok {
				continue // reported once by surfaceArms
			}
			want := answer.Offer.Listed()
			listed := arm.listed(reg.Path)
			served := arm.served(reg.Path)
			if want == (listed && served) {
				continue
			}
			var tb textbuf.Buffer
			tb.Byte('"').Str(reg.Path).Str(`" on `).Str(s.String()).Str(" (").Str(reg.Kind.String()).Str("): ")
			switch {
			case want && !listed:
				tb.Str("the surface does not list it")
			case want:
				tb.Str("listed, but the daemon dispatcher cannot serve it")
			default:
				tb.Str("the surface offers it")
			}
			tb.Str("; want ").Str(answer.Offer.String()).Str(": ").Str(answer.Reason)
			failures = append(failures, tb.String())
		}
	}
	return failures
}

// surfaceArm is what one surface's producer answers for a command path.
type surfaceArm struct {
	listed func(path string) bool // the producer lists the path
	served func(path string) bool // running it from this surface reaches a handler
}

// surfaceArms asks each surface's own producer once and returns its answers.
func surfaceArms(t *testing.T, server *pluginserver.Server) map[command.Surface]surfaceArm {
	t.Helper()
	d := server.Dispatcher()
	served := dispatcherServes(d)

	// SSH: the per-session tree, session_factory.go buildCommandTree and
	// mergePluginCommands.
	sshTree, err := buildCommandTree()
	if err != nil {
		t.Fatalf("ssh command tree: %v", err)
	}
	mergePluginCommands(sshTree, infra.HookParams{APIServer: func() *pluginserver.Server { return server }})

	// Web admin: the per-request tree service_web.go's adminCommandTrees hands
	// HandleAdminView and HandleAdminExecute, with the live plugin overlay.
	adminSchema, err := yangloader.DefaultLoader()
	if err != nil {
		t.Fatalf("web admin command tree: %v", err)
	}
	adminTree := adminCommandTrees(yangloader.BuildCommandTree(adminSchema),
		func() []command.CommandEntry { return d.Registry().VisibleCommandEntries() })()

	// Web terminal: the completer service_web.go builds over the same tree,
	// with the live plugin overlay main.go wires as WebCommands.
	webCompleter := newPluginAwareCommandCompleter(yangloader.BuildCommandTree(adminSchema),
		func() []command.CommandEntry { return d.Registry().VisibleCommandEntries() })

	// MCP tools/list, the REST command list and the gRPC command list: the one
	// list commandMetaSource answers from the live dispatcher.
	metas, err := commandMetaSource(server)()
	if err != nil {
		t.Fatalf("command metadata: %v", err)
	}
	inMeta := make(map[string]bool, len(metas))
	for _, m := range metas {
		inMeta[strings.ToLower(m.Name)] = true
	}
	metaListed := func(path string) bool { return inMeta[strings.ToLower(path)] }

	// `ze cli`: its interactive completion tree, built from the daemon's
	// `system command list` answer as the attached console reads it.
	cliTree, err := cliclient.BuildRuntimeTreeFromDispatch(func(input string) (cli.CommandOutput, error) {
		resp, err := serverDispatcher(server, "")(t.Context(), plugin.CallerIdentity{Username: "surface-check"}, input)
		if err != nil {
			return cli.CommandOutput{}, err
		}
		text, err := json.Marshal(resp.Data)
		if err != nil {
			return cli.CommandOutput{}, err
		}
		return cli.CommandOutput{Text: string(text)}, nil
	})
	if err != nil {
		t.Fatalf("ze cli command tree: %v", err)
	}

	shell := newShellWords(t)
	return map[command.Surface]surfaceArm{
		command.SurfaceSSH:      {listed: func(p string) bool { return treeHasPath(sshTree, p) }, served: served},
		command.SurfaceWebAdmin: {listed: func(p string) bool { return treeHasPath(adminTree, p) }, served: served},
		command.SurfaceWebCompletion: {listed: func(p string) bool {
			return completionOffers(webCompleter.Complete, p)
		}, served: served},
		command.SurfaceMCP:  {listed: metaListed, served: served},
		command.SurfaceREST: {listed: metaListed, served: served},
		command.SurfaceGRPC: {listed: metaListed, served: served},
		// `ze cli` answers a local-data command in its own process
		// (command.ServeLocal) and every other command through the daemon.
		command.SurfaceCLI: {listed: func(p string) bool { return treeHasPath(cliTree, p) }, served: func(p string) bool {
			return command.HasLocalData(p) || served(p)
		}},
		// Shell completion runs with no daemon; `ze <verb>` answers locally, so
		// its listing is the whole answer.
		command.SurfaceShell: {listed: shell.offers, served: func(string) bool { return true }},
	}
}

// dispatcherServes answers whether the daemon dispatcher routes a path to a
// handler: a builtin it holds, or a plugin command its registry holds.
func dispatcherServes(d *pluginserver.Dispatcher) func(path string) bool {
	builtins := make(map[string]bool)
	for _, c := range d.Commands() {
		builtins[strings.ToLower(c.Name)] = true
	}
	return func(path string) bool {
		if builtins[strings.ToLower(path)] {
			return true
		}
		return d.Registry().Lookup(path) != nil
	}
}

// treeHasPath answers whether each word of path is a child of the one before.
func treeHasPath(tree *command.Node, path string) bool {
	node := tree
	for word := range strings.FieldsSeq(path) {
		if node == nil || node.Children == nil {
			return false
		}
		node = node.Children[word]
	}
	return node != nil
}

// completionOffers answers whether completing the words before the last one
// offers the last one.
func completionOffers(complete func(string) []cli.Completion, path string) bool {
	words := strings.Fields(path)
	if len(words) == 0 {
		return false
	}
	input := ""
	if len(words) > 1 {
		input = strings.Join(words[:len(words)-1], " ") + " "
	}
	for _, c := range complete(input) {
		if c.Text == words[len(words)-1] {
			return true
		}
	}
	return false
}

// shellWords asks `ze completion words run <prefix>` and keeps each answer.
type shellWords struct {
	t     *testing.T
	seen  map[string][]string
	roots map[string]bool // the bash script's root names, read once
}

func newShellWords(t *testing.T) *shellWords {
	return &shellWords{t: t, seen: make(map[string][]string)}
}

// offers answers whether shell completion of the words before the last one
// offers the last one.
func (w *shellWords) offers(path string) bool {
	words := strings.Fields(path)
	if len(words) == 0 {
		return false
	}
	prefix := words[:len(words)-1]
	key := strings.Join(prefix, " ")
	candidates, ok := w.seen[key]
	if !ok {
		candidates = w.ask(prefix)
		w.seen[key] = candidates
	}
	if slices.Contains(candidates, words[len(words)-1]) {
		return true
	}
	// A path at an offline root is a `ze <root>` command, never a `ze run`
	// one, so the root's own completion is the shell's listing of it.
	if command.IsVerb(words[0]) {
		return false
	}
	return w.rootOffers(words)
}

// ask answers the first field of each record `ze completion words run
// <prefix>` writes: the daemon command tree `ze run` completes.
func (w *shellWords) ask(prefix []string) []string {
	w.t.Helper()
	return completionRecords(w.capture(append([]string{"words", "run"}, prefix...)))
}

// rootOffers answers whether shell completion offers path as a command at an
// offline root (`ze doctor`): a root is not a verb, so `ze run` never
// completes it. Its first word is one of the root names the generated bash
// script lists (`local commands="..."`, the registry's visible roots), and
// each later word is offered by `ze completion words <root> ...`, the root's
// own completion.
func (w *shellWords) rootOffers(words []string) bool {
	w.t.Helper()
	if w.roots == nil {
		w.roots = bashRootNames(w.t, w.capture([]string{"bash"}))
	}
	if !w.roots[words[0]] {
		return false
	}
	for i := 1; i < len(words); i++ {
		if !slices.Contains(completionRecords(w.capture(append([]string{"words"}, words[:i]...))), words[i]) {
			return false
		}
	}
	return true
}

// capture runs completion.Run, which writes to os.Stdout, with stdout pointed
// at a file, and returns what it wrote.
func (w *shellWords) capture(args []string) string {
	w.t.Helper()
	out, err := os.CreateTemp(w.t.TempDir(), "words")
	if err != nil {
		w.t.Fatalf("shell completion capture: %v", err)
	}
	defer out.Close() //nolint:errcheck // a read-only capture file

	saved := os.Stdout
	os.Stdout = out
	code := completion.Run(args)
	os.Stdout = saved
	if code != 0 {
		w.t.Errorf("ze completion %s: exit %d", strings.Join(args, " "), code)
		return ""
	}
	written, err := os.ReadFile(out.Name())
	if err != nil {
		w.t.Fatalf("shell completion capture: %v", err)
	}
	return string(written)
}

// completionRecords answers the first field of each completion record.
func completionRecords(text string) []string {
	var words []string
	for line := range strings.SplitSeq(text, "\n") {
		word, _, _ := strings.Cut(line, "\t")
		if word != "" {
			words = append(words, word)
		}
	}
	return words
}

// bashRootNames answers the root names the generated bash script completes.
func bashRootNames(t *testing.T, script string) map[string]bool {
	t.Helper()
	const marker = `local commands="`
	start := strings.Index(script, marker)
	if start < 0 {
		t.Fatalf("the generated bash completion script names no root commands (%s)", marker)
	}
	list, _, found := strings.Cut(script[start+len(marker):], `"`)
	if !found {
		t.Fatalf("the generated bash completion script's root list is not closed")
	}
	roots := make(map[string]bool)
	for name := range strings.FieldsSeq(list) {
		roots[name] = true
	}
	return roots
}

// registerStage1Commands registers every linked internal plugin's Stage 1
// command declarations, and the fixture plugin, into the dispatcher's command
// registry, as plugin startup does (startup.go). A name a builtin already
// holds is refused there, as in the daemon.
func registerStage1Commands(t *testing.T, d *pluginserver.Dispatcher) {
	t.Helper()
	for _, r := range pluginreg.All() {
		if len(r.Commands) == 0 {
			continue
		}
		defs := make([]pluginserver.CommandDef, 0, len(r.Commands))
		for _, decl := range r.Commands {
			defs = append(defs, pluginserver.CommandDef{
				Name: decl.Name, ShortHelp: decl.ShortHelp, Description: decl.Description, Hidden: decl.Hidden,
			})
		}
		d.Registry().Register(process.NewProcess(plugin.PluginConfig{Name: r.Name}), defs)
	}
	fixture := process.NewProcess(plugin.PluginConfig{Name: surfaceFixturePlugin})
	for _, res := range d.Registry().Register(fixture, []pluginserver.CommandDef{
		{Name: surfaceFixtureVisible, ShortHelp: "A fixture command with no YANG node."},
		{Name: surfaceFixtureHidden, ShortHelp: "A hidden fixture command.", Hidden: true},
	}) {
		if !res.OK {
			t.Fatalf("fixture plugin command %q: %s", res.Name, res.Error)
		}
	}
}

// collectCommandRegistrations enumerates every command from its registry:
// builtin RPCs under their YANG paths, local and local-data handlers, plugin
// Stage 1 declarations with no YANG node, streaming prefixes and CLI live
// views. A path declared by two sources keeps the first kind in that order.
func collectCommandRegistrations(t *testing.T, schema *yangloader.Resolved, views []cli.ViewInfo) []command.CommandRegistration {
	t.Helper()
	var regs []command.CommandRegistration
	seen := make(map[string]bool)
	streaming := make(map[string]bool)
	for _, prefix := range pluginserver.StreamingPrefixes() {
		streaming[strings.ToLower(prefix)] = true
	}
	add := func(reg command.CommandRegistration) {
		key := strings.ToLower(reg.Path)
		if seen[key] {
			return
		}
		seen[key] = true
		reg.Streaming = streaming[key]
		if reg.Streaming {
			oneShot, _ := pluginserver.StreamingOneShot(key)
			reg.StreamingOnly = oneShot == pluginserver.OneShotStreamingOnly
		}
		regs = append(regs, reg)
	}

	wireToPaths := yangloader.WireMethodToPaths(schema)
	yangPaths := make(map[string]bool)
	for _, paths := range wireToPaths {
		for _, p := range paths {
			yangPaths[strings.ToLower(p)] = true
		}
	}
	for _, rpc := range pluginserver.AllBuiltinRPCs() {
		if rpc.Handler == nil {
			continue // editor-internal, no command
		}
		for _, p := range wireToPaths[rpc.WireMethod] {
			add(command.CommandRegistration{Path: p, Kind: command.KindBuiltin})
		}
	}
	// A local command at an offline root carries the daemon path its
	// registration declares (registry.Meta.DaemonPath): AnswerSurface offers
	// that path on the daemon surfaces in its place, so the path must be a
	// registration of its own, judged on every surface like any other.
	var daemonPaths []command.LocalCommandEntry
	for _, e := range command.ListLocal() {
		if command.HasLocalData(e.Path) {
			add(command.CommandRegistration{Path: e.Path, Kind: command.KindLocalData, DaemonPath: e.Meta.DaemonPath})
		}
	}
	for _, e := range command.ListLocal() {
		add(command.CommandRegistration{Path: e.Path, Kind: command.KindLocal, DaemonPath: e.Meta.DaemonPath})
		if e.Meta.DaemonPath != "" {
			daemonPaths = append(daemonPaths, e)
		}
	}
	for _, r := range pluginreg.All() {
		for _, decl := range r.Commands {
			if yangPaths[strings.ToLower(decl.Name)] {
				continue // a YANG node: classified by its builtin registration
			}
			add(command.CommandRegistration{Path: decl.Name, Kind: command.KindPlugin, Hidden: decl.Hidden})
		}
	}
	add(command.CommandRegistration{Path: surfaceFixtureVisible, Kind: command.KindPlugin})
	add(command.CommandRegistration{Path: surfaceFixtureHidden, Kind: command.KindPlugin, Hidden: true})
	for _, prefix := range pluginserver.StreamingPrefixes() {
		if !seen[strings.ToLower(prefix)] {
			t.Errorf("streaming command %q has no one-shot registration, so no kind answers its surfaces", prefix)
		}
	}
	for _, v := range views {
		add(viewRegistration(v))
	}
	for _, e := range daemonPaths {
		if !seen[strings.ToLower(e.Meta.DaemonPath)] {
			t.Errorf("local command %q declares daemon path %q, which no registry holds, so no daemon surface serves it", e.Path, e.Meta.DaemonPath)
		}
	}
	if len(regs) == 0 {
		t.Fatal("no command was enumerated: the registries are empty")
	}
	return regs
}

// viewRegistration is a CLI live view as the check classifies it when no other
// registry holds its prefix: a view, TUI-only when its registration says so.
func viewRegistration(v cli.ViewInfo) command.CommandRegistration {
	return command.CommandRegistration{Path: v.Prefix, Kind: command.KindView, TUIOnly: v.TUIOnly}
}

// TestStreamingOneShotAnswersCarryData proves AC-9: a streaming command a
// one-shot surface offers (declared pluginserver.OneShotSample) answers one
// sample there, never an echo of its request. Method: every streaming prefix
// is read from the registry, its one-shot route is dispatched through the
// dispatcher every one-shot surface uses, and the answer must be a success
// carrying data. A payload that
// carries its own "status" word restates the response status rather than
// carrying data, which is the shape every monitor command answered before
// (`{"status":"monitor-configured"}`). A streaming-only command is judged by
// TestEveryCommandOnEverySurface instead: no one-shot surface may offer it.
//
// Each command is called with a valid argument for every mandatory leaf its
// YANG model declares, derived from the leaf's type (sampleArguments), so a
// command is judged on a call it can answer: an answer that refuses or fails
// is not a sample. The test runs in a user and network namespace of its own
// (userns.Enter), where it is root over its own loopback, so a probe command
// can open its ICMP socket without privilege on the host.
//
// A stream whose data a plugin serves declares the plugin command it forwards
// to (pluginserver.StreamingPluginCommand). The plugin registration that
// declares that command is found in the plugin registry (pluginOwning) and
// started in-process, with its dependencies and an empty section under each of
// its config roots, the way the daemon starts a plugin whose root the config
// holds. The sample then comes from that plugin's own producer, and with the
// plugin absent the refusal fails the test: a refusal is not data.
func TestStreamingOneShotAnswersCarryData(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	server := startStreamingOwners(t)
	schema, err := yangloader.DefaultLoader()
	if err != nil {
		t.Fatalf("yang loader: %v", err)
	}
	argDefs := yangloader.PathToArgDefs(schema)
	samples := 0
	for _, prefix := range pluginserver.StreamingPrefixes() {
		oneShot, ok := pluginserver.StreamingOneShot(prefix)
		if !ok {
			t.Fatalf("streaming prefix %q has no registration", prefix)
		}
		if oneShot == pluginserver.OneShotStreamingOnly {
			continue
		}
		samples++
		call := prefix + sampleArguments(t, prefix, argDefs[prefix])
		resp, err := serverDispatcher(server, "")(t.Context(), plugin.CallerIdentity{Username: "surface-check"}, call)
		if err != nil {
			t.Errorf("%q one-shot: %v", call, err)
			continue
		}
		if failure := oneShotWithoutData(resp); failure != "" {
			t.Errorf("%q declares a one-shot sample, but %q %s", prefix, call, failure)
		}
	}
	if samples == 0 {
		t.Fatal("no streaming command declares a one-shot sample: the check proved nothing")
	}
}

// startStreamingOwners answers a started plugin server running, in-process,
// every plugin that serves a command a sample stream declares it forwards to,
// with those plugins' dependencies. A declared command no linked registration
// serves fails the test: the stream could never answer.
func startStreamingOwners(t *testing.T) *pluginserver.Server {
	t.Helper()
	var owners []string
	for _, prefix := range pluginserver.StreamingPrefixes() {
		oneShot, ok := pluginserver.StreamingOneShot(prefix)
		if !ok || oneShot != pluginserver.OneShotSample {
			continue
		}
		forwarded, _ := pluginserver.StreamingPluginCommand(prefix)
		if forwarded == "" {
			continue // the daemon produces this stream itself
		}
		owner := pluginOwning(forwarded)
		if owner == nil {
			t.Fatalf("%q forwards to %q, which no linked plugin registration declares", prefix, forwarded)
		}
		if !slices.Contains(owners, owner.Name) {
			owners = append(owners, owner.Name)
		}
	}
	tree := make(map[string]any)
	for _, name := range owners {
		for _, root := range pluginreg.Lookup(name).ConfigRoots {
			tree[root] = map[string]any{}
		}
	}
	needHolds := zeconfig.ConfigNeedHolds(tree)
	names, err := pluginreg.ResolveDependencies(owners, nil, needHolds)
	if err != nil {
		t.Fatalf("dependencies of %v: %v", owners, err)
	}
	configs := make([]plugin.PluginConfig, 0, len(names))
	for _, name := range names {
		configs = append(configs, plugin.PluginConfig{Name: name, Internal: true})
	}
	provider := zeconfig.NewProvider()
	applyLoadedTreeToProvider(provider, tree)
	server, err := pluginserver.NewServer(&pluginserver.ServerConfig{
		Plugins: configs, ConfigNeedHolds: needHolds,
	}, &reloadTestReactor{tree: tree})
	if err != nil {
		t.Fatalf("server: %v", err)
	}
	manager := pluginmgr.NewManager()
	if err := manager.StartAll(t.Context(), server, provider); err != nil {
		t.Fatalf("start %v: %v", names, err)
	}
	server.SetProcessSpawner(manager)
	t.Cleanup(func() {
		server.Stop()
		if err := manager.StopAll(context.Background()); err != nil {
			t.Errorf("stop %v: %v", names, err)
		}
	})
	if err := server.StartWithContext(t.Context()); err != nil {
		t.Fatalf("start server with %v: %v", names, err)
	}
	if err := server.WaitForStartupComplete(t.Context()); err != nil {
		t.Fatalf("startup of %v: %v", names, err)
	}
	return server
}

// pluginOwning answers the linked plugin registration that declares command
// among the commands it serves, or nil when none does.
func pluginOwning(command string) *pluginreg.Registration {
	for _, reg := range pluginreg.All() {
		for i := range reg.Commands {
			if strings.EqualFold(reg.Commands[i].Name, command) {
				return reg
			}
		}
	}
	return nil
}

// TestStreamingOnlyRefusesOnOneShotSurfaces proves a streaming-only command
// typed in full on a one-shot surface (MCP, REST, gRPC, the web admin
// console) is refused, naming the terminal command that streams it, instead
// of answering a reply that looks like success and carries nothing (the old
// `{"status":"monitor-configured"}`). Method: every prefix the registry holds
// as OneShotStreamingOnly is dispatched through the dispatcher every one-shot
// surface uses.
func TestStreamingOnlyRefusesOnOneShotSurfaces(t *testing.T) {
	server, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, nil)
	if err != nil {
		t.Fatalf("server: %v", err)
	}
	prefixes := pluginserver.StreamingOnlyPrefixes()
	if len(prefixes) == 0 {
		t.Fatal("no streaming command is declared streaming-only: the check proved nothing")
	}
	for _, prefix := range prefixes {
		resp, err := serverDispatcher(server, "")(t.Context(), plugin.CallerIdentity{Username: "surface-check"}, prefix)
		if _, ok := errors.AsType[*pluginserver.StreamingOnlyError](err); !ok {
			t.Errorf("%q one-shot: got error %v and response %+v, want the streaming-only refusal", prefix, err, resp)
			continue
		}
		if !strings.Contains(err.Error(), "'ze "+prefix+"'") {
			t.Errorf("%q refusal %q does not name the command that streams it", prefix, err)
		}
	}
}

// sampleStrings are the values sampleValue offers a string leaf, in order. The
// first is a loopback address: a destination leaf takes it, and the call then
// stays on the test's own loopback.
var sampleStrings = []string{"127.0.0.1", "::1", "lo", "sample", "1"}

// sampleArguments answers ` <leaf> <value>` for every mandatory leaf of defs,
// each value derived from the leaf's own type by sampleValue, so the call is
// one the model accepts. A mandatory leaf whose type yields no value fails the
// test: the call could not be built, and guessing one would test nothing.
func sampleArguments(t *testing.T, prefix string, defs []command.ArgDef) string {
	t.Helper()
	var tb textbuf.Buffer
	for i := range defs {
		if !defs[i].Mandatory() {
			continue
		}
		tb.Byte(' ').Str(defs[i].Name())
		if defs[i].Kind() == command.ArgFlag {
			continue
		}
		value, ok := sampleValue(&defs[i])
		if !ok {
			t.Fatalf("%q: no valid sample value derives from the type of mandatory leaf %q", prefix, defs[i].Name())
		}
		tb.Byte(' ').Str(value)
	}
	return tb.String()
}

// sampleValue answers a value def accepts: the lowest value of a range, the
// first enum value, or the first of sampleStrings the leaf's pattern and
// length admit. Every candidate is judged by command.ValidateArgString, the
// validator every route runs, so an answer is always one the model accepts.
func sampleValue(def *command.ArgDef) (string, bool) {
	candidates := slices.Clone(sampleStrings)
	for r := range def.Ranges() {
		candidates = append([]string{strconv.FormatUint(r.Min, 10)}, candidates...)
		break
	}
	for value := range def.EnumValues() {
		candidates = append([]string{value}, candidates...)
		break
	}
	for _, candidate := range candidates {
		if command.ValidateArgString(candidate, def) == nil {
			return candidate, true
		}
	}
	return "", false
}

// oneShotWithoutData answers why resp carries no sample, or "" when it does.
func oneShotWithoutData(resp *plugin.Response) string {
	if resp == nil {
		return "the dispatcher answered nothing"
	}
	if resp.Status == plugin.StatusError {
		return "it refused a valid call, so it answered no sample: " + resp.Error
	}
	if resp.Data == nil {
		return "it answered no data"
	}
	text, err := json.Marshal(resp.Data)
	if err != nil {
		return "its data does not encode: " + err.Error()
	}
	var record map[string]any
	if json.Unmarshal(text, &record) != nil {
		return "" // a list or a scalar is data
	}
	if _, echo := record["status"]; echo {
		return "it answered a status word, not data: " + string(text)
	}
	return ""
}
