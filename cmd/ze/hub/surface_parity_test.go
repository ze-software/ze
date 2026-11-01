//go:build ze_ssh && ze_web

package hub

import (
	"bufio"
	"os"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config/infra"
	yangloader "github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/component/plugin"
	_ "github.com/ze-software/ze/internal/component/plugin/all"
	"github.com/ze-software/ze/internal/component/plugin/process"
	pluginreg "github.com/ze-software/ze/internal/component/plugin/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/plugins/completion"
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
// VALIDATES: AC-1, AC-2 and AC-3 of spec-command-surface-parity.
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
	regs := collectCommandRegistrations(t, schema)
	arms := surfaceArms(t, server)

	for _, reg := range regs {
		for _, s := range command.Surfaces() {
			want, err := command.OffersCommand(reg, s)
			if err != nil {
				t.Errorf("%v", err)
				continue
			}
			arm, ok := arms[s]
			if !ok {
				continue // reported once by surfaceArms
			}
			listed := arm.listed(reg.Path)
			served := arm.served(reg.Path)
			if want == (listed && served) {
				continue
			}
			switch {
			case want && !listed:
				t.Errorf("%q on %s (%s): must be offered, the surface does not list it", reg.Path, s, reg.Kind)
			case want:
				t.Errorf("%q on %s (%s): listed, but the daemon dispatcher cannot serve it", reg.Path, s, reg.Kind)
			default:
				t.Errorf("%q on %s (%s, hidden=%v): must not be offered, the surface offers it", reg.Path, s, reg.Kind, reg.Hidden)
			}
		}
	}
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

	// Web admin: the tree service_web.go hands HandleAdminView and
	// HandleAdminExecute, built once from the loaded modules.
	adminSchema, err := yangloader.DefaultLoader()
	if err != nil {
		t.Fatalf("web admin command tree: %v", err)
	}
	adminTree := yangloader.BuildCommandTree(adminSchema)

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

	// `ze cli`: buildRuntimeTreeFromDispatch (internal/component/cli/client)
	// is unexported, so this package cannot ask it.
	t.Errorf("surface %s: its producer buildRuntimeTreeFromDispatch is unexported, so the check cannot ask it", command.SurfaceCLI)

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
	t    *testing.T
	seen map[string][]string
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
	for _, c := range candidates {
		if c == words[len(words)-1] {
			return true
		}
	}
	return false
}

// ask runs completion.Run, which writes its records to os.Stdout, with stdout
// pointed at a file, and returns the first field of each record.
func (w *shellWords) ask(prefix []string) []string {
	w.t.Helper()
	out, err := os.CreateTemp(w.t.TempDir(), "words")
	if err != nil {
		w.t.Fatalf("shell completion capture: %v", err)
	}
	defer out.Close() //nolint:errcheck // a read-only capture file

	saved := os.Stdout
	os.Stdout = out
	code := completion.Run(append([]string{"words", "run"}, prefix...))
	os.Stdout = saved
	if code != 0 {
		w.t.Errorf("ze completion words run %s: exit %d", strings.Join(prefix, " "), code)
		return nil
	}
	if _, err := out.Seek(0, 0); err != nil {
		w.t.Fatalf("shell completion capture: %v", err)
	}
	var words []string
	scan := bufio.NewScanner(out)
	for scan.Scan() {
		word, _, _ := strings.Cut(scan.Text(), "\t")
		if word != "" {
			words = append(words, word)
		}
	}
	if err := scan.Err(); err != nil {
		w.t.Fatalf("shell completion capture: %v", err)
	}
	return words
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
func collectCommandRegistrations(t *testing.T, schema *yangloader.Resolved) []command.CommandRegistration {
	t.Helper()
	var regs []command.CommandRegistration
	seen := make(map[string]bool)
	add := func(reg command.CommandRegistration) {
		key := strings.ToLower(reg.Path)
		if seen[key] {
			return
		}
		seen[key] = true
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
	for _, e := range command.ListLocal() {
		if command.HasLocalData(e.Path) {
			add(command.CommandRegistration{Path: e.Path, Kind: command.KindLocalData})
		}
	}
	for _, e := range command.ListLocal() {
		add(command.CommandRegistration{Path: e.Path, Kind: command.KindLocal})
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
	for _, v := range cli.RegisteredViews() {
		add(command.CommandRegistration{Path: v.Prefix, Kind: command.KindView})
	}
	if len(regs) == 0 {
		t.Fatal("no command was enumerated: the registries are empty")
	}
	return regs
}
