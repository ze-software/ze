// Design: docs/architecture/plugin/feature-gates.md -- registration argument evaluation retains executable membership.
package repoinventory

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestCommittedRegistrationArgumentsCannotHideInstallers keeps the ordinary
// registration valid while an unreported field evaluates a plugin/YANG installer.
// An unsupported execution owner may refuse, but cannot publish the ordinary subset.
func TestCommittedRegistrationArgumentsCannotHideInstallers(t *testing.T) {
	for _, kind := range []string{"plugin", "yang"} {
		t.Run(kind, func(t *testing.T) {
			root := retainedControlFixture(t)
			for _, edit := range []struct{ file, before, after string }{
				{"internal/component/plugin/registry/registry.go", "Dependencies, OptionalDependencies", "ConfigReads, Dependencies, OptionalDependencies"},
			} {
				raw, err := os.ReadFile(filepath.Join(root, edit.file))
				if err != nil {
					t.Fatal(err)
				}
				if strings.Count(string(raw), edit.before) != 1 {
					t.Fatalf("argument fixture marker is not unique: %s", edit.file)
				}
				write(t, root, edit.file, strings.Replace(string(raw), edit.before, edit.after, 1))
			}
			const file = "internal/plugins/example/argument_installer.go"
			declarations := `package example
import r "github.com/ze-software/ze/internal/component/plugin/registry"
func argumentEngine() {}
func init() { _ = r.Register(r.Registration{Name: "argument-outer", RunEngine: argumentEngine, ConfigReads: installArgument()}) }
func installArgument() []string { return nil }
func installArgumentMember() { _ = r.Register(r.Registration{Name: "argument-plugin", RunEngine: argumentEngine}) }
`
			if kind == "yang" {
				write(t, root, "internal/component/config/yang/register.go", "package yang\nfunc RegisterModule(name, content string) {}\n")
				write(t, root, "internal/plugins/example/yang/argument.yang", "module argument { namespace urn:argument; prefix a; }\n")
				declarations = `package example
import (
 r "github.com/ze-software/ze/internal/component/plugin/registry"
 y "github.com/ze-software/ze/internal/component/config/yang"
)
func argumentEngine() {}
func init() { _ = r.Register(r.Registration{Name: "argument-outer", RunEngine: argumentEngine, ConfigReads: installArgument()}) }
func installArgument() []string { return nil }
func installArgumentMember() { y.RegisterModule("argument.yang", "module argument { namespace urn:argument; prefix a; }") }
`
			}
			write(t, root, file, declarations)
			commitInventoryFixture(t, root)
			for _, changed := range []bool{false, true} {
				if changed {
					write(t, root, file, strings.Replace(declarations, "func installArgument() []string { return nil }", "func installArgument() []string { installArgumentMember(); return nil }", 1))
					commitInventoryFixture(t, root)
				}
				inv, reportErr := Collect(root)
				plugins, pluginErr := DeclaredPlugins(root)
				if reportErr == nil {
					if pluginErr != nil {
						t.Fatalf("argument consumers disagree: report succeeded, plugins refused: %v", pluginErr)
					}
				} else {
					if !changed {
						t.Fatalf("ordinary argument control refused: report %v, plugins %v", reportErr, pluginErr)
					}
					if pluginErr == nil {
						t.Fatalf("argument consumers disagree: report refused %v, plugins succeeded", reportErr)
					}
					if inv.Plugins != nil {
						t.Fatalf("argument report refusal leaked partial output: %+v", inv.Plugins)
					}
					if plugins != nil {
						t.Fatalf("argument plugin refusal leaked partial output: %+v", plugins)
					}
					for _, err := range []error{reportErr, pluginErr} {
						if !strings.Contains(err.Error(), "ownership") {
							t.Fatalf("argument ownership masked by unrelated refusal: %v", err)
						}
					}
					continue
				}
				want := []string{"argument-outer", "committed-linux"}
				if changed && kind == "plugin" {
					want = []string{"argument-outer", "argument-plugin", "committed-linux"}
				}
				for _, got := range [][]Plugin{inv.Plugins, plugins} {
					names := make([]string, 0, len(got))
					for _, plugin := range got {
						names = append(names, plugin.Name)
					}
					if !slices.Equal(names, want) {
						t.Fatalf("registration argument lost executable plugin: %v, want %v", names, want)
					}
				}
				found := slices.ContainsFunc(inv.YANGModules, func(module YANGModule) bool { return module.Name == "argument.yang" })
				if found != (changed && kind == "yang") {
					t.Fatalf("registration argument lost or invented executable YANG membership: %+v", inv.YANGModules)
				}
			}
		})
	}
}
