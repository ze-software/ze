// Design: docs/architecture/plugin/plugin-system.md -- data-plane declarations over the composition root
// Related: all_test.go -- composition-root invariants over registry.All
// Related: vpp_backend_optional_test.go -- optional VPP backend dependency checks

package all

import (
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/iface"
	pppoedataplane "github.com/ze-software/ze/internal/component/l2tp/pppoe/dataplane"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/component/traffic"
	"github.com/ze-software/ze/internal/core/mplsfib"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const kernelBackendName = "kernel"

type declaredSelector struct {
	owner      string
	selector   *registry.DataPlaneSelector
	names      []string
	registered map[string]bool
}

// declaredDataPlaneSelectors reads every DataPlaneSelector registered in the
// composition root and separates the primary data-plane selector from secondary
// component selectors.
func declaredDataPlaneSelectors(t *testing.T) (declaredSelector, []declaredSelector) {
	t.Helper()
	var primary declaredSelector
	var others []declaredSelector
	foundPrimary := false

	for _, reg := range registry.All() {
		if reg.DataPlaneSelector == nil {
			continue
		}
		names := slices.Clone(reg.DataPlaneSelector.RegisteredBackends())
		slices.Sort(names)
		regSet := make(map[string]bool, len(names))
		for _, name := range names {
			regSet[name] = true
		}
		entry := declaredSelector{
			owner:      reg.Name,
			selector:   reg.DataPlaneSelector,
			names:      names,
			registered: regSet,
		}
		if reg.DataPlaneSelector.SelectsDataPlane {
			if foundPrimary {
				t.Fatalf("duplicate primary DataPlaneSelector: %q and %q", primary.owner, reg.Name)
			}
			primary = entry
			foundPrimary = true
			continue
		}
		others = append(others, entry)
	}

	if !foundPrimary {
		t.Fatal("composition root links no primary DataPlaneSelector (SelectsDataPlane == true)")
	}
	if len(primary.names) == 0 {
		t.Fatalf("primary DataPlaneSelector %q (%s) reports no registered backends",
			primary.owner, primary.selector.LeafPath)
	}
	return primary, others
}

// TestEveryPlaneNameIsAnInterfaceBackend verifies that every plane-keyed
// registry across the composition root uses names registered as interface
// backends.
//
// VALIDATES: AC-11 -- every registered firewall and traffic backend name, every
// FIB plugin's DataPlane, every PPPoE session provider key, and every MPLS
// forwarding reader key is the name of a registered interface backend.
// PREVENTS: a component or plugin registering a plane name under a stale or
// mismatched spelling that no interface backend answers to.
func TestEveryPlaneNameIsAnInterfaceBackend(t *testing.T) {
	primary, others := declaredDataPlaneSelectors(t)
	if len(others) == 0 {
		t.Fatal("composition root links no secondary DataPlaneSelector (expected firewall and traffic)")
	}

	seenOwners := make(map[string]bool, len(others))
	for _, sel := range others {
		seenOwners[sel.owner] = true
		if len(sel.names) == 0 {
			t.Errorf("selector %q (%s) reports no registered backends", sel.owner, sel.selector.LeafPath)
			continue
		}
		for _, name := range sel.names {
			if !primary.registered[name] {
				t.Errorf("selector %q (%s) registered backend %q, which is not a registered %s backend (%v)",
					sel.owner, sel.selector.LeafPath, name, primary.owner, primary.names)
			}
		}
	}
	for _, required := range []string{"firewall", "traffic"} {
		if registry.Has(required) && !seenOwners[required] {
			t.Errorf("registered plugin %q declares no DataPlaneSelector", required)
		}
	}

	fibCount := 0
	for _, reg := range registry.All() {
		if reg.DataPlane == "" {
			continue
		}
		fibCount++
		if !primary.registered[reg.DataPlane] {
			t.Errorf("FIB plugin %q declares DataPlane %q, which is not a registered %s backend (%v)",
				reg.Name, reg.DataPlane, primary.owner, primary.names)
		}
	}
	if fibCount == 0 {
		t.Fatal("composition root links no plugin declaring DataPlane")
	}

	pppoeNames := pppoedataplane.SessionDataplaneNames()
	for _, name := range pppoeNames {
		if !primary.registered[name] {
			t.Errorf("PPPoE session provider %q is not a registered %s backend (%v)",
				name, primary.owner, primary.names)
		}
	}

	// Verify every registered MPLS forwarding reader plane is a registered
	// interface backend, and linked planes carry their expected readers.
	for _, name := range mplsfib.ForwardingReaderNames() {
		if !primary.registered[name] {
			t.Errorf("MPLS forwarding reader plane %q is not a registered %s backend (%v)",
				name, primary.owner, primary.names)
		}
	}
	hasMPLS := hasBuiltinRPC("ze-mpls:show-forwarding")
	if hasMPLS {
		if _, ok := mplsfib.ForwardingReaderFor(kernelBackendName); !ok {
			t.Errorf("MPLS forwarding reader is not registered under %q", kernelBackendName)
		}
	}
	if registry.Has("fib-vpp") {
		const vppPlane = "vpp"
		if _, ok := mplsfib.ForwardingReaderFor(vppPlane); !ok {
			t.Errorf("fib-vpp is linked but MPLS forwarding reader is not registered under %q", vppPlane)
		}
	}
}

// hasBuiltinRPC reports whether the composition root registers the named RPC
// wire method.
func hasBuiltinRPC(wireMethod string) bool {
	for _, rpc := range pluginserver.AllBuiltinRPCs() {
		if rpc.WireMethod == wireMethod {
			return true
		}
	}
	return false
}

// TestSelectorDefaultsAreKernel verifies that every declared backend selector
// leaf defaults to kernel in the loaded YANG schema and that each component
// registers its kernel backend under that name.
//
// VALIDATES: AC-19 -- the schema default of /interface/backend,
// /firewall/backend, and /traffic/control/backend is kernel, every declared
// DataPlaneSelector defaults to kernel in the loaded schema, and each
// component registers a backend under kernel.
// PREVENTS: a selector leaf defaulting to a retired name or a component
// registering its kernel backend under a name other than kernel.
func TestSelectorDefaultsAreKernel(t *testing.T) {
	schema, err := config.YANGSchema()
	if err != nil {
		t.Fatalf("YANGSchema: %v", err)
	}

	requiredPaths := []string{
		"/interface/backend",
		"/firewall/backend",
		"/traffic/control/backend",
	}
	for _, leafPath := range requiredPaths {
		assertSelectorLeafDefault(t, schema, leafPath, kernelBackendName)
	}

	primary, others := declaredDataPlaneSelectors(t)
	allSelectors := append([]declaredSelector{primary}, others...)
	for _, sel := range allSelectors {
		assertSelectorLeafDefault(t, schema, sel.selector.LeafPath, kernelBackendName)
		if !sel.registered[kernelBackendName] {
			t.Errorf("selector %q (%s) does not register %q (registered: %v)",
				sel.owner, sel.selector.LeafPath, kernelBackendName, sel.names)
		}
	}

	if runtime.GOOS == "linux" {
		if got := iface.DefaultBackendName(); got != kernelBackendName {
			t.Errorf("iface.DefaultBackendName() = %q, want %q", got, kernelBackendName)
		}
		if got := traffic.DefaultBackendName(); got != kernelBackendName {
			t.Errorf("traffic.DefaultBackendName() = %q, want %q", got, kernelBackendName)
		}
	}
}

// assertSelectorLeafDefault checks that leafPath resolves to a LeafNode in
// schema whose Default equals want.
func assertSelectorLeafDefault(t *testing.T, schema *config.Schema, leafPath, want string) {
	t.Helper()
	lookupPath := strings.TrimPrefix(leafPath, "/")
	node, err := schema.Lookup(lookupPath)
	if err != nil {
		t.Errorf("schema.Lookup(%q): %v", lookupPath, err)
		return
	}
	leaf, ok := node.(*config.LeafNode)
	if !ok {
		t.Errorf("schema node %q is %T, want *config.LeafNode", leafPath, node)
		return
	}
	if leaf.Default != want {
		t.Errorf("schema leaf %q default = %q, want %q", leafPath, leaf.Default, want)
	}
}

// TestEveryBackendAnnotationNamesARegisteredBackend walks the loaded YANG
// schema and proves every ze:backend argument names a backend registered under
// the selector whose subtree it annotates.
//
// VALIDATES: AC-20 -- every ze:backend annotation in the loaded schema names a
// backend registered with the governing selector (or declared as a compiled-out
// optional provider of that selector in builds without its tag), read from the
// live registry and never from a hardcoded list.
// PREVENTS: a YANG ze:backend annotation retaining a retired backend name or a
// typo that no registered backend answers to.
func TestEveryBackendAnnotationNamesARegisteredBackend(t *testing.T) {
	schema, err := config.YANGSchema()
	if err != nil {
		t.Fatalf("YANGSchema: %v", err)
	}

	primary, others := declaredDataPlaneSelectors(t)
	annotatedCount := 0
	walkSchemaBackendAnnotations(schema, "", func(nodePath string, backends []string) {
		annotatedCount++
		sel := selectorForSchemaPath(nodePath, primary, others)
		for _, arg := range backends {
			if sel.registered[arg] {
				continue
			}
			// Optional providers (for example vpp when built without ze_vpp)
			// declare a ConfigNeed on the selector owner rather than
			// registering a factory in this build.
			if registry.CompiledOutValueError(sel.owner, arg) != nil {
				continue
			}
			t.Errorf("schema node %q carries ze:backend %q, which names no registered backend of %s (%s; registered: %v)",
				nodePath, arg, sel.owner, sel.selector.LeafPath, sel.names)
		}
	})

	if annotatedCount == 0 {
		t.Fatal("loaded YANG schema carries no ze:backend annotations, so the walk proved nothing")
	}
}

// walkSchemaBackendAnnotations visits every child node of cp depth-first and
// invokes visit for each node carrying a non-empty ze:backend annotation.
func walkSchemaBackendAnnotations(cp config.ChildProvider, prefix string, visit func(path string, backends []string)) {
	var pb textbuf.Buffer
	for _, name := range cp.Children() {
		child := cp.Get(name)
		if child == nil {
			continue
		}
		pb.Reset()
		if prefix != "" {
			pb.Str(prefix).Byte('/')
		}
		pb.Str(name)
		childPath := pb.String()

		if backends := nodeBackendAnnotation(child); len(backends) > 0 {
			visit(childPath, backends)
		}
		if next, ok := child.(config.ChildProvider); ok {
			walkSchemaBackendAnnotations(next, childPath, visit)
		}
	}
}

// nodeBackendAnnotation extracts the ze:backend slice from any schema node
// type that carries it.
func nodeBackendAnnotation(node config.Node) []string {
	switch n := node.(type) {
	case *config.LeafNode:
		return n.Backend
	case *config.ContainerNode:
		return n.Backend
	case *config.ListNode:
		return n.Backend
	case *config.InlineListNode:
		return n.Backend
	case *config.FlexNode:
		return n.Backend
	default:
		return nil
	}
}

// selectorForSchemaPath returns the declared selector governing nodePath by
// longest matching root prefix, falling back to the primary data-plane
// selector for nodes outside any secondary selector's roots.
func selectorForSchemaPath(nodePath string, primary declaredSelector, others []declaredSelector) declaredSelector {
	best := primary
	bestLen := -1
	for _, sel := range others {
		for _, root := range sel.selector.Roots {
			cleanRoot := strings.TrimPrefix(root, "/")
			if !schemaPathUnderRoot(nodePath, cleanRoot) {
				continue
			}
			if len(cleanRoot) > bestLen {
				best = sel
				bestLen = len(cleanRoot)
			}
		}
	}
	return best
}

// schemaPathUnderRoot reports whether nodePath equals root or is nested under
// root separated by '/'.
func schemaPathUnderRoot(nodePath, root string) bool {
	if nodePath == root {
		return true
	}
	if len(nodePath) <= len(root) {
		return false
	}
	if !strings.HasPrefix(nodePath, root) {
		return false
	}
	return nodePath[len(root)] == '/'
}

// TestHostSideDataPlaneMarkSetIsExact verifies over the composition root that
// exactly fib-kernel and sysctl set Registration.HostSideDataPlane (AC-5, AC-14).
//
// VALIDATES: AC-5, AC-14 -- over the composition root, the host-side data-plane
// mark is carried exclusively by fib-kernel and sysctl.
// PREVENTS: another plugin silently opting out of data-plane mismatch checks
// with HostSideDataPlane: true, or fib-kernel/sysctl losing the mark.
func TestHostSideDataPlaneMarkSetIsExact(t *testing.T) {
	var marked []string
	for _, reg := range registry.All() {
		if reg.HostSideDataPlane {
			marked = append(marked, reg.Name)
		}
	}
	slices.Sort(marked)
	want := []string{"fib-kernel", "sysctl"}
	if !slices.Equal(marked, want) {
		t.Fatalf("plugins with HostSideDataPlane = %v, want %v", marked, want)
	}
}

// TestFIBDummyRefusedBesideEveryRegisteredFIBPlugin verifies over the full
// composition root that every registered NoDataPlaneChoice FIB plugin is
// refused beside every other registered ProgramsFIB plugin, whether the other
// plugin is selected by its config root, by a bus endpoint, or by --plugin.
//
// VALIDATES: AC-4 over the composition root with zero hardcoded FIB plugin names.
// PREVENTS: a registered FIB plugin bypassing the no-plane refusal at either
// config validation door (VerifyPluginConfig or ReadConfig).
func TestFIBDummyRefusedBesideEveryRegisteredFIBPlugin(t *testing.T) {
	savedCLI := config.CommandLinePlugins()
	t.Cleanup(func() { config.RecordCommandLinePlugins(savedCLI) })
	config.RecordCommandLinePlugins(nil)

	var choices []*registry.Registration
	var others []*registry.Registration
	for _, reg := range registry.All() {
		if !reg.ProgramsFIB {
			continue
		}
		if reg.NoDataPlaneChoice {
			choices = append(choices, reg)
			continue
		}
		others = append(others, reg)
	}
	if len(choices) == 0 {
		t.Fatal("composition root links no ProgramsFIB plugin declaring NoDataPlaneChoice")
	}
	if len(others) == 0 {
		t.Fatal("composition root links no other ProgramsFIB plugin to test beside NoDataPlaneChoice")
	}

	assertBothDoorsRefuse := func(t *testing.T, cfg string, wantSubstrings ...string) {
		t.Helper()
		tree, err := config.ParseTreeForValidation(cfg)
		if err != nil {
			t.Fatalf("ParseTreeForValidation(%q): %v", cfg, err)
		}
		verifyErrs := config.VerifyPluginConfig(tree)
		if len(verifyErrs) == 0 {
			t.Fatalf("VerifyPluginConfig(%q) = none, want refusal", cfg)
		}
		_, readErr := config.ReadConfig(cfg, "test.conf", nil)
		if readErr == nil {
			t.Fatalf("ReadConfig(%q) = nil, want refusal", cfg)
		}
		verifyText := config.JoinPluginConfigVerifyErrors(verifyErrs).Error()
		readText := readErr.Error()
		for _, want := range wantSubstrings {
			if !strings.Contains(verifyText, want) {
				t.Errorf("VerifyPluginConfig missing %q:\n%s", want, verifyText)
			}
			if !strings.Contains(readText, want) {
				t.Errorf("ReadConfig missing %q:\n%s", want, readText)
			}
		}
	}

	for _, choice := range choices {
		if len(choice.ConfigRoots) == 0 {
			t.Fatalf("NoDataPlaneChoice plugin %q declares no ConfigRoots", choice.Name)
		}
		choiceRoot := choice.ConfigRoots[0]
		choiceDisplay := strings.ReplaceAll(strings.TrimPrefix(choiceRoot, "/"), "/", " ")
		choiceBlock := configBlockForRoots(choiceRoot)

		// Standalone no-plane choice commits cleanly.
		t.Run(choice.Name+"/alone", func(t *testing.T) {
			tree, err := config.ParseTreeForValidation(choiceBlock)
			if err != nil {
				t.Fatalf("ParseTreeForValidation: %v", err)
			}
			if errs := config.VerifyPluginConfig(tree); len(errs) != 0 {
				t.Fatalf("VerifyPluginConfig(alone) = %v, want none", errs)
			}
			if _, err := config.ReadConfig(choiceBlock, "test.conf", nil); err != nil {
				t.Fatalf("ReadConfig(alone) = %v, want nil", err)
			}
		})

		for _, other := range others {
			otherDisplay := other.Name
			if len(other.ConfigRoots) > 0 {
				otherDisplay = strings.ReplaceAll(strings.TrimPrefix(other.ConfigRoots[0], "/"), "/", " ")
			}
			wantWhy := choiceDisplay + " is the choice to program no route, and " + otherDisplay
			if other.DataPlane == "" {
				wantWhy += " is another FIB plugin"
			} else {
				wantWhy += " programs the " + other.DataPlane + " data plane"
			}

			if len(other.ConfigRoots) > 0 {
				t.Run(choice.Name+"/root/"+other.Name, func(t *testing.T) {
					cfg := configBlockForRoots(choiceRoot, other.ConfigRoots[0])
					assertBothDoorsRefuse(t, cfg,
						choiceDisplay+" is configured beside "+otherDisplay,
						wantWhy,
						"delete "+choiceDisplay+", or delete "+otherDisplay,
					)
				})
			}

			t.Run(choice.Name+"/endpoint/"+other.Name, func(t *testing.T) {
				cfg := choiceBlock + "bus { endpoint w { internal " + other.Name + "; } }\n"
				assertBothDoorsRefuse(t, cfg,
					choiceDisplay+" is configured and the plugin "+other.Name+" is selected",
					wantWhy,
					"delete "+choiceDisplay+", or delete bus endpoint w",
				)
			})

			t.Run(choice.Name+"/cli/"+other.Name, func(t *testing.T) {
				config.RecordCommandLinePlugins([]string{"ze." + other.Name})
				t.Cleanup(func() { config.RecordCommandLinePlugins(nil) })
				assertBothDoorsRefuse(t, choiceBlock,
					choiceDisplay+" is configured and the plugin "+other.Name+" is selected",
					wantWhy,
					"delete "+choiceDisplay+", or start ze without --plugin ze."+other.Name,
				)
			})
		}
	}
}

// configBlockForRoots builds valid hierarchical config text containing empty
// containers for each slash-separated root, merging sibling roots that share a
// top-level container (for example "fib/a" and "fib/b" -> "fib { a { } b { } }").
func configBlockForRoots(roots ...string) string {
	var topOrder []string
	children := make(map[string][]string)
	for _, r := range roots {
		parts := strings.Split(strings.TrimPrefix(r, "/"), "/")
		top := parts[0]
		if _, seen := children[top]; !seen {
			topOrder = append(topOrder, top)
			children[top] = nil
		}
		if len(parts) > 1 {
			children[top] = append(children[top], parts[1])
		}
	}
	var tb textbuf.Buffer
	for _, top := range topOrder {
		tb.Str(top).Str(" { ")
		for _, sub := range children[top] {
			tb.Str(sub).Str(" { } ")
		}
		tb.Str("}\n")
	}
	return tb.String()
}
