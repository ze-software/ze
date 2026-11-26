// Design: docs/architecture/core-design.md -- a data-plane-mismatched feature stays off at daemon start and is reported under doctor-dataplane-mismatch
// Related: dataplane_verdict.go -- evaluateDataPlaneEntries shared with strict commit/reload/validate refusal
// Related: loader.go -- LoadConfigForStart calling excludeDataPlaneMismatches

package config

import (
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/component/plugin"
)

// ExcludedDataPlane records one data-plane-mismatched feature that
// LoadConfigForStart left off in memory so the daemon starts on its active
// data plane and reports the mismatch under doctor-dataplane-mismatch.
type ExcludedDataPlane struct {
	Problem string
	Why     string
	Fix     string
}

// excludeDataPlaneMismatches evaluates data-plane compatibility for daemon
// startup (LoadConfigForStart). Hard refusals — unreadable or unknown/retired
// backend selectors, fib dummy conflicts, and empty-backend host verdicts —
// still return the exact strict refusal error. Every excludable mismatch
// disables the mismatched feature in memory (pruning its tree roots/nodes,
// applying declarative disable paths/leaves, and stripping its bus endpoint
// and CLI plugin selector) and re-evaluates the verdict; if any finding
// survives, it fails closed by returning the original refusal error so a
// mismatched feature is never programmed.
func excludeDataPlaneMismatches(
	tree *Tree,
	schema *Schema,
	cliPlugins []string,
) (filteredCLI []string, selection []plugin.PluginConfig, excluded []ExcludedDataPlane, err error) {
	combinedCLI := mergeCommandLinePluginLists(CommandLinePlugins(), cliPlugins)
	selection, err = pluginSelectionWithCLI(tree, combinedCLI)
	if err != nil {
		return nil, nil, nil, err
	}

	entries := evaluateDataPlaneEntries(tree, schema, selection)
	if len(entries) == 0 {
		return cliPlugins, selection, nil, nil
	}

	originalErr := findingsToError(verdictFindings(entries))
	for i := range entries {
		if entries[i].hardRefusal {
			return nil, nil, nil, originalErr
		}
	}

	filteredCLI = slices.Clone(cliPlugins)
	filteredCombined := slices.Clone(combinedCLI)
	excluded = make([]ExcludedDataPlane, 0, len(entries))
	for i := range entries {
		applyDataPlaneExclusion(tree, selection, &filteredCLI, &filteredCombined, &entries[i])
		excluded = append(excluded, ExcludedDataPlane{
			Problem: entries[i].finding.problem,
			Why:     entries[i].finding.why,
			Fix:     entries[i].finding.fix,
		})
	}

	selection, selErr := pluginSelectionWithCLI(tree, filteredCombined)
	if selErr != nil {
		return nil, nil, nil, originalErr
	}
	if remaining := evaluateDataPlaneEntries(tree, schema, selection); len(remaining) > 0 {
		return nil, nil, nil, originalErr
	}
	return filteredCLI, selection, excluded, nil
}

// applyDataPlaneExclusion mutates tree and the start-time CLI plugin slices to
// turn off the single mismatched feature described by entry.
func applyDataPlaneExclusion(
	tree *Tree,
	selection []plugin.PluginConfig,
	filteredCLI, filteredCombined *[]string,
	entry *dataPlaneVerdictEntry,
) {
	for _, root := range entry.removeRoots {
		tree.RemovePath(strings.TrimPrefix(root, PathSep))
	}
	if entry.removePath != "" {
		tree.RemovePath(entry.removePath)
	}
	if entry.stripPlugin != "" {
		stripPluginFromTree(tree, selection, entry.stripPlugin)
		*filteredCLI = filterCLIPlugins(*filteredCLI, entry.stripPlugin)
		*filteredCombined = filterCLIPlugins(*filteredCombined, entry.stripPlugin)
	}
	for _, path := range entry.disablePaths {
		tree.RemoveWildcardPath(path)
	}
	if len(entry.disableLeaves) > 0 {
		leafPaths := make([]string, 0, len(entry.disableLeaves))
		for leafPath := range entry.disableLeaves {
			leafPaths = append(leafPaths, leafPath)
		}
		slices.Sort(leafPaths)
		for _, leafPath := range leafPaths {
			tree.SetLeafPath(leafPath, entry.disableLeaves[leafPath])
		}
	}
}

// stripPluginFromTree removes every active `bus endpoint <key>` entry in tree
// that selects the registered plugin regName.
func stripPluginFromTree(tree *Tree, selection []plugin.PluginConfig, regName string) {
	endpoints := activeBusEndpoints(tree)
	if len(endpoints) == 0 {
		return
	}
	for _, pc := range selection {
		if !selectedPluginMatches(pc, regName) {
			continue
		}
		if _, isEndpoint := endpoints[pc.Name]; !isEndpoint {
			continue
		}
		tree.RemovePath(busContainerName + PathSep + busListEndpoint + "[" + pc.Name + "]")
	}
}

// filterCLIPlugins returns a copy of cliPlugins without any argument that
// resolves to the registered plugin regName.
func filterCLIPlugins(cliPlugins []string, regName string) []string {
	if len(cliPlugins) == 0 {
		return nil
	}
	out := make([]string, 0, len(cliPlugins))
	for _, arg := range cliPlugins {
		resolved, err := plugin.ResolvePlugin(arg)
		if err == nil && resolved.Name == regName {
			continue
		}
		out = append(out, arg)
	}
	return out
}
