// Design: docs/architecture/plugin/feature-gates.md -- retained source cannot silently omit executable registrations.
package repoinventory

import (
	"slices"
	"testing"
)

// retainedControlFixture proves an ordinary reachable registration is present
// before adding each independently discriminating source form.
func retainedControlFixture(t *testing.T) string {
	t.Helper()
	root := committedFixture(t)
	plugins, err := Plugins(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(plugins) != 1 || plugins[0].Name != "committed-linux" {
		t.Fatalf("ordinary registered control = %+v", plugins)
	}
	return root
}

// TestCommittedFamilyHelpersRetainCanonicalNames exercises the actual supported
// helper shapes: returned string slice, Family.String calls, and the bounded
// FlowSpec make/range/index/return projection. Families and their support flags
// must reach the public report under exact canonical keys.
//
// MUTATION: (internal/le/repo/inventory/externalrangeelements.go,
// `	result = e.withoutCollectionLocation(result, object, source)`,
// `	_ = source`): the range copy fam carries familySet's own location, the
// exported String's return grows the ambient pool with it, and the
// flowspec projection refuses as external mutation through familySet.
// MUTATION: (internal/le/repo/inventory/externaleffects.go,
// `			if e.metadata[object] && !e.projectedLocalWrite(source) {`,
// `			if e.metadata[object] {`): the projection's own names[i] store
// refuses as a mutated retained backing location.
func TestCommittedFamilyHelpersRetainCanonicalNames(t *testing.T) {
	cases := []struct {
		name, helper string
		want         []string
	}{
		{"returned-slice", `func declaredFamilies() []string { return []string{"ipv4/unicast", "ipv6/unicast"} }`, []string{"ipv4/unicast", "ipv6/unicast"}},
		{"family-methods", `func declaredFamilies() []string { return []string{IPv4.String(), IPv6.String()} }`, []string{"ipv4/flow", "ipv6/flow"}},
		{"flowspec-projection", `var familySet = []family.Family{IPv4, IPv6}
func declaredFamilies() []string {
 names := make([]string, len(familySet))
 for i, fam := range familySet { names[i] = fam.String() }
 return names
}`, []string{"ipv4/flow", "ipv6/flow"}},
	}
	for _, scenario := range cases {
		t.Run(scenario.name, func(t *testing.T) {
			root := retainedControlFixture(t)
			write(t, root, "internal/component/plugin/registry/registry.go", `package registry
type Registration struct { Name, Description, YANG string; Dependencies, OptionalDependencies, StartAfter, ConfigRoots, Families []string; RunEngine func(); InProcessNLRIDecoder, InProcessNLRIEncoder, InProcessRouteEncoder, InProcessConfigNLRIBuilder func() }
func Register(Registration) error { return nil }
`)
			write(t, root, "internal/core/family/family.go", `package family
type AFI uint16
type SAFI uint8
type Family struct { AFI AFI; SAFI SAFI; name string }
func MustRegister(afi AFI, safi SAFI, afiName, safiName string) Family { return Family{AFI: afi, SAFI: safi, name: afiName + "/" + safiName} }
func (f Family) String() string { return f.name }
`)
			write(t, root, "internal/plugins/example/families.go", `package example
import (
 r "github.com/ze-software/ze/internal/component/plugin/registry"
 family "github.com/ze-software/ze/internal/core/family"
)
var IPv4 = family.MustRegister(1, 133, "ipv4", "flow")
var IPv6 = family.MustRegister(2, 133, "ipv6", "flow")
func familyEngine() {}
func init() { _ = r.Register(r.Registration{Name: "family-provider", RunEngine: familyEngine, Families: declaredFamilies(), InProcessNLRIDecoder: familyEngine, InProcessNLRIEncoder: familyEngine, InProcessRouteEncoder: familyEngine, InProcessConfigNLRIBuilder: familyEngine}) }
`+scenario.helper+"\n")
			commitInventoryFixture(t, root)
			inv, err := Collect(root)
			if err != nil {
				t.Fatal(err)
			}
			names := make([]string, 0, len(inv.Plugins))
			for _, plugin := range inv.Plugins {
				names = append(names, plugin.Name)
			}
			if !slices.Equal(names, []string{"committed-linux", "family-provider"}) {
				t.Fatalf("helper changed executable membership: %v", names)
			}
			index := slices.IndexFunc(inv.Plugins, func(plugin Plugin) bool { return plugin.Name == "family-provider" })
			if !slices.Equal(inv.Plugins[index].Families, scenario.want) {
				t.Fatalf("declared families = %v, want %v", inv.Plugins[index].Families, scenario.want)
			}
			if len(inv.Families) != len(scenario.want) || len(inv.FamilySupport) != len(scenario.want) {
				t.Fatalf("family report keys/flags missing: %v %+v", inv.Families, inv.FamilySupport)
			}
			for _, family := range scenario.want {
				if inv.Families[family] != "family-provider" {
					t.Errorf("family %q owner = %q", family, inv.Families[family])
				}
				index := slices.IndexFunc(inv.FamilySupport, func(row Family) bool { return row.Family == family })
				if index < 0 {
					t.Fatalf("missing support row for %s", family)
				}
				row := inv.FamilySupport[index]
				if row.Plugin != "family-provider" || !row.Decode || !row.Encode || !row.RouteEncode || !row.ConfigNLRI {
					t.Errorf("family support row = %+v", row)
				}
			}
		})
	}
}

// TestCommittedCallableAliasesAreResolvedOrRefused keeps a valid ordinary
// control while plugin and standalone-YANG callable aliases execute in init.
// A bounded reader may refuse the form, but cannot return an incomplete success.
func TestCommittedCallableAliasesAreResolvedOrRefused(t *testing.T) {
	for _, kind := range []string{"plugin", "yang"} {
		t.Run(kind, func(t *testing.T) {
			root := retainedControlFixture(t)
			body := `package example
import r "github.com/ze-software/ze/internal/component/plugin/registry"
func aliasEngine() {}
func init() { install := r.Register; _ = install(r.Registration{Name: "alias-plugin", RunEngine: aliasEngine}) }
`
			if kind == "yang" {
				write(t, root, "internal/component/config/yang/register.go", "package yang\nfunc RegisterModule(name, content string) {}\n")
				write(t, root, "internal/plugins/example/yang/alias.yang", "module alias { namespace urn:alias; prefix a; }\n")
				body = `package example
import schema "github.com/ze-software/ze/internal/component/config/yang"
func init() { install := schema.RegisterModule; install("alias.yang", "module alias { namespace urn:alias; prefix a; }") }
`
			}
			write(t, root, "internal/plugins/example/alias.go", body)
			commitInventoryFixture(t, root)
			inv, err := Collect(root)
			if err != nil {
				if inv.Plugins != nil {
					t.Fatalf("refusal leaked partial plugins: %+v", inv.Plugins)
				}
				return
			}
			if kind == "plugin" {
				names := make([]string, 0, len(inv.Plugins))
				for _, plugin := range inv.Plugins {
					names = append(names, plugin.Name)
				}
				if !slices.Equal(names, []string{"alias-plugin", "committed-linux"}) {
					t.Fatalf("callable plugin alias silently omitted: %v", names)
				}
				return
			}
			if !slices.ContainsFunc(inv.YANGModules, func(module YANGModule) bool { return module.Name == "alias.yang" && module.Source == "plugin:example" }) {
				t.Fatalf("callable schema alias silently omitted: %+v", inv.YANGModules)
			}
		})
	}
}

// TestCommittedPackageInitializersAreResolvedOrRefused covers named and blank
// package variables whose initializer calls a helper that registers a plugin.
// A valid ordinary registration prevents an empty-population error hiding loss.
func TestCommittedPackageInitializersAreResolvedOrRefused(t *testing.T) {
	for _, variable := range []string{"installed", "_"} {
		t.Run(variable, func(t *testing.T) {
			root := retainedControlFixture(t)
			write(t, root, "internal/plugins/example/initializer.go", `package example
import r "github.com/ze-software/ze/internal/component/plugin/registry"
func initializerEngine() {}
func installAtDeclaration() int { _ = r.Register(r.Registration{Name: "initializer-plugin", RunEngine: initializerEngine}); return 1 }
var `+variable+` = installAtDeclaration()
`)
			commitInventoryFixture(t, root)
			plugins, err := Plugins(root)
			if err != nil {
				if plugins != nil {
					t.Fatalf("refusal leaked partial plugins: %+v", plugins)
				}
				return
			}
			names := make([]string, 0, len(plugins))
			for _, plugin := range plugins {
				names = append(names, plugin.Name)
			}
			if !slices.Equal(names, []string{"committed-linux", "initializer-plugin"}) {
				t.Fatalf("package initializer silently omitted: %v", names)
			}
		})
	}
}

// TestCommittedImmediateClosureArgumentsCannotDisappear proves evaluation of
// the call's arguments still executes even when its closure body ignores them.
func TestCommittedImmediateClosureArgumentsCannotDisappear(t *testing.T) {
	root := retainedControlFixture(t)
	write(t, root, "internal/plugins/example/argument.go", `package example
import r "github.com/ze-software/ze/internal/component/plugin/registry"
func argumentEngine() {}
func installInArgument() int { _ = r.Register(r.Registration{Name: "argument-plugin", RunEngine: argumentEngine}); return 1 }
func init() { func(_ int) {}(installInArgument()) }
`)
	commitInventoryFixture(t, root)
	plugins, err := Plugins(root)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(plugins))
	for _, plugin := range plugins {
		names = append(names, plugin.Name)
	}
	if !slices.Equal(names, []string{"argument-plugin", "committed-linux"}) {
		t.Fatalf("immediate closure argument registration silently omitted: %v", names)
	}
}

// TestCommittedFamilyHelperWriteOutsideProjectionRefuses keeps a write to a
// metadata slice refusing when its helper is not the finite family-name
// projection: the exemption covers only that projection's one store.
func TestCommittedFamilyHelperWriteOutsideProjectionRefuses(t *testing.T) {
	root := retainedControlFixture(t)
	write(t, root, "internal/component/plugin/registry/registry.go", `package registry
type Registration struct { Name, Description, YANG string; Dependencies, OptionalDependencies, StartAfter, ConfigRoots, Families []string; RunEngine func(); InProcessNLRIDecoder, InProcessNLRIEncoder, InProcessRouteEncoder, InProcessConfigNLRIBuilder func() }
func Register(Registration) error { return nil }
`)
	write(t, root, "internal/plugins/example/families.go", `package example
import r "github.com/ze-software/ze/internal/component/plugin/registry"
func familyEngine() {}
func declaredFamilies() []string { names := []string{"ipv4/unicast"}; names[0] = "ipv6/unicast"; return names }
func init() { _ = r.Register(r.Registration{Name: "family-provider", RunEngine: familyEngine, Families: declaredFamilies(), InProcessNLRIDecoder: familyEngine, InProcessNLRIEncoder: familyEngine, InProcessRouteEncoder: familyEngine, InProcessConfigNLRIBuilder: familyEngine}) }
`)
	commitInventoryFixture(t, root)
	if inv, err := Collect(root); err == nil {
		t.Fatalf("a write outside the family projection published: %+v", inv.Plugins)
	}
}
