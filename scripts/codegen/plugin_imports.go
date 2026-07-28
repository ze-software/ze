// Design: (none — build tool)
//
// plugin_imports generates internal/plugin/all/all.go from register.go discovery.
//
// It scans plugin directories for register.go files that import plugin/registry,
// and internal/**/yang/register.go for YANG schema packages, then generates
// the blank-import file that triggers init() registration.
//
// Usage: go run scripts/codegen/plugin_imports.go
// Called by: go generate ./internal/plugin/all/...
//
//go:build ignore

package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

func main() {
	checkOnly := flag.Bool("check", false, "verify generated all.go is current without writing it")
	selftest := flag.Bool("selftest", false, "run in-process assertions for the build-tag constraint logic and exit")
	flag.Parse()
	if *selftest {
		os.Exit(runSelftest())
	}
	if flag.NArg() != 0 {
		fatal(fmt.Errorf("unexpected arguments: %s", strings.Join(flag.Args(), " ")))
	}

	root, err := findModuleRoot()
	if err != nil {
		fatal(err)
	}

	featureTags, err = loadFeatureTags(root)
	if err != nil {
		fatal(err)
	}

	module, err := readModulePath(filepath.Join(root, "go.mod"))
	if err != nil {
		fatal(err)
	}

	plugins, err := discoverPlugins(root, module)
	if err != nil {
		fatal(err)
	}

	schemas, err := discoverSchemaPackages(filepath.Join(root, "internal"), module)
	if err != nil {
		fatal(err)
	}

	rpcs, err := discoverRPCPackages(root, module)
	if err != nil {
		fatal(err)
	}

	namespaces, err := discoverEventNamespaces(filepath.Join(root, "internal"), module, plugins)
	if err != nil {
		fatal(err)
	}

	allDir := filepath.Join(root, "internal", "component", "plugin", "all")
	output := filepath.Join(allDir, "all.go")

	// Partition build-tag-gated imports out of the universal all.go into
	// per-tag all_<tag>.go files (per-feature compile-out).
	byTag := map[string][]string{}
	plugins = filterTagged(plugins, byTag)
	schemas = filterTagged(schemas, byTag)
	rpcs = filterTagged(rpcs, byTag)
	namespaces = filterTagged(namespaces, byTag)

	if *checkOnly {
		if err := checkAllGo(output, plugins, schemas, rpcs, namespaces); err != nil {
			fatal(err)
		}
		if err := checkTaggedFiles(allDir, byTag); err != nil {
			fatal(err)
		}
		fmt.Fprintf(os.Stdout, "%s is current (%d plugins, %d schemas, %d rpcs, %d namespaces, %d gated groups)\n", output, len(plugins), len(schemas), len(rpcs), len(namespaces), len(byTag))
		return
	}

	if err := generateAllGo(output, plugins, schemas, rpcs, namespaces); err != nil {
		fatal(err)
	}
	if err := generateTaggedFiles(allDir, byTag); err != nil {
		fatal(err)
	}

	fmt.Fprintf(os.Stdout, "Generated %s with %d plugins, %d schemas, %d rpcs, %d namespaces, %d gated groups\n", output, len(plugins), len(schemas), len(rpcs), len(namespaces), len(byTag))
}

func fatal(err error) {
	fmt.Println("plugin_imports:", err)
	os.Exit(1)
}

// findModuleRoot walks up from the current directory to find go.mod.
func findModuleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found")
		}
		dir = parent
	}
}

// readModulePath reads the module path from go.mod.
func readModulePath(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module")), nil
		}
	}
	return "", fmt.Errorf("module directive not found in %s", path)
}

// pluginDirs lists existing plugin search roots (relative to repo root).
var pluginDirs = []string{
	"internal/component/bfd",
	"internal/component/bgp/plugin",
	"internal/component/bgp/plugins",
	"internal/component/bgp/reactor/filter",
	"internal/component/firewall/plugins",
	"internal/component/iface",
	"internal/component/sysctl",
	"internal/component/sysrib",
	"internal/component/traffic",
	"internal/component/vpp",
	"internal/plugins",
}

// nestedPluginDomains are component domain roots that may own a plugins/
// subtree. Deriving internal/component/<domain>/plugins from these roots lets
// BNG/VPN clustering move many edge plugins without naming individual plugin
// packages here.
var nestedPluginDomains = []string{
	"ike",
	"l2tp",
}

func pluginSearchRoots() []string {
	roots := make([]string, 0, len(pluginDirs)+len(nestedPluginDomains))
	seen := make(map[string]bool, len(pluginDirs)+len(nestedPluginDomains))
	for _, rel := range pluginDirs {
		if seen[rel] {
			continue
		}
		seen[rel] = true
		roots = append(roots, rel)
	}
	for _, domain := range nestedPluginDomains {
		rel := path.Join("internal/component", domain, "plugins")
		if seen[rel] {
			continue
		}
		seen[rel] = true
		roots = append(roots, rel)
	}
	return roots
}

// rpcRoot is the tree scanned for RPC command packages (any non-test,
// non-schema .go file that calls pluginserver.RegisterRPCs).
const rpcRoot = "internal/component"

// featureGatesManifest is the single source of truth for compile-out-able
// features (repo-relative). Every consumer (this generator, dep_audit.py, the
// Makefile, the test runner) derives from it; see ai/rules/feature-gate-registration.md.
const featureGatesManifest = "feature-gates.txt"

// featureTags maps a discovered import (by repo-relative suffix) to the
// //go:build tag that gates it. A tagged import is emitted into a generated
// all_<tag>.go (//go:build <tag>) instead of the flat all.go, so a build
// without that tag drops the package entirely (per-feature compile-out). It is
// loaded from featureGatesManifest in main() -- NOT hand-maintained here.
// See plan/spec-feature-gate-0-umbrella.md.
var featureTags map[string]string

// loadFeatureTags reads the feature-gate manifest ("<tag> <pkg>" per line) and
// returns the import-suffix map the generator needs: <pkg> -> tag and
// <pkg>/yang -> tag. The direct package entry gates RPC/registration imports
// (gNMI's show command); the YANG entry gates config schema imports. A feature
// without one of those packages simply never matches the corresponding
// discovery list, so deriving both suffixes for every entry is safe.
func loadFeatureTags(root string) (map[string]string, error) {
	f, err := os.Open(filepath.Join(root, featureGatesManifest))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", featureGatesManifest, err)
	}
	defer f.Close()

	tags := map[string]string{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			return nil, fmt.Errorf("%s: malformed line %q (want \"<tag> <pkg>\")", featureGatesManifest, line)
		}
		tag, pkg := fields[0], fields[1]
		tags[pkg] = tag
		tags[path.Join(pkg, "yang")] = tag
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return tags, nil
}

// tagFor returns the build tag gating importPath, or "" if it is universal.
func tagFor(importPath string) string {
	for suffix, tag := range featureTags {
		if !strings.HasSuffix(importPath, suffix) {
			continue
		}
		// Require a path boundary before the suffix (or an exact match) so a
		// suffix like "lg/yang" cannot match an unrelated "...blg/yang".
		boundary := len(importPath) - len(suffix) - 1
		if boundary < 0 || importPath[boundary] == '/' {
			return tag
		}
	}
	return ""
}

// filterTagged removes tagged imports from imps, accumulating them into byTag
// keyed by build tag, and returns the remaining universal imports.
func filterTagged(imps []string, byTag map[string][]string) []string {
	kept := make([]string, 0, len(imps))
	for _, imp := range imps {
		if tag := tagFor(imp); tag != "" {
			byTag[tag] = append(byTag[tag], imp)
			continue
		}
		kept = append(kept, imp)
	}
	return kept
}

// discoverPlugins finds plugin packages by looking for register.go files
// across all known plugin directories. Any register.go that is NOT in a
// schema/ subdirectory is treated as a plugin registration: this catches
// plugins registering via plugin/registry as well as those registering via
// component-local mechanisms (e.g. iface.RegisterBackend in iface/netlink).
func discoverPlugins(root, module string) ([]string, error) {
	var plugins []string
	seenPlugins := make(map[string]bool)
	for _, rel := range pluginSearchRoots() {
		dir := filepath.Join(root, rel)
		err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || d.Name() != "register.go" {
				return nil
			}
			// Skip schema/register.go and yang/register.go (handled by discoverSchemaPackages).
			dirName := filepath.Base(filepath.Dir(path))
			if dirName == "schema" || dirName == "yang" {
				return nil
			}
			// Skip packages marked with "codegen:skip" (e.g. CLI-only
			// command plugins wired via cmd/ze/main.go that would create
			// import cycles if added to plugin/all).
			if fileImports(path, "codegen:skip") {
				return nil
			}
			// Convert to full import path relative to module root.
			pkgRel, err := filepath.Rel(root, filepath.Dir(path))
			if err != nil {
				return err
			}
			plugin := module + "/" + pkgRel
			if seenPlugins[plugin] {
				return nil
			}
			seenPlugins[plugin] = true
			plugins = append(plugins, plugin)
			return nil
		})
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
	}

	sort.Strings(plugins)
	return plugins, nil
}

// discoverSchemaPackages finds schema packages that register YANG modules.
// Scans for schema/register.go files that import the yang package, anywhere
// under internal/. Schema packages under internal/plugins/ are included
// because their plugin parent does not transitively import them.
func discoverSchemaPackages(internalDir, module string) ([]string, error) {
	var imports []string

	err := filepath.Walk(internalDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if isInternalTestPath(path) {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		// Only look at schema/register.go or yang/register.go files.
		dirName := filepath.Base(filepath.Dir(path))
		if filepath.Base(path) != "register.go" || (dirName != "schema" && dirName != "yang") {
			return nil
		}
		// Skip config/yang itself — that's the registry, not a schema package.
		schemaRel, _ := filepath.Rel(internalDir, filepath.Dir(path))
		if schemaRel == filepath.Join("component", "config", "yang") {
			return nil
		}
		// Verify it imports the yang package (for RegisterModule)
		if !fileImports(path, "config/yang") {
			return nil
		}
		// Convert directory to import path
		schemaDir, _ := filepath.Rel(filepath.Dir(internalDir), filepath.Dir(path))
		imports = append(imports, module+"/"+schemaDir)
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Strings(imports)
	return imports, nil
}

// fileImports checks whether a Go source file imports a package matching the substring.
func fileImports(path, substr string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), substr) {
			return true
		}
	}
	return false
}

// discoverRPCPackages finds packages that register RPCs via pluginserver.RegisterRPCs.
// Scans all of internal/component/ and filters out packages that would create
// import cycles (plugin infrastructure, packages that import plugin/all).
func discoverRPCPackages(root, module string) ([]string, error) {
	allPkg := module + "/internal/component/plugin/all"
	serverPkg := module + "/internal/component/plugin/server"

	var rpcs []string

	dir := filepath.Join(root, rpcRoot)
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".go") {
			return nil
		}
		if strings.HasSuffix(d.Name(), "_test.go") {
			return nil
		}
		if filepath.Base(filepath.Dir(path)) == "schema" {
			return nil
		}
		if !fileCallsRegisterRPCs(path) {
			return nil
		}
		pkgRel, err := filepath.Rel(root, filepath.Dir(path))
		if err != nil {
			return err
		}
		rpcs = append(rpcs, module+"/"+pkgRel)
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Strings(rpcs)
	deduped := rpcs[:0]
	for i, p := range rpcs {
		if i == 0 || p != rpcs[i-1] {
			deduped = append(deduped, p)
		}
	}

	filtered := deduped[:0]
	for _, imp := range deduped {
		if imp == allPkg || imp == serverPkg {
			continue
		}
		rel := strings.TrimPrefix(imp, module+"/")
		if pkgImportsPath(filepath.Join(root, rel), "plugin/all") {
			continue
		}
		filtered = append(filtered, imp)
	}
	return filtered, nil
}

// fileCallsRegisterRPCs reports whether path contains a call to RegisterRPCs(
// (not just a reference in comments or the function definition).
func fileCallsRegisterRPCs(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "//") {
			continue
		}
		if strings.Contains(line, "RegisterRPCs(") && !strings.Contains(line, "func ") {
			return true
		}
	}
	return false
}

// pkgImportsPath reports whether any non-test .go file in dir has an
// import statement containing the given path fragment.
func pkgImportsPath(dir, path string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		if fileImportsPath(filepath.Join(dir, e.Name()), path) {
			return true
		}
	}
	return false
}

// fileImportsPath checks for path inside import statements only, ignoring comments.
func fileImportsPath(file, path string) bool {
	f, err := os.Open(file)
	if err != nil {
		return false
	}
	defer f.Close()

	inImport := false
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "import (") {
			inImport = true
			continue
		}
		if inImport && line == ")" {
			inImport = false
			continue
		}
		if inImport && strings.Contains(line, path) && !strings.HasPrefix(line, "//") {
			return true
		}
		if !inImport && strings.HasPrefix(line, "import ") && strings.Contains(line, path) {
			return true
		}
	}
	return false
}

// discoverEventNamespaces finds packages that call .RegisterNamespace() via
// any import alias. Scans all non-test .go files under internal/.
// Packages already in the plugins list are excluded (they're already imported).
func discoverEventNamespaces(internalDir, module string, plugins []string) ([]string, error) {
	pluginSet := make(map[string]bool, len(plugins))
	for _, p := range plugins {
		pluginSet[p] = true
	}

	var imports []string
	seen := make(map[string]bool)

	err := filepath.Walk(internalDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if isInternalTestPath(path) {
			return nil
		}
		if info.IsDir() || !strings.HasSuffix(info.Name(), ".go") {
			return nil
		}
		if strings.HasSuffix(info.Name(), "_test.go") {
			return nil
		}
		if filepath.Base(filepath.Dir(path)) == "schema" {
			return nil
		}
		if !fileImports(path, ".RegisterNamespace(") {
			return nil
		}
		pkgRel, _ := filepath.Rel(filepath.Dir(internalDir), filepath.Dir(path))
		imp := module + "/" + pkgRel
		if pluginSet[imp] || seen[imp] {
			return nil
		}
		seen[imp] = true
		imports = append(imports, imp)
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Strings(imports)
	return imports, nil
}

func isInternalTestPath(path string) bool {
	return strings.Contains(filepath.ToSlash(path), "/internal/test/")
}

// generateAllGo writes the all.go file with blank imports for plugins and schemas.
func generateAllGo(path string, plugins, schemas, rpcs, namespaces []string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	return writeAllGo(bufio.NewWriter(f), plugins, schemas, rpcs, namespaces)
}

// checkAllGo verifies all.go without mutating the working tree.
func checkAllGo(path string, plugins, schemas, rpcs, namespaces []string) error {
	var buf bytes.Buffer
	if err := writeAllGo(bufio.NewWriter(&buf), plugins, schemas, rpcs, namespaces); err != nil {
		return err
	}
	got, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if !bytes.Equal(got, buf.Bytes()) {
		return fmt.Errorf("%s is stale; run make generate", path)
	}
	return nil
}

// taggedFileName returns the generated file name for a build tag's import group
// (e.g. "ze_lg" -> "all_ze_lg.go").
func taggedFileName(tag string) string {
	var b strings.Builder
	b.WriteString("all_")
	b.WriteString(tag)
	b.WriteString(".go")
	return b.String()
}

// ancestorTagsOfImport returns the tags of EVERY other gated package whose path
// is a segment-aligned ancestor of importPath, sorted for determinism. A gated
// package nested inside another gate's package tree is a DEPENDENT piece: it is
// only ever compiled when the ancestor gate is on (it imports the ancestor's
// packages), so its blank import must AND every ancestor tag -- otherwise a
// build requesting the child tag without an ancestor would drag that ancestor's
// whole subtree back in. Example: ze_bmp lives at
// internal/component/bgp/plugins/bmp, under ze_bgp's internal/component/bgp, so
// its import is guarded //go:build ze_bgp && ze_bmp (feature-gate-11).
//
// ALL ancestors are collected, not just the nearest: a package nested two or
// more gates deep (grandparent/parent/child) must AND every level, or the
// constraint fails open and the outer subtrees leak back in when the child tag
// is requested alone. Selecting a single parent was correct only while the
// manifest's maximum nesting depth was 1.
//
// Ancestry is computed PER PACKAGE, not per tag: a tag may mix independent and
// dependent packages (ze_radius gates internal/component/radius, usable alone,
// AND l2tp/plugins/authradius, which needs ze_l2tp), and only the nested subset
// takes the compound constraint (feature-gate-12).
//
// The result is sorted (sort.Strings), so two different iteration orders of the
// featureTags map yield the SAME set of ancestors: the previous strict-> tie-break
// over map iteration order made the chosen ancestor non-deterministic when two
// candidates were equal length.
func ancestorTagsOfImport(importPath, tag string) []string {
	seen := map[string]bool{}
	for pSuffix, pTag := range featureTags {
		if pTag == tag {
			continue
		}
		// pSuffix is a segment-aligned ancestor path of importPath: bounded by a
		// '/' (or the string start) on the left and a '/' (or the end) on the right.
		idx := strings.Index(importPath, pSuffix)
		if idx < 0 {
			continue
		}
		if idx > 0 && importPath[idx-1] != '/' {
			continue
		}
		rest := importPath[idx+len(pSuffix):]
		if rest != "" && rest[0] != '/' {
			continue
		}
		seen[pTag] = true
	}
	tags := make([]string, 0, len(seen))
	for t := range seen {
		tags = append(tags, t)
	}
	sort.Strings(tags)
	return tags
}

// constraintForImport returns the //go:build expression gating one import of a
// tag's group: the tag alone, or "<ancestor> && ... && <tag>" for a dependent
// package nested under one or more OTHER gates (see ancestorTagsOfImport). The
// ancestors are emitted in sorted order with the group's own tag last, so the
// expression is deterministic and, for a package nested N gates deep, closed
// under every ancestor.
func constraintForImport(importPath, tag string) string {
	ancestors := ancestorTagsOfImport(importPath, tag)
	if len(ancestors) == 0 {
		return tag
	}
	var b strings.Builder
	for _, a := range ancestors {
		b.WriteString(a)
		b.WriteString(" && ")
	}
	b.WriteString(tag)
	return b.String()
}

// runSelftest exercises the build-tag constraint logic against synthetic
// feature-gate manifests the real one does not yet contain: a package nested
// two gates deep (which must AND both ancestors, not just the nearest), and an
// equal-length ancestor collision (whose selection must not depend on map
// iteration order). It fails closed -- any mismatch prints the offending case
// and returns a non-zero exit, so the shelling Go test
// (plugin_imports_selftest_test.go) goes red.
func runSelftest() int {
	saved := featureTags
	defer func() { featureTags = saved }()

	var failed []string
	check := func(name, got, want string) {
		if got == want {
			return
		}
		var b strings.Builder
		b.WriteString(name)
		b.WriteString(" (got ")
		b.WriteString(got)
		b.WriteString(", want ")
		b.WriteString(want)
		b.WriteByte(')')
		failed = append(failed, b.String())
	}

	// Finding 1 -- multi-level nesting: grandparent gp -> parent p -> child c.
	// A package two gates deep must AND BOTH ancestors; emitting only the nearest
	// fails open and leaks the grandparent subtree.
	featureTags = map[string]string{
		"internal/a":     "ze_gp",
		"internal/a/b":   "ze_p",
		"internal/a/b/c": "ze_c",
		"internal/solo":  "ze_solo",
	}
	check("depth0", constraintForImport("example.com/m/internal/a", "ze_gp"), "ze_gp")
	check("depth1", constraintForImport("example.com/m/internal/a/b", "ze_p"), "ze_gp && ze_p")
	check("depth2", constraintForImport("example.com/m/internal/a/b/c", "ze_c"), "ze_gp && ze_p && ze_c")
	check("independent", constraintForImport("example.com/m/internal/solo", "ze_solo"), "ze_solo")

	// Finding 2 -- determinism on an equal-length ancestor collision. Two gate
	// paths of the SAME length both segment-align as ancestors of one import; the
	// old strict-> selection returned whichever the map yielded first. All
	// ancestors are now collected and sorted, so the constraint is stable across
	// the map iteration orders Go randomizes per range.
	featureTags = map[string]string{
		"seg/aaa": "ze_two",
		"seg/bbb": "ze_one",
	}
	const tieImport = "example.com/m/seg/aaa/seg/bbb/leaf"
	const tieWant = "ze_one && ze_two && ze_leaf"
	for range 200 {
		if got := constraintForImport(tieImport, "ze_leaf"); got != tieWant {
			check("determinism", got, tieWant)
			break
		}
	}
	check("determinism-final", constraintForImport(tieImport, "ze_leaf"), tieWant)
	order, _ := constraintGroups("ze_leaf", []string{tieImport})
	var ob strings.Builder
	for i, o := range order {
		if i > 0 {
			ob.WriteByte('|')
		}
		ob.WriteString(o)
	}
	check("groups-order", ob.String(), tieWant)

	if len(failed) > 0 {
		var b strings.Builder
		b.WriteString("selftest FAILED:\n")
		for _, f := range failed {
			b.WriteString("  ")
			b.WriteString(f)
			b.WriteByte('\n')
		}
		os.Stdout.WriteString(b.String())
		return 1
	}
	os.Stdout.WriteString("selftest: PASS\n")
	return 0
}

// constraintGroups splits a tag's imports by their per-import constraint,
// returning the constraint expressions in deterministic order (plain tag
// first, then sorted compounds). A tag whose packages all share one constraint
// yields a single group -- the pre-feature-gate-12 behavior (ze_bmp keeps its
// one ze_bgp && ze_bmp file).
func constraintGroups(tag string, imports []string) (order []string, byConstraint map[string][]string) {
	byConstraint = make(map[string][]string)
	for _, imp := range imports {
		c := constraintForImport(imp, tag)
		byConstraint[c] = append(byConstraint[c], imp)
	}
	for c := range byConstraint {
		order = append(order, c)
	}
	sort.Slice(order, func(i, j int) bool {
		if (order[i] == tag) != (order[j] == tag) {
			return order[i] == tag // plain tag group first
		}
		return order[i] < order[j]
	})
	return order, byConstraint
}

// writeTaggedGo writes a //go:build <constraint> file blank-importing the gated
// packages, mirroring all.go's generated-marker style. Built with a string
// builder (no fmt verbs) and written in one call so the output is byte-stable
// for the --check comparison. `tag` names the group (file + comment);
// `constraint` is the build expression (tag alone, or "<parent> && <tag>").
func writeTaggedGo(w *bufio.Writer, tag, constraint string, imports []string) error {
	var b strings.Builder
	b.WriteString("// Code generated by scripts/codegen/plugin_imports.go; DO NOT EDIT.\n\n")
	b.WriteString("//go:build ")
	b.WriteString(constraint)
	b.WriteString("\n\n")
	b.WriteString("// Package all (")
	b.WriteString(tag)
	b.WriteString(" group): build-tag-gated plugin/schema imports,\n")
	b.WriteString("// dropped from the binary when ")
	b.WriteString(tag)
	b.WriteString(" is absent (per-feature compile-out).\n")
	b.WriteString("package all\n\n")
	b.WriteString("import (\n")
	// Dedup adjacent duplicates: a package discovered in more than one category
	// (e.g. plugin register.go AND a RegisterRPCs call) is pooled into this tag
	// group twice by filterTagged. Go tolerates duplicate blank imports, but one
	// per package is cleaner. imports is sorted by the caller, so adjacent dedup
	// suffices.
	prev := ""
	for i, imp := range imports {
		if i > 0 && imp == prev {
			continue
		}
		prev = imp
		b.WriteString("\t_ \"")
		b.WriteString(imp)
		b.WriteString("\"\n")
	}
	// ")\n", not ")\n\n": a trailing blank line is not gofmt-clean, and `make fmt`
	// strips it right back off, so emitting one puts the generator and the
	// formatter in a loop where each undoes the other on every run.
	b.WriteString(")\n")
	if _, err := w.WriteString(b.String()); err != nil {
		return err
	}
	return w.Flush()
}

// taggedGroupFileName names one constraint group's file. The tag's first
// (plain, or sole) group keeps the historic all_<tag>.go name; a dependent
// subset of a MIXED tag gets all_<tag>_<parent>.go (e.g.
// all_ze_radius_ze_l2tp.go for authradius under ze_l2tp).
func taggedGroupFileName(tag, constraint string, sole bool) string {
	if sole || constraint == tag {
		return taggedFileName(tag)
	}
	parent, _, _ := strings.Cut(constraint, " && ")
	var b strings.Builder
	b.WriteString("all_")
	b.WriteString(tag)
	b.WriteString("_")
	b.WriteString(parent)
	b.WriteString(".go")
	return b.String()
}

// generateTaggedFiles writes one all_<tag>.go per build tag (plus an
// all_<tag>_<parent>.go per dependent subset of a mixed tag) and removes any
// previously-generated tag file that is no longer needed.
func generateTaggedFiles(dir string, byTag map[string][]string) error {
	expected := make(map[string]bool, len(byTag))
	for tag, imports := range byTag {
		sort.Strings(imports)
		order, groups := constraintGroups(tag, imports)
		for _, constraint := range order {
			name := taggedGroupFileName(tag, constraint, len(order) == 1)
			expected[name] = true
			f, err := os.Create(filepath.Join(dir, name))
			if err != nil {
				return err
			}
			if err := writeTaggedGo(bufio.NewWriter(f), tag, constraint, groups[constraint]); err != nil {
				f.Close() //nolint:errcheck // best-effort on error path
				return err
			}
			if err := f.Close(); err != nil {
				return err
			}
		}
	}
	return removeStaleTaggedFiles(dir, expected)
}

// checkTaggedFiles verifies every generated tag-group file matches what would
// be generated and that no stale generated tag file remains.
func checkTaggedFiles(dir string, byTag map[string][]string) error {
	expected := make(map[string]bool, len(byTag))
	for tag, imports := range byTag {
		sort.Strings(imports)
		order, groups := constraintGroups(tag, imports)
		for _, constraint := range order {
			name := taggedGroupFileName(tag, constraint, len(order) == 1)
			expected[name] = true
			var buf bytes.Buffer
			if err := writeTaggedGo(bufio.NewWriter(&buf), tag, constraint, groups[constraint]); err != nil {
				return err
			}
			got, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				return fmt.Errorf("%s missing; run make generate", name)
			}
			if !bytes.Equal(got, buf.Bytes()) {
				return fmt.Errorf("%s is stale; run make generate", name)
			}
		}
	}
	return checkNoStaleTaggedFiles(dir, expected)
}

// generatedTagFile reports whether name is a generated all_<tag>.go (not a test
// file) carrying the generated marker -- the only files we will remove.
func generatedTagFile(dir, name string) bool {
	if !strings.HasPrefix(name, "all_") || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
		return false
	}
	return fileImports(filepath.Join(dir, name), "Code generated by scripts/codegen/plugin_imports.go")
}

// removeStaleTaggedFiles deletes generated all_<tag>.go files not in expected.
func removeStaleTaggedFiles(dir string, expected map[string]bool) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		name := e.Name()
		if expected[name] || !generatedTagFile(dir, name) {
			continue
		}
		if err := os.Remove(filepath.Join(dir, name)); err != nil {
			return err
		}
	}
	return nil
}

// checkNoStaleTaggedFiles errors if a generated all_<tag>.go exists that is not
// in expected (a tag was removed but its file lingers).
func checkNoStaleTaggedFiles(dir string, expected map[string]bool) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		name := e.Name()
		if expected[name] || !generatedTagFile(dir, name) {
			continue
		}
		return fmt.Errorf("%s is stale (no longer gated); run make generate", name)
	}
	return nil
}

func writeAllGo(w *bufio.Writer, plugins, schemas, rpcs, namespaces []string) error {

	fmt.Fprintln(w, "// Code generated by scripts/codegen/plugin_imports.go; DO NOT EDIT.")                        //nolint:errcheck // output
	fmt.Fprintln(w)                                                                                                //nolint:errcheck // output
	fmt.Fprintln(w, "// Package all imports all internal plugins and schema packages,")                            //nolint:errcheck // output
	fmt.Fprintln(w, "// triggering their init() registration.")                                                    //nolint:errcheck // output
	fmt.Fprintln(w, "//")                                                                                          //nolint:errcheck // output
	fmt.Fprintln(w, "// To add a plugin, create internal/component/bgp/plugins/<name>/register.go with an init()") //nolint:errcheck // output
	fmt.Fprintln(w, "// that calls registry.Register(). Then run: make generate")                                  //nolint:errcheck // output
	fmt.Fprintln(w, "package all")                                                                                 //nolint:errcheck // output
	fmt.Fprintln(w)                                                                                                //nolint:errcheck // output
	fmt.Fprintln(w, "import (")                                                                                    //nolint:errcheck // output

	// Schema packages first (infrastructure)
	if len(schemas) > 0 {
		fmt.Fprintln(w, "\t// Infrastructure schema packages — YANG module registration.") //nolint:errcheck // output
		for _, imp := range schemas {
			fmt.Fprintf(w, "\t_ \"%s\"\n", imp) //nolint:errcheck // output
		}
		fmt.Fprintln(w)                                                         //nolint:errcheck // output
		fmt.Fprintln(w, "\t// Plugin packages — plugin + schema registration.") //nolint:errcheck // output
	}

	for _, imp := range plugins {
		fmt.Fprintf(w, "\t_ \"%s\"\n", imp) //nolint:errcheck // output
	}

	// Event namespace packages -- events.RegisterNamespace registration.
	if len(namespaces) > 0 {
		fmt.Fprintln(w)                                                                            //nolint:errcheck // output
		fmt.Fprintln(w, "\t// Event namespace packages -- events.RegisterNamespace registration.") //nolint:errcheck // output
		for _, imp := range namespaces {
			fmt.Fprintf(w, "\t_ \"%s\"\n", imp) //nolint:errcheck // output
		}
	}

	// RPC command packages -- init() registers RPCs via pluginserver.RegisterRPCs.
	if len(rpcs) > 0 {
		fmt.Fprintln(w)                                                                         //nolint:errcheck // output
		fmt.Fprintln(w, "\t// RPC command packages -- pluginserver.RegisterRPCs registration.") //nolint:errcheck // output
		for _, imp := range rpcs {
			fmt.Fprintf(w, "\t_ \"%s\"\n", imp) //nolint:errcheck // output
		}
	}
	// No blank line after ")": see writeTaggedGo -- a trailing blank line is not
	// gofmt-clean, so `make fmt` and `make generate` would each undo the other.
	fmt.Fprintln(w, ")") //nolint:errcheck // output

	return w.Flush()
}
