// Design: docs/architecture/cli/command-namespacing.md -- dispatch-key migration leaves dead callers
//
// Package cidispatch enforces the invariant the verb-first CLI migration broke
// without anything noticing: EVERY command string a test, script, or Go call
// site sends to the daemon must still resolve to a registered command.
//
// The dispatcher registers each built-in under its YANG PATH, so moving a
// container in the command tree deletes the old dispatch key outright. The
// declaration side is gated -- the CLI grammar gate proves every DECLARED
// command is verb-first -- but nothing checked the CALL SITES, so eleven
// emitters kept sending `peer <n> detail`, `summary`, `bgp health` and `daemon
// reload` long after those keys ceased to exist. Six `.ci` tests passed anyway,
// because an observer failure never reached the runner.
//
// Fail-closed on ambiguity: a command string this checker cannot evaluate
// statically (concatenation, an f-string interpolation, a variable) is reported
// as UNVERIFIABLE and fails the gate as loudly as a dead one. Staying silent on
// a string it could not read would be the same blind spot in a new place.
// Genuinely dynamic emitters carry an explicit marker stating a reason.
//
// The walk is FLOORED and every read error is answered. The script this
// replaces walked its six roots relative to the working directory, dropped
// every walk error, and skipped a file it could not read: run anywhere but a
// checkout it reported `Emitters checked: 0` and `ci-dispatch-check: OK`.

package cidispatch

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// dynamicMarker exempts a genuinely non-static emitter. It must state a reason,
// so an exemption is a decision on the record rather than a quiet skip.
const dynamicMarker = "ze-dispatch-check: dynamic"

// scanRoots are the trees that hold command emitters.
var scanRoots = []string{"test", "internal", "cmd", "pkg", "scripts", "demos"}

// draftTestDir is the incubator for functional tests under development. It is
// gitignored and invisible to every repo-wide gate, so a half-written .ci
// cannot redden this check (test/draft/README.md). Spelled literally rather
// than imported: importing internal/test/runner here would cross a tier.
const draftTestDir = "test/draft"

// emitterFloor is the least emitters the walk must find before the gate
// believes it saw the tree. This checkout carried 1075 on 2026-08-26, so the
// floor fires on a tree that was never read rather than on one that shrank.
const emitterFloor = 300

// pyEmitters match the Python and .ci observer shapes that send a command
// string to the dispatcher. Group 1 is the emitter name, group 2 the command
// argument.
var pyEmitters = []*regexp.Regexp{
	regexp.MustCompile(`\b(dispatch)\s*\(\s*api\s*,\s*(.+?)\s*\)`),
	regexp.MustCompile(`\b(?:api\.)?(dispatch_until_done)\s*\(\s*(.+?)\s*[,)]`),
	regexp.MustCompile(`\bapi\.(dispatch|send|dispatch_until)\s*\(\s*(.+?)\s*[,)]`),
	// The two leading alternations bound the bare `send(` name: it must not be
	// a method call (`api.send`), and it must not be the tail of an f-string
	// prefix. gocritic reads the second `^` as dangling; it is not, because
	// each group is anchored independently and a match at the start of a line
	// is exactly what the first alternative admits.
	regexp.MustCompile(`(?:^|[^.\w])(?:^|[^f])(send)\s*\(\s*(.+?)\s*\)`), //nolint:gocritic // both anchors are load-bearing, see above
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

// firstLiteral finds the first quoted run in a computed command argument.
var firstLiteral = regexp.MustCompile(`'([^'\\]*)'|"([^"\\]*)"`)

// Check walks tree's emitters and answers every one that does not resolve.
//
// floor is a parameter rather than a constant because a fixture tree holds a
// handful of emitters: le passes emitterFloor and a test passes 0.
func Check(tree string, floor int) (Report, error) {
	surface, err := NewSurface(tree)
	if err != nil {
		return Report{}, err
	}
	return CheckWith(surface, tree, floor)
}

// CheckWith is Check against a surface the caller already built, which is what
// the selftest and the package test use rather than relinking the product.
func CheckWith(surface Surface, tree string, floor int) (Report, error) {
	report := Report{SchemaVersion: 1, CommandsKnown: len(surface.keys)}

	for _, root := range scanRoots {
		err := filepath.WalkDir(filepath.Join(tree, root), func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			relPath, relErr := filepath.Rel(tree, path)
			if relErr != nil {
				return relErr
			}
			rel := filepath.ToSlash(relPath)

			// test/draft/ holds functional tests under development and is
			// invisible to every repo-wide gate, this one included. It is
			// pruned at the directory so nothing inside is read.
			if entry.IsDir() {
				if rel == draftTestDir {
					return filepath.SkipDir
				}
				return nil
			}

			emitters, pyScoped, skip := EmittersFor(rel)
			if skip {
				return nil
			}
			src, readErr := os.ReadFile(path) //nolint:gosec // repository path
			if readErr != nil {
				return readErr
			}
			if pyScoped && !usesZeAPI(string(src)) {
				return nil
			}

			found, scanned, passthroughs := ScanFile(surface, rel, string(src), emitters)
			report.Findings = append(report.Findings, found...)
			report.EmittersChecked += scanned
			report.PassThrough += passthroughs
			return nil
		})
		if err != nil {
			return Report{}, err
		}
	}

	if report.EmittersChecked < floor {
		return Report{}, fmt.Errorf("the walk found %d emitters under %s, below the floor of %d: this tree was not read",
			report.EmittersChecked, tree, floor)
	}

	sort.Slice(report.Findings, func(i, j int) bool {
		if report.Findings[i].File != report.Findings[j].File {
			return report.Findings[i].File < report.Findings[j].File
		}
		return report.Findings[i].Line < report.Findings[j].Line
	})
	return report, nil
}

// ScanFile answers every non-resolving emitter in one file, the number of
// emitters it read, and the number that only forward a command decided
// elsewhere.
func ScanFile(surface Surface, path, src string, emitters []*regexp.Regexp) (findings []Finding, scanned, passthroughs int) {
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
		for _, pattern := range emitters {
			for _, match := range pattern.FindAllStringSubmatch(line, -1) {
				emitter, arg := match[1], match[2]
				// The bare `send(...)` name is not reserved: .ci scripts define
				// local helpers with the same name and a different signature
				// (`send(master, command, transcript)`). Only treat it as a
				// daemon emitter when its first argument opens a string.
				if emitter == "send" && !startsString(arg) {
					continue
				}
				scanned++

				finding, kind := judge(surface, path, i+1, emitter, arg)
				switch kind {
				case verdictPassThrough:
					passthroughs++
				case verdictFinding:
					findings = append(findings, finding)
				case verdictResolved:
				}
			}
		}
	}
	return findings, scanned, passthroughs
}

// verdict is what one emitter turned out to be.
type verdict int

const (
	verdictResolved verdict = iota
	verdictPassThrough
	verdictFinding
)

// judge decides one emitter: it resolves, it forwards a command decided
// elsewhere, or it is a finding.
func judge(surface Surface, path string, line int, emitter, arg string) (Finding, verdict) {
	cmd, isLiteral := literal(arg)
	if isLiteral {
		if skipCommand(cmd) || surface.Resolves(cmd) || surface.sendRewriteResolves(emitter, cmd) {
			return Finding{}, verdictResolved
		}
		return Finding{
			File: path, Line: line, Kind: KindDead,
			Emitter: emitter, Command: cmd,
			Detail: "resolves to no registered command; the dispatcher answers ErrUnknownCommand",
		}, verdictFinding
	}

	if passThrough.MatchString(strings.TrimSpace(arg)) {
		return Finding{}, verdictPassThrough
	}

	// Computed command: check the static prefix, which is the part a
	// command-tree migration invalidates.
	prefix := leadingLiteral(arg)
	if prefix != "" && (skipCommand(prefix) || surface.prefixKnown(prefix) || surface.sendRewriteResolves(emitter, prefix)) {
		return Finding{}, verdictResolved
	}

	kind, detail := KindUnverifiable, "no static prefix to check; make the command path literal or mark the line `"
	if prefix != "" {
		kind, detail = KindDead, "static command prefix matches no registered command; the dispatcher answers ErrUnknownCommand. Mark the line `"
	}
	var tb textbuf.Buffer
	return Finding{
		File: path, Line: line, Kind: kind,
		Emitter: emitter, Command: strings.TrimSpace(arg),
		Detail: tb.Str(detail).Str(dynamicMarker).Str(" -- <reason>` only if it is genuinely uncheckable").String(),
	}, verdictFinding
}

// EmittersFor decides how a file is scanned: which emitter shapes apply,
// whether the ze_api scoping rule applies (Python only), and whether the file
// is scanned at all.
//
// UNIT TESTS OF THE TOOLING ARE NOT EMITTERS. `_test.go` was excluded from the
// first version for a reason that applies verbatim to `_test.py`, and leaving
// the Python half in was an oversight, not a policy: both name a test that runs
// IN-PROCESS against a fake, never against a daemon. test/scripts/ze_api_test.py
// replaces `api._call_engine` with a function that raises, then dispatches
// "show bgp nonsense" precisely BECAUSE the string must not resolve -- that is
// the unknown-command path under test. Neither string ever reaches a
// dispatcher, so neither can rot the way this gate exists to catch. The
// daemon-driving Python surface -- .ci observer blocks, test/perf/run.py,
// test/scripts/ze_api.py itself -- carries no `_test.py` suffix and stays fully
// scanned.
//
// The rule is deliberately the file-name convention rather than a per-file
// allowlist, and the selftest asserts both halves of this decision, so widening
// the exemption cannot happen silently.
func EmittersFor(path string) (emitters []*regexp.Regexp, pyScoped, skip bool) {
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

// literal extracts a string literal from an argument expression. The bool is
// false when the value is not statically knowable at this call site.
func literal(arg string) (string, bool) {
	match := pyLiteral.FindStringSubmatch(strings.TrimSpace(arg))
	if match == nil {
		return "", false
	}
	if match[1] != "" {
		return match[1], true
	}
	return match[2], true
}

// leadingLiteral answers the static text at the START of a computed command
// argument: everything before the first interpolation, concatenation or call.
// `f'request log level {sub} debug'` yields "request log level"; a textbuf
// chain `tb.Str("show bgp peer ").Str(addr)` yields "show bgp peer".
//
// The static prefix is the part a command-tree migration breaks. Checking it
// turns the large class of legitimately-interpolated commands from unverifiable
// noise (which would get the gate routed around) into a real assertion on the
// only part that can be checked, while a computed command with NO static prefix
// stays unverifiable and fails.
func leadingLiteral(arg string) string {
	match := firstLiteral.FindStringSubmatch(arg)
	if match == nil {
		return ""
	}
	lit := match[1]
	if lit == "" {
		lit = match[2]
	}
	// An f-string's interpolation ends the static run.
	if index := strings.Index(lit, "{"); index >= 0 {
		lit = lit[:index]
	}
	return strings.TrimSpace(lit)
}

// skipCommand reports strings that are not dispatcher command paths at all: the
// plugin-session wire directives and the route-injection text bridge, both of
// which are category-exempt from the grammar gate for the same reason.
func skipCommand(cmd string) bool {
	if cmd == "" {
		return true
	}
	for _, prefix := range []string{"plugin ", "subscribe ", "announce ", "withdraw "} {
		if strings.HasPrefix(cmd, prefix) {
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
	trimmed := strings.TrimSpace(arg)
	trimmed = strings.TrimPrefix(trimmed, "f")
	return strings.HasPrefix(trimmed, "'") || strings.HasPrefix(trimmed, "\"")
}

// usesZeAPI reports whether a Python or .ci source talks to the daemon through
// ze_api at all. The bare `send(...)` emitter is only meaningful there: other
// scripts define their own unrelated send() (qemu-run.py sends console input,
// post_weekly.py sends Discord chunks) and matching those was noise, not
// signal.
func usesZeAPI(src string) bool {
	return strings.Contains(src, "ze_api") || strings.Contains(src, "from ze_api import")
}
