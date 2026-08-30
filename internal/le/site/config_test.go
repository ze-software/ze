// Design: website/AI.md -- the configuration reference reads the live YANG tree
package site

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// configurationPaths lays out one tree whose artifact carries the plugin
// registry and the configuration tree as gh-pages 2fa8fa2ad published them.
//
// Both are read from the ARTIFACT because a build publishes them before any
// producer runs: the tree comes from the daemon's own schema command and the
// registry from the daemon's own registrations, and neither has a committed
// source file to read instead.
func configurationPaths(t *testing.T) Paths {
	t.Helper()
	root := repositoryRoot(t)
	source := t.TempDir()
	output := t.TempDir()
	copyFixture(t, filepath.Join(root, "website", "data", "page-links.json"),
		filepath.Join(source, "data", "page-links.json"))
	copyFixture(t, filepath.Join("testdata", "published-plugin-registry.json"),
		filepath.Join(output, filepath.FromSlash(pluginFile)))
	copyFixture(t, filepath.Join("testdata", "published-yang-config-tree.json"),
		filepath.Join(output, filepath.FromSlash(configTreeFile)))
	return Paths{Repository: root, Source: source, Output: output}
}

// writeConfigTree replaces the artifact's configuration tree, so a case can
// state the schema it wants.
func writeConfigTree(t *testing.T, paths Paths, tree map[string]any) {
	t.Helper()
	content, err := json.MarshalIndent(tree, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(paths.Output, filepath.FromSlash(configTreeFile))
	if err := os.WriteFile(path, append(content, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: AC-3, AC-4. The configuration reference reads as the published
// page and carries the chrome the published page carries.
//
// The body fixture is the published <main> with the two application/json
// payloads emptied. visibleText does not read a script's content, so the
// comparison is unchanged by the cut, and the payloads have a case of their own
// below.
func TestTheConfigurationReferenceReadsAsThePublishedPage(t *testing.T) {
	paths := configurationPaths(t)

	routes, err := renderConfiguration(paths)
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 1 || routes[0] != configurationURL {
		t.Fatalf("the reference claimed %v, want [%s]", routes, configurationURL)
	}

	page := readArtifact(t, paths.Output, configurationDest)
	for _, chrome := range []string{
		"<title>Configuration Reference - Ze</title>",
		`<link rel="canonical" href="https://ze-software.net/reference/configuration/" />`,
		`<link rel="stylesheet" href="../../assets/site.css" />`,
		`<main id="top" class="has-page-sidebar" tabindex="-1">`,
		`<section aria-labelledby="config-ref-title" class="md-content reveal cat-operate">`,
		`<h1 id="config-ref-title">Configuration Reference</h1>`,
		`<div class="config-explorer" data-config-explorer>`,
		`<nav class="config-crumbs" aria-label="Breadcrumb"></nav>`,
		`<aside class="page-sidebar" aria-label="Related page links">`,
		"<footer>",
	} {
		if !strings.Contains(page, chrome) {
			t.Errorf("the configuration reference is missing %q", chrome)
		}
	}

	got := visibleText(mainContent(t, page))
	want := visibleText(readFixture(t, "published-configuration-body.html"))
	if got != want {
		t.Errorf("the reference reads as\n  %q\nthe published page reads as\n  %q", got, want)
	}
}

// VALIDATES: AC-4. The two payloads the browser reads are published, parse, and
// carry the whole schema and every plugin-owned path.
//
// Without them the page renders an empty level and says nothing, which no
// check over the visible text can see: the reader meets a browser with nothing
// to browse.
func TestTheConfigurationPayloadsCarryTheSchemaAndItsOwners(t *testing.T) {
	paths := configurationPaths(t)
	if _, err := renderConfiguration(paths); err != nil {
		t.Fatal(err)
	}
	page := readArtifact(t, paths.Output, configurationDest)

	tree := map[string]configNode{}
	decodeEmbeddedJSON(t, page, `<script id="config-tree" type="application/json">`, &tree)
	if len(tree) != 36 {
		t.Errorf("the page embeds %d configuration sections, the published tree has 36", len(tree))
	}
	if _, found := tree["bgp"]; !found {
		t.Error("the embedded tree has no bgp section")
	}

	owners := map[string]configOwnership{}
	decodeEmbeddedJSON(t, page, `<script id="config-owners" type="application/json">`, &owners)
	kernel, found := owners["fib/kernel"]
	if !found {
		t.Fatal("the embedded ownership says nothing about fib/kernel")
	}
	if kernel.Label != "fib-kernel" || len(kernel.Plugins) != 1 {
		t.Errorf("fib/kernel is owned by %q with %d plugins", kernel.Label, len(kernel.Plugins))
	}
	if len(kernel.Plugins) == 1 && (len(kernel.Plugins[0].YANG) != 1 ||
		kernel.Plugins[0].YANG[0].Href != repositoryBlob+"/internal/plugins/fib/kernel/yang/ze-fib-conf.yang") {
		t.Errorf("fib/kernel's YANG source is %v", kernel.Plugins[0].YANG)
	}
}

// decodeEmbeddedJSON reads one embedded payload out of the page.
func decodeEmbeddedJSON(t *testing.T, page, opening string, value any) {
	t.Helper()
	start := strings.Index(page, opening)
	if start < 0 {
		t.Fatalf("the page carries no %s", opening)
	}
	start += len(opening)
	end := strings.Index(page[start:], "</script>")
	if end < 0 {
		t.Fatalf("the payload after %s is never closed", opening)
	}
	payload := strings.ReplaceAll(page[start:start+end], `\u003c`, "<")
	if err := json.Unmarshal([]byte(payload), value); err != nil {
		t.Fatalf("the payload after %s does not parse: %v", opening, err)
	}
}

// VALIDATES: AC-5. The mirror states the whole configuration as the published
// mirror states it, section by section.
//
// The comparison starts after the opening paragraph, because this producer
// corrects one link the published mirror got wrong: see the case below.
func TestTheConfigurationMirrorReadsAsThePublishedMirror(t *testing.T) {
	paths := configurationPaths(t)
	if _, err := renderConfiguration(paths); err != nil {
		t.Fatal(err)
	}

	got := sectionsOf(t, readArtifact(t, paths.Output, configurationRoute+pageMirrorFile))
	want := sectionsOf(t, readFixture(t, "published-configuration.md"))
	if got == want {
		return
	}
	gotLines, wantLines := strings.Split(got, "\n"), strings.Split(want, "\n")
	for index := range max(len(gotLines), len(wantLines)) {
		gotLine, wantLine := lineAt(gotLines, index), lineAt(wantLines, index)
		if gotLine != wantLine {
			t.Fatalf("the mirror differs at section line %d\n  rendered:  %q\n  published: %q",
				index+1, gotLine, wantLine)
		}
	}
}

// sectionsOf answers a mirror from its first section heading onward.
func sectionsOf(t *testing.T, mirror string) string {
	t.Helper()
	start := strings.Index(mirror, "\n## ")
	if start < 0 {
		t.Fatal("the mirror carries no section")
	}
	return mirror[start:]
}

// lineAt answers one line, or the empty string past the end.
func lineAt(lines []string, index int) string {
	if index < len(lines) {
		return lines[index]
	}
	return ""
}

// VALIDATES: the page and its mirror both send a reader to the configuration
// guide's own route, on one address each.
//
// The published mirror linked reference/feature-status/configuration/, which
// the site has never published: the retired build's legacy-URL rewriting
// matched the features.md prefix and rewrote a path it did not own. The
// published page linked docs/features/configuration/, which is a redirect stub
// onto features/bgp-configuration/. One target, and it resolves.
func TestTheGuideLinkResolvesOnBothSurfaces(t *testing.T) {
	paths := configurationPaths(t)
	if _, err := renderConfiguration(paths); err != nil {
		t.Fatal(err)
	}

	page := readArtifact(t, paths.Output, configurationDest)
	if !strings.Contains(page, `<a href="../../features/bgp-configuration/">Configuration guide</a>`) {
		t.Error("the page does not link the configuration guide's own route")
	}
	mirror := readArtifact(t, paths.Output, configurationRoute+pageMirrorFile)
	if !strings.Contains(mirror, "[the Configuration guide](https://ze-software.net/features/bgp-configuration/)") {
		t.Error("the mirror does not link the configuration guide's own route")
	}
	for _, dead := range []string{"reference/feature-status/configuration/", "docs/features/configuration/"} {
		if strings.Contains(page, dead) || strings.Contains(mirror, dead) {
			t.Errorf("a surface still links %s", dead)
		}
	}
}

// VALIDATES: AC-8's own shape here. The sections are published in name order
// and the order never comes from a Go map.
func TestTheConfigurationSectionsAreInNameOrder(t *testing.T) {
	paths := configurationPaths(t)
	if _, err := renderConfiguration(paths); err != nil {
		t.Fatal(err)
	}
	mirror := readArtifact(t, paths.Output, configurationRoute+pageMirrorFile)

	previous := ""
	for line := range strings.SplitSeq(mirror, "\n") {
		name, isSection := strings.CutPrefix(line, "## ")
		if !isSection {
			continue
		}
		if name <= previous {
			t.Errorf("section %q follows %q, which is not name order", name, previous)
		}
		previous = name
	}
	if previous == "" {
		t.Error("the mirror states no section")
	}
}

// VALIDATES: a config root no schema node answers stops the build and is named,
// rather than publishing the owning plugin's section as core.
func TestAConfigRootWithNoSchemaNodeIsRefused(t *testing.T) {
	paths := configurationPaths(t)
	writeConfigTree(t, paths, map[string]any{
		"bfd": map[string]any{"name": "bfd", "kind": "container"},
	})

	_, err := renderConfiguration(paths)
	if err == nil {
		t.Fatal("the reference published a tree that answers almost no declared config root")
	}
	for _, want := range []string{"resolves to no node", "bgp", "declared by"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal %q does not say %q", err, want)
		}
	}
}

// VALIDATES: an empty or unreadable configuration tree is refused rather than
// published as a browser with nothing in it.
func TestAnEmptyConfigurationTreeIsRefused(t *testing.T) {
	for _, testCase := range []struct {
		name    string
		content string
		want    string
	}{
		{"an empty object", "{}\n", "names no configuration section"},
		{"not JSON at all", "not json\n", "read the published"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			paths := configurationPaths(t)
			path := filepath.Join(paths.Output, filepath.FromSlash(configTreeFile))
			if err := os.WriteFile(path, []byte(testCase.content), 0o644); err != nil {
				t.Fatal(err)
			}
			_, err := renderConfiguration(paths)
			if err == nil {
				t.Fatalf("the reference published %q", testCase.content)
			}
			if !strings.Contains(err.Error(), testCase.want) {
				t.Errorf("the refusal %q does not say %q", err, testCase.want)
			}
		})
	}
}

// VALIDATES: a keyed list reads as an operator would type it, and a leaf states
// the type it takes.
func TestANodeReadsAsAnOperatorWouldTypeIt(t *testing.T) {
	for _, testCase := range []struct {
		node  configNode
		head  string
		badge string
	}{
		{configNode{Name: "peer", Kind: "list[name]"}, "peer <name>", "list"},
		{configNode{Name: "hold-time", Kind: "leaf", Type: "uint16"}, "hold-time", "uint16"},
		{configNode{Name: "servers", Kind: "leaf-list", Type: "string"}, "servers", "string[]"},
		{configNode{Name: "bgp", Kind: "container"}, "bgp", "container"},
	} {
		if head := configNodeHead(&testCase.node); head != testCase.head {
			t.Errorf("%s reads as %q, want %q", testCase.node.Name, head, testCase.head)
		}
		if badge := configNodeBadge(&testCase.node); badge != testCase.badge {
			t.Errorf("%s is badged %q, want %q", testCase.node.Name, badge, testCase.badge)
		}
	}
}

// VALIDATES: a description holding a closing script tag cannot end the element
// that carries the configuration tree.
//
// The tree is the published file's own bytes rather than something Go encoded,
// so nothing else escapes them. Unescaped, the browser would read the rest of
// the schema as markup and the page would render as garbage from that node on.
func TestAClosingScriptTagInASchemaDescriptionCannotEndThePayload(t *testing.T) {
	paths := configurationPaths(t)
	// Written as bytes rather than through Go's encoder, which escapes "<" on
	// the way out: the guard exists for a payload that reaches the page with
	// its angle brackets intact.
	writeArtifactFile(t, paths.Output, configTreeFile,
		`{"static":{"name":"static","kind":"container",`+
			`"description":"Static routes. See </script><b>here</b> for the rest."}}`+"\n")
	writePublishedRegistry(t, paths, []registryPlugin{
		{Name: "static", Description: "Static routes", ConfigRoots: []string{"static"},
			SourceDir: "internal/plugins/static"},
	})

	if _, err := renderConfiguration(paths); err != nil {
		t.Fatal(err)
	}
	page := readArtifact(t, paths.Output, configurationDest)
	opening := `<script id="config-tree" type="application/json">`
	_, payload, found := strings.Cut(page, opening)
	if !found {
		t.Fatalf("the page carries no %s", opening)
	}
	payload, _, found = strings.Cut(payload, "</script>")
	if !found {
		t.Fatal("the configuration tree payload is never closed")
	}
	if strings.Contains(payload, "</b>") {
		t.Error("the description escaped its own payload and reached the page as markup")
	}
	tree := map[string]configNode{}
	decodeEmbeddedJSON(t, page, `<script id="config-tree" type="application/json">`, &tree)
	if tree["static"].Description != "Static routes. See </script><b>here</b> for the rest." {
		t.Errorf("the description reached the browser as %q", tree["static"].Description)
	}
}
