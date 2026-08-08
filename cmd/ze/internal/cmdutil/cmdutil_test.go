// Design: docs/architecture/api/commands.md -- cmdutil tests

package cmdutil

import (
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"

	cli "github.com/ze-software/ze/internal/component/cli/client"
	"github.com/ze-software/ze/internal/component/command/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// localAtStartup is the local-handler registry as the binary's init() functions
// left it, captured before any test runs.
//
// registry.ResetForTest clears EVERY registration, production ones included, and
// nothing puts them back: the registry has no unregister and no restore. Several
// tests here need a synthetic registration and use it to clean up, so a test that
// reads registry.ListLocal() gets whatever the tests before it happened to leave.
// Snapshotting in TestMain makes that ordering irrelevant.
var localAtStartup []registry.LocalCommandEntry

func TestMain(m *testing.M) {
	localAtStartup = registry.ListLocal()
	os.Exit(m.Run())
}

// declaredCommandPaths returns every absolute CLI path a registered built-in
// declares, deduplicated and skipping bare verbs. It reads the two registries
// BuildVerbCommandTree reads, so the expected set is DERIVED rather than a
// second hardcoded copy (ai/rules/evidence.md).
func declaredCommandPaths() []string {
	wireToPaths := cli.WireToPaths()
	seen := make(map[string]bool)
	var out []string
	for _, reg := range cli.AllCLIRPCs() {
		for _, path := range wireToPaths[reg.WireMethod] {
			if seen[path] || !strings.Contains(path, " ") {
				continue
			}
			seen[path] = true
			out = append(out, path)
		}
	}
	return out
}

// VALIDATES: every ze:command a built-in declares still resolves when a shell
// hands `ze` the whole path, verb included.
// PREVENTS: the tree/argv misalignment that made `ze show bgp rib status`
// answer `unknown command`. cli.BuildVerbCommandTree is RELATIVE to the verb
// (`bgp rib status`) while argv is absolute (`show bgp rib status`), and
// RunCommand walked the relative tree with the absolute words. 56 of the 63
// `ze show` commands were unreachable, and every other verb with them; the
// daemon and the interactive CLI resolve on a different path, so no functional
// test saw it. Interop scenarios 05, 13 and 35 were red on the `show bgp rib
// status` row alone.
//
// This drives ResolveCommand rather than IsValidCommand: a test that strips the
// verb itself would encode the correct alignment and pass against the defect.
func TestDeclaredCommandsResolveFromArgv(t *testing.T) {
	paths := declaredCommandPaths()
	if len(paths) < 100 {
		t.Fatalf("declared command paths = %d, want >= 100: the registry is empty, so this test proves nothing", len(paths))
	}
	declared := make(map[string]bool, len(paths))
	for _, path := range paths {
		declared[path] = true
	}

	for _, path := range paths {
		argv := strings.Fields(path)
		verb := argv[0]

		res, ok := ResolveCommand(argv, verb)
		if !ok {
			t.Errorf("ResolveCommand(%q) refused to resolve", path)
			continue
		}
		if !res.Valid {
			t.Errorf("`ze %s` does not resolve: relative words %q are not in the %q tree", path, res.Relative, verb)
			continue
		}
		if !res.Declared {
			t.Errorf("`ze %s` resolved but is not declared: no registered command owns it", path)
			continue
		}
		if got := textbuf.Join(res.Path, " "); got != path {
			t.Errorf("`ze %s` dispatches on %q, want %q: the daemon is keyed on the absolute path", path, got, path)
			continue
		}

		// Second walk: every read-only command rooted under ANOTHER verb is also
		// reachable as `ze show <path>`, because verbContextPath carries it into
		// the show tree unchanged. AbsoluteVerbPath must carry it back out
		// unchanged rather than prefixing `show`. Driving every such path here,
		// rather than one hand-picked `monitor ping`, is what makes the
		// inversion provable: the set is DERIVED from IsReadOnlyPath, the same
		// predicate verbContextPath consults.
		if verb == "show" || !pluginserver.IsReadOnlyPath(path) {
			continue
		}
		underShow := append([]string{"show"}, argv...)
		res, ok = ResolveCommand(underShow, "show")
		if !ok || !res.Valid {
			t.Errorf("`ze show %s` does not resolve (ok=%v valid=%v): a read-only command must stay reachable under show", path, ok, res.Valid)
			continue
		}
		got := textbuf.Join(res.Path, " ")
		if got == path {
			continue
		}
		// A second registration may declare `show <path>` outright, which then
		// occupies the same node of the show tree. `system subsystem list`
		// (ze-system:subsystem-list) and `show system subsystem list`
		// (ze-show:system-subsystem-list) are the live pair. Dispatching to the
		// declared show-rooted command is right there; prefixing `show` onto a
		// path nothing declares is the inversion this walk exists to catch.
		if declared[got] && got == "show "+path {
			continue
		}
		t.Errorf("`ze show %s` dispatches on %q, want %q: the verb must not be prefixed to a carried path", path, got, path)
	}
}

// valueTakingCommands returns every node of the `show` tree that declares its
// own arguments and has no children, as a verb-relative path. That shape is the
// one the walk used to abandon, so the set is DERIVED from the live tree rather
// than listed: a value-taking command registered tomorrow is covered the day it
// registers, and a hardcoded list would go on passing without it.
func valueTakingCommands(t *testing.T) [][]string {
	t.Helper()
	var out [][]string
	var walk func(node *cli.Command, rel []string)
	walk = func(node *cli.Command, rel []string) {
		if len(rel) > 0 && len(node.Children) == 0 && len(node.ArgDefs) > 0 {
			out = append(out, rel)
		}
		for name, child := range node.Children {
			walk(child, append(append([]string{}, rel...), name))
		}
	}
	walk(cli.BuildVerbCommandTree("show"), nil)
	sort.Slice(out, func(i, j int) bool {
		return strings.Join(out[i], " ") < strings.Join(out[j], " ")
	})
	return out
}

// VALIDATES: a command whose grammar ends in a positional VALUE resolves from a
// shell argv, and the daemon is asked to run the line exactly as it was typed.
// PREVENTS: the walk giving up at a node with no children before it consults
// that node's ArgDefs. `ze show pki certificate name web` answered `unknown
// command`, and so did every other command ending in a value: nothing declares
// the typed path, no fallback covers it, and only `show crashes` and `show host`
// looked healthy because an offline fallback answered them.
//
// The inline subtest is the contrast that makes this discriminating. That shape
// WORKED throughout, because its selector sits mid-path where the node still has
// children, and it is the one shape whose dispatch REORDERS. A repair that
// lifted trailing values by making everything trailing would go green above and
// red there.
func TestTrailingValueCommandsResolveFromArgv(t *testing.T) {
	const value = "a-value"

	paths := valueTakingCommands(t)
	if len(paths) < 20 {
		t.Fatalf("value-taking commands in the `show` tree = %d, want >= 20: the tree is empty, so this test proves nothing", len(paths))
	}

	covered := 0
	for _, rel := range paths {
		argv := append([]string{"show"}, rel...)
		base, ok := ResolveCommand(argv, "show")
		if !ok || !base.Valid || !base.Declared {
			// The command itself does not resolve, which is a different defect
			// with its own test (TestDeclaredCommandsResolveFromArgv).
			continue
		}
		covered++
		t.Run(strings.Join(rel, " "), func(t *testing.T) {
			withValue := append(append([]string{}, argv...), value)
			res, ok := ResolveCommand(withValue, "show")
			if !ok || !res.Valid || !res.Declared {
				t.Fatalf("`ze %s` does not resolve (ok=%v valid=%v declared=%v): a trailing value must not cost the command its path", strings.Join(withValue, " "), ok, res.Valid, res.Declared)
			}
			want := textbuf.Join(base.Path, " ") + " " + value
			if got := res.DispatchString(); got != want {
				t.Errorf("dispatch = %q, want %q: a trailing value is passed through in the position it was typed", got, want)
			}
		})
	}
	if covered < 20 {
		t.Errorf("declared value-taking commands exercised = %d, want >= 20", covered)
	}

	t.Run("inline selector still reorders", func(t *testing.T) {
		res, ok := ResolveCommand([]string{"show", "bgp", "peer", "edge1", "detail"}, "show")
		if !ok || !res.Valid || !res.Declared {
			t.Fatalf("`ze show bgp peer edge1 detail` does not resolve (ok=%v valid=%v declared=%v)", ok, res.Valid, res.Declared)
		}
		const want = "show bgp peer detail edge1"
		if got := res.DispatchString(); got != want {
			t.Errorf("dispatch = %q, want %q: an inline selector moves to the end, the daemon is keyed on the action word", got, want)
		}
	})
}

// valuePlaceholder matches a positional placeholder as the YANG descriptions
// spell one: `<ip>`, `<peer-name>`, `<link|area|as>`, `<128-255>`.
var valuePlaceholder = regexp.MustCompile(`<[a-zA-Z0-9_ .|/-]+>`)

// placeholderValueCommands returns every ze:command whose YANG DESCRIPTION
// shows a positional placeholder, as an absolute CLI path.
//
// THIS SET IS INDEPENDENT OF THE OLD KEY AND NOT OF THE NEW ONE. Say it plainly
// rather than claim otherwise, because a reader deciding what this test is worth
// needs the real answer.
//
// Independent of the OLD key: the previous rule ended a path where ArgDefs said
// it ended, and ArgDefs are derived from ze:command LEAVES. A description is
// prose a human wrote beside the grammar; BuildCommandTree
// (internal/component/config/yang/command.go) copies it from the YANG
// `description` statement and reads no leaf and no ArgDef. That is why this set
// caught what a round of 44/44 green over an ArgDefs-derived set could not: every
// leafless command was excluded from that set and the repair alike.
//
// NOT independent of the NEW key, and the filter is where it goes. The walk
// keeps a node only when registered[node.WireMethod] holds, over
// cli.YANGCommandTree -- the same population cliWireToPaths carries and
// cli.AbsoluteVerbPath scans. So res.Declared is true by construction for every
// case here: this test can prove the value ARRIVES and the dispatch string is
// rebuilt, and it cannot prove that the declaration verdict itself is right.
// What can is the daemon, asked directly: test/ui/cli-verb-daemon-dispatch.ci
// checks 11 and 14 run the same argv as `ze <verb> ...` and as `ze cli -c`
// against one live daemon and require the two answers to agree.
//
// The filter is not droppable. A wire method this binary's build tags left out
// has no handler to reach at all, so keeping it would fail the test on a fact
// about the build rather than about the resolver.
func placeholderValueCommands(t *testing.T) []string {
	t.Helper()

	registered := make(map[string]bool)
	for _, reg := range cli.AllCLIRPCs() {
		registered[reg.WireMethod] = true
	}

	var out []string
	var walk func(node *cli.Command, path []string)
	walk = func(node *cli.Command, path []string) {
		if len(path) > 0 && node.WireMethod != "" && registered[node.WireMethod] &&
			valuePlaceholder.MatchString(node.Description) {
			out = append(out, strings.Join(path, " "))
		}
		for name, child := range node.Children {
			walk(child, append(append([]string{}, path...), name))
		}
	}
	walk(cli.YANGCommandTree(), nil)
	sort.Strings(out)
	return out
}

// VALIDATES: a command whose description promises a positional value accepts
// that value from a shell argv, and dispatches on the declared path with the
// value behind it.
// PREVENTS: ExtractValues deciding where a path ends by a rule the daemon does
// not use. matchCommandTokens (internal/component/plugin/server/command.go)
// matches a registered key as a PREFIX and hands tokens[inIdx:] to the handler,
// consulting no ArgDefs to find the boundary. ExtractValues consulted ArgDefs,
// which are derived from ze:command LEAVES, so the 30 declared commands whose
// YANG declares no leaf lost their value to treeWords: IsValidCommand then
// rejected the path and the operator got `unknown command` for a line
// `ze cli -c` ran. handleRouteLookup (internal/component/iface/cmd/
// show_route_lookup.go) reads args[0] while `container lookup`
// (internal/component/iface/yang/ze-iface-show-cmd.yang) declares no leaf, so
// `ze show route lookup 1.2.3.4` was unreachable while the same words through
// `ze cli -c` were not.
func TestDescribedValueCommandsAcceptTheirValue(t *testing.T) {
	const value = "a-value"

	paths := placeholderValueCommands(t)
	if len(paths) < 25 {
		t.Fatalf("commands whose description shows a value placeholder = %d, want >= 25: the YANG tree is empty, so this test proves nothing", len(paths))
	}

	exercised := 0
	for _, path := range paths {
		for _, form := range verbForms(path) {
			exercised++
			t.Run(strings.Join(form.argv, " "), func(t *testing.T) {
				base, ok := ResolveCommand(form.argv, form.verb)
				if !ok || !base.Valid || !base.Declared {
					t.Fatalf("`ze %s` does not resolve on its own (ok=%v valid=%v declared=%v)", strings.Join(form.argv, " "), ok, base.Valid, base.Declared)
				}
				if got := textbuf.Join(base.Path, " "); got != path {
					t.Fatalf("dispatch path = %q, want %q", got, path)
				}

				withValue := append(append([]string{}, form.argv...), value)
				res, ok := ResolveCommand(withValue, form.verb)
				if !ok || !res.Valid || !res.Declared {
					t.Fatalf("`ze %s` does not resolve (ok=%v valid=%v declared=%v): the description promises a value, so the value must not cost the command its path", strings.Join(withValue, " "), ok, res.Valid, res.Declared)
				}
				if got, want := res.DispatchString(), path+" "+value; got != want {
					t.Errorf("dispatch = %q, want %q", got, want)
				}
			})
		}
	}
	if exercised < 25 {
		t.Errorf("verb forms exercised = %d, want >= 25", exercised)
	}
}

// verbForm is one argv shape that reaches an absolute CLI path.
type verbForm struct {
	verb string
	argv []string
}

// verbForms returns every argv RunCommand can be handed for one absolute path.
//
// Two exist, and verbContextPath (internal/component/cli/client/verb_tree.go)
// owns both conditions, so this reads them rather than restating them: a command
// rooted under its own verb is reached as `ze <verb> <rest>`, and a read-only
// command rooted under ANOTHER verb is ALSO reached as `ze show <path>` whole.
//
// A single-word path yields no form. `announce <unicast|blackhole|flowspec>`
// and `withdraw tag <key> <value>` are declared with a value placeholder and
// have no verb-relative remainder at all, so `ze announce ...` is a root
// dispatch and never enters ResolveCommand.
//
// The reachability test is FindNode over the base path, which walks tree
// children only. It does not ask whether anything declares the path, so a
// resolver that wrongly forgot a declaration cannot make its own case
// disappear from this list.
func verbForms(path string) []verbForm {
	words := strings.Fields(path)
	var out []verbForm
	if len(words) > 1 {
		if FindNode(words[1:], cli.BuildVerbCommandTree(words[0])) != nil {
			out = append(out, verbForm{verb: words[0], argv: words})
		}
	}
	if words[0] != "show" && pluginserver.IsReadOnlyPath(path) {
		if FindNode(words, cli.BuildVerbCommandTree("show")) != nil {
			out = append(out, verbForm{verb: "show", argv: append([]string{"show"}, words...)})
		}
	}
	return out
}

// VALIDATES: every command a plugin registers a LOCAL handler for is still
// reached from a shell argv, with and without a trailing value.
// PREVENTS: the daemon's path boundary being applied to the local-handler
// lookup, whose keys are a different namespace. `show debug` is a declared
// ze:command and `profile` is no node of the show tree, so that boundary ends
// `ze show debug profile name default` at `show debug` -- a key no local handler
// holds. runShowProfile (internal/plugins/debug/register.go registers it at
// `show debug profile`) became unreachable and the command went to the daemon,
// which answered `no credentials`.
//
// THE CASES COME FROM registry.ListLocal, A THIRD SOURCE. The repair keys on
// cli.AbsoluteVerbPath (the RPC registry) and the sibling test above keys on
// YANG descriptions; neither can see a local-only path, which is exactly what
// this one broke. The local registry is populated by each plugin's register.go
// and reads no YANG at all.
//
// THIS BINARY CANNOT SEE EVERY REGISTRATION, AND THE SYNTHETIC CASE IS WHY IT
// IS HERE. `internal/plugins/debug`, which owns the path that actually broke, is
// imported by cmd/ze and cmd/ze imports this package, so linking it here is an
// import cycle. The 13 registrations reachable from this package all sit under a
// tree node with children, so none of them reproduces the shape. The synthetic
// subtest registers one that does, and the live path is covered end to end by
// test/ui/cli-verb-daemon-dispatch.ci and test/ui/debug-enable-show.ci.
func TestRegisteredLocalCommandsStayReachable(t *testing.T) {
	entries := registry.ListLocal()
	if len(entries) < 10 {
		t.Fatalf("registered local commands = %d, want >= 10: the registry is empty, so this test proves nothing", len(entries))
	}

	exercised := 0
	for _, entry := range entries {
		words := strings.Fields(entry.Path)
		if len(words) < 2 {
			continue // a bare verb is a root dispatch, not a verb dispatch
		}
		exercised++
		t.Run(entry.Path, func(t *testing.T) {
			for _, argv := range [][]string{words, append(append([]string{}, words...), "a-value")} {
				res, _ := ResolveCommand(argv, words[0])
				handler, _ := matchLocalHandler(res.Local, res.LocalValues)
				if handler == nil {
					t.Errorf("`ze %s` reaches no local handler: Local=%q LocalValues=%q, but %q is registered", strings.Join(argv, " "), res.Local, res.LocalValues, entry.Path)
				}
			}
		})
	}
	if exercised < 10 {
		t.Errorf("multi-word local commands exercised = %d, want >= 10", exercised)
	}

	t.Run("under a declared leaf the tree does not extend", func(t *testing.T) {
		defer registry.ResetForTest()

		leaf := declaredChildlessNode(t)
		path := textbuf.Join(leaf, " ") + " profile"
		registry.MustRegisterLocal(path, func(_ []string) int { return 0 })

		argv := append(append([]string{}, leaf...), "profile", "name", "default")
		res, _ := ResolveCommand(argv, leaf[0])
		handler, args := matchLocalHandler(res.Local, res.LocalValues)
		if handler == nil {
			t.Fatalf("`ze %s` reaches no local handler: Local=%q LocalValues=%q, but %q is registered", strings.Join(argv, " "), res.Local, res.LocalValues, path)
		}
		if len(args) != 2 || args[0] != "name" || args[1] != "default" {
			t.Errorf("handler args = %q, want [name default]", args)
		}
	})
}

// shadowedDeclaredChildren returns, for every LOCAL registration this binary
// holds, the absolute paths of the ze:commands declared BELOW it -- the paths a
// longest-prefix local lookup captures without being registered for them.
//
// THE SELECTION SOURCE IS THE AUTHORED YANG TREE, NOT THE RPC REGISTRY THE FIX
// KEYS ON. cli.IsDeclaredCommand, which is what registry.LookupLocal asks, reads
// AllCLIRPCs x cliWireToPaths. cli.YANGCommandTree is yang.BuildCommandTree over
// the module text: a node carries WireMethod because a `ze:command` statement is
// written on its container, and no registration is consulted. A child this
// binary's build tags left unregistered is therefore still listed here, and the
// case fails rather than disappearing -- which is the whole point, because a
// child nothing serves must still not be answered by an interface-name lookup.
func shadowedDeclaredChildren(t *testing.T) map[string][]string {
	t.Helper()

	out := make(map[string][]string)
	for _, entry := range localAtStartup {
		node := cli.YANGCommandTree()
		for word := range strings.FieldsSeq(entry.Path) {
			if node == nil {
				break
			}
			node = node.Children[word]
		}
		if node == nil {
			continue // local-only path, nothing in YANG below it
		}
		var walk func(n *cli.Command, path []string)
		walk = func(n *cli.Command, path []string) {
			if len(path) > 0 && n.WireMethod != "" {
				out[entry.Path] = append(out[entry.Path], entry.Path+" "+strings.Join(path, " "))
			}
			for name, child := range n.Children {
				walk(child, append(append([]string{}, path...), name))
			}
		}
		walk(node, nil)
		sort.Strings(out[entry.Path])
	}
	return out
}

// VALIDATES: a command declared BELOW a locally registered path is not answered
// by that path's handler, whether it is typed bare or with a value after it.
// PREVENTS: longest-prefix local lookup handing a short registration the whole
// subtree below it. `show interface` is registered at two words
// (internal/component/iface/cli/register.go) while ze-iface-interface-cmd.yang
// declares seven commands under it, so all seven reached cmdShow
// (internal/component/iface/cli/show.go), which reads args[0] as an interface
// NAME: `ze show interface brief` looked for an interface called "brief", and
// the same for scan, type, errors, rate, name <n> detail and name <n> counters.
// Every one of them is published in docs/guide/command-reference.md.
//
// THIS IS THE FAST PIN, NOT THE INDEPENDENT ORACLE. It asserts only that the
// local handler is refused; what the operator then GETS is the daemon's answer,
// and only the daemon can be asked for it. test/ui/cli-verb-daemon-dispatch.ci
// check 14 runs each of these argvs against a live daemon twice, once as
// `ze show interface <child>` and once as `ze cli -c "show interface <child>"`,
// and requires the two to agree.
func TestLocalHandlerDoesNotSwallowDeclaredChildren(t *testing.T) {
	shadowed := shadowedDeclaredChildren(t)
	total := 0
	for _, children := range shadowed {
		total += len(children)
	}
	if total == 0 {
		t.Fatal("no local registration in this binary has a ze:command declared below it: the shape this test pins cannot be built, so it proves nothing")
	}

	for local, children := range shadowed {
		for _, child := range children {
			words := strings.Fields(child)
			for _, argv := range [][]string{words, append(append([]string{}, words...), "a-value")} {
				t.Run(strings.Join(argv, " "), func(t *testing.T) {
					res, _ := ResolveCommand(argv, words[0])
					handler, args := matchLocalHandler(res.Local, res.LocalValues)
					if handler != nil {
						t.Errorf("`ze %s` reached the handler registered at %q with args %q: a declared command was answered by the registration above it", strings.Join(argv, " "), local, args)
					}
					if !res.Declared {
						t.Errorf("`ze %s` does not resolve to a declared path (Path=%q): refusing the local handler leaves it with nothing to dispatch", strings.Join(argv, " "), res.Path)
					}
				})
			}
		}
	}
}

// declaredChildlessNode returns the absolute path of a `show` tree node that
// ends a declared command and has no children, which is the shape a local-only
// subcommand hangs below. The three properties are the search predicate, so a
// tree that stops holding such a node fails the test rather than passing over
// nothing.
func declaredChildlessNode(t *testing.T) []string {
	t.Helper()
	tree := cli.BuildVerbCommandTree("show")
	names := make([]string, 0, len(tree.Children))
	for name := range tree.Children {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		child := tree.Children[name]
		if len(child.Children) > 0 {
			continue
		}
		if abs, declared := cli.AbsoluteVerbPath("show", []string{name}); declared {
			return abs
		}
	}
	t.Fatal("no declared childless node in the `show` tree: the shape a local-only subcommand hangs below can no longer be built, so this test proves nothing")
	return nil
}

// VALIDATES: an offline fallback registered on an UNDECLARED grouping container
// makes that container dispatchable, so RunCommand serves it instead of printing
// its subcommands and exiting 1.
//
// THE FALLBACK REGISTRATION HERE IS SYNTHETIC, AND SO IS THE SHAPE IT PINS. No
// production registration has all three of (no ze:command, children in the verb
// tree, a registered offline fallback), so do not go hunting for the real one.
// Both live fallbacks cover a DECLARED path and reach Dispatchable through its
// Declared branch instead:
//
//	`show host`    internal/plugins/host/register.go     -> ze-show:host-all
//	`show crashes` internal/plugins/crashes/register.go  -> ze-show:crashes
//
// The fallback branch of Resolution.Dispatchable is therefore unreachable from
// today's registrations. It is kept as insurance for the next plugin that
// registers one before declaring its path, and this test is what keeps it
// working. Checked 2026-08-08.
//
// PREVENTS: the regression that made RunShow (internal/plugins/host/host.go)
// unreachable through `ze show host` when `container host` still declared no
// ze:command. RunCommand gated its subcommand-list branch on Declared alone,
// printed the list and exited 1, and the fallback lookup two lines above it
// changed nothing.
//
// test-relax: the Valid / not-Declared / has-children assertions this test used
// to make against the hardcoded `show host` are not dropped. They are the
// SELECTION PREDICATE of undeclaredGroupingContainer below, which fails the test
// when no node in the live tree satisfies them. `show host` gained a ze:command
// on 2026-08-08, so a hardcoded path here would now assert about a case it no
// longer covers.
func TestSyntheticOfflineFallbackBeatsGroupingContainer(t *testing.T) {
	defer registry.ResetForTest()

	path, res := undeclaredGroupingContainer(t)
	joined := strings.Join(path, " ")
	t.Logf("grouping container under test: `%s`", joined)

	if res.Dispatchable() {
		t.Fatalf("`%s` is dispatchable with no fallback registered", joined)
	}

	registry.MustRegisterOfflineFallback(joined, func(_ []string) int { return 0 })
	if !res.Dispatchable() {
		t.Errorf("`%s` is not dispatchable with an offline fallback registered: RunCommand will print its subcommands and exit 1 instead of serving it", joined)
	}

	// The fallback is longest-prefix, so it also covers a trailing argument an
	// operator types with no daemon running.
	sub, ok := ResolveCommand(append(append([]string{}, path...), "no-such-child"), "show")
	if !ok {
		t.Fatalf("`ze %s no-such-child` does not resolve", joined)
	}
	if !sub.Dispatchable() {
		t.Errorf("`ze %s no-such-child` is not dispatchable: the fallback lookup is not longest-prefix", joined)
	}
}

// undeclaredGroupingContainer finds a `show` subtree node that has children and
// that no registered command declares, and returns its absolute path with the
// resolution for it. The three properties the caller needs -- resolves, Valid,
// not Declared, children present -- are the search predicate, so a tree that
// stops holding such a node fails the test rather than passing over nothing.
func undeclaredGroupingContainer(t *testing.T) ([]string, Resolution) {
	t.Helper()
	tree := cli.BuildVerbCommandTree("show")
	var walk func(node *cli.Command, rel []string) ([]string, Resolution, bool)
	walk = func(node *cli.Command, rel []string) ([]string, Resolution, bool) {
		names := make([]string, 0, len(node.Children))
		for name := range node.Children {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			child := node.Children[name]
			if len(child.Children) == 0 {
				continue
			}
			childRel := append(append([]string{}, rel...), name)
			argv := append([]string{"show"}, childRel...)
			if res, ok := ResolveCommand(argv, "show"); ok && res.Valid && !res.Declared {
				return argv, res, true
			}
			if p, res, ok := walk(child, childRel); ok {
				return p, res, true
			}
		}
		return nil, Resolution{}, false
	}
	path, res, ok := walk(tree, nil)
	if !ok {
		t.Fatal("no undeclared grouping container in the `show` tree: the fallback branch of Resolution.Dispatchable can no longer be exercised, so either delete it or restore a case that reaches it")
	}
	return path, res
}

// VALIDATES: a local handler registered at a path whose last word is a format
// keyword is still reached.
// PREVENTS: ResolveCommand's `every word was the format keyword` early return
// shadowing the local registry. ResolveCommand answers ok=false there, and
// RunCommand consults matchLocalHandler BEFORE that verdict for exactly this
// case; checking ok first would answer `unknown show command: json`.
func TestLocalHandlerWithFormatKeywordPathStillRuns(t *testing.T) {
	defer registry.ResetForTest()

	called := false
	if err := RegisterLocalCommand("show json", func(_ []string) int {
		called = true
		return 7
	}); err != nil {
		t.Fatal(err)
	}

	if code := RunCommand([]string{"show", "json"}, "show"); code != 7 {
		t.Errorf("RunCommand(`ze show json`) = %d, want 7", code)
	}
	if !called {
		t.Error("the local handler registered at `show json` was never called")
	}
}

// VALIDATES: three commands the interop harness names resolve from a shell
// argv, one by one so a failure says which one.
//
// IT DOES NOT PROTECT THE HARNESS, AND NOTHING HERE DOES. Every scenario query
// now goes through Ze.cli (test/interop/interop.py), which runs `ze cli -c` and
// therefore never enters RunCommand at all. `show isis neighbor` is asked of
// FRR's vtysh, not of ze, and `show bgp peer list` appears nowhere under
// test/interop/. Only `show bgp rib status` is a live harness query, and it
// reaches the daemon by the `ze cli -c` path this test does not exercise.
//
// So read this as three named paths kept resolving, no more. A regression in
// the verb form would go unnoticed by the interop suite; what covers the verb
// form end to end is test/ui/cli-verb-daemon-dispatch.ci.
func TestInteropHarnessCommandsResolve(t *testing.T) {
	for _, path := range []string{
		"show bgp rib status",
		"show bgp peer list",
		"show isis neighbor",
	} {
		t.Run(path, func(t *testing.T) {
			res, ok := ResolveCommand(strings.Fields(path), "show")
			if !ok || !res.Valid || !res.Declared {
				t.Fatalf("`ze %s` does not resolve (ok=%v valid=%v declared=%v)", path, ok, res.Valid, res.Declared)
			}
			if got := textbuf.Join(res.Path, " "); got != path {
				t.Errorf("dispatch path = %q, want %q", got, path)
			}
		})
	}
}

// VALIDATES: a read-only command rooted under another verb keeps its own root
// when it is reached through `ze show`.
// PREVENTS: rebuilding the absolute path by always prefixing the verb, which
// would send `show monitor ping` to a daemon keyed on `monitor ping`.
// verbContextPath carries such a path into the show tree unchanged, so
// AbsoluteVerbPath must carry it back out unchanged.
func TestReadOnlyCommandUnderShowKeepsItsRoot(t *testing.T) {
	res, ok := ResolveCommand([]string{"show", "monitor", "ping"}, "show")
	if !ok || !res.Valid {
		t.Fatalf("`ze show monitor ping` does not resolve (ok=%v valid=%v)", ok, res.Valid)
	}
	if got := textbuf.Join(res.Path, " "); got != "monitor ping" {
		t.Errorf("dispatch path = %q, want %q", got, "monitor ping")
	}
}

// VALIDATES: a locally handled command reached through the verb keeps its `show`
// prefix, finds its handler, and hands the handler the flag typed after it.
// PREVENTS: `ze show version` losing its `show` prefix and missing the local
// handler, which is registered under the absolute path; and the flag being
// dropped on the way there.
//
// IT ASSERTS THE CALL, NOT THE INTERMEDIATE SHAPE, and the difference matters
// because res.Local is not where the flag gets separated. extractLocalValues
// passes endsCommand=nil, so the trailing branch never runs on the local path
// and `--extended` stays inside res.Local (["show","version","--extended"]),
// with res.LocalValues empty. What ends the path at the registered key is
// registry.LookupLocal's own longest-prefix match, which returns `--extended` as
// the handler's argument. Asserting res.Local would therefore pin the shape of
// an intermediate that carries the flag either way, and would say nothing about
// whether the handler ever got it.
func TestLocalHandlerUnderVerbGetsItsFlag(t *testing.T) {
	defer registry.ResetForTest()

	var got []string
	if err := RegisterLocalCommand("show version", func(args []string) int {
		got = args
		return 3
	}); err != nil {
		t.Fatal(err)
	}

	if code := RunCommand([]string{"show", "version", "--extended"}, "show"); code != 3 {
		t.Errorf("RunCommand(`ze show version --extended`) = %d, want 3: the local handler was not reached", code)
	}
	if len(got) != 1 || got[0] != "--extended" {
		t.Errorf("handler args = %v, want [--extended]", got)
	}
}

// VALIDATES: ExtractOutputFormat removes trailing format keyword.
// PREVENTS: format extraction breaking command dispatch.
func TestExtractOutputFormat(t *testing.T) {
	tests := []struct {
		name       string
		words      []string
		wantWords  []string
		wantFormat string
	}{
		{"json suffix", []string{"peer", "list", "json"}, []string{"peer", "list"}, "json"},
		{"yaml suffix", []string{"peer", "list", "yaml"}, []string{"peer", "list"}, "yaml"},
		{"table suffix", []string{"peer", "list", "table"}, []string{"peer", "list"}, "table"},
		{"no format", []string{"peer", "list"}, []string{"peer", "list"}, ""},
		{"format in middle", []string{"peer", "json", "list"}, []string{"peer", "json", "list"}, ""},
		{"only format", []string{"json"}, nil, "json"},
		{"empty", nil, nil, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			words, format := ExtractOutputFormat(tt.words)
			if format != tt.wantFormat {
				t.Errorf("format = %q, want %q", format, tt.wantFormat)
			}
			if len(words) != len(tt.wantWords) {
				t.Errorf("words = %v, want %v", words, tt.wantWords)
				return
			}
			for i, w := range words {
				if w != tt.wantWords[i] {
					t.Errorf("words[%d] = %q, want %q", i, w, tt.wantWords[i])
				}
			}
		})
	}
}

// VALIDATES: looksLikeSelector matches IPv4, IPv6, and glob patterns.
// PREVENTS: IPv6 peer selectors not being recognized.
func TestLooksLikeSelector(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"127.0.0.1", true},
		{"192.168.*.*", true},
		{"*", true},
		{"::1", true},
		{"2001:db8::1", true},
		{"fe80::1%eth0", true},
		{"as65001", true},
		{"AS64512", true},
		{"as", false},
		{"assign", false},
		{"asset", false},
		{"peer-name", false},
		{"show", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := looksLikeSelector(tt.input)
			if got != tt.want {
				t.Errorf("looksLikeSelector(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// VALIDATES: DescribeCommand returns description or summarizes children.
// PREVENTS: garbled help output for group commands.
func TestDescribeCommand(t *testing.T) {
	// Leaf with description.
	leaf := &cli.Command{Description: "Show BGP peers"}
	if got := DescribeCommand(leaf); got != "Show BGP peers" {
		t.Errorf("leaf desc = %q, want %q", got, "Show BGP peers")
	}

	// Group with children.
	group := &cli.Command{
		Children: map[string]*cli.Command{
			"bgp":  {},
			"peer": {},
		},
	}
	got := DescribeCommand(group)
	want := "subcommands: bgp, peer"
	if got != want {
		t.Errorf("group desc = %q, want %q", got, want)
	}

	// Empty node.
	empty := &cli.Command{}
	if got := DescribeCommand(empty); got != "" {
		t.Errorf("empty desc = %q, want empty", got)
	}
}

// VALIDATES: SuggestFromTree returns suggestions for typos.
// PREVENTS: missing "did you mean?" hints for unknown commands.
func TestSuggestFromTree(t *testing.T) {
	tree := &cli.Command{
		Children: map[string]*cli.Command{
			"peer":    {Description: "Peer commands"},
			"summary": {Description: "Summary"},
		},
	}

	// Close match should suggest.
	got := SuggestFromTree("pear", tree)
	if got != "peer" {
		t.Errorf("SuggestFromTree(pear) = %q, want %q", got, "peer")
	}

	// Nil tree children.
	got = SuggestFromTree("anything", &cli.Command{})
	if got != "" {
		t.Errorf("SuggestFromTree(nil children) = %q, want empty", got)
	}
}

// VALIDATES: RegisterLocalCommand stores handler and RunCommand dispatches it.
// PREVENTS: local handler registration silently failing.
func TestRegisterLocalCommandAndDispatch(t *testing.T) {
	// Clean up after test.
	defer registry.ResetForTest()

	called := false
	err := RegisterLocalCommand("test cmd", func(_ []string) int {
		called = true
		return 42
	})
	if err != nil {
		t.Fatalf("RegisterLocalCommand returned error: %v", err)
	}

	if !registry.HasLocal("test cmd") {
		t.Fatal("handler not found in registry")
	}
	handler, _ := registry.LookupLocal([]string{"test", "cmd"}, cli.IsDeclaredCommand)
	if handler == nil {
		t.Fatal("LookupLocal returned nil")
	}
	code := handler(nil)
	if !called {
		t.Error("handler was not called")
	}
	if code != 42 {
		t.Errorf("handler returned %d, want 42", code)
	}
}

// VALIDATES: RegisterLocalCommand rejects empty path.
// PREVENTS: empty key in localHandlers map causing silent misdispatch.
func TestRegisterLocalCommandEmptyPath(t *testing.T) {
	err := RegisterLocalCommand("", func(_ []string) int { return 0 })
	if err == nil {
		t.Error("expected error for empty path, got nil")
		registry.ResetForTest() // cleanup
	}
}

// VALIDATES: RegisterLocalCommand rejects nil handler.
// PREVENTS: nil function call panic at dispatch time.
func TestRegisterLocalCommandNilHandler(t *testing.T) {
	err := RegisterLocalCommand("test nil", nil)
	if err == nil {
		t.Error("expected error for nil handler, got nil")
		registry.ResetForTest() // cleanup
	}
}

// VALIDATES: RegisterLocalCommand overwrites existing entry.
// PREVENTS: stale handlers persisting after re-registration.
func TestRegisterLocalCommandOverwrite(t *testing.T) {
	defer registry.ResetForTest()

	first := false
	second := false

	if err := RegisterLocalCommand("overwrite", func(_ []string) int {
		first = true
		return 1
	}); err != nil {
		t.Fatal(err)
	}

	if err := RegisterLocalCommand("overwrite", func(_ []string) int {
		second = true
		return 2
	}); err != nil {
		t.Fatal(err)
	}

	handler, _ := registry.LookupLocal([]string{"overwrite"}, cli.IsDeclaredCommand)
	if handler == nil {
		t.Fatal("LookupLocal returned nil after overwrite")
	}
	code := handler(nil)
	if first {
		t.Error("first handler was called after overwrite")
	}
	if !second {
		t.Error("second handler was not called")
	}
	if code != 2 {
		t.Errorf("handler returned %d, want 2", code)
	}
}

// VALIDATES: matchLocalHandler finds longest prefix and passes remaining args.
// PREVENTS: wrong prefix matching or lost arguments.
func TestMatchLocalHandler(t *testing.T) {
	defer registry.ResetForTest()

	// Register handlers for testing.
	short := func(_ []string) int { return 1 }
	long := func(_ []string) int { return 2 }
	if err := RegisterLocalCommand("show bgp", short); err != nil {
		t.Fatal(err)
	}
	if err := RegisterLocalCommand("show bgp decode", long); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		words    []string
		values   []string
		wantCode int // -1 means no match (nil handler)
		wantArgs []string
	}{
		{"exact match", []string{"show", "bgp", "decode"}, nil, 2, nil},
		{"prefix with remaining args", []string{"show", "bgp", "decode", "--update", "FF"}, nil, 2, []string{"--update", "FF"}},
		{"shorter prefix", []string{"show", "bgp", "foo"}, nil, 1, []string{"foo"}},
		{"longest wins over shorter", []string{"show", "bgp", "decode", "x"}, nil, 2, []string{"x"}},
		{"with selector", []string{"show", "bgp", "decode"}, []string{"1.2.3.4"}, 2, []string{"1.2.3.4"}},
		{"args and selector", []string{"show", "bgp", "decode", "x"}, []string{"1.2.3.4"}, 2, []string{"x", "1.2.3.4"}},
		// A trailing multi-value grammar reaches a local handler as separate
		// arguments, in the order typed. Joining them into one string and
		// splitting it again would lose a value that carries a space.
		{"several values", []string{"show", "bgp", "decode"}, []string{"count", "5"}, 2, []string{"count", "5"}},
		{"no match", []string{"unknown", "cmd"}, nil, -1, nil},
		{"empty words", nil, nil, -1, nil},
		{"single word no match", []string{"version"}, nil, -1, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler, args := matchLocalHandler(tt.words, tt.values)
			if tt.wantCode == -1 {
				if handler != nil {
					t.Error("expected nil handler, got non-nil")
				}
				return
			}
			if handler == nil {
				t.Fatal("expected handler, got nil")
			}
			code := handler(nil)
			if code != tt.wantCode {
				t.Errorf("handler returned %d, want %d", code, tt.wantCode)
			}
			if len(args) != len(tt.wantArgs) {
				t.Errorf("args = %v, want %v", args, tt.wantArgs)
				return
			}
			for i, a := range args {
				if a != tt.wantArgs[i] {
					t.Errorf("args[%d] = %q, want %q", i, a, tt.wantArgs[i])
				}
			}
		})
	}
}
