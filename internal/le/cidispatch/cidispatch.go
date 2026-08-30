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
const dynamicMarker = "le-ci-dispatch: dynamic"

// scanRoots are the trees that hold native command emitters.
var scanRoots = []string{"internal", "cmd", "pkg"}

// emitterFloor is the least emitters the walk must find before the check
// believes it saw the tree. It is deliberately below the current population
// and still rejects an empty or unrelated checkout.
const emitterFloor = 10

// goEmitters match Go call sites that send a command string to the dispatcher.
var goEmitters = []*regexp.Regexp{
	regexp.MustCompile(`\b(DispatchCommand)\s*\(\s*[^,]+,\s*(.+?)\s*[,)]`),
	regexp.MustCompile(`\bsshclient\.(ExecCommand)\s*\(\s*[^,]+,\s*(.+?)\s*\)`),
	regexp.MustCompile(`\b(SendCommand)\s*\(\s*(.+?)\s*\)`),
	regexp.MustCompile(`\b(commandExecutor)\s*\(\s*(.+?)\s*\)`),
}

// quotedLiteral matches a whole-argument single-quoted or double-quoted
// literal.
var quotedLiteral = regexp.MustCompile(`^(?:'([^'\\]*)'|"([^"\\]*)")$`)

// passThrough matches an argument that is a bare variable or field: the command
// was decided elsewhere and this call only forwards it (`SendCommand(input)` in
// the CLI client forwards whatever the operator typed). Such a site emits no
// fixed command, so there is nothing for this check to resolve. It is counted
// and reported, never failed. An argument that BUILDS a command from literals
// (concatenation or a textbuf chain) is failed as unverifiable because that is
// the shape every defect this check exists for took.
var passThrough = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*)*$`)

// firstLiteral finds the first quoted run in a computed command argument.
var firstLiteral = regexp.MustCompile(`'([^'\\]*)'|"([^"\\]*)"`)

// Check walks tree's native emitters and answers every one that does not
// resolve.
//
// floor is a parameter because a fixture tree holds fewer emitters than the
// checkout.
// testdataDir is the directory name the Go tool treats as invisible. Every
// tree walk in this package skips it, so the name is declared once rather
// than repeated at each walk.
const testdataDir = "testdata"

func Check(tree string, floor int) (Report, error) {
	surface, err := newSurface(tree)
	if err != nil {
		return Report{}, err
	}
	return CheckWith(surface, tree, floor)
}

// CheckWith is Check against a surface the caller already built, which is what
// the selftest and package tests use.
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

			if entry.IsDir() {
				if entry.Name() == testdataDir {
					return filepath.SkipDir
				}
				return nil
			}

			// This package's selftest stores emitter source as data. It never
			// sends those commands to a daemon.
			if rel == "internal/le/cidispatch/selftest.go" {
				return nil
			}
			emitters, skip := emittersFor(rel)
			if skip {
				return nil
			}
			src, readErr := os.ReadFile(path) //nolint:gosec // repository path
			if readErr != nil {
				return readErr
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
		if skipCommand(cmd) || surface.Resolves(cmd) {
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
	if prefix != "" && (skipCommand(prefix) || surface.prefixKnown(prefix)) {
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

// emittersFor decides whether one source file contains native command
// emitters. Package tests are not runtime emitters and are skipped.
func emittersFor(path string) (emitters []*regexp.Regexp, skip bool) {
	if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
		return nil, true
	}
	return goEmitters, false
}

// literal extracts a string literal from an argument expression. The bool is
// false when the value is not statically knowable at this call site.
func literal(arg string) (string, bool) {
	match := quotedLiteral.FindStringSubmatch(strings.TrimSpace(arg))
	if match == nil {
		return "", false
	}
	if match[1] != "" {
		return match[1], true
	}
	return match[2], true
}

// leadingLiteral answers the static text at the start of a computed command
// argument. A textbuf chain such as
// `tb.Str("show bgp peer ").Str(addr)` yields "show bgp peer".
//
// The static prefix is the part a command-tree migration breaks. Checking it
// Checking the static prefix catches command-tree migrations while permitting
// computed suffixes. A computed command with no static prefix is unverifiable
// and fails.
func leadingLiteral(arg string) string {
	match := firstLiteral.FindStringSubmatch(arg)
	if match == nil {
		return ""
	}
	lit := match[1]
	if lit == "" {
		lit = match[2]
	}
	return strings.TrimSpace(lit)
}

// skipCommand reports strings that are not dispatcher command paths: the
// plugin-session wire directives and route-injection text bridge.
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

// isComment reports a Go comment. Prose in a comment is not an emitter.
func isComment(line string) bool {
	trimmed := strings.TrimSpace(line)
	return strings.HasPrefix(trimmed, "//")
}

// isDeclaration reports a function definition rather than a call. Without
// this, a SendCommand method's parameter list reads as an emitted command.
func isDeclaration(line string) bool {
	return strings.HasPrefix(strings.TrimSpace(line), "func ")
}
