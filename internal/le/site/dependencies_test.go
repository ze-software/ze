// Design: website/AI.md -- the dependency reference is curated membership over go.mod's versions
package site

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/deps/credits"
)

// dependenciesPaths lays out one checkout carrying the curated list and the
// go.mod that was in the tree when the published page was rendered.
//
// Both are snapshots, so the parity test below compares one fixed input against
// one fixed page: go.mod gains a dependency most weeks, and reading the live one
// would make the golden move.
func dependenciesPaths(t *testing.T) Paths {
	t.Helper()
	paths := dataPagePaths(t, map[string]string{dependenciesDataFile: "published-dependencies.json"})
	repository := t.TempDir()
	copyFixture(t, filepath.Join("testdata", "published-go.mod"), filepath.Join(repository, "go.mod"))
	paths.Repository = repository
	return paths
}

// writeGoMod replaces the go.mod of a laid-out checkout.
func writeGoMod(t *testing.T, paths Paths, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(paths.Repository, "go.mod"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: the page publishes the curated categories, and the modules inside
// each one, in the data file's own order.
//
// go.mod lists its requirements alphabetically, so a page that took its order
// from go.mod would put charm.land first and interleave the groups. The method
// is positional over the rendered page.
func TestTheDependencyPageKeepsTheCuratedOrder(t *testing.T) {
	paths := dependenciesPaths(t)

	if _, err := renderDependencies(paths); err != nil {
		t.Fatal(err)
	}
	page := readArtifact(t, paths.Output, dependenciesDest)

	var data credits.Declaration
	if err := readSourceJSON(paths.Source, dependenciesDataFile, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Categories) != 8 {
		t.Fatalf("the fixture holds %d categories, want the published 8", len(data.Categories))
	}

	previous := -1
	modules := 0
	for _, category := range data.Categories {
		at := strings.Index(page, "<summary>"+strings.ReplaceAll(category.Name, "&", "&amp;"))
		if at < 0 {
			t.Fatalf("the page carries no %q group", category.Name)
		}
		if at < previous {
			t.Fatalf("group %q sits above the group declared before it", category.Name)
		}
		previous = at
		for _, module := range category.Modules {
			modules++
			at := strings.Index(page, `<td><code class="concept-software">`+module.Module+"</code></td>")
			if at < 0 {
				t.Fatalf("the page carries no row for %s", module.Module)
			}
			if at < previous {
				t.Fatalf("module %s sits above the module declared before it", module.Module)
			}
			previous = at
		}
	}
	if modules != 42 {
		t.Fatalf("the fixture holds %d modules, want the published 42", modules)
	}
}

// VALIDATES: go.mod supplies the version of a curated module and nothing else.
//
// The method changes one version in go.mod and reorders the require block, then
// asserts that the cell moved and the page's order did not.
func TestGoModSuppliesTheVersionAndNeverTheOrder(t *testing.T) {
	paths := dependenciesPaths(t)
	writeSourceData(t, paths, dependenciesDataFile, `{"categories":[
		{"name":"Second Group","modules":[{"module":"example.com/beta","why":"Beta."}]},
		{"name":"First Group","modules":[{"module":"example.com/alpha","why":"Alpha."}]}]}`)
	writeGoMod(t, paths, "module x\n\nrequire (\n\texample.com/alpha v1.2.3\n\texample.com/beta v9.9.9\n)\n")

	if _, err := renderDependencies(paths); err != nil {
		t.Fatal(err)
	}
	page := readArtifact(t, paths.Output, dependenciesDest)

	if !strings.Contains(page, `<td><code class="concept-software">example.com/beta</code></td><td><code>v9.9.9</code></td>`) {
		t.Error("the page does not carry the version go.mod pins for example.com/beta")
	}
	second := strings.Index(page, "<summary>Second Group")
	first := strings.Index(page, "<summary>First Group")
	if second < 0 || first < 0 {
		t.Fatal("the page carries neither group")
	}
	if second > first {
		t.Error("the page ordered the groups by go.mod rather than by the curated file")
	}
}

// VALIDATES: an indirect requirement is not a direct dependency, so it neither
// reaches the page nor makes the drift check red.
func TestAnIndirectRequirementIsNotADependency(t *testing.T) {
	paths := dependenciesPaths(t)
	writeSourceData(t, paths, dependenciesDataFile,
		`{"categories":[{"name":"Group","modules":[{"module":"example.com/alpha","why":"Alpha."}]}]}`)
	writeGoMod(t, paths,
		"module x\n\nrequire (\n\texample.com/alpha v1.0.0\n\texample.com/transitive v0.1.0 // indirect\n)\n")

	if _, err := renderDependencies(paths); err != nil {
		t.Fatal(err)
	}
	page := readArtifact(t, paths.Output, dependenciesDest)

	if strings.Contains(page, "example.com/transitive") {
		t.Error("the page publishes an indirect requirement as a dependency")
	}
}

// VALIDATES: a direct dependency with no curated entry is refused by name.
//
// The retired renderer warned and its build then exited non-zero, so the page
// never published a partial list. Without the refusal the module is simply
// absent from the page and nothing says so.
func TestADirectDependencyWithNoCuratedEntryIsRefused(t *testing.T) {
	paths := dependenciesPaths(t)
	writeGoMod(t, paths, "module x\n\nrequire (\n\texample.com/undocumented v1.0.0\n)\n")

	_, err := renderDependencies(paths)
	if err == nil {
		t.Fatal("a go.mod carrying an undocumented direct dependency was published")
	}
	if !strings.Contains(err.Error(), "example.com/undocumented") {
		t.Errorf("the refusal is %q, which does not name the undocumented module", err)
	}
}

// VALIDATES: a curated entry for a module go.mod no longer requires directly is
// refused by name, so the page cannot publish a dependency Ze has dropped.
func TestACuratedEntryGoModNoLongerRequiresIsRefused(t *testing.T) {
	paths := dependenciesPaths(t)
	writeSourceData(t, paths, dependenciesDataFile, `{"categories":[
		{"name":"Group","modules":[
			{"module":"example.com/alpha","why":"Alpha."},
			{"module":"example.com/retired","why":"Gone."}]}]}`)
	writeGoMod(t, paths, "module x\n\nrequire (\n\texample.com/alpha v1.0.0\n)\n")

	_, err := renderDependencies(paths)
	if err == nil {
		t.Fatal("a curated entry for a dropped dependency was published")
	}
	if !strings.Contains(err.Error(), "example.com/retired") {
		t.Errorf("the refusal is %q, which does not name the retired module", err)
	}
}

// VALIDATES: the dependency reference reads as the published page and its mirror
// is the published mirror byte for byte.
func TestTheDependencyPageReadsAsThePublishedPage(t *testing.T) {
	paths := dependenciesPaths(t)

	routes, err := renderDependencies(paths)
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 1 || routes[0] != "/reference/dependencies/" {
		t.Fatalf("the producer claimed %v, want [/reference/dependencies/]", routes)
	}

	page := readArtifact(t, paths.Output, dependenciesDest)
	for _, chrome := range []string{
		"<title>Dependencies - Ze</title>",
		`<link rel="canonical" href="https://ze-software.net/reference/dependencies/" />`,
		`<link rel="stylesheet" href="../../assets/site.css" />`,
		`<main id="top" class="has-page-sidebar" tabindex="-1">`,
		`<section aria-labelledby="dependencies-title" class="md-content reveal cat-platform">`,
		`<h1 id="dependencies-title">Dependencies</h1>`,
		`<input id="dep-search" type="search"`,
		`<aside class="page-sidebar" aria-label="Related page links">`,
		"<footer>",
	} {
		if !strings.Contains(page, chrome) {
			t.Errorf("the dependency page is missing %q", chrome)
		}
	}

	mirror := readArtifact(t, paths.Output, "reference/dependencies/"+pageMirrorFile)
	// With -update (the package's one golden flag, rfcdata_test.go), the two
	// output goldens are written from this render instead of compared, so
	// they are always the renderer's own output and never typed by hand.
	if *updateRFCBars {
		writeDependencyGolden(t, "published-dependencies-body.html", mainContent(t, page))
		writeDependencyGolden(t, "published-dependencies.md", mirror)
		return
	}

	got := visibleText(mainContent(t, page))
	want := visibleText(readFixture(t, "published-dependencies-body.html"))
	if got != want {
		t.Errorf("the dependency page reads as\n  %q\nthe published page reads as\n  %q", got, want)
	}

	if mirror != readFixture(t, "published-dependencies.md") {
		t.Errorf("the mirror is\n%q\nthe published mirror is\n%q",
			mirror, readFixture(t, "published-dependencies.md"))
	}
}

// writeDependencyGolden writes one output golden of the dependency page under
// testdata, for the -update run of the parity test above.
func writeDependencyGolden(t *testing.T, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join("testdata", name), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: the section names the heading that labels it.
//
// visibleText cannot see an attribute, so the parity test above passes whether
// or not aria-labelledby names an element the page carries.
func TestTheDependencyPageIsLabelledByItsOwnHeading(t *testing.T) {
	paths := dependenciesPaths(t)

	if _, err := renderDependencies(paths); err != nil {
		t.Fatal(err)
	}
	page := readArtifact(t, paths.Output, dependenciesDest)

	if !strings.Contains(page, `aria-labelledby="dependencies-title"`) {
		t.Error("the section carries no aria-labelledby")
	}
	if !strings.Contains(page, `id="dependencies-title"`) {
		t.Error(`aria-labelledby names dependencies-title, which the page does not carry`)
	}
}

// VALIDATES: this checkout's own go.mod and curated list agree, so a build of
// this tree publishes every direct dependency Ze takes.
//
// The test above proves the drift check refuses; this one proves the tree it
// refuses is not this one. Without it the check is armed against a defect the
// repository already carries and no build could run.
func TestThisCheckoutHasNoDependencyDrift(t *testing.T) {
	root := repositoryRoot(t)
	var data credits.Declaration
	if err := readSourceJSON(filepath.Join(root, "website"), dependenciesDataFile, &data); err != nil {
		t.Fatal(err)
	}
	versions, err := directModuleVersions(filepath.Join(root, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	if err := checkDependencyDrift(versions, data); err != nil {
		t.Fatalf("this checkout would refuse its own dependency page: %v", err)
	}
	t.Logf("%d direct dependencies, all curated across %d groups", len(versions), len(data.Categories))
}

// VALIDATES: a pipe inside a curated reason stays inside its own mirror cell.
//
// A bare pipe ends a Markdown table cell, so an unescaped one splits the row and
// pushes the rest of the reason into a fourth column the header does not have.
// No published reason carries one today, which is why this states the case
// rather than relying on the parity fixture to meet it.
func TestAPipeInADependencyReasonStaysInItsCell(t *testing.T) {
	paths := dependenciesPaths(t)
	writeSourceData(t, paths, dependenciesDataFile, `{"categories":[{"name":"Group","modules":[
		{"module":"example.com/alpha","why":"Parses table|json|yaml output."}]}]}`)
	writeGoMod(t, paths, "module x\n\nrequire (\n\texample.com/alpha v1.0.0\n)\n")

	if _, err := renderDependencies(paths); err != nil {
		t.Fatal(err)
	}
	mirror := readArtifact(t, paths.Output, "reference/dependencies/"+pageMirrorFile)

	row := "| `example.com/alpha` | `v1.0.0` | not recorded | not recorded | Parses table\\|json\\|yaml output. |"
	if !strings.Contains(mirror, row) {
		t.Errorf("the mirror row is not\n  %s\nthe mirror reads\n%s", row, mirror)
	}
}

// VALIDATES: a curated list that names one module twice is refused by name.
//
// The drift check counts a module once, so a duplicate would pass the count and
// publish the same row in two groups.
func TestACuratedModuleNamedTwiceIsRefused(t *testing.T) {
	paths := dependenciesPaths(t)
	writeSourceData(t, paths, dependenciesDataFile, `{"categories":[
		{"name":"One","modules":[{"module":"example.com/alpha","why":"Alpha."}]},
		{"name":"Two","modules":[{"module":"example.com/alpha","why":"Alpha again."}]}]}`)
	writeGoMod(t, paths, "module x\n\nrequire (\n\texample.com/alpha v1.0.0\n)\n")

	_, err := renderDependencies(paths)
	if err == nil {
		t.Fatal("a curated list naming a module twice was published")
	}
	if !strings.Contains(err.Error(), "example.com/alpha") {
		t.Errorf("the refusal is %q, which does not name the doubled module", err)
	}
}

// VALIDATES: a go.mod with no direct requirement is refused rather than
// publishing a page whose every version cell is empty.
func TestAGoModWithNoDirectRequirementIsRefused(t *testing.T) {
	paths := dependenciesPaths(t)
	writeGoMod(t, paths, "module x\n\nrequire (\n\texample.com/alpha v1.0.0 // indirect\n)\n")

	_, err := renderDependencies(paths)
	if err == nil {
		t.Fatal("a go.mod with no direct requirement was published")
	}
	// The phrase is this guard's own. The drift check below it also names
	// go.mod, so a test looking for the file name alone would pass with this
	// guard removed.
	if !strings.Contains(err.Error(), "declares no direct dependency") {
		t.Errorf("the refusal is %q, which is not this guard's", err)
	}
}

// creditedDeclaration is a declaration carrying every licence shape the page
// renders: holders, a licence note, a work whose files name no holder, a
// linked module, the toolchain work and a copied work with no version.
const creditedDeclaration = `{
	"categories":[{"name":"Group","modules":[
		{"module":"example.com/alpha","why":"Alpha.","licence":"MIT","copyright":"Alpha <a@example.com>",
			"source":"https://example.com/alpha"},
		{"module":"example.com/gpl","why":"Lints.","licence":"GPL-3.0","copyright":"","copyright-unstated":true,
			"source":"https://example.com/gpl","licence-note":"states neither only nor or-later"}]}],
	"linked":[{"module":"example.com/indirect","licence":"BSD-3-Clause","copyright":"Indirect Ltd",
		"source":"https://example.com/indirect"}],
	"works":[
		{"name":"Go standard library and runtime","version":"","toolchain":true,"licence":"BSD-3-Clause",
			"copyright":"The Go Authors","source":"https://github.com/golang/go","why":"Go.","group":"Go Toolchain",
			"files":["third_party/go/LICENSE"]},
		{"name":"Copied writer","version":"","licence":"BSD-3-Clause","copyright":"the writer authors",
			"source":"https://example.com/writer","why":"Writes.","group":"Copied and Ported Source",
			"files":["internal/writer/LICENSE"],"carried-by":["github.com/ze-software/ze/internal/writer"]}]}`

// creditedGoMod is the go.mod the credited declaration renders against: two
// direct requirements, one indirect one and the go directive.
const creditedGoMod = "module x\n\ngo 1.27.4\n\nrequire (\n\texample.com/alpha v1.0.0\n\texample.com/gpl v2.0.0\n)\n\n" +
	"require (\n\texample.com/indirect v0.4.0 // indirect\n)\n"

// VALIDATES: AC-16. Each curated module shows its licence and copyright, a
// section "Also in the shipped binary" lists every other shipped work with
// version, licence and copyright, and the mirror carries the same.
//
// The method renders one declaration holding every shape a cell takes and
// asserts each cell on the page and in the mirror: a licence note beside its
// identifier, the statement for a work whose files name no holder, the version
// of a linked module read from go.mod's indirect requirement, Go's version from
// the go directive, and the statement for a copied work with no version. No
// cell is empty, because an empty cell reads as "none".
func TestDependenciesPageShowsLicenceAndShippedSection(t *testing.T) {
	paths := dependenciesPaths(t)
	writeSourceData(t, paths, dependenciesDataFile, creditedDeclaration)
	writeGoMod(t, paths, creditedGoMod)

	if _, err := renderDependencies(paths); err != nil {
		t.Fatal(err)
	}
	page := readArtifact(t, paths.Output, dependenciesDest)
	for _, want := range []string{
		"<th>Module</th><th>Version</th><th>Licence</th><th>Copyright</th><th>Why we use it</th>",
		`<td><code class="concept-software">example.com/alpha</code></td><td><code>v1.0.0</code></td>` +
			`<td><code>MIT</code></td><td>Alpha &lt;a@example.com&gt;</td><td>Alpha.</td>`,
		`<td><code>GPL-3.0</code> (states neither only nor or-later)</td><td>No copyright holder stated</td>`,
		`<h2 id="shipped-title">Also in the shipped binary</h2>`,
		"<th>Name</th><th>Version</th><th>Licence</th><th>Copyright</th></tr>",
		"<summary>Indirect Go Modules",
		`<a href="https://example.com/indirect"><code class="concept-software">example.com/indirect</code></a>` +
			`</td><td><code>v0.4.0</code></td><td><code>BSD-3-Clause</code></td><td>Indirect Ltd</td>`,
		"<summary>Go Toolchain",
		`<code class="concept-software">Go standard library and runtime</code></a></td><td><code>go1.27.4</code></td>`,
		"<summary>Copied and Ported Source",
		`<code class="concept-software">Copied writer</code></a></td><td>not versioned</td>`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("the page does not carry\n  %s", want)
		}
	}
	if strings.Contains(page, "<td></td>") || strings.Contains(page, "<td><code></code></td>") {
		t.Error("the page carries an empty cell, which reads as none")
	}
	if strings.Index(page, `id="shipped-title"`) < strings.Index(page, "example.com/gpl") {
		t.Error("the shipped section sits above the curated groups")
	}

	mirror := readArtifact(t, paths.Output, "reference/dependencies/"+pageMirrorFile)
	for _, want := range []string{
		"| Module | Version | Licence | Copyright | Why we use it |",
		"| `example.com/alpha` | `v1.0.0` | `MIT` | Alpha <a@example.com> | Alpha. |",
		"| `example.com/gpl` | `v2.0.0` | `GPL-3.0` (states neither only nor or-later) | No copyright holder stated | Lints. |",
		"## Also in the shipped binary",
		"### Indirect Go Modules (1)",
		"| [`example.com/indirect`](https://example.com/indirect) | `v0.4.0` | `BSD-3-Clause` | Indirect Ltd |",
		"| [`Go standard library and runtime`](https://github.com/golang/go) | `go1.27.4` | `BSD-3-Clause` | The Go Authors |",
		"| [`Copied writer`](https://example.com/writer) | not versioned | `BSD-3-Clause` | the writer authors |",
	} {
		if !strings.Contains(mirror, want) {
			t.Errorf("the mirror does not carry\n  %s\nthe mirror reads\n%s", want, mirror)
		}
	}
	if strings.Contains(mirror, "|  |") || strings.Contains(mirror, "``") {
		t.Errorf("the mirror carries an empty cell, which reads as none:\n%s", mirror)
	}
}

// VALIDATES: AC-16's survivor. The lead sentence counts the curated direct
// modules only: the linked modules and the other works are in the shipped
// binary but are not direct dependencies.
//
// The credited declaration holds two curated modules, one linked module and
// two works, so a lead counting everything would say five.
func TestDependenciesLeadCountsDirectModulesOnly(t *testing.T) {
	paths := dependenciesPaths(t)
	writeSourceData(t, paths, dependenciesDataFile, creditedDeclaration)
	writeGoMod(t, paths, creditedGoMod)

	if _, err := renderDependencies(paths); err != nil {
		t.Fatal(err)
	}
	page := readArtifact(t, paths.Output, dependenciesDest)
	mirror := readArtifact(t, paths.Output, "reference/dependencies/"+pageMirrorFile)
	for name, content := range map[string]string{"page": page, "mirror": mirror} {
		if !strings.Contains(content, "Ze uses 2 direct Go dependencies.") {
			t.Errorf("the %s's lead does not count the 2 curated modules alone", name)
		}
	}
}

// VALIDATES: a linked module go.mod does not require is refused by name,
// rather than published with an empty version.
func TestALinkedModuleGoModDoesNotRequireIsRefused(t *testing.T) {
	paths := dependenciesPaths(t)
	writeSourceData(t, paths, dependenciesDataFile, creditedDeclaration)
	writeGoMod(t, paths, "module x\n\ngo 1.27.4\n\nrequire (\n\texample.com/alpha v1.0.0\n\texample.com/gpl v2.0.0\n)\n")

	_, err := renderDependencies(paths)
	if err == nil {
		t.Fatal("a linked module with no version in go.mod was published")
	}
	if !strings.Contains(err.Error(), "example.com/indirect") {
		t.Errorf("the refusal is %q, which does not name the linked module", err)
	}
}

// VALIDATES: the toolchain work is refused when go.mod states no go directive,
// rather than published with an empty version.
func TestAToolchainWorkWithNoGoDirectiveIsRefused(t *testing.T) {
	paths := dependenciesPaths(t)
	writeSourceData(t, paths, dependenciesDataFile, creditedDeclaration)
	writeGoMod(t, paths, strings.Replace(creditedGoMod, "go 1.27.4\n\n", "", 1))

	_, err := renderDependencies(paths)
	if err == nil {
		t.Fatal("the toolchain work was published with no go directive to state its version")
	}
	if !strings.Contains(err.Error(), "no go directive") {
		t.Errorf("the refusal is %q, which is not this guard's", err)
	}
}

// VALIDATES: the page reads the declaration through the credits tool's own
// reader, so a key that schema does not hold is refused rather than read as
// absent: a misspelled "licence" would otherwise publish "not recorded".
func TestAnUnknownKeyInTheDependencyDeclarationIsRefused(t *testing.T) {
	paths := dependenciesPaths(t)
	writeSourceData(t, paths, dependenciesDataFile, `{"categories":[{"name":"Group","modules":[
		{"module":"example.com/alpha","why":"Alpha.","license":"MIT"}]}]}`)
	writeGoMod(t, paths, "module x\n\nrequire (\n\texample.com/alpha v1.0.0\n)\n")

	_, err := renderDependencies(paths)
	if err == nil {
		t.Fatal("a declaration carrying an unknown key was published")
	}
	if !strings.Contains(err.Error(), "license") {
		t.Errorf("the refusal is %q, which does not name the unknown key", err)
	}
}
