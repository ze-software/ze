// Design: ai/rules/plugins.md -- process-boundary guard
//
// plugin_process_boundary enforces an invariant discovered during the
// AS112/cos advisory-doctor-check review:
// a plugin calling another in-process package's plain exported function
// directly -- bypassing DirectBridge/DispatchCommand -- for a SAME-PROCESS-
// EFFECT operation (one that only reaches the engine's real, shared package
// state when the plugin runs in the same OS process as the engine) silently
// no-ops, or never fires, when that plugin is configured to run as an
// external subprocess (plugin { external <name> { ... } }). Five confirmed
// instances so far, all fixed: as112 (iface.RegisterOwnedAddresses/
// UnregisterOwnedAddresses -- refuses to start external), cos
// (iface.GetBackend -- warns), traffic-usage and flow-export
// (iface.SubscribeCollectNotify -- both refuse), ddos-detect
// (iface.SubscribeCollectNotify + trafficstat.EnsureGlobal/Global -- warns).
//
// It scans every package under the generator's plugin search roots
// (pluginDirs + nestedPluginDomains in scripts/codegen/plugin_imports.go,
// derived at runtime -- see loadScanRootsFrom) for calls to the
// dangerousCalls list below. Per file, the local identifier for each watched import path is
// resolved from that file's own import declarations (go/parser, imports
// only) rather than assumed to be the package's default name, so a renamed
// import (e.g. `ifcomp "internal/component/iface"`) is still caught. A match
// is a finding UNLESS the SAME PACKAGE DIRECTORY also contains a call to
// .IsInternal( or a locally-defined warnIfExternal( helper somewhere (not
// necessarily the same file or function) -- this is a presence heuristic,
// not proof the guard actually covers the call at runtime, the same rigor
// level as the existing no-direct-resolution guard
// (scripts/checks/iface_resolution.go). New dangerousCalls entries are added
// as new instances of the class are discovered and fixed; new allowlist
// entries are added with a stated reason for a legitimate exemption (e.g.
// the owning package itself calling its own function).
//
// Usage:     go run scripts/checks/plugin_process_boundary.go [--json|--selftest]
// Called by: make ze-plugin-boundary-check (wired into ze-precommit-verify via
//            scripts/status/verify_run.go's stagesForMode) and
//            scripts/checks/plugin_process_boundary_test.go
//
//go:build ignore

package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
)

// allowlist maps a path prefix (a file, or a directory ending in "/",
// relative to the scan root with forward slashes) to the reason a package is
// exempt from the guard requirement entirely. This is for the OWNING
// packages of the dangerous functions themselves -- their own callers are
// not a cross-process boundary. A scanned file whose path has any of these
// prefixes never contributes a finding or a guard signal.
var allowlist = map[string]string{
	"internal/component/iface/":       "the owning package -- its own callers of RegisterOwnedAddresses/GetBackend/SubscribeCollectNotify etc. are not a cross-process boundary.",
	"internal/component/trafficstat/": "the owning package -- its own callers of EnsureGlobal/Global are not a cross-process boundary.",
}

// dangerousCall names a known same-process-effect function by owning import
// path + exported symbol: a plugin package calling it needs an
// IsInternal()/warnIfExternal() guard somewhere in the same package
// directory. Resolved per-file against that file's ACTUAL import alias (see
// resolveImportAliases), not a hardcoded package name -- a renamed import
// does not bypass detection.
type dangerousCall struct {
	importPath string
	symbol     string
}

var dangerousCalls = []dangerousCall{
	{"github.com/ze-software/ze/internal/component/iface", "RegisterOwnedAddresses"},
	{"github.com/ze-software/ze/internal/component/iface", "UnregisterOwnedAddresses"},
	{"github.com/ze-software/ze/internal/component/iface", "GetBackend"},
	{"github.com/ze-software/ze/internal/component/iface", "SubscribeCollectNotify"},
	{"github.com/ze-software/ze/internal/component/iface", "UnsubscribeCollectNotify"},
	{"github.com/ze-software/ze/internal/component/trafficstat", "EnsureGlobal"},
	{"github.com/ze-software/ze/internal/component/trafficstat", "Global"},
}

// watchedImportPaths is the deduplicated set of import paths dangerousCalls
// cares about, resolved once per file.
func watchedImportPaths() []string {
	seen := map[string]bool{}
	var out []string
	for _, dc := range dangerousCalls {
		if !seen[dc.importPath] {
			seen[dc.importPath] = true
			out = append(out, dc.importPath)
		}
	}
	return out
}

// guardPatterns: a package directory containing a match for any of these
// anywhere is considered guarded. Presence-based, not flow-proven -- same
// limitation as the dangerous-call side.
var guardPatterns = []*regexp.Regexp{
	regexp.MustCompile(`\.IsInternal\(\)`),
	regexp.MustCompile(`\bwarnIfExternal\(`),
}

// generatorSource is the plugin-namespace source of truth: the composition-
// root generator whose pluginDirs + nestedPluginDomains lists say where plugin
// packages (subject to the internal/external process-placement choice) live.
// The scan roots are DERIVED from it at runtime -- never a second hardcoded
// list -- so a namespace added to the generator is automatically scanned
// (spec-layout-0-umbrella child 2; same single-discovery-source rule as
// dep_audit.py's parse_plugin_dirs, tiers blocker B-1).
const generatorSource = "scripts/codegen/plugin_imports.go"

// parseStringList extracts the quoted elements of `var <name> = []string{...}`
// from the generator source text. Mirrors dep_audit.py's _parse_string_list.
func parseStringList(text, name string) []string {
	re := regexp.MustCompile(`var ` + regexp.QuoteMeta(name) + ` = \[\]string\{([^}]*)\}`)
	m := re.FindStringSubmatch(text)
	if m == nil {
		return nil
	}
	var out []string
	for _, q := range regexp.MustCompile(`"([^"]+)"`).FindAllStringSubmatch(m[1], -1) {
		out = append(out, q[1])
	}
	return out
}

// loadScanRootsFrom derives the scan roots from the generator file at path:
// every pluginDirs entry plus internal/component/<domain>/plugins for every
// nestedPluginDomains entry, deduplicated. It fails loud when pluginDirs
// cannot be parsed (a generator refactor must break the gate visibly, never
// silently scan nothing).
func loadScanRootsFrom(genPath string) ([]string, error) {
	raw, err := os.ReadFile(genPath)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", genPath, err)
	}
	text := string(raw)
	pluginDirs := parseStringList(text, "pluginDirs")
	if len(pluginDirs) == 0 {
		return nil, fmt.Errorf("%s: could not parse pluginDirs -- gate cannot run safely", genPath)
	}
	seen := map[string]bool{}
	var roots []string
	add := func(rel string) {
		if rel == "" || seen[rel] {
			return
		}
		seen[rel] = true
		roots = append(roots, rel)
	}
	for _, rel := range pluginDirs {
		add(rel)
	}
	for _, domain := range parseStringList(text, "nestedPluginDomains") {
		add(path.Join("internal/component", domain, "plugins"))
	}
	return roots, nil
}

type finding struct {
	File string `json:"file"`
	Line int    `json:"line"`
	Code string `json:"code"`
}

// stripComment returns the code portion of a Go source line, dropping a
// leading or trailing "//" comment so a pattern mentioned only in prose does
// not register as a call. Mirrors iface_resolution.go's stripComment.
func stripComment(line string) string {
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

// allowed reports whether rel is exempt (the owning package of a dangerous
// function, per the allowlist doc comment above).
func allowed(rel string) bool {
	for prefix := range allowlist {
		if strings.HasPrefix(rel, prefix) {
			return true
		}
	}
	return false
}

// resolveImportAliases parses only the import declarations of the Go source
// at path (parser.ImportsOnly -- fast, and immune to the line-scanner's
// blind spot for renamed imports) and returns the local identifier bound to
// each watched import path actually imported by this file. A path absent
// from the result means this file does not import it at all. A blank (`_`)
// or dot (`.`) import is skipped -- neither can be the target of a
// pkg.Symbol call the line scanner would recognize.
func resolveImportAliases(path string, watch []string) (map[string]string, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
	if err != nil {
		return nil, err
	}
	result := map[string]string{}
	for _, imp := range f.Imports {
		importPath, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			continue
		}
		if !slices.Contains(watch, importPath) {
			continue
		}
		switch {
		case imp.Name == nil:
			parts := strings.Split(importPath, "/")
			result[importPath] = parts[len(parts)-1]
		case imp.Name.Name == "_" || imp.Name.Name == ".":
			// blank/dot import: no qualified-call form exists to detect.
		default:
			result[importPath] = imp.Name.Name
		}
	}
	return result, nil
}

// packageScan holds the accumulated dangerous-call findings and guard
// presence for one plugin package directory.
type packageScan struct {
	findings []finding
	guarded  bool
}

// scan walks every root in roots for plugin-package Go source (excluding
// _test.go and allowlisted paths), and returns every same-process-effect
// call site in a package directory that has no IsInternal()/warnIfExternal()
// guard anywhere in that same directory. roots are relative to the current
// working directory (the caller sets it, matching iface_resolution.go).
func scan(roots []string) ([]finding, error) {
	watch := watchedImportPaths()
	packages := map[string]*packageScan{}

	for _, root := range roots {
		if fi, statErr := os.Stat(root); statErr != nil || !fi.IsDir() {
			continue
		}
		walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			rel := filepath.ToSlash(path)
			if allowed(rel) {
				return nil
			}
			pkgDir := filepath.ToSlash(filepath.Dir(rel))

			aliases, aerr := resolveImportAliases(path, watch)
			if aerr != nil {
				return fmt.Errorf("parse imports in %s: %w", path, aerr)
			}

			// Build this file's own dangerous-call patterns from the aliases
			// it actually imports -- a file importing neither iface nor
			// trafficstat has an empty pattern set and is skipped entirely.
			var filePatterns []*regexp.Regexp
			if len(aliases) > 0 {
				for _, dc := range dangerousCalls {
					local, ok := aliases[dc.importPath]
					if !ok {
						continue
					}
					filePatterns = append(filePatterns, regexp.MustCompile(`\b`+regexp.QuoteMeta(local)+`\.`+regexp.QuoteMeta(dc.symbol)+`\b`))
				}
			}

			f, oerr := os.Open(path)
			if oerr != nil {
				return oerr
			}
			defer f.Close()
			sc := bufio.NewScanner(f)
			sc.Buffer(make([]byte, 1024*1024), 1024*1024)
			ln := 0
			for sc.Scan() {
				ln++
				code := stripComment(sc.Text())
				if code == "" {
					continue
				}
				for _, re := range guardPatterns {
					if re.MatchString(code) {
						ps := packages[pkgDir]
						if ps == nil {
							ps = &packageScan{}
							packages[pkgDir] = ps
						}
						ps.guarded = true
					}
				}
				for _, re := range filePatterns {
					if re.MatchString(code) {
						ps := packages[pkgDir]
						if ps == nil {
							ps = &packageScan{}
							packages[pkgDir] = ps
						}
						ps.findings = append(ps.findings, finding{File: rel, Line: ln, Code: strings.TrimSpace(sc.Text())})
						break
					}
				}
			}
			return sc.Err()
		})
		if walkErr != nil {
			return nil, fmt.Errorf("scan error in %s: %w", root, walkErr)
		}
	}

	var unguarded []finding
	pkgDirs := make([]string, 0, len(packages))
	for pkgDir := range packages {
		pkgDirs = append(pkgDirs, pkgDir)
	}
	sort.Strings(pkgDirs)
	for _, pkgDir := range pkgDirs {
		ps := packages[pkgDir]
		if ps.guarded || len(ps.findings) == 0 {
			continue
		}
		unguarded = append(unguarded, ps.findings...)
	}

	sort.Slice(unguarded, func(i, j int) bool {
		if unguarded[i].File != unguarded[j].File {
			return unguarded[i].File < unguarded[j].File
		}
		return unguarded[i].Line < unguarded[j].Line
	})
	return unguarded, nil
}

func main() {
	jsonOut := false
	selftest := false
	printRoots := false
	for _, a := range os.Args[1:] {
		switch a {
		case "--json":
			jsonOut = true
		case "--selftest":
			selftest = true
		case "--print-roots":
			printRoots = true
		}
	}

	if selftest {
		os.Exit(runSelftest())
	}

	scanRoots, err := loadScanRootsFrom(generatorSource)
	if err != nil {
		fmt.Fprintf(os.Stderr, "plugin-process-boundary: %v\n", err)
		os.Exit(2)
	}

	if printRoots {
		for _, r := range scanRoots {
			fmt.Println(r)
		}
		return
	}

	unguarded, err := scan(scanRoots)
	if err != nil {
		fmt.Fprintf(os.Stderr, "plugin-process-boundary: %v\n", err)
		os.Exit(2)
	}

	if jsonOut {
		_ = json.NewEncoder(os.Stdout).Encode(unguarded)
		if len(unguarded) > 0 {
			os.Exit(1)
		}
		return
	}

	if len(unguarded) > 0 {
		fmt.Fprintf(os.Stderr, "plugin-process-boundary: %d unguarded same-process-effect call site(s):\n", len(unguarded))
		for _, f := range unguarded {
			fmt.Fprintf(os.Stderr, "  %s:%d: %s\n", f.File, f.Line, f.Code)
		}
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "This calls a same-process-effect function directly (bypassing DirectBridge/")
		fmt.Fprintln(os.Stderr, "DispatchCommand), which silently no-ops or never fires when the plugin runs as")
		fmt.Fprintln(os.Stderr, "an external subprocess (see as112/cos/traffic-usage/flow-export/ddos-detect for")
		fmt.Fprintln(os.Stderr, "precedent). Add a p.IsInternal() check (refuse to start if the call is the")
		fmt.Fprintln(os.Stderr, "plugin's core purpose) or a warnIfExternal(p.IsInternal()) helper (if the plugin")
		fmt.Fprintln(os.Stderr, "still provides real value external) somewhere in this plugin's own package.")
		os.Exit(1)
	}

	fmt.Println("plugin-process-boundary: OK")
}

// runSelftest exercises scan() against isolated temp-dir fixtures (no repo
// mutation -- mirrors dep_audit.py's --selftest convention). It proves the
// import-alias resolution actually works, which the real-tree smoke test
// cannot: the real tree currently has zero dangerous calls made through a
// renamed import, so a regression there would otherwise go undetected.
func runSelftest() int {
	dir, err := os.MkdirTemp("", "plugin-process-boundary-selftest-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "plugin-process-boundary selftest: %v\n", err)
		return 2
	}
	defer os.RemoveAll(dir)

	write := func(rel, content string) {
		full := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			panic(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			panic(err)
		}
	}

	const ifacePkg = "github.com/ze-software/ze/internal/component/iface"

	// Unguarded, default import name -- must be flagged.
	write("internal/plugins/plain/register.go", `package plain

import "`+ifacePkg+`"

func run() {
	iface.GetBackend()
}
`)

	// Unguarded, RENAMED import alias -- must ALSO be flagged. This is the
	// case the plain-package-name-prefix version of this checker missed.
	write("internal/plugins/aliased/register.go", `package aliased

import ifcomp "`+ifacePkg+`"

func run() {
	ifcomp.GetBackend()
}
`)

	// Guarded, renamed import alias, guard in a DIFFERENT file in the same
	// package -- must NOT be flagged (package-directory-wide presence check).
	write("internal/plugins/guardedaliased/register.go", `package guardedaliased

import ifcomp "`+ifacePkg+`"

func run() {
	ifcomp.GetBackend()
}
`)
	write("internal/plugins/guardedaliased/guard.go", `package guardedaliased

func checkInternal() {
	p.IsInternal()
}
`)

	// Blank import of the watched package -- no qualified-call form exists,
	// must not panic and must not be flagged.
	write("internal/plugins/blankimport/register.go", `package blankimport

import _ "`+ifacePkg+`"
`)

	unguarded, err := scan([]string{filepath.Join(dir, "internal", "plugins")})
	if err != nil {
		fmt.Fprintf(os.Stderr, "plugin-process-boundary selftest: scan: %v\n", err)
		return 2
	}

	byPkg := map[string]bool{}
	for _, f := range unguarded {
		byPkg[filepath.ToSlash(filepath.Dir(f.File))] = true
	}

	var failed []string
	check := func(cond bool, msg string) {
		if cond {
			failed = append(failed, msg)
		}
	}

	plainPkg := filepath.ToSlash(filepath.Join(dir, "internal/plugins/plain"))
	check(!byPkg[plainPkg], "default-name dangerous call not flagged")
	aliasedPkg := filepath.ToSlash(filepath.Join(dir, "internal/plugins/aliased"))
	check(!byPkg[aliasedPkg], "RENAMED-IMPORT dangerous call not flagged (alias resolution regressed)")
	guardedPkg := filepath.ToSlash(filepath.Join(dir, "internal/plugins/guardedaliased"))
	check(byPkg[guardedPkg], "guarded package (guard in a sibling file) wrongly flagged")
	blankPkg := filepath.ToSlash(filepath.Join(dir, "internal/plugins/blankimport"))
	check(byPkg[blankPkg], "blank import wrongly flagged")

	// --- scan-root derivation from the generator (spec-layout-2) ---
	// A fixture generator file must yield pluginDirs + the expanded
	// nestedPluginDomains, deduplicated; an unparseable pluginDirs must be a
	// loud error, never an empty (scan-nothing) result.
	write("scripts/codegen/plugin_imports.go", `package main

var pluginDirs = []string{
	"internal/plugins",
	"internal/component/firewall/plugins",
	"internal/plugins",
}

var nestedPluginDomains = []string{
	"l2tp",
}
`)
	roots, rerr := loadScanRootsFrom(filepath.Join(dir, "scripts/codegen/plugin_imports.go"))
	check(rerr != nil, "fixture generator parse errored")
	wantRoots := []string{
		"internal/plugins",
		"internal/component/firewall/plugins",
		"internal/component/l2tp/plugins",
	}
	check(!slices.Equal(roots, wantRoots), "derived roots wrong (dedup + nested expansion)")

	write("scripts/codegen/broken.go", "package main\n// no pluginDirs here\n")
	_, berr := loadScanRootsFrom(filepath.Join(dir, "scripts/codegen/broken.go"))
	check(berr == nil, "unparseable pluginDirs did not fail loud")

	if len(failed) > 0 {
		fmt.Fprintln(os.Stderr, "plugin-process-boundary selftest FAILED:")
		for _, m := range failed {
			fmt.Fprintf(os.Stderr, "  %s\n", m)
		}
		fmt.Fprintf(os.Stderr, "  findings: %v\n", unguarded)
		return 1
	}

	fmt.Println("plugin-process-boundary selftest OK")
	return 0
}
