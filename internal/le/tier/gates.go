// Design: ai/rules/architecture.md -- the five checks the tier gate runs
//
// Overview: tier.go -- the import audit these checks read
//
// gates.go contains the five `le tier check` checks in script order: engine
// placement, non-engine categories, core imports, disableable-feature imports,
// and lint build-tag drift.
//
// Each check answers its page and code. The run uses the FIRST nonzero code, so
// callers act on the first failure.
package tier

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/pyfmt"
)

// The files the gate reads and writes, all tree-relative.
const (
	// Baseline contains engines in the wrong tier that are scheduled to move.
	// New violations and stale entries fail. Thus, the file can only shrink, and
	// an empty file means full enforcement.
	Baseline = "scripts/dev/tier_migration_baseline.txt"
	// NonEngineCategories is the human-readable source of truth for
	// intentional non-engine placements.
	NonEngineCategories = "scripts/dev/tier_non_engine_categories.txt"
	// CoreImportBaseline holds the grandfathered upward imports out of
	// internal/core, at PAIR granularity so a new pair in an already-baselined
	// file is still caught.
	CoreImportBaseline = "scripts/dev/core_import_baseline.txt"
	// FeatureGatesManifest is the single declaration point for the
	// compile-out-able features.
	FeatureGatesManifest = "feature-gates.txt"
	// Golangci is the lint configuration whose build-tags list is static YAML
	// and therefore cannot read the manifest at run time.
	Golangci = ".golangci.yml"
)

// CategoryPlannedViolation is the one category whose rationale must cite the
// spec that removes it, which is what stops it becoming a permanent home.
const CategoryPlannedViolation = "planned-violation"

// LegalNonEngineCategories contains the four categories that a non-engine
// placement can declare.
var LegalNonEngineCategories = [...]string{"domain-library", "framework", "host-service", CategoryPlannedViolation}

// DomainLibraryPrefixes contains the two subtrees permitted in a domain-library row.
var DomainLibraryPrefixes = [...]string{"internal/component/l2tp", "internal/component/ike"}

// NonFeaturePrefixes are the trees whose import of an engine is not a feature
// depending on it.
var NonFeaturePrefixes = [...]string{"cmd/ze/", "internal/core/", "internal/chaos/", "internal/test/"}

// DisableableNonProdPrefixes are the trees that are NOT part of the production
// ze daemon and therefore cannot pin a disableable feature into it.
//
// cmd/ze is deliberately absent: the gated service and seam files live there
// and MUST carry the build tag, so they stay in scope.
var DisableableNonProdPrefixes = [...]string{
	"internal/chaos/",
	"internal/test/",
	"internal/perf/",
	"scripts/",
}

// GolangciBaseTags are the non-feature build tags that legitimately appear in
// the lint configuration beside every gate tag.
var GolangciBaseTags = [...]string{"ze_core"}

// The core tier's own rule: internal/core is the leaf library tier and imports
// neither of the two areas above it.
const CoreAreaPrefix = "internal/core/"

// CoreForbidden are the two areas internal/core must not import.
var CoreForbidden = [...]string{AreaComponent, AreaPlugins}

// CoreFixRoutes are the routes a baselined upward import must name, so nothing
// can be baselined without one.
var CoreFixRoutes = [...]string{"hand-fixable", "generator-fixable", "needs-design"}

// ---------------------------------------------------------------------------
// Engine placement
// ---------------------------------------------------------------------------

// ReadBaseline answers the engine directories the migration baseline names.
func ReadBaseline(tree string) (map[string]bool, error) {
	keys := make(map[string]bool)
	raw, err := os.ReadFile(filepath.Join(tree, Baseline)) //nolint:gosec // a manifest of the tree the caller named
	if errors.Is(err, fs.ErrNotExist) {
		return keys, nil
	}
	if err != nil {
		return nil, err
	}
	for line := range strings.SplitSeq(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		keys[strings.Fields(line)[0]] = true
	}
	return keys, nil
}

// baselineHeader is the comment block the baseline writer emits, one line per
// element.
var baselineHeader = [...]string{
	"# Tier migration baseline -- TRANSITIONAL, not a permanent allowlist.",
	"# Each row is a misplaced engine scheduled to move; the gate FAILS on a NEW",
	"# misplacement and on a STALE entry (one no longer misplaced). An empty file",
	"# means full engine-placement enforcement with zero exceptions.",
	"# See ai/rules/architecture.md and spec-tiers-0-umbrella (closed).",
	`# columns: <current-dir>\t<expected-area>\t<resolving-child-spec>`,
}

// WriteBaseline regenerates the migration baseline from the misplacements the
// tree currently holds.
func WriteBaseline(tree string, misplaced map[string]string) error {
	var tb textbuf.Buffer
	for _, line := range baselineHeader {
		tb.Str(line).Byte('\n')
	}
	for _, engine := range sortedKeys(misplaced) {
		spec := "spec-tiers-3"
		if strings.HasSuffix(misplaced[engine], "plugins") {
			spec = "spec-tiers-2"
		}
		tb.Str(engine).Byte('\t').Str(misplaced[engine]).Byte('\t').Str(spec).Byte('\n')
	}

	out := filepath.Join(tree, Baseline)
	if err := os.MkdirAll(filepath.Dir(out), 0o750); err != nil {
		return err
	}
	return os.WriteFile(out, []byte(tb.String()), 0o600)
}

// enginePlacementGate compares the engines that are in the wrong tier against
// the baseline that schedules their move.
func enginePlacementGate(tree, module string, edges Edges) (GateResult, error) {
	misplaced, err := EngineMisplacements(tree, module, edges)
	if err != nil {
		return GateResult{}, err
	}
	baseline, err := ReadBaseline(tree)
	if err != nil {
		return GateResult{}, err
	}

	var arrived, stale []string
	for engine := range misplaced {
		if !baseline[engine] {
			arrived = append(arrived, engine)
		}
	}
	for engine := range baseline {
		if _, ok := misplaced[engine]; !ok {
			stale = append(stale, engine)
		}
	}
	sort.Strings(arrived)
	sort.Strings(stale)

	result := GateResult{Name: "engine-placement"}
	var tb textbuf.Buffer
	if len(arrived) > 0 {
		tb.Str("FAIL: new misplaced engine(s) -- wrong module tier:\n")
		for _, engine := range arrived {
			tb.Str("  ").Str(engine).Str("  must move to ").Str(misplaced[engine]).Str("/\n")
		}
		tb.Str("  Rule: ai/rules/architecture.md (engine -> component if a feature depends on it, else plugins).\n")
		tb.Str("  If this is an intentional, scheduled move, add it to ").Str(Baseline).Str(".\n")
	}
	if len(stale) > 0 {
		tb.Str("FAIL: stale baseline entry(ies) -- no longer misplaced, remove from ").Str(Baseline).Str(":\n")
		for _, engine := range stale {
			tb.Str("  ").Str(engine).Byte('\n')
		}
	}
	if len(arrived) > 0 || len(stale) > 0 {
		result.Diagnosis = tb.String()
		result.Code = 2
		return result, nil
	}

	if len(misplaced) > 0 {
		tb.Str("OK: engine placement clean; ").Int(int64(len(misplaced))).
			Str(" engine(s) baselined (pending migration):\n")
		for _, engine := range sortedKeys(misplaced) {
			tb.Str("  ").Str(engine).Str(" -> ").Str(misplaced[engine]).Str("/\n")
		}
	} else {
		tb.Str("OK: engine placement clean; no exceptions (baseline empty).\n")
	}
	tb.Str("advisory: engine placement is mechanical; intentional non-engine framework/host/domain placements are declared in ").
		Str(NonEngineCategories).Str(".\n")
	result.Page = tb.String()
	return result, nil
}

// ---------------------------------------------------------------------------
// Non-engine category manifest
// ---------------------------------------------------------------------------

// ManifestRow is one declared non-engine placement.
type ManifestRow struct {
	Category  string
	Rationale string
	Line      int
}

// specReferenceRe is the spec citation a planned-violation rationale must carry.
var specReferenceRe = regexp.MustCompile(`\bspec-[A-Za-z0-9][A-Za-z0-9_.-]*\b`)

// LoadNonEngineCategories parses the manifest into its rows and the problems
// with the file itself.
func LoadNonEngineCategories(tree string) (map[string]ManifestRow, []string, error) {
	rows := make(map[string]ManifestRow)
	raw, err := os.ReadFile(filepath.Join(tree, NonEngineCategories)) //nolint:gosec // a manifest of the tree the caller named
	if errors.Is(err, fs.ErrNotExist) {
		var tb textbuf.Buffer
		return rows, []string{tb.Str(NonEngineCategories).Str(": missing non-engine category manifest").String()}, nil
	}
	if err != nil {
		return nil, nil, err
	}

	legal := make(map[string]bool, len(LegalNonEngineCategories))
	for _, category := range LegalNonEngineCategories {
		legal[category] = true
	}

	var problems []string
	for number, line := range strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n") {
		lineNumber := number + 1
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		parts := splitN(trimmed, 3)
		if len(parts) != 3 || strings.TrimSpace(parts[2]) == "" {
			problems = append(problems, manifestProblem(lineNumber, "expected '<path> <category> <rationale>'"))
			continue
		}
		rel, category, rationale := parts[0], parts[1], strings.TrimSpace(parts[2])
		if _, ok := rows[rel]; ok {
			var tb textbuf.Buffer
			problems = append(problems, manifestProblem(lineNumber, tb.Str("duplicate row for ").Str(rel).String()))
			continue
		}
		if !legal[category] {
			var tb textbuf.Buffer
			problems = append(problems, manifestProblem(lineNumber, tb.Str("unknown category ").Str(pyfmt.Repr(category)).
				Str("; expected one of ").Join(LegalNonEngineCategories[:], ", ").String()))
			continue
		}
		if category == CategoryPlannedViolation && !specReferenceRe.MatchString(rationale) {
			problems = append(problems, manifestProblem(lineNumber, "planned-violation rationale must cite a spec-* reference"))
			continue
		}
		rows[rel] = ManifestRow{Category: category, Rationale: rationale, Line: lineNumber}
	}
	return rows, problems, nil
}

// manifestProblem renders one manifest problem, which names the file and the
// line it is about.
func manifestProblem(line int, detail string) string {
	var tb textbuf.Buffer
	return tb.Str(NonEngineCategories).Byte(':').Int(int64(line)).Str(": ").Str(detail).String()
}

// splitN splits on runs of whitespace into at most n fields, which is Python's
// str.split(None, n-1).
func splitN(line string, n int) []string {
	fields := make([]string, 0, n)
	rest := line
	for len(fields) < n-1 {
		rest = strings.TrimLeft(rest, " \t")
		if rest == "" {
			break
		}
		end := strings.IndexAny(rest, " \t")
		if end < 0 {
			fields = append(fields, rest)
			return fields
		}
		fields = append(fields, rest[:end])
		rest = rest[end:]
	}
	rest = strings.TrimLeft(rest, " \t")
	if rest != "" {
		fields = append(fields, rest)
	}
	return fields
}

// NonEngineArea answers the registry area a manifest row's path sits under, or
// the empty string when it sits under neither.
func NonEngineArea(rel string) string {
	switch {
	case strings.HasPrefix(rel, "internal/component/"):
		return AreaComponent
	case strings.HasPrefix(rel, "internal/plugins/"):
		return AreaPlugins
	}
	return ""
}

// under reports whether rel is the prefix or sits inside it.
func under(rel, prefix string) bool {
	return underDir(rel, prefix)
}

// setupFeatureRegistration reports a cmd/ze per-feature setup registration.
func setupFeatureRegistration(importer string) bool {
	return strings.HasPrefix(importer, "cmd/ze/") && setupFeatureRe.MatchString(importer)
}

// ValidNonEngineCategory reports whether a row's category is legal WHERE it
// sits, and says why when it is not.
func ValidNonEngineCategory(rel, category string, row Row) (bool, string) {
	switch category {
	case "framework":
		if strings.HasPrefix(rel, "internal/component/") {
			return true, ""
		}
		if strings.HasPrefix(rel, "internal/plugins/") && slices.ContainsFunc(row.Registration, setupFeatureRegistration) {
			return true, ""
		}
		return false, "framework rows under internal/plugins must be setup-feature registrations"
	case "host-service":
		if strings.HasPrefix(rel, "internal/component/") {
			return true, ""
		}
		return false, "host-service rows are allowed only under internal/component"
	case "domain-library":
		for _, prefix := range DomainLibraryPrefixes {
			if under(rel, prefix) {
				return true, ""
			}
		}
		var tb textbuf.Buffer
		return false, tb.Str("domain-library rows are allowed only under ").Join(DomainLibraryPrefixes[:], ", ").String()
	case CategoryPlannedViolation:
		if strings.HasPrefix(rel, "internal/component/") || strings.HasPrefix(rel, "internal/plugins/") {
			return true, ""
		}
		return false, "planned-violation rows are allowed only under internal/component or internal/plugins"
	}
	var tb textbuf.Buffer
	return false, tb.Str("unknown category ").Str(pyfmt.Repr(category)).String()
}

// NonEngineCategoryProblems answers everything wrong with the manifest and with
// the placements it is supposed to classify.
func NonEngineCategoryProblems(tree, module string, edges Edges) ([]string, error) {
	manifest, problems, err := LoadNonEngineCategories(tree)
	if err != nil {
		return nil, err
	}

	engineDirList, err := FindEngineDirs(tree, NestedNamespaces(PluginDirs()))
	if err != nil {
		return nil, err
	}
	engineDirs := make(map[string]bool, len(engineDirList))
	engines := make(map[string]bool, len(engineDirList))
	for _, dir := range engineDirList {
		engineDirs[dir] = true
		engines[TopSubsystem(dir)] = true
	}

	audited := make(map[string]Row)
	for _, area := range [...]string{AreaComponent, AreaPlugins} {
		rows, classifyErr := Classify(area, tree, module, edges, engines)
		if classifyErr != nil {
			return nil, classifyErr
		}
		for _, row := range rows {
			var tb textbuf.Buffer
			audited[tb.Str(area).Byte('/').Str(row.Name).String()] = row
		}
	}

	for _, rel := range sortedManifestKeys(manifest) {
		meta := manifest[rel]
		if NonEngineArea(rel) == "" {
			var tb textbuf.Buffer
			problems = append(problems, manifestProblem(meta.Line,
				tb.Str(rel).Str(" is outside internal/component or internal/plugins").String()))
			continue
		}
		if info, statErr := os.Stat(filepath.Join(tree, rel)); statErr != nil || !info.IsDir() {
			var tb textbuf.Buffer
			problems = append(problems, manifestProblem(meta.Line, tb.Str(rel).Str(" does not exist").String()))
			continue
		}

		row, ok := audited[rel]
		if !ok {
			row = edges.classifyRel(rel, module, engineDirs)
		}
		if row.IsEngine {
			var tb textbuf.Buffer
			problems = append(problems, manifestProblem(meta.Line, tb.Str(rel).
				Str(" is an engine; engine placement is mechanical, not a non-engine category").String()))
		}
		if valid, reason := ValidNonEngineCategory(rel, meta.Category, row); !valid {
			var tb textbuf.Buffer
			problems = append(problems, manifestProblem(meta.Line, tb.Str(rel).Str(" uses category ").
				Str(pyfmt.Repr(meta.Category)).Str(": ").Str(reason).String()))
		}
	}

	for _, rel := range sortedRowKeys(audited) {
		row := audited[rel]
		if row.IsEngine || row.IsRegistered {
			continue
		}
		if _, ok := manifest[rel]; ok {
			continue
		}
		var tb textbuf.Buffer
		problems = append(problems, tb.Str(rel).Str(": unclassified non-engine placement; add a ").
			Str(NonEngineCategories).Str(" row or move it to the mechanical tier").String())
	}
	return problems, nil
}

// nonEngineCategoryGate reports whether the manifest and the placements agree.
func nonEngineCategoryGate(tree, module string, edges Edges) (GateResult, error) {
	problems, err := NonEngineCategoryProblems(tree, module, edges)
	if err != nil {
		return GateResult{}, err
	}

	result := GateResult{Name: "non-engine-categories"}
	var tb textbuf.Buffer
	if len(problems) > 0 {
		tb.Str("FAIL: non-engine tier category manifest mismatch:\n")
		for _, problem := range problems {
			tb.Str("  ").Str(problem).Byte('\n')
		}
		tb.Str("  Rule: ai/rules/architecture.md (non-engine categories are manifest-backed).\n")
		result.Diagnosis = tb.String()
		result.Code = 2
		return result, nil
	}

	manifest, _, err := LoadNonEngineCategories(tree)
	if err != nil {
		return GateResult{}, err
	}
	result.Page = tb.Str("OK: non-engine placement categories clean; ").Int(int64(len(manifest))).
		Str(" manifest row(s).\n").String()
	return result, nil
}

// ---------------------------------------------------------------------------
// Core import direction
// ---------------------------------------------------------------------------

// CorePair is one upward import out of internal/core: the file that makes it
// and the package it reaches.
type CorePair struct {
	File    string
	Package string
}

// CoreDirectionViolations answers every upward import out of internal/core.
// Test files count: the grandfathered set includes one.
func CoreDirectionViolations(module string, edges Edges) []CorePair {
	bases := make([]string, 0, len(CoreForbidden))
	for _, area := range CoreForbidden {
		var tb textbuf.Buffer
		bases = append(bases, tb.Str(module).Byte('/').Str(area).String())
	}

	seen := make(map[CorePair]bool)
	var pairs []CorePair
	for imported, importers := range edges {
		matched := false
		for _, base := range bases {
			if underDir(imported, base) {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		rel := imported[len(module)+1:]
		for _, importer := range importers {
			if !strings.HasPrefix(importer, CoreAreaPrefix) {
				continue
			}
			pair := CorePair{File: importer, Package: rel}
			if seen[pair] {
				continue
			}
			seen[pair] = true
			pairs = append(pairs, pair)
		}
	}
	sortPairs(pairs)
	return pairs
}

// ReadCoreImportBaseline parses the shrink-only baseline. An illegal fix route
// is a problem, so nothing can be baselined without a named route.
func ReadCoreImportBaseline(tree string) ([]CorePair, []string, error) {
	raw, err := os.ReadFile(filepath.Join(tree, CoreImportBaseline)) //nolint:gosec // a baseline of the tree the caller named
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}

	legal := make(map[string]bool, len(CoreFixRoutes))
	for _, route := range CoreFixRoutes {
		legal[route] = true
	}

	var pairs []CorePair
	var problems []string
	for number, line := range strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n") {
		lineNumber := number + 1
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		parts := splitN(trimmed, 3)
		if len(parts) != 3 {
			problems = append(problems, coreProblem(lineNumber,
				"expected '<core-file> <imported-package> <fix-route>: <rationale>'"))
			continue
		}
		route, _, _ := strings.Cut(parts[2], ":")
		route = strings.TrimSpace(route)
		if !legal[route] {
			var tb textbuf.Buffer
			problems = append(problems, coreProblem(lineNumber, tb.Str("fix route ").Str(pyfmt.Repr(route)).
				Str(" not one of ").Str(pyfmt.List(sortedCopy(CoreFixRoutes[:]))).String()))
			continue
		}
		pairs = append(pairs, CorePair{File: parts[0], Package: parts[1]})
	}
	sortPairs(pairs)
	return pairs, problems, nil
}

// coreProblem renders one baseline problem, which names the file and the line.
func coreProblem(line int, detail string) string {
	var tb textbuf.Buffer
	return tb.Str(CoreImportBaseline).Byte(':').Int(int64(line)).Str(": ").Str(detail).String()
}

// coreDirectionGate compares the upward imports against the shrink-only
// baseline: a new pair and a stale row both fail.
func coreDirectionGate(tree, module string, edges Edges) (GateResult, error) {
	current := CoreDirectionViolations(module, edges)
	baseline, problems, err := ReadCoreImportBaseline(tree)
	if err != nil {
		return GateResult{}, err
	}

	result := GateResult{Name: "core-import-direction"}
	var tb textbuf.Buffer
	if len(problems) > 0 {
		tb.Str("FAIL: ").Str(CoreImportBaseline).Str(" malformed:\n")
		for _, problem := range problems {
			tb.Str("  ").Str(problem).Byte('\n')
		}
		result.Diagnosis = tb.String()
		result.Code = 2
		return result, nil
	}

	arrived := pairsNotIn(current, baseline)
	stale := pairsNotIn(baseline, current)
	if len(arrived) > 0 {
		tb.Str("FAIL: new upward import(s) from internal/core -- wrong direction:\n")
		for _, pair := range arrived {
			tb.Str("  ").Str(pair.File).Str(" imports ").Str(pair.Package).Byte('\n')
		}
		tb.Str("  Rule: internal/core is the leaf tier and imports neither internal/component nor internal/plugins (ai/rules/architecture.md).\n")
		tb.Str("  A deliberate, spec-referenced exception goes in ").Str(CoreImportBaseline).
			Str(" with a fix route (hand-fixable / generator-fixable / needs-design).\n")
	}
	if len(stale) > 0 {
		tb.Str("FAIL: stale core-import baseline row(s) -- the upward import is gone, remove from ").
			Str(CoreImportBaseline).Str(":\n")
		for _, pair := range stale {
			tb.Str("  ").Str(pair.File).Str(" imports ").Str(pair.Package).Byte('\n')
		}
	}
	if len(arrived) > 0 || len(stale) > 0 {
		result.Diagnosis = tb.String()
		result.Code = 2
		return result, nil
	}

	if len(current) > 0 {
		files := make(map[string]bool, len(current))
		for _, pair := range current {
			files[pair.File] = true
		}
		tb.Str("OK: core import direction clean; ").Int(int64(len(current))).Str(" pair(s) in ").
			Int(int64(len(files))).Str(" file(s) baselined (pending fix).\n")
	} else {
		tb.Str("OK: core import direction clean; no exceptions (baseline empty).\n")
	}
	result.Page = tb.String()
	return result, nil
}

// pairsNotIn answers the pairs of left that right does not hold.
func pairsNotIn(left, right []CorePair) []CorePair {
	held := make(map[CorePair]bool, len(right))
	for _, pair := range right {
		held[pair] = true
	}
	var out []CorePair
	for _, pair := range left {
		if !held[pair] {
			out = append(out, pair)
		}
	}
	sortPairs(out)
	return out
}

// sortPairs orders pairs by file then package, which is Python's tuple order.
func sortPairs(pairs []CorePair) {
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].File != pairs[j].File {
			return pairs[i].File < pairs[j].File
		}
		return pairs[i].Package < pairs[j].Package
	})
}

// ---------------------------------------------------------------------------
// Disableable-feature direct imports
// ---------------------------------------------------------------------------

// LoadFeatureGates parses the manifest into a gated-package -> build-tag map.
//
// The manifest is the DISABLEABLE map: the package each //go:build ze_<tag>
// guards. A manifest that cannot be read stops the gate, because a gate map
// that came back empty would find no violation anywhere.
func LoadFeatureGates(tree string) (map[string]string, error) {
	raw, err := os.ReadFile(filepath.Join(tree, FeatureGatesManifest)) //nolint:gosec // a manifest of the tree the caller named
	if err != nil {
		return nil, err
	}

	gates := make(map[string]string)
	for line := range strings.SplitSeq(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			var tb textbuf.Buffer
			return nil, errors.New(tb.Str(FeatureGatesManifest).Str(": malformed line ").
				Str(pyfmt.Repr(line)).Str(" (want '<tag> <pkg>')").String())
		}
		gates[parts[1]] = parts[0]
	}
	return gates, nil
}

// tagRequired reports whether a //go:build constraint positively requires a tag.
func tagRequired(constraint, tag string) bool {
	quoted := regexp.QuoteMeta(tag)
	var tb textbuf.Buffer
	negated := regexp.MustCompile(tb.Str(`!\s*`).Str(quoted).Str(`\b`).String())
	if negated.MatchString(constraint) {
		return false
	}
	tb.Reset()
	return regexp.MustCompile(tb.Str(`\b`).Str(quoted).Str(`\b`).String()).MatchString(constraint)
}

// FileRequiresTag reports whether a file carries a build constraint requiring
// the tag.
//
// A file that cannot be read answers false, which REPORTS a violation. That is
// the fail-closed direction and it is kept: an unreadable importer is treated
// as an always-on one until somebody looks.
func FileRequiresTag(tree, rel, tag string) bool {
	raw, err := os.ReadFile(filepath.Join(tree, rel)) //nolint:gosec // a Go file of the tree the caller named
	if err != nil {
		return false
	}
	for line := range strings.SplitSeq(string(raw), "\n") {
		trimmed := strings.TrimSpace(line)
		if constraint, ok := strings.CutPrefix(trimmed, "//go:build"); ok {
			return tagRequired(constraint, tag)
		}
		if trimmed != "" && !strings.HasPrefix(trimmed, "//") {
			return false // reached code before any build constraint
		}
	}
	return false
}

// sameFeatureImporter reports whether a file is in ANY package with the same
// tag.
//
// Such an importer belongs to the feature and disappears with the imported
// package when the tag is off. It cannot pin that package into a no-tag build.
// A protocol's engine, transport, and cli packages can therefore import one
// another.
func sameFeatureImporter(importer, tag string, gates map[string]string) bool {
	for pkg, other := range gates {
		if other == tag && underDir(importer, pkg) {
			return true
		}
	}
	return false
}

// DisableableViolation is one always-on file directly importing a gated
// package.
type DisableableViolation struct {
	Importer string
	Package  string
	Tag      string
}

// DisableableViolations answers every always-on file that directly imports a
// disableable feature package.
func DisableableViolations(tree, module string, edges Edges, gates map[string]string) []DisableableViolation {
	packages := make([]string, 0, len(gates))
	for pkg := range gates {
		packages = append(packages, pkg)
	}
	sort.Strings(packages)

	var out []DisableableViolation
	for _, pkg := range packages {
		tag := gates[pkg]
		var tb textbuf.Buffer
		importPath := tb.Str(module).Byte('/').Str(pkg).String()
		for _, importer := range edges[importPath] {
			if sameFeatureImporter(importer, tag, gates) || strings.HasSuffix(importer, "_test.go") {
				continue
			}
			if hasAnyPrefix(importer, DisableableNonProdPrefixes[:]) {
				continue
			}
			if !FileRequiresTag(tree, importer, tag) {
				out = append(out, DisableableViolation{Importer: importer, Package: pkg, Tag: tag})
			}
		}
	}
	return out
}

// disableableGate reports an always-on import of a compile-out-able feature. It
// prints nothing when it passes, which is what the script did.
func disableableGate(tree, module string, edges Edges) (GateResult, error) {
	gates, err := LoadFeatureGates(tree)
	if err != nil {
		return GateResult{}, err
	}

	result := GateResult{Name: "disableable-imports"}
	violations := DisableableViolations(tree, module, edges, gates)
	if len(violations) == 0 {
		return result, nil
	}

	var tb textbuf.Buffer
	tb.Str("FAIL: disableable feature(s) imported by always-on code -- breaks compile-out:\n")
	for _, violation := range violations {
		tb.Str("  ").Str(violation.Importer).Str(" imports ").Str(violation.Package).
			Str(" without //go:build ").Str(violation.Tag).Byte('\n')
	}
	tb.Str("  Rule: a compile-out-able feature is reached ONLY via build-tag-gated registration (ai/rules/architecture.md). Gate the importer with //go:build <tag> or route it through the service construction registry.\n")
	result.Diagnosis = tb.String()
	result.Code = 2
	return result, nil
}

// ---------------------------------------------------------------------------
// Lint build-tag drift
// ---------------------------------------------------------------------------

// buildTagsKeyRe and buildTagItemRe read the build-tags list out of the lint
// configuration without a YAML dependency.
var (
	buildTagsKeyRe = regexp.MustCompile(`^\s*build-tags:\s*$`)
	buildTagItemRe = regexp.MustCompile(`^\s*-\s*(\S+)\s*$`)
)

// golangciComment opens a YAML comment, which is stripped before a line is
// read as a list item.
const golangciComment = "#"

// ParseGolangciBuildTags answers the build tags the lint configuration
// declares.
//
// An inline comment is stripped first: YAML allows one and a build tag is a Go
// identifier, so a '#' never belongs to a tag.
func ParseGolangciBuildTags(tree string) (map[string]bool, error) {
	raw, err := os.ReadFile(filepath.Join(tree, Golangci)) //nolint:gosec // the lint configuration of the tree the caller named
	if err != nil {
		return nil, err
	}

	tags := make(map[string]bool)
	capturing := false
	for line := range strings.SplitSeq(string(raw), "\n") {
		line, _, _ = strings.Cut(line, golangciComment)
		if buildTagsKeyRe.MatchString(line) {
			capturing = true
			continue
		}
		if !capturing {
			continue
		}
		if match := buildTagItemRe.FindStringSubmatch(line); match != nil {
			tags[match[1]] = true
			continue
		}
		if strings.TrimSpace(line) == "" {
			continue // a blank or comment-only line inside the list
		}
		break // the next key: the build-tags list is done
	}
	return tags, nil
}

// golangciDriftGate compares the lint configuration's feature tags against the
// manifest's gate tags.
//
// The lint build-tags list is static YAML and cannot read the manifest, so it
// is the one consumer that does not derive automatically. Keeping the two in
// lock-step is what gives every //go:build ze_<feature> file lint coverage.
func golangciDriftGate(tree string) (GateResult, error) {
	gates, err := LoadFeatureGates(tree)
	if err != nil {
		return GateResult{}, err
	}
	manifestTags := make(map[string]bool, len(gates))
	for _, tag := range gates {
		manifestTags[tag] = true
	}

	declared, err := ParseGolangciBuildTags(tree)
	if err != nil {
		return GateResult{}, err
	}
	for _, base := range GolangciBaseTags {
		delete(declared, base)
	}

	result := GateResult{Name: "golangci-drift"}
	missing := missingFrom(manifestTags, declared)
	extra := missingFrom(declared, manifestTags)
	if len(missing) == 0 && len(extra) == 0 {
		return result, nil
	}

	var tb textbuf.Buffer
	tb.Str("FAIL: ").Str(Golangci).Str(" build-tags drifted from ").Str(FeatureGatesManifest).Str(":\n")
	if len(missing) > 0 {
		tb.Str("  add to ").Str(Golangci).Str(" build-tags: ").Str(pyfmt.List(missing)).Byte('\n')
	}
	if len(extra) > 0 {
		tb.Str("  remove from ").Str(Golangci).Str(" build-tags (no such gate): ").Str(pyfmt.List(extra)).Byte('\n')
	}
	tb.Str("  build-tags must be ").Str(pyfmt.List(sortedCopy(GolangciBaseTags[:]))).
		Str(" + every gate tag in ").Str(FeatureGatesManifest).Str(".\n")
	result.Diagnosis = tb.String()
	result.Code = 2
	return result, nil
}

// missingFrom answers the members of left that right does not hold, in order.
func missingFrom(left, right map[string]bool) []string {
	var out []string
	for item := range left {
		if !right[item] {
			out = append(out, item)
		}
	}
	sort.Strings(out)
	return out
}

// sortedCopy answers a sorted copy, so a package-level list is never reordered
// by a message that renders it.
func sortedCopy(items []string) []string {
	out := make([]string, len(items))
	copy(out, items)
	sort.Strings(out)
	return out
}

// sortedManifestKeys and sortedRowKeys answer a map's keys in order.
func sortedManifestKeys(items map[string]ManifestRow) []string {
	keys := make([]string, 0, len(items))
	for key := range items {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedRowKeys(items map[string]Row) []string {
	keys := make([]string, 0, len(items))
	for key := range items {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// Check runs the five gates in the script's order and answers what each said.
func Check(tree string) (CheckReport, error) {
	module, err := ModulePath(tree)
	if err != nil {
		return CheckReport{}, err
	}
	edges, err := CollectEdges(tree, module)
	if err != nil {
		return CheckReport{}, err
	}
	return CheckWith(tree, module, edges)
}

// CheckWith runs the five gates over an audit already collected, which is what
// the selftest needs.
func CheckWith(tree, module string, edges Edges) (CheckReport, error) {
	steps := []func() (GateResult, error){
		func() (GateResult, error) { return enginePlacementGate(tree, module, edges) },
		func() (GateResult, error) { return nonEngineCategoryGate(tree, module, edges) },
		func() (GateResult, error) { return coreDirectionGate(tree, module, edges) },
		func() (GateResult, error) { return disableableGate(tree, module, edges) },
		func() (GateResult, error) { return golangciDriftGate(tree) },
	}

	report := CheckReport{Gates: make([]GateResult, 0, len(steps))}
	for _, step := range steps {
		result, err := step()
		if err != nil {
			return CheckReport{}, err
		}
		report.Gates = append(report.Gates, result)
	}
	report.Failed = FirstFailure(report.Gates)
	return report, nil
}

// FirstFailure answers the FIRST nonzero run code. A later failure gives no
// information about the first failure that a caller must correct.
//
// A separate function makes the rule directly testable. All five checks
// currently answer 0 or 2, so their output cannot expose first-versus-last
// behavior. Untested rules can silently stop holding.
func FirstFailure(gates []GateResult) int {
	for _, gate := range gates {
		if gate.Code != 0 {
			return gate.Code
		}
	}
	return 0
}

// underDir reports whether rel names dir or a descendant.
//
// It avoids `strings.HasPrefix(rel, dir+"/")` for two reasons.
// c_string_concat refuses `+` beside string literals in compiled Go.
// performance.md explains the second reason: concatenation allocates a string
// for each query. This implementation allocates nothing.
func underDir(rel, dir string) bool {
	if rel == dir {
		return true
	}
	return len(rel) > len(dir) && rel[:len(dir)] == dir && rel[len(dir)] == '/'
}
