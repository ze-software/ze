// Design: docs/architecture/api/architecture.md -- typed auto-load reasons, offline implied-endpoint evaluation, and compiled-bus implied/graph providers.
// Related: startup_autoload.go -- live auto-load selection feeding recordAutoLoadReasons.
// Related: bus_filter.go -- CompiledBus and CompiledParticipantFilter read by BuildImpliedParticipants.
// Related: delivery_graph.go -- runtime DeliveryGraph and InspectEdges.

package server

import (
	"maps"
	"runtime"
	"slices"
	"strings"
	"sync"
	"weak"

	"github.com/ze-software/ze/internal/component/config"
	plugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

const (
	maxConfigWalkDepth        = 32
	configSectionBus          = "bus"
	configSectionFamily       = "family"
	configListEndpoint        = "endpoint"
	configContainerConn       = "connection"
	configContainerRemote     = "remote"
	configLeafIP              = "ip"
	configKeywordDynamic      = "dynamic"
	reasonPrefixConfigSection = "config section "
	reasonPrefixRegistration  = "registration of "
	reasonPrefixInheritedFrom = "inherited from "
	reasonPrefixFeatureReq    = "feature requirement for "
	busPathSep                = "/"
)

type serverAutoLoadStore struct {
	mu      sync.RWMutex
	reasons map[string]AutoLoadReason
}

var (
	autoLoadStoresMu sync.Mutex
	autoLoadStores   = map[weak.Pointer[Server]]*serverAutoLoadStore{}
)

func cleanupServerAutoLoadStore(wp weak.Pointer[Server]) {
	autoLoadStoresMu.Lock()
	delete(autoLoadStores, wp)
	autoLoadStoresMu.Unlock()
}

func (s *Server) autoLoadStore(create bool) *serverAutoLoadStore {
	if s == nil {
		return nil
	}
	wp := weak.Make(s)
	autoLoadStoresMu.Lock()
	defer autoLoadStoresMu.Unlock()
	if store, ok := autoLoadStores[wp]; ok {
		return store
	}
	if !create {
		return nil
	}
	store := &serverAutoLoadStore{
		reasons: make(map[string]AutoLoadReason),
	}
	autoLoadStores[wp] = store
	runtime.AddCleanup(s, cleanupServerAutoLoadStore, wp)
	return store
}

// recordAutoLoadReasons stores the typed auto-load reason for each selected
// internal endpoint on s. Safe for concurrent use.
func (s *Server) recordAutoLoadReasons(entries []AutoLoadedEndpoint) {
	if s == nil {
		return
	}
	if len(entries) == 0 {
		return
	}
	store := s.autoLoadStore(true)
	store.mu.Lock()
	for i := range entries {
		name := entries[i].Config.Name
		if name == "" {
			continue
		}
		if entries[i].Reason.Kind == "" {
			continue
		}
		store.reasons[name] = entries[i].Reason
	}
	store.mu.Unlock()
}

// clearAutoLoadReason removes any recorded auto-load reason for name when the
// endpoint is unloaded or stopped. Safe for concurrent use.
func (s *Server) clearAutoLoadReason(name string) {
	if s == nil {
		return
	}
	if name == "" {
		return
	}
	store := s.autoLoadStore(false)
	if store == nil {
		return
	}
	store.mu.Lock()
	delete(store.reasons, name)
	store.mu.Unlock()
}

// AutoLoadReasonFor returns the recorded typed AutoLoadReason for name on s.
// Safe for concurrent use.
func (s *Server) AutoLoadReasonFor(name string) (AutoLoadReason, bool) {
	if s == nil {
		return AutoLoadReason{}, false
	}
	if name == "" {
		return AutoLoadReason{}, false
	}
	store := s.autoLoadStore(false)
	if store == nil {
		return AutoLoadReason{}, false
	}
	store.mu.RLock()
	reason, ok := store.reasons[name]
	store.mu.RUnlock()
	return reason, ok
}

// AutoLoadedEndpoints returns the deterministically sorted list of currently
// tracked auto-loaded endpoints on s as config.ImpliedEndpoint entries,
// excluding any endpoint explicitly configured in s.config.Plugins.
// Safe for concurrent use.
func (s *Server) AutoLoadedEndpoints() []config.ImpliedEndpoint {
	if s == nil {
		return nil
	}
	store := s.autoLoadStore(false)
	if store == nil {
		return nil
	}
	store.mu.RLock()
	names := slices.Sorted(maps.Keys(store.reasons))
	snapshot := make(map[string]AutoLoadReason, len(names))
	for _, name := range names {
		snapshot[name] = store.reasons[name]
	}
	store.mu.RUnlock()

	out := make([]config.ImpliedEndpoint, 0, len(names))
	for _, name := range names {
		if s.hasConfiguredPlugin(name) {
			continue
		}
		reason := snapshot[name]
		out = append(out, config.ImpliedEndpoint{
			Name:           name,
			CompiledPlugin: name,
			ReasonKind:     reason.Kind,
			ReasonSource:   reason.Source,
			ImpliedBy:      reason.ImpliedBy(),
		})
	}
	return out
}

func firstMatchingConfigRoot(pluginName string, pathSet map[string]bool) string {
	reg := registry.Lookup(pluginName)
	if reg == nil {
		return ""
	}
	for _, root := range reg.ConfigRoots {
		if pathSet[root] {
			return root
		}
	}
	if len(reg.ConfigRoots) > 0 {
		return reg.ConfigRoots[0]
	}
	return ""
}

func configNeedAutoLoadReason(owner string, need registry.ConfigNeed) AutoLoadReason {
	var srcBuf, detailBuf textbuf.Buffer
	pathSlash := srcBuf.Join(need.Path, busPathSep).Byte('=').Str(need.Value).String()
	detail := detailBuf.Str("config need ").Join(need.Path, " ").Byte(' ').Str(need.Value).
		Str(" of ").Str(owner).String()
	return AutoLoadReason{
		Kind:   AutoLoadReasonConfigNeed,
		Source: pathSlash,
		Detail: detail,
	}
}

// attributeResolvedReasons assigns a deterministic AutoLoadReason to every
// plugin in resolved by mirroring registry.ResolveDependencies' breadth-first
// expansion from needed and seedReasons.
func attributeResolvedReasons(
	needed []string,
	resolved []string,
	seedReasons map[string]AutoLoadReason,
	needHolds registry.NeedHolds,
) map[string]AutoLoadReason {
	reasons := make(map[string]AutoLoadReason, len(resolved))
	resolvedSet := make(map[string]bool, len(resolved))
	for _, name := range resolved {
		resolvedSet[name] = true
	}

	queue := make([]string, 0, len(resolved))
	enqueued := make(map[string]bool, len(resolved))
	for _, name := range needed {
		if r, ok := seedReasons[name]; ok {
			reasons[name] = r
		}
		if resolvedSet[name] && !enqueued[name] {
			enqueued[name] = true
			queue = append(queue, name)
		}
	}

	// Bounded by len(resolved): each resolved plugin enters queue at most once.
	for head := 0; head < len(queue); head++ {
		owner := queue[head]
		reg := registry.Lookup(owner)
		if reg == nil {
			continue
		}
		for _, dep := range reg.Dependencies {
			if !resolvedSet[dep] {
				continue
			}
			if _, exists := reasons[dep]; !exists {
				reasons[dep] = AutoLoadReason{
					Kind:   AutoLoadReasonDependency,
					Source: owner,
					Detail: "dependency of " + owner,
				}
			}
			if !enqueued[dep] {
				enqueued[dep] = true
				queue = append(queue, dep)
			}
		}
		for _, dep := range reg.OptionalDependencies {
			if !resolvedSet[dep] {
				continue
			}
			if _, exists := reasons[dep]; !exists {
				if need, matched := matchingHeldConfigNeed(reg, dep, needHolds); matched {
					reasons[dep] = configNeedAutoLoadReason(owner, need)
				} else {
					reasons[dep] = AutoLoadReason{
						Kind:   AutoLoadReasonDependency,
						Source: owner,
						Detail: "optional dependency of " + owner,
					}
				}
			}
			if !enqueued[dep] {
				enqueued[dep] = true
				queue = append(queue, dep)
			}
		}
	}

	return reasons
}

func matchingHeldConfigNeed(reg *registry.Registration, provider string, needHolds registry.NeedHolds) (registry.ConfigNeed, bool) {
	if reg == nil {
		return registry.ConfigNeed{}, false
	}
	if needHolds == nil {
		return registry.ConfigNeed{}, false
	}
	for _, need := range reg.ConfigNeeds {
		if !need.PullsProvider() {
			continue
		}
		if !slices.Contains(need.Providers, provider) {
			continue
		}
		if needHolds(reg.Name, need) {
			return need, true
		}
	}
	return registry.ConfigNeed{}, false
}

// enrichDormantReason appends the compiled sender plugin name and explicit
// bus > endpoint load line when a dormant reception's message is declared as a
// send by a compiled-in plugin that is not currently loaded (Q-11(c), M-4).
func enrichDormantReason(msg, baseReason string) string {
	if msg == "" {
		return baseReason
	}
	if strings.Contains(baseReason, "bus { endpoint ") {
		return baseReason
	}
	senders := compiledSendersForMessage(msg)
	if len(senders) == 0 {
		return baseReason
	}
	sender := senders[0]
	var tb textbuf.Buffer
	if baseReason != "" {
		tb.Str(baseReason).Str("; ")
	}
	tb.Str("load ").Str(sender).Str(" with: bus { endpoint ").Str(sender).
		Str(" { internal ").Str(sender).Str("; } }")
	return tb.String()
}

func compiledSendersForMessage(msg string) []string {
	var senders []string
	for _, reg := range registry.All() {
		for i := range reg.Sends {
			if reg.Sends[i].Message == msg {
				senders = append(senders, reg.Name)
				break
			}
		}
	}
	slices.Sort(senders)
	return senders
}

// EvaluateImpliedEndpoints evaluates the pure-config auto-load rules
// (ConfigRoots, DataPlane, Families, ConfigNeeds, and Dependencies) against
// tree, excluding any endpoint explicitly written under bus > endpoint.
func EvaluateImpliedEndpoints(tree *config.Tree) []config.ImpliedEndpoint {
	if tree == nil {
		return nil
	}

	explicitNames, explicitCompiled := collectExplicitEndpointSets(tree)
	treeMap := tree.ToMap()
	needHolds := config.ConfigNeedHolds(treeMap)

	var needed []string
	seen := make(map[string]bool)
	seedReasons := make(map[string]AutoLoadReason)

	addNeeded := func(name string, reason AutoLoadReason) {
		if name == "" {
			return
		}
		if explicitNames[name] {
			return
		}
		if explicitCompiled[name] {
			return
		}
		if seen[name] {
			return
		}
		seen[name] = true
		seedReasons[name] = reason
		needed = append(needed, name)
	}

	// Rule 1: ConfigRoots (config-path).
	paths := config.CollectContainerPaths(tree)
	pathSet := make(map[string]bool, len(paths))
	for _, p := range paths {
		pathSet[p] = true
	}
	for _, pluginName := range registry.PluginsForConfigPaths(paths) {
		matchedRoot := firstMatchingConfigRoot(pluginName, pathSet)
		addNeeded(pluginName, AutoLoadReason{
			Kind:   AutoLoadReasonConfigPath,
			Source: matchedRoot,
			Detail: reasonPrefixConfigSection + matchedRoot,
		})
	}

	// Rule 2: DataPlane writer (data-plane).
	appendOfflineDataPlaneWriter(&needed, seen, seedReasons, explicitNames, explicitCompiled, treeMap)

	// Rule 3: Families (family).
	for _, fam := range collectConfiguredFamiliesFromTree(tree) {
		pluginName := plugin.GetPluginForFamily(fam)
		if pluginName == "" {
			continue
		}
		addNeeded(pluginName, AutoLoadReason{
			Kind:   AutoLoadReasonFamily,
			Source: fam,
			Detail: "address family " + fam,
		})
	}

	// Rule 4a: Held ConfigNeeds of explicitly written compiled endpoints.
	if needHolds != nil {
		for _, expName := range slices.Sorted(maps.Keys(explicitCompiled)) {
			reg := registry.Lookup(expName)
			if reg == nil {
				continue
			}
			for _, need := range reg.ConfigNeeds {
				if !need.PullsProvider() {
					continue
				}
				if !needHolds(reg.Name, need) {
					continue
				}
				for _, provider := range need.Providers {
					if registry.Lookup(provider) == nil {
						continue
					}
					addNeeded(provider, configNeedAutoLoadReason(reg.Name, need))
				}
			}
		}
	}

	if len(needed) == 0 {
		return nil
	}

	// Rule 4b: Hard and pulled optional Dependencies / ConfigNeeds.
	resolved := needed
	if needHolds != nil {
		if expanded, err := registry.ResolveDependencies(needed, nil, needHolds); err == nil {
			resolved = expanded
		}
	}

	reasons := attributeResolvedReasons(needed, resolved, seedReasons, needHolds)

	out := make([]config.ImpliedEndpoint, 0, len(resolved))
	for _, name := range resolved {
		if explicitNames[name] {
			continue
		}
		if explicitCompiled[name] {
			continue
		}
		reason := reasons[name]
		out = append(out, config.ImpliedEndpoint{
			Name:           name,
			CompiledPlugin: name,
			ReasonKind:     reason.Kind,
			ReasonSource:   reason.Source,
			ImpliedBy:      reason.ImpliedBy(),
		})
	}
	slices.SortFunc(out, func(a, b config.ImpliedEndpoint) int {
		return strings.Compare(a.Name, b.Name)
	})
	return out
}

func collectExplicitEndpointSets(tree *config.Tree) (names, compiled map[string]bool) {
	names = make(map[string]bool)
	compiled = make(map[string]bool)
	if tree == nil {
		return names, compiled
	}
	if eps, err := config.ExtractPluginsFromTree(tree); err == nil {
		for i := range eps {
			names[eps[i].Name] = true
			if id := eps[i].CompiledIdentity(); id != "" {
				compiled[id] = true
			}
		}
		return names, compiled
	}
	busContainer := tree.GetContainer(configSectionBus)
	if busContainer == nil {
		return names, compiled
	}
	for _, entry := range busContainer.GetListOrdered(configListEndpoint) {
		names[entry.Key] = true
		if internalVal, ok := entry.Value.Get("internal"); ok && internalVal != "" {
			compiled[strings.TrimPrefix(internalVal, "ze.")] = true
		}
	}
	return names, compiled
}

func appendOfflineDataPlaneWriter(
	needed *[]string,
	seen map[string]bool,
	seedReasons map[string]AutoLoadReason,
	explicitNames map[string]bool,
	explicitCompiled map[string]bool,
	treeMap map[string]any,
) {
	producers := make([]string, 0, len(*needed)+len(explicitCompiled))
	producers = append(producers, *needed...)
	producers = append(producers, slices.Sorted(maps.Keys(explicitCompiled))...)
	producerIndex := slices.IndexFunc(producers, registry.NeedsDataPlane)
	if producerIndex < 0 {
		return
	}
	producer := producers[producerIndex]

	for _, name := range *needed {
		if reg := registry.Lookup(name); reg != nil && reg.ProgramsFIB {
			return
		}
	}
	for name := range explicitNames {
		if reg := registry.Lookup(name); reg != nil && reg.ProgramsFIB {
			return
		}
	}
	for name := range explicitCompiled {
		if reg := registry.Lookup(name); reg != nil && reg.ProgramsFIB {
			return
		}
	}

	dp := ConfigDataPlane(treeMap)
	if dp == "" {
		return
	}
	writer, err := registry.DataPlaneWriter(producer, dp)
	// refuse-by-default: guards appendOfflineDataPlaneWriter -- offline implied display omits a writer when the plane cannot be resolved
	if err != nil {
		return
	}
	if writer == "" {
		return
	}
	if explicitNames[writer] {
		return
	}
	if explicitCompiled[writer] {
		return
	}
	if seen[writer] {
		return
	}
	seen[writer] = true
	var tb textbuf.Buffer
	seedReasons[writer] = AutoLoadReason{
		Kind:   AutoLoadReasonDataPlane,
		Source: dp,
		Detail: tb.Str("data plane ").Str(dp).Str(" for ").Str(producer).String(),
	}
	*needed = append(*needed, writer)
}

func collectConfiguredFamiliesFromTree(tree *config.Tree) []string {
	if tree == nil {
		return nil
	}
	seen := make(map[string]bool)
	var out []string
	walkTreeForFamilies(tree, 0, seen, &out)
	slices.Sort(out)
	return out
}

// walkTreeForFamilies descends into containers and list entries bounded by
// maxConfigWalkDepth to collect configured address families recognized by
// plugin.GetPluginForFamily.
func walkTreeForFamilies(cur *config.Tree, depth int, seen map[string]bool, out *[]string) {
	if cur == nil {
		return
	}
	if depth >= maxConfigWalkDepth {
		return
	}
	for _, name := range cur.ContainerNames() {
		sub := cur.GetContainer(name)
		if sub == nil {
			continue
		}
		if name == configSectionFamily {
			collectFamilyContainer(sub, seen, out)
			continue
		}
		walkTreeForFamilies(sub, depth+1, seen, out)
	}
	for _, listName := range cur.ListNames() {
		for _, entry := range cur.GetListOrdered(listName) {
			walkTreeForFamilies(entry.Value, depth+1, seen, out)
		}
	}
}

func collectFamilyContainer(famTree *config.Tree, seen map[string]bool, out *[]string) {
	record := func(fam string) {
		if fam == "" {
			return
		}
		if seen[fam] {
			return
		}
		if plugin.GetPluginForFamily(fam) == "" {
			return
		}
		seen[fam] = true
		*out = append(*out, fam)
	}

	for _, child := range famTree.ContainerNames() {
		record(child)
		sub := famTree.GetContainer(child)
		if sub == nil {
			continue
		}
		for _, subName := range sub.ContainerNames() {
			var tb textbuf.Buffer
			record(tb.Str(child).Byte('/').Str(subName).String())
		}
		for _, leafKey := range sub.Values() {
			var tb textbuf.Buffer
			record(tb.Str(child).Byte('/').Str(leafKey).String())
		}
	}
	for _, listName := range famTree.ListNames() {
		record(listName)
		for _, entry := range famTree.GetListOrdered(listName) {
			record(entry.Key)
			var tb textbuf.Buffer
			record(tb.Str(listName).Byte('/').Str(entry.Key).String())
		}
	}
	for _, key := range famTree.Values() {
		record(key)
		if val, ok := famTree.Get(key); ok {
			record(val)
		}
	}
	for _, key := range famTree.MultiValueNames() {
		record(key)
		for _, v := range famTree.GetSlice(key) {
			record(v)
		}
	}
}

// BuildImpliedParticipants projects every participant filter in compiled into
// config.ImpliedParticipant entries carrying exact origins, producing lines,
// deduplicated covered targets, replaced registered defaults, effective tags,
// and dormancy explanations.
func BuildImpliedParticipants(compiled *CompiledBus) []config.ImpliedParticipant {
	if compiled == nil {
		return nil
	}

	type participantMsgKey struct {
		path string
		msg  string
	}
	requiredSend := make(map[participantMsgKey]bool)
	requiredRecv := make(map[participantMsgKey]bool)
	for _, edge := range compiled.Edges() {
		if edge.Origin != BusOriginRequired {
			continue
		}
		if edge.SenderPath != "" {
			requiredSend[participantMsgKey{path: edge.SenderPath, msg: edge.Message}] = true
		}
		if edge.ReceiverPath != "" {
			requiredRecv[participantMsgKey{path: edge.ReceiverPath, msg: edge.Message}] = true
		}
	}

	paths := compiled.Participants()
	out := make([]config.ImpliedParticipant, 0, len(paths))
	for _, path := range paths {
		f := compiled.Filter(path)
		if f == nil {
			continue
		}
		if len(f.SendRules) == 0 && len(f.ReceiveRules) == 0 && !f.SendAllowNone && !f.ReceiveAllowNone {
			continue
		}

		label := participantLabel(f)
		dormantByMsg := make(map[string]string, len(f.DormantReceptions))
		for i := range f.DormantReceptions {
			dr := &f.DormantReceptions[i]
			dormantByMsg[dr.Message] = dr.Reason
		}

		ip := config.ImpliedParticipant{
			Path:               f.Path,
			EndpointName:       f.EndpointName,
			ParentPath:         f.ParentPath,
			SendAllow:          slices.Clone(f.AllowedSendMessages),
			SendAllowNone:      f.SendAllowNone,
			SendAllowOrigin:    impliedOriginFromBus(f.SendAllowOrigin.Origin),
			SendAllowSource:    formatAllowSource(f.SendAllowOrigin, busDirSend, label, f.ParentPath),
			ReceiveAllow:       slices.Clone(f.AllowedReceiveMessages),
			ReceiveAllowNone:   f.ReceiveAllowNone,
			ReceiveAllowOrigin: impliedOriginFromBus(f.ReceiveAllowOrigin.Origin),
			ReceiveAllowSource: formatAllowSource(f.ReceiveAllowOrigin, busDirReceive, label, f.ParentPath),
		}

		for _, msg := range slices.Sorted(maps.Keys(f.ReceiveRules)) {
			r := f.ReceiveRules[msg]
			if r == nil {
				continue
			}
			isRequired := requiredRecv[participantMsgKey{path: f.Path, msg: msg}]
			ip.Receive = append(ip.Receive, buildImpliedReceiveRule(f, r, label, isRequired, dormantByMsg[msg]))
		}

		for _, msg := range slices.Sorted(maps.Keys(f.SendRules)) {
			r := f.SendRules[msg]
			if r == nil {
				continue
			}
			isRequired := requiredSend[participantMsgKey{path: f.Path, msg: msg}]
			ip.Send = append(ip.Send, buildImpliedSendRule(f, r, label, isRequired))
		}

		out = append(out, ip)
	}
	return out
}

func buildImpliedReceiveRule(
	f *CompiledParticipantFilter,
	r *BusCompiledReceiveRule,
	label string,
	isRequired bool,
	recordedDormant string,
) config.ImpliedDirectionRule {
	targets := uniqueSortedBusStrings(r.ResolvedPaths)
	dormant := r.Dormant
	dormantReason := recordedDormant
	if !dormant && r.Allowed && !r.FromNone && len(targets) == 0 {
		dormant = true
		if len(r.FromPaths) > 0 {
			var tb textbuf.Buffer
			dormantReason = tb.Str("no matching active sender for ").Join(r.FromPaths, ", ").String()
		} else {
			dormantReason = "no active sender for " + r.Message
		}
	}
	if dormant {
		if dormantReason == "" {
			dormantReason = "no active sender for " + r.Message
		}
		if len(r.FromPaths) == 0 {
			dormantReason = enrichDormantReason(r.Message, dormantReason)
		}
	}

	origin, source := resolveReceiveRuleOriginAndSource(r, label, f.ParentPath, isRequired, dormant)
	return config.ImpliedDirectionRule{
		Message:          r.Message,
		Allowed:          r.Allowed,
		Origin:           origin,
		Source:           source,
		Targets:          targets,
		ReplacedDefaults: slices.Clone(r.ReplacedDefaults),
		Tags:             cloneTagValueMap(r.InboundTagMap),
		Dormant:          dormant,
		DormantReason:    dormantReason,
	}
}

func buildImpliedSendRule(
	f *CompiledParticipantFilter,
	r *BusCompiledSendRule,
	label string,
	isRequired bool,
) config.ImpliedDirectionRule {
	targets := uniqueSortedBusStrings(r.ResolvedPaths)
	dormant := false
	dormantReason := ""
	isCLI := strings.HasPrefix(r.Message, busCLIPrefix)
	if !isCLI && r.Allowed && !r.ToNone && len(targets) == 0 && len(r.ToPaths) > 0 {
		dormant = true
		var tb textbuf.Buffer
		dormantReason = tb.Str("no matching active destination for ").Join(r.ToPaths, ", ").String()
	}

	origin, source := resolveSendRuleOriginAndSource(r, label, f.ParentPath, isRequired, dormant)
	return config.ImpliedDirectionRule{
		Message:          r.Message,
		Allowed:          r.Allowed,
		Origin:           origin,
		Source:           source,
		Targets:          targets,
		ReplacedDefaults: slices.Clone(r.ReplacedDefaults),
		Tags:             cloneTagValueMap(r.OutboundTagMap),
		Dormant:          dormant,
		DormantReason:    dormantReason,
	}
}

func resolveReceiveRuleOriginAndSource(
	r *BusCompiledReceiveRule,
	label, parentPath string,
	isRequired, dormant bool,
) (string, string) {
	if dormant {
		return config.ImpliedOriginDormant, formatDirectionRuleSource(r.FromOrigin, busDirReceive, busKeyFrom, label, parentPath)
	}
	if isRequired {
		return config.ImpliedOriginRequired, reasonPrefixFeatureReq + label
	}
	if r.FromOrigin.Origin != BusOriginRegistered {
		return impliedOriginFromBus(r.FromOrigin.Origin), formatDirectionRuleSource(r.FromOrigin, busDirReceive, busKeyFrom, label, parentPath)
	}
	if r.AllowOrigin.Origin != BusOriginRegistered {
		return impliedOriginFromBus(r.AllowOrigin.Origin), formatAllowSource(r.AllowOrigin, busDirReceive, label, parentPath)
	}
	for _, tagName := range slices.Sorted(maps.Keys(r.TagOrigins)) {
		to := r.TagOrigins[tagName]
		if to.Origin != BusOriginRegistered {
			return impliedOriginFromBus(to.Origin), formatTagRuleSource(to, busDirReceive, r.Message, tagName, label, parentPath)
		}
	}
	return config.ImpliedOriginRegistered, reasonPrefixRegistration + label
}

func resolveSendRuleOriginAndSource(
	r *BusCompiledSendRule,
	label, parentPath string,
	isRequired, dormant bool,
) (string, string) {
	if dormant {
		return config.ImpliedOriginDormant, formatDirectionRuleSource(r.ToOrigin, busDirSend, busKeyTo, label, parentPath)
	}
	if isRequired {
		return config.ImpliedOriginRequired, reasonPrefixFeatureReq + label
	}
	if r.ToOrigin.Origin != BusOriginRegistered {
		return impliedOriginFromBus(r.ToOrigin.Origin), formatDirectionRuleSource(r.ToOrigin, busDirSend, busKeyTo, label, parentPath)
	}
	if r.AllowOrigin.Origin != BusOriginRegistered {
		return impliedOriginFromBus(r.AllowOrigin.Origin), formatAllowSource(r.AllowOrigin, busDirSend, label, parentPath)
	}
	for _, tagName := range slices.Sorted(maps.Keys(r.TagOrigins)) {
		to := r.TagOrigins[tagName]
		if to.Origin != BusOriginRegistered {
			return impliedOriginFromBus(to.Origin), formatTagRuleSource(to, busDirSend, r.Message, tagName, label, parentPath)
		}
	}
	return config.ImpliedOriginRegistered, reasonPrefixRegistration + label
}

func participantLabel(f *CompiledParticipantFilter) string {
	if f.EndpointName != "" {
		return f.EndpointName
	}
	return f.Path
}

func impliedOriginFromBus(origin BusEdgeOrigin) string {
	switch origin {
	case BusOriginRegistered:
		return config.ImpliedOriginRegistered
	case BusOriginWritten:
		return config.ImpliedOriginWritten
	case BusOriginInherited:
		return config.ImpliedOriginInherited
	case BusOriginRequired:
		return config.ImpliedOriginRequired
	case BusOriginDormant:
		return config.ImpliedOriginDormant
	default:
		panic("BUG: server: invalid BusEdgeOrigin value")
	}
}

func formatAllowSource(origin BusRuleOrigin, dirName, label, parentPath string) string {
	switch origin.Origin {
	case BusOriginRegistered:
		return reasonPrefixRegistration + label
	case BusOriginInherited:
		if len(origin.Sources) > 0 {
			return reasonPrefixInheritedFrom + textbuf.Join(origin.Sources, ", ")
		}
		if parentPath != "" {
			return reasonPrefixInheritedFrom + parentPath
		}
		return config.ImpliedOriginInherited
	case BusOriginWritten:
		if len(origin.Sources) > 0 {
			var tb textbuf.Buffer
			return tb.Str(textbuf.Join(origin.Sources, ", ")).Str(busSepArrow).Str(dirName).Str(busSepArrow).Str(busKeyAllow).String()
		}
		return label
	case BusOriginRequired:
		return reasonPrefixFeatureReq + label
	case BusOriginDormant:
		return reasonPrefixRegistration + label
	default:
		panic("BUG: server: invalid BusEdgeOrigin value")
	}
}

func formatDirectionRuleSource(origin BusRuleOrigin, dirName, targetKey, label, parentPath string) string {
	switch origin.Origin {
	case BusOriginRegistered:
		return reasonPrefixRegistration + label
	case BusOriginInherited:
		if len(origin.Sources) > 0 {
			return reasonPrefixInheritedFrom + textbuf.Join(origin.Sources, ", ")
		}
		if parentPath != "" {
			return reasonPrefixInheritedFrom + parentPath
		}
		return config.ImpliedOriginInherited
	case BusOriginWritten:
		if len(origin.Sources) > 0 {
			var tb textbuf.Buffer
			return tb.Str(textbuf.Join(origin.Sources, ", ")).Str(busSepArrow).Str(dirName).Str(busSepArrow).Str(targetKey).String()
		}
		return label
	case BusOriginRequired:
		return reasonPrefixFeatureReq + label
	case BusOriginDormant:
		return reasonPrefixRegistration + label
	default:
		panic("BUG: server: invalid BusEdgeOrigin value")
	}
}

func formatTagRuleSource(origin BusRuleOrigin, dirName, msg, tagName, label, parentPath string) string {
	switch origin.Origin {
	case BusOriginRegistered:
		return reasonPrefixRegistration + label
	case BusOriginInherited:
		if len(origin.Sources) > 0 {
			return reasonPrefixInheritedFrom + textbuf.Join(origin.Sources, ", ")
		}
		if parentPath != "" {
			return reasonPrefixInheritedFrom + parentPath
		}
		return config.ImpliedOriginInherited
	case BusOriginWritten:
		if len(origin.Sources) > 0 {
			var tb textbuf.Buffer
			return tb.Str(textbuf.Join(origin.Sources, ", ")).Str(busSepArrow).Str(dirName).
				Str(busSepArrow).Str(busLabelMsg).Str(msg).Str(busSepArrow).Str(busLabelTag).Str(tagName).String()
		}
		return label
	case BusOriginRequired:
		return reasonPrefixFeatureReq + label
	case BusOriginDormant:
		return reasonPrefixRegistration + label
	default:
		panic("BUG: server: invalid BusEdgeOrigin value")
	}
}

// provideImpliedBus implements config.ImpliedBusProvider using live daemon
// state when provided, or pure-config offline evaluation and bus compilation
// when running offline.
func provideImpliedBus(tree *config.Tree, opts config.ImpliedOptions) ([]config.ImpliedEndpoint, []config.ImpliedParticipant, error) {
	if !opts.Offline {
		if compiled, ok := opts.LiveCompiled.(*CompiledBus); ok && compiled != nil {
			return slices.Clone(opts.LiveEndpoints), BuildImpliedParticipants(compiled), nil
		}
	}
	if tree == nil {
		return slices.Clone(opts.LiveEndpoints), nil, nil
	}
	endpoints := EvaluateImpliedEndpoints(tree)
	compiled, err := CompileOfflineBusFromTree(tree, endpoints)
	if err != nil {
		return nil, nil, err
	}
	return endpoints, BuildImpliedParticipants(compiled), nil
}

// provideGraphBusEdges implements config.GraphBusEdgeProvider by compiling the
// same offline bus graph from tree and projecting every delivery edge with its
// compiled origin into config.GraphEdge entries.
func provideGraphBusEdges(tree *config.Tree) []config.GraphEdge {
	if tree == nil {
		return nil
	}
	endpoints := EvaluateImpliedEndpoints(tree)
	compiled, err := CompileOfflineBusFromTree(tree, endpoints)
	var out []config.GraphEdge
	seenPair := make(map[string]bool)

	addEdge := func(from, to, origin string) {
		if from == "" {
			return
		}
		if to == "" {
			return
		}
		if from == to {
			return
		}
		var kb textbuf.Buffer
		pairKey := kb.Str(from).Byte(0).Str(to).String()
		seenPair[pairKey] = true
		out = append(out, config.GraphEdge{
			From:   from,
			To:     to,
			Kind:   config.EdgeProcessBinds,
			Origin: origin,
		})
	}

	if err == nil && compiled != nil {
		for _, edge := range compiled.Edges() {
			fromID := graphNodeIDForOfflineBusPath(edge.SenderPath)
			toID := graphNodeIDForOfflineBusPath(edge.ReceiverPath)
			addEdge(fromID, toID, impliedOriginFromBus(edge.Origin))
		}
	}

	// Preserve written target edges referencing endpoints without recorded
	// declarations (such as offline graphs built before external query mode).
	appendWrittenGraphEdges(tree, seenPair, addEdge)
	return out
}

func appendWrittenGraphEdges(tree *config.Tree, seenPair map[string]bool, addEdge func(from, to, origin string)) {
	emitBlock := func(srcID string, block plugin.BusBlock) {
		if srcID == "" {
			return
		}
		for _, target := range collectOfflineBlockTargets(block) {
			dstID := graphNodeIDForOfflineBusPath(target)
			if dstID == "" {
				continue
			}
			if dstID == srcID {
				continue
			}
			var kb textbuf.Buffer
			pairKey := kb.Str(srcID).Byte(0).Str(dstID).String()
			if seenPair[pairKey] {
				continue
			}
			addEdge(srcID, dstID, config.ImpliedOriginWritten)
		}
	}

	if busTree := tree.GetContainer(configSectionBus); busTree != nil {
		if busCfg, err := plugin.ParseBusConfigFromMap(busTree.ToMap()); err == nil {
			for _, epName := range slices.Sorted(maps.Keys(busCfg.Endpoints)) {
				emitBlock("plugin/"+epName, busCfg.Endpoints[epName])
			}
			for _, pPath := range slices.Sorted(maps.Keys(busCfg.Participants)) {
				emitBlock(graphNodeIDForOfflineBusPath(pPath), busCfg.Participants[pPath].Block)
			}
		}
	}

	for _, tmpl := range collectOfflineNodeTemplates() {
		var matches []offlineWalkedNode
		walkOfflineParticipantNodes(tree, tmpl.segments, 0, "", false, &matches)
		for _, m := range matches {
			busSub := m.tree.GetContainer(configSectionBus)
			if busSub == nil {
				continue
			}
			block, err := plugin.ParseBusBlockFromMap(busSub.ToMap(), m.path+" > bus")
			if err != nil {
				continue
			}
			emitBlock(graphNodeIDForOfflineBusPath(m.path), block)
		}
	}
}

func collectOfflineBlockTargets(block plugin.BusBlock) []string {
	var out []string
	appendDir := func(dir plugin.BusDirectionBlock) {
		if dir.TargetsPresent && !dir.TargetsNone {
			out = append(out, dir.Targets...)
		}
		for _, msg := range slices.Sorted(maps.Keys(dir.Messages)) {
			rule := dir.Messages[msg]
			if rule != nil && rule.TargetsPresent && !rule.TargetsNone {
				out = append(out, rule.Targets...)
			}
		}
	}
	appendDir(block.Send)
	appendDir(block.Receive)
	return out
}

func graphNodeIDForOfflineBusPath(path string) string {
	if strings.HasPrefix(path, plugin.BusEndpointPathPrefix) {
		name := strings.TrimPrefix(path, plugin.BusEndpointPathPrefix)
		if name == "" {
			return ""
		}
		var tb textbuf.Buffer
		return tb.Str("plugin/").Str(name).String()
	}
	segs := plugin.PathSegments(path)
	if len(segs) == 0 {
		return ""
	}
	var tb textbuf.Buffer
	if len(segs) == 1 {
		return tb.Str("section/").Str(segs[0]).String()
	}
	return tb.Str(segs[len(segs)-2]).Byte('/').Str(segs[len(segs)-1]).String()
}

type offlineNodeTemplate struct {
	ownerKind       string
	segments        []string
	secondaryPrefix string
	sends           []rpc.MessageSendDecl
	receives        []rpc.MessageReceiveDecl
	isContainer     bool
}

type offlineWalkedNode struct {
	path       string
	tree       *config.Tree
	hasListKey bool
}

// CompileOfflineBusFromTree builds an offline CompiledBus from tree,
// compiled-in registrations, recorded external declarations, and the given
// implied endpoints.
func CompileOfflineBusFromTree(tree *config.Tree, impliedEndpoints []config.ImpliedEndpoint) (*CompiledBus, error) {
	if tree == nil {
		return CompileBus(NewTagInterner(), nil)
	}

	var busCfg plugin.BusConfig
	if busTree := tree.GetContainer(configSectionBus); busTree != nil {
		parsed, err := plugin.ParseBusConfigFromMap(busTree.ToMap())
		if err != nil {
			return nil, err
		}
		busCfg = parsed
	}

	explicitEndpoints, err := config.ExtractPluginsFromTree(tree)
	if err != nil {
		return nil, err
	}

	extDeclByName := make(map[string]config.ExternalBusDecl)
	for _, ext := range config.ExternalBusDeclarations() {
		extDeclByName[ext.Endpoint] = ext
	}

	byPath := make(map[string]*BusParticipantInput)
	singletonByKind := make(map[string]string)

	// 1. Discover participant nodes from compiled-in and external templates.
	templates := collectOfflineNodeTemplates()
	for i := range templates {
		tmpl := &templates[i]
		var matches []offlineWalkedNode
		walkOfflineParticipantNodes(tree, tmpl.segments, 0, "", false, &matches)
		for _, m := range matches {
			var ownBlock plugin.BusBlock
			source := m.path + " > bus"
			if busSub := m.tree.GetContainer(configSectionBus); busSub != nil {
				parsed, parseErr := plugin.ParseBusBlockFromMap(busSub.ToMap(), source)
				if parseErr != nil {
					return nil, parseErr
				}
				ownBlock = parsed
			}

			var secondary []string
			if tmpl.secondaryPrefix != "" {
				if ipVal, ok := extractOfflineRemoteIP(m.tree); ok {
					var tb textbuf.Buffer
					ipPath := tb.Str(strings.TrimSuffix(tmpl.secondaryPrefix, busPathSep)).Byte('/').Str(ipVal).String()
					if ipPath != m.path {
						secondary = []string{ipPath}
					}
				}
			}

			var sends []rpc.MessageSendDecl
			var receives []rpc.MessageReceiveDecl
			if !tmpl.isContainer {
				sends = tmpl.sends
				receives = tmpl.receives
			}

			input := &BusParticipantInput{
				Path:           m.path,
				OwnerEndpoint:  tmpl.ownerKind,
				SecondaryPaths: secondary,
				Sends:          sends,
				Receives:       receives,
				OwnBlock:       ownBlock,
				OwnOrigin:      BusOriginWritten,
				OwnSource:      source,
				PureContainer:  tmpl.isContainer,
			}
			byPath[m.path] = input
			if !m.hasListKey && !tmpl.isContainer {
				singletonByKind[tmpl.ownerKind] = m.path
			}
		}
	}

	// 2. Link parent containers among discovered participant paths.
	discoveredPaths := slices.Sorted(maps.Keys(byPath))
	for _, pPath := range discoveredPaths {
		bestParent := ""
		for _, cand := range discoveredPaths {
			if cand == pPath {
				continue
			}
			if !plugin.PathCovers(cand, pPath) {
				continue
			}
			if len(cand) > len(bestParent) {
				bestParent = cand
			}
		}
		byPath[pPath].ParentPath = bestParent
	}

	// 3. Add explicit endpoints from bus > endpoint.
	seenEndpoint := make(map[string]bool)
	for i := range explicitEndpoints {
		ep := &explicitEndpoints[i]
		seenEndpoint[ep.Name] = true
		kind := ep.Name
		var sends []rpc.MessageSendDecl
		var receives []rpc.MessageReceiveDecl
		if ep.IsCompiled() {
			kind = ep.CompiledIdentity()
			if reg := registry.Lookup(kind); reg != nil {
				sends = reg.Sends
				receives = reg.Receives
			}
		} else if ext, ok := extDeclByName[ep.Name]; ok {
			sends = ext.Sends
			receives = ext.Receives
		}

		epPath := plugin.EndpointPath(ep.Name)
		if singletonPath, ok := singletonByKind[kind]; ok {
			p := byPath[singletonPath]
			p.EndpointName = ep.Name
			if !slices.Contains(p.SecondaryPaths, epPath) {
				p.SecondaryPaths = append(p.SecondaryPaths, epPath)
			}
			applyEndpointBusConfig(p, busCfg)
			continue
		}

		input := &BusParticipantInput{
			Path:         epPath,
			EndpointName: ep.Name,
			Sends:        sends,
			Receives:     receives,
		}
		applyEndpointBusConfig(input, busCfg)
		byPath[epPath] = input
	}

	// 4. Add implied auto-loaded endpoints.
	for i := range impliedEndpoints {
		imp := &impliedEndpoints[i]
		if seenEndpoint[imp.Name] {
			continue
		}
		seenEndpoint[imp.Name] = true
		regName := imp.CompiledPlugin
		if regName == "" {
			regName = imp.Name
		}
		reg := registry.Lookup(regName)
		if reg == nil {
			continue
		}
		epPath := plugin.EndpointPath(imp.Name)
		if singletonPath, ok := singletonByKind[reg.Name]; ok {
			p := byPath[singletonPath]
			p.EndpointName = imp.Name
			if !slices.Contains(p.SecondaryPaths, epPath) {
				p.SecondaryPaths = append(p.SecondaryPaths, epPath)
			}
			applyEndpointBusConfig(p, busCfg)
			continue
		}
		// Skip multi-instance protocol owners whose nodes were already
		// instantiated above; they do not double-register at /bus/endpoint/.
		if len(reg.ParticipantNodes) > 1 {
			continue
		}
		canonicalPath := epPath
		var secondary []string
		if single := singletonParticipantPath(reg.ParticipantNodes); single != "" && single != epPath {
			canonicalPath = single
			secondary = []string{epPath}
		}
		if _, exists := byPath[canonicalPath]; exists {
			continue
		}
		input := &BusParticipantInput{
			Path:           canonicalPath,
			EndpointName:   imp.Name,
			SecondaryPaths: secondary,
			Sends:          reg.Sends,
			Receives:       reg.Receives,
		}
		applyEndpointBusConfig(input, busCfg)
		byPath[canonicalPath] = input
	}

	// 5. Apply central bus > participant overrides and ensure parent containers exist.
	parentContainers := make(map[string]bool)
	for _, p := range byPath {
		applyCentralOverrideToInput(p, busCfg, false)
		if p.ParentPath != "" {
			parentContainers[p.ParentPath] = true
		}
	}
	for _, parentPath := range slices.Sorted(maps.Keys(parentContainers)) {
		if _, exists := byPath[parentPath]; exists {
			continue
		}
		cInput := &BusParticipantInput{Path: parentPath, PureContainer: true}
		if ov, ok := busCfg.Participants[parentPath]; ok {
			cInput.BusSideBlock = ov.Block.Clone()
			cInput.BusSideSource = ov.Source
		}
		byPath[parentPath] = cInput
	}

	inputs := make([]BusParticipantInput, 0, len(byPath))
	for _, path := range slices.Sorted(maps.Keys(byPath)) {
		inputs = append(inputs, *byPath[path])
	}
	return CompileBus(NewTagInterner(), inputs)
}

func collectOfflineNodeTemplates() []offlineNodeTemplate {
	var templates []offlineNodeTemplate
	addNodes := func(owner string, nodes []rpc.ParticipantNodeDecl, sends []rpc.MessageSendDecl, receives []rpc.MessageReceiveDecl) {
		for i := range nodes {
			segs := stripOfflineSchemaPrefixes(nodes[i].SchemaPath)
			if len(segs) == 0 {
				continue
			}
			templates = append(templates, offlineNodeTemplate{
				ownerKind:       owner,
				segments:        segs,
				secondaryPrefix: nodes[i].SecondaryIPPath,
				sends:           sends,
				receives:        receives,
			})
		}
	}
	for _, reg := range registry.All() {
		addNodes(reg.Name, reg.ParticipantNodes, reg.Sends, reg.Receives)
	}
	for _, ext := range config.ExternalBusDeclarations() {
		addNodes(ext.Endpoint, ext.ParticipantNodes, ext.Sends, ext.Receives)
	}

	// Mark any template whose schema path is a strict prefix of another
	// template of the same owner as a pure container (inheritable config
	// holder, not an active message participant of its own).
	for i := range templates {
		for j := range templates {
			if i == j {
				continue
			}
			if templates[i].ownerKind != templates[j].ownerKind {
				continue
			}
			if isStrictSegmentPrefix(templates[i].segments, templates[j].segments) {
				templates[i].isContainer = true
				break
			}
		}
	}
	return templates
}

func isStrictSegmentPrefix(short, long []string) bool {
	if len(short) >= len(long) {
		return false
	}
	for i := range short {
		if short[i] != long[i] {
			return false
		}
	}
	return true
}

func stripOfflineSchemaPrefixes(schemaPath string) []string {
	trimmed := strings.TrimPrefix(schemaPath, busPathSep)
	if trimmed == "" {
		return nil
	}
	rawParts := strings.Split(trimmed, busPathSep)
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

// walkOfflineParticipantNodes walks cur along finite schema segments (bounded
// by len(segments)) and collects every concrete container or list entry.
func walkOfflineParticipantNodes(
	cur *config.Tree,
	segments []string,
	idx int,
	prefix string,
	hasListKey bool,
	out *[]offlineWalkedNode,
) {
	if cur == nil {
		return
	}
	if idx >= len(segments) {
		*out = append(*out, offlineWalkedNode{
			path:       prefix,
			tree:       cur,
			hasListKey: hasListKey,
		})
		return
	}
	seg := segments[idx]
	if sub := cur.GetContainer(seg); sub != nil {
		walkOfflineParticipantNodes(sub, segments, idx+1, prefix+busPathSep+seg, hasListKey, out)
	}
	for _, entry := range cur.GetListOrdered(seg) {
		walkOfflineParticipantNodes(entry.Value, segments, idx+1, prefix+busPathSep+seg+busPathSep+entry.Key, true, out)
	}
}

func extractOfflineRemoteIP(nodeTree *config.Tree) (string, bool) {
	if nodeTree == nil {
		return "", false
	}
	conn := nodeTree.GetContainer(configContainerConn)
	if conn == nil {
		return "", false
	}
	remote := conn.GetContainer(configContainerRemote)
	if remote == nil {
		return "", false
	}
	ipVal, ok := remote.Get(configLeafIP)
	if !ok {
		return "", false
	}
	if ipVal == "" {
		return "", false
	}
	if ipVal == configKeywordDynamic {
		return "", false
	}
	return ipVal, true
}
