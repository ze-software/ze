// The guard lives in an EXTERNAL test package so it can import the composition
// root. `internal/component/plugin/all` imports this package's plugins, so an
// in-package test importing it would be a cycle; `reactor_test` compiles after
// both and has no such edge.
package reactor_test

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/test/tmpfs"

	// Registers every YANG module, which is what lets ParseTreeForValidation
	// read a config here the way `ze config validate` reads it. Without it the
	// schema resolves nothing past ze-extensions, every parse fails, and the
	// whole walk silently degrades to the text scan below.
	// TestReceivedOnlyGuardReadsTheTreeThroughTheParser is what refuses that.
	_ "github.com/ze-software/ze/internal/component/plugin/all"
)

// noSentUpdates names the in-tree plugins that must never be fed the UPDATEs ze
// SENDS, with the reason each one states in its own source. A plain `update`
// token grants both directions, so a config that writes it hands these plugins
// the sent direction as well.
var noSentUpdates = map[string]string{
	"bgp-rr": "forwarding an UPDATE raises the sent event the forward is waiting on " +
		"(rr.go: ForwardUpdate -> onMessageSent -> deliver -> block)",
	"bgp-rs": "same circular deadlock as bgp-rr (server.go, above SetStartupSubscriptions)",
	"bgp-rpki": "origin validation judges what the peer announced; ze's own announcements " +
		"carry no origin to validate (rpki.go declares update direction received)",
	"bgp-rpki-decorator": "it decorates a received UPDATE with its validation state " +
		"(decorator.go declares update direction received)",
}

// TestNoConfigFeedsSentUpdatesToAReceivedOnlyPlugin walks every config in the
// tree and refuses a receive list that hands bgp-rr, bgp-rs, bgp-rpki or
// bgp-rpki-decorator the UPDATEs ze sends.
//
// VALIDATES: R-15. Direction is not a preference for these four plugins. Two of
// them deadlock on the sent direction and the other two judge a message that
// carries nothing to judge.
// PREVENTS: a config granting plain `update` or `*` to one of them, which reads
// as a wrong filter today and becomes a daemon that stops once delivery honors
// the config.
//
// The last three assertions guard the guard's own reader rather than the tree.
// A walk that finds nothing passes every "no violation" check, so the counters
// say HOW the tree was read: dropping the composition-root import, or a schema
// change that makes every document fail to parse, would leave the text scan as
// the only reader and every violation check still green.
//
// a second test held those three assertions and ran a SECOND full
// walk to reach them. No assertion is dropped -- the three moved here, and what
// went with them is the duplicate walk, which re-read every config in the
// repository and rebuilt the YANG schema once per document to reach numbers
// this walk had already produced.
func TestNoConfigFeedsSentUpdatesToAReceivedOnlyPlugin(t *testing.T) {
	files, checked, parsed := walkTreeConfigs(t)

	require.NotZero(t, files, "no config could grant a received-only plugin anything; the walk is looking in the wrong place")
	require.Greater(t, parsed.documents, parsed.refused,
		"more texts are being refused than read; a surface that used to parse stopped parsing, or the walk is offering the parser texts that are not documents")
	require.NotZero(t, checked.tree+checked.text, "no attach block named one of the received-only plugins; the resolver is broken")
	require.NotZero(t, parsed.documents, "no document in the tree parsed; the schema is not loaded in this binary")
	require.NotZero(t, checked.tree, "no attach block naming a pinned plugin was reached through a parsed tree")
	require.Greater(t, checked.tree, checked.text,
		"the text scan is now finding more pinned blocks than the parser; a surface that used to parse stopped parsing")
}

// counts records how the walk reached what it judged, so the two tests above can
// tell "nothing to find" from "nothing readable".
type counts struct {
	tree int // attach blocks naming a pinned plugin, found by walking a parsed tree
	text int // the same, found by the text scan of a document the parser refused
}

type parseCounts struct {
	documents int // texts ParseTreeForValidation accepted
	refused   int // texts it refused, which the text scan then read
}

// walkTreeConfigs runs the guard over every config-bearing file in the tree and
// returns how many files carried an attach block, how each judged block was
// reached, and how the documents split between the parser and the text scan.
func walkTreeConfigs(t *testing.T) (files int, checked counts, parsed parseCounts) {
	t.Helper()

	root := filepath.Join("..", "..", "..", "..")
	dirs := []string{"test", "docs", "demos", "contrib"}
	// `.j2` is deliberately absent. A Jinja template is not a document either
	// reader can judge: the parser cannot accept `{{ }}` and `{% %}`, and feeding
	// the text scan a family of texts that can never parse would make
	// parseCounts.refused grow for a reason that is not "a surface stopped
	// parsing", which is the one thing that counter exists to say.
	// `contrib/netlab/ze/ze.j2` is the only template that writes an attach block,
	// and what it RENDERS is judged: contrib/netlab/golden/r1.conf, r2.conf and
	// r3.conf are committed and are read here as configs. A template whose output
	// has no committed golden would be unread, and that is the limit.
	exts := map[string]bool{".ci": true, ".conf": true, ".md": true, ".et": true}

	for _, dir := range dirs {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || !exts[filepath.Ext(path)] {
				return err
			}
			body, err := os.ReadFile(path) //nolint:gosec // test walks the repository's own fixtures
			if err != nil {
				return err
			}
			text := string(body)
			// `attach` alone, not `attach process `: the nested spelling puts the
			// process name on a later line, and a prefilter that wants both words
			// together skips the whole file before the block matcher runs.
			if !strings.Contains(text, "attach") || !namesAPinnedPlugin(text) {
				return nil
			}
			files++

			var violations []string
			if filepath.Ext(path) == ".ci" {
				violations = scanCITest(text, &checked, &parsed)
			} else {
				violations = scanOneDocument(text, wholeFile(), &checked, &parsed)
			}
			require.Emptyf(t, violations, "%s grants a received-only plugin the UPDATEs ze sends:\n%s",
				path, strings.Join(violations, "\n"))
			return nil
		})
		require.NoError(t, err)
	}
	return files, checked, parsed
}

// namesAPinnedPlugin answers whether a text could grant one of the four plugins
// anything at all.
//
// Reaching a pinned plugin means SPELLING its name in that document: either the
// attach block names it, or the block names an alias whose `use` names it, and
// an alias is declared in the document that uses it. A text spelling none of the
// four can hold no violation, in any shape, so it is not read.
//
// This is what keeps the walk at seconds. Every parse rebuilds the whole YANG
// schema (internal/component/config/yang_schema.go, YANGSchema caches nothing),
// and offering it every attach-bearing file in the repository cost about 30
// seconds against about 7 for the files that could actually carry a grant.
func namesAPinnedPlugin(text string) bool {
	for name := range noSentUpdates {
		if strings.Contains(text, name) {
			return true
		}
	}
	return false
}

// where names the place a violation was found, and the two readers need two
// different answers. A parsed tree has no line numbers to give, so a violation
// found in one is named by its config path. The text scan has lines, and they
// are the file's lines only when the text IS the file.
type where struct {
	// block names the embedded document this text came from, and is empty for
	// a file. It prefixes both answers.
	block string
	// line names one line. A `.ci` directive line has no line number in the
	// file, so its locator names the line itself instead of numbering it.
	line func(n int) string
}

// wholeFile is the locator for a text that IS the file.
func wholeFile() where {
	return where{line: func(n int) string { return fmt.Sprintf("line %d", n) }}
}

// inBlock is the locator for a document embedded in a `.ci`, whose line numbers
// are its own and not the file's.
func inBlock(name string) where {
	return where{block: name, line: func(n int) string { return fmt.Sprintf("line %d", n) }}
}

// at names a config path inside this document.
func (w where) at(path string) string {
	if w.block == "" {
		return path
	}
	return w.block + " " + path
}

// atLine names one line of this document.
func (w where) atLine(n int) string {
	if w.block == "" {
		return w.line(n)
	}
	return w.block + " " + w.line(n)
}

// scanOneDocument judges one text, through the config parser when the parser
// accepts it and through the text scan when it does not.
//
// The parser is ze's own reader, and not a reader like it:
// config.ParseTreeForValidation is the function `ze config validate` calls
// (internal/component/config/cli/cmd_validate.go:248), and it routes on
// DetectFormat to the same tokenizer, YANG schema and SetParser the daemon
// loads with (internal/component/config/loader.go, parseTreeWithYANG).
// Whatever spelling ze accepts, the tree it produces is the same tree, so the
// block spelling, the brace nesting, the quoting, the comment that carries a
// brace and the set form stop being separate cases here.
//
// The text scan below reads what has no parseable document: a snippet in a
// guide, a table cell, an editor script, and any file the native parser refuses
// (the legacy test/exabgp-compat/native/api-*.conf fixtures are the population
// this last case exists for -- internal/component/bgp/config/loader_test.go
// excludes them from parser coverage by name). What it can and cannot see is
// pinned by TestReceivedOnlyGuardTextScanLimits, not claimed here.
func scanOneDocument(text string, loc where, checked *counts, parsed *parseCounts) []string {
	if tree, err := config.ParseTreeForValidation(text); err == nil && tree != nil {
		parsed.documents++
		violations, n := scanParsedTree(tree.ToMap(), loc)
		checked.tree += n
		return violations
	}
	parsed.refused++
	violations, n := scanText(text, loc)
	checked.text += n
	return violations
}

// scanCITest judges a functional test, one embedded document at a time.
//
// A `.ci` file is not a config: it is a script whose config lives in a
// heredoc. tmpfs.Parse is that format's own reader (internal/test/tmpfs), and
// it returns the same three populations the runner gets -- `stdin=` blocks,
// `tmpfs=` files, and every remaining directive line. Each block is a document
// on its own, so each is offered to the config parser on its own.
//
// A block's line numbers are not the file's, so a violation the text scan finds
// inside one names the block it came from.
func scanCITest(text string, checked *counts, parsed *parseCounts) []string {
	fs, err := tmpfs.Parse(strings.NewReader(text))
	if err != nil {
		// A `.ci` this format's own reader refuses is read as flat text, which
		// keeps the file judged rather than skipped.
		return scanOneDocument(text, wholeFile(), checked, parsed)
	}

	var violations []string
	names := make([]string, 0, len(fs.StdinBlocks))
	for name := range fs.StdinBlocks {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		violations = append(violations, scanCIBlock(string(fs.StdinBlocks[name]), "stdin "+name, checked, parsed)...)
	}
	for _, f := range fs.Files {
		violations = append(violations, scanCIBlock(string(f.Content), "tmpfs "+f.Path, checked, parsed)...)
	}

	// The directive lines carry the CLI `set` form and the `exec=` lines. They
	// are named by their own text, because their position among the directives
	// is not their position in the file.
	other := fs.OtherLines
	if grants := strings.Join(other, "\n"); strings.Contains(grants, "attach") {
		directives := where{block: "directive", line: func(n int) string {
			if n >= 1 && n <= len(other) {
				return strconv.Quote(strings.TrimSpace(other[n-1]))
			}
			return fmt.Sprintf("%d", n)
		}}
		violations = append(violations, scanOneDocument(grants, directives, checked, parsed)...)
	}
	return violations
}

// scanCIBlock judges one embedded block, skipping the blocks that grant
// nothing. A block with no `attach` in it can carry no grant, in any spelling:
// the block form and the set form both spell the word.
func scanCIBlock(body, name string, checked *counts, parsed *parseCounts) []string {
	if !strings.Contains(body, "attach") {
		return nil
	}
	return scanOneDocument(ciVariables.Replace(body), inBlock(name), checked, parsed)
}

// ciVariables applies the runner's own substitutions, so a block is offered to
// the parser as the document ze is handed rather than as the template the file
// holds. $PORT2 is replaced before $PORT because "$PORT2" contains "$PORT"
// (internal/test/runner/runner_exec.go says the same thing where it expands a
// tmpfs file). The values are arbitrary: nothing here binds a port, and only the
// parse has to succeed.
//
// Measured with a built `ze config validate` over the 75 attach-bearing blocks
// this walk reads: 49 parsed untouched, 25 failed only on an unexpanded $PORT2,
// and one is a python script that is not config at all. A variable the runner
// adds later and this list does not carry costs its blocks the parser and
// leaves them on the text scan below, which reads them but reads them by shape.
var ciVariables = strings.NewReplacer("$PORT2", "1791", "$PORT", "1790")

// scanParsedTree walks the tree ze built and records every receive token that
// grants a pinned plugin the sent direction.
//
// The walk looks for `attach` -> `process` at any depth rather than at a fixed
// path, because a peer, a group and a group's peer all carry the same
// container. It is not looking at spellings at all: `attach process x { }`,
// `attach { process x { } }`, a quoted name, a quoted value and a one-line
// block are one tree by the time this runs.
func scanParsedTree(root map[string]any, loc where) (violations []string, checked int) {
	alias := treeAliases(root)

	var walk func(node map[string]any, path string)
	walk = func(node map[string]any, path string) {
		for name, entry := range childMap(childMap(node, "attach"), "process") {
			binding, ok := entry.(map[string]any)
			if !ok {
				continue
			}
			plugin, reason, pinned := resolvePinned(alias, name, declaredTarget(binding))
			if !pinned {
				continue
			}
			checked++
			at := loc.at(path + "/attach/process/" + name)
			for _, tok := range leafList(binding, "receive") {
				violations = appendViolation(violations, at, plugin, reason, tok)
			}
		}
		for key, child := range node {
			if sub, ok := child.(map[string]any); ok {
				walk(sub, path+"/"+key)
			}
		}
	}
	walk(root, "")

	// Map iteration order is random and a failure message is read by a person.
	sort.Strings(violations)
	return violations, checked
}

// treeAliases reads the plugin declarations of a parsed config, so a block
// naming an alias is judged against the plugin behind it.
//
// BOTH declaration lists carry BOTH leaves: `internal <name> { use <plugin> }`
// runs the plugin in process, and `external <name> { run <command> }` names one
// too (internal/component/plugin/yang/ze-plugin-conf.yang, list internal and
// list external). Every alias in this tree today is declared under internal with
// `use`, and reading only that pair was a hole: `external rrx { run ze.bgp-rr }`
// validates, and startInternal (plugin/process/process.go) resolves it to the
// registered bgp-rr runner exactly as `use bgp-rr` does.
func treeAliases(root map[string]any) map[string]string {
	alias := map[string]string{}
	plugins := childMap(root, "plugin")
	for _, kind := range []string{"internal", "external"} {
		for name, entry := range childMap(plugins, kind) {
			decl, ok := entry.(map[string]any)
			if !ok {
				continue
			}
			if target := declaredTarget(decl); target != "" {
				alias[unquoteName(name)] = target
			}
		}
	}
	return alias
}

// declaredTarget names the plugin one `use` or `run` leaf reaches, or "" when
// the node carries neither.
//
// The leaf is read the way startInternal (internal/component/plugin/process/process.go)
// reads it, and no wider: a bare registered name is taken as itself, and a `ze.`
// prefix is stripped because ResolvePlugin (plugin/resolve.go) treats
// `ze.<name>` as the internal plugin <name>. Anything else -- a path, a command
// with arguments, `ze plugin <name>` -- resolves to an EXTERNAL plugin there, so
// it never reaches a registered runner and must not be resolved to one here.
func declaredTarget(node map[string]any) string {
	for _, key := range []string{"use", "run"} {
		if v, ok := node[key].(string); ok && v != "" {
			return strings.TrimPrefix(unquoteName(v), "ze.")
		}
	}
	return ""
}

// childMap returns one child container of a tree node, or nil.
func childMap(node map[string]any, key string) map[string]any {
	if node == nil {
		return nil
	}
	child, _ := node[key].(map[string]any)
	return child
}

// leafList returns the members of a leaf-list.
//
// Tree.ToMap collapses a one-member leaf-list to a bare string and keeps a
// longer one as a slice (internal/component/config/tree.go, ToMap), so
// `receive update` and `receive [ update state ]` arrive as different Go types
// and both are members here.
func leafList(node map[string]any, key string) []string {
	switch v := node[key].(type) {
	case string:
		return []string{v}
	case []string:
		return v
	default:
		return nil
	}
}

// resolvePinned answers whether one attach block names a received-only plugin,
// following an alias to the plugin behind it.
//
// inline is the plugin the BLOCK ITSELF names, and it wins over the alias map.
// `attach process rrx { run ze.bgp-rr; receive [ update ] }` declares its plugin
// where it attaches it: extractInlinePluginsFromMap
// (internal/component/bgp/config/plugins.go) builds the PluginConfig from that
// block, and validatePeerProcessRefs (same file) skips its own name check for
// exactly this shape, so nothing else in the config has to mention rrx. Reading
// only the alias map left that grant unchecked -- resolvePinned answered
// not-pinned, the counter never moved, and R-15's deadlock passed in silence.
func resolvePinned(alias map[string]string, name, inline string) (plugin, reason string, pinned bool) {
	plugin = unquoteName(name)
	if inline != "" {
		plugin = inline
	} else if to, ok := alias[plugin]; ok {
		plugin = to
	}
	reason, pinned = noSentUpdates[plugin]
	return plugin, reason, pinned
}

// appendViolation judges one token and records it when it grants a received-only
// plugin the sent direction. Resolution goes through the grammar's own producer,
// so a custom type the test binary does not register is left alone rather than
// guessed at: only a token that resolves to the base UPDATE type, or the
// wildcard that names every type, can carry the sent direction.
func appendViolation(out []string, at, plugin, reason, tok string) []string {
	if tok == events.TokenWildcard {
		return append(out, fmt.Sprintf(
			"%s: %s is fed every type in both directions by `receive [ * ]`: %s",
			at, plugin, reason))
	}

	eventType, dir, ok := events.SplitTypeToken(bgpevents.Namespace, tok)
	if !ok || eventType != bgpevents.EventUpdate || dir == events.DirReceived {
		return out
	}
	return append(out, fmt.Sprintf(
		"%s: %s is granted %q, which feeds it the UPDATEs ze sends: %s. Write update-received.",
		at, plugin, tok, reason))
}

// ---------------------------------------------------------------------------
// The text scan. It reads what has no parseable document.
//
// Everything below is pattern matching over lines, and it is here because a
// guide's snippet, a markdown table cell and a legacy fixture are not documents
// ze can parse. It is NOT a second config reader: it recognizes the spellings
// this tree writes, and it will not recognize a spelling nobody has written
// yet. That is the whole reason the parser above exists, and the reason the
// walk hands it every document it can.
// ---------------------------------------------------------------------------

// None of these is anchored to the start of a line, and that is deliberate. A
// markdown table cell, a `plugin { }` wrapper and a one-line alias each put
// something in front of the keyword.
//
// Every name is captured with its quotes and unquoted by unquoteName, and
// targetToken skips an opening quote for the same reason: a list key is a word
// OR a quoted string (config/parser_list.go, parseList takes the key from
// tokenWord and tokenString alike), so `process "bgp-rr" { }` is the same
// block as the bare spelling.
var (
	attachOpen   = regexp.MustCompile(`attach\s+process\s+(\S+)\s*\{(.*)$`)
	attachSet    = regexp.MustCompile(`\bset\s+.*\battach\s+process\s+(\S+)\s+receive\s+(.*?)\s*$`)
	attachNested = regexp.MustCompile(`\battach\s*\{(.*)$`)
	processOpen  = regexp.MustCompile(`\bprocess\s+(\S+)\s*\{(.*)$`)
	// Both declaration lists and both leaves. `external <name> { run ze.bgp-rr }`
	// reaches the registered runner exactly as `internal <name> { use bgp-rr }`
	// does (declaredTarget says why), so a scan that read only the internal/use
	// pair judged the alias name itself and found nothing pinned.
	declOpen    = regexp.MustCompile(`\b(?:internal|external)\s+(\S+)\s*\{(.*)$`)
	targetToken = regexp.MustCompile(`\b(?:use|run)\s+['"]?([A-Za-z0-9_.:-]+)`)
)

// textTarget names the plugin an attach block declares in its own body, or ""
// when it declares none. The tree reader's declaredTarget answers the same
// question from a parsed node; this one answers it from the block's text.
func textTarget(body string) string {
	m := targetToken.FindStringSubmatch(body)
	if m == nil {
		return ""
	}
	return strings.TrimPrefix(m[1], "ze.")
}

// unquoteName strips the quotes a captured name carries.
//
// The parser stores the value without them (config/tokenizer.go, readString), so
// the quoted spelling names the same plugin, while a raw `"bgp-rr"` matches no
// entry in noSentUpdates and would leave the block unread. Same reason the
// quotes come off a receive VALUE in listFields, one level down.
func unquoteName(s string) string {
	return strings.Trim(s, `"'`)
}

// scanText returns every received-only violation in one text the parser
// refused, and how many attach blocks naming a pinned plugin it checked.
//
// Finding is separated from asserting so the shape tests below can drive the
// whole scan over a config they write themselves and read the answer back. A
// guard whose only caller is a tree walk can only be tested by the tree, which
// is how three live shapes passed it.
func scanText(text string, loc where) (violations []string, checked int) {
	lines := strings.Split(text, "\n")
	alias := aliasMap(lines)

	// judge reads one attach block, whichever spelling opened it, and records
	// every receive token that grants a pinned plugin the sent direction.
	//
	// rest is whatever followed the opening brace on the line that opened the
	// block, which carries the whole block when it is written on one line. The
	// body is joined once and read twice: for the plugin the block names inline,
	// and for the receive list.
	judge := func(start int, name, rest string) {
		body := attachBody(lines, start, rest)
		plugin, reason, pinned := resolvePinned(alias, name, textTarget(body))
		if !pinned {
			return
		}
		checked++
		for _, tok := range listTokens(body, "receive") {
			violations = appendViolation(violations, loc.atLine(start+1), plugin, reason, tok)
		}
	}

	// attachDepth is the brace depth inside an `attach { }` container, and zero
	// outside one. It is what tells a `process <name> {` line that it opens an
	// attach block rather than naming something else.
	attachDepth := 0

	for i, line := range lines {
		// The CLI's `set` form, which a guide writes instead of a block. It is
		// the same grant and the same hazard, and a block-only walk is blind to
		// it: three `set bgp peer <x> attach process bgp-rr` lines in
		// docs/guide/flowspec-route-reflector.md carried no receive list at all.
		// A whole file of `set` lines is a document the parser reads (its own
		// format, config/serialize_set.go DetectFormat), so this matcher only
		// ever sees the `set` lines a guide writes between paragraphs of prose.
		if m := attachSet.FindStringSubmatch(line); m != nil {
			// No inline target: the `set` form names the process and its receive
			// list on one line and carries no block to declare a plugin in.
			plugin, reason, pinned := resolvePinned(alias, m[1], "")
			if !pinned {
				continue
			}
			checked++
			for tok := range strings.FieldsSeq(strings.Trim(m[2], "[] ")) {
				violations = appendViolation(violations, loc.atLine(i+1), plugin, reason, unquoteName(tok))
			}
			continue
		}
		if m := attachOpen.FindStringSubmatch(line); m != nil {
			judge(i, m[1], m[2])
			continue
		}
		// The nested spelling, `attach { process <name> { ... } }`. The parser
		// reads it and the flat one alike: ze:flatten changes what ze PRINTS, not
		// what it accepts (internal/component/config/flatten.go, and the
		// automatic brace insertion in parser.go it points at). Nothing in this
		// tree writes it today, and a guard cleared for a HANG must not depend on
		// nobody writing legal config.
		if m := attachNested.FindStringSubmatch(line); m != nil {
			if p := processOpen.FindStringSubmatch(m[1]); p != nil {
				judge(i, p[1], p[2])
			}
			attachDepth = 1 + braceDelta(m[1])
			continue
		}
		if attachDepth > 0 {
			if p := processOpen.FindStringSubmatch(line); p != nil {
				judge(i, p[1], p[2])
			}
			attachDepth += braceDelta(line)
			if attachDepth < 0 {
				attachDepth = 0
			}
		}
	}
	return violations, checked
}

// braceDelta is how much one line changes the brace depth.
func braceDelta(s string) int {
	return strings.Count(s, "{") - strings.Count(s, "}")
}

// aliasMap reads the `internal <alias> { use <plugin> }` and
// `external <alias> { run <plugin> }` declarations of one text, so a block
// naming the alias is judged against the plugin behind it.
//
// The declaration is written on one line as often as on two, and the one-line
// form is the one every guide uses. A `use` matcher anchored to the start of its
// own line therefore recorded no alias at all from `docs/guide/bgp-resilience.md`
// or `docs/guide/plugins.md`, and every block naming one went unchecked.
func aliasMap(lines []string) map[string]string {
	alias := map[string]string{}
	current := ""
	for _, line := range lines {
		if m := declOpen.FindStringSubmatch(line); m != nil {
			if u := targetToken.FindStringSubmatch(m[2]); u != nil {
				alias[unquoteName(m[1])] = strings.TrimPrefix(u[1], "ze.")
				current = ""
				continue
			}
			current = unquoteName(m[1])
			continue
		}
		if current == "" {
			continue
		}
		if u := targetToken.FindStringSubmatch(line); u != nil {
			alias[current] = strings.TrimPrefix(u[1], "ze.")
			current = ""
		}
	}
	return alias
}

// attachBody joins the attach block that opens at lines[start], stopping at the
// brace that CLOSES it rather than at the first closing brace inside it.
//
// A nested container is why the difference matters. `content { format parsed }`
// sits before `receive` in `test/plugin/check.ci` and in five
// `test/exabgp-compat/native/api-*.conf`, and a scan that stopped at the
// container's own brace never reached the receive list of any of them.
func attachBody(lines []string, start int, rest string) string {
	var b strings.Builder
	depth := 1
	consume := func(s string) bool {
		for _, c := range s {
			switch c {
			case '{':
				depth++
			case '}':
				depth--
				if depth == 0 {
					return true
				}
			}
			b.WriteRune(c)
		}
		b.WriteByte('\n')
		return false
	}
	if consume(rest) {
		return b.String()
	}
	for j := start + 1; j < len(lines); j++ {
		if consume(lines[j]) {
			break
		}
	}
	return b.String()
}

// newlineToken is the end of a line, kept as a token of its own so the bare
// `receive a` form stops where the line does.
const newlineToken = "\n"

// listFields splits an attach block body into tokens, with every bracket, brace
// and semicolon standing alone, the newline kept as a token, and the quotes
// stripped from a quoted value.
//
// The newline needs carrying by hand. strings.Fields treats it as whitespace
// and drops it, so a reader that pads it and then calls Fields has no line ends
// at all: the bare form then runs to the end of the block and reads the next
// key's values as its own grants.
//
// The quotes need stripping for the same reason the parser strips them:
// `receive "update";` is a legal grant of plain update, because the tokenizer
// stores the value without its quotes (config/tokenizer.go, readString) and a
// leaf-list takes a quoted string wherever it takes a word (parser_list.go).
// Handed the raw field, events.SplitTypeToken answers not-ok on `"update"` and
// the violation goes unreported.
func listFields(body string) []string {
	padded := strings.NewReplacer(
		"[", " [ ", "]", " ] ",
		"{", " { ", "}", " } ",
		";", " ; ",
	).Replace(body)

	var fields []string
	for line := range strings.SplitSeq(padded, "\n") {
		for f := range strings.FieldsSeq(line) {
			fields = append(fields, strings.Trim(f, `"'`))
		}
		fields = append(fields, newlineToken)
	}
	return fields
}

// listTokens returns the values of one list inside an attach block body.
//
// TWO spellings parse, and both are read here: a bracket list `receive [ a b ]`
// written on one line, and a bare `receive a;` ending at its semicolon or at the
// end of its line. A leaf-list value is a word or a quoted string
// (config/parser_list.go, parseValueOrArray).
//
// The brace form `receive { a b }` is read as well, and it does NOT parse:
// parseValueOrArray refuses the LBRACE with "expected value or ';' in receive".
// A bracket list wrapped over several lines does not parse either, because the
// tokenizer inserts a semicolon at each line end and the list sees it
// (config/tokenizer.go, insertSemi). Both are read anyway: reading a shape the
// parser refuses costs one comparison and fails CLOSED, while missing a shape it
// accepts is the defect this guard exists to prevent. Neither is evidence about
// the tree -- the five `test/exabgp-compat/native/api-*.conf` that write the
// brace form are excluded from parser coverage by name, as "legacy ExaBGP-style
// API example awaiting native conversion" (bgp/config/loader_test.go,
// legacyExampleFixtureReason).
func listTokens(body, keyword string) []string {
	fields := listFields(body)

	var out []string
	for i := 0; i < len(fields); i++ {
		if fields[i] != keyword {
			continue
		}
		j := i + 1
		for j < len(fields) && fields[j] == newlineToken {
			j++
		}
		if j >= len(fields) {
			break
		}
		closer := ""
		switch fields[j] {
		case "[":
			closer = "]"
		case "{":
			closer = "}"
		}
		if closer != "" {
			for j++; j < len(fields) && fields[j] != closer; j++ {
				if fields[j] != newlineToken && fields[j] != ";" {
					out = append(out, fields[j])
				}
			}
			i = j
			continue
		}
		for ; j < len(fields) && fields[j] != ";" && fields[j] != newlineToken; j++ {
			out = append(out, fields[j])
		}
		i = j
	}
	return out
}

// TestReceivedOnlyGuardReadsEveryShape is the guard's own guard.
//
// The cases divide in two, and the difference is stated per case rather than
// claimed for all of them. The first four are shapes that EXIST in this tree and
// that an earlier reader passed while granting plain `update` to a deadlocking
// plugin. The rest are shapes the PARSER accepts and nothing in the tree writes
// yet. A guard cleared for a hang must not depend on nobody writing legal
// config.
//
// Each case also declares which reader answers it. A case written as a whole
// config document goes through the parser, where the spelling stops mattering.
// A case written as a fragment -- a guide's snippet, a table cell -- has no
// document to parse, and the text scan is what reads it. The field is asserted,
// so a fragment that starts parsing, or a document that stops, is a failure
// rather than a silent change of reader.
//
// VALIDATES: the scan reads a receive list behind a nested container, a list in
// brace form, an alias declared on one line, a block inside a markdown table
// cell, a bare value, the nested `attach { process <name> { } }` spelling, a
// quoted value, and a quoted process or alias NAME.
// PREVENTS: a shape-blind rewrite of attachBody, listFields, aliasMap or
// unquoteName, which is worse than having no guard at all because this one is
// cited as R-15's clearance.
func TestReceivedOnlyGuardReadsEveryShape(t *testing.T) {
	cases := []struct {
		name      string
		viaParser bool
		config    string
	}{
		{
			// test/plugin/check.ci and five test/exabgp-compat/native/api-*.conf.
			// An earlier scan stopped at the container's closing brace. Written
			// as a guide writes it, without the peer it belongs to, so it is a
			// fragment here even though the file it came from parses.
			name: "nested container before the receive list",
			config: `
		attach process bgp-rr {
			content {
				format parsed
			}
			receive [ update ]
		}`,
		},
		{
			// An earlier scan read the opening brace as the granted type. The
			// form does not parse (listTokens says why), and it is read because
			// doing so fails closed: the five
			// test/exabgp-compat/native/api-*.conf that write it are legacy
			// fixtures excluded from parser coverage by name, and they attach
			// announce-routes rather than a pinned plugin, so they are not
			// evidence that anything live carries this shape.
			name: "brace form receive list",
			config: `
		attach process bgp-rs {
			receive {
				update
			}
		}`,
		},
		{
			// docs/guide/bgp-resilience.md, plugins.md, flowspec-route-reflector.md.
			// It LOOKS like a whole document and the parser refuses it: automatic
			// semicolon insertion fires at a newline, so a closing brace on the
			// same line as the statement it closes is a syntax error. Measured
			// with a built `ze config validate`: "line 2: expected ';' after use
			// value, got RBRACE". The text scan is what reads it, and it has to,
			// because this is the form four guides wrote.
			name: "alias declared on one line, which does not parse",
			config: `
plugin {
    internal rrx { use bgp-rr }
}
bgp {
    peer edge1 {
        attach process rrx { receive [ update ] }
    }
}`,
		},
		{
			// The same one-line spelling with the semicolons the parser wants.
			// This one IS valid config, so the parser answers it and the alias is
			// resolved from the tree rather than from a regex.
			name:      "one-line block closed by a semicolon",
			viaParser: true,
			config: `
plugin {
    internal rrx { use bgp-rr; }
}
bgp {
    peer edge1 {
        attach process rrx { receive [ update ]; }
    }
}`,
		},
		{
			// A guide writes the block in a table cell, which puts a pipe in front
			// of the keyword. Nothing about a table row is a config document.
			name:   "block inside a markdown table cell",
			config: "| `attach process bgp-rpki { receive [ update ] }` | grants both directions |",
		},
		{
			// leaf-list receive carries no ze:syntax extension, so yangToNode
			// builds a ValueOrArrayNode for it and the parser takes a bare
			// `receive <value>;` as readily as a bracket list
			// (internal/component/config/parser_list.go, parseValueOrArray).
			name: "bare value form",
			config: `
		attach process bgp-rpki-decorator {
			receive update;
		}`,
		},
		{
			// The nested spelling of the same block. ze:flatten decides how ze
			// PRINTS an attach block; the parser accepts both spellings
			// (internal/component/config/flatten.go).
			name: "nested attach container",
			config: `
		attach {
			process bgp-rr {
				receive [ update ]
			}
		}`,
		},
		{
			// The same nested spelling written on one line, which is how a guide
			// or a table cell would carry it.
			name:   "nested attach container on one line",
			config: `attach { process bgp-rs { receive [ update ] } }`,
		},
		{
			// A quoted value is the same grant: the tokenizer stores `update`
			// without its quotes, so the config is valid and the grant is plain.
			name: "quoted value",
			config: `
		attach process bgp-rpki {
			receive "update";
		}`,
		},
		{
			// And inside a bracket list, where the quotes sit around the member.
			name: "quoted member in a bracket list",
			config: `
		attach process bgp-rpki-decorator {
			receive [ "update" ]
		}`,
		},
		{
			// The process NAME quoted, which is the same block: parseList takes a
			// list key from a word or from a quoted string
			// (config/parser_list.go), and the tokenizer stores it without the
			// quotes. Read raw, `"bgp-rr"` matches no pinned plugin and the whole
			// block goes unchecked -- a silent pass on the deadlock this guard
			// clears.
			name: "quoted process name",
			config: `
		attach process "bgp-rr" {
			receive [ update ]
		}`,
		},
		{
			// The alias declaration quoted on both sides, so the alias key, the
			// plugin behind it and the block naming it are each read through the
			// quotes. Written one-line without its semicolons, as the guides
			// wrote it, so the parser refuses it and the text scan answers.
			name: "quoted alias and quoted block name",
			config: `
plugin {
    internal "rrx" { use "bgp-rs" }
}
bgp {
    peer edge1 {
        attach process "rrx" { receive [ update ] }
    }
}`,
		},
		{
			// The CLI set form, written as a guide writes it. Set format is a
			// document of its own (config/serialize_set.go, DetectFormat), so
			// SetParser reads it and the tree is the same tree.
			name:      "set form",
			viaParser: true,
			config: `set plugin internal rrx use bgp-rr
set bgp peer edge1 attach process rrx receive [ update ]`,
		},
		{
			// The block declares its own plugin and nothing else in the config
			// mentions rrx. extractInlinePluginsFromMap (bgp/config/plugins.go)
			// builds the PluginConfig from this block and startInternal
			// (plugin/process/process.go) resolves `ze.bgp-rr` to the registered
			// runner, so this is the same deadlock as an alias. Both readers
			// answered not-pinned here until 2026-08-15.
			name:      "plugin declared inline on the attach block",
			viaParser: true,
			config: `
bgp {
    peer edge1 {
        attach process rrx {
            run ze.bgp-rr;
            receive [ update ];
        }
    }
}`,
		},
		{
			// The other half of the same hole: the declaration is in the plugin
			// block, under external rather than internal, and it says `run` rather
			// than `use`. `internal rrx { use ze.bgp-rr }` is refused by the
			// validator, so this is the shape that reaches bgp-rr through a `run`
			// declaration.
			name:      "external declaration with a run leaf",
			viaParser: true,
			config: `
plugin {
    external rrx {
        run ze.bgp-rr;
    }
}
bgp {
    peer edge1 {
        attach process rrx {
            receive [ update ];
        }
    }
}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			violations, checked, parsed := shapeScan(tc.config)
			require.Equal(t, tc.viaParser, parsed,
				"which reader answers this shape has changed; update the case or find out why")
			require.NotZero(t, checked,
				"the scan did not even recognize the attach block, so it can refuse nothing")
			require.NotEmpty(t, violations,
				"a plain `update` grant to a received-only plugin must be refused in this shape")
		})
	}

	// The converse, so the shapes above are not passing for want of discrimination:
	// the same shapes with a direction-carrying token are legal and must stay silent.
	t.Run("update-received in every shape is accepted", func(t *testing.T) {
		for _, tc := range cases {
			legal := strings.ReplaceAll(tc.config, "receive [ update ]", "receive [ update-received ]")
			legal = strings.ReplaceAll(legal, "receive update;", "receive update-received;")
			legal = strings.ReplaceAll(legal, `receive "update";`, `receive "update-received";`)
			legal = strings.ReplaceAll(legal, `receive [ "update" ]`, `receive [ "update-received" ]`)
			legal = strings.ReplaceAll(legal, "receive {\n\t\t\t\tupdate", "receive {\n\t\t\t\tupdate-received")
			violations, checked, _ := shapeScan(legal)
			require.NotZero(t, checked, tc.name)
			require.Empty(t, violations, "%s: update-received is the correct spelling and must pass", tc.name)
		}
	})
}

// TestReceivedOnlyGuardTextScanLimits states, as behavior rather than as a
// comment, what the text scan cannot see.
//
// The scan counts braces by iterating runes. It keeps no comment state and no
// string state (braceDelta, and the consume closure in attachBody), so a brace
// written inside a `#` comment moves the depth as a real one does. That is the
// class of hole four review rounds kept finding: each round patched a spelling,
// and the reader stayed a reader of spellings.
//
// VALIDATES: one named hole in the text scan, and that the parser closes THAT
// one. The same config is missed by the text scan and caught by the parser,
// which is the whole argument for reading the tree.
//
// It does NOT establish that the parser closes every hole, and this comment said
// it did until 2026-08-15. Two shapes that PARSE were missed by the parsed-tree
// reader itself for as long as it existed: a plugin declared inline on the
// attach block (`attach process rrx { run ze.bgp-rr; ... }`) and one declared
// under `external ... { run ... }`. Neither is a text-scan limit. Both are cases
// in TestReceivedOnlyGuardReadsEveryShape now, which is where a claim about what
// the readers see belongs -- an assertion the tree can fail, not a sentence.
// PREVENTS: a comment claiming the text scan is complete. It is not, it cannot
// be, and a guard cited as R-15's clearance must not be believed further than it
// reads.
func TestReceivedOnlyGuardTextScanLimits(t *testing.T) {
	// A closing brace inside a comment. The scan ends the attach block at it and
	// never reaches the receive list below.
	const fragment = `
		attach process bgp-rr {
			# closing the example: }
			receive [ update ]
		}`

	violations, checked, parsed := shapeScan(fragment)
	require.False(t, parsed, "a fragment has no document to parse")
	require.NotZero(t, checked, "the block itself is recognized")
	require.Empty(t, violations,
		"pinning a KNOWN hole: a brace in a comment truncates the block the text scan reads")

	// The same grant inside a document. The parser has comment state, so the
	// tree carries the receive list and the guard refuses it.
	document := "bgp {\n    peer edge1 {" + fragment + "\n    }\n}"
	violations, checked, parsed = shapeScan(document)
	require.True(t, parsed, "this IS a document, so the parser must answer it")
	require.NotZero(t, checked, "the parsed tree must carry the attach block")
	require.NotEmpty(t, violations, "the parser sees the grant the text scan missed")
}

// shapeScan drives one hand-written config through the same two readers the
// tree walk uses, and says which one answered.
func shapeScan(text string) (violations []string, checked int, parsed bool) {
	var c counts
	var p parseCounts
	violations = scanOneDocument(text, wholeFile(), &c, &p)
	return violations, c.tree + c.text, p.documents > 0
}

// TestReceivedOnlyGuardStopsAtTheEndOfABareValue pins where a bare grant ends.
//
// VALIDATES: a bare `receive <value>` reads one list, not the rest of the block.
// PREVENTS: a tokenizer that drops the line ends, which is what strings.Fields
// does to a padded newline. The bare grant then runs on into the next key and
// reads `send [ update ]` as a receive grant, so a correctly written block is
// reported as the deadlock it does not have. A guard cited as clearance must not
// invent the violation it exists to find.
func TestReceivedOnlyGuardStopsAtTheEndOfABareValue(t *testing.T) {
	// Correct: bgp-rr is fed the received direction only, and separately allowed
	// to reflect. Nothing here grants it the UPDATEs ze sends. Written as a
	// fragment, which is what puts it on the text scan.
	const config = `
		attach process bgp-rr {
			receive update-received
			send [ update ]
		}`

	violations, checked, parsed := shapeScan(config)
	require.False(t, parsed, "a fragment has no document to parse")
	require.NotZero(t, checked, "the scan did not recognize the attach block")
	require.Empty(t, violations, "the send list is not a receive grant")
}
