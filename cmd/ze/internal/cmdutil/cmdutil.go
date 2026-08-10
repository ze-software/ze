// Design: docs/architecture/api/commands.md — shared CLI command utilities
//
// Package cmdutil provides shared logic for unified CLI verb dispatch.
// Verb commands (show, set, del, etc.) use this package for tree walking,
// local handler lookup, validation, flag extraction, and help formatting.
package cmdutil

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/ze-software/ze/cmd/ze/internal/suggest"
	cli "github.com/ze-software/ze/internal/component/cli/client"
	cmd "github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// LocalHandler is a function that handles a command locally (in-process),
// without connecting to the daemon. Kept as a type alias so callers that
// imported `cmdutil.LocalHandler` continue to compile.
type LocalHandler = registry.LocalHandler

// registerLocalCommand is a thin passthrough to the registry package.
// cmdutil historically owned this registry but cannot own it now because
// cmdutil imports cli (for BuildCommandTree), which would create an
// import cycle when each subcommand package's register.go registers
// itself. The canonical owner is the registry (leaf package, no cmd/ze
// deps); cmdutil forwards for source-compatibility with old callers.
func registerLocalCommand(path string, handler LocalHandler) error {
	return registry.RegisterLocal(path, handler)
}

// matchLocalHandler is a thin adapter over registry.LookupLocal that
// re-applies the values-as-trailing-args convention used by RunCommand.
//
// cli.IsDeclaredCommand is what lets the lookup refuse a handler registered
// above a declared command (registry.LookupLocal, the shadow rule). The
// registry cannot answer that itself: it is a leaf package by design and must
// not import the CLI, so the caller that already knows both namespaces supplies
// the answer.
func matchLocalHandler(words, values []string) (LocalHandler, []string) {
	handler, args := registry.LookupLocal(words, cli.IsDeclaredCommand)
	if handler == nil {
		return nil, nil
	}
	return handler, append(args, values...)
}

// Resolution is what one argv resolves to against one verb's command tree.
//
// Every path field is ABSOLUTE, verb included, because the local-handler
// registry, the offline-fallback registry and the daemon dispatcher are all
// keyed on the absolute path. Relative holds the same command as the tree
// spells it, which is the only form the tree can be walked with.
type Resolution struct {
	// Tree is the verb's command tree, built once and shared by every walk.
	Tree *cli.Command
	// Local is the path a local handler is looked up under. It keeps the
	// output-format keyword, so a handler that takes `json` as an argument
	// still receives it, and it keeps every trailing word, because
	// registry.LookupLocal ends the path itself (extractLocalValues).
	Local []string
	// LocalValues are the values an INLINE selector was lifted out of Local
	// into. A trailing value is not lifted: it stays in Local, where
	// registry.LookupLocal's own longest-prefix match returns it as an argument.
	LocalValues []string
	// Path is the absolute command the daemon dispatches on.
	Path []string
	// Relative is Path as the verb-relative tree spells it.
	Relative []string
	// Values are the positional values lifted out of Path, in the order they
	// were typed. Empty when the argv named a command and nothing else.
	Values []string
	// Format is the trailing yaml/json/table keyword, empty when none was typed.
	Format string
	// Valid reports that Relative names a node in Tree.
	Valid bool
	// Declared reports that a registered command declares Path exactly. False
	// with children under Relative means a grouping container such as `show bgp`.
	Declared bool
}

// dispatchable reports whether anything will answer Path.
//
// Two things can: a registered command that declares the path exactly, or an
// offline fallback covering it. The second kind is the reason this is not just
// Declared.
//
// NO PRODUCTION REGISTRATION REACHES THE SECOND BRANCH TODAY (checked
// 2026-08-08). Both offline fallbacks cover a declared path: `show host`
// (internal/plugins/host/register.go) declares ze-show:host-all and
// `show crashes` (internal/plugins/crashes/register.go) declares
// ze-show:crashes. The branch is kept for the next plugin that registers a
// fallback before declaring its path, and
// TestSyntheticOfflineFallbackBeatsGroupingContainer keeps it working from a
// synthetic registration. `show host` was that case until it was declared: with
// no ze:command and nine children it read as a bare grouping container, so
// RunCommand printed a subcommand list, exited 1, and RunShow
// (internal/plugins/host/host.go) was unreachable through `ze show host`.
//
// The fallback lookup is longest-prefix, matching registry.LookupLocal: a
// fallback at `show host` also answers `show host cpu`, which is what lets an
// operator read hardware inventory with no daemon running.
func (r Resolution) dispatchable() bool {
	if r.Declared {
		return true
	}
	handler, _ := registry.LookupOfflineFallback(r.Path)
	return handler != nil
}

// ResolveCommand matches argv -- verb first, exactly as a shell hands it to
// `ze` -- against that verb's command tree.
//
// argv carries the verb (`show bgp rib status`) while cli.BuildVerbCommandTree
// is RELATIVE to it (`bgp rib status`). This function is where the two are
// aligned, and where every absolute form is rebuilt with cli.AbsoluteVerbPath.
// Walking the relative tree with absolute words made 56 of the 63 `ze show`
// commands answer `unknown command`, and every other verb with them.
//
// RunCommand is the production caller. It is exported because RunCommand ends
// in an SSH dispatch and cannot run without a daemon, so this is the seam a
// test drives with a real argv to prove a declared verb still resolves.
// ok is false when the verb was typed with nothing after it.
func ResolveCommand(args []string, cmdName string) (Resolution, bool) {
	if len(args) == 0 {
		return Resolution{}, false
	}
	verbWords := args
	if args[0] == cmdName {
		verbWords = args[1:]
	}
	if len(verbWords) == 0 {
		return Resolution{}, false
	}

	res := Resolution{Tree: cli.BuildVerbCommandTree(cmdName)}

	// Separate the command words from the positional values typed after or
	// inside them, for example `show bgp peer edge1 detail`.
	localRel, localValues := extractLocalValues(verbWords, res.Tree)
	res.Local, _ = cli.AbsoluteVerbPath(cmdName, localRel)
	res.LocalValues = localValues

	// Extract the output format keyword (yaml/json/table) from the end of the
	// command. Done after Local so format keywords are not silently stripped
	// from commands that do not support them.
	verbWords, res.Format = extractOutputFormat(verbWords)
	if len(verbWords) == 0 {
		return res, false // every word was the format keyword
	}
	res.Relative, res.Values = ExtractValues(verbWords, res.Tree, cmdName)
	res.Path, res.Declared = cli.AbsoluteVerbPath(cmdName, res.Relative)
	res.Valid = IsValidCommand(res.Relative, res.Tree)
	return res, true
}

// dispatchString is the command line the daemon is asked to run: the absolute
// path with the positional values put back after it.
//
// RunCommand is the production caller. It is a method rather than four lines
// inside RunCommand because RunCommand ends in an SSH dispatch, so a test that
// wants to assert what the daemon receives has no other way to read it, and a
// test that rebuilt the string itself would assert against its own copy.
//
// The two extraction shapes end differently here, on purpose. A trailing value
// (`show pki certificate name web`) comes back in the position it was typed. An
// inline selector (`show bgp peer edge1 detail`) is REORDERED to sit after the
// action word, which is the form the daemon is keyed on.
func (r Resolution) dispatchString() string {
	var tb textbuf.Buffer
	tb.Join(r.Path, " ")
	for _, value := range r.Values {
		tb.Byte(' ').Str(value)
	}
	return tb.String()
}

// RunCommand resolves argv against the verb's command tree and delegates
// execution. Local handlers run in-process; daemon commands go through cli.Run
// via SSH. The cmdName is used in error/hint messages.
//
// Read-only filtering does NOT happen here. It is structural: verbContextPath
// (internal/component/cli/client/verb_tree.go) admits a command into the `show`
// tree only when it is rooted under `show` or pluginserver.IsReadOnlyPath says
// so, and a path outside the tree never reaches this function with Valid set.
// A readOnly parameter was carried here until 2026-08-08 and rejected nothing.
func RunCommand(args []string, cmdName string) int {
	res, ok := ResolveCommand(args, cmdName)

	// Check the local handler registry first (offline commands like version,
	// completion). Longest prefix match: "show bgp decode update hex" matches
	// handler "show bgp decode" with remaining args ["update", "hex"].
	//
	// This runs BEFORE the resolution verdict on purpose. res.Local keeps the
	// output-format keyword, and ResolveCommand answers ok=false when every word
	// after the verb was one, so a handler registered at a path ending in
	// `json`, `yaml` or `table` is reachable only from here.
	//
	// Running first does NOT make a local handler beat a declared command: the
	// lookup itself refuses a match that would swallow one (registry.LookupLocal,
	// the shadow rule), so the order decides only what happens when nothing is
	// declared below the handler's path.
	if handler, handlerArgs := matchLocalHandler(res.Local, res.LocalValues); handler != nil {
		return handler(handlerArgs)
	}
	if !ok {
		return -1 // signal caller to show usage
	}

	// A command absent from this binary's local command tree may still be a
	// daemon command with a registered offline fallback (e.g. show crashes, show
	// host). Route those to the daemon path below: cli.Run serves them from the
	// daemon when reachable and from the in-process fallback when not, so they
	// are never rejected as unknown and the fallback never shadows the daemon.
	dispatchable := res.dispatchable()
	if !res.Valid && !dispatchable {
		fmt.Fprintf(os.Stderr, "error: unknown command: %s\n", textbuf.Join(args, " "))
		if suggestion := SuggestFromTree(res.Relative[0], res.Tree); suggestion != "" {
			fmt.Fprintf(os.Stderr, "hint: did you mean 'ze %s %s'?\n", cmdName, suggestion)
		}
		fmt.Fprintf(os.Stderr, "hint: run 'ze %s help' for available commands\n", cmdName)
		return 1
	}

	// Grouping container (`show bgp`): nothing declares this exact path and no
	// offline fallback covers it, so there is nothing to dispatch. List its
	// members and exit 1, the code ai/rules/cli.md gives an incomplete command.
	if !dispatchable {
		if node := FindNode(res.Relative, res.Tree); node != nil && len(node.Children) > 0 {
			fmt.Fprintf(os.Stderr, "%s subcommands:\n", textbuf.Join(res.Path, " "))
			printChildren(node)
			return 1
		}
	}

	// The CLI resolves the structural command path, then passes the extracted
	// values as regular handler arguments.
	runCmd := res.dispatchString()

	var cliArgs []string
	if res.Format != "" {
		cliArgs = append(cliArgs, "--format", res.Format)
	}
	cliArgs = append(cliArgs, "-c", runCmd)

	return cli.Run(cliArgs)
}

// extractOutputFormat removes a trailing format keyword (yaml/json/table) from command words.
func extractOutputFormat(words []string) ([]string, string) {
	if len(words) == 0 {
		return words, ""
	}
	last := words[len(words)-1]
	switch last {
	case "yaml", "json", "table":
		return words[:len(words)-1], last
	}
	return words, ""
}

// IsValidCommand checks if the command words match a path in the given tree.
func IsValidCommand(words []string, tree *cli.Command) bool {
	if len(words) == 0 {
		return false
	}
	current := tree

	for _, word := range words {
		if current.Children == nil {
			return false
		}
		child, ok := current.Children[word]
		if !ok {
			return false
		}
		current = child
	}

	return current.Description != "" || len(current.Children) > 0
}

// SuggestFromTree returns a "did you mean?" suggestion for the first command word.
func SuggestFromTree(word string, tree *cli.Command) string {
	if tree.Children == nil {
		return ""
	}
	candidates := make([]string, 0, len(tree.Children))
	for k := range tree.Children {
		candidates = append(candidates, k)
	}
	return suggest.Command(word, candidates)
}

// CommandEntry holds a top-level command name and description for help display.
type CommandEntry struct {
	Name string
	Desc string
}

// commandList returns sorted top-level commands from the tree.
func commandList(tree *cli.Command) []CommandEntry {
	if tree.Children == nil {
		return nil
	}

	keys := make([]string, 0, len(tree.Children))
	for k := range tree.Children {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	entries := make([]CommandEntry, 0, len(keys))
	for _, name := range keys {
		child := tree.Children[name]
		entries = append(entries, CommandEntry{
			Name: name,
			Desc: DescribeCommand(child),
		})
	}
	return entries
}

// ExtractValues splits command words into the words that name a node in the
// tree and the positional values typed with them.
//
// THE RULE IS THE DAEMON'S, TRANSPLANTED, NOT A HEURISTIC OF ITS OWN.
// matchCommandTokens (internal/component/plugin/server/command.go) walks the
// registered command keys longest-first and returns tokens[inIdx:] as the
// handler's arguments, so on the daemon a DECLARED path is a prefix and every
// word after it is a value. It never consults ArgDefs to find where the path
// ends. Any rule here that answers differently ships as `unknown command` for a
// line `ze cli -c` runs, which is the whole defect class this function has now
// produced twice.
//
// Two shapes exist, and they differ in where the value sits:
//
//   - INLINE, tried first because the daemon tries it first: its longest-key
//     walk consumes a selector mid-path rather than stopping short of the
//     action word. The value sits between a resource token and a later action
//     token, and a node further down the remaining path declares a mandatory
//     argument for it: "show bgp peer edge1 detail" ->
//     treeWords=["show","bgp","peer","detail"], values=["edge1"]. Only this
//     shape reorders, and DispatchString is where the value goes back.
//
//   - TRAILING. The walk reaches a node that ENDS A DECLARED COMMAND and meets
//     a word that is not one of its children, so that word and every word after
//     it is one of its values: "show pki certificate name web" ->
//     treeWords=["show","pki","certificate","name"], values=["web"].
//     Keying this on ArgDefs was the second defect: 30 declared commands read
//     args[0] in their handler while their YANG declares no leaf, so ArgDefs is
//     empty for them and `ze show route lookup 1.2.3.4` answered `unknown
//     command` while `ze cli -c "show route lookup 1.2.3.4"` ran. ArgDefs are
//     derived from ze:command leaves, so a node that has them is declared:
//     keying on declaration widens the rule without dropping a case.
//
// Taking every remaining word in the trailing shape, rather than one, is what
// keeps a multi-value grammar (`show bgp irr check <peer> <prefix>`) and a
// keyword-value tail (`show log recent count 5`) intact: DispatchString then
// rebuilds exactly the typed line.
//
// THE TRADE, unchanged in kind by the widening: under a node that takes values,
// a mistyped subcommand (`show capture rwa`) becomes a value and reaches the
// daemon's positional matcher instead of SuggestFromTree, so the operator gets
// an empty result rather than "did you mean". The daemon answers the same argv
// the same way, so this is the two resolvers agreeing rather than a client-side
// loss, and the alternative -- refusing a value because it could have been a
// typo -- is the defect above.
func ExtractValues(words []string, tree *cli.Command, verb string) (treeWords, values []string) {
	return extractValues(words, tree, func(prefix []string) bool {
		return endsDeclaredCommand(verb, prefix)
	})
}

// extractLocalValues splits words for the LOCAL-HANDLER lookup, which is keyed
// on a different registry and therefore ends its paths somewhere else.
//
// registry.LookupLocal already does a longest-prefix match over the words it is
// handed and returns the rest as the handler's arguments, so the TRAILING split
// is done, by the registry that owns those keys. Doing it here as well cuts the
// path short: `ze show debug profile name default` is served by runShowProfile,
// registered at `show debug profile` (internal/plugins/debug/register.go), while
// `show debug` is a declared ze:command and `profile` is no node of the show
// TREE. The daemon's boundary therefore ends the path at `show debug`, which no
// local handler holds, and the command went to the daemon and answered `no
// credentials` (test/ui/debug-enable-show.ci).
//
// The INLINE shape still applies here, because it REORDERS rather than trims: a
// handler registered at `show bgp peer detail` is not reachable from the words
// `show bgp peer edge1 detail` in the order they were typed.
func extractLocalValues(words []string, tree *cli.Command) (treeWords, values []string) {
	return extractValues(words, tree, nil)
}

// extractValues is the shared walk. endsCommand is the TRAILING boundary and is
// nil for a caller that has its own.
//
// IT EXTRACTS AT MOST ONE GROUP AND RETURNS. The inline branch returns the
// moment it lifts a selector instead of resuming the walk on the words after
// it, so an inline selector followed by a further value would leave that value
// in Relative and the client would answer `unknown command` for a line
// matchCommandTokens accepts. No argv reaches that today: an inline target must
// declare a mandatory leaf (hasImplicitSelectorArg), and every node in the tree
// that does is childless, so nothing can follow it but the value the trailing
// branch already takes whole. This is the walk's shape, not a live defect;
// giving a mandatory-leaf node children is what would make it one.
func extractValues(words []string, tree *cli.Command, endsCommand func(prefix []string) bool) (treeWords, values []string) {
	if len(words) < 2 {
		return words, nil
	}

	current := tree
	for i, word := range words {
		if child, ok := current.Children[word]; ok {
			current = child
			continue
		}
		if shouldExtractSelector(current, words, i) {
			treeWords = make([]string, 0, len(words)-1)
			treeWords = append(treeWords, words[:i]...)
			treeWords = append(treeWords, words[i+1:]...)
			return treeWords, words[i : i+1 : i+1]
		}
		// i == 0 would leave no words naming a command at all, and RunCommand
		// reads Relative[0] to build its suggestion.
		if i > 0 && endsCommand != nil && endsCommand(words[:i]) {
			return words[:i:i], words[i:]
		}
		return words, nil
	}
	return words, nil
}

// endsDeclaredCommand reports whether rel, verb-relative, names a command some
// registered ze:command declares. It asks cli.AbsoluteVerbPath, which reads the
// same two registrations the daemon's dispatcher is keyed on, so the client
// cannot decide a path ends somewhere the daemon would not.
func endsDeclaredCommand(verb string, rel []string) bool {
	if len(rel) == 0 {
		return false
	}
	_, declared := cli.AbsoluteVerbPath(verb, rel)
	return declared
}

// shouldExtractSelector reports whether words[idx] is an INLINE selector: a
// value with more command words after it, where those later words reach a node
// declaring a mandatory argument the selector fills. It is asked before the
// trailing shape because the daemon asks it first: matchBuiltinTokens sorts its
// keys longest-first, so `show ospf neighbor 1.2.3.4 detail` binds to the
// four-word key with a selector rather than to the three-word key with two
// spare arguments.
//
// THE looksLikeSelector BRANCH DECIDES NOTHING. Its condition is a conjunction
// whose second half is the very expression the next line returns, so both
// outcomes of looksLikeSelector reach the same verdict. What the word LOOKS like
// stopped mattering when the grammar became the test: a selector is whatever
// fills a mandatory argument, address-shaped or not. looksLikeSelector has no
// production caller outside TestLooksLikeSelector.
func shouldExtractSelector(current *cli.Command, words []string, idx int) bool {
	word := words[idx]
	if idx+1 >= len(words) || current.Children == nil {
		return false
	}
	if looksLikeSelector(word) && pathExpectsImplicitSelector(current, words[idx+1:]) {
		return true
	}
	return pathExpectsImplicitSelector(current, words[idx+1:])
}

func pathExpectsImplicitSelector(current *cli.Command, remaining []string) bool {
	node := current
	for _, word := range remaining {
		if node.Children == nil {
			return false
		}
		child, ok := node.Children[word]
		if !ok {
			return false
		}
		node = child
		if hasImplicitSelectorArg(node) {
			return true
		}
	}
	return false
}

func hasImplicitSelectorArg(node *cli.Command) bool {
	for i := range node.ArgDefs {
		def := node.ArgDefs[i]
		if def.Kind != cmd.ArgString || !def.Mandatory {
			continue
		}
		if !strings.EqualFold(def.Name, node.Name) {
			return true
		}
	}
	return false
}

// looksLikeSelector returns true if the word looks like an IP address or glob pattern.
// Matches: "127.0.0.1", "192.168.*.*", "10.0.0.0/24", "*", "::1", "2001:db8::1".
func looksLikeSelector(s string) bool {
	if s == "*" {
		return true
	}
	if len(s) > 2 && (s[0] == 'a' || s[0] == 'A') && (s[1] == 's' || s[1] == 'S') {
		allDigits := true
		for i := 2; i < len(s); i++ {
			if s[i] < '0' || s[i] > '9' {
				allDigits = false
				break
			}
		}
		if allDigits {
			return true
		}
	}
	// Contains dot (IPv4) or colon (IPv6)
	return strings.ContainsAny(s, ".:")
}

// FindNode returns the command node at the given path, or nil if not found.
func FindNode(words []string, tree *cli.Command) *cli.Command {
	current := tree
	for _, word := range words {
		if current.Children == nil {
			return nil
		}
		child, ok := current.Children[word]
		if !ok {
			return nil
		}
		current = child
	}
	return current
}

// printChildren prints the children of a command node to stderr.
func printChildren(node *cli.Command) {
	entries := commandList(node)
	for _, e := range entries {
		fmt.Fprintf(os.Stderr, "  %-20s %s\n", e.Name, e.Desc)
	}
}

// DescribeCommand returns a description for a command node.
// Uses the node's own description if it's a leaf, or summarizes children.
func DescribeCommand(cmd *cli.Command) string {
	if cmd.Description != "" {
		return cmd.Description
	}
	if len(cmd.Children) == 0 {
		return ""
	}
	subs := make([]string, 0, len(cmd.Children))
	for k := range cmd.Children {
		subs = append(subs, k)
	}
	sort.Strings(subs)
	var tb textbuf.Buffer
	return tb.Str("subcommands: ").Join(subs, ", ").String()
}

// printCommandList writes the formatted command list to stderr.
func printCommandList(tree *cli.Command) {
	entries := commandList(tree)
	for _, e := range entries {
		fmt.Fprintf(os.Stderr, "  %-16s %s\n", e.Name, e.Desc)
	}
}
