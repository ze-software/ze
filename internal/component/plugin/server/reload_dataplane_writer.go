// Design: docs/architecture/plugin/plugin-system.md -- reload data-plane writer judgement
// Related: startup_autoload.go -- boot auto-load and stopCollectedProcesses
// Related: reload.go -- reloadConfig driving auto-load and deferred stop
// Related: ../registry/dataplane_writer.go -- JudgeDataPlaneWriter shared by boot, reload, and doctor

package server

import (
	"context"
	"fmt"
	"slices"

	"github.com/ze-software/ze/internal/component/config"
	plugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// fibProcessKey uniquely identifies a running or chosen ProgramsFIB process by
// its endpoint/process name, compiled registration identity, and whether it
// runs in a separate process.
type fibProcessKey struct {
	name            string
	compiledID      string
	separateProcess bool
}

func fibKeyOfConfig(cfg plugin.PluginConfig) fibProcessKey {
	return fibProcessKey{
		name:            cfg.Name,
		compiledID:      cfg.CompiledIdentity(),
		separateProcess: cfg.SeparateProcess,
	}
}

func fibKeyOfProcess(proc *process.Process) (fibProcessKey, bool) {
	if proc == nil {
		return fibProcessKey{}, false
	}
	cfg := proc.Config()
	reg := registryRow(cfg)
	if reg == nil {
		return fibProcessKey{}, false
	}
	if !reg.ProgramsFIB {
		return fibProcessKey{}, false
	}
	return fibProcessKey{
		name:            proc.Name(),
		compiledID:      cfg.CompiledIdentity(),
		separateProcess: cfg.SeparateProcess,
	}, true
}

// collectAllContainerMapPaths collects every container path present in tree,
// sorted deterministically.
func collectAllContainerMapPaths(tree map[string]any) []string {
	if len(tree) == 0 {
		return nil
	}
	var paths []string
	for k, v := range tree {
		sub, ok := v.(map[string]any)
		if !ok {
			continue
		}
		paths = append(paths, k)
		collectContainerMapPaths(sub, k, &paths)
	}
	slices.Sort(paths)
	return paths
}

// chosenFIBPlugins answers the ProgramsFIB plugins chosen by (tree, selection):
// compiled-in ProgramsFIB entries in selection (bus endpoints and recorded
// --plugin entries) plus any ProgramsFIB plugin whose ConfigRoots are present
// in tree unless already selected in selection by CompiledIdentity.
func chosenFIBPlugins(tree map[string]any, selection []plugin.PluginConfig) []plugin.PluginConfig {
	var chosen []plugin.PluginConfig
	seenKeys := make(map[fibProcessKey]bool)
	selectedIDs := make(map[string]bool)

	for _, pc := range selection {
		if pc.RunsExternalProgram() {
			continue
		}
		id := pc.CompiledIdentity()
		if id == "" {
			continue
		}
		if !registry.ProgramsFIB(id) {
			continue
		}
		entry := pc
		if entry.Encoder == "" {
			entry.Encoder = encoderJSON
		}
		key := fibKeyOfConfig(entry)
		if seenKeys[key] {
			continue
		}
		seenKeys[key] = true
		selectedIDs[id] = true
		chosen = append(chosen, entry)
	}

	allPaths := collectAllContainerMapPaths(tree)
	for _, name := range registry.PluginsForConfigPaths(allPaths) {
		if !registry.ProgramsFIB(name) {
			continue
		}
		if selectedIDs[name] {
			continue
		}
		entry := plugin.PluginConfig{
			Name:           name,
			CompiledPlugin: name,
			Encoder:        encoderJSON,
			Internal:       true,
		}
		key := fibKeyOfConfig(entry)
		if seenKeys[key] {
			continue
		}
		seenKeys[key] = true
		chosen = append(chosen, entry)
	}
	return chosen
}

func sameChosenFIB(a, b []plugin.PluginConfig) bool {
	if len(a) != len(b) {
		return false
	}
	setA := make(map[fibProcessKey]bool, len(a))
	for _, pc := range a {
		setA[fibKeyOfConfig(pc)] = true
	}
	for _, pc := range b {
		if !setA[fibKeyOfConfig(pc)] {
			return false
		}
	}
	return true
}

func runningDataPlaneProducers(pm *process.ProcessManager) []string {
	if pm == nil {
		return nil
	}
	var producers []string
	for _, proc := range pm.AllProcesses() {
		reg := registryRow(proc.Config())
		if reg != nil && reg.NeedsDataPlane {
			producers = append(producers, reg.Name)
		}
	}
	slices.Sort(producers)
	return slices.Compact(producers)
}

func chosenStateMatchesRunning(
	candidateChosen []plugin.PluginConfig,
	willRunProducers []string,
	runningFIB []*process.Process,
	runningFIBByKey map[fibProcessKey]*process.Process,
) bool {
	if len(candidateChosen) > 0 {
		if len(runningFIB) != len(candidateChosen) {
			return false
		}
		for _, pc := range candidateChosen {
			if _, ok := runningFIBByKey[fibKeyOfConfig(pc)]; !ok {
				return false
			}
		}
		return true
	}
	if len(willRunProducers) == 0 {
		return len(runningFIB) == 0
	}
	return len(runningFIB) == 1
}

type reloadWriterVerdict struct {
	start        []plugin.PluginConfig
	startReasons map[string]AutoLoadReason
	stop         map[string]bool
}

// judgeReloadWriters evaluates the data-plane writer judgement over the whole
// reload candidate (producers that will run, chosen FIB plugins in the
// candidate tree and selection, and the candidate's configured data plane).
//
// The judgement follows the four-row table in docs/architecture/plugin/plugin-system.md:
//  1. Chosen FIB plugins unchanged and producer set unchanged (and running
//     processes already match): start nothing, stop nothing.
//  2. One or more chosen FIB plugins: start every chosen one not already
//     running, stop every running ProgramsFIB process not chosen.
//  3. Zero chosen FIB plugins and one or more producers will run: keep a
//     running engine-added writer (even across an interface backend change);
//     otherwise ask registry.JudgeDataPlaneWriter for the selected plane's
//     writer and stop every other running ProgramsFIB process.
//  4. Zero chosen FIB plugins and zero producers will run: stop every running
//     ProgramsFIB process.
func (s *Server) judgeReloadWriters(
	runningTree, newTree map[string]any,
	runningSelection, candidateSelection []plugin.PluginConfig,
	willRunProducers []string,
	newPathSet map[string]bool,
) (reloadWriterVerdict, error) {
	var verdict reloadWriterVerdict
	pm := s.procManager.Load()

	runningChosen := chosenFIBPlugins(runningTree, runningSelection)
	candidateChosen := chosenFIBPlugins(newTree, candidateSelection)

	runningProducers := runningDataPlaneProducers(pm)
	sortedWillRun := slices.Clone(willRunProducers)
	slices.Sort(sortedWillRun)
	sortedWillRun = slices.Compact(sortedWillRun)

	var runningFIB []*process.Process
	runningFIBByKey := make(map[fibProcessKey]*process.Process)
	var downFIB []plugin.PluginConfig
	stoppedFIBByKey := make(map[fibProcessKey]bool)
	if pm != nil {
		for _, proc := range pm.AllProcesses() {
			if key, ok := fibKeyOfProcess(proc); ok {
				runningFIB = append(runningFIB, proc)
				runningFIBByKey[key] = proc
			}
		}
		for _, cfg := range pm.ConfiguredConfigs() {
			if pm.GetProcess(cfg.Name) != nil {
				continue
			}
			reg := registryRow(cfg)
			if reg == nil {
				continue
			}
			if !reg.ProgramsFIB {
				continue
			}
			downFIB = append(downFIB, cfg)
			if _, stopped := pm.Stopped(cfg.Name); stopped {
				stoppedFIBByKey[fibKeyOfConfig(cfg)] = true
			}
		}
	}

	// Row 1: chosen FIB plugins unchanged and producer set unchanged -> swap no writer.
	if sameChosenFIB(runningChosen, candidateChosen) && slices.Equal(runningProducers, sortedWillRun) {
		if len(downFIB) == 0 && chosenStateMatchesRunning(candidateChosen, sortedWillRun, runningFIB, runningFIBByKey) {
			return verdict, nil
		}
	}

	// Row 2: one or more chosen FIB plugins in the candidate.
	if len(candidateChosen) > 0 {
		chosenKeys := make(map[fibProcessKey]bool, len(candidateChosen))
		for _, pc := range candidateChosen {
			key := fibKeyOfConfig(pc)
			chosenKeys[key] = true
			if _, running := runningFIBByKey[key]; !running && !stoppedFIBByKey[key] {
				verdict.start = append(verdict.start, pc)
				if pc.Name == pc.CompiledIdentity() && !pc.SeparateProcess {
					if matchedRoot := firstMatchingConfigRoot(pc.Name, newPathSet); matchedRoot != "" {
						if verdict.startReasons == nil {
							verdict.startReasons = make(map[string]AutoLoadReason)
						}
						verdict.startReasons[pc.Name] = AutoLoadReason{
							Kind:   AutoLoadReasonConfigPath,
							Source: matchedRoot,
							Detail: "config section " + matchedRoot,
						}
					}
				}
			}
		}
		for _, proc := range runningFIB {
			key, _ := fibKeyOfProcess(proc)
			if !chosenKeys[key] {
				if verdict.stop == nil {
					verdict.stop = make(map[string]bool)
				}
				verdict.stop[proc.Name()] = true
				logger().Info("config reload: stopping FIB plugin no longer selected by configuration",
					"plugin", proc.Name())
			}
		}
		for _, cfg := range downFIB {
			if !chosenKeys[fibKeyOfConfig(cfg)] {
				if verdict.stop == nil {
					verdict.stop = make(map[string]bool)
				}
				verdict.stop[cfg.Name] = true
				logger().Info("config reload: unconfiguring down FIB plugin no longer selected by configuration",
					"plugin", cfg.Name)
			}
		}
		return verdict, nil
	}

	// Row 4: zero chosen FIB plugins and zero producers will run.
	if len(sortedWillRun) == 0 {
		for _, proc := range runningFIB {
			if verdict.stop == nil {
				verdict.stop = make(map[string]bool)
			}
			verdict.stop[proc.Name()] = true
			logger().Info("config reload: stopping FIB plugin because no route producer remains",
				"plugin", proc.Name())
		}
		for _, cfg := range downFIB {
			if verdict.stop == nil {
				verdict.stop = make(map[string]bool)
			}
			verdict.stop[cfg.Name] = true
			logger().Info("config reload: unconfiguring down FIB plugin because no route producer remains",
				"plugin", cfg.Name)
		}
		return verdict, nil
	}

	// Row 3: zero chosen FIB plugins and one or more producers will run.
	runningChosenKeys := make(map[fibProcessKey]bool, len(runningChosen))
	for _, pc := range runningChosen {
		runningChosenKeys[fibKeyOfConfig(pc)] = true
	}
	var keptEngineWriter *process.Process
	for _, proc := range runningFIB {
		key, _ := fibKeyOfProcess(proc)
		if !runningChosenKeys[key] {
			keptEngineWriter = proc
			break
		}
	}
	if keptEngineWriter != nil {
		for _, proc := range runningFIB {
			if proc.Name() == keptEngineWriter.Name() {
				continue
			}
			if verdict.stop == nil {
				verdict.stop = make(map[string]bool)
			}
			verdict.stop[proc.Name()] = true
			logger().Info("config reload: stopping displaced FIB plugin", "plugin", proc.Name())
		}
		for _, cfg := range downFIB {
			if cfg.Name == keptEngineWriter.Name() {
				continue
			}
			if verdict.stop == nil {
				verdict.stop = make(map[string]bool)
			}
			verdict.stop[cfg.Name] = true
			logger().Info("config reload: unconfiguring displaced down FIB plugin", "plugin", cfg.Name)
		}
		return verdict, nil
	}

	dataPlane := ConfigDataPlane(newTree)
	writer, err := registry.JudgeDataPlaneWriter(sortedWillRun, nil, dataPlane)
	if err != nil {
		return reloadWriterVerdict{}, err
	}
	if writer != "" {
		writerKey := fibProcessKey{name: writer, compiledID: writer, separateProcess: false}
		if _, alreadyRunning := runningFIBByKey[writerKey]; !alreadyRunning && !stoppedFIBByKey[writerKey] {
			verdict.start = append(verdict.start, plugin.PluginConfig{
				Name:           writer,
				CompiledPlugin: writer,
				Encoder:        encoderJSON,
				Internal:       true,
			})
			var tb textbuf.Buffer
			verdict.startReasons = map[string]AutoLoadReason{
				writer: {
					Kind:   AutoLoadReasonDataPlane,
					Source: dataPlane,
					Detail: tb.Str("data plane ").Str(dataPlane).Str(" for ").Str(sortedWillRun[0]).String(),
				},
			}
			logger().Info("auto-loading the FIB plugin for the configured data plane",
				"plugin", writer, "data-plane", dataPlane)
		}
		for _, proc := range runningFIB {
			key, _ := fibKeyOfProcess(proc)
			if key == writerKey {
				continue
			}
			if verdict.stop == nil {
				verdict.stop = make(map[string]bool)
			}
			verdict.stop[proc.Name()] = true
			logger().Info("config reload: stopping displaced FIB plugin", "plugin", proc.Name())
		}
		for _, cfg := range downFIB {
			if fibKeyOfConfig(cfg) == writerKey {
				continue
			}
			if verdict.stop == nil {
				verdict.stop = make(map[string]bool)
			}
			verdict.stop[cfg.Name] = true
			logger().Info("config reload: unconfiguring displaced down FIB plugin", "plugin", cfg.Name)
		}
	}
	return verdict, nil
}

// collectNonFIBRemovedProcesses returns the non-ProgramsFIB plugins (running or
// configured down with no live process) removed by removedRoots (without orphan
// expansion).
func (s *Server) collectNonFIBRemovedProcesses(pm *process.ProcessManager, removedRoots []string, tree map[string]any) map[string]bool {
	stopped := make(map[string]bool)
	if pm == nil {
		return stopped
	}
	if len(removedRoots) == 0 {
		return stopped
	}
	removedSet := make(map[string]bool, len(removedRoots))
	for _, root := range removedRoots {
		removedSet[root] = true
	}
	for pluginName, roots := range registry.ConfigRootsMap() {
		if registry.ProgramsFIB(pluginName) {
			continue
		}
		if s.hasConfiguredPlugin(pluginName) {
			continue
		}
		proc := pm.GetProcess(pluginName)
		if proc != nil {
			if proc.RegistrationName() != pluginName {
				continue
			}
		} else if pm.RegistrationFor(pluginName) != pluginName {
			continue
		}
		if rootRemovalStopsOwner(roots, removedSet, tree) {
			stopped[pluginName] = true
		}
	}
	return stopped
}

// collectNonFIBAddedPlugins returns the non-ProgramsFIB plugins needed for
// addedRoots and newly held ConfigNeeds in newTree, along with their seed
// reasons and the expanded container path set of addedRoots.
func (s *Server) collectNonFIBAddedPlugins(
	pm *process.ProcessManager,
	newTree map[string]any,
	addedRoots []string,
) (needed []string, seedReasons map[string]AutoLoadReason, pathSet map[string]bool) {
	newPaths := make([]string, 0, len(addedRoots))
	for _, root := range addedRoots {
		newPaths = append(newPaths, root)
		sub := navigateNestedMap(newTree, root)
		if sub != nil {
			collectContainerMapPaths(sub, root, &newPaths)
		}
	}

	pathSet = make(map[string]bool, len(newPaths))
	for _, p := range newPaths {
		pathSet[p] = true
	}

	seen := make(map[string]bool)
	seedReasons = make(map[string]AutoLoadReason)
	for _, pluginName := range registry.PluginsForConfigPaths(newPaths) {
		if registry.ProgramsFIB(pluginName) {
			continue
		}
		if s.isPluginLoaded(pluginName) {
			continue
		}
		if s.hasConfiguredPlugin(pluginName) {
			continue
		}
		if pm != nil && pm.GetProcess(pluginName) != nil {
			continue
		}
		seen[pluginName] = true
		matchedRoot := firstMatchingConfigRoot(pluginName, pathSet)
		seedReasons[pluginName] = AutoLoadReason{
			Kind:   AutoLoadReasonConfigPath,
			Source: matchedRoot,
			Detail: "config section " + matchedRoot,
		}
		needed = append(needed, pluginName)
		logger().Info("config reload: auto-loading plugin for new config path", "plugin", pluginName, "root", matchedRoot)
	}

	needHolds := config.ConfigNeedHolds(newTree)
	if needHolds != nil {
		needed = appendHeldNeedProvidersWithReasons(needed, seen, needHolds, func(name string) bool {
			return s.isRegistrationRunning(pm, name)
		}, seedReasons)
	}
	return needed, seedReasons, pathSet
}

func computeWillRunProducers(pm *process.ProcessManager, nonFIBStopped map[string]bool, nonFIBAdded []string) []string {
	var producers []string
	if pm != nil {
		for _, proc := range pm.AllProcesses() {
			if nonFIBStopped[proc.Name()] {
				continue
			}
			reg := registryRow(proc.Config())
			if reg != nil && reg.NeedsDataPlane {
				producers = append(producers, reg.Name)
			}
		}
	}
	for _, name := range nonFIBAdded {
		if registry.NeedsDataPlane(name) {
			producers = append(producers, name)
		}
	}
	slices.Sort(producers)
	return slices.Compact(producers)
}

func (s *Server) effectiveReloadSelections(candidateSelection []plugin.PluginConfig, hasSelection bool) (runningSel, effectiveCandidate []plugin.PluginConfig) {
	runningSel = s.runningSelectionOrBoot()
	if hasSelection {
		return runningSel, candidateSelection
	}
	return runningSel, runningSel
}

// selectReloadAutoLoadWithReasons computes the full start set for a reload
// (owners of added roots, held ConfigNeed providers, the writer judgement's
// ProgramsFIB starts, and their resolved dependencies).
func (s *Server) selectReloadAutoLoadWithReasons(
	runningTree, newTree map[string]any,
	addedRoots, removedRoots []string,
	candidateSelection []plugin.PluginConfig,
	hasSelection bool,
) ([]AutoLoadedEndpoint, error) {
	pm := s.procManager.Load()
	nonFIBNeeded, seedReasons, pathSet := s.collectNonFIBAddedPlugins(pm, newTree, addedRoots)

	nonFIBStopped := s.collectNonFIBRemovedProcesses(pm, removedRoots, newTree)
	if pm != nil && len(nonFIBStopped) > 0 {
		nonFIBStopped = s.collectOrphanedDependencies(pm, nonFIBStopped)
	}
	willRunProducers := computeWillRunProducers(pm, nonFIBStopped, nonFIBNeeded)

	runningSel, effectiveCandidate := s.effectiveReloadSelections(candidateSelection, hasSelection)
	verdict, err := s.judgeReloadWriters(runningTree, newTree, runningSel, effectiveCandidate, willRunProducers, pathSet)
	if err != nil {
		return nil, err
	}

	if len(nonFIBNeeded) == 0 && len(verdict.start) == 0 {
		return nil, nil
	}

	resolveInput := make([]string, 0, len(nonFIBNeeded)+len(verdict.start))
	resolveSeen := make(map[string]bool, len(nonFIBNeeded)+len(verdict.start))
	for _, name := range nonFIBNeeded {
		if !resolveSeen[name] {
			resolveSeen[name] = true
			resolveInput = append(resolveInput, name)
		}
	}
	chosenByCompiledID := make(map[string][]plugin.PluginConfig, len(verdict.start))
	for _, pc := range verdict.start {
		id := pc.CompiledIdentity()
		if id == "" {
			id = pc.Name
		}
		chosenByCompiledID[id] = append(chosenByCompiledID[id], pc)
		if !resolveSeen[id] {
			resolveSeen[id] = true
			resolveInput = append(resolveInput, id)
		}
		if reason, ok := verdict.startReasons[pc.Name]; ok {
			seedReasons[id] = reason
		}
	}

	needHolds := config.ConfigNeedHolds(newTree)
	resolved, err := registry.ResolveDependencies(resolveInput, nil, needHolds)
	if err != nil {
		logger().Error("config reload: dependency resolution failed, aborting auto-load",
			"plugins", resolveInput, "error", err, "code", CodeDoctorConfigReloadRefused)
		raiseReloadRefused(reloadRefusalMessage(
			"ze could not resolve the plugins the configuration needs, so it started none of them",
			resolveInput, err), err)
		return nil, fmt.Errorf("config-path auto-load dependency resolution: %w", err)
	}

	reasons := attributeResolvedReasons(resolveInput, resolved, seedReasons, needHolds)
	var entries []AutoLoadedEndpoint
	for _, name := range resolved {
		if configs := chosenByCompiledID[name]; len(configs) > 0 {
			for _, pc := range configs {
				entries = append(entries, AutoLoadedEndpoint{
					Config: pc,
					Reason: verdict.startReasons[pc.Name],
				})
			}
			continue
		}
		if s.isPluginLoaded(name) {
			continue
		}
		if s.hasConfiguredPlugin(name) {
			continue
		}
		if pm != nil && pm.GetProcess(name) != nil {
			continue
		}
		entries = append(entries, AutoLoadedEndpoint{
			Config: plugin.PluginConfig{
				Name:           name,
				CompiledPlugin: name,
				Encoder:        encoderJSON,
				Internal:       true,
			},
			Reason: reasons[name],
		})
	}

	return entries, nil
}

// collectReloadStopProcesses returns every running or configured-down plugin
// that a reload from (runningTree, runningSelection) to
// (newTree, candidateSelection) must stop or unconfigure: owners of removed
// roots, displaced or unneeded ProgramsFIB plugins judged by
// judgeReloadWriters, and their transitively orphaned dependencies.
func (s *Server) collectReloadStopProcesses(
	runningTree, newTree map[string]any,
	addedRoots, removedRoots []string,
	candidateSelection []plugin.PluginConfig,
	hasSelection bool,
) map[string]bool {
	pm := s.procManager.Load()
	if pm == nil {
		return nil
	}

	nonFIBStopped := s.collectNonFIBRemovedProcesses(pm, removedRoots, newTree)
	nonFIBStoppedExpanded := s.collectOrphanedDependencies(pm, nonFIBStopped)
	nonFIBAdded, _, pathSet := s.collectNonFIBAddedPlugins(pm, newTree, addedRoots)
	willRunProducers := computeWillRunProducers(pm, nonFIBStoppedExpanded, nonFIBAdded)

	runningSel, effectiveCandidate := s.effectiveReloadSelections(candidateSelection, hasSelection)
	verdict, err := s.judgeReloadWriters(runningTree, newTree, runningSel, effectiveCandidate, willRunProducers, pathSet)
	if err != nil {
		// An unresolvable data plane is refused by autoLoadForReload before
		// any stop executes; return only the non-FIB removals here.
		return nonFIBStoppedExpanded
	}

	stopped := make(map[string]bool, len(nonFIBStopped)+len(verdict.stop))
	for name := range nonFIBStopped {
		stopped[name] = true
	}
	for name := range verdict.stop {
		stopped[name] = true
	}
	return s.collectOrphanedDependencies(pm, stopped)
}

// autoLoadForReload selects and starts reload plugins, deferring any start
// whose process name currently collides with a running process in
// preflightStopped (same-name bus endpoint replacement, e.g. changing one
// compiled-in ProgramsFIB plugin to another on the same endpoint key) until
// stopCollectedProcesses frees that slot.
func (s *Server) autoLoadForReload(
	ctx context.Context,
	runningTree, newTree map[string]any,
	addedRoots, removedRoots []string,
	candidateSelection []plugin.PluginConfig,
	hasSelection bool,
	preflightStopped map[string]bool,
) (started []string, deferred []AutoLoadedEndpoint, err error) {
	entries, err := s.selectReloadAutoLoadWithReasons(runningTree, newTree, addedRoots, removedRoots, candidateSelection, hasSelection)
	if err != nil {
		return nil, nil, err
	}
	if len(entries) == 0 {
		return nil, nil, nil
	}

	pm := s.procManager.Load()
	var immediate []AutoLoadedEndpoint
	for _, entry := range entries {
		if pm != nil && preflightStopped[entry.Config.Name] && pm.GetProcess(entry.Config.Name) != nil {
			deferred = append(deferred, entry)
			continue
		}
		immediate = append(immediate, entry)
	}
	if len(immediate) == 0 {
		return nil, deferred, nil
	}
	s.recordAutoLoadReasons(immediate)
	recovery, _ := ctx.Value(removalRecoveryContextKey{}).(bool)
	started, err = s.startAutoLoaded(ctx, autoLoadedConfigs(immediate), newTree, recovery)
	if err != nil {
		return nil, nil, err
	}
	return started, deferred, nil
}

// stopOrderGraph builds the dependency graph for stopCollectedProcesses,
// adding an ordering edge from every NeedsDataPlane producer to every
// ProgramsFIB writer in selected so that reversing the topological tiers
// always stops producers before the FIB writer (AC-16).
func stopOrderGraph(selected []plugin.PluginConfig) map[string][]string {
	graph := selectedPluginGraph(selected)
	var producers []string
	var writers []string
	for _, cfg := range selected {
		reg := registryRow(cfg)
		if reg == nil {
			continue
		}
		if reg.NeedsDataPlane {
			producers = append(producers, cfg.Name)
		}
		if reg.ProgramsFIB {
			writers = append(writers, cfg.Name)
		}
	}
	if len(producers) == 0 {
		return graph
	}
	if len(writers) == 0 {
		return graph
	}
	for _, producer := range producers {
		for _, writer := range writers {
			if producer == writer {
				continue
			}
			if !slices.Contains(graph[producer], writer) {
				graph[producer] = append(graph[producer], writer)
			}
		}
	}
	return graph
}
