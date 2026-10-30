package all

import (
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/component/policy"
)

// policyRoutingPlugin is policy routing's registration name. It owns
// `policy route`, the one list under the policy root that is not a filter type.
const policyRoutingPlugin = "policy-routes"

// TestFilterAttachmentOwnersDependOnPolicy: every plugin whose config holds a
// filter attachment, and every plugin that owns a filter type, lists policy in
// its hard Dependencies, so loading it loads the filter library (AC-18).
// Method: derive the owners from the registries (policy.AttachmentOwners, and
// each Registration.FilterTypes), never from a hand list, and check each edge.
func TestFilterAttachmentOwnersDependOnPolicy(t *testing.T) {
	owners := policy.AttachmentOwners()
	if !slices.Contains(owners, "rib") {
		t.Fatalf("the system RIB registers the rib import attachment, so it must be an owner: %v", owners)
	}
	for _, reg := range registry.All() {
		if len(reg.FilterTypes) > 0 {
			owners = append(owners, reg.Name)
		}
	}
	for _, owner := range owners {
		reg := registry.Lookup(owner)
		if reg == nil {
			t.Errorf("attachment owner %q is not a registered plugin", owner)
			continue
		}
		if !slices.Contains(reg.Dependencies, policy.PluginName) {
			t.Errorf("plugin %q holds a filter attachment or owns a filter type but does not list %q in Dependencies: %v",
				owner, policy.PluginName, reg.Dependencies)
		}
	}
}

// policyTree builds `policy { <list> <entry> { } }`.
func policyTree(list, entry string) *config.Tree {
	root := config.NewTree()
	block := config.NewTree()
	block.AddListEntry(list, entry, config.NewTree())
	root.SetContainer(policy.RootName, block)
	return root
}

// TestPolicyOnlyConfigAutoLoadsPolicy: a config holding only a policy block
// auto-loads the policy plugin, and only `policy route` auto-loads policy
// routing (AC-18). Method: collect each config's paths with the producer boot
// auto-load uses and ask registry.PluginsForConfigPaths which plugins they
// select.
func TestPolicyOnlyConfigAutoLoadsPolicy(t *testing.T) {
	selected := registry.PluginsForConfigPaths(config.CollectContainerPaths(policyTree("prefix-list", "CUSTOMERS")))
	if !slices.Contains(selected, policy.PluginName) {
		t.Fatalf("a policy-only config must auto-load %q: %v", policy.PluginName, selected)
	}
	if slices.Contains(selected, policyRoutingPlugin) {
		t.Fatalf("a policy block holding only a prefix-list must not auto-load policy routing: %v", selected)
	}

	if registry.Lookup(policyRoutingPlugin) == nil {
		return // a build without policy routing (ze_policyroute) has no route list to load it
	}
	selected = registry.PluginsForConfigPaths(config.CollectContainerPaths(policyTree("route", "R")))
	if !slices.Contains(selected, policyRoutingPlugin) {
		t.Fatalf("policy route R must auto-load policy routing: %v", selected)
	}
}

// TestProtocolLoadsPolicyAsDependency: a config with only an isis block loads
// policy as a dependency of isis (AC-18). Method: resolve isis's dependencies
// as startup does; skipped in a build without IS-IS (ze_isis).
func TestProtocolLoadsPolicyAsDependency(t *testing.T) {
	if registry.Lookup("isis") == nil {
		t.Skip("this build carries no IS-IS (ze_isis)")
	}
	names, err := registry.ResolveDependencies([]string{"isis"}, nil, config.ConfigNeedHolds(map[string]any{}))
	if err != nil {
		t.Fatalf("ResolveDependencies: %v", err)
	}
	if !slices.Contains(names, policy.PluginName) {
		t.Fatalf("loading isis must load %q: %v", policy.PluginName, names)
	}
}

// TestEveryFilterTypeDeclaresFields: every ze:filter list in the loaded schema
// declared the route fields it reads and sets, and every declaration names a
// ze:filter list, so a new filter type without a field declaration fails here
// rather than being refused at every attachment (spec "Fields each filter type
// reads and sets"). Method: derive the filter types from the schema every
// linked plugin registered (config.YANGSchema, ListNode.Filter under the
// policy root), never from a hand list, and compare with
// policy.DeclaredFilterTypes.
func TestEveryFilterTypeDeclaresFields(t *testing.T) {
	schema, err := config.YANGSchema()
	if err != nil {
		t.Fatalf("YANGSchema: %v", err)
	}
	node, err := schema.Lookup(policy.RootName)
	if err != nil {
		t.Fatalf("schema has no %s root: %v", policy.RootName, err)
	}
	root, ok := node.(*config.ContainerNode)
	if !ok {
		t.Fatalf("the %s schema node is %T, not a container", policy.RootName, node)
	}
	var marked []string
	for _, name := range root.Children() {
		if list, isList := root.Get(name).(*config.ListNode); isList && list.Filter {
			marked = append(marked, name)
		}
	}
	if len(marked) == 0 {
		t.Fatalf("no ze:filter list under the %s root: the schema did not load the filter plugins", policy.RootName)
	}
	declared := policy.DeclaredFilterTypes()
	for _, name := range marked {
		if !slices.Contains(declared, name) {
			t.Errorf("filter type %q (a ze:filter list under %s) declares no route fields: "+
				"call policy.MustRegisterFilterType from its plugin's init()", name, policy.RootName)
		}
	}
	for _, name := range declared {
		if !slices.Contains(marked, name) {
			t.Errorf("policy.MustRegisterFilterType declared %q, which is no ze:filter list under %s in the loaded schema",
				name, policy.RootName)
		}
	}
}
