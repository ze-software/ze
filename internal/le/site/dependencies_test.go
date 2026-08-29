// Design: website/AI.md -- the dependency reference is curated membership over go.mod's versions
package site

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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

	var data dependencyData
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
			at := strings.Index(page, "<td><code>"+module.Module+"</code></td>")
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

	if !strings.Contains(page, "<td><code>example.com/beta</code></td><td><code>v9.9.9</code></td>") {
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

	got := visibleText(mainContent(t, page))
	want := visibleText(readFixture(t, "published-dependencies-body.html"))
	if got != want {
		t.Errorf("the dependency page reads as\n  %q\nthe published page reads as\n  %q", got, want)
	}

	mirror := readArtifact(t, paths.Output, "reference/dependencies/"+pageMirrorFile)
	if mirror != readFixture(t, "published-dependencies.md") {
		t.Errorf("the mirror is\n%q\nthe published mirror is\n%q",
			mirror, readFixture(t, "published-dependencies.md"))
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
	var data dependencyData
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

	row := "| `example.com/alpha` | `v1.0.0` | Parses table\\|json\\|yaml output. |"
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
