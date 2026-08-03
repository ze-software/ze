// Design: ai/rules/cli.md -- dispatch-key migration leaves dead callers
//
// ci_dispatch_commands enforces the invariant the verb-first CLI migration broke
// without anything noticing: EVERY command string a test, script, or Go call
// site sends to the daemon must still resolve to a registered command.
//
// The dispatcher registers each built-in under its YANG PATH, so moving a
// container in the command tree deletes the old dispatch key outright
// (ai/rules/cli.md, "Migrating a Built-in Command's Path"). The
// declaration side is gated -- `make ze-cli-grammar-check` proves every DECLARED
// command is verb-first -- but nothing checked the CALL SITES, so eleven
// emitters kept sending `peer <n> detail`, `summary`, `bgp health` and
// `daemon reload` long after those keys ceased to exist. Six `.ci` tests passed
// anyway, because an observer failure never reached the runner.
//
// Resolution is delegated to the REAL matcher: this builds a
// pluginserver.Dispatcher from the live YANG command tree plus the registered
// RPC set and calls Dispatcher.Resolves, the matching half of Dispatch. There is
// deliberately no second copy of matchCommandTokens here -- a checker that
// reimplemented inline-selector matching would drift from the dispatcher and
// start lying (ai/rules/evidence.md).
//
// Fail-closed on ambiguity: a command string this checker cannot evaluate
// statically (concatenation, an f-string interpolation, a variable) is reported
// as UNVERIFIABLE and fails the gate as loudly as a dead one. Staying silent on
// a string it could not read would be the same blind spot in a new place.
// Genuinely dynamic emitters carry an explicit marker stating a reason.
//
// Usage:     go run scripts/checks/ci_dispatch_commands.go [--json|--selftest]
// Called by: make ze-ci-dispatch-check (routed onto the verify path by
//            scripts/dev/verify_wiring_docs.py when a .ci/.py/.go emitter or a
//            command YANG file changes) and
//            scripts/checks/ci_dispatch_commands_test.go

//go:build ignore

package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	// Blank import triggers every init() registration, so the command surface
	// matches the runtime one exactly (same import set as
	// scripts/inventory/commands.go).
	_ "github.com/ze-software/ze/internal/component/plugin/all"

	"github.com/ze-software/ze/internal/component/command"
	yangloader "github.com/ze-software/ze/internal/component/config/yang"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// Finding is one emitter that does not resolve.
type Finding struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Kind    string `json:"kind"` // "dead" or "unverifiable"
	Emitter string `json:"emitter"`
	Command string `json:"command"`
	Detail  string `json:"detail,omitempty"`
}

// dynamicMarker exempts a genuinely non-static emitter. It must state a reason,
// so an exemption is a decision on the record rather than a quiet skip.
const dynamicMarker = "ze-dispatch-check: dynamic"

// scanRoots are the trees that hold command emitters.
var scanRoots = []string{"test", "internal", "cmd", "pkg", "scripts", "demos"}

// draftTestDir is the incubator for functional tests under development. It is
// gitignored and invisible to every repo-wide gate, so a half-written .ci cannot
// redden this check (test/draft/README.md, internal/test/runner/draft_dir.go).
// Spelled literally rather than imported: this file is a standalone check under
// scripts/, and importing internal/test/runner here would cross a module tier.
const draftTestDir = "test/draft"

func main() {
	jsonMode := flag.Bool("json", false, "emit findings as JSON")
	selftest := flag.Bool("selftest", false, "run built-in fixture tests")
	flag.Parse()

	if *selftest {
		if err := runSelftest(); err != nil {
			fmt.Fprintln(os.Stderr, "selftest:", err)
			os.Exit(1)
		}
		fmt.Fprintln(os.Stdout, "ci-dispatch-check selftest: OK")
		return
	}

	resolver, keys, err := newResolver()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error: build command surface:", err)
		os.Exit(1)
	}

	findings, scanned, passthroughs := scanAll(resolver, keys)
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].File != findings[j].File {
			return findings[i].File < findings[j].File
		}
		return findings[i].Line < findings[j].Line
	})

	if *jsonMode {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(map[string]any{ //nolint:errcheck // stdout write
			"schema-version":   1,
			"commands-known":   len(keys),
			"emitters-checked": scanned,
			"pass-through":     passthroughs,
			"findings":         findings,
		})
		if len(findings) > 0 {
			os.Exit(1)
		}
		return
	}

	report(os.Stdout, findings, len(keys), scanned, passthroughs)
	if len(findings) > 0 {
		os.Exit(1)
	}
}

// newResolver builds a Dispatcher carrying every registered command path with
// its YANG-declared ArgDefs, so inline-selector forms match exactly as they do
// at runtime. Returns the dispatcher and the number of distinct keys.
func newResolver() (*pluginserver.Dispatcher, []string, error) {
	loader, err := yangloader.DefaultLoader()
	if err != nil {
		return nil, nil, fmt.Errorf("load YANG: %w", err)
	}
	wireToPaths := yangloader.WireMethodToPaths(loader)
	pathToArgDefs := yangloader.PathToArgDefs(loader)

	d := pluginserver.NewDispatcher()
	seen := make(map[string]bool)
	register := func(path string, defs []command.ArgDef) {
		if path == "" || seen[path] {
			return
		}
		seen[path] = true
		d.RegisterWithOptions(path, nil, "", pluginserver.RegisterOptions{ArgDefs: defs})
	}

	for _, rpc := range pluginserver.AllBuiltinRPCs() {
		for _, path := range wireToPaths[rpc.WireMethod] {
			register(path, pathToArgDefs[path])
		}
	}
	// Streaming handlers ("monitor event") and the TUI-only dashboard
	// ("monitor bgp") are dispatchable but are not builtin RPCs.
	for _, prefix := range pluginserver.StreamingPrefixes() {
		register(prefix, pathToArgDefs[prefix])
	}
	register("monitor bgp", pathToArgDefs["monitor bgp"])

	// The SSH exec middleware intercepts these THREE before the dispatcher ever
	// sees them and routes them to shutdownFunc/restartFunc/rebootFunc
	// (internal/component/ssh/ssh.go, execMiddleware). Its own comment records
	// that the dispatcher deliberately registers no key for them, so a gate that
	// only knows the dispatcher would call `ze signal stop` dead and push someone
	// into "fixing" a command that works. They are a real surface, not an
	// exemption.
	for _, lifecycle := range []string{"stop", "restart", "reboot"} {
		register(lifecycle, nil)
	}

	// Plugin commands are declared in sdk.CommandDecl literals and only reach a
	// registry when a plugin completes stage 1, so they cannot be read from a
	// registry at check time. Parsing the literals is the only static source; a
	// name that is not a literal is a hard error rather than a skip, because an
	// unread declaration would turn every legitimate use of that command into a
	// false "dead" finding.
	decls, declErrs := pluginCommandDecls()
	for _, name := range decls {
		register(name, nil)
	}
	if len(declErrs) > 0 {
		return nil, nil, fmt.Errorf("plugin CommandDecl names not statically readable: %s",
			textbuf.Join(declErrs, "; "))
	}

	if len(seen) == 0 {
		return nil, nil, fmt.Errorf("no commands registered -- refusing to pass every emitter vacuously")
	}
	keys := make([]string, 0, len(seen))
	for k := range seen {
		keys = append(keys, strings.ToLower(k))
	}
	sort.Strings(keys)
	return d, keys, nil
}

// pluginCommandDecls parses `sdk.CommandDecl{Name: "..."}` literals out of the
// tree. Returns the names, plus the sites whose Name is not a string literal.
func pluginCommandDecls() ([]string, []string) {
	var names []string
	var errs []string
	fset := token.NewFileSet()

	// Several plugins name their commands with package constants
	// (`{Name: cmdShowVRRP}`), so a literal-only reader would report them as
	// unreadable and fail the gate on a perfectly good pattern. Collect every
	// package-level string constant first, keyed by package directory, and
	// resolve identifier values against it.
	constsByDir := stringConstsByDir(fset)

	for _, root := range []string{"internal", "pkg"} {
		_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error { //nolint:errcheck // per-file
			if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") {
				return nil
			}
			if strings.HasSuffix(path, "_test.go") {
				return nil
			}
			src, readErr := os.ReadFile(path) //nolint:gosec // repo-relative walk
			if readErr != nil || !strings.Contains(string(src), "CommandDecl") {
				return nil
			}
			file, parseErr := parser.ParseFile(fset, path, src, 0)
			if parseErr != nil {
				return nil
			}
			ast.Inspect(file, func(n ast.Node) bool {
				lit, ok := n.(*ast.CompositeLit)
				if !ok || !isCommandDeclType(lit.Type) {
					return true
				}
				// In `[]sdk.CommandDecl{{Name: "..."}, ...}` the ELEMENT literals
				// carry no type of their own (Go elides it), so the KeyValue
				// pairs live one level down. Flatten both shapes: the slice form
				// and a bare `sdk.CommandDecl{Name: "..."}`.
				fields := make([]ast.Expr, 0, len(lit.Elts))
				for _, elt := range lit.Elts {
					if inner, isLit := elt.(*ast.CompositeLit); isLit {
						fields = append(fields, inner.Elts...)
						continue
					}
					fields = append(fields, elt)
				}
				for _, elt := range fields {
					kv, isKV := elt.(*ast.KeyValueExpr)
					if !isKV {
						continue
					}
					if id, isID := kv.Key.(*ast.Ident); !isID || id.Name != "Name" {
						continue
					}
					name, isLit := stringLit(kv.Value)
					if !isLit {
						if id, isID := kv.Value.(*ast.Ident); isID {
							if v, known := constsByDir[filepath.Dir(path)][id.Name]; known {
								names = append(names, v)
								continue
							}
						}
						pos := fset.Position(kv.Value.Pos())
						var tb textbuf.Buffer
						errs = append(errs, tb.Str(pos.Filename).Byte(':').Int(int64(pos.Line)).String())
						continue
					}
					names = append(names, name)
				}
				return true
			})
			return nil
		})
	}
	return names, errs
}

// stringConstsByDir collects package-level `const`/`var` string literals keyed
// by package directory, so a CommandDecl naming its command with a constant
// resolves instead of failing the gate.
func stringConstsByDir(fset *token.FileSet) map[string]map[string]string {
	out := make(map[string]map[string]string)
	for _, root := range []string{"internal", "pkg"} {
		_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error { //nolint:errcheck // per-file
			if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") {
				return nil
			}
			src, readErr := os.ReadFile(path) //nolint:gosec // repo-relative walk
			if readErr != nil {
				return nil
			}
			file, parseErr := parser.ParseFile(fset, path, src, 0)
			if parseErr != nil {
				return nil
			}
			dir := filepath.Dir(path)
			for _, decl := range file.Decls {
				gd, isGen := decl.(*ast.GenDecl)
				if !isGen || (gd.Tok != token.CONST && gd.Tok != token.VAR) {
					continue
				}
				for _, spec := range gd.Specs {
					vs, isVal := spec.(*ast.ValueSpec)
					if !isVal {
						continue
					}
					for i, ident := range vs.Names {
						if i >= len(vs.Values) {
							continue
						}
						if lit, isLit := stringLit(vs.Values[i]); isLit {
							if out[dir] == nil {
								out[dir] = make(map[string]string)
							}
							out[dir][ident.Name] = lit
						}
					}
				}
			}
			return nil
		})
	}
	return out
}

func isCommandDeclType(e ast.Expr) bool {
	switch t := e.(type) {
	case *ast.SelectorExpr:
		return t.Sel.Name == "CommandDecl"
	case *ast.Ident:
		return t.Name == "CommandDecl"
	case *ast.ArrayType:
		return isCommandDeclType(t.Elt)
	}
	return false
}

func stringLit(e ast.Expr) (string, bool) {
	bl, ok := e.(*ast.BasicLit)
	if !ok || bl.Kind != token.STRING {
		return "", false
	}
	s, err := strconv.Unquote(bl.Value)
	if err != nil {
		return "", false
	}
	return s, true
}

// --- emitter recognition -------------------------------------------------

// pyEmitters match the Python/.ci observer shapes that send a command string to
// the dispatcher. Group 1 is the emitter name, group 2 the command argument.
var pyEmitters = []*regexp.Regexp{
	regexp.MustCompile(`\b(dispatch)\s*\(\s*api\s*,\s*(.+?)\s*\)`),
	regexp.MustCompile(`\b(?:api\.)?(dispatch_until_done)\s*\(\s*(.+?)\s*[,)]`),
	regexp.MustCompile(`\bapi\.(dispatch|send|dispatch_until)\s*\(\s*(.+?)\s*[,)]`),
	regexp.MustCompile(`(?:^|[^.\w])(?:^|[^f])(send)\s*\(\s*(.+?)\s*\)`),
	regexp.MustCompile(`\b(dispatch_until)\s*\(\s*api\s*,\s*(.+?)\s*,`),
}

// goEmitters match Go call sites that send a command string to the dispatcher.
var goEmitters = []*regexp.Regexp{
	regexp.MustCompile(`\b(DispatchCommand)\s*\(\s*[^,]+,\s*(.+?)\s*[,)]`),
	regexp.MustCompile(`\bsshclient\.(ExecCommand)\s*\(\s*[^,]+,\s*(.+?)\s*\)`),
	regexp.MustCompile(`\b(SendCommand)\s*\(\s*(.+?)\s*\)`),
	regexp.MustCompile(`\b(commandExecutor)\s*\(\s*(.+?)\s*\)`),
}

// pyLiteral matches a whole-argument single-quoted or double-quoted literal.
var pyLiteral = regexp.MustCompile(`^(?:'([^'\\]*)'|"([^"\\]*)")$`)

// passThrough matches an argument that is a bare variable or field: the command
// was decided elsewhere and this call only forwards it (`SendCommand(input)` in
// the CLI client forwards whatever the operator typed). Such a site emits no
// fixed command, so there is nothing for this gate to resolve -- it is counted
// and reported, never failed. An argument that BUILDS a command from literals
// (concatenation, a textbuf chain, a format string) is a different thing
// entirely and is failed as unverifiable: that is the shape every defect this
// gate exists for took.
var passThrough = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*)*$`)

// literal extracts a string literal from an argument expression. ok=false means
// the value is not statically knowable at this call site.
func literal(arg string) (string, bool) {
	m := pyLiteral.FindStringSubmatch(strings.TrimSpace(arg))
	if m == nil {
		return "", false
	}
	if m[1] != "" {
		return m[1], true
	}
	return m[2], true
}

// skipCommand reports strings that are not dispatcher command paths at all: the
// plugin-session wire directives and the route-injection text bridge, both of
// which are category-exempt from the grammar gate for the same reason.
func skipCommand(cmd string) bool {
	if cmd == "" {
		return true
	}
	for _, p := range []string{"plugin ", "subscribe ", "announce ", "withdraw "} {
		if strings.HasPrefix(cmd, p) {
			return true
		}
	}
	return false
}

// isComment reports a line whose content is a Go or Python comment. Prose in a
// docstring or a `//` comment is not an emitter, and reading it as one produced
// findings like send("command: str") from a function signature in a docstring.
func isComment(line string) bool {
	trimmed := strings.TrimSpace(line)
	return strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "#")
}

// isDeclaration reports a function DEFINITION rather than a call. Without this
// the parameter list reads as an argument: `func (c *cliClient) SendCommand(
// command string)` was reported as an emitter sending the command "command
// string".
func isDeclaration(line string) bool {
	trimmed := strings.TrimSpace(line)
	return strings.HasPrefix(trimmed, "func ") || strings.HasPrefix(trimmed, "def ") ||
		strings.HasPrefix(trimmed, "async def ")
}

// startsString reports whether an argument expression begins with a string
// literal (or an f-string), which is what a command argument looks like.
func startsString(arg string) bool {
	a := strings.TrimSpace(arg)
	a = strings.TrimPrefix(a, "f")
	return strings.HasPrefix(a, "'") || strings.HasPrefix(a, "\"")
}

// usesZeAPI reports whether a Python/.ci source talks to the daemon through
// ze_api at all. The bare `send(...)` emitter is only meaningful there: other
// scripts define their own unrelated send() (qemu-run.py sends console input,
// post_weekly.py sends Discord chunks) and matching those was noise, not signal.
func usesZeAPI(src string) bool {
	return strings.Contains(src, "ze_api") || strings.Contains(src, "from ze_api import")
}

// leadingLiteral returns the static text at the START of a computed command
// argument: everything before the first interpolation, concatenation or call.
// `f'request log level {sub} debug'` yields "request log level"; a textbuf chain
// `tb.Str("show bgp peer ").Str(addr)` yields "show bgp peer".
//
// The static prefix is the part a command-tree migration breaks. Checking it
// turns the large class of legitimately-interpolated commands from unverifiable
// noise (which would get the gate routed around) into a real assertion on the
// only part that can be checked, while a computed command with NO static prefix
// stays unverifiable and fails.
var firstLiteral = regexp.MustCompile(`'([^'\\]*)'|"([^"\\]*)"`)

func leadingLiteral(arg string) string {
	m := firstLiteral.FindStringSubmatch(arg)
	if m == nil {
		return ""
	}
	lit := m[1]
	if lit == "" {
		lit = m[2]
	}
	// An f-string's interpolation ends the static run.
	if idx := strings.Index(lit, "{"); idx >= 0 {
		lit = lit[:idx]
	}
	return strings.TrimSpace(lit)
}

// resolves reports whether the dispatcher could route cmd.
//
// It asks the REAL dispatcher rather than reimplementing matchCommandTokens
// (ai/rules/evidence.md), through already-exported API only: every
// command is registered here with a nil handler, so a matched command returns
// StatusDone and an unmatched one returns ErrUnknownCommand. Any OTHER error --
// "requires a selector", an argument-validation complaint -- means the command
// EXISTS and this checker simply did not supply realistic arguments, which is
// not what the gate is asking about.
func resolves(d *pluginserver.Dispatcher, cmd string) bool {
	_, err := d.Dispatch(nil, cmd)
	return !errors.Is(err, pluginserver.ErrUnknownCommand)
}

// sendRewriteResolves models the ONE rewrite ze_api performs on the caller's
// behalf: API.send() turns `peer <sel> <lifecycle-action>` into
// `request peer <sel> <action>` before dispatching (test/scripts/ze_api.py,
// send/_is_peer_action). Tests written against that helper legitimately spell
// the peer form, so resolving the pre-rewrite string is what the gate must do --
// otherwise it reports teardown-cmd.ci and teardown-msg.ci as dead when they
// are correct.
func sendRewriteResolves(d *pluginserver.Dispatcher, emitter, cmd string) bool {
	if emitter != "send" || !strings.HasPrefix(cmd, "peer ") {
		return false
	}
	var tb textbuf.Buffer
	return resolves(d, tb.Str("request ").Str(cmd).String())
}

// prefixKnown reports whether prefix is the start of some registered command,
// or is itself a resolvable command with trailing arguments.
func prefixKnown(resolver *pluginserver.Dispatcher, keys []string, prefix string) bool {
	if prefix == "" {
		return false
	}
	if resolves(resolver, prefix) {
		return true
	}
	lower := strings.ToLower(prefix)
	for _, k := range keys {
		if strings.HasPrefix(k, lower) {
			return true
		}
	}
	return false
}

// emittersFor decides how a file is scanned: which emitter shapes apply, whether
// the ze_api scoping rule applies (Python only), and whether the file is scanned
// at all.
//
// UNIT TESTS OF THE TOOLING ARE NOT EMITTERS. `_test.go` was excluded from the
// first version for a reason that applies verbatim to `_test.py`, and leaving the
// Python half in was an oversight, not a policy: both name a test that runs
// IN-PROCESS against a fake, never against a daemon. test/scripts/ze_api_test.py
// replaces `api._call_engine` with a function that raises, then dispatches
// "show bgp nonsense" precisely BECAUSE the string must not resolve -- that is the
// unknown-command path under test. scripts/dev/ci_observer_recover_check_test.py
// passes observer source as triple-quoted FIXTURE text to a scanner. Neither
// string ever reaches a dispatcher, so neither can rot the way this gate exists to
// catch: a dead command there fails nothing and, more to the point, false-passes
// nothing. The daemon-driving Python surface -- .ci observer blocks,
// test/perf/run.py, test/scripts/ze_api.py itself -- carries no `_test.py` suffix
// and stays fully scanned.
//
// The rule is deliberately the file-name convention rather than a per-file
// allowlist: `python3 <file>_test.py` under TestPythonUnitTests
// (scripts/dev/python_tests_test.go) is what "unit test" means here, and an
// allowlist would need a human to maintain it. runSelftest asserts both halves of
// this decision, so widening the exemption cannot happen silently.
func emittersFor(path string) (emitters []*regexp.Regexp, pyScoped, skip bool) {
	switch filepath.Ext(path) {
	case ".ci", ".py":
		if strings.HasSuffix(path, "_test.py") {
			return nil, false, true
		}
		return pyEmitters, true, false
	case ".go":
		if strings.HasSuffix(path, "_test.go") {
			return nil, false, true
		}
		return goEmitters, false, false
	}
	return nil, false, true
}

func scanAll(resolver *pluginserver.Dispatcher, keys []string) ([]Finding, int, int) {
	var findings []Finding
	scanned := 0
	passthroughs := 0

	for _, root := range scanRoots {
		_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error { //nolint:errcheck // per-file
			if err != nil {
				return nil
			}
			// test/draft/ holds functional tests under development and is
			// invisible to every repo-wide gate, this one included
			// (test/draft/README.md). Pruned at the directory so nothing inside
			// is read.
			if info.IsDir() {
				if filepath.ToSlash(path) == draftTestDir {
					return filepath.SkipDir
				}
				return nil
			}
			emitters, pyScoped, skip := emittersFor(path)
			if skip {
				return nil
			}
			src, readErr := os.ReadFile(path) //nolint:gosec // repo-relative walk
			if readErr != nil {
				return nil
			}
			if pyScoped && !usesZeAPI(string(src)) {
				return nil
			}
			f, n, pt := scanFile(resolver, keys, path, string(src), emitters)
			findings = append(findings, f...)
			scanned += n
			passthroughs += pt
			return nil
		})
	}
	return findings, scanned, passthroughs
}

func scanFile(resolver *pluginserver.Dispatcher, keys []string, path, src string, emitters []*regexp.Regexp) ([]Finding, int, int) {
	var findings []Finding
	scanned := 0
	passthroughs := 0
	lines := strings.Split(src, "\n")

	for i, line := range lines {
		if isComment(line) || isDeclaration(line) {
			continue
		}
		if strings.Contains(line, dynamicMarker) {
			continue
		}
		if i > 0 && strings.Contains(lines[i-1], dynamicMarker) {
			continue
		}
		for _, re := range emitters {
			for _, m := range re.FindAllStringSubmatch(line, -1) {
				emitter, arg := m[1], m[2]
				// The bare `send(...)` name is not reserved: .ci scripts define
				// local helpers with the same name and a different signature
				// (`send(master, command, transcript)`). Only treat it as a
				// daemon emitter when its first argument opens a string.
				if emitter == "send" && !startsString(arg) {
					continue
				}
				scanned++
				cmd, ok := literal(arg)
				if !ok {
					if passThrough.MatchString(strings.TrimSpace(arg)) {
						passthroughs++
						continue
					}
					// Computed command: check the static prefix, which is the
					// part a command-tree migration invalidates.
					prefix := leadingLiteral(arg)
					if prefix != "" && (skipCommand(prefix) || prefixKnown(resolver, keys, prefix) || sendRewriteResolves(resolver, emitter, prefix)) {
						continue
					}
					kind, detail := "unverifiable", "no static prefix to check; make the command path literal or mark the line `"
					if prefix != "" {
						kind, detail = "dead", "static command prefix matches no registered command; the dispatcher answers ErrUnknownCommand. Mark the line `"
					}
					var tb textbuf.Buffer
					findings = append(findings, Finding{
						File: path, Line: i + 1, Kind: kind,
						Emitter: emitter, Command: strings.TrimSpace(arg),
						Detail: tb.Str(detail).Str(dynamicMarker).Str(" -- <reason>` only if it is genuinely uncheckable").String(),
					})
					continue
				}
				if skipCommand(cmd) || resolves(resolver, cmd) || sendRewriteResolves(resolver, emitter, cmd) {
					continue
				}
				findings = append(findings, Finding{
					File: path, Line: i + 1, Kind: "dead",
					Emitter: emitter, Command: cmd,
					Detail: "resolves to no registered command; the dispatcher answers ErrUnknownCommand",
				})
			}
		}
	}
	return findings, scanned, passthroughs
}

func report(w io.Writer, findings []Finding, keyCount, scanned, passthroughs int) {
	var tb textbuf.Buffer
	fmt.Fprintln(w, "# Dispatch Command Call-Site Gate")
	fmt.Fprintln(w)
	fmt.Fprintln(w, tb.Str("Registered commands: ").Int(int64(keyCount)).String())
	fmt.Fprintln(w, tb.Reset().Str("Emitters checked:    ").Int(int64(scanned)).String())
	fmt.Fprintln(w, tb.Reset().Str("Pass-through (var):  ").Int(int64(passthroughs)).String())
	fmt.Fprintln(w)

	if len(findings) == 0 {
		fmt.Fprintln(w, "ci-dispatch-check: OK")
		return
	}
	dead, unver := 0, 0
	for _, f := range findings {
		if f.Kind == "dead" {
			dead++
		} else {
			unver++
		}
	}
	for _, f := range findings {
		fmt.Fprintln(w, tb.Reset().Str(f.File).Byte(':').Int(int64(f.Line)).Str(": ").
			Str(f.Kind).Str(": ").Str(f.Emitter).Byte('(').Quoted(f.Command).Byte(')').String())
		fmt.Fprintln(w, tb.Reset().Str("    ").Str(f.Detail).String())
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, tb.Reset().Str("ci-dispatch-check: FAIL (").Int(int64(dead)).
		Str(" dead, ").Int(int64(unver)).Str(" unverifiable)").String())
}

// runSelftest exercises the resolver and the emitter recogniser on fixtures, so
// a regression in either shows up as a failure rather than as silence.
func runSelftest() error {
	resolver, keys, err := newResolver()
	if err != nil {
		return err
	}
	if len(keys) < 100 {
		return fmt.Errorf("only %d commands registered; the surface did not load", len(keys))
	}

	for _, live := range []string{
		"show bgp summary", "show bgp peer list", "show bgp peer 10.0.0.1 detail",
		"request peer 10.0.0.1 flush", "request reload", "show status", "request halt",
		"monitor bgp", "show bgp health", "show runtime memory",
		"peer * update text nlri ipv4/unicast add 10.0.0.0/24",
	} {
		if !resolves(resolver, live) {
			return fmt.Errorf("live command %q did not resolve", live)
		}
	}
	for _, dead := range []string{
		"summary", "peer 10.0.0.1 detail", "peer 10.0.0.1 flush", "peer * list",
		"daemon reload", "daemon status", "daemon quit", "reload",
		"bgp health", "runtime memory", "bgp monitor", "bgp summary",
		"rpki status", "adj-rib-in show",
	} {
		if resolves(resolver, dead) {
			return fmt.Errorf("removed command %q still resolved", dead)
		}
	}

	fixture := strings.Join([]string{
		// A removed spelling, verbatim: the defect this gate exists for.
		"    resp = dispatch(api, 'bgp health')",
		// A live command: must not be flagged.
		"    resp = dispatch(api, 'show bgp summary')",
		// A pass-through variable: no fixed command, counted not failed.
		"    resp = dispatch(api, cmd_from_a_variable)",
		// Computed but with a checkable static prefix that is live.
		"    resp = dispatch(api, 'show bgp peer ' + addr + ' detail')",
		// Computed with a static prefix that is DEAD: must be caught.
		"    resp = dispatch(api, 'bgp health for ' + addr)",
		// Computed with no static prefix at all: unverifiable, fails loudly.
		"    resp = dispatch(api, build_command(kind))",
		// The documented escape hatch, which must actually exempt.
		"    # ze-dispatch-check: dynamic -- table driven, each entry checked below",
		"    resp = dispatch(api, build_command(other))",
	}, "\n")
	got, _, passthroughs := scanFile(resolver, keys, "fixture.ci", fixture, pyEmitters)
	if len(got) != 3 {
		return fmt.Errorf("fixture: expected 3 findings, got %d: %+v", len(got), got)
	}
	if got[0].Kind != "dead" || got[0].Command != "bgp health" {
		return fmt.Errorf("fixture: expected dead 'bgp health', got %+v", got[0])
	}
	if got[1].Kind != "dead" {
		return fmt.Errorf("fixture: expected a dead static prefix to be caught, got %+v", got[1])
	}
	if got[2].Kind != "unverifiable" {
		return fmt.Errorf("fixture: expected a prefix-less computed command to be unverifiable, got %+v", got[2])
	}
	if passthroughs != 1 {
		return fmt.Errorf("fixture: expected 1 pass-through variable, got %d", passthroughs)
	}

	// The file-selection rule, asserted in BOTH directions. The scanned half is
	// what stops the unit-test exemption from quietly widening into "any file
	// with test in the name"; the skipped half is what stops it from being
	// deleted by someone who reads it as dead code.
	for _, scanned := range []string{
		"test/plugin/cursor-replay.ci",
		"test/scripts/ze_api.py",
		"test/perf/run.py",
		"internal/component/ssh/ssh.go",
		"scripts/dev/ci_observer_recover_check.py",
	} {
		if _, _, skip := emittersFor(scanned); skip {
			return fmt.Errorf("file selection: %q must be scanned but is skipped", scanned)
		}
	}
	for _, skipped := range []string{
		"test/scripts/ze_api_test.py",
		"scripts/dev/ci_observer_recover_check_test.py",
		"scripts/checks/ci_dispatch_commands_test.go",
		"docs/performance.md",
	} {
		if _, _, skip := emittersFor(skipped); !skip {
			return fmt.Errorf("file selection: %q must be skipped but is scanned", skipped)
		}
	}
	return nil
}
