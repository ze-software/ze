// Design: docs/guide/route-policy.md -- completing and checking filter names in a chain
// Related: rib_compile.go -- readRIBChain, the check a Loc-RIB import chain passes at verify
// Related: fields.go -- CheckChain, the field fit a chain member passes
// Related: registry.go -- BuildRegistry, the namespace the names come from
//
// A chain's `filters` leaf-list declares `ze:reference "policy-filter"`, so the
// editor completes it and commit judges it through the source below. The
// source finds the attachment the chain sits at from the leaf's data path, and
// asks that attachment's owner, member by member, the question its own load
// asks: a name is offered only when the owner would accept it there.

package policy

import (
	"fmt"
	"strings"
	"sync"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/yang"
)

// ReferenceSourceName is the ze:reference argument a chain's filters leaf
// declares.
const ReferenceSourceName = "policy-filter"

// ChainMemberCheck refuses one member of a chain at an attachment, the way
// the attachment's owner refuses it when the config loads. reg is the
// namespace of the config's policy root, and context names where the chain
// sits, for the refusal. It MUST be safe for concurrent use.
type ChainMemberCheck func(reg *Registry, member, context string) error

// chainSite is where a chain is configured: the config root it sits under
// and its direction, the container between `filter` and `filters`.
type chainSite struct {
	root      string
	direction string
}

// chainLeaf is the attachment a chain site fills, and its owner's member
// check: a registered one, or for a Loc-RIB import the import itself, whose
// check reads the policy root it is asked over (ribMemberCheck).
type chainLeaf struct {
	attachment string
	check      ChainMemberCheck
	rib        *ribImport
}

// chainLeaves holds the chain sites MustRegisterChainLeaf declared. Written
// from init(), read by completion and commit. Safe for concurrent use.
var chainLeaves = struct {
	mu     sync.RWMutex
	bySite map[chainSite]chainLeaf
}{bySite: make(map[chainSite]chainLeaf)}

// MustRegisterChainLeaf declares that every chain configured under root at
// `filter <direction> filters`, at any depth, attaches at attachment, and that
// check is the owner's refusal of one member. It is for an attachment whose
// chain is not a Loc-RIB import: a Loc-RIB import's site is its
// MustRegisterRIBImport root, and its check is readRIBChain's. Call it from
// the owner's init(), after MustRegisterAttachment. A second registration of
// one site is a Ze defect and panics.
func MustRegisterChainLeaf(root, direction, attachment string, check ChainMemberCheck) {
	if root == "" {
		panic("BUG: policy: chain leaf for " + attachment + " registered with no config root")
	}
	if direction == "" {
		panic("BUG: policy: chain leaf for " + attachment + " registered with no direction")
	}
	if check == nil {
		panic("BUG: policy: chain leaf for " + attachment + " registered with no member check")
	}
	vocab.mu.RLock()
	_, known := vocab.attachments[attachment]
	vocab.mu.RUnlock()
	if !known {
		panic("BUG: policy: chain leaf " + attachment + " registered before its attachment")
	}
	site := chainSite{root: root, direction: direction}
	chainLeaves.mu.Lock()
	defer chainLeaves.mu.Unlock()
	if _, dup := chainLeaves.bySite[site]; dup {
		panic("BUG: policy: chain leaf " + root + " filter " + direction + " registered twice")
	}
	chainLeaves.bySite[site] = chainLeaf{attachment: attachment, check: check}
}

// filterReference is the policy-filter reference source. It answers the
// filter instances the chain at `at` may hold: every instance its attachment's
// owner accepts there, its fields fitting what that attachment's routes carry.
// Commit asks the same owner check of each value, so a spelling the owner
// accepts but completion does not offer (a qualified `<type>:<name>`, an
// external plugin's `<plugin>:<filter>` on BGP) still commits.
func filterReference(root map[string]any, at []string) yang.ReferenceAnswer {
	leaf, err := chainLeafAt(at)
	if err != nil {
		return refusedAnswer(err)
	}
	policyCfg, _ := root[RootName].(map[string]any) // no policy root: a namespace with no instance
	reg, err := registryOfData(policyCfg)
	if err != nil {
		return refusedAnswer(err)
	}
	if leaf.rib != nil {
		leaf.check = ribMemberCheck(*leaf.rib, policyCfg)
	}
	context := strings.Join(at, " ")

	var names []string
	for _, name := range reg.Names() {
		if leaf.check(reg, name, context) == nil {
			names = append(names, name)
		}
	}
	return yang.ReferenceAnswer{
		Names: names,
		Check: func(value string) error { return leaf.check(reg, value, context) },
	}
}

// refusedAnswer offers nothing and refuses every value with err, for a chain
// whose attachment or namespace cannot be found.
func refusedAnswer(err error) yang.ReferenceAnswer {
	return yang.ReferenceAnswer{Check: func(string) error { return err }}
}

// chainLeafAt answers the attachment of the chain leaf at the data path at,
// `<root> ... filter <direction> filters`: a Loc-RIB import registered for
// root, else a site MustRegisterChainLeaf declared.
func chainLeafAt(at []string) (chainLeaf, error) {
	last := len(at) - 1
	if last < 3 {
		return chainLeaf{}, fmt.Errorf("policy: %q is not a filter chain: a chain is <root> filter <direction> filters",
			strings.Join(at, " "))
	}
	if at[last-2] != "filter" {
		return chainLeaf{}, fmt.Errorf("policy: %q is not a filter chain: a chain is <root> filter <direction> filters",
			strings.Join(at, " "))
	}
	site := chainSite{root: at[0], direction: at[last-1]}
	for _, imp := range ribReg.importList() {
		if imp.Root != site.root {
			continue
		}
		if site.direction != "import" {
			continue
		}
		return chainLeaf{attachment: imp.Attachment, rib: &imp}, nil
	}
	chainLeaves.mu.RLock()
	leaf, ok := chainLeaves.bySite[site]
	chainLeaves.mu.RUnlock()
	if !ok {
		return chainLeaf{}, fmt.Errorf("policy: no filter attachment is registered for %s filter %s, "+
			"so no name can be checked there: build ze with the component that owns %s",
			site.root, site.direction, site.root)
	}
	return leaf, nil
}

// ribFinalAccept is the `final accept` the one-member chain ribMemberCheck
// asks about carries: readRIBChain refuses a chain with no final, and the
// member check does not depend on which final it is.
const ribFinalAccept = "accept"

// ribMemberCheck is the check readRIBChain runs at verify over policyCfg,
// asked of a chain holding member alone: every member is judged on its own
// there too.
func ribMemberCheck(imp ribImport, policyCfg map[string]any) ChainMemberCheck {
	return func(reg *Registry, member, _ string) error {
		chainCfg := map[string]any{"filter": map[string]any{"import": map[string]any{
			"filters": []string{member},
			"final":   ribFinalAccept,
		}}}
		_, _, err := readRIBChain(imp, chainCfg, reg, policyCfg)
		return err
	}
}

// registryOfData builds the filter instance namespace from the policy root as
// Tree.ToPluginMap lowers it, through BuildRegistry, the builder a config load
// uses, so an entry of a list that is not a filter type is refused for what it
// is. policyCfg may be nil: the namespace then holds no instance.
func registryOfData(policyCfg map[string]any) (*Registry, error) {
	schema, err := config.YANGSchema()
	if err != nil {
		return nil, fmt.Errorf("policy registry: %w", err)
	}
	node, err := schema.Lookup(RootName)
	if err != nil {
		return nil, fmt.Errorf("policy registry: %w", err)
	}
	policySchema, ok := node.(*config.ContainerNode)
	if !ok {
		return nil, fmt.Errorf("policy registry: the %s schema node is not a container", RootName)
	}
	if policyCfg == nil {
		return BuildRegistry(nil, policySchema)
	}
	policyTree, err := config.TreeFromPluginMap(policyCfg)
	if err != nil {
		return nil, fmt.Errorf("policy registry: %w", err)
	}
	return BuildRegistry(policyTree, policySchema)
}
