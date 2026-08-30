// Design: website/AI.md -- the plugin catalog reads the live registry and publishes its own order
package site

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/inventory"
)

// pluginCatalogPaths lays out one tree whose artifact carries the plugin
// registry as gh-pages 2fa8fa2ad published it, so the parity cases below
// compare one fixed input against one fixed page.
//
// The registry is read from the ARTIFACT rather than from the live process,
// because a build publishes data/plugin-registry.json before any producer runs
// and every surface reads that one file. A test that read the live registry
// would answer twelve plugins in an untagged `go test` and eighty-eight in the
// shipped daemon.
func pluginCatalogPaths(t *testing.T) Paths {
	t.Helper()
	root := repositoryRoot(t)
	source := t.TempDir()
	output := t.TempDir()
	copyFixture(t, filepath.Join(root, "website", "data", "page-links.json"),
		filepath.Join(source, "data", "page-links.json"))
	copyFixture(t, filepath.Join("testdata", "published-plugin-registry.json"),
		filepath.Join(output, filepath.FromSlash(pluginFile)))
	return Paths{Repository: root, Source: source, Output: output}
}

// writePublishedRegistry replaces the artifact's plugin registry, so a case can
// state the registrations it wants rather than the whole published set.
func writePublishedRegistry(t *testing.T, paths Paths, plugins []registryPlugin) {
	t.Helper()
	content, err := json.MarshalIndent(plugins, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(paths.Output, filepath.FromSlash(pluginFile))
	if err := os.WriteFile(path, append(content, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: AC-9. The three fields the catalog page shows and a
// registry.Registration does not carry reach the page and its mirror.
//
// The method renders one plugin whose registration declares an optional
// dependency, a source directory and two YANG files, and looks for each of the
// three on the detail page. Before this phase inventory.Plugin held none of
// them, so all three would be blank.
func TestPluginCatalogCarriesTheFieldsThePageShows(t *testing.T) {
	paths := pluginCatalogPaths(t)
	writePublishedRegistry(t, paths, []registryPlugin{
		{
			Name: "bgp-rs", Description: "Route Server",
			ConfigRoots: []string{"bgp"}, Dependencies: []string{"bgp"},
			OptionalDependencies: []string{"bgp-adj-rib-in"},
			SourceDir:            "internal/component/bgp/plugins/rs",
			// The producer states the file's own order, which inventory
			// sorted; it must not re-sort and so publish a different one.
			YangFiles: []string{
				"internal/component/bgp/plugins/rs/yang/ze-rs-conf.yang",
				"internal/component/bgp/plugins/rs/yang/ze-rs-api.yang",
			},
		},
		{
			Name: "bgp-adj-rib-in", Description: "Adj-RIB-In storage (raw hex replay)",
			SourceDir: "internal/component/bgp/plugins/adj_rib_in",
		},
	})

	if _, err := renderPluginCatalog(paths); err != nil {
		t.Fatal(err)
	}

	page := readArtifact(t, paths.Output, pluginsDirectory+"/bgp-rs/"+pageIndexFile)
	for _, want := range []string{
		"<div><dt>Source path</dt><dd><code>internal/component/bgp/plugins/rs</code></dd></div>",
		"<div><dt>YANG modules</dt><dd>2</dd></div>",
		"<code>internal/component/bgp/plugins/rs/yang/ze-rs-api.yang</code>",
		`<h3>Optional</h3>`,
		`<a href="../bgp-adj-rib-in/"><code>bgp-adj-rib-in</code></a>`,
		`<a href="../../../reference/configuration/#bgp"><code>bgp</code></a>`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("the bgp-rs page is missing %q", want)
		}
	}

	mirror := readArtifact(t, paths.Output, pluginsDirectory+"/bgp-rs/"+pageMirrorFile)
	for _, want := range []string{
		"| Source path | `internal/component/bgp/plugins/rs` |",
		"| YANG modules | 2 |",
		"- Optional: [`bgp-adj-rib-in`](../bgp-adj-rib-in/index.md)",
		"YANG files: `internal/component/bgp/plugins/rs/yang/ze-rs-conf.yang`, " +
			"`internal/component/bgp/plugins/rs/yang/ze-rs-api.yang`", // <!-- doc-links: ignore (fixture data: the second name is deliberately out of alphabetical order and never existed on disk) -->
	} {
		if !strings.Contains(mirror, want) {
			t.Errorf("the bgp-rs mirror is missing %q", want)
		}
	}

	// The plugin bgp-rs names as optional must say so on its own page too, or
	// the relation is stated in one direction only.
	dependency := readArtifact(t, paths.Output, pluginsDirectory+"/bgp-adj-rib-in/"+pageMirrorFile)
	if !strings.Contains(dependency, "- Optional dependency for: [`bgp-rs`](../bgp-rs/index.md)") {
		t.Error("bgp-adj-rib-in does not say that bgp-rs uses it optionally")
	}
}

// VALIDATES: AC-3, AC-4, AC-5. One detail page reads as the published page, and
// its mirror is byte-identical to the published mirror.
func TestAPluginPageReadsAsThePublishedPage(t *testing.T) {
	paths := pluginCatalogPaths(t)

	routes, err := renderPluginCatalog(paths)
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 97 {
		t.Errorf("the catalog claimed %d routes, want 97: the index and 96 published plugins", len(routes))
	}

	page := readArtifact(t, paths.Output, pluginsDirectory+"/bfd/"+pageIndexFile)
	for _, chrome := range []string{
		"<title>bfd plugin - Ze</title>",
		`<link rel="canonical" href="https://ze-software.net/reference/plugins/bfd/" />`,
		`<link rel="stylesheet" href="../../../assets/site.css" />`,
		`<main id="top" class="has-page-sidebar" tabindex="-1">`,
		`<section class="md-content reveal cat-automate plugin-detail" aria-labelledby="plugin-detail-title">`,
		`<h1 id="plugin-detail-title"><code>bfd</code></h1>`,
		`<aside class="page-sidebar" aria-label="Related page links">`,
		"<footer>",
	} {
		if !strings.Contains(page, chrome) {
			t.Errorf("the bfd plugin page is missing %q", chrome)
		}
	}

	got := visibleText(mainContent(t, page))
	want := visibleText(readFixture(t, "published-plugin-bfd-body.html"))
	if got != want {
		t.Errorf("the bfd page reads as\n  %q\nthe published page reads as\n  %q", got, want)
	}

	mirror := readArtifact(t, paths.Output, pluginsDirectory+"/bfd/"+pageMirrorFile)
	if mirror != readFixture(t, "published-plugin-bfd.md") {
		t.Errorf("the bfd mirror is\n%q\nthe published mirror is\n%q",
			mirror, readFixture(t, "published-plugin-bfd.md"))
	}
}

// VALIDATES: AC-3, AC-4, AC-5. The catalog page reads as the published catalog,
// and its mirror is byte-identical to the published mirror.
func TestThePluginCatalogReadsAsThePublishedCatalog(t *testing.T) {
	paths := pluginCatalogPaths(t)

	if _, err := renderPluginCatalog(paths); err != nil {
		t.Fatal(err)
	}

	page := readArtifact(t, paths.Output, pluginsDest)
	got := visibleText(mainContent(t, page))
	want := visibleText(readFixture(t, "published-plugins-index-body.html"))
	if got != want {
		t.Errorf("the catalog reads as\n  %q\nthe published catalog reads as\n  %q", got, want)
	}

	mirror := readArtifact(t, paths.Output, pluginsDirectory+"/"+pageMirrorFile)
	if mirror != readFixture(t, "published-plugins-index.md") {
		t.Errorf("the catalog mirror differs from the published mirror")
		writeDiffArtifact(t, mirror, readFixture(t, "published-plugins-index.md"))
	}
}

// writeDiffArtifact reports the first line on which two documents differ.
func writeDiffArtifact(t *testing.T, got, want string) {
	t.Helper()
	gotLines, wantLines := strings.Split(got, "\n"), strings.Split(want, "\n")
	for index := range max(len(gotLines), len(wantLines)) {
		gotLine, wantLine := "", ""
		if index < len(gotLines) {
			gotLine = gotLines[index]
		}
		if index < len(wantLines) {
			wantLine = wantLines[index]
		}
		if gotLine != wantLine {
			t.Errorf("line %d\n  rendered: %q\n  published: %q", index+1, gotLine, wantLine)
			return
		}
	}
}

// VALIDATES: AC-3. The catalog's group headings and its search console carry
// the attributes visibleText cannot see, so a group whose heading lost its id
// leaves every card in it unlabelled for a screen reader.
func TestEveryPluginGroupIsLabelledByItsOwnHeading(t *testing.T) {
	paths := pluginCatalogPaths(t)

	if _, err := renderPluginCatalog(paths); err != nil {
		t.Fatal(err)
	}
	page := readArtifact(t, paths.Output, pluginsDest)
	for _, want := range []string{
		`<section class="md-content reveal cat-automate plugin-catalog" data-plugin-catalog aria-labelledby="plugin-catalog-title">`,
		`<div class="plugin-console" role="search" aria-label="Search plugins">`,
		`<p id="plugin-status" class="plugin-status search-status" aria-live="polite"></p>`,
		`<section class="plugin-group" data-plugin-group data-family="bgp-nlri" data-category="routing" aria-labelledby="plugin-group-bgp-nlri">`,
		`<h2 id="plugin-group-bgp-nlri">BGP NLRI</h2>`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("the catalog is missing %q", want)
		}
	}
}

// VALIDATES: the catalog lists its areas by label with the test harness last,
// its cards by plugin name inside each area, and gives each plugin the slug the
// name order assigns.
//
// The method is positional over the rendered page: every group heading and card
// anchor is looked up and its offset must rise. A group sorted by identifier
// rather than by label would put `bgp-nlri` before `bgp-redistribute` and after
// `bgp`, which the label order also does, so the case checks a pair the two
// orders disagree about: `interface` sorts before `ipsec-xfrm` by identifier
// and `Interface` after `IKE`.
func TestTheCatalogOrdersAreasByLabelWithTheTestHarnessLast(t *testing.T) {
	paths := pluginCatalogPaths(t)

	if _, err := renderPluginCatalog(paths); err != nil {
		t.Fatal(err)
	}
	page := readArtifact(t, paths.Output, pluginsDest)

	// The three pairs below are the ones the two candidate orders disagree
	// about. Sorting by identifier would put `interface` before `isis` and
	// `routing-table` before `rsvp-te`; sorting by label puts IS-IS before
	// Interface and RSVP TE before Routing Table.
	assertRisingOffsets(t, page, "the catalog's areas", []string{
		`<h2 id="plugin-group-anomaly">Anomaly</h2>`,
		`<h2 id="plugin-group-bgp">BGP</h2>`,
		`<h2 id="plugin-group-bgp-filter">BGP Filter</h2>`,
		`<h2 id="plugin-group-bgp-nlri">BGP NLRI</h2>`,
		`<h2 id="plugin-group-isis">IS-IS</h2>`,
		`<h2 id="plugin-group-interface">Interface</h2>`,
		`<h2 id="plugin-group-rib">RIB</h2>`,
		`<h2 id="plugin-group-rsvp-te">RSVP TE</h2>`,
		`<h2 id="plugin-group-routing-table">Routing Table</h2>`,
		`<h2 id="plugin-group-vpp">VPP</h2>`,
		`<h2 id="plugin-group-test-harness">Test Harness</h2>`,
	})
	assertRisingOffsets(t, page, "the cards of the BGP area", []string{
		`id="plugin-bgp"`, `id="plugin-bgp-adj-rib-in"`, `id="plugin-bgp-aigp"`,
	})
}

// assertRisingOffsets checks that each marker appears after the one before it.
func assertRisingOffsets(t *testing.T, page, what string, markers []string) {
	t.Helper()
	previous := -1
	for _, marker := range markers {
		offset := strings.Index(page, marker)
		if offset < 0 {
			t.Errorf("%s: %q is absent", what, marker)
			return
		}
		if offset <= previous {
			t.Errorf("%s: %q is out of order", what, marker)
			return
		}
		previous = offset
	}
}

// VALIDATES: AC-2. A plugin the registry no longer carries loses its page, and
// the catalog stops linking it.
func TestARetiredPluginLosesItsPage(t *testing.T) {
	paths := pluginCatalogPaths(t)
	if _, err := renderPluginCatalog(paths); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(paths.Output, filepath.FromSlash(pluginsDirectory+"/fakeas112"))); err != nil {
		t.Fatalf("the first render published no fakeas112 page: %v", err)
	}

	writePublishedRegistry(t, paths, []registryPlugin{
		{Name: "bfd", Description: "Bidirectional Forwarding Detection", SourceDir: "internal/component/bfd"},
	})
	if _, err := renderPluginCatalog(paths); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(paths.Output, filepath.FromSlash(pluginsDirectory+"/fakeas112"))); !os.IsNotExist(err) {
		t.Errorf("fakeas112 still has a page after leaving the registry: %v", err)
	}
	if strings.Contains(readArtifact(t, paths.Output, pluginsDest), "fakeas112") {
		t.Error("the catalog still names fakeas112")
	}
}

// VALIDATES: AC-1. The catalog claims each route once, and every route it
// claims is one the published site has.
func TestThePluginCatalogClaimsEachPublishedRouteOnce(t *testing.T) {
	paths := pluginCatalogPaths(t)

	routes, err := renderPluginCatalog(paths)
	if err != nil {
		t.Fatal(err)
	}
	seen := make(map[string]bool, len(routes))
	for _, route := range routes {
		if seen[route] {
			t.Errorf("the catalog claimed %s twice", route)
		}
		seen[route] = true
		if !strings.HasPrefix(route, pluginsRoute) || !strings.HasSuffix(route, "/") {
			t.Errorf("%q is not a plugin catalog route", route)
		}
		page := filepath.Join(paths.Output, filepath.FromSlash(strings.Trim(route, "/")), pageIndexFile)
		if _, err := os.Stat(page); err != nil {
			t.Errorf("the catalog claimed %s and wrote no page: %v", route, err)
		}
	}
	if !seen[pluginsRoute] {
		t.Errorf("the catalog did not claim its own index %s", pluginsRoute)
	}
}

// VALIDATES: a published registry naming no plugin, or a plugin with no source
// directory, is refused by name rather than publishing an empty catalog.
func TestAnUnusableRegistryIsRefusedByName(t *testing.T) {
	for _, testCase := range []struct {
		name    string
		plugins []registryPlugin
		want    string
	}{
		{"no plugin at all", []registryPlugin{}, "names no plugin"},
		{
			"a plugin with no name",
			[]registryPlugin{{Description: "x", SourceDir: "internal/plugins/x"}},
			"carries a plugin with no name",
		},
		{
			"a plugin with no source directory",
			[]registryPlugin{{Name: "x", Description: "x"}},
			"states no source directory",
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			paths := pluginCatalogPaths(t)
			writePublishedRegistry(t, paths, testCase.plugins)
			_, err := renderPluginCatalog(paths)
			if err == nil {
				t.Fatalf("the catalog published %v", testCase.plugins)
			}
			if !strings.Contains(err.Error(), testCase.want) {
				t.Errorf("the catalog refused with %q, which does not say %q", err, testCase.want)
			}
		})
	}
}

// VALIDATES: the published registry file states every field the catalog shows,
// and states an absent list as an empty array rather than as null.
func TestThePublishedRegistryStatesEveryFieldTheCatalogShows(t *testing.T) {
	content, err := marshalPluginRegistry([]inventory.Plugin{
		{
			Name: "static", Description: "Static routes",
			ConfigRoots:          []string{"static"},
			OptionalDependencies: []string{"interface"},
			SourceDir:            "internal/plugins/static",
			YANGFiles:            []string{"internal/plugins/static/yang/ze-static-conf.yang"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`"optional_dependencies": [` + "\n      \"interface\"",
		`"source_dir": "internal/plugins/static"`,
		`"yang_files": [` + "\n      \"internal/plugins/static/yang/ze-static-conf.yang\"",
		`"dependencies": []`,
	} {
		if !strings.Contains(content, want) {
			t.Errorf("the published registry is missing %q, it is\n%s", want, content)
		}
	}
	if strings.Contains(content, "null") {
		t.Errorf("the published registry states a null list:\n%s", content)
	}
}

// VALIDATES: two plugins whose names fold to one slug take separate pages, and
// which one takes the plain slug is decided by name order rather than by the
// order the catalog happens to list them in.
//
// No published pair collides, so this is the only case that can see the rule.
// `bgp rs` and `bgp-rs` both fold to `bgp-rs`; the second by name takes
// `bgp-rs-2`, and the catalog lists them under different areas so the listing
// order is not the name order.
func TestTwoPluginsSharingASlugAreSeparatedByNameOrder(t *testing.T) {
	paths := pluginCatalogPaths(t)
	writePublishedRegistry(t, paths, []registryPlugin{
		{
			Name: "bgp-rs", Description: "Route Server",
			ConfigRoots: []string{"vpp"}, SourceDir: "internal/component/vpp",
		},
		{
			Name: "bgp rs", Description: "Route Server, spelled with a space",
			ConfigRoots: []string{"anomaly"}, SourceDir: "internal/plugins/anomaly/detect",
		},
	})

	routes, err := renderPluginCatalog(paths)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{pluginsRoute, pluginsRoute + "bgp-rs/", pluginsRoute + "bgp-rs-2/"} {
		if !slices.Contains(routes, want) {
			t.Errorf("the catalog claimed %v, which does not carry %s", routes, want)
		}
	}
	plain := readArtifact(t, paths.Output, pluginsDirectory+"/bgp-rs/"+pageMirrorFile)
	if !strings.Contains(plain, "# `bgp rs` plugin") {
		t.Error("the plain slug went to the plugin that does not sort first by name")
	}
}

// VALIDATES: a card names its first three dependencies and counts the rest, and
// an area whose plugins name more than three source areas states three.
//
// No published plugin declares a fourth dependency and no published area draws
// on a fourth source, so the published fixture cannot reach either cap: both
// branches are unreachable from the parity cases above.
func TestACardNamesThreeDependenciesAndCountsTheRest(t *testing.T) {
	paths := pluginCatalogPaths(t)
	writePublishedRegistry(t, paths, []registryPlugin{
		{
			Name: "wide", Description: "Depends on four",
			ConfigRoots:  []string{"static"},
			Dependencies: []string{"one", "two", "three", "four"},
			SourceDir:    "internal/plugins/static",
		},
		{Name: "one", Description: "One", ConfigRoots: []string{"static"}, SourceDir: "internal/plugins/static/one"},
		{Name: "two", Description: "Two", ConfigRoots: []string{"static"}, SourceDir: "internal/plugins/static/two"},
		{Name: "three", Description: "Three", ConfigRoots: []string{"static"}, SourceDir: "internal/plugins/static/three"},
		{Name: "four", Description: "Four", ConfigRoots: []string{"static"}, SourceDir: "internal/plugins/static/four"},
	})

	if _, err := renderPluginCatalog(paths); err != nil {
		t.Fatal(err)
	}
	page := readArtifact(t, paths.Output, pluginsDest)
	for _, want := range []string{
		`<span class="chip">needs:one</span>`,
		`<span class="chip">needs:three</span>`,
		`<span class="chip">+1 deps</span>`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("the catalog is missing %q", want)
		}
	}
	if strings.Contains(page, `<span class="chip">needs:four</span>`) {
		t.Error("the fourth dependency is named as well as counted")
	}
}

// VALIDATES: a plugin that declares nothing at all says so, rather than showing
// an empty badge row a reader would read as a rendering fault.
func TestAPluginThatDeclaresNothingSaysSo(t *testing.T) {
	paths := pluginCatalogPaths(t)
	writePublishedRegistry(t, paths, []registryPlugin{
		{Name: "bgp-aigp", Description: "Accumulated IGP Metric (RFC 7311)",
			SourceDir: "internal/component/bgp/plugins/aigp"},
	})

	if _, err := renderPluginCatalog(paths); err != nil {
		t.Fatal(err)
	}
	page := readArtifact(t, paths.Output, pluginsDest)
	if !strings.Contains(page, `<span class="chip">no config</span>`) {
		t.Error("a plugin declaring nothing shows no badge at all")
	}
	if !strings.Contains(page, `<dl class="plugin-meta"><div><dt>Config</dt><dd>None</dd></div></dl>`) {
		t.Error("a plugin declaring nothing leaves its meta list empty")
	}
}

// VALIDATES: a pipe in a plugin's purpose stays inside its cell of the catalog
// mirror, rather than splitting the row into two more columns.
//
// No published description carries one, so the escape has no case in the parity
// comparison above: the same gap phase 6 found in the dependency page.
func TestAPipeInAPluginDescriptionStaysInItsCell(t *testing.T) {
	paths := pluginCatalogPaths(t)
	writePublishedRegistry(t, paths, []registryPlugin{
		{
			Name: "bgp-filter-modify", Description: "Modify filter (accept|reject|modify)",
			ConfigRoots: []string{"bgp"}, SourceDir: "internal/component/bgp/plugins/filter_modify",
		},
	})

	if _, err := renderPluginCatalog(paths); err != nil {
		t.Fatal(err)
	}
	mirror := readArtifact(t, paths.Output, pluginsDirectory+"/"+pageMirrorFile)
	var row string
	for line := range strings.SplitSeq(mirror, "\n") {
		if strings.HasPrefix(line, "| [`bgp-filter-modify`]") {
			row = line
		}
	}
	if row == "" {
		t.Fatal("the mirror has no row for bgp-filter-modify")
	}
	if !strings.Contains(row, `Modify filter (accept\|reject\|modify)`) {
		t.Errorf("the row states the purpose as %q", row)
	}
	if cells := strings.Count(row, "|") - strings.Count(row, `\|`); cells != 6 {
		t.Errorf("the row has %d separators, want the six a five-column row carries: %q", cells, row)
	}
}

// VALIDATES: AC-1. Every route the catalog claims from the PUBLISHED registry
// is a route the site published, and the arithmetic lands where phase 7 says.
//
// The published registry and the published route list are both taken from
// gh-pages HEAD 2fa8fa2ad, so the two describe one site. A slug rule that
// renamed one directory would move 96 routes at once, and the coverage check
// would report 96 unclaimed pages beside 96 new ones with nothing to say they
// are the same page.
func TestThePluginCatalogClaimsOnlyPublishedRoutes(t *testing.T) {
	published := make(map[string]bool)
	for route := range strings.SplitSeq(strings.TrimSpace(readFixture(t, "published-routes.txt")), "\n") {
		published[route] = true
	}
	paths := pluginCatalogPaths(t)

	claimed, err := renderPluginCatalog(paths)
	if err != nil {
		t.Fatal(err)
	}
	for _, route := range claimed {
		if !published[route] {
			t.Errorf("%s is claimed but was never published", route)
		}
	}
	if len(claimed) != 97 {
		t.Fatalf("the catalog claims %d routes, want 97", len(claimed))
	}
	if !published[configurationURL] {
		t.Errorf("%s is claimed by the configuration reference but was never published", configurationURL)
	}
	t.Logf("the catalog and the configuration reference claim %d of the %d published routes",
		len(claimed)+1, len(published))
}

// VALIDATES: AC-2, AC-16. A build refreshes the plugin registry and the YANG
// configuration tree from the live product, over whatever the seed carried.
//
// Both are published data files rather than routes, so the coverage arithmetic
// cannot see them: nothing else in this package would notice a build that
// stopped writing either, and three page families read the first one. The
// method is the one a reader would use, a seed carrying a stale file and a
// build over it.
func TestABuildRefreshesThePluginRegistryAndTheConfigurationTree(t *testing.T) {
	stubLiveInputs(t, `[{"path":"show test","description":"Show rows","mode":"read-only"}]`)
	stubProducers(t)
	root, output := siteFixture(t)

	seed := filepath.Join(filepath.Dir(root), "gh-pages", "data")
	if err := os.MkdirAll(seed, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"plugin-registry.json", "yang-config-tree.json"} {
		if err := os.WriteFile(filepath.Join(seed, name), []byte(`[{"name":"stale"}]`), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := Build(BuildOptions{Repository: root, Output: output}); err != nil {
		t.Fatalf("build: %v", err)
	}

	registry := readArtifact(t, output, pluginFile)
	if strings.Contains(registry, "stale") {
		t.Error("the published plugin registry survived from the seed")
	}
	var plugins []registryPlugin
	if err := json.Unmarshal([]byte(registry), &plugins); err != nil {
		t.Fatalf("the published plugin registry does not parse: %v", err)
	}
	if len(plugins) != 1 || plugins[0].SourceDir != "internal/plugins/static" {
		t.Errorf("the published plugin registry states %+v", plugins)
	}

	tree := readArtifact(t, output, configTreeFile)
	if strings.Contains(tree, "stale") {
		t.Error("the published configuration tree survived from the seed")
	}
	var sections map[string]configNode
	if err := json.Unmarshal([]byte(tree), &sections); err != nil {
		t.Fatalf("the published configuration tree does not parse: %v", err)
	}
	if _, found := sections["static"]; !found {
		t.Errorf("the published configuration tree states %v", sections)
	}
}
