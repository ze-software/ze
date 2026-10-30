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
	var cfg ribImportConfig
	for _, imp := range imports {
		chain, configured, err := readRIBChain(imp, subtrees[imp.Root], reg, policyCfg)
		if err != nil {
			return ribImportConfig{}, false, err
		}
		if !configured {
			continue
		}
		if _, err := compileRIBChain(&chain); err != nil {
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
		compiled, err := compileRIBChain(chain)
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
			instances = append(instances, Instance{Name: name, Type: filterType})
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

	context := imp.Attachment + " filter"
	for _, name := range chain.refs {
		if strings.Contains(name, ":") {
			return ribChainConfig{}, false, fmt.Errorf("%s %q: the Loc-RIB gate runs only filters defined under the policy root, "+
				"so a <plugin>:<filter> reference cannot judge %s routes; name a policy filter instance instead",
				context, name, imp.Attachment)
		}
		if err := reg.ValidateName(name, context); err != nil {
			return ribChainConfig{}, false, err
		}
	}
	if err := CheckChain(imp.Attachment, reg, chain.refs); err != nil {
		return ribChainConfig{}, false, err
	}
	for _, name := range chain.refs {
		inst, _ := reg.Lookup(name) // ValidateName above proved the name is an instance
		instanceCfg, _ := policyCfg[inst.Type].(map[string]any)[name].(map[string]any)
		chain.instances = append(chain.instances, ribInstanceConfig{filterType: inst.Type, config: instanceCfg})
	}
	return chain, true, nil
}

// compileRIBChain compiles one verified chain. A filter type with no route
// evaluator is refused here, naming the types that have one.
func compileRIBChain(chain *ribChainConfig) (*ribChain, error) {
	compiled := &ribChain{evaluators: make([]RouteEvaluator, len(chain.refs)), final: chain.final}
	for i, name := range chain.refs {
		filterType := chain.instances[i].filterType
		compile, ok := ribReg.compiler(filterType)
		if !ok {
			return nil, fmt.Errorf("%s filter %q (%s): a %s filter cannot run at the Loc-RIB gate, which judges every route in the engine "+
				"and never calls a plugin process; remove it from this chain or use a filter of type %s",
				chain.imp.Attachment, name, filterType, filterType, strings.Join(ribReg.evaluatorTypes(), ", "))
		}
		evaluator, err := compile(name, chain.instances[i].config)
		if err != nil {
			return nil, fmt.Errorf("%s filter %q (%s): %w", chain.imp.Attachment, name, filterType, err)
		}
		compiled.evaluators[i] = evaluator
	}
	return compiled, nil
}

// ribChain is one compiled Loc-RIB import chain. Immutable after compile, so
// safe for concurrent use.
type ribChain struct {
	evaluators []RouteEvaluator
	final      Final
}

// Evaluate runs the chain with Ze chain semantics: a filter that does not
// reject passes the route on with its sets applied to the view the next filter
// reads, a reject stops the chain, and final decides after the last filter.
// It allocates nothing: the route is passed by value.
func (c *ribChain) Evaluate(r ribpolicy.Route) ribpolicy.Verdict {
	var sets ribpolicy.Sets
	for _, evaluate := range c.evaluators {
		result := evaluate(r)
		if result.Reject {
			return ribpolicy.Verdict{Action: ribpolicy.ActionReject}
		}
		if result.Sets.HasDistance {
			sets.Distance, sets.HasDistance = result.Sets.Distance, true
		}
		if result.Sets.HasTag {
			sets.Tag, sets.HasTag = result.Sets.Tag, true
			r.Tag, r.HasTag = result.Sets.Tag, true
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
