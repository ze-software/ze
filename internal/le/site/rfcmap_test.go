// Design: docs/contributing/gh-pages.md -- standalone RFC map publication
// Related: rfcmap_model_test.go supplies the small recorded-input fixture.
package site

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// TestRFCMapProducerPublishesNewPage renders recorded inputs and checks the
// route, static fallback, mirror, local assets and independent proof boundary.
func TestRFCMapProducerPublishesNewPage(t *testing.T) {
	paths := rfcMapModelFixture(t)
	rfcMapTestAssets(t, paths)
	ledgerPath := filepath.Join(paths.Output, rfcLedgerFile)
	ledger, err := os.ReadFile(ledgerPath)
	if err != nil {
		t.Fatal(err)
	}
	writeFixtureFile(t, ledgerPath, strings.Replace(string(ledger), `"summary":true`,
		`"summary":true,"walked":true,"mandatory-proven":7,"mandatory-total":11`, 1))
	before := rfcMapTestFiles(t, paths.Output)
	routes, err := renderRFCMap(paths)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(routes, []string{"/quality/rfc-map/"}) {
		t.Errorf("producer claims %v, want only the map route", routes)
	}
	after := rfcMapTestFiles(t, paths.Output)
	var added []string
	for name := range after {
		if _, exists := before[name]; !exists {
			added = append(added, name)
		}
	}
	slices.Sort(added)
	want := []string{"assets/rfc-map.css", "assets/rfc-map.js", "data/rfc-map.json",
		"quality/rfc-map/index.html", "quality/rfc-map/index.md"}
	if !slices.Equal(added, want) {
		t.Errorf("new artifacts = %v, want %v", added, want)
	}
	page := after["quality/rfc-map/index.html"]
	for _, fragment := range []string{
		`data-rfc-map`, `data-source="../../data/rfc-map.json"`,
		`data-rfc-base="../../data/rfc/"`, `data-roadmap="../../data/release-roadmap.json"`,
		`href="../../assets/rfc-map.css"`, `src="../../assets/rfc-map.js"`,
		`<noscript>`, `href="../rfc-compliance/rfc4271/"`,
		`rfc10000`, `no recorded summary`, `revision and age are unknown`,
	} {
		if !strings.Contains(page, fragment) {
			t.Errorf("published page omits %q", fragment)
		}
	}
	mirror := after["quality/rfc-map/index.md"]
	for _, fragment := range []string{"# RFC dependency map", "recorded-revision",
		"rfc10000 (no recorded summary)", "[Existing profiles](../rfc-compliance/profiles/)",
		"[Specification inventory](../../project/roadmap/)", "SHA-256"} {
		if !strings.Contains(mirror, fragment) {
			t.Errorf("static mirror omits %q", fragment)
		}
	}
	if after["assets/rfc-map.css"] != "/* map-local fixture */\n" {
		t.Error("page-local stylesheet changed during publication")
	}
	if after["assets/rfc-map.js"] != "/* map-local script fixture */\n" {
		t.Error("page-local script changed during publication")
	}
	var projected struct {
		Nodes []map[string]json.RawMessage `json:"nodes"`
	}
	if err := json.Unmarshal([]byte(after["data/rfc-map.json"]), &projected); err != nil {
		t.Fatal(err)
	}
	if len(projected.Nodes) != 4 {
		t.Fatalf("published map carries %d nodes, want four", len(projected.Nodes))
	}
	for _, node := range projected.Nodes {
		for _, key := range []string{"mandatory-proven", "mandatory-total", "optional-proven",
			"optional-total", "mandatory-by-kind", "sections"} {
			if _, copied := node[key]; copied {
				t.Errorf("map node copies proof measurement %q instead of referring to canonical figures", key)
			}
		}
	}
}

// TestRFCMapPreservesExistingPages checks successful publication and preflight
// failures against byte snapshots of existing pages, shared assets and map files.
func TestRFCMapPreservesExistingPages(t *testing.T) {
	for _, mode := range []string{"successful publication", "invalid recorded input", "missing local asset"} {
		t.Run(mode, func(t *testing.T) {
			paths := rfcMapModelFixture(t)
			rfcMapTestAssets(t, paths)
			if mode == "invalid recorded input" {
				writeFixtureFile(t, filepath.Join(paths.Output, rfcLedgerFile), "{invalid JSON")
			}
			if mode == "missing local asset" {
				paths.Source = filepath.Join(paths.Repository, "missing-assets")
				manifest, err := os.ReadFile(filepath.Join(paths.Repository, "website", rfcMapLayersFile))
				if err != nil {
					t.Fatal(err)
				}
				writeFixtureFile(t, filepath.Join(paths.Source, rfcMapLayersFile), string(manifest))
			}
			for _, name := range []string{"index.html", "guides/example/index.md", "assets/site.js",
				"assets/site.css", "quality/rfc-map/index.html", "quality/rfc-map/index.md",
				"assets/rfc-map.css", "assets/rfc-map.js", "data/rfc-map.json"} {
				writeFixtureFile(t, filepath.Join(paths.Output, name), "existing bytes: "+name+"\n")
			}
			before := rfcMapTestFiles(t, paths.Output)
			_, err := renderRFCMap(paths)
			if mode == "successful publication" {
				if err != nil {
					t.Fatal(err)
				}
				after := rfcMapTestFiles(t, paths.Output)
				for _, name := range []string{"index.html", "guides/example/index.md", "assets/site.js",
					"assets/site.css", rfcLedgerFile, roadmapDataFile} {
					if after[name] != before[name] {
						t.Errorf("standalone producer changed existing artifact %s", name)
					}
				}
				return
			}
			if err == nil {
				t.Fatal("broken prerequisite did not refuse publication")
			}
			if after := rfcMapTestFiles(t, paths.Output); !reflect.DeepEqual(after, before) {
				t.Error("preflight failure changed existing output before refusing")
			}
		})
	}
}

// rfcMapTestAssets supplies only the two local assets this producer must copy.
func rfcMapTestAssets(t *testing.T, paths Paths) {
	t.Helper()
	writeFixtureFile(t, filepath.Join(paths.Source, "assets", "rfc-map.css"), "/* map-local fixture */\n")
	writeFixtureFile(t, filepath.Join(paths.Source, "assets", "js", "rfc-map.js"), "/* map-local script fixture */\n")
}

// rfcMapTestFiles snapshots the bounded files in one test-owned artifact root.
func rfcMapTestFiles(t *testing.T, root string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(root, func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(root, name)
		if err != nil {
			return err
		}
		content, err := os.ReadFile(name)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(relative)] = string(content)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}
