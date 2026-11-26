// Design: docs/architecture/doctor-and-health-checks.md -- parsed-config readiness checks and diagnostic codes
// Related: dataplane_start_exclude.go -- LoadConfigForStart exclusion of mismatched features
// Related: backend_gate.go -- engine-owned whole-tree ze:backend walk called by evaluateDataPlaneVerdict
// Related: loader.go -- loadConfig door calling refuseDataPlaneMismatches
// Related: plugin_verify.go -- verifyTree door calling refuseDataPlaneMismatches
// Related: declared_schemas.go -- CommandLinePlugins process record merged by candidatePluginSelection
// Related: ../plugin/registry/dataplane_decl.go -- DataPlaneSelector and DataPlaneFeature declarations

package config

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// CodeDoctorDataPlaneMismatch is reported when a configuration mixes features,
// FIB writers, or backend selectors across incompatible data planes, or sets a
// selector leaf to a backend name that no registered backend answers.
const CodeDoctorDataPlaneMismatch = "doctor-dataplane-mismatch"

const (
	// dataPlaneCheckName is the doctor check name registered for data-plane
	// compatibility diagnostics.
	dataPlaneCheckName = "dataplane-compatibility"
	fixPrefixSet       = "set "
	fixAltPrefix       = " (or "
	fixValidateCmd     = "ze config validate"
)

// dataPlaneFinding is one data-plane compatibility mismatch produced by
// evaluateDataPlaneVerdict and rendered as either an operator error (at the
// config doors) or a doctor diagnostic (in ze doctor).
type dataPlaneFinding struct {
	problem     string
	why         string
	fix         string
	hostVerdict bool
}

func (f dataPlaneFinding) operError() error {
	err := opererr.New(f.problem, f.why, f.fix)
	if f.hostVerdict {
		return diagnostic.HostVerdict(err)
	}
	return err
}

func (f dataPlaneFinding) diagnostic() diagnostic.Diagnostic {
	return diagnostic.Diagnostic{
		Code:     CodeDoctorDataPlaneMismatch,
		Severity: diagnostic.SeverityError,
		Message:  f.problem + ": " + f.why,
		Fix:      f.fix,
	}
}

// registeredSelector holds one component's declared DataPlaneSelector together
// with the values resolved from a candidate configuration and schema.
type registeredSelector struct {
	reg        *registry.Registration
	sel        *registry.DataPlaneSelector
	display    string
	backends   []string
	value      string
	defaultVal string
	explicit   bool
	valid      bool
	unmetNeed  bool
}

// pluginSelectionSource records how a DataPlaneInUseSelectedPlugin feature was
// selected in the candidate configuration or command line.
type pluginSelectionSource struct {
	matched   bool
	blockKind string
	blockKey  string
	cliArg    string
}

// activeFeatureMatch is one in-use DataPlaneFeature resolved against a
// candidate configuration tree and plugin selection.
type activeFeatureMatch struct {
	owner      *registry.Registration
	feat       registry.DataPlaneFeature
	configPath string
	secondFix  string
	source     pluginSelectionSource
}

// dataPlaneVerdictEntry pairs one dataPlaneFinding with its start-time
// exclusion plan. Entries with hardRefusal == true are never excluded at start
// (unreadable/unknown selectors, retired backend names, fib dummy conflicts,
// and host-verdict empty backends).
type dataPlaneVerdictEntry struct {
	finding       dataPlaneFinding
	hardRefusal   bool
	removeRoots   []string
	stripPlugin   string
	removePath    string
	disablePaths  []string
	disableLeaves map[string]string
}

// lazyLoweredTree caches Tree.ToPluginMap so predicate-bearing features lower
// the candidate tree at most once per verdict evaluation.
type lazyLoweredTree struct {
	tree    *Tree
	lowered map[string]any
	ready   bool
}

func (l *lazyLoweredTree) get() map[string]any {
	if !l.ready {
		l.lowered = l.tree.ToPluginMap()
		l.ready = true
	}
	return l.lowered
}

// ResolveDataPlaneSelector resolves the backend selector declared by ownerName
// against tree and schema, returning the effective value (written leaf or
// schema default) and the schema default. It fails closed whenever the
// selector, schema, leaf, or required schema default cannot be read.
func ResolveDataPlaneSelector(tree *Tree, schema *Schema, ownerName string) (value, defaultVal string, err error) {
	reg := registry.Lookup(ownerName)
	if reg == nil {
		return "", "", fmt.Errorf("data-plane selector owner %q is not registered", ownerName)
	}
	if reg.DataPlaneSelector == nil {
		return "", "", fmt.Errorf("plugin %q declares no data-plane selector", ownerName)
	}
	value, defaultVal, _, err = resolveSelectorLeaf(tree, schema, ownerName, reg.DataPlaneSelector)
	if err != nil {
		return "", "", err
	}
	return value, defaultVal, nil
}

// ResolvePrimaryDataPlane resolves the primary data-plane selector
// (SelectsDataPlane == true) against tree and schema. It fails closed whenever
// no primary selector is registered or its value/default cannot be read.
func ResolvePrimaryDataPlane(tree *Tree, schema *Schema) (string, error) {
	primary, _ := collectRegisteredSelectors(registry.All())
	if primary == nil {
		return "", errors.New("primary data-plane selector is not registered")
	}
	value, _, _, err := resolveSelectorLeaf(tree, schema, primary.reg.Name, primary.sel)
	if err != nil {
		return "", err
	}
	return value, nil
}

// ResolvePrimaryDataPlaneFromMap resolves the primary data-plane selector
// (SelectsDataPlane == true) from a lowered config map and schema, falling back
// to the selector leaf's schema default when the leaf is absent. It fails
// closed whenever no primary selector is registered or its value/default cannot
// be read.
func ResolvePrimaryDataPlaneFromMap(tree map[string]any, schema *Schema) (string, error) {
	primary, _ := collectRegisteredSelectors(registry.All())
	if primary == nil {
		return "", errors.New("primary data-plane selector is not registered")
	}
	if schema == nil {
		return "", fmt.Errorf("config schema is unavailable while resolving %q selector %q", primary.reg.Name, primary.sel.LeafPath)
	}
	cleanPath := strings.TrimPrefix(primary.sel.LeafPath, PathSep)
	if cleanPath == "" {
		return "", fmt.Errorf("plugin %q declares an empty selector leaf path", primary.reg.Name)
	}
	node, lookupErr := schema.Lookup(cleanPath)
	if lookupErr != nil {
		return "", fmt.Errorf("selector leaf %q not found in schema: %w", primary.sel.LeafPath, lookupErr)
	}
	leaf, ok := node.(*LeafNode)
	if !ok {
		return "", fmt.Errorf("selector path %q is not a leaf in schema", primary.sel.LeafPath)
	}
	if written, found := selectorLeafWrittenInMap(tree, cleanPath); found {
		return written, nil
	}
	if leaf.Default == "" {
		return "", fmt.Errorf("selector leaf %q has no configured value and no schema default", primary.sel.LeafPath)
	}
	return leaf.Default, nil
}

// selectorLeafWrittenInMap reads a slash-separated leaf path from a lowered
// config map.
func selectorLeafWrittenInMap(tree map[string]any, cleanPath string) (string, bool) {
	if len(tree) == 0 {
		return "", false
	}
	parts := SplitPath(cleanPath)
	if len(parts) == 0 {
		return "", false
	}
	cur := tree
	for _, seg := range parts[:len(parts)-1] {
		next, ok := cur[seg].(map[string]any)
		if !ok {
			return "", false
		}
		cur = next
	}
	raw, ok := cur[parts[len(parts)-1]]
	if !ok {
		return "", false
	}
	val, ok := raw.(string)
	if !ok {
		return "", false
	}
	if val == "" {
		return "", false
	}
	return val, true
}

// dataPlaneDoctorCheck builds the doctor registration for data-plane
// compatibility. It runs in DoctorPhaseParsedConfig, ahead of ReadConfig, so a
// loader refusal does not hide the mismatch diagnostics.
func dataPlaneDoctorCheck() diagnostic.DoctorCheck {
	return diagnostic.DoctorCheck{
		Name:         dataPlaneCheckName,
		Phase:        diagnostic.DoctorPhaseParsedConfig,
		Order:        100,
		Component:    doctorComponent,
		Dependencies: []string{"config-tree"},
		Platforms:    []string{diagnostic.DoctorPlatformAny},
		Codes:        []string{CodeDoctorDataPlaneMismatch},
		Check:        checkDataPlaneCompatibility,
	}
}

// checkDataPlaneCompatibility runs the data-plane compatibility verdict over
// the parsed config tree for `ze doctor`.
func checkDataPlaneCompatibility(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	tree, ok := ctx.Tree.(*Tree)
	// refuse-by-default: guards dataPlaneDoctorCheck -- a context holding a non-*Tree is rejected by the doctor runner
	if !ok {
		return nil
	}
	// refuse-by-default: guards dataPlaneDoctorCheck -- missing-config phase supplies a nil tree
	if tree == nil {
		return nil
	}
	schema, err := YANGSchema()
	if err != nil {
		return []diagnostic.Diagnostic{{
			Code:     CodeDoctorDataPlaneMismatch,
			Severity: diagnostic.SeverityError,
			Message:  "data-plane compatibility could not be checked: YANG schema unavailable: " + err.Error(),
			Fix:      fixValidateCmd,
		}}
	}
	selection, err := candidatePluginSelection(tree, nil)
	// refuse-by-default: guards ReadConfig -- when plugin selection fails, ReadConfig and section checks report the refusal
	if err != nil {
		return nil
	}
	return dataPlaneMismatchDiagnostics(tree, schema, selection)
}

// candidatePluginSelection extracts the plugins declared in tree and merges
// the process-wide recorded command-line plugins together with cliPlugins.
// This is the single reader of CommandLinePlugins that decides the verdict's
// plugin selection.
func candidatePluginSelection(tree *Tree, cliPlugins []string) ([]plugin.PluginConfig, error) {
	combined := mergeCommandLinePluginLists(CommandLinePlugins(), cliPlugins)
	return pluginSelectionWithCLI(tree, combined)
}

// pluginSelectionWithCLI extracts the plugins declared in tree and merges them
// with an explicit command-line plugin list without re-reading
// CommandLinePlugins.
func pluginSelectionWithCLI(tree *Tree, cliPlugins []string) ([]plugin.PluginConfig, error) {
	declared, err := ExtractPluginsFromTree(tree)
	if err != nil {
		return nil, err
	}
	merged, err := MergeCliPlugins(declared, cliPlugins)
	if err != nil {
		return nil, fmt.Errorf("resolve plugins: %w", err)
	}
	return merged, nil
}

// mergeCommandLinePluginLists returns the ordered, deduplicated union of
// recorded and caller command-line plugin arguments.
func mergeCommandLinePluginLists(recorded, caller []string) []string {
	if len(recorded) == 0 && len(caller) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(recorded)+len(caller))
	out := make([]string, 0, len(recorded)+len(caller))
	for _, entry := range recorded {
		if seen[entry] {
			continue
		}
		seen[entry] = true
		out = append(out, entry)
	}
	for _, entry := range caller {
		if seen[entry] {
			continue
		}
		seen[entry] = true
		out = append(out, entry)
	}
	return out
}

// refuseDataPlaneMismatches evaluates data-plane compatibility over tree and
// returns the joined operator-error refusals, or nil when no mismatch exists.
func refuseDataPlaneMismatches(tree *Tree, schema *Schema, selection []plugin.PluginConfig) error {
	return findingsToError(evaluateDataPlaneVerdict(tree, schema, selection))
}

// dataPlaneMismatchDiagnostics evaluates data-plane compatibility over tree
// and renders each finding as a doctor diagnostic under
// CodeDoctorDataPlaneMismatch.
func dataPlaneMismatchDiagnostics(tree *Tree, schema *Schema, selection []plugin.PluginConfig) []diagnostic.Diagnostic {
	return findingsToDiagnostics(evaluateDataPlaneVerdict(tree, schema, selection))
}

// findingsToError renders findings as a single operator error or a joined set
// of operator errors.
func findingsToError(findings []dataPlaneFinding) error {
	if len(findings) == 0 {
		return nil
	}
	if len(findings) == 1 {
		return findings[0].operError()
	}
	errs := make([]error, len(findings))
	for i, f := range findings {
		errs[i] = f.operError()
	}
	return errors.Join(errs...)
}

// findingsToDiagnostics renders findings as doctor diagnostics.
func findingsToDiagnostics(findings []dataPlaneFinding) []diagnostic.Diagnostic {
	if len(findings) == 0 {
		return nil
	}
	diags := make([]diagnostic.Diagnostic, len(findings))
	for i, f := range findings {
		diags[i] = f.diagnostic()
	}
	return diags
}

// evaluateDataPlaneVerdict evaluates data-plane compatibility declarations
// from the plugin registry against tree, schema, and the candidate's plugin
// selection. It reads no process record so tests can drive it with explicit
// selections; both config doors (loadConfig and verifyTree) and the
// dataplane-compatibility doctor check share this single evaluator.
func evaluateDataPlaneVerdict(tree *Tree, schema *Schema, selection []plugin.PluginConfig) []dataPlaneFinding {
	return verdictFindings(evaluateDataPlaneEntries(tree, schema, selection))
}

// verdictFindings projects the operator-facing findings out of entries in
// evaluation order.
func verdictFindings(entries []dataPlaneVerdictEntry) []dataPlaneFinding {
	if len(entries) == 0 {
		return nil
	}
	findings := make([]dataPlaneFinding, len(entries))
	for i := range entries {
		findings[i] = entries[i].finding
	}
	return findings
}

// hardVerdictEntries wraps hard-refusal findings so start-time exclusion never
// drops them.
func hardVerdictEntries(findings []dataPlaneFinding) []dataPlaneVerdictEntry {
	if len(findings) == 0 {
		return nil
	}
	out := make([]dataPlaneVerdictEntry, len(findings))
	for i, f := range findings {
		out[i] = dataPlaneVerdictEntry{
			finding:     f,
			hardRefusal: true,
		}
	}
	return out
}

// evaluateDataPlaneEntries is the single shared evaluator behind strict
// refusal (evaluateDataPlaneVerdict) and start-time exclusion
// (excludeDataPlaneMismatches). Every finding is paired with either
// hardRefusal=true or the exact in-memory exclusion action that leaves the
// mismatched feature off.
func evaluateDataPlaneEntries(tree *Tree, schema *Schema, selection []plugin.PluginConfig) []dataPlaneVerdictEntry {
	// refuse-by-default: guards refuseDataPlaneMismatches -- a nil tree holds no configuration to evaluate
	if tree == nil {
		return nil
	}
	regs := registry.All()
	primary, secondaries := collectRegisteredSelectors(regs)
	// refuse-by-default: guards refuseDataPlaneMismatches -- no primary DataPlaneSelector is registered in this build
	if primary == nil {
		return nil
	}
	if schema == nil {
		return []dataPlaneVerdictEntry{{
			finding: dataPlaneFinding{
				problem: "data-plane compatibility could not be evaluated: config schema is unavailable",
				why:     "ze requires the YANG schema to read selector defaults and judge data-plane compatibility",
				fix:     fixValidateCmd,
			},
			hardRefusal: true,
		}}
	}

	presentPaths := presentPathSet(tree)
	lowered := &lazyLoweredTree{tree: tree}
	standalone := registry.StandaloneDataPlaneFeatures()
	producersBySelector := collectFollowsSelectorMatches(tree, regs, standalone, presentPaths, selection, lowered)

	var entries []dataPlaneVerdictEntry
	entries = append(entries, hardVerdictEntries(resolveSelectorEntry(tree, schema, regs, primary))...)

	for _, sec := range secondaries {
		_, written := selectorLeafWritten(tree, sec.sel.LeafPath)
		if !written && !selectorHasPresentRoot(sec, presentPaths) && len(producersBySelector[sec.reg.Name]) == 0 {
			continue
		}
		entries = append(entries, hardVerdictEntries(resolveSelectorEntry(tree, schema, regs, sec))...)
	}

	// Stop before plane-mismatch comparisons when the primary selector is
	// unreadable, unknown, or covered by an unmet ConfigNeed: comparing
	// features against a nonexistent plane would only produce noise.
	if !primary.valid {
		return entries
	}

	entries = append(entries, evaluateSecondarySelectors(primary, secondaries, presentPaths, producersBySelector)...)
	entries = append(entries, evaluateFixedPlaneFeatures(tree, regs, standalone, primary, presentPaths, selection, lowered)...)
	entries = append(entries, evaluateFIBWriters(regs, primary, presentPaths)...)
	entries = append(entries, hardVerdictEntries(evaluateNoPlaneFIBChoice(tree, regs, presentPaths, selection))...)
	entries = append(entries, evaluateBackendFeatureWalkEntries(lowered.get(), schema, primary, secondaries)...)
	return entries
}

// evaluateBackendFeatureWalk runs the single engine-owned ze:backend walk over
// the candidate tree as written and turns every mismatch into a finding shared
// by both config doors and ze doctor (AC-9, AC-10).
func evaluateBackendFeatureWalk(
	treeMap map[string]any,
	schema *Schema,
	primary *registeredSelector,
	secondaries []*registeredSelector,
) []dataPlaneFinding {
	return verdictFindings(evaluateBackendFeatureWalkEntries(treeMap, schema, primary, secondaries))
}

// evaluateBackendFeatureWalkEntries runs the engine-owned ze:backend walk and
// pairs each non-hostVerdict mismatch with its concrete Tree.RemovePath target.
func evaluateBackendFeatureWalkEntries(
	treeMap map[string]any,
	schema *Schema,
	primary *registeredSelector,
	secondaries []*registeredSelector,
) []dataPlaneVerdictEntry {
	toWalkSelector := func(s *registeredSelector) backendWalkSelector {
		return backendWalkSelector{
			leafPath:   s.sel.LeafPath,
			display:    s.display,
			roots:      s.sel.Roots,
			active:     s.value,
			defaultVal: s.defaultVal,
			valid:      s.valid,
		}
	}
	walkSecondaries := make([]backendWalkSelector, len(secondaries))
	for i, sec := range secondaries {
		walkSecondaries[i] = toWalkSelector(sec)
	}
	raw := walkBackendFeaturesTree(treeMap, schema, toWalkSelector(primary), walkSecondaries)
	if len(raw) == 0 {
		return nil
	}
	entries := make([]dataPlaneVerdictEntry, 0, len(raw))
	for i := range raw {
		f := &raw[i]
		if f.hostVerdict {
			fix := fixValidateCmd
			if f.selector.defaultVal != "" {
				fix = fixPrefixSet + f.selector.display + " " + f.selector.defaultVal
			}
			entries = append(entries, dataPlaneVerdictEntry{
				finding: dataPlaneFinding{
					problem:     "backend feature gate: no backend configured at " + f.selector.leafPath,
					why:         f.selector.display + " has no configured backend and no default on this host",
					fix:         fix,
					hostVerdict: true,
				},
				hardRefusal: true,
			})
			continue
		}
		var why textbuf.Buffer
		why.Str(`this node declares ze:backend "`).Str(textbuf.Join(f.mismatch.supporting, " ")).
			Str(`", and `).Str(f.selector.display).Str(` is "`).Str(f.mismatch.active).Byte('"')
		fix := fixValidateCmd
		if len(f.mismatch.supporting) > 0 {
			fix = fixPrefixSet + f.selector.display + " " + f.mismatch.supporting[0]
		}
		entries = append(entries, dataPlaneVerdictEntry{
			finding: dataPlaneFinding{
				problem: formatBackendError(f.mismatch.path, f.mismatch.active, f.mismatch.supporting).Error(),
				why:     why.String(),
				fix:     fix,
			},
			removePath: backendMismatchRemovalPath(f.mismatch.steps),
		})
	}
	return entries
}

// collectRegisteredSelectors partitions all registered DataPlaneSelectors into
// the single primary selector (SelectsDataPlane == true) and all secondary
// selectors, preserving registry.All()'s sorted owner order.
func collectRegisteredSelectors(regs []*registry.Registration) (*registeredSelector, []*registeredSelector) {
	var primary *registeredSelector
	var secondaries []*registeredSelector
	for _, reg := range regs {
		if reg.DataPlaneSelector == nil {
			continue
		}
		entry := &registeredSelector{
			reg:     reg,
			sel:     reg.DataPlaneSelector,
			display: configPathDisplay(reg.DataPlaneSelector.LeafPath),
		}
		if reg.DataPlaneSelector.SelectsDataPlane {
			primary = entry
			continue
		}
		secondaries = append(secondaries, entry)
	}
	return primary, secondaries
}

// resolveSelectorEntry resolves entry against tree and schema, validates the
// value against its RegisteredBackends, and returns any unreadable or
// unknown-backend finding (deferring values covered by an unmet ConfigNeed to
// refuseAbsentOptionalProviders).
func resolveSelectorEntry(tree *Tree, schema *Schema, regs []*registry.Registration, entry *registeredSelector) []dataPlaneFinding {
	val, defVal, explicit, err := resolveSelectorLeaf(tree, schema, entry.reg.Name, entry.sel)
	if err != nil {
		return []dataPlaneFinding{{
			problem: "data-plane selector " + entry.display + " could not be read",
			why:     err.Error(),
			fix:     fixValidateCmd,
		}}
	}
	entry.value = val
	entry.defaultVal = defVal
	entry.explicit = explicit
	entry.backends = sortedBackends(entry.sel.RegisteredBackends())
	if slices.Contains(entry.backends, val) {
		entry.valid = true
		return nil
	}
	if coveredByUnmetConfigNeed(regs, entry.sel.LeafPath, val) {
		entry.unmetNeed = true
		return nil
	}
	return []dataPlaneFinding{unknownBackendFinding(entry)}
}

// unknownBackendFinding formats the AC-19 refusal for a selector leaf whose
// value names no backend registered in this build.
func unknownBackendFinding(entry *registeredSelector) dataPlaneFinding {
	var problem textbuf.Buffer
	problem.Str(entry.display).Str(` "`).Str(entry.value).Str(`" names no backend this build registers`)

	var why textbuf.Buffer
	if len(entry.backends) > 0 {
		why.Str("the backends are ").Str(textbuf.Join(entry.backends, ", "))
	} else {
		why.Str("no backend is registered for ").Str(entry.display).Str(" in this build")
	}

	fix := fixValidateCmd
	if entry.defaultVal != "" {
		fix = fixPrefixSet + entry.display + " " + entry.defaultVal
	} else if len(entry.backends) > 0 {
		fix = fixPrefixSet + entry.display + " " + entry.backends[0]
	}

	return dataPlaneFinding{
		problem: problem.String(),
		why:     why.String(),
		fix:     fix,
	}
}

// evaluateSecondarySelectors compares every active, valid secondary selector
// and its active FollowsSelector producers against primary.
func evaluateSecondarySelectors(
	primary *registeredSelector,
	secondaries []*registeredSelector,
	presentPaths map[string]bool,
	producersBySelector map[string][]activeFeatureMatch,
) []dataPlaneVerdictEntry {
	var entries []dataPlaneVerdictEntry
	for _, sec := range secondaries {
		if !sec.valid {
			continue
		}
		if sec.value == primary.value {
			continue
		}
		presentRoot, hasRoot := firstPresentSelectorRoot(sec, presentPaths)
		if hasRoot || sec.explicit {
			roots := append(slices.Clone(sec.sel.Roots), sec.sel.LeafPath)
			entries = append(entries, dataPlaneVerdictEntry{
				finding:     secondarySelectorMismatchFinding(primary, sec, presentRoot),
				removeRoots: roots,
				stripPlugin: sec.reg.Name,
			})
		}
		matches := producersBySelector[sec.reg.Name]
		for i := range matches {
			entries = append(entries, featureMatchVerdictEntry(
				followsSelectorProducerFinding(primary, sec, matches[i]),
				matches[i],
			))
		}
	}
	return entries
}

// featureMatchVerdictEntry pairs a feature mismatch finding with the in-memory
// exclusion action derived from match.feat.InUseKind.
func featureMatchVerdictEntry(finding dataPlaneFinding, match activeFeatureMatch) dataPlaneVerdictEntry {
	entry := dataPlaneVerdictEntry{finding: finding}
	switch match.feat.InUseKind {
	case registry.DataPlaneInUseConfigRoot:
		if match.owner != nil {
			entry.removeRoots = slices.Clone(match.owner.ConfigRoots)
			entry.stripPlugin = match.owner.Name
		}
	case registry.DataPlaneInUseSelectedPlugin:
		if match.source.matched && match.source.blockKind != "" {
			entry.removePath = busContainerName + PathSep + match.source.blockKind + "[" + match.source.blockKey + "]"
		}
		if match.owner != nil {
			entry.stripPlugin = match.owner.Name
		}
	case registry.DataPlaneInUsePredicate:
		entry.disablePaths = slices.Clone(match.feat.DisablePaths)
		entry.disableLeaves = match.feat.DisableLeaves
	case registry.DataPlaneInUseUnspecified:
		// Never returned by evaluateFeatureInUse.
	default:
		// Open set: DataPlaneInUseKind comes from another package.
	}
	return entry
}

// secondarySelectorMismatchFinding builds the AC-3 / AC-4 refusal when a
// secondary selector's backend differs from the primary data plane.
func secondarySelectorMismatchFinding(primary, sec *registeredSelector, presentRoot string) dataPlaneFinding {
	subject := sec.display
	if presentRoot != "" {
		subject = configPathDisplay(presentRoot)
	}

	var problem textbuf.Buffer
	if sec.explicit {
		problem.Str(sec.display).Str(` is "`).Str(sec.value).Str(`", and `).Str(primary.display).Str(` is "`).Str(primary.value).Byte('"')
	} else {
		problem.Str(subject).Str(" uses ").Str(sec.display).Str(` "`).Str(sec.value).Str(`", and `).Str(primary.display).Str(` is "`).Str(primary.value).Byte('"')
	}

	var why textbuf.Buffer
	why.Str(primary.display).Str(` "`).Str(primary.value).Str(`" forwards the packets, so `).
		Str(subject).Str(` on "`).Str(sec.value).Str(`" would act on a different data plane`)

	var fix textbuf.Buffer
	fix.Str(fixPrefixSet).Str(sec.display).Byte(' ').Str(primary.value)
	if sec.explicit {
		fix.Str(fixAltPrefix).Str(fixPrefixSet).Str(primary.display).Byte(' ').Str(sec.value).Byte(')')
	}

	return dataPlaneFinding{
		problem: problem.String(),
		why:     why.String(),
		fix:     fix.String(),
	}
}

// followsSelectorProducerFinding builds the AC-3 refusal when an in-use
// feature following sec mismatches primary.
func followsSelectorProducerFinding(primary, sec *registeredSelector, match activeFeatureMatch) dataPlaneFinding {
	subject := featureSubject(match)

	var problem textbuf.Buffer
	problem.Str(subject).Str(" uses ").Str(sec.display).Str(` "`).Str(sec.value).
		Str(`", and `).Str(primary.display).Str(` is "`).Str(primary.value).Byte('"')

	var why textbuf.Buffer
	if match.feat.Why != "" {
		why.Str(match.feat.Why).Str("; ")
	}
	why.Str(primary.display).Str(` "`).Str(primary.value).Str(`" forwards the packets on a different data plane`)

	var fix textbuf.Buffer
	fix.Str(fixPrefixSet).Str(sec.display).Byte(' ').Str(primary.value)
	if alt := featureSecondFix(match); alt != "" {
		fix.Str(fixAltPrefix).Str(alt).Byte(')')
	}

	return dataPlaneFinding{
		problem: problem.String(),
		why:     why.String(),
		fix:     fix.String(),
	}
}

// evaluateFixedPlaneFeatures checks every fixed-plane DataPlaneFeature across
// registered plugins (skipping HostSideDataPlane registrations) and standalone
// subsystems.
func evaluateFixedPlaneFeatures(
	tree *Tree,
	regs []*registry.Registration,
	standalone []registry.DataPlaneFeature,
	primary *registeredSelector,
	presentPaths map[string]bool,
	selection []plugin.PluginConfig,
	lowered *lazyLoweredTree,
) []dataPlaneVerdictEntry {
	var entries []dataPlaneVerdictEntry
	for _, reg := range regs {
		if reg.HostSideDataPlane {
			continue
		}
		for _, feat := range reg.DataPlaneFeatures {
			if len(feat.Planes) == 0 {
				continue
			}
			match, inUse := evaluateFeatureInUse(tree, reg, feat, presentPaths, selection, lowered)
			if !inUse {
				continue
			}
			if slices.Contains(feat.Planes, primary.value) {
				continue
			}
			entries = append(entries, featureMatchVerdictEntry(fixedPlaneFeatureFinding(primary, match), match))
		}
	}
	for _, feat := range standalone {
		if len(feat.Planes) == 0 {
			continue
		}
		match, inUse := evaluateFeatureInUse(tree, nil, feat, presentPaths, selection, lowered)
		if !inUse {
			continue
		}
		if slices.Contains(feat.Planes, primary.value) {
			continue
		}
		entries = append(entries, featureMatchVerdictEntry(fixedPlaneFeatureFinding(primary, match), match))
	}
	return entries
}

// fixedPlaneFeatureFinding builds the AC-5 / AC-15 refusal for an in-use
// feature whose declared Planes exclude primary.value.
func fixedPlaneFeatureFinding(primary *registeredSelector, match activeFeatureMatch) dataPlaneFinding {
	subject := featureSubject(match)

	var problem textbuf.Buffer
	problem.Str(subject).Str(" cannot run with ").Str(primary.display).Str(` "`).Str(primary.value).Byte('"')

	var why textbuf.Buffer
	if match.feat.Why == "" {
		why.Str(subject).Str(" supports only ").Str(textbuf.Join(match.feat.Planes, ", ")).
			Str(", while ").Str(primary.display).Str(` is "`).Str(primary.value).Byte('"')
	} else {
		why.Str(match.feat.Why).Str(", while ").Str(primary.display).Str(` is "`).Str(primary.value).Byte('"')
	}

	var fix textbuf.Buffer
	fix.Str(fixPrefixSet).Str(primary.display).Byte(' ').Str(match.feat.Planes[0])
	if alt := featureSecondFix(match); alt != "" {
		fix.Str(fixAltPrefix).Str(alt).Byte(')')
	}

	return dataPlaneFinding{
		problem: problem.String(),
		why:     why.String(),
		fix:     fix.String(),
	}
}

// evaluateFIBWriters enforces AC-1 (unmarked FIB writer of another plane) and
// AC-14 (host-side-marked FIB writer of another plane alone without a writer
// of the selected plane).
func evaluateFIBWriters(
	regs []*registry.Registration,
	primary *registeredSelector,
	presentPaths map[string]bool,
) []dataPlaneVerdictEntry {
	var selectedPlaneFIBRoot string
	hasSelectedPlaneFIB := false

	type configuredFIB struct {
		reg       *registry.Registration
		rootLabel string
	}
	var configured []configuredFIB

	for _, reg := range regs {
		if !reg.ProgramsFIB {
			continue
		}
		if reg.DataPlane == "" {
			continue
		}
		if reg.DataPlane == primary.value && selectedPlaneFIBRoot == "" && len(reg.ConfigRoots) > 0 {
			selectedPlaneFIBRoot = configPathDisplay(reg.ConfigRoots[0])
		}
		root, present := firstPresentRoot(reg.ConfigRoots, presentPaths)
		if !present {
			continue
		}
		if reg.DataPlane == primary.value {
			hasSelectedPlaneFIB = true
		}
		configured = append(configured, configuredFIB{
			reg:       reg,
			rootLabel: configPathDisplay(root),
		})
	}

	var entries []dataPlaneVerdictEntry
	for _, fib := range configured {
		if fib.reg.DataPlane == primary.value {
			continue
		}
		if !fib.reg.HostSideDataPlane {
			var problem textbuf.Buffer
			problem.Str(fib.rootLabel).Str(` programs "`).Str(fib.reg.DataPlane).
				Str(`", and `).Str(primary.display).Str(` is "`).Str(primary.value).Byte('"')
			var why textbuf.Buffer
			why.Str(`its routes target "`).Str(fib.reg.DataPlane).
				Str(`" while `).Str(primary.display).Str(` "`).Str(primary.value).Str(`" forwards the packets`)
			entries = append(entries, dataPlaneVerdictEntry{
				finding: dataPlaneFinding{
					problem: problem.String(),
					why:     why.String(),
					fix:     fixPrefixSet + primary.display + " " + fib.reg.DataPlane,
				},
				removeRoots: slices.Clone(fib.reg.ConfigRoots),
				stripPlugin: fib.reg.Name,
			})
			continue
		}
		if hasSelectedPlaneFIB {
			continue
		}
		var problem textbuf.Buffer
		problem.Str(fib.rootLabel).Str(` programs "`).Str(fib.reg.DataPlane).
			Str(`" only, and `).Str(primary.display).Str(` is "`).Str(primary.value).Byte('"')
		if selectedPlaneFIBRoot != "" {
			var why textbuf.Buffer
			why.Str(primary.display).Str(` "`).Str(primary.value).
				Str(`" forwards the packets, and with no `).Str(selectedPlaneFIBRoot).Str(" block its FIB gets no route")
			entries = append(entries, dataPlaneVerdictEntry{
				finding: dataPlaneFinding{
					problem: problem.String(),
					why:     why.String(),
					fix:     fixPrefixSet + selectedPlaneFIBRoot,
				},
				removeRoots: slices.Clone(fib.reg.ConfigRoots),
				stripPlugin: fib.reg.Name,
			})
			continue
		}
		var why textbuf.Buffer
		why.Str(primary.display).Str(` "`).Str(primary.value).
			Str(`" forwards the packets, and no FIB writer for "`).Str(primary.value).Str(`" is configured`)
		entries = append(entries, dataPlaneVerdictEntry{
			finding: dataPlaneFinding{
				problem: problem.String(),
				why:     why.String(),
				fix:     fixPrefixSet + primary.display + " " + fib.reg.DataPlane,
			},
			removeRoots: slices.Clone(fib.reg.ConfigRoots),
			stripPlugin: fib.reg.Name,
		})
	}
	return entries
}

// fibUseKind describes how a candidate configuration or command line selected
// a ProgramsFIB plugin.
type fibUseKind uint8

const (
	fibUseNone fibUseKind = iota
	fibUseConfigRoot
	fibUseBusEndpoint
	fibUseCLIPlugin
)

// fibPluginUse records one active ProgramsFIB plugin and how it was selected
// in the candidate configuration.
type fibPluginUse struct {
	reg         *registry.Registration
	kind        fibUseKind
	rootLabel   string
	endpointKey string
	cliArg      string
}

// evaluateNoPlaneFIBChoice refuses any candidate that selects a ProgramsFIB
// plugin marked NoDataPlaneChoice beside any other ProgramsFIB plugin in use
// (by config root, bus endpoint, or --plugin). Every name, root, and plane in
// the emitted finding is read from the registrations and selection.
func evaluateNoPlaneFIBChoice(
	tree *Tree,
	regs []*registry.Registration,
	presentPaths map[string]bool,
	selection []plugin.PluginConfig,
) []dataPlaneFinding {
	endpoints := activeBusEndpoints(tree)
	var inUse []fibPluginUse
	hasChoice := false
	for _, reg := range regs {
		if !reg.ProgramsFIB {
			continue
		}
		use := resolveFIBPluginUse(reg, presentPaths, endpoints, selection)
		if use.kind == fibUseNone {
			continue
		}
		if reg.NoDataPlaneChoice {
			hasChoice = true
		}
		inUse = append(inUse, use)
	}
	if !hasChoice {
		return nil
	}
	if len(inUse) < 2 {
		return nil
	}

	var findings []dataPlaneFinding
	for i, choice := range inUse {
		if !choice.reg.NoDataPlaneChoice {
			continue
		}
		for j, other := range inUse {
			if i == j {
				continue
			}
			if other.reg.NoDataPlaneChoice && j < i {
				continue
			}
			findings = append(findings, noPlaneFIBChoiceFinding(choice, other))
		}
	}
	return findings
}

// activeBusEndpoints returns the active `bus > endpoint <key>` entries in tree.
func activeBusEndpoints(tree *Tree) map[string]*Tree {
	if tree == nil {
		return nil
	}
	busContainer := tree.GetContainer(busContainerName)
	if busContainer == nil {
		return nil
	}
	if busContainer.IsInactive() {
		return nil
	}
	raw := busContainer.GetList(busListEndpoint)
	if len(raw) == 0 {
		return nil
	}
	active := make(map[string]*Tree, len(raw))
	for key, ep := range raw {
		if ep != nil && ep.IsInactive() {
			continue
		}
		active[key] = ep
	}
	return active
}

// resolveFIBPluginUse determines whether reg is in use by a config root in
// presentPaths, by an explicit bus endpoint in endpoints, or by a --plugin
// selector in selection. External endpoints never select a compiled registration.
func resolveFIBPluginUse(
	reg *registry.Registration,
	presentPaths map[string]bool,
	endpoints map[string]*Tree,
	selection []plugin.PluginConfig,
) fibPluginUse {
	if root, present := firstPresentRoot(reg.ConfigRoots, presentPaths); present {
		return fibPluginUse{
			reg:       reg,
			kind:      fibUseConfigRoot,
			rootLabel: configPathDisplay(root),
		}
	}

	var cliFallback fibPluginUse
	for _, pc := range selection {
		if pc.RunsExternalProgram() {
			continue
		}
		if pc.CompiledIdentity() != reg.Name {
			continue
		}
		if _, isEndpoint := endpoints[pc.Name]; isEndpoint {
			return fibPluginUse{
				reg:         reg,
				kind:        fibUseBusEndpoint,
				endpointKey: pc.Name,
			}
		}
		if cliFallback.kind == fibUseNone {
			cliFallback = fibPluginUse{
				reg:    reg,
				kind:   fibUseCLIPlugin,
				cliArg: cliPluginArgument(pc),
			}
		}
	}
	return cliFallback
}

// noPlaneFIBChoiceFinding builds the three-line refusal for a NoDataPlaneChoice
// FIB plugin configured beside another ProgramsFIB plugin.
func noPlaneFIBChoiceFinding(choice, other fibPluginUse) dataPlaneFinding {
	choiceDisplay := fibUseDisplayName(choice)
	otherDisplay := fibUseDisplayName(other)

	var problem textbuf.Buffer
	switch {
	case choice.kind == fibUseConfigRoot && other.kind == fibUseConfigRoot:
		problem.Str(choice.rootLabel).Str(" is configured beside ").Str(other.rootLabel)
	case choice.kind == fibUseConfigRoot:
		problem.Str(choice.rootLabel).Str(" is configured and the plugin ").Str(other.reg.Name).Str(" is selected")
	case other.kind == fibUseConfigRoot:
		problem.Str("the plugin ").Str(choice.reg.Name).Str(" is selected beside ").Str(other.rootLabel)
	default:
		problem.Str("the plugin ").Str(choice.reg.Name).Str(" is selected beside the plugin ").Str(other.reg.Name)
	}

	var why textbuf.Buffer
	why.Str(choiceDisplay).Str(" is the choice to program no route, and ").Str(otherDisplay)
	if other.reg.DataPlane == "" {
		why.Str(" is another FIB plugin")
	} else {
		why.Str(" programs the ").Str(other.reg.DataPlane).Str(" data plane")
	}

	var fix textbuf.Buffer
	fix.Str(fibUseDeleteAction(choice)).Str(", or ").Str(fibUseDeleteAction(other))

	return dataPlaneFinding{
		problem: problem.String(),
		why:     why.String(),
		fix:     fix.String(),
	}
}

// fibUseDisplayName returns the operator-facing config path for u when its
// registration owns a ConfigRoot, falling back to its plugin name.
func fibUseDisplayName(u fibPluginUse) string {
	if u.kind == fibUseConfigRoot {
		return u.rootLabel
	}
	if len(u.reg.ConfigRoots) > 0 {
		return configPathDisplay(u.reg.ConfigRoots[0])
	}
	return u.reg.Name
}

// fibUseDeleteAction returns the operator action that removes u from the
// candidate configuration or startup flags.
func fibUseDeleteAction(u fibPluginUse) string {
	switch u.kind {
	case fibUseConfigRoot:
		return "delete " + u.rootLabel
	case fibUseBusEndpoint:
		return "delete bus endpoint " + u.endpointKey
	case fibUseCLIPlugin:
		return "start ze without --plugin " + u.cliArg
	case fibUseNone:
		panic("BUG: fibUseDeleteAction called for unused FIB plugin " + u.reg.Name)
	default:
		panic("BUG: fibUseDeleteAction called with unknown fibUseKind")
	}
}

// collectFollowsSelectorMatches evaluates all FollowsSelector features once
// and groups active matches by their target selector owner name.
func collectFollowsSelectorMatches(
	tree *Tree,
	regs []*registry.Registration,
	standalone []registry.DataPlaneFeature,
	presentPaths map[string]bool,
	selection []plugin.PluginConfig,
	lowered *lazyLoweredTree,
) map[string][]activeFeatureMatch {
	bySelector := make(map[string][]activeFeatureMatch)
	for _, reg := range regs {
		if reg.HostSideDataPlane {
			continue
		}
		for _, feat := range reg.DataPlaneFeatures {
			if feat.FollowsSelector == "" {
				continue
			}
			match, inUse := evaluateFeatureInUse(tree, reg, feat, presentPaths, selection, lowered)
			if !inUse {
				continue
			}
			bySelector[feat.FollowsSelector] = append(bySelector[feat.FollowsSelector], match)
		}
	}
	for _, feat := range standalone {
		if feat.FollowsSelector == "" {
			continue
		}
		match, inUse := evaluateFeatureInUse(tree, nil, feat, presentPaths, selection, lowered)
		if !inUse {
			continue
		}
		bySelector[feat.FollowsSelector] = append(bySelector[feat.FollowsSelector], match)
	}
	return bySelector
}

// evaluateFeatureInUse determines whether feat is active in tree / selection
// according to feat.InUseKind.
func evaluateFeatureInUse(
	tree *Tree,
	owner *registry.Registration,
	feat registry.DataPlaneFeature,
	presentPaths map[string]bool,
	selection []plugin.PluginConfig,
	lowered *lazyLoweredTree,
) (activeFeatureMatch, bool) {
	switch feat.InUseKind {
	case registry.DataPlaneInUseConfigRoot:
		if owner == nil {
			return activeFeatureMatch{}, false
		}
		if _, present := firstPresentRoot(owner.ConfigRoots, presentPaths); !present {
			return activeFeatureMatch{}, false
		}
		return activeFeatureMatch{
			owner:      owner,
			feat:       feat,
			configPath: configPathDisplay(feat.ConfigPath),
		}, true
	case registry.DataPlaneInUsePredicate:
		if feat.InUse == nil {
			return activeFeatureMatch{}, false
		}
		use := feat.InUse(lowered.get())
		if !use.InUse {
			return activeFeatureMatch{}, false
		}
		cfgPath := feat.ConfigPath
		if use.ConfigPath != "" {
			cfgPath = use.ConfigPath
		}
		return activeFeatureMatch{
			owner:      owner,
			feat:       feat,
			configPath: configPathDisplay(cfgPath),
			secondFix:  use.SecondFix,
		}, true
	case registry.DataPlaneInUseSelectedPlugin:
		if owner == nil {
			return activeFeatureMatch{}, false
		}
		src := findSelectedPluginSource(tree, selection, owner.Name)
		if !src.matched {
			return activeFeatureMatch{}, false
		}
		return activeFeatureMatch{
			owner:      owner,
			feat:       feat,
			configPath: configPathDisplay(feat.ConfigPath),
			source:     src,
		}, true
	case registry.DataPlaneInUseUnspecified:
		return activeFeatureMatch{}, false
	default:
		// Open set: DataPlaneInUseKind comes from another package.
		return activeFeatureMatch{}, false
	}
}

// featureSubject formats the operator-facing subject naming an active feature.
func featureSubject(match activeFeatureMatch) string {
	if match.source.matched {
		if match.source.blockKind != "" {
			return busContainerName + " " + match.source.blockKind + " " + match.source.blockKey
		}
		return "--plugin " + match.source.cliArg
	}
	return match.configPath
}

// featureSecondFix returns an alternative fix for an active feature, if any.
func featureSecondFix(match activeFeatureMatch) string {
	if match.secondFix != "" {
		return match.secondFix
	}
	if !match.source.matched {
		return ""
	}
	if match.source.blockKind != "" {
		return "delete " + busContainerName + " " + match.source.blockKind + " " + match.source.blockKey
	}
	return "start without --plugin " + match.source.cliArg
}

// findSelectedPluginSource reports whether selection selects the registered
// plugin named regName, and identifies whether it came from a `bus endpoint
// <key>` block in tree or from a `--plugin <arg>` flag.
func findSelectedPluginSource(tree *Tree, selection []plugin.PluginConfig, regName string) pluginSelectionSource {
	var cliFallback pluginSelectionSource
	endpoints := activeBusEndpoints(tree)

	for _, pc := range selection {
		if !selectedPluginMatches(pc, regName) {
			continue
		}
		if _, ok := endpoints[pc.Name]; ok {
			return pluginSelectionSource{
				matched:   true,
				blockKind: busListEndpoint,
				blockKey:  pc.Name,
			}
		}
		if !cliFallback.matched {
			cliFallback = pluginSelectionSource{
				matched: true,
				cliArg:  cliPluginArgument(pc),
			}
		}
	}
	return cliFallback
}

// selectedPluginMatches reports whether pc selects the registered plugin named
// regName. A genuinely external binary (`external <name> { run "/bin/foo"; }`)
// that happens to reuse a registered plugin's name as its block key is not
// treated as selecting the compiled-in registration.
func selectedPluginMatches(pc plugin.PluginConfig, regName string) bool {
	if pc.RunsExternalProgram() {
		resolved, err := plugin.ResolvePlugin(pc.Run)
		// refuse-by-default: guards findSelectedPluginSource -- an unparseable external command line cannot select a compiled-in registration
		if err != nil {
			return false
		}
		return resolved.Name == regName
	}
	return plugin.RegistryName(pc) == regName
}

// cliPluginArgument formats the --plugin argument represented by a CLI-merged
// PluginConfig entry.
func cliPluginArgument(pc plugin.PluginConfig) string {
	if pc.Internal {
		return "ze." + pc.Name
	}
	if pc.Run != "" {
		return pc.Run
	}
	return pc.Name
}

// resolveSelectorLeaf reads the selector leaf from tree (if explicitly written)
// or falls back to the leaf's YANG default from schema. It fails closed if the
// schema, leaf path, leaf node, or required default is unavailable.
func resolveSelectorLeaf(
	tree *Tree,
	schema *Schema,
	ownerName string,
	sel *registry.DataPlaneSelector,
) (value, defaultVal string, explicit bool, err error) {
	if schema == nil {
		return "", "", false, fmt.Errorf("config schema is unavailable while resolving %q selector %q", ownerName, sel.LeafPath)
	}
	cleanPath := strings.TrimPrefix(sel.LeafPath, PathSep)
	if cleanPath == "" {
		return "", "", false, fmt.Errorf("plugin %q declares an empty selector leaf path", ownerName)
	}
	node, lookupErr := schema.Lookup(cleanPath)
	if lookupErr != nil {
		return "", "", false, fmt.Errorf("selector leaf %q not found in schema: %w", sel.LeafPath, lookupErr)
	}
	leaf, ok := node.(*LeafNode)
	if !ok {
		return "", "", false, fmt.Errorf("selector path %q is not a leaf in schema", sel.LeafPath)
	}
	defaultVal = leaf.Default
	if written, found := selectorLeafWritten(tree, sel.LeafPath); found {
		return written, defaultVal, true, nil
	}
	if defaultVal == "" {
		return "", "", false, fmt.Errorf("selector leaf %q has no configured value and no schema default", sel.LeafPath)
	}
	return defaultVal, defaultVal, false, nil
}

// selectorLeafWritten reads the written value of leafPath from tree, ignoring
// inactive containers and inactive leaves.
func selectorLeafWritten(tree *Tree, leafPath string) (string, bool) {
	if tree == nil {
		return "", false
	}
	cleanPath := strings.TrimPrefix(leafPath, PathSep)
	parts := SplitPath(cleanPath)
	if len(parts) == 0 {
		return "", false
	}
	cur := tree
	for _, seg := range parts[:len(parts)-1] {
		if seg == "" {
			return "", false
		}
		next := cur.GetContainer(seg)
		if next == nil {
			return "", false
		}
		if next.IsInactive() {
			return "", false
		}
		cur = next
	}
	leafName := parts[len(parts)-1]
	if cur.IsLeafInactive(leafName) {
		return "", false
	}
	val, ok := cur.Get(leafName)
	if !ok {
		return "", false
	}
	if val == "" {
		return "", false
	}
	return val, true
}

// coveredByUnmetConfigNeed reports whether setting leafPath to value matches a
// registered ConfigNeed whose providers are all absent from this build. When
// true, refuseAbsentOptionalProviders owns refusing that setting with the
// build-tag guidance (U-4).
func coveredByUnmetConfigNeed(regs []*registry.Registration, leafPath, value string) bool {
	parts := SplitPath(strings.TrimPrefix(leafPath, PathSep))
	for _, reg := range regs {
		for _, need := range reg.ConfigNeeds {
			if need.Value != value {
				continue
			}
			if !slices.Equal(need.Path, parts) {
				continue
			}
			if !anyProviderRegistered(need.Providers) {
				return true
			}
		}
	}
	return false
}

// anyProviderRegistered reports whether at least one provider name is
// registered in the plugin registry.
func anyProviderRegistered(providers []string) bool {
	for _, name := range providers {
		if registry.Lookup(name) != nil {
			return true
		}
	}
	return false
}

// presentPathSet returns a lookup set of all container and non-empty list
// paths present in tree.
func presentPathSet(tree *Tree) map[string]bool {
	paths := CollectContainerPaths(tree)
	set := make(map[string]bool, len(paths))
	for _, p := range paths {
		set[p] = true
	}
	return set
}

// selectorHasPresentRoot reports whether any root governed by sec is present
// in presentPaths.
func selectorHasPresentRoot(sec *registeredSelector, presentPaths map[string]bool) bool {
	_, present := firstPresentSelectorRoot(sec, presentPaths)
	return present
}

// firstPresentSelectorRoot returns the first governed root of sec present in
// presentPaths.
func firstPresentSelectorRoot(sec *registeredSelector, presentPaths map[string]bool) (string, bool) {
	return firstPresentRoot(sec.sel.Roots, presentPaths)
}

// firstPresentRoot returns the first root in roots that exists in presentPaths.
func firstPresentRoot(roots []string, presentPaths map[string]bool) (string, bool) {
	for _, root := range roots {
		clean := strings.TrimPrefix(root, PathSep)
		if presentPaths[clean] {
			return clean, true
		}
	}
	return "", false
}

// sortedBackends returns a sorted, deduplicated copy of names.
func sortedBackends(names []string) []string {
	if len(names) == 0 {
		return nil
	}
	out := slices.Clone(names)
	slices.Sort(out)
	return slices.Compact(out)
}

// configPathDisplay formats a slash-separated YANG/config path into operator
// space-separated syntax (for example, "/traffic/control/backend" becomes
// "traffic control backend", and "fib/vpp" becomes "fib vpp").
func configPathDisplay(path string) string {
	trimmed := strings.TrimPrefix(path, PathSep)
	return strings.ReplaceAll(trimmed, PathSep, " ")
}
