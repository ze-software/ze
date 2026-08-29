// Design: website/AI.md -- llms.txt is the one denormalized answer for a machine reader
package site

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// llmsPaths lays out one artifact carrying every data file llms.txt reads.
//
// The five files a page family owns are this checkout's own committed sources,
// so the sections derived from them are the ones the site really publishes. The
// three a later phase produces are small fixtures, because no producer writes
// them yet: phase 7 writes the plugin registry, phase 9 the facts snapshot, and
// the configuration reference the YANG tree.
func llmsPaths(t *testing.T) Paths {
	t.Helper()
	paths := commandSurfacePaths(t)
	root := repositoryRoot(t)
	for _, name := range []string{featuresFile, dependencyFile, navFile} {
		copyFixture(t, filepath.Join(root, "website", filepath.FromSlash(name)),
			filepath.Join(paths.Output, filepath.FromSlash(name)))
	}
	writeArtifactFile(t, paths.Output, factsFile, `{"cli_commands":402,"config_sections":36,
		"dependencies":42,"changes":37,"features":{"core_experimental":52,"planned":4},
		"tests":{"unit_display":"23,700+","fuzz_display":"78","e2e_display":"1,700+"},
		"interop":{"scenarios":130,"target_display":"9"},"generated_at":"2026-08-27"}`)
	writeArtifactFile(t, paths.Output, pluginFile, `[{"name":"bfd",
		"description":"Bidirectional Forwarding Detection (RFC 5880, 5881, 5883)",
		"config_roots":["bfd"],"dependencies":[],"optional_dependencies":[],
		"source_dir":"internal/component/bfd","yang_files":["a.yang","b.yang"]}]`)
	writeArtifactFile(t, paths.Output, configTreeFile, `{"bgp":{"kind":"container",
		"description":"BGP speaker configuration.","children":[{"name":"group"},{"name":"neighbor"}]}}`)
	return paths
}

// writeArtifactFile puts one data file into an artifact.
func writeArtifactFile(t *testing.T, output, name, content string) {
	t.Helper()
	path := filepath.Join(output, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// llmsSection matches a top-level heading of llms.txt.
var llmsSection = regexp.MustCompile(`(?m)^(#|##) .+$`)

// VALIDATES: llms.txt carries every section the retired renderer emitted, not
// the command section alone.
//
// This is the second live regression phase 4 closes. Since the interpreter
// cutover, internal/le/docvalid wrote the whole file from the command catalog:
// the published file is 1035 lines and the one a Go build produced was 399,
// carrying one of its eighteen sections. Nothing noticed, because no check
// answers for a published file that is not a route.
func TestLLMSCarriesEverySectionThePublishedFileHas(t *testing.T) {
	paths := llmsPaths(t)

	routes, err := renderLLMS(paths)
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 0 {
		t.Errorf("the llms producer claimed %v; llms.txt is a published file, not a route", routes)
	}
	content := readArtifact(t, paths.Output, llmsFile)
	got := llmsSection.FindAllString(content, -1)
	want := strings.Split(strings.TrimSpace(readFixture(t, "published-llms-sections.txt")), "\n")
	if len(got) != len(want) {
		t.Fatalf("llms.txt carries %d sections and the published file carries %d:\ngot  %v\nwant %v",
			len(got), len(want), got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Errorf("section %d is %q, the published file has %q", index+1, got[index], want[index])
		}
	}
}

// VALIDATES: each restored section states the facts it exists to carry.
//
// A section heading with nothing under it would satisfy the test above, so each
// section is checked for one fact only its own input can produce.
func TestEachLLMSSectionCarriesItsOwnInput(t *testing.T) {
	paths := llmsPaths(t)
	if _, err := renderLLMS(paths); err != nil {
		t.Fatal(err)
	}
	content := readArtifact(t, paths.Output, llmsFile)

	for _, fact := range []struct{ section, evidence string }{
		{"Product snapshot", "52 shipped or experimental feature cards, 4 roadmap cards, 402 CLI commands"},
		{"Quality and verification model", "`./le verify worktree` runs the whole native verification"},
		{"Comparison positioning", "Ze is compared with VyOS and freeRtr"},
		{"Feature inventory", "- AI Tool Interfaces [automate, current]: chips: MCP"},
		{"Configuration model roots", "- `bgp`: BGP speaker configuration. Children: group, neighbor."},
		{"Plugin registry", "- `bfd`: Bidirectional Forwarding Detection"},
		{"CLI command surface", "- `show bgp` (read-only; wire ze-bgp:overview"},
		{"Vendor command equivalents", "Ze: `show bgp`. Junos MX: `show bgp summary` (operational, verified)"},
		{"Dependency rationale", "- `charm.land/bubbletea/v2`: The Elm-style TUI framework"},
		{"Complete documentation index", "(web: https://ze-software.net/use-cases/route-server/)"},
		{"Page map", "(web: https://ze-software.net/guides/quickstart/)"},
	} {
		if !strings.Contains(content, fact.evidence) {
			t.Errorf("the %s section does not carry %q", fact.section, fact.evidence)
		}
	}
}

// VALIDATES: the page map states this build's own numbers, not the menu's prose.
//
// Four navigation descriptions name a count. website/data/nav.json writes each
// one by hand, so "402 commands" is authored once and read for a year. The
// build already derived the number, so the file states the derived one.
func TestThePageMapTakesItsCountsFromTheFactsSnapshot(t *testing.T) {
	paths := llmsPaths(t)
	writeArtifactFile(t, paths.Output, factsFile, `{"cli_commands":7,"config_sections":36,
		"dependencies":3,"changes":9,"features":{"core_experimental":5,"planned":4},
		"tests":{"unit_display":"1","fuzz_display":"2","e2e_display":"3"},
		"interop":{"scenarios":4,"target_display":"5"},"generated_at":"2026-08-27"}`)

	if _, err := renderLLMS(paths); err != nil {
		t.Fatal(err)
	}
	content := readArtifact(t, paths.Output, llmsFile)
	for _, fresh := range []string{
		"5 features, color-coded by category",
		"7 commands, generated from the live binary",
		"3 direct packages, generated from go.mod",
		"9 weekly updates, newest first",
	} {
		if !strings.Contains(content, fresh) {
			t.Errorf("the page map does not carry %q", fresh)
		}
	}
}

// VALIDATES: an input llms.txt states a fact from stops the build when it is
// absent, rather than leaving the section out.
//
// A file that reads complete and states less than it claims is the exact defect
// this phase closes, so a missing input is a refusal and not a shorter file.
func TestAMissingLLMSInputStopsTheBuild(t *testing.T) {
	paths := llmsPaths(t)
	absent := filepath.Join(paths.Output, filepath.FromSlash(pluginFile))
	if err := os.Remove(absent); err != nil {
		t.Fatal(err)
	}

	_, err := renderLLMS(paths)
	if err == nil {
		t.Fatal("llms.txt was published without its plugin registry rather than refused")
	}
	if !strings.Contains(err.Error(), pluginFile) {
		t.Errorf("the refusal does not name the missing input: %v", err)
	}
	if _, err := os.Stat(filepath.Join(paths.Output, llmsFile)); !os.IsNotExist(err) {
		t.Errorf("a partial llms.txt was written before the refusal: %v", err)
	}
}
