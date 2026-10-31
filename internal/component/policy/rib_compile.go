// Design: docs/architecture/rib/unified-locrib.md -- who publishes the import policy snapshot
// Overview: rib.go -- the route evaluators and Loc-RIB attachments this compiles
// Related: register.go -- the policy plugin whose config transaction calls it
//
// The compiler of the Loc-RIB import chains. The policy plugin runs in the
// engine and reads the policy root and every root holding a Loc-RIB import
// attachment; its config transaction parses and checks each chain at verify,
// then compiles them into one ribpolicy.Snapshot at apply and publishes it.

package policy

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/configvalue"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	ribpolicy "github.com/ze-software/ze/internal/core/rib/policy"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// ribImportConfig is the verified config of every Loc-RIB import chain: plain
// data, so the config transaction's DeepEqual skips an apply that changes no
// chain and no instance a chain names. Its zero value configures no chain.
type ribImportConfig struct {
	chains []ribChainConfig
	// policy is the policy root a chain's instances resolve their references
	// in (a match naming a prefix-list), so a change to a referenced instance
	// also re-applies the chains.
	policy map[string]any
}

// ribChainConfig is one attachment's chain: its refs in order, the config of
// the instance each ref names (parallel to refs), and its final.
type ribChainConfig struct {
	imp       ribImport
	refs      []string
	instances []ribInstanceConfig
	final     Final
}

type ribInstanceConfig struct {
	filterType string
	config     map[string]any
}

// parseRIBImports is the policy plugin's sdk.ConfigParser. It reads every
// registered Loc-RIB import chain from its root and refuses one naming an
// unknown filter, a filter reading a field the attachment's routes do not
// carry, or a filter type with no route evaluator. A delivery holding neither
// the policy root nor any attachment root carries nothing for the plugin.
func parseRIBImports(sections []sdk.ConfigSection) (ribImportConfig, bool, error) {
	imports := ribReg.importList()
	subtrees := make(map[string]map[string]any, len(sections))
	present := false
	for _, section := range sections {
		if !readsRoot(imports, section.Root) {
			continue
		}
		subtree, ok := parseSubtree(section.Data, section.Root)
		if !ok {
			return ribImportConfig{}, false, fmt.Errorf("policy: the %s section is not a JSON object", section.Root)
		}
		subtrees[section.Root] = subtree
		present = true
	}
	if !present {
		return ribImportConfig{}, false, nil
	}

	policyCfg := subtrees[RootName]
	reg, err := registryFromPolicy(policyCfg)
	if err != nil {
		return ribImportConfig{}, false, err
	}
	cfg := ribImportConfig{policy: policyCfg}
	for _, imp := range imports {
		chain, configured, err := readRIBChain(imp, subtrees[imp.Root], reg, policyCfg)
		if err != nil {
			return ribImportConfig{}, false, err
		}
		if !configured {
			continue
		}
		if _, err := compileRIBChain(&chain, policyCfg); err != nil {
			return ribImportConfig{}, false, err
		}
		cfg.chains = append(cfg.chains, chain)
	}
	return cfg, true, nil
}

// installRIBImports is the policy plugin's sdk.ConfigInstaller: it compiles the
// verified chains and publishes the snapshot, which re-judges every Loc-RIB
// path (locrib.PublishPolicy). No chain publishes nil, the gate's zero-cost
// path; when nil is already published there is nothing to re-judge.
func installRIBImports(cfg ribImportConfig) error {
	snap, err := compileRIBSnapshot(cfg)
	if err != nil {
		return fmt.Errorf("policy: Loc-RIB import chains: %w", err)
	}
	if snap == nil && ribpolicy.Current() == nil {
		return nil
	}
	locrib.PublishPolicy(snap)
	logger().Info("Loc-RIB import policy published", "chains", len(cfg.chains))
	return nil
}

// compileRIBSnapshot compiles every chain into one snapshot, nil when no chain
// is configured.
func compileRIBSnapshot(cfg ribImportConfig) (*ribpolicy.Snapshot, error) {
	if len(cfg.chains) == 0 {
		return nil, nil //nolint:nilnil // nil is the published "no policy" snapshot, ribpolicy.Set
	}
	sources := make(map[redistevents.ProtocolID]ribpolicy.Chain)
	var central ribpolicy.Chain
	for i := range cfg.chains {
		chain := &cfg.chains[i]
		compiled, err := compileRIBChain(chain, cfg.policy)
		if err != nil {
			return nil, err
		}
		switch chain.imp.Scope {
		case RIBScopeSource:
			sources[chain.imp.protocolID] = compiled
		case RIBScopeCentral:
			central = compiled
		case RIBScopeUnspecified:
			return nil, errors.New("BUG: Loc-RIB import " + chain.imp.Attachment + " has no scope")
		}
	}
	return ribpolicy.NewSnapshot(sources, central)
}

// readsRoot answers whether the policy plugin reads root.
func readsRoot(imports []ribImport, root string) bool {
	if root == RootName {
		return true
	}
	for _, imp := range imports {
		if imp.Root == root {
			return true
		}
	}
	return false
}

// registryFromPolicy builds the filter instance namespace from the policy
// root's JSON: an entry of every filter type list is an instance.
func registryFromPolicy(policyCfg map[string]any) (*Registry, error) {
	types := registry.FilterTypesMap()
	for _, name := range vocab.declaredFilterTypes() {
		types[name] = ""
	}
	var instances []Instance
	for _, filterType := range slices.Sorted(maps.Keys(types)) {
		block, ok := policyCfg[filterType].(map[string]any)
		if !ok {
			continue
		}
		for _, name := range slices.Sorted(maps.Keys(block)) {
			instanceCfg, _ := block[name].(map[string]any) // an entry with no leaves is an empty instance
			instances = append(instances, Instance{Name: name, Type: filterType, Config: instanceCfg})
		}
	}
	return NewRegistry(instances...)
}

// readRIBChain reads `filter { import { filters [ ... ]; final ...; } }` from
// one attachment's root and checks every name it holds. configured is false
// when the root holds no import chain.
func readRIBChain(imp ribImport, rootCfg map[string]any, reg *Registry, policyCfg map[string]any) (ribChainConfig, bool, error) {
	filterCfg, ok := rootCfg["filter"].(map[string]any)
	if !ok {
		return ribChainConfig{}, false, nil
	}
	importCfg, ok := filterCfg["import"].(map[string]any)
	if !ok {
		return ribChainConfig{}, false, nil
	}
	chain := ribChainConfig{imp: imp, refs: configvalue.LeafList(importCfg["filters"])}
	finalText, _ := importCfg["final"].(string)
	switch finalText {
	case "accept":
		chain.final = FinalAccept
	case "reject":
		chain.final = FinalReject
	default:
		return ribChainConfig{}, false, fmt.Errorf("%s/filter/import/final: missing or invalid final %q; "+
			"every configured policy requires a final decision; write import { filters [ NAME ]; final accept; } or final reject",
			imp.Root, finalText)
	}

	instances, err := checkRouteChain(imp.Attachment, chain.refs, reg, policyCfg)
	if err != nil {
		return ribChainConfig{}, false, err
	}
	chain.instances = instances
	return chain, true, nil
}

// checkRouteChain checks every name of one attachment's chain against the
// policy root and answers the config of the instance each name holds, parallel
// to refs. A chain judged in the engine runs only filters defined under the
// policy root: a `<plugin>:<filter>` reference names a plugin process, which
// the engine never calls.
func checkRouteChain(attachment string, refs []string, reg *Registry, policyCfg map[string]any) ([]ribInstanceConfig, error) {
	context := attachment + " filter"
	if !vocab.isRouteAttachment(attachment) {
		return nil, fmt.Errorf("BUG: policy: %s is not registered with MustRegisterRouteAttachment, "+
			"so its routes are not a ribpolicy.Route a chain can judge", attachment)
	}
	for _, name := range refs {
		if strings.Contains(name, ":") {
			return nil, fmt.Errorf("%s %q: a %s chain runs only filters defined under the policy root, "+
				"so a <plugin>:<filter> reference cannot judge its routes; name a policy filter instance instead",
				context, name, attachment)
		}
		if err := reg.ValidateName(name, context); err != nil {
			return nil, err
		}
	}
	if err := CheckChain(attachment, reg, refs); err != nil {
		return nil, err
	}
	instances := make([]ribInstanceConfig, 0, len(refs))
	for _, name := range refs {
		inst, _ := reg.Lookup(name) // ValidateName above proved the name is an instance
		instanceCfg, _ := policyCfg[inst.Type].(map[string]any)[name].(map[string]any)
		instances = append(instances, ribInstanceConfig{filterType: inst.Type, config: instanceCfg})
	}
	return instances, nil
}

// CompileRouteChain checks and compiles the chain an attachment's config names,
// refs in order and final after the last, against the filter instances under
// the policy root (policyCfg, the root's own JSON object). It is the one compile
// path every engine-judged chain takes: the Loc-RIB import chains of this
// plugin, the redistribute import chains, and the IGP export chains. A name that
// is not an instance, a filter reading a field the attachment's routes do not
// carry, and a filter type with no route evaluator are each refused by name.
// The attachment MUST be registered (MustRegisterAttachment).
func CompileRouteChain(attachment string, refs []string, final Final, policyCfg map[string]any) (*RouteChain, error) {
	if final != FinalAccept && final != FinalReject {
		return nil, fmt.Errorf("%s: missing or invalid final; every configured policy requires a final decision; "+
			"write filters [ NAME ]; final accept; or final reject", attachment)
	}
	reg, err := registryFromPolicy(policyCfg)
	if err != nil {
		return nil, err
	}
	instances, err := checkRouteChain(attachment, refs, reg, policyCfg)
	if err != nil {
		return nil, err
	}
	return compileRouteChain(attachment, refs, instances, final, policyCfg)
}

// compileRIBChain compiles one verified Loc-RIB import chain.
func compileRIBChain(chain *ribChainConfig, policyCfg map[string]any) (*RouteChain, error) {
	return compileRouteChain(chain.imp.Attachment, chain.refs, chain.instances, chain.final, policyCfg)
}

// compileRouteChain compiles one checked chain (checkRouteChain), instances
// parallel to refs. A filter type with no route evaluator is refused here,
// naming the types that have one.
func compileRouteChain(attachment string, refs []string, instances []ribInstanceConfig, final Final, policyCfg map[string]any) (*RouteChain, error) {
	compiled := &RouteChain{evaluators: make([]RouteEvaluator, len(refs)), final: final}
	resolve := func(name, filterType string) (RouteEvaluator, error) {
		instanceCfg, ok := policyCfg[filterType].(map[string]any)[name].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%q is not a %s defined under the %s root; define policy %s %s, or name an existing one",
				name, filterType, RootName, filterType, name)
		}
		return compileRIBEvaluator(filterType, name, instanceCfg, refuseNestedReference)
	}
	for i, name := range refs {
		filterType := instances[i].filterType
		evaluator, err := compileRIBEvaluator(filterType, name, instances[i].config, resolve)
		if err != nil {
			return nil, fmt.Errorf("%s filter %q (%s): %w", attachment, name, filterType, err)
		}
		compiled.evaluators[i] = evaluator
	}
	return compiled, nil
}

// compileRIBEvaluator compiles one instance with its type's route evaluator.
// A filter type with none is refused, naming the types that have one.
func compileRIBEvaluator(filterType, name string, instanceCfg map[string]any, resolve InstanceResolver) (RouteEvaluator, error) {
	compile, ok := ribReg.compiler(filterType)
	if !ok {
		return nil, fmt.Errorf("a %s filter cannot run in an engine-judged chain, which judges every route in the engine "+
			"and never calls a plugin process; remove it from this chain or use a filter of type %s",
			filterType, strings.Join(ribReg.evaluatorTypes(), ", "))
	}
	return compile(name, instanceCfg, resolve)
}

// refuseNestedReference is the resolver a referenced instance compiles with:
// references are one level deep (InstanceResolver).
func refuseNestedReference(name, filterType string) (RouteEvaluator, error) {
	return nil, fmt.Errorf("a referenced filter cannot reference another filter (%s %q); name it from the chain directly", filterType, name)
}

// RouteChain is one compiled engine-judged chain (CompileRouteChain): a Loc-RIB
// import chain, a redistribute import chain, an IGP export chain. It is a
// ribpolicy.Chain. Immutable after compile, so safe for concurrent use.
type RouteChain struct {
	evaluators []RouteEvaluator
	final      Final
}

var _ ribpolicy.Chain = (*RouteChain)(nil)

// Evaluate runs the chain with Ze chain semantics: a filter that does not
// reject passes the route on with its sets applied to the view the next filter
// reads, a reject stops the chain, and final decides after the last filter.
// It allocates nothing: the route is passed by value.
func (c *RouteChain) Evaluate(r ribpolicy.Route) ribpolicy.Verdict {
	var sets ribpolicy.Sets
	for _, evaluate := range c.evaluators {
		result := evaluate(r)
		if result.Reject {
			return ribpolicy.Verdict{Action: ribpolicy.ActionReject}
		}
		sets = sets.Override(result.Sets)
		// The next filter reads the route as the earlier ones left it.
		if result.Sets.HasTag {
			r.Tag, r.HasTag = result.Sets.Tag, true
		}
		if result.Sets.HasMetric {
			r.Metric = result.Sets.Metric
		}
	}
	switch c.final {
	case FinalAccept:
		return ribpolicy.Verdict{Action: ribpolicy.ActionAccept, Sets: sets}
	case FinalReject:
		return ribpolicy.Verdict{Action: ribpolicy.ActionReject}
	}
	// A chain is compiled only from a verified final; zero is a Ze defect,
	// and the gate fails closed on it.
	return ribpolicy.Verdict{Action: ribpolicy.ActionUnspecified}
}

func (r *ribRegistry) evaluatorTypes() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return slices.Sorted(maps.Keys(r.evaluators))
}
