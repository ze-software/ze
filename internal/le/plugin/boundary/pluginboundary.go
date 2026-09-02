// Design: docs/architecture/core-design.md -- the plugin process-boundary guard
//
// Package pluginboundary enforces an invariant discovered during the AS112/cos
// advisory-doctor-check review: a plugin calling another in-process package's
// plain exported function directly -- bypassing DirectBridge/DispatchCommand --
// for a SAME-PROCESS-EFFECT operation (one that only reaches the engine's real,
// shared package state when the plugin runs in the same OS process as the
// engine) silently no-ops, or never fires, when that plugin is configured to
// run as an external subprocess (plugin { external <name> { ... } }). Five
// confirmed instances so far, all fixed: as112
// (iface.RegisterOwnedAddresses/UnregisterOwnedAddresses -- refuses to start
// external), cos (iface.GetBackend -- warns), traffic-usage and flow-export
// (iface.SubscribeCollectNotify -- both refuse), ddos-detect
// (iface.SubscribeCollectNotify + trafficstat.EnsureGlobal/Global -- warns).
//
// It scans every package under the composition-root generator's plugin search
// roots for calls to the dangerousCalls list. Per file, the local identifier
// for each watched import path is resolved from that file's own import
// declarations (go/parser, imports only) rather than assumed to be the
// package's default name, so a renamed import (e.g. `ifcomp
// "internal/component/iface"`) is still caught. A match is a finding UNLESS the
// SAME PACKAGE DIRECTORY also contains a call to .IsInternal( or a
// locally-defined warnIfExternal( helper somewhere (not necessarily the same
// file or function) -- this is a presence heuristic, not proof the guard
// actually covers the call at runtime, the same rigor level as
// internal/le/ifaceresolution.
//
// The scan roots are DERIVED from the generator, never a second hardcoded list,
// so a namespace added to the generator is automatically scanned. The
// derivation is a CALL, internal/le/pluginimports.PluginSearchRoots. The script
// this replaces regexed the generator's SOURCE TEXT for its `pluginDirs` and
// `nestedPluginDomains` declarations, which bought the same single source and
// cost a parse that a generator refactor could break -- and which would break
// outright the day that script is deleted.

package pluginboundary

import (
	"bufio"
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	pluginimports "github.com/ze-software/ze/internal/le/plugin/imports"
	"github.com/ze-software/ze/internal/le/population"
)

// allowlist maps a path prefix (a file, or a directory ending in "/", relative
// to the tree with forward slashes) to the reason a package is exempt from the
// guard requirement entirely. This is for the OWNING packages of the dangerous
// functions themselves -- their own callers are not a cross-process boundary. A
// scanned file whose path has any of these prefixes never contributes a finding
// or a guard signal.
//
// It is EMPTY, and the accounting is why. An entry is owed only where it
// SUPPRESSES something, and neither original entry did. trafficstat is an owning
// package that is not a plugin search root, so the walk never reached it. iface
// is a root, but no file under it calls a watched symbol or carries a guard, so
// the exemption erased nothing there either.
//
// The day a subpackage of an owning package makes one of these calls, the gate
// reports it. The entry comes back then, with a reason somebody wrote then.
var allowlist = map[string]string{}

// The two owning packages whose exported functions only reach real, shared
// engine state in the same OS process. They are constants because the watch
// list and the fixture must name the same package.
//
// Neither is in the allowlist. An owning package earns an entry only when the
// walk reaches it AND a file under it makes one of these calls. No file does
// today.
const (
	ifacePackage       = "github.com/ze-software/ze/internal/component/iface"
	trafficstatPackage = "github.com/ze-software/ze/internal/component/trafficstat"
)

// dangerousCall names a known same-process-effect function by owning import
// path plus exported symbol: a plugin package calling it needs an
// IsInternal()/warnIfExternal() guard somewhere in the same package directory.
// It is resolved per-file against that file's ACTUAL import alias, not a
// hardcoded package name, so a renamed import does not bypass detection.
type dangerousCall struct {
	importPath string
	symbol     string
}

// dangerousCalls is the whole watch list. An entry is added as a new instance
// of the class is discovered and fixed.
var dangerousCalls = []dangerousCall{
	{ifacePackage, "RegisterOwnedAddresses"},
	{ifacePackage, "UnregisterOwnedAddresses"},
	{ifacePackage, "GetBackend"},
	{ifacePackage, "SubscribeCollectNotify"},
	{ifacePackage, "UnsubscribeCollectNotify"},
	{trafficstatPackage, "EnsureGlobal"},
	{trafficstatPackage, "Global"},
}

// guardPatterns: a package directory containing a match for any of these
// anywhere is considered guarded. Presence-based, not flow-proven -- the same
// limitation as the dangerous-call side.
var guardPatterns = []*regexp.Regexp{
	regexp.MustCompile(`\.IsInternal\(\)`),
	regexp.MustCompile(`\bwarnIfExternal\(`),
}

// scanFloor is the least non-test Go files the walk must read before the gate
// believes it saw the plugin tree. This checkout carried 1629 on 2026-08-26, so
// the floor fires on a tree that was never read rather than on one that shrank.
const scanFloor = 400

// lineBudget bounds one source line the scanner will read. A generated table
// can be long, and a line past this is a file the gate could not read rather
// than a call site it may pass over.
const lineBudget = 1024 * 1024

// Roots answers every plugin search root, relative to the tree, in the order
// the generator declares them.
func Roots() []string { return pluginimports.PluginSearchRoots() }

// Check walks tree's plugin search roots and answers every same-process-effect
// call site in a package directory that has no IsInternal()/warnIfExternal()
// guard anywhere in that same directory.
//
// floor is a parameter rather than a constant because a fixture tree holds a
// handful of files: le passes scanFloor and a test passes 0.
func Check(tree string, floor int) (Findings, error) {
	findings, _, err := scan(tree, treeRoots(tree), floor)
	return findings, err
}

// CheckCheckout is Check plus the accounting over the allowlist itself, for a
// caller that is judging the real repository.
//
// The two are separate for the reason floor is a parameter. Over a fixture, an
// entry that suppresses nothing means the tree does not hold that code. Over the
// checkout it means an exemption that has stopped doing anything. Only the
// caller knows which tree it handed over.
func CheckCheckout(tree string, floor int) (Findings, error) {
	findings, matched, err := scan(tree, treeRoots(tree), floor)
	if err != nil {
		return nil, err
	}
	coverage, err := population.Exemptions("plugin-boundary allowlist", allowlist, matched)
	if err != nil {
		return nil, err
	}
	if len(coverage.Unexcused) != 0 {
		return nil, fmt.Errorf("%w: %s -- delete the entry, or correct the path it was meant to name",
			ErrDeadAllowlistEntry, strings.Join(coverage.Unexcused, ", "))
	}
	return findings, nil
}

// ErrDeadAllowlistEntry names an allowlist entry that erased nothing in the tree
// the walk just read.
var ErrDeadAllowlistEntry = errors.New("a plugin-boundary allowlist entry suppresses nothing")

// suppressed reports whether a probe scan of one exempt file produced anything
// the exemption then erased. That is a dangerous call, or the guard signal that
// would have covered its package.
//
// Both count. The allowlist doc states an exempt file contributes neither, so an
// entry doing either job is still doing work.
func suppressed(probe map[string]*packageScan) bool {
	for _, found := range probe {
		if len(found.findings) != 0 || found.guarded {
			return true
		}
	}
	return false
}

func treeRoots(tree string) []string {
	roots := make([]string, 0, len(Roots()))
	for _, rel := range Roots() {
		roots = append(roots, filepath.Join(tree, filepath.FromSlash(rel)))
	}
	return roots
}

// packageScan holds the accumulated dangerous-call findings and guard presence
// for one plugin package directory.
type packageScan struct {
	findings []Finding
	guarded  bool
}

// scan walks every root for plugin-package Go source (excluding _test.go and
// allowlisted paths) and answers the unguarded call sites.
//
// A root the generator declares but the tree does not carry is passed over,
// which is the generator's own contract for the same list: internal/component/
// ike/plugins is declared for a domain whose plugins have not landed. Every
// OTHER stat failure stops the run, and the file count is floored, because the
// script this replaced answered `if fi, statErr := os.Stat(root); statErr != nil
// || !fi.IsDir() { continue }` -- one skip for both facts, so a run whose roots
// ALL resolved to nothing printed OK having read no file.
func scan(tree string, roots []string, floor int) (Findings, map[string]bool, error) {
	watch := watchedImportPaths()
	packages := map[string]*packageScan{}
	read := 0
	matched := make(map[string]bool, len(allowlist))

	for _, root := range roots {
		info, statErr := os.Stat(root)
		switch {
		case errors.Is(statErr, fs.ErrNotExist):
			continue
		case statErr != nil:
			return nil, nil, fmt.Errorf("stat the scan root %s: %w", root, statErr)
		case !info.IsDir():
			return nil, nil, fmt.Errorf("the scan root %s is not a directory", root)
		}

		walkErr := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			relPath, relErr := filepath.Rel(tree, path)
			if relErr != nil {
				return relErr
			}
			rel := filepath.ToSlash(relPath)
			read++
			if prefix, exempt := AllowlistedBy(rel); exempt {
				// The file is scanned into a throwaway map, so the exemption
				// still erases what it finds. The entry is credited only where
				// it erased something. Skipping outright would make "this entry
				// did work" mean "a file exists under it", which stays true for
				// an entry whose file stopped making the call.
				probe := map[string]*packageScan{}
				if err := scanFile(path, rel, watch, probe); err != nil {
					return err
				}
				if suppressed(probe) {
					matched[prefix] = true
				}
				return nil
			}
			return scanFile(path, rel, watch, packages)
		})
		if walkErr != nil {
			return nil, nil, fmt.Errorf("scan error in %s: %w", root, walkErr)
		}
	}

	if read < floor {
		return nil, nil, fmt.Errorf("the walk read %d non-test Go files under %s, below the floor of %d: this tree was not read", read, tree, floor)
	}

	var unguarded Findings
	pkgDirs := make([]string, 0, len(packages))
	for pkgDir := range packages {
		pkgDirs = append(pkgDirs, pkgDir)
	}
	sort.Strings(pkgDirs)
	for _, pkgDir := range pkgDirs {
		found := packages[pkgDir]
		if found.guarded || len(found.findings) == 0 {
			continue
		}
		unguarded = append(unguarded, found.findings...)
	}

	sort.Slice(unguarded, func(i, j int) bool {
		if unguarded[i].File != unguarded[j].File {
			return unguarded[i].File < unguarded[j].File
		}
		return unguarded[i].Line < unguarded[j].Line
	})
	return unguarded, matched, nil
}

// scanFile records one file's dangerous calls and guard presence against its
// package directory's accumulated scan.
func scanFile(path, rel string, watch []string, packages map[string]*packageScan) error {
	pkgDir := filepath.ToSlash(filepath.Dir(rel))

	aliases, err := resolveImportAliases(path, watch)
	if err != nil {
		return fmt.Errorf("parse imports in %s: %w", rel, err)
	}

	// Build this file's own dangerous-call patterns from the aliases it
	// actually imports -- a file importing neither iface nor trafficstat has an
	// empty pattern set and is skipped entirely.
	var filePatterns []*regexp.Regexp
	for _, call := range dangerousCalls {
		local, ok := aliases[call.importPath]
		if !ok {
			continue
		}
		filePatterns = append(filePatterns, regexp.MustCompile(
			`\b`+regexp.QuoteMeta(local)+`\.`+regexp.QuoteMeta(call.symbol)+`\b`))
	}

	file, err := os.Open(path) //nolint:gosec // repository path
	if err != nil {
		return err
	}
	defer file.Close() //nolint:errcheck // read-only scan

	entry := func() *packageScan {
		found := packages[pkgDir]
		if found == nil {
			found = &packageScan{}
			packages[pkgDir] = found
		}
		return found
	}

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, lineBudget), lineBudget)
	for line := 1; scanner.Scan(); line++ {
		code := StripComment(scanner.Text())
		if code == "" {
			continue
		}
		for _, pattern := range guardPatterns {
			if pattern.MatchString(code) {
				entry().guarded = true
			}
		}
		for _, pattern := range filePatterns {
			if pattern.MatchString(code) {
				found := entry()
				found.findings = append(found.findings, Finding{
					File: rel, Line: line, Code: strings.TrimSpace(scanner.Text()),
				})
				break
			}
		}
	}
	return scanner.Err()
}

// watchedImportPaths is the deduplicated set of import paths dangerousCalls
// cares about, resolved once per run.
func watchedImportPaths() []string {
	seen := map[string]bool{}
	var out []string
	for _, call := range dangerousCalls {
		if seen[call.importPath] {
			continue
		}
		seen[call.importPath] = true
		out = append(out, call.importPath)
	}
	return out
}

// resolveImportAliases parses only the import declarations of the Go source at
// path (parser.ImportsOnly -- fast, and immune to the line scanner's blind spot
// for renamed imports) and answers the local identifier bound to each watched
// import path this file actually imports. A path absent from the result means
// this file does not import it at all. A blank (`_`) or dot (`.`) import is
// skipped -- neither can be the target of a pkg.Symbol call the line scanner
// would recognize.
func resolveImportAliases(path string, watch []string) (map[string]string, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
	if err != nil {
		return nil, err
	}

	result := map[string]string{}
	for _, imported := range file.Imports {
		importPath, unquoteErr := strconv.Unquote(imported.Path.Value)
		if unquoteErr != nil {
			continue
		}
		if !slices.Contains(watch, importPath) {
			continue
		}
		switch {
		case imported.Name == nil:
			parts := strings.Split(importPath, "/")
			result[importPath] = parts[len(parts)-1]
		case imported.Name.Name == "_" || imported.Name.Name == ".":
			// blank/dot import: no qualified-call form exists to detect.
		default:
			result[importPath] = imported.Name.Name
		}
	}
	return result, nil
}

// Allowlisted reports whether rel is exempt -- the owning package of a
// dangerous function.
func Allowlisted(rel string) bool {
	_, exempt := AllowlistedBy(rel)
	return exempt
}

// AllowlistedBy answers the allowlist entry that exempts rel, and whether one
// does.
//
// It names WHICH entry because the entries are a population of their own. One
// that suppresses nothing is an exemption nobody rechecked. The walk is the
// only producer that can tell.
func AllowlistedBy(rel string) (string, bool) {
	for prefix := range allowlist {
		if strings.HasPrefix(rel, prefix) {
			return prefix, true
		}
	}
	return "", false
}

// StripComment answers the code portion of a Go source line, dropping a leading
// or trailing "//" comment so a pattern mentioned only in prose does not
// register as a call.
func StripComment(line string) string {
	if strings.HasPrefix(strings.TrimSpace(line), "//") {
		return ""
	}
	for i := 1; i+1 < len(line); i++ {
		if line[i] == '/' && line[i+1] == '/' && (line[i-1] == ' ' || line[i-1] == '\t') {
			return line[:i]
		}
	}
	return line
}
