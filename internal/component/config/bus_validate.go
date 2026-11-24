// Design: docs/architecture/api/architecture.md -- bus configuration validation, endpoint parsing, and Refusal 6 conflict checks
// Related: register_bus.go -- registration seams and external declaration store
// Related: bus_validate_rules.go -- commit refusals 1-5 and 9-11
// Related: ../plugin/bus_path.go -- short bus path syntax and segment prefix matching
// Related: ../plugin/bus_config.go -- BusBlock AST and equality helpers

package config

import (
	"errors"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

const (
	busContainerName      = "bus"
	busListEndpoint       = "endpoint"
	busListParticipant    = "participant"
	busLeafInternal       = "internal"
	busLeafProcess        = "process"
	busContainerExternal  = "external"
	busLeafEncoding       = "encoding"
	busLeafOnExit         = "on-exit"
	busProcessInDaemon    = "in-daemon"
	busProcessSeparate    = "separate"
	busDirectionReceive   = "receive"
	busDirectionSend      = "send"
	busTargetKeyFrom      = "from"
	busTargetKeyTo        = "to"
	busLabelNone          = "none"
	busEndpointPrefixHint = "/bus/endpoint"
)

// participantNodeTemplate describes one registered participant schema node
// template discovered from compiled-in and external registrations.
type participantNodeTemplate struct {
	ownerKind  string
	shortPath  string
	segments   []string
	sends      []rpc.MessageSendDecl
	receives   []rpc.MessageReceiveDecl
	hasDecl    bool
	isExternal bool
}

// ValidateBusConfig validates the bus configuration and every participant bus
// block in tree, enforcing structural endpoint rules and all eleven commit
// refusals. Registered as an original-tree validator so ze config validate,
// editor commit, reload, and ze start share one verdict.
func ValidateBusConfig(tree *Tree) error {
	if tree == nil {
		return nil
	}

	// 1. Validate structural endpoint definitions and collect endpoint entries.
	endpoints, endpointBlocks, endpointKindMap, err := parseBusEndpoints(tree)
	if err != nil {
		return err
	}

	// 2. Discover configured participants from registered participant nodes,
	// explicit bus endpoints, and auto-loaded compiled-in plugins.
	templates := collectParticipantNodeTemplates()
	participants, singletonByKind, err := discoverParticipants(tree, templates, endpoints, endpointBlocks, endpointKindMap)
	if err != nil {
		return err
	}

	// 3. Check Refusal 6 between a plugin's singleton config container and its
	// explicit bus endpoint when both carry bus blocks.
	if err := checkEndpointVsSingletonConflicts(participants, singletonByKind, endpointBlocks, endpointKindMap); err != nil {
		return err
	}

	// 4. Parse and apply central bus > participant <path> overrides, enforcing
	// path syntax, Refusal 1 on the participant key, and Refusal 6 same-rank
	// conflicts.
	prefixOverrides, err := applyCentralParticipantOverrides(tree, participants, templates)
	if err != nil {
		return err
	}

	// 5. Run Refusals 1-5 and 9-11 on every participant and prefix override.
	allTargets := make([]*BusValidatedParticipant, 0, len(participants)+len(prefixOverrides))
	for _, path := range slices.Sorted(maps.Keys(participants)) {
		allTargets = append(allTargets, participants[path])
	}
	allTargets = append(allTargets, prefixOverrides...)

	for _, p := range allTargets {
		if p.Block.IsEmpty() {
			continue
		}
		if err := validateParticipantBusBlock(tree, p, participants, templates); err != nil {
			return err
		}
	}

	// 6. Run registered feature-requirement validators (Refusal 7).
	ctx := &BusValidationContext{
		Participants: participants,
		Endpoints:    endpoints,
	}
	busRequirementValidatorsMu.RLock()
	reqValidators := slices.Clone(busRequirementValidators)
	busRequirementValidatorsMu.RUnlock()
	for _, rv := range reqValidators {
		if errs := rv.fn(tree, ctx); len(errs) > 0 {
			return errs[0]
		}
	}

	return nil
}

// parseBusEndpoints validates every entry under bus > endpoint and extracts its
// PluginConfig and parsed BusBlock.
func parseBusEndpoints(tree *Tree) (
	endpoints []plugin.PluginConfig,
	blocks map[string]plugin.BusBlock,
	kindByEndpoint map[string]string,
	err error,
) {
	blocks = make(map[string]plugin.BusBlock)
	kindByEndpoint = make(map[string]string)

	busContainer := tree.GetContainer(busContainerName)
	if busContainer == nil {
		return nil, blocks, kindByEndpoint, nil
	}

	seen := make(map[string]bool)
	for _, entry := range busContainer.GetListOrdered(busListEndpoint) {
		name, ep := entry.Key, entry.Value
		if strings.HasPrefix(name, "_") {
			var tb textbuf.Buffer
			return nil, nil, nil, opererr.New(
				tb.Str("bus > endpoint ").Quoted(name).Str(": names starting with underscore are reserved").String(),
				"underscore-prefixed endpoint names are reserved for internal daemon machinery",
				"rename the endpoint to a name not starting with '_' under bus { endpoint <name> { ... } }")
		}
		if seen[name] {
			var tb textbuf.Buffer
			return nil, nil, nil, opererr.New(
				tb.Str("bus > endpoint ").Quoted(name).Str(": duplicate endpoint name").String(),
				"each bus endpoint must have a unique name",
				"keep one bus { endpoint "+name+" { ... } } block")
		}
		seen[name] = true

		pc, kind, parseErr := parseSingleEndpointConfig(name, ep)
		if parseErr != nil {
			return nil, nil, nil, parseErr
		}
		block, blockErr := plugin.ParseBusBlockFromMap(ep.ToMap(), "bus > endpoint "+name)
		if blockErr != nil {
			return nil, nil, nil, blockErr
		}
		endpoints = append(endpoints, pc)
		blocks[name] = block
		kindByEndpoint[name] = kind
	}
	return endpoints, blocks, kindByEndpoint, nil
}

// parseSingleEndpointConfig validates structural rules for one bus > endpoint
// entry and returns its PluginConfig and resolved kind name. Shared with
// ExtractPluginsFromTree so extraction and validation enforce identical rules.
func parseSingleEndpointConfig(name string, ep *Tree) (plugin.PluginConfig, string, error) {
	internalVal, hasInternal := ep.Get(busLeafInternal)
	internalSet := hasInternal && internalVal != ""
	extContainer := ep.GetContainer(busContainerExternal)
	externalSet := extContainer != nil

	if internalSet && externalSet {
		var problem, fix textbuf.Buffer
		problem.Str("bus > endpoint ").Quoted(name).Str(": internal and external are mutually exclusive")
		fix.Str("keep either internal <plugin>; or external { run \"...\"; } in bus { endpoint ").Str(name).Str(" { ... } }")
		return plugin.PluginConfig{}, "", opererr.New(
			problem.String(),
			"a bus endpoint runs either a compiled-in plugin or an external program, never both",
			fix.String())
	}
	if !internalSet && !externalSet {
		var problem, fix textbuf.Buffer
		problem.Str("bus > endpoint ").Quoted(name).Str(": must specify either internal or external")
		fix.Str("add internal <plugin>; or external { run \"...\"; } to bus { endpoint ").Str(name).Str(" { ... } }")
		return plugin.PluginConfig{}, "", opererr.New(
			problem.String(),
			"every bus endpoint must identify either a compiled-in plugin or an external program to run",
			fix.String())
	}

	procMode, hasProc := ep.Get(busLeafProcess)
	procSet := hasProc && procMode != ""
	if procSet && externalSet {
		var problem, fix textbuf.Buffer
		problem.Str("bus > endpoint ").Quoted(name).Str(": process is valid only with internal endpoints")
		fix.Str("remove process from bus { endpoint ").Str(name).Str(" { ... } }")
		return plugin.PluginConfig{}, "", opererr.New(
			problem.String(),
			"external endpoints always run as separate processes; the process option chooses in-daemon or separate for compiled-in plugins only",
			fix.String())
	}
	if procSet && procMode != busProcessInDaemon && procMode != busProcessSeparate {
		var problem, fix textbuf.Buffer
		problem.Str("bus > endpoint ").Quoted(name).Str(": invalid process mode ").Quoted(procMode)
		fix.Str("set process in-daemon; or process separate; in bus { endpoint ").Str(name).Str(" { ... } }")
		return plugin.PluginConfig{}, "", opererr.New(
			problem.String(),
			"process must be either in-daemon or separate",
			fix.String())
	}

	pc := plugin.PluginConfig{Name: name}
	var kind string
	if internalSet {
		compiled := strings.TrimPrefix(internalVal, "ze.")
		reg := registry.Lookup(compiled)
		if reg == nil {
			var problem, why, fix textbuf.Buffer
			problem.Str("bus > endpoint ").Quoted(name).Str(": unknown compiled-in plugin ").Quoted(internalVal)
			why.Str("no compiled-in plugin named ").Quoted(compiled).Str(" is registered in this binary")
			if available := registry.Names(); len(available) > 0 {
				why.Str(" (registered: ").Join(available, ", ").Byte(')')
			}
			fix.Str("set internal to a registered plugin name in bus { endpoint ").Str(name).Str(" { internal <plugin>; } }")
			return plugin.PluginConfig{}, "", opererr.New(problem.String(), why.String(), fix.String())
		}
		if procMode == busProcessSeparate && reg.RunsInProcessOnly {
			var problem, why, fix textbuf.Buffer
			problem.Str("bus > endpoint ").Quoted(name).Str(": process separate is not supported for ").Quoted(reg.Name)
			why.Str("plugin ").Quoted(reg.Name).Str(" accesses daemon-local state and declares RunsInProcessOnly, so it can run only inside the daemon")
			fix.Str("bus { endpoint ").Str(name).Str(" { internal ").Str(reg.Name).Str("; process in-daemon; } }")
			return plugin.PluginConfig{}, "", opererr.New(problem.String(), why.String(), fix.String())
		}
		pc.CompiledPlugin = reg.Name
		pc.SeparateProcess = procMode == busProcessSeparate
		pc.Internal = !pc.SeparateProcess
		kind = reg.Name
	} else {
		if !ipc.ValidPluginName(name) {
			var problem, why, fix textbuf.Buffer
			problem.Str("bus > endpoint ").Quoted(name).Str(": invalid endpoint name for external program")
			why.Str("external endpoint names are bound into the connect-back token and must match ").Str(ipc.PluginNameForm)
			fix.Str("rename the endpoint under bus { endpoint <name> { external { ... } } }")
			return plugin.PluginConfig{}, "", opererr.New(problem.String(), why.String(), fix.String())
		}
		runVal, _ := extContainer.Get("run")
		if runVal == "" {
			var problem, fix textbuf.Buffer
			problem.Str("bus > endpoint ").Quoted(name).Str(": external endpoint requires run")
			fix.Str("add run \"<command>\"; inside bus { endpoint ").Str(name).Str(" { external { ... } } }")
			return plugin.PluginConfig{}, "", opererr.New(
				problem.String(),
				"an external endpoint cannot be started without a command line",
				fix.String())
		}
		pc.Run = runVal
		if v, ok := extContainer.Get("respawn"); ok {
			respawn, err := ParseBoolStrict(v)
			if err != nil {
				var problem, fix textbuf.Buffer
				problem.Str("bus > endpoint ").Quoted(name).Str(": invalid respawn value ").Quoted(v)
				fix.Str("set respawn true; or respawn false; inside bus { endpoint ").Str(name).Str(" { external { ... } } }")
				return plugin.PluginConfig{}, "", opererr.New(problem.String(), "respawn must be true or false", fix.String())
			}
			pc.Respawn = plugin.RespawnDeclined
			if respawn {
				pc.Respawn = plugin.RespawnAsked
			}
		}
		if v, ok := extContainer.Get("timeout"); ok {
			d, err := parsePositiveDuration(v)
			if err != nil {
				var problem, fix textbuf.Buffer
				problem.Str("bus > endpoint ").Quoted(name).Str(": invalid timeout value ").Quoted(v)
				fix.Str("set timeout to a positive duration such as 5s inside bus { endpoint ").Str(name).Str(" { external { ... } } }")
				return plugin.PluginConfig{}, "", opererr.New(problem.String(), err.Error(), fix.String())
			}
			pc.StageTimeout = d
		}
		if v, ok := extContainer.Get(busLeafOnExit); ok && v != "" {
			if v != plugin.OnExitWithdraw && v != plugin.OnExitKeep {
				var problem, fix textbuf.Buffer
				problem.Str("bus > endpoint ").Quoted(name).Str(": invalid on-exit value ").Quoted(v)
				fix.Str("set on-exit withdraw; or on-exit keep; inside bus { endpoint ").Str(name).Str(" { external { ... } } }")
				return plugin.PluginConfig{}, "", opererr.New(problem.String(), "on-exit must be withdraw or keep", fix.String())
			}
			pc.OnExit = v
		}
		kind = name
	}

	if v, ok := ep.Get(busLeafEncoding); ok && v != "" {
		if v != plugin.EncodingJSON && v != plugin.EncodingText {
			var problem, fix textbuf.Buffer
			problem.Str("bus > endpoint ").Quoted(name).Str(": invalid encoding ").Quoted(v)
			fix.Str("set encoding json; or encoding text; in bus { endpoint ").Str(name).Str(" { ... } }")
			return plugin.PluginConfig{}, "", opererr.New(problem.String(), "encoding must be json or text", fix.String())
		}
		pc.Encoder = v
		if v == plugin.EncodingText {
			pc.ReceiveUpdate = true
		}
	}

	return pc, kind, nil
}

func collectParticipantNodeTemplates() []participantNodeTemplate {
	var templates []participantNodeTemplate
	for _, reg := range registry.All() {
		for _, node := range reg.ParticipantNodes {
			segs := stripSchemaPrefixes(node.SchemaPath)
			if len(segs) == 0 {
				continue
			}
			templates = append(templates, participantNodeTemplate{
				ownerKind: reg.Name,
				shortPath: "/" + textbuf.Join(segs, "/"),
				segments:  segs,
				sends:     reg.Sends,
				receives:  reg.Receives,
				hasDecl:   true,
			})
		}
	}
	for _, ext := range ExternalBusDeclarations() {
		for _, node := range ext.ParticipantNodes {
			segs := stripSchemaPrefixes(node.SchemaPath)
			if len(segs) == 0 {
				continue
			}
			templates = append(templates, participantNodeTemplate{
				ownerKind:  ext.Endpoint,
				shortPath:  "/" + textbuf.Join(segs, "/"),
				segments:   segs,
				sends:      ext.Sends,
				receives:   ext.Receives,
				hasDecl:    true,
				isExternal: true,
			})
		}
	}
	return templates
}

func stripSchemaPrefixes(schemaPath string) []string {
	trimmed := strings.TrimPrefix(schemaPath, "/")
	if trimmed == "" {
		return nil
	}
	rawParts := strings.Split(trimmed, "/")
	out := make([]string, 0, len(rawParts))
	for _, part := range rawParts {
		if part == "" {
			continue
		}
		if _, local, ok := strings.Cut(part, ":"); ok {
			out = append(out, local)
		} else {
			out = append(out, part)
		}
	}
	return out
}

func singletonSchemaShortPath(nodes []rpc.ParticipantNodeDecl) string {
	if len(nodes) != 1 {
		return ""
	}
	segs := stripSchemaPrefixes(nodes[0].SchemaPath)
	if len(segs) == 0 {
		return ""
	}
	return "/" + textbuf.Join(segs, "/")
}

// discoverParticipants builds the map of all active configured participants in
// tree from registered participant node templates, explicit bus endpoints, and
// auto-loaded compiled-in plugins.
func discoverParticipants(
	tree *Tree,
	templates []participantNodeTemplate,
	endpoints []plugin.PluginConfig,
	endpointBlocks map[string]plugin.BusBlock,
	endpointKindMap map[string]string,
) (map[string]*BusValidatedParticipant, map[string]string, error) {
	participants := make(map[string]*BusValidatedParticipant)
	singletonByKind := make(map[string]string)

	for _, tmpl := range templates {
		var matches []walkedParticipantNode
		walkParticipantNodes(tree, tmpl.segments, 0, "", false, &matches)
		for _, m := range matches {
			block := plugin.BusBlock{}
			source := m.path + " > bus"
			if busSub := m.tree.GetContainer(busContainerName); busSub != nil {
				parsed, err := plugin.ParseBusBlockFromMap(busSub.ToMap(), source)
				if err != nil {
					return nil, nil, err
				}
				block = parsed
			}
			participants[m.path] = &BusValidatedParticipant{
				Path:           m.path,
				Kind:           tmpl.ownerKind,
				HasDeclaration: tmpl.hasDecl,
				Sends:          tmpl.sends,
				Receives:       tmpl.receives,
				Block:          block,
				Source:         source,
			}
			if !m.hasListKey {
				singletonByKind[tmpl.ownerKind] = m.path
			}
		}
	}

	explicitCompiledKinds := make(map[string]bool)
	for _, ep := range endpoints {
		epPath := plugin.EndpointPath(ep.Name)
		kind := endpointKindMap[ep.Name]
		p := &BusValidatedParticipant{
			Path:   epPath,
			Kind:   kind,
			Block:  endpointBlocks[ep.Name],
			Source: "bus > endpoint " + ep.Name,
		}
		if ep.IsCompiled() {
			explicitCompiledKinds[ep.CompiledIdentity()] = true
			if reg := registry.Lookup(ep.CompiledIdentity()); reg != nil {
				p.HasDeclaration = true
				p.Sends = reg.Sends
				p.Receives = reg.Receives
				if sp := singletonSchemaShortPath(reg.ParticipantNodes); sp != "" {
					if _, exists := participants[sp]; !exists {
						participants[sp] = &BusValidatedParticipant{
							Path:           sp,
							Kind:           reg.Name,
							HasDeclaration: true,
							Sends:          reg.Sends,
							Receives:       reg.Receives,
							Block:          endpointBlocks[ep.Name],
							Source:         p.Source,
						}
						singletonByKind[reg.Name] = sp
					}
				}
			}
		} else if extDecl, ok := lookupExternalBusDecl(ep.Name); ok {
			p.HasDeclaration = true
			p.Sends = extDecl.Sends
			p.Receives = extDecl.Receives
		}
		participants[epPath] = p
	}

	var activeCompiled []string
	seenCompiled := make(map[string]bool)
	addActiveCompiled := func(name string) {
		if name == "" {
			return
		}
		if seenCompiled[name] {
			return
		}
		seenCompiled[name] = true
		activeCompiled = append(activeCompiled, name)
	}
	for name := range explicitCompiledKinds {
		addActiveCompiled(name)
	}
	for _, reg := range registry.All() {
		if explicitCompiledKinds[reg.Name] {
			continue
		}
		if len(reg.ParticipantNodes) > 1 {
			continue
		}
		for _, root := range reg.ConfigRoots {
			if tree.GetContainer(root) != nil {
				addActiveCompiled(reg.Name)
				break
			}
		}
	}
	resolvedCompiled := activeCompiled
	if len(activeCompiled) > 0 {
		if expanded, depErr := registry.ResolveDependencies(activeCompiled, nil, ConfigNeedHolds(tree.ToMap())); depErr == nil {
			resolvedCompiled = expanded
		}
	}
	for _, name := range resolvedCompiled {
		if explicitCompiledKinds[name] {
			continue
		}
		reg := registry.Lookup(name)
		if reg == nil {
			continue
		}
		if len(reg.ParticipantNodes) > 1 {
			continue
		}
		if sp := singletonSchemaShortPath(reg.ParticipantNodes); sp != "" {
			if _, exists := participants[sp]; !exists {
				participants[sp] = &BusValidatedParticipant{
					Path:           sp,
					Kind:           reg.Name,
					HasDeclaration: true,
					Sends:          reg.Sends,
					Receives:       reg.Receives,
					Source:         sp,
				}
			}
			if _, hasSingleton := singletonByKind[reg.Name]; !hasSingleton {
				singletonByKind[reg.Name] = sp
			}
		}
		epPath := plugin.EndpointPath(reg.Name)
		if _, exists := participants[epPath]; exists {
			continue
		}
		participants[epPath] = &BusValidatedParticipant{
			Path:           epPath,
			Kind:           reg.Name,
			HasDeclaration: true,
			Sends:          reg.Sends,
			Receives:       reg.Receives,
			Source:         epPath,
		}
	}

	allPaths := slices.Sorted(maps.Keys(participants))
	for _, path := range allPaths {
		var ancestors []string
		for _, cand := range allPaths {
			if cand == path {
				continue
			}
			if plugin.PathCovers(cand, path) {
				ancestors = append(ancestors, cand)
			}
		}
		slices.SortFunc(ancestors, func(a, b string) int {
			if len(a) != len(b) {
				return len(b) - len(a)
			}
			return strings.Compare(a, b)
		})
		participants[path].Ancestors = ancestors
	}

	return participants, singletonByKind, nil
}

type walkedParticipantNode struct {
	path       string
	tree       *Tree
	hasListKey bool
}

// walkParticipantNodes walks tree along a finite YANG schema path (bounded by
// len(segments)), emitting every concrete container or list entry matched.
func walkParticipantNodes(cur *Tree, segments []string, idx int, prefix string, hasListKey bool, out *[]walkedParticipantNode) {
	if cur == nil {
		return
	}
	if idx >= len(segments) {
		*out = append(*out, walkedParticipantNode{
			path:       prefix,
			tree:       cur,
			hasListKey: hasListKey,
		})
		return
	}
	seg := segments[idx]
	if sub := cur.GetContainer(seg); sub != nil {
		walkParticipantNodes(sub, segments, idx+1, prefix+"/"+seg, hasListKey, out)
	}
	for _, entry := range cur.GetListOrdered(seg) {
		walkParticipantNodes(entry.Value, segments, idx+1, prefix+"/"+seg+"/"+entry.Key, true, out)
	}
}

// checkEndpointVsSingletonConflicts enforces Refusal 6 between a plugin's own
// singleton config section and its bus > endpoint <name> entry.
func checkEndpointVsSingletonConflicts(
	participants map[string]*BusValidatedParticipant,
	singletonByKind map[string]string,
	endpointBlocks map[string]plugin.BusBlock,
	endpointKindMap map[string]string,
) error {
	for _, epName := range slices.Sorted(maps.Keys(endpointKindMap)) {
		kind := endpointKindMap[epName]
		singletonPath, ok := singletonByKind[kind]
		if !ok {
			continue
		}
		singletonP := participants[singletonPath]
		epBlock := endpointBlocks[epName]
		if singletonP == nil {
			continue
		}
		epSource := "bus > endpoint " + epName
		if err := checkBusBlockConflict(singletonP.Source, singletonP.Block, epSource, epBlock); err != nil {
			return err
		}
		merged := mergeBusBlocks(singletonP.Block, epBlock)
		singletonP.Block = merged
		if epP := participants[plugin.EndpointPath(epName)]; epP != nil {
			epP.Block = merged
		}
	}
	return nil
}

// applyCentralParticipantOverrides parses bus > participant <path> entries,
// checks path syntax and Refusal 1, checks Refusal 6 against any same-rank
// participant block, and returns any prefix-only overrides for downstream
// validation.
func applyCentralParticipantOverrides(
	tree *Tree,
	participants map[string]*BusValidatedParticipant,
	templates []participantNodeTemplate,
) ([]*BusValidatedParticipant, error) {
	busContainer := tree.GetContainer(busContainerName)
	if busContainer == nil {
		return nil, nil
	}
	var prefixOverrides []*BusValidatedParticipant
	for _, entry := range busContainer.GetListOrdered(busListParticipant) {
		pPath := entry.Key
		source := "bus > participant " + pPath
		if err := plugin.ValidateBusPathSyntax(pPath); err != nil {
			return nil, opererr.Wrap(err,
				source+": invalid participant path "+pPath,
				"bus > participant entries must name a valid short slash participant path",
				"write a valid path under bus { participant <path> { ... } }")
		}
		block, err := plugin.ParseBusBlockFromMap(entry.Value.ToMap(), source)
		if err != nil {
			return nil, err
		}

		resolvedPaths, dormant, err := resolveTargetPath(tree, pPath, source, participants, templates)
		if err != nil {
			return nil, err
		}

		if direct, ok := participants[pPath]; ok {
			if err := checkBusBlockConflict(direct.Source, direct.Block, source, block); err != nil {
				return nil, err
			}
			direct.Block = mergeBusBlocks(direct.Block, block)
			continue
		}

		if !dormant {
			if _, isSecondary := matchingSecondaryResolver(pPath); isSecondary {
				for _, cp := range resolvedPaths {
					if direct, ok := participants[cp]; ok {
						if err := checkBusBlockConflict(direct.Source, direct.Block, source, block); err != nil {
							return nil, err
						}
						direct.Block = mergeBusBlocks(direct.Block, block)
					}
				}
				continue
			}
		}

		wrapper := synthesizePrefixParticipant(pPath, source, block, resolvedPaths, participants)
		prefixOverrides = append(prefixOverrides, wrapper)
	}
	return prefixOverrides, nil
}

func synthesizePrefixParticipant(
	path, source string,
	block plugin.BusBlock,
	coveredPaths []string,
	participants map[string]*BusValidatedParticipant,
) *BusValidatedParticipant {
	out := &BusValidatedParticipant{
		Path:   path,
		Kind:   path,
		Block:  block,
		Source: source,
	}
	seenSend := make(map[string]bool)
	seenRecv := make(map[string]bool)
	for _, cp := range coveredPaths {
		p := participants[cp]
		if p == nil {
			continue
		}
		if !p.HasDeclaration {
			continue
		}
		out.HasDeclaration = true
		for _, s := range p.Sends {
			if !seenSend[s.Message] {
				seenSend[s.Message] = true
				out.Sends = append(out.Sends, s)
			}
		}
		for _, r := range p.Receives {
			if !seenRecv[r.Message] {
				seenRecv[r.Message] = true
				out.Receives = append(out.Receives, r)
			}
		}
	}
	return out
}

// checkBusBlockConflict enforces Refusal 6: the same field for the same message
// written in two places with different content is refused naming both lines.
// Identical content (order-independent set comparison) is accepted.
func checkBusBlockConflict(srcA string, a plugin.BusBlock, srcB string, b plugin.BusBlock) error {
	if err := checkDirectionConflict(srcA, busDirectionReceive, busTargetKeyFrom, a.Receive, srcB, b.Receive); err != nil {
		return err
	}
	return checkDirectionConflict(srcA, busDirectionSend, busTargetKeyTo, a.Send, srcB, b.Send)
}

func checkDirectionConflict(srcA, dir, targetKey string, a plugin.BusDirectionBlock, srcB string, b plugin.BusDirectionBlock) error {
	if a.AllowPresent && b.AllowPresent && !plugin.BusAllowEqual(a, b) {
		lineA := srcA + " > " + dir + " > allow"
		lineB := srcB + " > " + dir + " > allow"
		return refusal6ConflictError(lineA, formatAllowOrTargets(a.AllowNone, a.Allow), lineB, formatAllowOrTargets(b.AllowNone, b.Allow))
	}
	if a.TargetsPresent && b.TargetsPresent && !plugin.BusTargetsEqual(a, b) {
		lineA := srcA + " > " + dir + " > " + targetKey
		lineB := srcB + " > " + dir + " > " + targetKey
		return refusal6ConflictError(lineA, formatAllowOrTargets(a.TargetsNone, a.Targets), lineB, formatAllowOrTargets(b.TargetsNone, b.Targets))
	}
	for _, msg := range slices.Sorted(maps.Keys(a.Messages)) {
		ra := a.Messages[msg]
		rb, ok := b.Messages[msg]
		if !ok {
			continue
		}
		if ra == nil {
			continue
		}
		if rb == nil {
			continue
		}
		if ra.TargetsPresent && rb.TargetsPresent && !plugin.BusMessageRuleTargetsEqual(ra, rb) {
			lineA := srcA + " > " + dir + " > message " + msg + " > " + targetKey
			lineB := srcB + " > " + dir + " > message " + msg + " > " + targetKey
			return refusal6ConflictError(lineA, formatAllowOrTargets(ra.TargetsNone, ra.Targets), lineB, formatAllowOrTargets(rb.TargetsNone, rb.Targets))
		}
		for _, tagName := range slices.Sorted(maps.Keys(ra.Tags)) {
			va := ra.Tags[tagName]
			vb, hasB := rb.Tags[tagName]
			if !hasB {
				continue
			}
			if !plugin.StringSetEqual(va, vb) {
				lineA := srcA + " > " + dir + " > message " + msg + " > tag " + tagName
				lineB := srcB + " > " + dir + " > message " + msg + " > tag " + tagName
				return refusal6ConflictError(lineA, formatListValue(va), lineB, formatListValue(vb))
			}
		}
	}
	return nil
}

func refusal6ConflictError(lineA, valA, lineB, valB string) *opererr.Error {
	var problem, why, fix textbuf.Buffer
	problem.Str("conflicting bus rules for the same participant: ").
		Str(lineA).Str(" sets ").Str(valA).Str(", while ").
		Str(lineB).Str(" sets ").Str(valB)
	why.Str("the same field for the same participant cannot be written in two places with different values")
	fix.Str("keep one value and remove or match the conflicting entry in ").Str(lineA).Str(" or ").Str(lineB)
	return opererr.New(problem.String(), why.String(), fix.String())
}

func mergeBusBlocks(a, b plugin.BusBlock) plugin.BusBlock {
	return plugin.BusBlock{
		Receive: mergeDirectionBlocks(a.Receive, b.Receive),
		Send:    mergeDirectionBlocks(a.Send, b.Send),
	}
}

func mergeDirectionBlocks(a, b plugin.BusDirectionBlock) plugin.BusDirectionBlock {
	out := a.Clone()
	if !out.AllowPresent && b.AllowPresent {
		out.AllowPresent = true
		out.AllowNone = b.AllowNone
		out.Allow = slices.Clone(b.Allow)
	}
	if !out.TargetsPresent && b.TargetsPresent {
		out.TargetsPresent = true
		out.TargetsNone = b.TargetsNone
		out.Targets = slices.Clone(b.Targets)
	}
	for msg, rb := range b.Messages {
		if rb == nil {
			continue
		}
		if out.Messages == nil {
			out.Messages = make(map[string]*plugin.BusMessageRule)
		}
		ra, exists := out.Messages[msg]
		if !exists {
			out.Messages[msg] = rb.Clone()
			continue
		}
		if ra == nil {
			out.Messages[msg] = rb.Clone()
			continue
		}
		if !ra.TargetsPresent && rb.TargetsPresent {
			ra.TargetsPresent = true
			ra.TargetsNone = rb.TargetsNone
			ra.Targets = slices.Clone(rb.Targets)
		}
		for tagName, vals := range rb.Tags {
			if ra.Tags == nil {
				ra.Tags = make(map[string][]string)
			}
			if _, hasTag := ra.Tags[tagName]; !hasTag {
				ra.Tags[tagName] = slices.Clone(vals)
			}
		}
	}
	return out
}

func parsePositiveDuration(raw string) (time.Duration, error) {
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, err
	}
	if d < 0 {
		return 0, errors.New("timeout must be positive")
	}
	return d, nil
}
