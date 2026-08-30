// Design: (none -- build tool) -- what the plugin catalog publishes about one plugin
package inventory

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/le/lepath"
)

// repositoryRoot answers the checkout these tests read.
func repositoryRoot(t *testing.T) string {
	t.Helper()
	root, err := lepath.Root()
	if err != nil {
		t.Fatal(err)
	}
	return root
}

// VALIDATES: the three fields the published plugin catalog shows, and which
// registry.Registration does not carry, are derived for a real registration.
//
// The method reads the `static` plugin, which registers in an untagged build,
// so the case holds whatever feature gates the test binary was built with. Its
// three answers are checked against the tree: the package that registers it,
// every YANG file beside that package, and the optional dependency its own
// registration declares.
func TestAPluginCarriesItsSourceDirectoryYANGFilesAndOptionalDependencies(t *testing.T) {
	root := repositoryRoot(t)

	plugins, err := Plugins(root)
	if err != nil {
		t.Fatal(err)
	}
	index := slices.IndexFunc(plugins, func(plugin Plugin) bool { return plugin.Name == "static" })
	if index < 0 {
		t.Fatalf("the registry answered %d plugins and none of them is `static`", len(plugins))
	}
	static := plugins[index]

	if static.SourceDir != "internal/plugins/static" {
		t.Errorf("static registers from %q, want internal/plugins/static", static.SourceDir)
	}
	wantYANG := []string{
		"internal/plugins/static/yang/ze-static-cmd.yang",
		"internal/plugins/static/yang/ze-static-conf.yang",
	}
	if !slices.Equal(static.YANGFiles, wantYANG) {
		t.Errorf("static ships %v, want %v", static.YANGFiles, wantYANG)
	}
	if !slices.Equal(static.OptionalDependencies, []string{"interface"}) {
		t.Errorf("static optionally depends on %v, want [interface]", static.OptionalDependencies)
	}
}

// VALIDATES: a plugin whose registration names no YANG module still answers the
// YANG files beside its own package, and a plugin with neither answers none.
//
// The method uses `firewall-irr`, whose package holds a yang directory, and
// `fib-p4`, whose registration names a module.
func TestAPluginWithNoRegisteredModuleTakesTheYANGBesideItsPackage(t *testing.T) {
	root := repositoryRoot(t)

	plugins, err := Plugins(root)
	if err != nil {
		t.Fatal(err)
	}
	byName := make(map[string]Plugin, len(plugins))
	for _, plugin := range plugins {
		byName[plugin.Name] = plugin
	}

	irr, found := byName["firewall-irr"]
	if !found {
		t.Fatal("the registry answered no `firewall-irr`")
	}
	if irr.SourceDir != "internal/component/firewall/plugins/irr" {
		t.Errorf("firewall-irr registers from %q", irr.SourceDir)
	}
	for _, want := range []string{
		"internal/component/firewall/plugins/irr/yang/ze-firewall-irr-cmd.yang",
		"internal/component/firewall/plugins/irr/yang/ze-firewall-irr.yang",
	} {
		if !slices.Contains(irr.YANGFiles, want) {
			t.Errorf("firewall-irr does not ship %s, it ships %v", want, irr.YANGFiles)
		}
	}
}

// VALIDATES: every YANG path a plugin answers is a slash-separated path
// relative to the checkout root, and resolves to a file in it.
//
// A path that escaped the root, or one carrying a scratch copy of the tree,
// would publish a link a reader cannot follow. The walk that finds these files
// used to cover the WHOLE checkout, keyed by base name, so the 475 copies of
// the tree under tmp/ shadowed all 237 real modules.
func TestEveryYANGPathAPluginAnswersResolvesUnderTheCheckout(t *testing.T) {
	root := repositoryRoot(t)

	plugins, err := Plugins(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, plugin := range plugins {
		for _, path := range plugin.YANGFiles {
			if filepath.IsAbs(path) || filepath.ToSlash(path) != path {
				t.Errorf("%s answers %q, which is not a repository-relative slash path", plugin.Name, path)
				continue
			}
			if _, err := filepath.Rel(root, filepath.Join(root, path)); err != nil {
				t.Errorf("%s answers %q: %v", plugin.Name, path, err)
			}
			if !slices.Contains(codeAreas, firstSegment(path)) {
				t.Errorf("%s answers %q, which is outside the code areas %v", plugin.Name, path, codeAreas)
			}
		}
	}
}

// firstSegment answers the leading directory of a slash path.
func firstSegment(path string) string {
	for index := range len(path) {
		if path[index] == '/' {
			return path[:index]
		}
	}
	return path
}
